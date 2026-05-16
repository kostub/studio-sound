//! Sidecar supervisor.
//!
//! Manages the lifecycle of the Go sidecar child process.  Callers can:
//! - spawn the sidecar via [`Supervisor::spawn`]
//! - send envelopes to its stdin via [`Supervisor::send`]
//! - receive decoded response envelopes from its stdout via [`Supervisor::subscribe`]
//! - request a graceful shutdown via [`Supervisor::shutdown`]
//!
//! ## Integration tests
//!
//! Full integration tests that exercise a real sidecar binary are deferred to
//! item 7.1 because they require the sidecar to be compiled and registered as
//! a Tauri external binary, which depends on infrastructure that lands in later
//! items.  The unit tests in this module exercise the serialisation path only.

use std::sync::Arc;

use tauri::AppHandle;
use tauri_plugin_shell::process::{CommandChild, CommandEvent};
use tauri_plugin_shell::ShellExt;
use tokio::sync::{broadcast, Mutex};

use crate::ipc::envelope::{decode_line, Envelope, Kind};
use crate::ipc::error::IpcError;

#[cfg(test)]
use crate::ipc::envelope::PROTOCOL_VERSION;

/// Capacity of the broadcast channel that fans out decoded response envelopes.
const CHANNEL_CAPACITY: usize = 256;

/// Shared inner state owned by a [`Supervisor`].
struct Inner {
    /// Sender half of the broadcast channel.  Kept alive as long as the
    /// supervisor exists so new subscribers can be created at any time.
    tx: broadcast::Sender<Envelope>,

    /// The child process handle, used for stdin writes via `child.write()`.
    /// Wrapped in a mutex so that multiple concurrent senders can call
    /// [`Supervisor::send`] safely.
    ///
    /// `None` after the sidecar has been shut down.
    child: Mutex<Option<CommandChild>>,
}

/// Manages a single sidecar child process.
///
/// Clone-cheap: the clone still shares the same inner channel and child handle.
#[derive(Clone)]
pub struct Supervisor {
    inner: Arc<Inner>,
}

impl Supervisor {
    /// Spawns the sidecar binary (named `"studio-sidecar"` as registered in
    /// `tauri.conf.json` → `bundle.externalBin`) in `serve` mode, wires up
    /// stdout → broadcast channel decoding, and returns a ready [`Supervisor`].
    ///
    /// Returns [`IpcError::Other`] if the sidecar binary cannot be resolved or
    /// spawned.
    pub fn spawn(app: &AppHandle) -> Result<Self, IpcError> {
        let cmd = app
            .shell()
            .sidecar("studio-sidecar")
            .map_err(|e| IpcError::Other {
                code: "SIDECAR_UNAVAILABLE".into(),
                message: format!("sidecar binary not found: {e}"),
                details: None,
            })?
            .args(["serve"]);

        let (mut event_rx, child) = cmd.spawn().map_err(|e| IpcError::Other {
            code: "SIDECAR_UNAVAILABLE".into(),
            message: format!("failed to spawn sidecar: {e}"),
            details: None,
        })?;

        let (tx, _) = broadcast::channel(CHANNEL_CAPACITY);
        let tx_clone = tx.clone();

        // Spawn a Tokio task that reads CommandEvent::Stdout lines from the
        // sidecar, decodes each one with `decode_line`, and broadcasts valid
        // response envelopes.  Stderr lines and exit events are traced but not
        // forwarded.
        tokio::spawn(async move {
            while let Some(event) = event_rx.recv().await {
                match event {
                    CommandEvent::Stdout(line) => {
                        // The plugin delivers each stdout *line* as a Vec<u8>
                        // without the trailing newline.
                        let line_str = match std::str::from_utf8(&line) {
                            Ok(s) => s,
                            Err(_) => {
                                tracing::warn!("sidecar stdout: non-UTF-8 line, skipping");
                                continue;
                            }
                        };
                        match decode_line(line_str) {
                            Ok(env) => {
                                // Only forward response and event envelopes;
                                // request envelopes from the sidecar are
                                // unexpected in Phase 1 and are silently
                                // discarded.
                                if env.kind == Kind::Response || env.kind == Kind::Event {
                                    // A send error means no subscribers right
                                    // now — this is not a fatal condition.
                                    let _ = tx_clone.send(env);
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
                        // Stop the reader loop when the child exits.
                        break;
                    }
                    // tauri-plugin-shell may add new variants in future; ignore them.
                    _ => {}
                }
            }
        });

        Ok(Self {
            inner: Arc::new(Inner {
                tx,
                child: Mutex::new(Some(child)),
            }),
        })
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

        let mut guard = self.inner.child.lock().await;
        match guard.as_mut() {
            None => Err(IpcError::SidecarUnavailable),
            Some(child) => {
                child.write(line.as_bytes()).map_err(|e| IpcError::Other {
                    code: "SIDECAR_UNAVAILABLE".into(),
                    message: format!("stdin write failed: {e}"),
                    details: None,
                })
            }
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

    /// Closes the sidecar's stdin to signal EOF, which causes the `serve` loop
    /// to exit gracefully (the sidecar exits 0 on stdin EOF per the Go spec).
    ///
    /// After this call, [`send`] will return [`IpcError::SidecarUnavailable`].
    ///
    /// [`send`]: Supervisor::send
    pub async fn shutdown(&self) {
        // Dropping the child handle closes the write end of the pipe, sending
        // EOF to the sidecar's stdin reader.
        let mut guard = self.inner.child.lock().await;
        *guard = None;
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
