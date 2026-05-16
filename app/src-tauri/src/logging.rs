//! Tauri-side tracing initialisation.
//!
//! Installs a `tracing-subscriber` that writes JSON-formatted records to
//! `<app_log_dir>/tauri.log` via `tracing-appender` (non-blocking, daily
//! rotation). The returned `WorkerGuard` must be kept alive for the lifetime
//! of the process — drop it and the buffered writer thread shuts down,
//! truncating pending log records.
//!
//! Log filtering: respects `RUST_LOG` if set, otherwise defaults to `info`.

use std::fs;

use tauri::{AppHandle, Manager};
use tracing_appender::non_blocking::WorkerGuard;
use tracing_subscriber::{fmt, prelude::*, EnvFilter};

/// Background flusher guard. Stored as Tauri managed state so it lives for the
/// process lifetime; dropping it flushes and stops the writer thread.
pub struct LogGuard(#[allow(dead_code)] WorkerGuard);

/// Resolves `<app_log_dir>`, creates it if missing, and installs a global
/// `tracing` subscriber that writes JSON records there with daily rotation.
pub fn init_tracing(app: &AppHandle) -> Result<LogGuard, Box<dyn std::error::Error>> {
    let log_dir = app.path().app_log_dir()?;
    fs::create_dir_all(&log_dir)?;

    let file_appender = tracing_appender::rolling::daily(&log_dir, "tauri.log");
    let (non_blocking, guard) = tracing_appender::non_blocking(file_appender);

    let filter = EnvFilter::try_from_default_env().unwrap_or_else(|_| EnvFilter::new("info"));

    tracing_subscriber::registry()
        .with(filter)
        .with(fmt::layer().json().with_writer(non_blocking))
        .try_init()
        .map_err(|e| format!("install tracing subscriber: {e}"))?;

    tracing::info!(
        log_dir = %log_dir.display(),
        "tauri tracing initialised"
    );

    Ok(LogGuard(guard))
}
