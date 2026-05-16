mod commands;
pub mod ipc;
mod logging;

use std::sync::Arc;

use tauri::Manager;

use ipc::client::IpcClient;
use ipc::supervisor::Supervisor;

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .setup(|app| {
            // Initialise tracing first so that supervisor spawn / startup logs
            // land in the on-disk log file. The returned guard must be kept
            // alive for the lifetime of the process; we stash it in managed
            // state.
            let log_guard = logging::init_tracing(app.handle())?;
            app.manage(log_guard);

            // Spawn the sidecar supervisor and wrap it in an IpcClient.
            // The IpcClient is stored as Tauri managed state so that command
            // handlers can access it via `State<'_, Arc<IpcClient>>`.
            let supervisor = Arc::new(Supervisor::spawn(app.handle())?);
            let client = Arc::new(IpcClient::new(supervisor));
            app.manage(client);
            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            commands::ipc_ping,
            commands::ipc_echo,
            commands::ipc_shutdown,
            commands::open_logs_folder,
        ])
        .run(tauri::generate_context!())
        .expect("error while running Studio Sound App");
}
