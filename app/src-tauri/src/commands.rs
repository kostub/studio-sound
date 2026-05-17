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
use crate::ipc::error::SerializableIpcError;

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
) -> Result<serde_json::Value, SerializableIpcError> {
    client
        .call("system.ping", serde_json::Value::Null, default_timeout("system.ping"))
        .await
        .map_err(SerializableIpcError::from)
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

/// Opens the platform-specific application log directory in the system file manager
/// (Finder on macOS, Explorer on Windows, or the default file manager on Linux).
///
/// The log directory is the Tauri-resolved `app_log_dir` for this application
/// (`~/Library/Logs/com.studiosound.app/` on macOS, `%LOCALAPPDATA%\com.studiosound.app\Logs\` on Windows).
#[tauri::command]
pub async fn open_logs_folder(app: tauri::AppHandle) -> Result<(), String> {
    use tauri::Manager;

    let log_dir = app
        .path()
        .app_log_dir()
        .map_err(|e| format!("failed to resolve log directory: {e}"))?;

    // Create the directory if it doesn't exist so the file manager can open it.
    tokio::fs::create_dir_all(&log_dir)
        .await
        .map_err(|e| format!("failed to create log directory: {e}"))?;

    let path_str = log_dir
        .to_str()
        .ok_or_else(|| "log directory path is not valid UTF-8".to_string())?;

    #[cfg(target_os = "macos")]
    {
        tokio::process::Command::new("open")
            .arg(path_str)
            .spawn()
            .map_err(|e| format!("failed to open log directory: {e}"))?;
    }

    #[cfg(target_os = "windows")]
    {
        tokio::process::Command::new("explorer")
            .arg(path_str)
            .spawn()
            .map_err(|e| format!("failed to open log directory: {e}"))?;
    }

    #[cfg(target_os = "linux")]
    {
        tokio::process::Command::new("xdg-open")
            .arg(path_str)
            .spawn()
            .map_err(|e| format!("failed to open log directory: {e}"))?;
    }

    Ok(())
}
