use super::SignatureSource;
use std::{collections::HashMap, sync::OnceLock};

#[cfg(test)]
#[path = "precompiles_abi_test.rs"]
mod precompiles_abi_test;

#[derive(Default)]
pub struct PrecompileSource;

#[derive(serde::Deserialize)]
struct EmbeddedPrecompiles {
    abi: serde_json::Value,
}

impl EmbeddedPrecompiles {
    fn into_contract(self) -> ethabi::Contract {
        let abi = serde_json::to_vec(&self.abi).expect("embedded ABI must serialize");
        ethabi::Contract::load(abi.as_slice()).expect("embedded precompile ABI must be valid")
    }
}

struct Signatures {
    functions: HashMap<String, Vec<String>>,
    events: HashMap<String, Vec<String>>,
    complete_events: HashMap<String, Vec<alloy_json_abi::Event>>,
}

impl PrecompileSource {
    pub fn new() -> Self {
        Self
    }

    fn signatures() -> &'static Signatures {
        static SIGNATURES: OnceLock<Signatures> = OnceLock::new();
        SIGNATURES.get_or_init(|| {
            let embedded: EmbeddedPrecompiles =
                serde_json::from_str(include_str!("precompiles.json"))
                    .expect("embedded precompile ABI must be valid");
            let abi = embedded.into_contract();
            let mut functions: HashMap<String, Vec<String>> = HashMap::new();
            let mut events: HashMap<String, Vec<String>> = HashMap::new();
            let mut complete_events: HashMap<String, Vec<alloy_json_abi::Event>> = HashMap::new();
            for function in abi.functions() {
                let signature = format!(
                    "{}({})",
                    function.name,
                    function
                        .inputs
                        .iter()
                        .map(|input| input.kind.to_string())
                        .collect::<Vec<_>>()
                        .join(",")
                );
                let signatures = functions
                    .entry(hex::encode(function.short_signature()))
                    .or_default();
                if !signatures.contains(&signature) {
                    signatures.push(signature);
                }
            }
            for event in abi.events() {
                let mut description =
                    serde_json::to_value(event).expect("embedded event must serialize");
                description["type"] = "event".into();
                let description = description.to_string();
                let description = serde_json::from_str(&description)
                    .expect("embedded event must have a valid complete ABI");
                let descriptions = complete_events
                    .entry(hex::encode(event.signature()))
                    .or_default();
                if !descriptions.contains(&description) {
                    descriptions.push(description);
                }
                let signature = format!(
                    "{}({})",
                    event.name,
                    event
                        .inputs
                        .iter()
                        .map(|input| input.kind.to_string())
                        .collect::<Vec<_>>()
                        .join(",")
                );
                let signatures = events.entry(hex::encode(event.signature())).or_default();
                if !signatures.contains(&signature) {
                    signatures.push(signature);
                }
            }
            Signatures {
                functions,
                events,
                complete_events,
            }
        })
    }

    fn lookup<T: Clone>(signatures: &HashMap<String, Vec<T>>, selector: &str) -> Vec<T> {
        let selector = selector.strip_prefix("0x").unwrap_or(selector);
        signatures
            .get(&selector.to_ascii_lowercase())
            .cloned()
            .unwrap_or_default()
    }
}

#[async_trait::async_trait]
impl super::CompleteSignatureSource for PrecompileSource {
    fn covers_event(&self, hex: &str) -> bool {
        !Self::lookup(&Self::signatures().complete_events, hex).is_empty()
    }

    async fn get_event_signatures(
        &self,
        hex: &str,
    ) -> Result<Vec<alloy_json_abi::Event>, anyhow::Error> {
        Ok(Self::lookup(&Self::signatures().complete_events, hex))
    }

    async fn batch_get_event_signatures(
        &self,
        hex: &[String],
    ) -> Result<Vec<Vec<alloy_json_abi::Event>>, anyhow::Error> {
        let signatures = Self::signatures();
        Ok(hex
            .iter()
            .map(|selector| Self::lookup(&signatures.complete_events, selector))
            .collect())
    }

    fn source(&self) -> String {
        "precompiles".to_owned()
    }
}

#[async_trait::async_trait]
impl SignatureSource for PrecompileSource {
    fn covers_function(&self, hex: &str) -> bool {
        !Self::lookup(&Self::signatures().functions, hex).is_empty()
    }

    fn covers_event(&self, hex: &str) -> bool {
        !Self::lookup(&Self::signatures().events, hex).is_empty()
    }

    fn accepts_imports(&self) -> bool {
        false
    }

    async fn create_signatures(&self, _abi: &str) -> Result<(), anyhow::Error> {
        anyhow::bail!("embedded precompile inventory is read-only")
    }

    async fn get_function_signatures(&self, hex: &str) -> Result<Vec<String>, anyhow::Error> {
        Ok(Self::lookup(&Self::signatures().functions, hex))
    }

    async fn get_event_signatures(&self, hex: &str) -> Result<Vec<String>, anyhow::Error> {
        Ok(Self::lookup(&Self::signatures().events, hex))
    }

    async fn batch_get_event_signatures(
        &self,
        hex: &[String],
    ) -> Result<Vec<Vec<String>>, anyhow::Error> {
        let signatures = Self::signatures();
        Ok(hex
            .iter()
            .map(|selector| Self::lookup(&signatures.events, selector))
            .collect())
    }

    fn source(&self) -> String {
        "precompiles".to_owned()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn selector_resolves_to_exchange_signature() {
        let source = PrecompileSource::new();
        let selector = ethabi::short_signature("cancelOrder", &[ethabi::ParamType::FixedBytes(32)]);
        let selector = hex::encode(selector);
        let expected = vec!["cancelOrder(bytes32)".to_owned()];
        for selector in [selector.clone(), format!("0x{selector}")] {
            assert_eq!(
                source.get_function_signatures(&selector).await.unwrap(),
                expected
            );
        }
        assert_eq!(source.source(), "precompiles");
        assert!(source.create_signatures("[]").await.is_err());

        let topic = hex::encode(ethabi::long_signature(
            "OrderCancelRequested",
            &[
                ethabi::ParamType::FixedBytes(32),
                ethabi::ParamType::FixedBytes(32),
                ethabi::ParamType::Address,
                ethabi::ParamType::Uint(64),
            ],
        ));
        let expected_event =
            vec!["OrderCancelRequested(bytes32,bytes32,address,uint64)".to_owned()];
        assert_eq!(
            source.get_event_signatures(&topic).await.unwrap(),
            expected_event
        );
        assert_eq!(
            source
                .batch_get_event_signatures(&[format!("0x{topic}"), "00".repeat(32), topic])
                .await
                .unwrap(),
            vec![expected_event.clone(), vec![], expected_event]
        );
    }

    #[tokio::test]
    async fn unknown_selector_returns_empty() {
        let source = PrecompileSource::new();
        assert!(source
            .get_function_signatures("0xffffffff")
            .await
            .unwrap()
            .is_empty());
        assert!(source
            .get_event_signatures(&"ff".repeat(32))
            .await
            .unwrap()
            .is_empty());
        assert!(source
            .batch_get_event_signatures(&[])
            .await
            .unwrap()
            .is_empty());
    }
}
