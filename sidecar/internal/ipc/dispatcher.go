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

func (d *Dispatcher) Serve(ctx context.Context, stdin io.Reader, stdout io.Writer) error {
	var wmu sync.Mutex
	writeLine := func(b []byte) {
		wmu.Lock()
		defer wmu.Unlock()
		_, _ = stdout.Write(append(b, '\n'))
	}

	reader := bufio.NewReaderSize(stdin, maxMessageSize+1)

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
			return err
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
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
		} else {
			code = CodeMalformedEnvelope
		}
		b, _ := EncodeError(env.ID, NewRPCError(code, err.Error()))
		write(b)
		return
	}

	d.mu.RLock()
	h, ok := d.handlers[env.Method]
	d.mu.RUnlock()

	if !ok {
		b, _ := EncodeError(env.ID, NewRPCError(CodeUnknownMethod, "unknown method: "+env.Method))
		write(b)
		return
	}

	defer func() {
		if r := recover(); r != nil {
			d.log.Error("handler panic", "method", env.Method, "panic", r)
			b, _ := EncodeError(env.ID, NewRPCError(CodeInternalError, "internal error"))
			write(b)
		}
	}()

	result, herr := h(ctx, env.ID, env.Payload)
	if herr != nil {
		var rpcErr *RPCError
		if errors.As(herr, &rpcErr) {
			b, _ := EncodeError(env.ID, rpcErr)
			write(b)
		} else {
			b, _ := EncodeError(env.ID, NewRPCError(CodeInternalError, herr.Error()))
			write(b)
		}
		return
	}

	b, _ := EncodeResponse(env.ID, result)
	write(b)
}
