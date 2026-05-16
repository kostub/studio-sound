use thiserror::Error;

#[derive(Debug, Error)]
pub enum IpcError {
    #[error("protocol version mismatch: got {got}, want {want}")]
    ProtocolVersionMismatch { got: u8, want: u8 },
    #[error("malformed envelope: {0}")]
    MalformedEnvelope(String),
    #[error("serde error: {0}")]
    Serde(#[from] serde_json::Error),
    #[error("sidecar unavailable")]
    SidecarUnavailable,
    #[error("sidecar busy")]
    SidecarBusy,
    #[error("request timed out")]
    Timeout,
    #[error("unknown method: {0}")]
    UnknownMethod(String),
    #[error("ipc error {code}: {message}")]
    Other {
        code: String,
        message: String,
        details: Option<serde_json::Value>,
    },
}
