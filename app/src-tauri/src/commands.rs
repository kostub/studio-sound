//! Tauri command handlers for the IPC layer.
//!
//! Each command accesses the shared [`IpcClient`] from Tauri managed state and
//! delegates to [`IpcClient::call`] with the appropriate method name and timeout.
//!
//! # Error mapping
//! Tauri requires that command errors implement `serde::Serialize`.  Rather than
//! adding that bound to [`IpcError`] (which would pull in Serialize for all its
//! variants), these commands map errors to a `String` via `.map_err(|e|
//! e.to_string())`.  The frontend can parse the string to distinguish error
//! kinds if needed; Phase 6 will introduce structured error serialisation.

use std::sync::Arc;
use std::time::Duration;

use tauri::State;

use crate::ipc::client::IpcClient;

/// Per-method timeout table.  All Phase 1 methods use modest timeouts; the
/// fallback catches any future `ipc_call` with an unknown method.
fn default_timeout(method: &str) -> Duration {
    match method {
        "system.ping" => Duration::from_secs(2),
        "system.echo" => Duration::from_secs(5),
        "system.shutdown" => Duration::from_secs(2),
        _ => Duration::from_secs(10),
    }
}

/// Sends a `system.ping` request to the sidecar and returns the result.
///
/// Expected result shape (per schema):
/// ```json
/// { "pong": true, "sidecarVersion": "…", "uptimeMs": 0, "supportedProtocolVersions": [1] }
/// ```
#[tauri::command]
pub async fn ipc_ping(
    client: State<'_, Arc<IpcClient>>,
) -> Result<serde_json::Value, String> {
    client
        .call("system.ping", serde_json::Value::Null, default_timeout("system.ping"))
        .await
        .map_err(|e| e.to_string())
}

/// Sends a `system.echo` request with the given `text` to the sidecar.
///
/// The sidecar echoes back `{ "text": "<input>" }`.
#[tauri::command]
pub async fn ipc_echo(
    text: String,
    client: State<'_, Arc<IpcClient>>,
) -> Result<serde_json::Value, String> {
    let payload = serde_json::json!({ "text": text });
    client
        .call("system.echo", payload, default_timeout("system.echo"))
        .await
        .map_err(|e| e.to_string())
}

/// Sends a `system.shutdown` request to the sidecar.
///
/// The sidecar responds with `{ "accepted": true }` and schedules its own exit.
#[tauri::command]
pub async fn ipc_shutdown(
    client: State<'_, Arc<IpcClient>>,
) -> Result<serde_json::Value, String> {
    client
        .call(
            "system.shutdown",
            serde_json::Value::Null,
            default_timeout("system.shutdown"),
        )
        .await
        .map_err(|e| e.to_string())
}
