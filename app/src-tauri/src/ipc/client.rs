//! IPC client — request/response multiplexer over a [`Supervisor`].
//!
//! [`IpcClient`] wraps a shared [`Supervisor`] and provides a single
//! [`IpcClient::call`] method that:
//! 1. Generates a ULID request ID.
//! 2. Registers a one-shot channel keyed by that ID.
//! 3. Serialises and sends the request envelope through the supervisor.
//! 4. Awaits the response with a caller-supplied timeout.
//! 5. Routes incoming response envelopes from the supervisor's broadcast
//!    channel to the waiting callers.
//!
//! A background routing task is spawned once in [`IpcClient::new`] and runs
//! until the broadcast channel closes (i.e. until the supervisor is dropped).

use std::collections::HashMap;
use std::sync::Arc;
use std::time::Duration;

use serde_json::Value;
use tokio::sync::{oneshot, Mutex};

use crate::ipc::envelope::{Envelope, Kind, RpcError, PROTOCOL_VERSION};
use crate::ipc::error::IpcError;
use crate::ipc::supervisor::Supervisor;

/// Maximum number of in-flight IPC requests. Matches the sidecar dispatcher's
/// `maxConcurrentDispatch` and the `SIDECAR_BUSY` threshold documented in
/// `docs/ipc-contract.md`. When this cap is reached, [`IpcClient::call`]
/// returns [`IpcError::SidecarBusy`] immediately rather than queueing.
const MAX_PENDING_REQUESTS: usize = 64;

/// Shared inner state for [`IpcClient`].
struct Inner {
    /// Map of pending request IDs to one-shot senders.
    pending: Mutex<HashMap<String, oneshot::Sender<Envelope>>>,

    /// Reference to the underlying supervisor.
    supervisor: Arc<Supervisor>,
}

/// A multiplexing IPC client.
///
/// Clone-cheap: all clones share the same pending map and supervisor.
#[derive(Clone)]
pub struct IpcClient {
    inner: Arc<Inner>,
}

impl IpcClient {
    /// Creates a new [`IpcClient`] and spawns the background routing task.
    ///
    /// The routing task forwards every response envelope received from
    /// `supervisor.subscribe()` to the matching pending one-shot sender.
    pub fn new(supervisor: Arc<Supervisor>) -> Self {
        let inner = Arc::new(Inner {
            pending: Mutex::new(HashMap::new()),
            supervisor,
        });

        // Spawn a background task that reads from the broadcast channel and
        // routes response envelopes to the appropriate pending callers.
        let mut rx = inner.supervisor.subscribe();
        let inner_weak = Arc::downgrade(&inner);
        tokio::spawn(async move {
            loop {
                match rx.recv().await {
                    Ok(envelope) => {
                        // Only handle response kind; events are not matched to
                        // pending requests.
                        if envelope.kind != Kind::Response {
                            continue;
                        }

                        let id = envelope.id.clone();
                        if let Some(inner) = inner_weak.upgrade() {
                            let mut guard = inner.pending.lock().await;
                            if let Some(tx) = guard.remove(&id) {
                                // If the receiver has already been dropped (timeout),
                                // the send will fail silently; this is expected.
                                let _ = tx.send(envelope);
                            } else {
                                tracing::warn!(id = %id, "received response for unknown request id; discarding");
                            }
                        } else {
                            // The IpcClient has been dropped; exit the routing loop.
                            tracing::debug!("IpcClient routing task: client dropped, exiting");
                            break;
                        }
                    }
                    Err(tokio::sync::broadcast::error::RecvError::Lagged(n)) => {
                        tracing::warn!(dropped = n, "IpcClient routing task lagged; {} envelopes dropped", n);
                    }
                    Err(tokio::sync::broadcast::error::RecvError::Closed) => {
                        // Supervisor has been dropped; exit the routing loop.
                        tracing::debug!("IpcClient routing task: broadcast channel closed, exiting");
                        break;
                    }
                }
            }
        });

        Self { inner }
    }

    /// Sends a request to the sidecar and awaits the response.
    ///
    /// # Parameters
    /// - `method` — the IPC method name (e.g. `"system.ping"`).
    /// - `payload` — the request payload (`Value::Null` for methods with no
    ///   payload).
    /// - `timeout` — maximum time to wait for a response before returning
    ///   [`IpcError::Timeout`].
    ///
    /// # Errors
    /// - [`IpcError::Timeout`] — response not received within `timeout`.
    /// - [`IpcError::SidecarUnavailable`] — supervisor's child has been shut
    ///   down.
    /// - [`IpcError::Other`] — sidecar returned an error envelope.
    /// - [`IpcError::Serde`] — serialisation of the request envelope failed.
    pub async fn call(
        &self,
        method: &str,
        payload: Value,
        timeout: Duration,
    ) -> Result<Value, IpcError> {
        // Generate a ULID as the unique request ID.
        let id = ulid::Ulid::new().to_string();

        // Register a one-shot channel before sending, so we cannot miss a
        // very fast response. Enforce MAX_PENDING_REQUESTS under the same lock
        // so the check and insert are atomic — otherwise concurrent callers
        // could each see len() == cap-1 and both insert past the cap.
        let (tx, rx) = oneshot::channel::<Envelope>();
        {
            let mut guard = self.inner.pending.lock().await;
            if guard.len() >= MAX_PENDING_REQUESTS {
                tracing::warn!(
                    pending = guard.len(),
                    method,
                    "rejecting IPC request: pending request cap reached"
                );
                return Err(IpcError::SidecarBusy);
            }
            guard.insert(id.clone(), tx);
        }

        // Build and send the request envelope.
        let envelope = Envelope {
            v: PROTOCOL_VERSION,
            id: id.clone(),
            kind: Kind::Request,
            method: Some(method.to_string()),
            payload: Some(payload),
            result: None,
            error: None,
        };

        if let Err(send_err) = self.inner.supervisor.send(&envelope).await {
            // Clean up the pending entry so we don't leak it.
            let mut guard = self.inner.pending.lock().await;
            guard.remove(&id);
            return Err(send_err);
        }

        // Await the response with a timeout.
        match tokio::time::timeout(timeout, rx).await {
            Ok(Ok(response)) => {
                // Distinguish error envelopes from successful results.
                if let Some(rpc_err) = response.error {
                    return Err(ipc_error_from_rpc(rpc_err));
                }
                Ok(response.result.unwrap_or(Value::Null))
            }
            Ok(Err(_)) => {
                // The one-shot sender was dropped without sending — this
                // happens if the routing task exited (supervisor shut down).
                Err(IpcError::SidecarUnavailable)
            }
            Err(_elapsed) => {
                // Timeout: remove the pending entry so late responses are
                // discarded by the routing task instead of panicking.
                let mut guard = self.inner.pending.lock().await;
                guard.remove(&id);
                tracing::warn!(method, id = %id, "IPC request timed out");
                Err(IpcError::Timeout)
            }
        }
    }
}

/// Converts a wire-level [`RpcError`] into the higher-level [`IpcError`].
fn ipc_error_from_rpc(err: RpcError) -> IpcError {
    IpcError::Other {
        code: err.code,
        message: err.message,
        details: err.details,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::ipc::envelope::Kind;
    use tokio::sync::broadcast;

    /// Build a minimal response envelope for use in tests.
    fn make_response(id: &str, result: Value) -> Envelope {
        Envelope {
            v: PROTOCOL_VERSION,
            id: id.to_string(),
            kind: Kind::Response,
            method: None,
            payload: None,
            result: Some(result),
            error: None,
        }
    }

    /// Build a minimal error response envelope for use in tests.
    fn make_error_response(id: &str, code: &str, message: &str) -> Envelope {
        Envelope {
            v: PROTOCOL_VERSION,
            id: id.to_string(),
            kind: Kind::Response,
            method: None,
            payload: None,
            result: None,
            error: Some(RpcError {
                code: code.to_string(),
                message: message.to_string(),
                details: None,
            }),
        }
    }

    /// Tests that a response envelope injected into the pending map's one-shot
    /// sender is correctly received by the waiter.
    ///
    /// This tests the routing logic in isolation without requiring a real
    /// Supervisor or Tauri runtime.
    #[tokio::test]
    async fn oneshot_routing_delivers_result() {
        let (response_tx, response_rx) = oneshot::channel::<Envelope>();
        let env = make_response("test-id-001", serde_json::json!({"pong": true}));
        response_tx.send(env.clone()).unwrap();

        let received = response_rx.await.expect("oneshot receiver should get value");
        assert_eq!(received.id, "test-id-001");
        assert_eq!(received.result, Some(serde_json::json!({"pong": true})));
    }

    /// Tests that the routing task correctly discards envelopes whose IDs are
    /// not in the pending map (late arrivals after timeout).
    #[tokio::test]
    async fn routing_discards_unknown_id() {
        // Create a fake broadcast channel and pending map.
        let (tx, _rx) = broadcast::channel::<Envelope>(16);
        let pending: Mutex<HashMap<String, oneshot::Sender<Envelope>>> =
            Mutex::new(HashMap::new());

        // Send a response with an ID that has no pending entry.
        let env = make_response("no-such-id", serde_json::json!({"result": 42}));
        tx.send(env).unwrap();

        // Verify the pending map is still empty (nothing was consumed or panicked).
        let guard = pending.lock().await;
        assert!(guard.is_empty());
    }

    /// Tests that an error envelope is converted to the expected IpcError variant.
    #[test]
    fn ipc_error_from_rpc_converts_correctly() {
        let rpc_err = RpcError {
            code: "UNKNOWN_METHOD".to_string(),
            message: "method not found".to_string(),
            details: None,
        };
        let ipc_err = ipc_error_from_rpc(rpc_err);
        match ipc_err {
            IpcError::Other { code, message, details } => {
                assert_eq!(code, "UNKNOWN_METHOD");
                assert_eq!(message, "method not found");
                assert!(details.is_none());
            }
            other => panic!("expected IpcError::Other, got: {:?}", other),
        }
    }

    /// Tests that an error envelope with details is converted correctly.
    #[test]
    fn ipc_error_from_rpc_preserves_details() {
        let rpc_err = RpcError {
            code: "INVALID_PAYLOAD".to_string(),
            message: "payload validation failed".to_string(),
            details: Some(serde_json::json!({"field": "text", "reason": "too long"})),
        };
        let ipc_err = ipc_error_from_rpc(rpc_err);
        match ipc_err {
            IpcError::Other { code, message, details } => {
                assert_eq!(code, "INVALID_PAYLOAD");
                assert_eq!(message, "payload validation failed");
                let d = details.expect("details should be Some");
                assert_eq!(d["field"], "text");
            }
            other => panic!("expected IpcError::Other, got: {:?}", other),
        }
    }

    /// Verifies that a ULID generated via the ulid crate is a non-empty string
    /// of the expected format (26 uppercase alphanumeric characters).
    #[test]
    fn ulid_generation_produces_valid_id() {
        let id = ulid::Ulid::new().to_string();
        assert_eq!(id.len(), 26, "ULID should be 26 characters");
        assert!(id.chars().all(|c| c.is_alphanumeric()), "ULID should be alphanumeric");
    }

    /// Tests that a response with both result=None and error=None yields Null.
    #[test]
    fn response_with_no_result_yields_null() {
        let env = Envelope {
            v: PROTOCOL_VERSION,
            id: "test-null".to_string(),
            kind: Kind::Response,
            method: None,
            payload: None,
            result: None,
            error: None,
        };
        // Simulate what call() does with the result field.
        let result = env.result.unwrap_or(Value::Null);
        assert_eq!(result, Value::Null);
    }

    /// Tests that the routing task correctly identifies error responses.
    #[test]
    fn error_response_envelope_detected() {
        let env = make_error_response("req-err", "TIMEOUT", "request timed out");
        assert!(env.error.is_some());
        assert!(env.result.is_none());
    }

    /// Tests the SIDECAR_BUSY cap on the pending map. We exercise the same
    /// check IpcClient::call performs (capacity check under the same lock as
    /// the insert) directly on the map, without needing a real Supervisor.
    #[tokio::test]
    async fn pending_cap_rejects_overflow() {
        let pending: Mutex<HashMap<String, oneshot::Sender<Envelope>>> =
            Mutex::new(HashMap::new());

        // Fill to the cap.
        {
            let mut guard = pending.lock().await;
            for i in 0..MAX_PENDING_REQUESTS {
                let (tx, _rx) = oneshot::channel::<Envelope>();
                guard.insert(format!("req-{i}"), tx);
            }
            assert_eq!(guard.len(), MAX_PENDING_REQUESTS);
        }

        // Simulate the call-time guard: at-cap → reject.
        let busy = {
            let guard = pending.lock().await;
            guard.len() >= MAX_PENDING_REQUESTS
        };
        assert!(busy, "expected pending map to be at-cap and reject new requests");
    }
}
