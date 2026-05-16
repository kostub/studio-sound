package ipc

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
)

type Handler func(ctx context.Context, id string, payload json.RawMessage) (any, error)

type Dispatcher struct {
	mu       sync.RWMutex
	handlers map[string]Handler
	log      *slog.Logger
}

// NewDispatcher creates a new Dispatcher. The provided logger is used for
// structured logging within the dispatch loop (e.g. handler panics). If log
// is nil, the default slog logger is used.
func NewDispatcher(log *slog.Logger) *Dispatcher {
	if log == nil {
		log = slog.Default()
	}
	return &Dispatcher{handlers: make(map[string]Handler), log: log}
}

func (d *Dispatcher) Register(method string, h Handler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[method] = h
}

var (
	newline = []byte{'\n'}
)

// maxConcurrentDispatch caps the number of in-flight handler goroutines.
// Documented in docs/ipc-contract.md as the SIDECAR_BUSY threshold.
const maxConcurrentDispatch = 64

func (d *Dispatcher) Serve(ctx context.Context, stdin io.Reader, stdout io.Writer) error {
	var wmu sync.Mutex
	writeLine := func(b []byte) {
		wmu.Lock()
		defer wmu.Unlock()
		_, _ = stdout.Write(b)
		_, _ = stdout.Write(newline)
	}

	reader := bufio.NewReaderSize(stdin, maxMessageSize+1)

	// If ctx is cancelled while we're blocked in readLine, close stdin so the
	// blocking syscall returns. Without this, system.shutdown cancels ctx but
	// the readLine call only unblocks when its caller (e.g. the Rust
	// supervisor) closes the write end of the pipe.
	if closer, ok := stdin.(io.Closer); ok {
		closeOnce := sync.Once{}
		go func() {
			<-ctx.Done()
			closeOnce.Do(func() { _ = closer.Close() })
		}()
	}

	// Bounded semaphore: cap concurrent in-flight handler goroutines.
	sem := make(chan struct{}, maxConcurrentDispatch)

	var wg sync.WaitGroup
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		default:
		}

		// Read up to maxMessageSize+1 bytes to detect oversize lines.
		line, err := readLine(reader)
		if err != nil {
			wg.Wait()
			if errors.Is(err, io.EOF) {
				return io.EOF
			}
			// If ctx was cancelled, the stdin Close above may have surfaced
			// as a "file already closed" error — translate that to ctx.Err().
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			return err
		}

		// Acquire a dispatch slot before launching the goroutine. We tolerate
		// brief blocking here on the read loop — this is the documented
		// back-pressure mechanism for in-flight requests.
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			d.dispatch(ctx, line, writeLine)
		}()
	}
}

// readLine reads one newline-terminated line from r, returning the line
// without the trailing newline. If the line exceeds maxMessageSize bytes,
// the oversized bytes are drained and a sentinel oversize slice is returned
// so that dispatch can emit MESSAGE_TOO_LARGE.
func readLine(r *bufio.Reader) ([]byte, error) {
	// Peek up to maxMessageSize+1 bytes to check for oversize before reading.
	// We use ReadSlice which reads up to the delimiter or buffer capacity.
	var full []byte
	for {
		frag, err := r.ReadSlice('\n')
		full = append(full, frag...)
		if err == nil {
			// We hit the newline.
			break
		}
		if err == bufio.ErrBufferFull {
			// Partial read; buffer full but no newline yet — continue accumulating
			// but stop at maxMessageSize+1 so we can detect the oversize condition.
			if len(full) > maxMessageSize {
				// Drain the rest of the line so the stream can recover.
				for {
					_, derr := r.ReadSlice('\n')
					if derr == nil || derr == io.EOF {
						break
					}
					if derr != bufio.ErrBufferFull {
						break
					}
				}
				// Return an oversize sentinel: a slice larger than maxMessageSize.
				return full, nil
			}
			continue
		}
		if err == io.EOF {
			if len(full) == 0 {
				return nil, io.EOF
			}
			// Partial line at EOF — treat as the line content.
			break
		}
		return nil, err
	}
	// Trim the trailing newline (and optional carriage return).
	line := full
	line = bytes.TrimRight(line, "\r\n")
	return line, nil
}

func (d *Dispatcher) dispatch(ctx context.Context, line []byte, write func([]byte)) {
	env, err := DecodeLine(line)
	if err != nil {
		var code string
		if errors.Is(err, ErrMessageTooLarge) {
			code = CodeMessageTooLarge
		} else if errors.Is(err, ErrProtocolVersionMismatch) {
			code = CodeProtocolVersionMismatch
		} else {
			code = CodeMalformedEnvelope
		}
		d.writeError(write, env.ID, NewRPCError(code, err.Error()), "decode error")
		return
	}

	d.mu.RLock()
	h, ok := d.handlers[env.Method]
	d.mu.RUnlock()

	if !ok {
		d.writeError(write, env.ID, NewRPCError(CodeUnknownMethod, "unknown method: "+env.Method), "unknown method")
		return
	}

	defer func() {
		if r := recover(); r != nil {
			d.log.Error("handler panic", "method", env.Method, "panic", r)
			d.writeError(write, env.ID, NewRPCError(CodeInternalError, "internal error"), "handler panic")
		}
	}()

	result, herr := h(ctx, env.ID, env.Payload)
	if herr != nil {
		var rpcErr *RPCError
		if errors.As(herr, &rpcErr) {
			d.writeError(write, env.ID, rpcErr, "handler RPCError")
		} else {
			d.writeError(write, env.ID, NewRPCError(CodeInternalError, herr.Error()), "handler error")
		}
		return
	}

	b, err := EncodeResponse(env.ID, result)
	if err != nil {
		d.log.Error("failed to encode response", "id", env.ID, "method", env.Method, "error", err)
		// Fall back to an INTERNAL_ERROR envelope so the caller doesn't hang
		// until its timeout fires. The fallback only references concrete
		// string fields, so it cannot fail for the same reason.
		d.writeError(write, env.ID, NewRPCError(CodeInternalError, "failed to encode response"), "response encode failure")
		return
	}
	write(b)
}

// writeError encodes an RPC error envelope and writes it. If encoding fails
// (only possible for exotic RPCError shapes that should never occur in
// practice), the failure is logged and the envelope is dropped — there is no
// further fallback to attempt.
func (d *Dispatcher) writeError(write func([]byte), id string, rpcErr *RPCError, errContext string) {
	b, err := EncodeError(id, rpcErr)
	if err != nil {
		d.log.Error("failed to encode error envelope", "context", errContext, "id", id, "error", err)
		return
	}
	write(b)
}
