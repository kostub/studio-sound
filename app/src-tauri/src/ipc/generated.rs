#![allow(clippy::redundant_closure_call)]
#![allow(clippy::needless_lifetimes)]
#![allow(clippy::match_single_binding)]
#![allow(clippy::clone_on_copy)]

#[doc = r" Error types."]
pub mod error {
    #[doc = r" Error from a `TryFrom` or `FromStr` implementation."]
    pub struct ConversionError(::std::borrow::Cow<'static, str>);
    impl ::std::error::Error for ConversionError {}
    impl ::std::fmt::Display for ConversionError {
        fn fmt(&self, f: &mut ::std::fmt::Formatter<'_>) -> Result<(), ::std::fmt::Error> {
            ::std::fmt::Display::fmt(&self.0, f)
        }
    }
    impl ::std::fmt::Debug for ConversionError {
        fn fmt(&self, f: &mut ::std::fmt::Formatter<'_>) -> Result<(), ::std::fmt::Error> {
            ::std::fmt::Debug::fmt(&self.0, f)
        }
    }
    impl From<&'static str> for ConversionError {
        fn from(value: &'static str) -> Self {
            Self(value.into())
        }
    }
    impl From<String> for ConversionError {
        fn from(value: String) -> Self {
            Self(value.into())
        }
    }
}
#[doc = "`Envelope`"]
#[doc = r""]
#[doc = r" <details><summary>JSON schema</summary>"]
#[doc = r""]
#[doc = r" ```json"]
#[doc = "{"]
#[doc = "  \"$id\": \"https://studiosound.app/schemas/envelope.schema.json\","]
#[doc = "  \"title\": \"Envelope\","]
#[doc = "  \"type\": \"object\","]
#[doc = "  \"required\": ["]
#[doc = "    \"id\","]
#[doc = "    \"kind\","]
#[doc = "    \"v\""]
#[doc = "  ],"]
#[doc = "  \"properties\": {"]
#[doc = "    \"error\": {"]
#[doc = "      \"type\": \"object\","]
#[doc = "      \"required\": ["]
#[doc = "        \"code\","]
#[doc = "        \"message\""]
#[doc = "      ],"]
#[doc = "      \"properties\": {"]
#[doc = "        \"code\": {"]
#[doc = "          \"type\": \"string\","]
#[doc = "          \"pattern\": \"^[A-Z][A-Z0-9_]*$\""]
#[doc = "        },"]
#[doc = "        \"details\": {"]
#[doc = "          \"type\": ["]
#[doc = "            \"object\","]
#[doc = "            \"null\""]
#[doc = "          ]"]
#[doc = "        },"]
#[doc = "        \"message\": {"]
#[doc = "          \"type\": \"string\","]
#[doc = "          \"maxLength\": 500"]
#[doc = "        }"]
#[doc = "      },"]
#[doc = "      \"additionalProperties\": false"]
#[doc = "    },"]
#[doc = "    \"id\": {"]
#[doc = "      \"type\": \"string\","]
#[doc = "      \"maxLength\": 64,"]
#[doc = "      \"minLength\": 1"]
#[doc = "    },"]
#[doc = "    \"kind\": {"]
#[doc = "      \"type\": \"string\","]
#[doc = "      \"enum\": ["]
#[doc = "        \"request\","]
#[doc = "        \"response\","]
#[doc = "        \"event\""]
#[doc = "      ]"]
#[doc = "    },"]
#[doc = "    \"method\": {"]
#[doc = "      \"type\": \"string\","]
#[doc = "      \"pattern\": \"^[a-z][a-z0-9]*(\\\\.[a-z][a-z0-9]*)+$\""]
#[doc = "    },"]
#[doc = "    \"payload\": {"]
#[doc = "      \"type\": ["]
#[doc = "        \"object\","]
#[doc = "        \"null\""]
#[doc = "      ]"]
#[doc = "    },"]
#[doc = "    \"result\": {"]
#[doc = "      \"type\": ["]
#[doc = "        \"object\","]
#[doc = "        \"null\""]
#[doc = "      ]"]
#[doc = "    },"]
#[doc = "    \"v\": {"]
#[doc = "      \"type\": \"integer\","]
#[doc = "      \"const\": 1"]
#[doc = "    }"]
#[doc = "  },"]
#[doc = "  \"additionalProperties\": false"]
#[doc = "}"]
#[doc = r" ```"]
#[doc = r" </details>"]
#[derive(:: serde :: Deserialize, :: serde :: Serialize, Clone, Debug)]
#[serde(deny_unknown_fields)]
pub struct Envelope {
    #[serde(default, skip_serializing_if = "::std::option::Option::is_none")]
    pub error: ::std::option::Option<EnvelopeError>,
    pub id: EnvelopeId,
    pub kind: EnvelopeKind,
    #[serde(default, skip_serializing_if = "::std::option::Option::is_none")]
    pub method: ::std::option::Option<EnvelopeMethod>,
    #[serde(default, skip_serializing_if = "::std::option::Option::is_none")]
    pub payload:
        ::std::option::Option<::serde_json::Map<::std::string::String, ::serde_json::Value>>,
    #[serde(default, skip_serializing_if = "::std::option::Option::is_none")]
    pub result:
        ::std::option::Option<::serde_json::Map<::std::string::String, ::serde_json::Value>>,
    pub v: i64,
}
impl Envelope {
    pub fn builder() -> builder::Envelope {
        Default::default()
    }
}
#[doc = "`EnvelopeError`"]
#[doc = r""]
#[doc = r" <details><summary>JSON schema</summary>"]
#[doc = r""]
#[doc = r" ```json"]
#[doc = "{"]
#[doc = "  \"type\": \"object\","]
#[doc = "  \"required\": ["]
#[doc = "    \"code\","]
#[doc = "    \"message\""]
#[doc = "  ],"]
#[doc = "  \"properties\": {"]
#[doc = "    \"code\": {"]
#[doc = "      \"type\": \"string\","]
#[doc = "      \"pattern\": \"^[A-Z][A-Z0-9_]*$\""]
#[doc = "    },"]
#[doc = "    \"details\": {"]
#[doc = "      \"type\": ["]
#[doc = "        \"object\","]
#[doc = "        \"null\""]
#[doc = "      ]"]
#[doc = "    },"]
#[doc = "    \"message\": {"]
#[doc = "      \"type\": \"string\","]
#[doc = "      \"maxLength\": 500"]
#[doc = "    }"]
#[doc = "  },"]
#[doc = "  \"additionalProperties\": false"]
#[doc = "}"]
#[doc = r" ```"]
#[doc = r" </details>"]
#[derive(:: serde :: Deserialize, :: serde :: Serialize, Clone, Debug)]
#[serde(deny_unknown_fields)]
pub struct EnvelopeError {
    pub code: EnvelopeErrorCode,
    #[serde(default, skip_serializing_if = "::std::option::Option::is_none")]
    pub details:
        ::std::option::Option<::serde_json::Map<::std::string::String, ::serde_json::Value>>,
    pub message: EnvelopeErrorMessage,
}
impl EnvelopeError {
    pub fn builder() -> builder::EnvelopeError {
        Default::default()
    }
}
#[doc = "`EnvelopeErrorCode`"]
#[doc = r""]
#[doc = r" <details><summary>JSON schema</summary>"]
#[doc = r""]
#[doc = r" ```json"]
#[doc = "{"]
#[doc = "  \"type\": \"string\","]
#[doc = "  \"pattern\": \"^[A-Z][A-Z0-9_]*$\""]
#[doc = "}"]
#[doc = r" ```"]
#[doc = r" </details>"]
#[derive(:: serde :: Serialize, Clone, Debug, Eq, Hash, Ord, PartialEq, PartialOrd)]
#[serde(transparent)]
pub struct EnvelopeErrorCode(::std::string::String);
impl ::std::ops::Deref for EnvelopeErrorCode {
    type Target = ::std::string::String;
    fn deref(&self) -> &::std::string::String {
        &self.0
    }
}
impl ::std::convert::From<EnvelopeErrorCode> for ::std::string::String {
    fn from(value: EnvelopeErrorCode) -> Self {
        value.0
    }
}
impl ::std::str::FromStr for EnvelopeErrorCode {
    type Err = self::error::ConversionError;
    fn from_str(value: &str) -> ::std::result::Result<Self, self::error::ConversionError> {
        static PATTERN: ::std::sync::LazyLock<::regress::Regex> =
            ::std::sync::LazyLock::new(|| ::regress::Regex::new("^[A-Z][A-Z0-9_]*$").unwrap());
        if PATTERN.find(value).is_none() {
            return Err("doesn't match pattern \"^[A-Z][A-Z0-9_]*$\"".into());
        }
        Ok(Self(value.to_string()))
    }
}
impl ::std::convert::TryFrom<&str> for EnvelopeErrorCode {
    type Error = self::error::ConversionError;
    fn try_from(value: &str) -> ::std::result::Result<Self, self::error::ConversionError> {
        value.parse()
    }
}
impl ::std::convert::TryFrom<&::std::string::String> for EnvelopeErrorCode {
    type Error = self::error::ConversionError;
    fn try_from(
        value: &::std::string::String,
    ) -> ::std::result::Result<Self, self::error::ConversionError> {
        value.parse()
    }
}
impl ::std::convert::TryFrom<::std::string::String> for EnvelopeErrorCode {
    type Error = self::error::ConversionError;
    fn try_from(
        value: ::std::string::String,
    ) -> ::std::result::Result<Self, self::error::ConversionError> {
        value.parse()
    }
}
impl<'de> ::serde::Deserialize<'de> for EnvelopeErrorCode {
    fn deserialize<D>(deserializer: D) -> ::std::result::Result<Self, D::Error>
    where
        D: ::serde::Deserializer<'de>,
    {
        ::std::string::String::deserialize(deserializer)?
            .parse()
            .map_err(|e: self::error::ConversionError| {
                <D::Error as ::serde::de::Error>::custom(e.to_string())
            })
    }
}
#[doc = "`EnvelopeErrorMessage`"]
#[doc = r""]
#[doc = r" <details><summary>JSON schema</summary>"]
#[doc = r""]
#[doc = r" ```json"]
#[doc = "{"]
#[doc = "  \"type\": \"string\","]
#[doc = "  \"maxLength\": 500"]
#[doc = "}"]
#[doc = r" ```"]
#[doc = r" </details>"]
#[derive(:: serde :: Serialize, Clone, Debug, Eq, Hash, Ord, PartialEq, PartialOrd)]
#[serde(transparent)]
pub struct EnvelopeErrorMessage(::std::string::String);
impl ::std::ops::Deref for EnvelopeErrorMessage {
    type Target = ::std::string::String;
    fn deref(&self) -> &::std::string::String {
        &self.0
    }
}
impl ::std::convert::From<EnvelopeErrorMessage> for ::std::string::String {
    fn from(value: EnvelopeErrorMessage) -> Self {
        value.0
    }
}
impl ::std::str::FromStr for EnvelopeErrorMessage {
    type Err = self::error::ConversionError;
    fn from_str(value: &str) -> ::std::result::Result<Self, self::error::ConversionError> {
        if value.chars().count() > 500usize {
            return Err("longer than 500 characters".into());
        }
        Ok(Self(value.to_string()))
    }
}
impl ::std::convert::TryFrom<&str> for EnvelopeErrorMessage {
    type Error = self::error::ConversionError;
    fn try_from(value: &str) -> ::std::result::Result<Self, self::error::ConversionError> {
        value.parse()
    }
}
impl ::std::convert::TryFrom<&::std::string::String> for EnvelopeErrorMessage {
    type Error = self::error::ConversionError;
    fn try_from(
        value: &::std::string::String,
    ) -> ::std::result::Result<Self, self::error::ConversionError> {
        value.parse()
    }
}
impl ::std::convert::TryFrom<::std::string::String> for EnvelopeErrorMessage {
    type Error = self::error::ConversionError;
    fn try_from(
        value: ::std::string::String,
    ) -> ::std::result::Result<Self, self::error::ConversionError> {
        value.parse()
    }
}
impl<'de> ::serde::Deserialize<'de> for EnvelopeErrorMessage {
    fn deserialize<D>(deserializer: D) -> ::std::result::Result<Self, D::Error>
    where
        D: ::serde::Deserializer<'de>,
    {
        ::std::string::String::deserialize(deserializer)?
            .parse()
            .map_err(|e: self::error::ConversionError| {
                <D::Error as ::serde::de::Error>::custom(e.to_string())
            })
    }
}
#[doc = "`EnvelopeId`"]
#[doc = r""]
#[doc = r" <details><summary>JSON schema</summary>"]
#[doc = r""]
#[doc = r" ```json"]
#[doc = "{"]
#[doc = "  \"type\": \"string\","]
#[doc = "  \"maxLength\": 64,"]
#[doc = "  \"minLength\": 1"]
#[doc = "}"]
#[doc = r" ```"]
#[doc = r" </details>"]
#[derive(:: serde :: Serialize, Clone, Debug, Eq, Hash, Ord, PartialEq, PartialOrd)]
#[serde(transparent)]
pub struct EnvelopeId(::std::string::String);
impl ::std::ops::Deref for EnvelopeId {
    type Target = ::std::string::String;
    fn deref(&self) -> &::std::string::String {
        &self.0
    }
}
impl ::std::convert::From<EnvelopeId> for ::std::string::String {
    fn from(value: EnvelopeId) -> Self {
        value.0
    }
}
impl ::std::str::FromStr for EnvelopeId {
    type Err = self::error::ConversionError;
    fn from_str(value: &str) -> ::std::result::Result<Self, self::error::ConversionError> {
        if value.chars().count() > 64usize {
            return Err("longer than 64 characters".into());
        }
        if value.chars().count() < 1usize {
            return Err("shorter than 1 characters".into());
        }
        Ok(Self(value.to_string()))
    }
}
impl ::std::convert::TryFrom<&str> for EnvelopeId {
    type Error = self::error::ConversionError;
    fn try_from(value: &str) -> ::std::result::Result<Self, self::error::ConversionError> {
        value.parse()
    }
}
impl ::std::convert::TryFrom<&::std::string::String> for EnvelopeId {
    type Error = self::error::ConversionError;
    fn try_from(
        value: &::std::string::String,
    ) -> ::std::result::Result<Self, self::error::ConversionError> {
        value.parse()
    }
}
impl ::std::convert::TryFrom<::std::string::String> for EnvelopeId {
    type Error = self::error::ConversionError;
    fn try_from(
        value: ::std::string::String,
    ) -> ::std::result::Result<Self, self::error::ConversionError> {
        value.parse()
    }
}
impl<'de> ::serde::Deserialize<'de> for EnvelopeId {
    fn deserialize<D>(deserializer: D) -> ::std::result::Result<Self, D::Error>
    where
        D: ::serde::Deserializer<'de>,
    {
        ::std::string::String::deserialize(deserializer)?
            .parse()
            .map_err(|e: self::error::ConversionError| {
                <D::Error as ::serde::de::Error>::custom(e.to_string())
            })
    }
}
#[doc = "`EnvelopeKind`"]
#[doc = r""]
#[doc = r" <details><summary>JSON schema</summary>"]
#[doc = r""]
#[doc = r" ```json"]
#[doc = "{"]
#[doc = "  \"type\": \"string\","]
#[doc = "  \"enum\": ["]
#[doc = "    \"request\","]
#[doc = "    \"response\","]
#[doc = "    \"event\""]
#[doc = "  ]"]
#[doc = "}"]
#[doc = r" ```"]
#[doc = r" </details>"]
#[derive(
    :: serde :: Deserialize,
    :: serde :: Serialize,
    Clone,
    Copy,
    Debug,
    Eq,
    Hash,
    Ord,
    PartialEq,
    PartialOrd,
)]
pub enum EnvelopeKind {
    #[serde(rename = "request")]
    Request,
    #[serde(rename = "response")]
    Response,
    #[serde(rename = "event")]
    Event,
}
impl ::std::fmt::Display for EnvelopeKind {
    fn fmt(&self, f: &mut ::std::fmt::Formatter<'_>) -> ::std::fmt::Result {
        match *self {
            Self::Request => f.write_str("request"),
            Self::Response => f.write_str("response"),
            Self::Event => f.write_str("event"),
        }
    }
}
impl ::std::str::FromStr for EnvelopeKind {
    type Err = self::error::ConversionError;
    fn from_str(value: &str) -> ::std::result::Result<Self, self::error::ConversionError> {
        match value {
            "request" => Ok(Self::Request),
            "response" => Ok(Self::Response),
            "event" => Ok(Self::Event),
            _ => Err("invalid value".into()),
        }
    }
}
impl ::std::convert::TryFrom<&str> for EnvelopeKind {
    type Error = self::error::ConversionError;
    fn try_from(value: &str) -> ::std::result::Result<Self, self::error::ConversionError> {
        value.parse()
    }
}
impl ::std::convert::TryFrom<&::std::string::String> for EnvelopeKind {
    type Error = self::error::ConversionError;
    fn try_from(
        value: &::std::string::String,
    ) -> ::std::result::Result<Self, self::error::ConversionError> {
        value.parse()
    }
}
impl ::std::convert::TryFrom<::std::string::String> for EnvelopeKind {
    type Error = self::error::ConversionError;
    fn try_from(
        value: ::std::string::String,
    ) -> ::std::result::Result<Self, self::error::ConversionError> {
        value.parse()
    }
}
#[doc = "`EnvelopeMethod`"]
#[doc = r""]
#[doc = r" <details><summary>JSON schema</summary>"]
#[doc = r""]
#[doc = r" ```json"]
#[doc = "{"]
#[doc = "  \"type\": \"string\","]
#[doc = "  \"pattern\": \"^[a-z][a-z0-9]*(\\\\.[a-z][a-z0-9]*)+$\""]
#[doc = "}"]
#[doc = r" ```"]
#[doc = r" </details>"]
#[derive(:: serde :: Serialize, Clone, Debug, Eq, Hash, Ord, PartialEq, PartialOrd)]
#[serde(transparent)]
pub struct EnvelopeMethod(::std::string::String);
impl ::std::ops::Deref for EnvelopeMethod {
    type Target = ::std::string::String;
    fn deref(&self) -> &::std::string::String {
        &self.0
    }
}
impl ::std::convert::From<EnvelopeMethod> for ::std::string::String {
    fn from(value: EnvelopeMethod) -> Self {
        value.0
    }
}
impl ::std::str::FromStr for EnvelopeMethod {
    type Err = self::error::ConversionError;
    fn from_str(value: &str) -> ::std::result::Result<Self, self::error::ConversionError> {
        static PATTERN: ::std::sync::LazyLock<::regress::Regex> =
            ::std::sync::LazyLock::new(|| {
                ::regress::Regex::new("^[a-z][a-z0-9]*(\\.[a-z][a-z0-9]*)+$").unwrap()
            });
        if PATTERN.find(value).is_none() {
            return Err("doesn't match pattern \"^[a-z][a-z0-9]*(\\.[a-z][a-z0-9]*)+$\"".into());
        }
        Ok(Self(value.to_string()))
    }
}
impl ::std::convert::TryFrom<&str> for EnvelopeMethod {
    type Error = self::error::ConversionError;
    fn try_from(value: &str) -> ::std::result::Result<Self, self::error::ConversionError> {
        value.parse()
    }
}
impl ::std::convert::TryFrom<&::std::string::String> for EnvelopeMethod {
    type Error = self::error::ConversionError;
    fn try_from(
        value: &::std::string::String,
    ) -> ::std::result::Result<Self, self::error::ConversionError> {
        value.parse()
    }
}
impl ::std::convert::TryFrom<::std::string::String> for EnvelopeMethod {
    type Error = self::error::ConversionError;
    fn try_from(
        value: ::std::string::String,
    ) -> ::std::result::Result<Self, self::error::ConversionError> {
        value.parse()
    }
}
impl<'de> ::serde::Deserialize<'de> for EnvelopeMethod {
    fn deserialize<D>(deserializer: D) -> ::std::result::Result<Self, D::Error>
    where
        D: ::serde::Deserializer<'de>,
    {
        ::std::string::String::deserialize(deserializer)?
            .parse()
            .map_err(|e: self::error::ConversionError| {
                <D::Error as ::serde::de::Error>::custom(e.to_string())
            })
    }
}
#[doc = r" Types for composing complex structures."]
pub mod builder {
    #[derive(Clone, Debug)]
    pub struct Envelope {
        error: ::std::result::Result<
            ::std::option::Option<super::EnvelopeError>,
            ::std::string::String,
        >,
        id: ::std::result::Result<super::EnvelopeId, ::std::string::String>,
        kind: ::std::result::Result<super::EnvelopeKind, ::std::string::String>,
        method: ::std::result::Result<
            ::std::option::Option<super::EnvelopeMethod>,
            ::std::string::String,
        >,
        payload: ::std::result::Result<
            ::std::option::Option<::serde_json::Map<::std::string::String, ::serde_json::Value>>,
            ::std::string::String,
        >,
        result: ::std::result::Result<
            ::std::option::Option<::serde_json::Map<::std::string::String, ::serde_json::Value>>,
            ::std::string::String,
        >,
        v: ::std::result::Result<i64, ::std::string::String>,
    }
    impl ::std::default::Default for Envelope {
        fn default() -> Self {
            Self {
                error: Ok(Default::default()),
                id: Err("no value supplied for id".to_string()),
                kind: Err("no value supplied for kind".to_string()),
                method: Ok(Default::default()),
                payload: Ok(Default::default()),
                result: Ok(Default::default()),
                v: Err("no value supplied for v".to_string()),
            }
        }
    }
    impl Envelope {
        pub fn error<T>(mut self, value: T) -> Self
        where
            T: ::std::convert::TryInto<::std::option::Option<super::EnvelopeError>>,
            T::Error: ::std::fmt::Display,
        {
            self.error = value
                .try_into()
                .map_err(|e| format!("error converting supplied value for error: {e}"));
            self
        }
        pub fn id<T>(mut self, value: T) -> Self
        where
            T: ::std::convert::TryInto<super::EnvelopeId>,
            T::Error: ::std::fmt::Display,
        {
            self.id = value
                .try_into()
                .map_err(|e| format!("error converting supplied value for id: {e}"));
            self
        }
        pub fn kind<T>(mut self, value: T) -> Self
        where
            T: ::std::convert::TryInto<super::EnvelopeKind>,
            T::Error: ::std::fmt::Display,
        {
            self.kind = value
                .try_into()
                .map_err(|e| format!("error converting supplied value for kind: {e}"));
            self
        }
        pub fn method<T>(mut self, value: T) -> Self
        where
            T: ::std::convert::TryInto<::std::option::Option<super::EnvelopeMethod>>,
            T::Error: ::std::fmt::Display,
        {
            self.method = value
                .try_into()
                .map_err(|e| format!("error converting supplied value for method: {e}"));
            self
        }
        pub fn payload<T>(mut self, value: T) -> Self
        where
            T: ::std::convert::TryInto<
                ::std::option::Option<
                    ::serde_json::Map<::std::string::String, ::serde_json::Value>,
                >,
            >,
            T::Error: ::std::fmt::Display,
        {
            self.payload = value
                .try_into()
                .map_err(|e| format!("error converting supplied value for payload: {e}"));
            self
        }
        pub fn result<T>(mut self, value: T) -> Self
        where
            T: ::std::convert::TryInto<
                ::std::option::Option<
                    ::serde_json::Map<::std::string::String, ::serde_json::Value>,
                >,
            >,
            T::Error: ::std::fmt::Display,
        {
            self.result = value
                .try_into()
                .map_err(|e| format!("error converting supplied value for result: {e}"));
            self
        }
        pub fn v<T>(mut self, value: T) -> Self
        where
            T: ::std::convert::TryInto<i64>,
            T::Error: ::std::fmt::Display,
        {
            self.v = value
                .try_into()
                .map_err(|e| format!("error converting supplied value for v: {e}"));
            self
        }
    }
    impl ::std::convert::TryFrom<Envelope> for super::Envelope {
        type Error = super::error::ConversionError;
        fn try_from(value: Envelope) -> ::std::result::Result<Self, super::error::ConversionError> {
            Ok(Self {
                error: value.error?,
                id: value.id?,
                kind: value.kind?,
                method: value.method?,
                payload: value.payload?,
                result: value.result?,
                v: value.v?,
            })
        }
    }
    impl ::std::convert::From<super::Envelope> for Envelope {
        fn from(value: super::Envelope) -> Self {
            Self {
                error: Ok(value.error),
                id: Ok(value.id),
                kind: Ok(value.kind),
                method: Ok(value.method),
                payload: Ok(value.payload),
                result: Ok(value.result),
                v: Ok(value.v),
            }
        }
    }
    #[derive(Clone, Debug)]
    pub struct EnvelopeError {
        code: ::std::result::Result<super::EnvelopeErrorCode, ::std::string::String>,
        details: ::std::result::Result<
            ::std::option::Option<::serde_json::Map<::std::string::String, ::serde_json::Value>>,
            ::std::string::String,
        >,
        message: ::std::result::Result<super::EnvelopeErrorMessage, ::std::string::String>,
    }
    impl ::std::default::Default for EnvelopeError {
        fn default() -> Self {
            Self {
                code: Err("no value supplied for code".to_string()),
                details: Ok(Default::default()),
                message: Err("no value supplied for message".to_string()),
            }
        }
    }
    impl EnvelopeError {
        pub fn code<T>(mut self, value: T) -> Self
        where
            T: ::std::convert::TryInto<super::EnvelopeErrorCode>,
            T::Error: ::std::fmt::Display,
        {
            self.code = value
                .try_into()
                .map_err(|e| format!("error converting supplied value for code: {e}"));
            self
        }
        pub fn details<T>(mut self, value: T) -> Self
        where
            T: ::std::convert::TryInto<
                ::std::option::Option<
                    ::serde_json::Map<::std::string::String, ::serde_json::Value>,
                >,
            >,
            T::Error: ::std::fmt::Display,
        {
            self.details = value
                .try_into()
                .map_err(|e| format!("error converting supplied value for details: {e}"));
            self
        }
        pub fn message<T>(mut self, value: T) -> Self
        where
            T: ::std::convert::TryInto<super::EnvelopeErrorMessage>,
            T::Error: ::std::fmt::Display,
        {
            self.message = value
                .try_into()
                .map_err(|e| format!("error converting supplied value for message: {e}"));
            self
        }
    }
    impl ::std::convert::TryFrom<EnvelopeError> for super::EnvelopeError {
        type Error = super::error::ConversionError;
        fn try_from(
            value: EnvelopeError,
        ) -> ::std::result::Result<Self, super::error::ConversionError> {
            Ok(Self {
                code: value.code?,
                details: value.details?,
                message: value.message?,
            })
        }
    }
    impl ::std::convert::From<super::EnvelopeError> for EnvelopeError {
        fn from(value: super::EnvelopeError) -> Self {
            Self {
                code: Ok(value.code),
                details: Ok(value.details),
                message: Ok(value.message),
            }
        }
    }
}
