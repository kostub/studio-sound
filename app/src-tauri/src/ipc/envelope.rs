use serde::{Deserialize, Serialize};
use serde_json::Value;

use crate::ipc::error::IpcError;

pub const PROTOCOL_VERSION: u8 = 1;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(rename_all = "lowercase")]
pub enum Kind {
    Request,
    Response,
    Event,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Envelope {
    pub v: u8,
    pub id: String,
    pub kind: Kind,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub method: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub payload: Option<Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub result: Option<Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<RpcError>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RpcError {
    pub code: String,
    pub message: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub details: Option<Value>,
}

/// Decode a single newline-delimited JSON line into an [`Envelope`].
///
/// Returns [`IpcError::MalformedEnvelope`] for invalid JSON and
/// [`IpcError::ProtocolVersionMismatch`] when `v` differs from
/// [`PROTOCOL_VERSION`].
pub fn decode_line(line: &str) -> Result<Envelope, IpcError> {
    let env: Envelope = serde_json::from_str(line)
        .map_err(|e| IpcError::MalformedEnvelope(e.to_string()))?;
    if env.v != PROTOCOL_VERSION {
        return Err(IpcError::ProtocolVersionMismatch {
            got: env.v,
            want: PROTOCOL_VERSION,
        });
    }
    Ok(env)
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    fn make_envelope() -> Envelope {
        Envelope {
            v: PROTOCOL_VERSION,
            id: "01HZZZZ00000000000000000001".to_string(),
            kind: Kind::Request,
            method: Some("system.ping".to_string()),
            payload: None,
            result: None,
            error: None,
        }
    }

    #[test]
    fn round_trip_serialise_and_decode() {
        let env = make_envelope();
        let serialized = serde_json::to_string(&env).expect("serialization failed");
        let decoded = decode_line(&serialized).expect("decode_line failed");

        assert_eq!(decoded.v, PROTOCOL_VERSION);
        assert_eq!(decoded.id, env.id);
        assert_eq!(decoded.kind, env.kind);
        assert_eq!(decoded.method, env.method);
        assert!(decoded.payload.is_none());
        assert!(decoded.result.is_none());
        assert!(decoded.error.is_none());
    }

    #[test]
    fn round_trip_with_result() {
        let env = Envelope {
            v: PROTOCOL_VERSION,
            id: "01HZZZZ00000000000000000002".to_string(),
            kind: Kind::Response,
            method: None,
            payload: None,
            result: Some(json!({"pong": true, "sidecarVersion": "0.1.0"})),
            error: None,
        };
        let serialized = serde_json::to_string(&env).expect("serialization failed");
        // method should be omitted when None
        assert!(!serialized.contains("\"method\""));
        let decoded = decode_line(&serialized).expect("decode_line failed");
        assert_eq!(decoded.kind, Kind::Response);
        let result = decoded.result.expect("result should be present");
        assert_eq!(result["pong"], json!(true));
    }

    #[test]
    fn wrong_version_returns_protocol_version_mismatch() {
        let env = json!({
            "v": 99u8,
            "id": "01HZZZZ00000000000000000003",
            "kind": "request",
            "method": "system.ping"
        });
        let line = serde_json::to_string(&env).unwrap();
        let err = decode_line(&line).expect_err("should fail on wrong version");
        match err {
            IpcError::ProtocolVersionMismatch { got, want } => {
                assert_eq!(got, 99);
                assert_eq!(want, PROTOCOL_VERSION);
            }
            other => panic!("expected ProtocolVersionMismatch, got: {:?}", other),
        }
    }

    #[test]
    fn invalid_json_returns_malformed_envelope() {
        let err = decode_line("{not valid json}").expect_err("should fail on invalid JSON");
        match err {
            IpcError::MalformedEnvelope(_) => {} // expected
            other => panic!("expected MalformedEnvelope, got: {:?}", other),
        }
    }

    #[test]
    fn missing_required_field_returns_malformed_envelope() {
        // Missing "id" which is required
        let line = r#"{"v":1,"kind":"request"}"#;
        let err = decode_line(line).expect_err("should fail on missing required field");
        match err {
            IpcError::MalformedEnvelope(_) => {} // expected
            other => panic!("expected MalformedEnvelope, got: {:?}", other),
        }
    }
}
