//! Sidecar supervisor.
//!
//! Manages the lifecycle of the Go sidecar child process.  Callers can:
//! - spawn the sidecar via [`Supervisor::spawn`]
//! - send envelopes to its stdin via [`Supervisor::send`]
//! - receive decoded response envelopes from its stdout via [`Supervisor::subscribe`]
//! - request a graceful shutdown via [`Supervisor::shutdown`]
//!
//! ## Auto-restart on crash
//!
//! The supervisor monitors the child process via its `CommandEvent` stream.
//! When the child exits *without* a prior call to [`Supervisor::shutdown`],
//! the supervisor respawns it (with capped exponential backoff). Pending
//! requests on the previous child are not replayed — they will time out on
//! the [`IpcClient`] layer above us. This matches the testability bar in the
//! Phase 1 plan ("Killing the sidecar process externally causes the UI to
//! surface a clean error and the app to respawn it on the next call").

use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::time::Duration;

use tauri::{AppHandle, Manager};
use tauri_plugin_shell::process::{CommandChild, CommandEvent};
use tauri_plugin_shell::ShellExt;
use tokio::sync::broadcast;

use crate::ipc::envelope::{decode_line, Envelope, Kind};
use crate::ipc::error::IpcError;

#[cfg(test)]
use crate::ipc::envelope::PROTOCOL_VERSION;

/// Capacity of the broadcast channel that fans out decoded response envelopes.
const CHANNEL_CAPACITY: usize = 256;

/// Maximum number of consecutive respawn attempts before the supervisor
/// gives up and transitions to a permanent unavailable state.
const MAX_RESTART_ATTEMPTS: u32 = 8;

/// Initial backoff between respawn attempts; doubles each attempt up to
/// `MAX_BACKOFF`.
const INITIAL_BACKOFF: Duration = Duration::from_millis(200);
const MAX_BACKOFF: Duration = Duration::from_secs(30);

/// Information needed to spawn (or respawn) the sidecar.
struct SpawnContext {
    app: AppHandle,
    log_path: PathBuf,
}

/// Shared inner state owned by a [`Supervisor`].
struct Inner {
    /// Sender half of the broadcast channel.  Kept alive as long as the
    /// supervisor exists so new subscribers can be created at any time.
    tx: broadcast::Sender<Envelope>,

    /// The currently-live child handle, wrapped in `Arc<std::sync::Mutex<_>>`
    /// so that:
    ///   1. `Supervisor::send` can clone the inner `Arc` and hand it to a
    ///      `spawn_blocking` closure for the (potentially-blocking) stdin
    ///      write *without* moving the only handle into that closure — so a
    ///      `JoinError` cannot orphan the child handle.
    ///   2. The outer `std::sync::Mutex<Option<_>>` lets the read loop swap
    ///      in a freshly-spawned child after a crash, without invalidating
    ///      already-cloned Arcs (which will just write to a dead pipe and
    ///      surface the error to the caller).
    ///
    /// `None` after [`Supervisor::shutdown`] or after the restart budget is
    /// exhausted.
    state: std::sync::Mutex<Option<Arc<std::sync::Mutex<CommandChild>>>>,

    /// Set to `true` by [`Supervisor::shutdown`] so the read loop knows the
    /// child exit was intentional and skips the respawn path.
    shutdown_requested: AtomicBool,

    /// Captured spawn parameters so the read loop can respawn the child
    /// without re-querying `app.path()` etc.
    spawn_ctx: SpawnContext,
}

/// Manages a single sidecar child process.
///
/// Clone-cheap: the clone still shares the same inner channel, child handle,
/// and shutdown flag.
#[derive(Clone)]
pub struct Supervisor {
    inner: Arc<Inner>,
}

impl Supervisor {
    /// Spawns the sidecar binary (named `"studio-sidecar"` as registered in
    /// `tauri.conf.json` → `bundle.externalBin`) in `serve` mode, wires up
    /// stdout → broadcast channel decoding, and returns a ready [`Supervisor`].
    ///
    /// Sets `STUDIO_LOG_FILE=<app_log_dir>/sidecar.log` in the child's
    /// environment so the sidecar's structured logs land in the documented
    /// log directory.
    ///
    /// Returns [`IpcError::Other`] if the log directory cannot be resolved,
    /// or if the sidecar binary cannot be found or spawned for the very first
    /// time. Later respawn failures are logged and retried in the background.
    pub fn spawn(app: &AppHandle) -> Result<Self, IpcError> {
        // Resolve and create the log directory so the sidecar can open its
        // log file on first write.
        let log_dir = app.path().app_log_dir().map_err(|e| IpcError::Other {
            code: "SIDECAR_UNAVAILABLE".into(),
            message: format!("failed to resolve app log directory: {e}"),
            details: None,
        })?;
        std::fs::create_dir_all(&log_dir).map_err(|e| IpcError::Other {
            code: "SIDECAR_UNAVAILABLE".into(),
            message: format!("failed to create log directory {}: {e}", log_dir.display()),
            details: None,
        })?;
        let log_path = log_dir.join("sidecar.log");

        let (tx, _) = broadcast::channel(CHANNEL_CAPACITY);
        let inner = Arc::new(Inner {
            tx,
            state: std::sync::Mutex::new(None),
            shutdown_requested: AtomicBool::new(false),
            spawn_ctx: SpawnContext {
                app: app.clone(),
                log_path,
            },
        });

        // First spawn is synchronous so we can fail fast at startup if the
        // sidecar binary is missing.
        let (event_rx, child) = spawn_child(&inner.spawn_ctx)?;
        install_child(&inner, child);

        tokio::spawn(read_loop(Arc::clone(&inner), event_rx));

        Ok(Self { inner })
    }

    /// Serialises `env` as a single NDJSON line and writes it to the sidecar's
    /// stdin via `CommandChild::write`.
    ///
    /// Returns [`IpcError::SidecarUnavailable`] if the sidecar has been shut
    /// down, or [`IpcError::Serde`] if serialisation fails.
    pub async fn send(&self, env: &Envelope) -> Result<(), IpcError> {
        let line = {
            let mut buf = serde_json::to_string(env)?;
            buf.push('\n');
            buf
        };

        let child_arc = {
            let guard = self.inner.state.lock().unwrap_or_else(|p| p.into_inner());
            match guard.as_ref() {
                Some(c) => Arc::clone(c),
                None => return Err(IpcError::SidecarUnavailable),
            }
        };

        // `child.write` performs a synchronous stdin write that can block if
        // the OS pipe buffer is full and the sidecar is slow to drain. We hold
        // an `Arc` over the child instead of moving the only handle into the
        // closure, so even a `JoinError` (panic in the writer task) cannot
        // orphan the child — the supervisor's own Arc clone in `state` keeps
        // the handle alive.
        let bytes = line.into_bytes();
        let write_res = tokio::task::spawn_blocking(move || {
            let mut child = child_arc
                .lock()
                .unwrap_or_else(|poisoned| poisoned.into_inner());
            child.write(&bytes)
        })
        .await;

        match write_res {
            Ok(Ok(())) => Ok(()),
            Ok(Err(e)) => Err(IpcError::Other {
                code: "SIDECAR_UNAVAILABLE".into(),
                message: format!("stdin write failed: {e}"),
                details: None,
            }),
            Err(e) => Err(IpcError::Other {
                code: "INTERNAL_ERROR".into(),
                message: format!("blocking task failed: {e}"),
                details: None,
            }),
        }
    }

    /// Returns a new [`broadcast::Receiver`] that will yield every decoded
    /// [`Envelope`] received from the sidecar's stdout.
    ///
    /// Multiple subscribers can be created; each receives an independent copy
    /// of every envelope.
    pub fn subscribe(&self) -> broadcast::Receiver<Envelope> {
        self.inner.tx.subscribe()
    }

    /// Marks the supervisor as shut down: the next child-exit event will be
    /// treated as graceful, the auto-restart logic will not fire, and the
    /// child handle is dropped (which closes stdin and signals EOF to the
    /// sidecar's read loop).
    ///
    /// After this call, [`send`] returns [`IpcError::SidecarUnavailable`].
    ///
    /// [`send`]: Supervisor::send
    pub async fn shutdown(&self) {
        self.inner.shutdown_requested.store(true, Ordering::SeqCst);
        let mut guard = self
            .inner
            .state
            .lock()
            .unwrap_or_else(|p| p.into_inner());
        *guard = None;
    }
}

/// Build and spawn the sidecar command. Returns the event stream and child
/// handle on success.
fn spawn_child(
    ctx: &SpawnContext,
) -> Result<(tauri::async_runtime::Receiver<CommandEvent>, CommandChild), IpcError> {
    let cmd = ctx
        .app
        .shell()
        .sidecar("studio-sidecar")
        .map_err(|e| IpcError::Other {
            code: "SIDECAR_UNAVAILABLE".into(),
            message: format!("sidecar binary not found: {e}"),
            details: None,
        })?
        .args(["serve"])
        .env("STUDIO_LOG_FILE", ctx.log_path.to_string_lossy().to_string());

    cmd.spawn().map_err(|e| IpcError::Other {
        code: "SIDECAR_UNAVAILABLE".into(),
        message: format!("failed to spawn sidecar: {e}"),
        details: None,
    })
}

/// Replace the supervisor's child slot with a freshly-spawned handle.
fn install_child(inner: &Arc<Inner>, child: CommandChild) {
    let mut guard = inner.state.lock().unwrap_or_else(|p| p.into_inner());
    *guard = Some(Arc::new(std::sync::Mutex::new(child)));
}

/// Read the sidecar's stdout/stderr event stream, decode response envelopes,
/// and broadcast them. On unexpected termination, respawn the child with
/// capped exponential backoff.
async fn read_loop(
    inner: Arc<Inner>,
    mut event_rx: tauri::async_runtime::Receiver<CommandEvent>,
) {
    let mut attempt: u32 = 0;
    loop {
        let mut clean_exit = false;
        while let Some(event) = event_rx.recv().await {
            match event {
                CommandEvent::Stdout(line) => {
                    let line_str = match std::str::from_utf8(&line) {
                        Ok(s) => s,
                        Err(_) => {
                            tracing::warn!("sidecar stdout: non-UTF-8 line, skipping");
                            continue;
                        }
                    };
                    match decode_line(line_str) {
                        Ok(env) => {
                            if env.kind == Kind::Response || env.kind == Kind::Event {
                                let _ = inner.tx.send(env);
                            }
                        }
                        Err(e) => {
                            tracing::warn!("sidecar stdout: decode error: {e}");
                        }
                    }
                }
                CommandEvent::Stderr(line) => {
                    let s = String::from_utf8_lossy(&line);
                    tracing::debug!(target: "sidecar::stderr", "{s}");
                }
                CommandEvent::Error(e) => {
                    tracing::error!("sidecar process error: {e}");
                }
                CommandEvent::Terminated(status) => {
                    tracing::info!(
                        code = status.code,
                        signal = status.signal,
                        "sidecar terminated"
                    );
                    clean_exit = matches!(status.code, Some(0));
                    break;
                }
                _ => {}
            }
        }

        // Either the event channel closed or we received Terminated. Decide
        // whether to respawn.
        if inner.shutdown_requested.load(Ordering::SeqCst) {
            tracing::info!("sidecar reader: graceful shutdown, exiting read loop");
            return;
        }

        if clean_exit {
            // A clean exit (code 0) without a shutdown request is still
            // unexpected, but treat it as recoverable by resetting the
            // attempt counter.
            attempt = 0;
        }

        // Retry-with-backoff loop: keep trying to spawn until success or
        // until the attempt budget is exhausted.
        loop {
            attempt = attempt.saturating_add(1);
            if attempt > MAX_RESTART_ATTEMPTS {
                tracing::error!(
                    attempts = attempt - 1,
                    "sidecar restart budget exhausted; supervisor entering permanent unavailable state"
                );
                let mut guard = inner.state.lock().unwrap_or_else(|p| p.into_inner());
                *guard = None;
                return;
            }

            let backoff = INITIAL_BACKOFF
                .saturating_mul(1u32 << attempt.min(8))
                .min(MAX_BACKOFF);
            tracing::warn!(
                attempt,
                backoff_ms = backoff.as_millis() as u64,
                "respawning sidecar after backoff"
            );
            tokio::time::sleep(backoff).await;

            // Recheck shutdown — the user may have asked us to stop while we
            // were sleeping.
            if inner.shutdown_requested.load(Ordering::SeqCst) {
                tracing::info!("sidecar reader: shutdown requested during backoff");
                return;
            }

            match spawn_child(&inner.spawn_ctx) {
                Ok((new_rx, new_child)) => {
                    install_child(&inner, new_child);
                    event_rx = new_rx;
                    attempt = 0;
                    tracing::info!("sidecar respawned");
                    break;
                }
                Err(e) => {
                    tracing::error!(error = %e, "failed to respawn sidecar; will retry");
                    // Continue the retry-with-backoff loop.
                }
            }
        }
    }
}

/// Construct a minimal [`Envelope`] for use in tests.
#[cfg(test)]
fn make_request_envelope(method: &str) -> Envelope {
    Envelope {
        v: PROTOCOL_VERSION,
        id: "01TEST000000000000000000001".to_string(),
        kind: Kind::Request,
        method: Some(method.to_string()),
        payload: None,
        result: None,
        error: None,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    /// Verify that serialising an Envelope to a string and appending `\n`
    /// produces valid NDJSON that `decode_line` can round-trip back.
    #[test]
    fn ndjson_serialisation_round_trip() {
        let env = make_request_envelope("system.ping");
        let mut line = serde_json::to_string(&env).expect("serialise envelope");
        // NDJSON contract: exactly one newline at the end.
        assert!(!line.contains('\n'), "serialised JSON must not contain interior newlines");
        line.push('\n');

        // Strip the trailing newline and decode.
        let trimmed = line.trim_end_matches('\n');
        let decoded = decode_line(trimmed).expect("decode round-trip");

        assert_eq!(decoded.v, env.v);
        assert_eq!(decoded.id, env.id);
        assert_eq!(decoded.kind, env.kind);
        assert_eq!(decoded.method, env.method);
    }

    /// Verify that the broadcast channel correctly delivers a sent envelope to
    /// multiple independent subscribers (without requiring a live sidecar).
    #[tokio::test]
    async fn broadcast_channel_fanout() {
        let (tx, mut rx1) = broadcast::channel::<Envelope>(16);
        let mut rx2 = tx.subscribe();

        let env = make_request_envelope("system.echo");
        tx.send(env.clone()).expect("send to broadcast");

        let got1 = rx1.recv().await.expect("rx1 receive");
        let got2 = rx2.recv().await.expect("rx2 receive");

        assert_eq!(got1.id, env.id);
        assert_eq!(got2.id, env.id);
        assert_eq!(got1.method, env.method);
    }

    /// Verify that `send` serialises an envelope and appends exactly one `\n`.
    /// This uses a mock stdin (we test the serialisation layer here; the actual
    /// stdin write path is covered by integration tests in item 7.1).
    #[test]
    fn send_produces_valid_ndjson_bytes() {
        let env = make_request_envelope("system.ping");

        // Replicate what Supervisor::send does.
        let mut line = serde_json::to_string(&env).expect("serialise");
        line.push('\n');

        // Must be exactly one newline, at the end.
        assert_eq!(line.chars().filter(|&c| c == '\n').count(), 1);
        assert!(line.ends_with('\n'));

        // The content before the newline must decode cleanly.
        let trimmed = line.trim_end_matches('\n');
        let decoded = decode_line(trimmed).expect("decode");
        assert_eq!(decoded.id, env.id);
    }
}
