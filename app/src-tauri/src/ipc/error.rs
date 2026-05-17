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

#[derive(Debug, serde::Serialize)]
pub struct SerializableIpcError {
    pub code: String,
    pub message: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub details: Option<serde_json::Value>,
}

impl From<IpcError> for SerializableIpcError {
    fn from(err: IpcError) -> Self {
        match &err {
            IpcError::ProtocolVersionMismatch { .. } => Self {
                code: "PROTOCOL_VERSION_MISMATCH".into(),
                message: err.to_string(),
                details: None,
            },
            IpcError::MalformedEnvelope(_) => Self {
                code: "MALFORMED_ENVELOPE".into(),
                message: err.to_string(),
                details: None,
            },
            IpcError::Serde(_) => Self {
                code: "MALFORMED_ENVELOPE".into(),
                message: err.to_string(),
                details: None,
            },
            IpcError::SidecarUnavailable => Self {
                code: "SIDECAR_UNAVAILABLE".into(),
                message: err.to_string(),
                details: None,
            },
            IpcError::SidecarBusy => Self {
                code: "SIDECAR_BUSY".into(),
                message: err.to_string(),
                details: None,
            },
            IpcError::Timeout => Self {
                code: "TIMEOUT".into(),
                message: err.to_string(),
                details: None,
            },
            IpcError::UnknownMethod(_) => Self {
                code: "UNKNOWN_METHOD".into(),
                message: err.to_string(),
                details: None,
            },
            // `Other` is the only arm that does not use `err.to_string()` for
            // the message: the Display impl prefixes "ipc error {code}: " and
            // drops `details`. We instead forward the raw fields verbatim so
            // sidecar-originated errors round-trip unchanged to the frontend.
            IpcError::Other { code, message, details } => Self {
                code: code.clone(),
                message: message.clone(),
                details: details.clone(),
            },
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn serializable_from_protocol_mismatch() {
        let s: SerializableIpcError = IpcError::ProtocolVersionMismatch { got: 2, want: 1 }.into();
        assert_eq!(s.code, "PROTOCOL_VERSION_MISMATCH");
        assert!(s.message.contains("got 2"));
        assert!(s.details.is_none());
    }

    #[test]
    fn serializable_from_timeout() {
        let s: SerializableIpcError = IpcError::Timeout.into();
        assert_eq!(s.code, "TIMEOUT");
    }

    #[test]
    fn serializable_from_unknown_method() {
        let s: SerializableIpcError = IpcError::UnknownMethod("media.bogus".into()).into();
        assert_eq!(s.code, "UNKNOWN_METHOD");
        assert!(s.message.contains("media.bogus"));
    }

    #[test]
    fn serializable_from_other_preserves_code_message_details() {
        let s: SerializableIpcError = IpcError::Other {
            code: "FILE_NOT_FOUND".into(),
            message: "missing".into(),
            details: Some(serde_json::json!({"path": "/x"})),
        }
        .into();
        assert_eq!(s.code, "FILE_NOT_FOUND");
        assert_eq!(s.message, "missing");
        assert_eq!(s.details.as_ref().unwrap()["path"], "/x");
    }

    #[test]
    fn serializable_round_trips_through_serde_json() {
        let s: SerializableIpcError = IpcError::SidecarUnavailable.into();
        let v = serde_json::to_value(&s).unwrap();
        assert_eq!(v["code"], "SIDECAR_UNAVAILABLE");
        assert!(v["message"].is_string());
        assert!(v.get("details").is_none(), "details omitted when None");
    }
}
