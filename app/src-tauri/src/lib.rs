mod commands;
pub mod ipc;

use std::sync::Arc;

use tauri::Manager;

use ipc::client::IpcClient;
use ipc::supervisor::Supervisor;

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .setup(|app| {
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
        ])
        .run(tauri::generate_context!())
        .expect("error while running Studio Sound App");
}
