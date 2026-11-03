use crate::GetSourceFilesResponse;
use serde::Deserialize;
use serde_json::{json, Value};
use std::collections::BTreeMap;

// Match status, chain ID and address are always included in the minimal response.
// chainId and address are not accepted as field selectors by the v2 API.
pub(crate) const CONTRACT_FIELDS: &str = "sources,metadata,deployment.transactionHash,compilation,stdJsonOutput,creationBytecode.transformationValues,runtimeBytecode.transformationValues,runtimeBytecode.immutableReferences";

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct ApiError {
    pub custom_code: String,
    pub message: String,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct Submission {
    pub verification_id: String,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct Job {
    pub is_job_completed: bool,
    pub error: Option<ApiError>,
}

#[derive(Deserialize, PartialEq)]
#[serde(rename_all = "snake_case")]
pub(crate) enum Match {
    Match,
    ExactMatch,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct Chain {
    pub chain_id: u64,
    pub supported: bool,
}

#[derive(Deserialize)]
struct Source {
    content: String,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct Contract {
    pub creation_match: Option<Match>,
    pub runtime_match: Option<Match>,
    chain_id: String,
    address: String,
    sources: Option<BTreeMap<String, Source>>,
    metadata: Option<Value>,
    deployment: Option<Value>,
    compilation: Option<Value>,
    std_json_output: Option<Value>,
    creation_bytecode: Option<Value>,
    runtime_bytecode: Option<Value>,
}

impl Contract {
    pub fn into_source_files(self) -> Result<GetSourceFilesResponse, String> {
        let full = self.creation_match == Some(Match::ExactMatch)
            || self.runtime_match == Some(Match::ExactMatch);
        let status = if full { "full" } else { "partial" };
        let directory = if full { "full_match" } else { "partial_match" };
        let prefix = format!("contracts/{directory}/{}/{}", self.chain_id, self.address);
        let mut files = Vec::new();
        let mut add_file = |name: &str, path: String, content: String| {
            files.push(json!({"name": name, "path": path, "content": content}));
        };
        let sources = self.sources.as_ref().ok_or("sources are missing")?;
        if sources.is_empty() {
            return Err("sources are missing".into());
        }
        for (path, source) in sources {
            let name = path
                .rsplit('/')
                .next()
                .ok_or("source filename is missing")?;
            add_file(
                name,
                format!("{prefix}/sources/{path}"),
                source.content.clone(),
            );
        }
        let metadata = self.metadata.as_ref().ok_or("metadata file is missing")?;
        add_file(
            "metadata.json",
            format!("{prefix}/metadata.json"),
            metadata.to_string(),
        );
        if let Some(hash) = self
            .deployment
            .as_ref()
            .and_then(|v| v.get("transactionHash"))
            .and_then(Value::as_str)
        {
            add_file(
                "creator-tx-hash.txt",
                format!("{prefix}/creator-tx-hash.txt"),
                hash.to_string(),
            );
        }
        let libraries = self.library_map()?;
        if !libraries.is_empty() {
            add_file(
                "library-map.json",
                format!("{prefix}/library-map.json"),
                serde_json::to_string(&libraries).map_err(|error| error.to_string())?,
            );
        }
        if let Some(arguments) = self
            .creation_bytecode
            .as_ref()
            .and_then(|v| v.pointer("/transformationValues/constructorArguments"))
            .and_then(Value::as_str)
        {
            if arguments != "0x" && !arguments.is_empty() {
                add_file(
                    "constructor-args.txt",
                    format!("{prefix}/constructor-args.txt"),
                    arguments.to_string(),
                );
            }
        }
        if let Some(references) = self
            .runtime_bytecode
            .as_ref()
            .and_then(|v| v.get("immutableReferences"))
        {
            if !references.is_null() && references.as_object().is_some_and(|v| !v.is_empty()) {
                add_file(
                    "immutable-references.json",
                    format!("{prefix}/immutable-references.json"),
                    references.to_string(),
                );
            }
        }
        serde_json::from_value(json!({"status": status, "files": files}))
            .map_err(|error| error.to_string())
    }

    fn library_map(&self) -> Result<BTreeMap<String, String>, String> {
        let mut libraries = BTreeMap::new();
        for bytecode in [&self.creation_bytecode, &self.runtime_bytecode]
            .into_iter()
            .flatten()
        {
            let Some(values) = bytecode.pointer("/transformationValues/libraries") else {
                continue;
            };
            let values = values
                .as_object()
                .ok_or("library transformations must be an object")?;
            for (name, address) in values {
                let address = address.as_str().ok_or("library address must be a string")?;
                let placeholder = self.library_placeholder(name)?;
                if let Some(previous) = libraries.insert(placeholder, address.to_string()) {
                    if previous != address {
                        return Err(format!("conflicting addresses for library {name}"));
                    }
                }
            }
        }
        Ok(libraries)
    }

    fn library_placeholder(&self, name: &str) -> Result<String, String> {
        let (source, library) = name.rsplit_once(':').ok_or("invalid library identifier")?;
        let compilation = self
            .compilation
            .as_ref()
            .ok_or("compilation details are missing")?;
        let target = compilation
            .get("fullyQualifiedName")
            .and_then(Value::as_str)
            .ok_or("compilation target is missing")?;
        let (target_source, target_name) = target
            .rsplit_once(':')
            .ok_or("invalid compilation target")?;
        let contract = self
            .std_json_output
            .as_ref()
            .and_then(|v| v.get("contracts"))
            .and_then(|v| v.get(target_source))
            .and_then(|v| v.get(target_name))
            .and_then(|v| v.get("evm"))
            .ok_or("compiler bytecode output is missing")?;
        for kind in ["bytecode", "deployedBytecode"] {
            let Some(bytecode) = contract.get(kind) else {
                continue;
            };
            let Some(references) = bytecode
                .get("linkReferences")
                .and_then(|v| v.get(source))
                .and_then(|v| v.get(library))
                .and_then(Value::as_array)
            else {
                continue;
            };
            if let Some(reference) = references.first() {
                let start = reference
                    .get("start")
                    .and_then(Value::as_u64)
                    .ok_or("invalid library offset")?;
                let length = reference
                    .get("length")
                    .and_then(Value::as_u64)
                    .ok_or("invalid library length")?;
                let object = bytecode
                    .get("object")
                    .and_then(Value::as_str)
                    .ok_or("compiler bytecode is missing")?;
                let object = object.strip_prefix("0x").unwrap_or(object);
                let start = usize::try_from(start)
                    .ok()
                    .and_then(|v| v.checked_mul(2))
                    .ok_or("library offset overflow")?;
                let end = usize::try_from(length)
                    .ok()
                    .and_then(|v| v.checked_mul(2))
                    .and_then(|v| start.checked_add(v))
                    .ok_or("library length overflow")?;
                let placeholder = object
                    .get(start..end)
                    .ok_or("library reference is outside the bytecode")?;
                // Sourcify recompiles with zero library addresses, so its standard
                // JSON output can contain zeros instead of Solidity's link token.
                if placeholder == "0000000000000000000000000000000000000000" {
                    let version = compilation
                        .get("compilerVersion")
                        .and_then(Value::as_str)
                        .ok_or("compiler version is missing")?;
                    return solidity_library_placeholder(name, version);
                }
                if placeholder.len() != 40
                    || !placeholder.starts_with("__")
                    || !placeholder.ends_with("__")
                {
                    return Err(format!(
                        "compiler output has no placeholder for library {name}"
                    ));
                }
                return Ok(placeholder.to_string());
            }
        }
        Err(format!(
            "compiler link reference for library {name} is missing"
        ))
    }
}

fn solidity_library_placeholder(name: &str, version: &str) -> Result<String, String> {
    let mut version = version.split('.');
    let major: u64 = version
        .next()
        .ok_or("compiler major version is missing")?
        .parse()
        .map_err(|_| "invalid compiler major version")?;
    let minor: u64 = version
        .next()
        .ok_or("compiler minor version is missing")?
        .parse()
        .map_err(|_| "invalid compiler minor version")?;
    if major == 0 && minor < 5 {
        let identifier = name
            .get(..name.len().min(36))
            .ok_or("invalid library name")?;
        return Ok(format!("__{identifier:_<36}__"));
    }
    let hash = blockscout_display_bytes::ToHex::to_hex(&crate::keccak::keccak256(name.as_bytes()));
    Ok(format!("__${}$__", &hash[2..36]))
}
