use k256::ecdsa::SigningKey;
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use sha3::Keccak256;
use std::collections::{BTreeMap, BTreeSet};
use std::fs::{self, File, OpenOptions};
use std::io::{Read, Write};
use std::os::unix::fs::{DirBuilderExt, MetadataExt, OpenOptionsExt, PermissionsExt};
use std::path::{Path, PathBuf};
use std::time::Duration;
use zeroize::Zeroizing;

pub type Result<T> = std::result::Result<T, Box<dyn std::error::Error + Send + Sync>>;
const MAX_INPUT: u64 = 64 * 1024 * 1024;
const INPUT_FILES: [&str; 5] = [
    "genesis.json",
    "config.toml",
    "app.toml",
    "priv_validator_key.json",
    "rpc-ca.der",
];
const OUTPUT_FILES: [&str; 8] = [
    "config/genesis.json",
    "config/config.toml",
    "config/app.toml",
    "config/priv_validator_key.json",
    "node.env",
    "core.env",
    "rpc-ca.der",
    ".native-prepared.json",
];

pub fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}

fn hash(bytes: &[u8]) -> String {
    hex(&Sha256::digest(bytes))
}

fn canonical(value: &Value) -> Result<Vec<u8>> {
    fn sorted(value: &Value) -> Value {
        match value {
            Value::Object(values) => {
                let values: BTreeMap<_, _> =
                    values.iter().map(|(k, v)| (k.clone(), sorted(v))).collect();
                Value::Object(values.into_iter().collect())
            }
            Value::Array(values) => Value::Array(values.iter().map(sorted).collect()),
            value => value.clone(),
        }
    }
    Ok(serde_json::to_vec(&sorted(value))?)
}

fn read_file(path: &Path, maximum: u64, secret: bool) -> Result<Zeroizing<Vec<u8>>> {
    let mut file = OpenOptions::new()
        .read(true)
        .custom_flags(rustix::fs::OFlags::NOFOLLOW.bits() as i32)
        .open(path)?;
    let info = file.metadata()?;
    if !info.is_file()
        || info.nlink() != 1
        || info.len() > maximum
        || info.mode() & 0o022 != 0
        || (secret
            && (info.mode() & 0o777 != 0o600 || info.uid() != rustix::process::getuid().as_raw()))
    {
        return Err(format!("unsafe native input: {}", path.display()).into());
    }
    let mut raw = Zeroizing::new(Vec::new());
    (&mut file).take(maximum + 1).read_to_end(&mut raw)?;
    if raw.len() as u64 > maximum {
        return Err("native input exceeds bound".into());
    }
    Ok(raw)
}

fn private_directory(path: &Path) -> Result<()> {
    match fs::DirBuilder::new().mode(0o700).create(path) {
        Ok(()) => (),
        Err(error) if error.kind() == std::io::ErrorKind::AlreadyExists => (),
        Err(error) => return Err(error.into()),
    }
    let info = fs::symlink_metadata(path)?;
    if !info.is_dir()
        || info.mode() & 0o777 != 0o700
        || info.uid() != rustix::process::getuid().as_raw()
    {
        return Err("native state directory requires private ownership".into());
    }
    Ok(())
}

fn fingerprints(prefix: &str, value: &Value, fields: &mut BTreeMap<String, String>) -> Result<()> {
    // Include containers so additions, removals and empty objects are bound too.
    fields.insert(prefix.to_owned(), hash(&canonical(value)?));
    match value {
        Value::Object(values) => {
            for (key, value) in values {
                fingerprints(
                    &format!("{prefix}/{}", key.replace('~', "~0").replace('/', "~1")),
                    value,
                    fields,
                )?;
            }
        }
        Value::Array(values) => {
            for (index, value) in values.iter().enumerate() {
                fingerprints(&format!("{prefix}/{index}"), value, fields)?;
            }
        }
        _ => (),
    }
    Ok(())
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ImmutableInputs {
    pub digest: String,
    pub fields: BTreeMap<String, String>,
}

impl ImmutableInputs {
    pub fn capture(source: &Path, options: &BTreeMap<String, String>) -> Result<Self> {
        let mut fields = BTreeMap::new();
        for name in INPUT_FILES {
            let raw = read_file(
                &source.join(name),
                MAX_INPUT,
                name == "priv_validator_key.json",
            )?;
            if name.ends_with(".json") {
                let value: Value = serde_json::from_slice(&raw)?;
                fingerprints(name, &value, &mut fields)?;
            } else {
                fields.insert(name.to_owned(), hash(&raw));
            }
        }
        for (key, value) in options {
            fields.insert(format!("options/{key}"), hash(value.as_bytes()));
        }
        for key in ["receipt-key-file", "funding-sender-key-file"] {
            let path = options.get(key).ok_or("missing protected signing input")?;
            fields.insert(
                format!("signing/{key}"),
                hash(&read_file(Path::new(path), 4096, true)?),
            );
        }
        let digest = hash(&serde_json::to_vec(&fields)?);
        Ok(Self { digest, fields })
    }

    fn compare(&self, requested: &Self) -> Result<()> {
        if self == requested {
            return Ok(());
        }
        let changed = self
            .fields
            .keys()
            .chain(requested.fields.keys())
            .filter(|key| self.fields.get(*key) != requested.fields.get(*key))
            .max_by_key(|key| key.matches('/').count())
            .map(String::as_str)
            .unwrap_or("digest");
        Err(format!("immutable native input drift: {changed}").into())
    }
}

#[derive(Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct InitializationRecord {
    schema: String,
    inputs: ImmutableInputs,
    outputs: BTreeMap<String, String>,
}

pub struct BootstrapGuard {
    _lock: File,
    state: PathBuf,
    data: PathBuf,
    inputs: ImmutableInputs,
    initialized: bool,
}

impl BootstrapGuard {
    pub fn open(state: &Path, data: &Path, inputs: ImmutableInputs) -> Result<Self> {
        private_directory(state)?;
        let lock = OpenOptions::new()
            .read(true)
            .write(true)
            .create(true)
            .truncate(false)
            .mode(0o600)
            .custom_flags(rustix::fs::OFlags::NOFOLLOW.bits() as i32)
            .open(state.join("owner.lock"))?;
        lock.try_lock()
            .map_err(|_| "native initialization is already owned")?;
        let mut guard = Self {
            _lock: lock,
            state: state.to_owned(),
            data: data.to_owned(),
            inputs,
            initialized: false,
        };
        match fs::symlink_metadata(state.join("initialized.json")) {
            Ok(_) => {
                let record: InitializationRecord = serde_json::from_slice(&read_file(
                    &state.join("initialized.json"),
                    MAX_INPUT,
                    true,
                )?)?;
                if record.schema != "paxeer/native-initialization/1" {
                    return Err("unknown native initialization marker".into());
                }
                record.inputs.compare(&guard.inputs)?;
                if record.outputs != guard.output_binding()? {
                    return Err("initialized native generation has changed".into());
                }
                guard.initialized = true;
            }
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => {
                if data.exists() && fs::read_dir(data)?.next().is_some() {
                    return Err(
                        "unbound native generation; reconcile with the maintained supervisor"
                            .into(),
                    );
                }
            }
            Err(error) => return Err(error.into()),
        }
        Ok(guard)
    }

    pub fn initialized(&self) -> bool {
        self.initialized
    }

    fn output_binding(&self) -> Result<BTreeMap<String, String>> {
        OUTPUT_FILES
            .iter()
            .map(|name| {
                Ok((
                    (*name).to_owned(),
                    hash(&read_file(
                        &self.data.join(name),
                        MAX_INPUT,
                        name.contains("priv_validator_key"),
                    )?),
                ))
            })
            .collect()
    }

    pub fn complete(&mut self, current: &ImmutableInputs) -> Result<()> {
        if self.initialized {
            return Err("native generation is already initialized".into());
        }
        self.inputs.compare(current)?;
        for name in INPUT_FILES {
            let target = if name == "rpc-ca.der" {
                name.to_owned()
            } else {
                format!("config/{name}")
            };
            let raw = read_file(
                &self.data.join(target),
                MAX_INPUT,
                name == "priv_validator_key.json",
            )?;
            let digest = if name.ends_with(".json") {
                hash(&canonical(&serde_json::from_slice(&raw)?)?)
            } else {
                hash(&raw)
            };
            if self.inputs.fields.get(name) != Some(&digest) {
                return Err(format!(
                    "native bootstrap output differs from requested input: {name}"
                )
                .into());
            }
        }
        let record = InitializationRecord {
            schema: "paxeer/native-initialization/1".into(),
            inputs: self.inputs.clone(),
            outputs: self.output_binding()?,
        };
        let pending = self.state.join("initialized.pending");
        // An interrupted publication is safe to replace while holding the owner lock.
        match fs::remove_file(&pending) {
            Ok(()) => (),
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => (),
            Err(error) => return Err(error.into()),
        }
        let mut output = OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(&pending)?;
        output.write_all(&serde_json::to_vec(&record)?)?;
        output.sync_all()?;
        fs::rename(pending, self.state.join("initialized.json"))?;
        File::open(&self.state)?.sync_all()?;
        self.initialized = true;
        Ok(())
    }
}

pub struct BootstrapRequest {
    pub data: PathBuf,
    pub source: PathBuf,
    pub options: BTreeMap<String, String>,
    pub arguments: Vec<String>,
}

impl BootstrapRequest {
    pub fn parse(arguments: &[String]) -> Result<Self> {
        let mut options: BTreeMap<String, String> = [
            ("paxd", "/usr/local/bin/paxd"),
            ("moniker", "hosted-native"),
            ("chain-rpc-url", "http://127.0.0.1:8545"),
            ("consensus-rpc-url", "http://127.0.0.1:26657"),
            ("rpc-bind-address", "127.0.0.1"),
            ("consumer-uid", "4021"),
            ("consumer-gid", "4020"),
            ("ready-seconds", "120"),
        ]
        .into_iter()
        .map(|(k, v)| (k.into(), v.into()))
        .collect();
        let allowed = [
            "data-dir",
            "native-inputs",
            "paxd",
            "moniker",
            "chain-id",
            "network-id",
            "evm-chain-id",
            "asset",
            "gateway-endpoint",
            "chain-rpc-url",
            "consensus-rpc-url",
            "rpc-bind-address",
            "receipt-key-file",
            "treasury-signer-socket",
            "funding-sender-key-file",
            "consumer-uid",
            "consumer-gid",
            "ready-seconds",
        ];
        let mut seen = BTreeSet::new();
        if arguments.len() % 2 != 0 {
            return Err("bootstrap requires explicit flag/value pairs".into());
        }
        for pair in arguments.chunks_exact(2) {
            let key = pair[0]
                .strip_prefix("--")
                .ok_or("bootstrap requires named inputs")?;
            if !allowed.contains(&key) || !seen.insert(key) || pair[1].is_empty() {
                return Err("unknown, duplicate or empty native bootstrap input".into());
            }
            options.insert(key.into(), pair[1].clone());
        }
        for key in [
            "data-dir",
            "native-inputs",
            "chain-id",
            "network-id",
            "evm-chain-id",
            "asset",
            "gateway-endpoint",
            "receipt-key-file",
            "treasury-signer-socket",
            "funding-sender-key-file",
        ] {
            if !options.contains_key(key) {
                return Err(format!("missing native input --{key}").into());
            }
        }
        if options["chain-id"] != "hyperpax_125-1" || options["evm-chain-id"] != "125" {
            return Err("native deployment requires chain id 125".into());
        }
        for key in [
            "data-dir",
            "native-inputs",
            "receipt-key-file",
            "treasury-signer-socket",
            "funding-sender-key-file",
            "paxd",
        ] {
            if !Path::new(&options[key]).is_absolute() {
                return Err(format!("{key} requires an absolute path").into());
            }
        }
        Ok(Self {
            data: PathBuf::from(&options["data-dir"]),
            source: PathBuf::from(&options["native-inputs"]),
            options,
            arguments: arguments.to_vec(),
        })
    }

    pub fn open(&self) -> Result<BootstrapGuard> {
        let name = self
            .data
            .file_name()
            .ok_or("native home needs a directory name")?
            .to_str()
            .ok_or("native home must be UTF-8")?;
        let state = self
            .data
            .with_file_name(format!(".{name}.native-initialization"));
        BootstrapGuard::open(
            &state,
            &self.data,
            ImmutableInputs::capture(&self.source, &self.options)?,
        )
    }

    pub fn command(&self, script: &Path) -> std::process::Command {
        let mut command = std::process::Command::new(script);
        command.args(&self.arguments);
        command
    }
}

#[derive(Debug, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct AccountIdentity {
    pub address: String,
    pub public_key: String,
}

pub fn account_identity(secret: &[u8]) -> Result<AccountIdentity> {
    let key = SigningKey::from_slice(secret).map_err(|_| "invalid secp256k1 signing scalar")?;
    let public = key.verifying_key();
    let uncompressed = public.to_encoded_point(false);
    let digest = Keccak256::digest(&uncompressed.as_bytes()[1..]);
    let lower = hex(&digest[12..]);
    let checksum = Keccak256::digest(lower.as_bytes());
    let address: String = lower
        .chars()
        .enumerate()
        .map(|(i, c)| {
            let nibble = if i % 2 == 0 {
                checksum[i / 2] >> 4
            } else {
                checksum[i / 2] & 15
            };
            if nibble >= 8 {
                c.to_ascii_uppercase()
            } else {
                c
            }
        })
        .collect();
    Ok(AccountIdentity {
        address: format!("0x{address}"),
        public_key: format!("0x{}", hex(public.to_encoded_point(true).as_bytes())),
    })
}

fn scalar_file(path: &Path) -> Result<Zeroizing<Vec<u8>>> {
    let raw = read_file(path, 67, true)?;
    let text = std::str::from_utf8(&raw)?.trim_end_matches('\n');
    let text = text.strip_prefix("0x").unwrap_or(text);
    if text.len() != 64 || !text.bytes().all(|c| c.is_ascii_hexdigit()) {
        return Err("protected account key must contain a 32-byte hexadecimal scalar".into());
    }
    let mut bytes = Zeroizing::new(Vec::with_capacity(32));
    for index in (0..64).step_by(2) {
        bytes.push(u8::from_str_radix(&text[index..index + 2], 16)?);
    }
    Ok(bytes)
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
pub struct TenantAccount {
    pub schema: String,
    pub tenant: String,
    pub key_file: PathBuf,
    pub chain_rpc: String,
    pub consensus_rpc: String,
    pub minimum_balance: String,
}

fn quantity(value: &str) -> Result<[u8; 32]> {
    let digits = value
        .strip_prefix("0x")
        .ok_or("balance requires an Ethereum quantity")?;
    if digits.is_empty()
        || digits.len() > 64
        || (digits.len() > 1 && digits.starts_with('0'))
        || !digits
            .bytes()
            .all(|c| c.is_ascii_digit() || (b'a'..=b'f').contains(&c))
    {
        return Err("invalid Ethereum quantity".into());
    }
    let padded = format!("{digits:0>64}");
    let mut bytes = [0; 32];
    for (i, byte) in bytes.iter_mut().enumerate() {
        *byte = u8::from_str_radix(&padded[i * 2..i * 2 + 2], 16)?;
    }
    Ok(bytes)
}

fn rpc(url: &str, method: &str, params: Value) -> Result<Value> {
    let address = url
        .strip_prefix("http://127.0.0.1:")
        .and_then(|v| v.strip_suffix('/'))
        .ok_or("native evidence requires a loopback root RPC origin")?;
    if address.parse::<u16>()? == 0 {
        return Err("RPC port cannot be zero".into());
    }
    let agent: ureq::Agent = ureq::Agent::config_builder()
        .proxy(None)
        .max_redirects(0)
        .timeout_global(Some(Duration::from_secs(5)))
        .build()
        .into();
    let mut reply = agent
        .post(url)
        .send_json(json!({"jsonrpc":"2.0", "id":method, "method":method, "params":params}))?;
    if reply.status() != 200
        || !reply
            .headers()
            .get("content-type")
            .and_then(|v| v.to_str().ok())
            .is_some_and(|v| v.split(';').next() == Some("application/json"))
    {
        return Err("native evidence transport refused".into());
    }
    let raw = reply
        .body_mut()
        .with_config()
        .limit(8 * 1024 * 1024)
        .read_to_vec()?;
    let value: Value = serde_json::from_slice(&raw)?;
    if value["jsonrpc"] != "2.0" || value["id"] != method || value.get("error").is_some() {
        return Err("native evidence RPC refused".into());
    }
    value
        .get("result")
        .cloned()
        .ok_or_else(|| "native evidence RPC result missing".into())
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct NativeEvidence {
    pub schema: String,
    pub chain_id: u64,
    pub consensus_chain_id: String,
    pub height: u64,
    pub block_hash: String,
    pub validators: Vec<Value>,
}

fn hash_text(value: &Value) -> Result<String> {
    let value = value.as_str().ok_or("missing block hash")?;
    if value.len() != 64 || !value.bytes().all(|c| c.is_ascii_hexdigit()) {
        return Err("invalid committed block hash".into());
    }
    Ok(value.to_ascii_lowercase())
}

impl NativeEvidence {
    pub fn fetch(chain_rpc: &str, consensus_rpc: &str) -> Result<Self> {
        if rpc(chain_rpc, "eth_chainId", json!([]))? != "0x7d" {
            return Err("native evidence requires chain id 125".into());
        }
        let status = rpc(consensus_rpc, "status", json!({}))?;
        if status["node_info"]["network"] != "hyperpax_125-1"
            || status["sync_info"]["catching_up"] != false
        {
            return Err("native consensus is not committed and caught up".into());
        }
        let height = status["sync_info"]["latest_block_height"]
            .as_str()
            .ok_or("missing committed height")?
            .parse::<u64>()?;
        if height == 0 || status["sync_info"]["latest_block_height"] != height.to_string() {
            return Err("invalid committed height".into());
        }
        let block_hash = hash_text(&status["sync_info"]["latest_block_hash"])?;
        let commit = rpc(
            consensus_rpc,
            "commit",
            json!({"height":height.to_string()}),
        )?;
        let header = &commit["signed_header"]["header"];
        let signed = &commit["signed_header"]["commit"];
        if commit["canonical"] != true
            || header["chain_id"] != "hyperpax_125-1"
            || header["height"] != height.to_string()
            || signed["height"] != height.to_string()
            || hash_text(&signed["block_id"]["hash"])? != block_hash
            || !signed["signatures"]
                .as_array()
                .is_some_and(|values| !values.is_empty())
        {
            return Err("native signed commit differs from status".into());
        }
        hash_text(&header["app_hash"])?;
        let block = rpc(
            chain_rpc,
            "eth_getBlockByNumber",
            json!([format!("0x{height:x}"), false]),
        )?;
        if block["number"] != format!("0x{height:x}")
            || block["hash"].as_str().map(str::to_ascii_lowercase)
                != Some(format!("0x{block_hash}"))
        {
            return Err("native EVM block differs from committed consensus height".into());
        }
        let mut validators = Vec::new();
        let mut addresses = BTreeSet::new();
        let mut expected_total = None;
        for page in 1..=41 {
            let result = rpc(
                consensus_rpc,
                "validators",
                json!({"height":height.to_string(), "page":page.to_string(), "per_page":"100"}),
            )?;
            let total = result["total"]
                .as_str()
                .ok_or("missing validator total")?
                .parse::<usize>()?;
            let members = result["validators"]
                .as_array()
                .ok_or("missing native validators")?;
            if result["block_height"] != height.to_string()
                || total == 0
                || total > 4096
                || expected_total.is_some_and(|previous| previous != total)
                || members.is_empty()
                || members.len() > 100
                || result["count"] != members.len().to_string()
            {
                return Err("validator page differs from committed height or bounds".into());
            }
            expected_total = Some(total);
            for member in members {
                let address = member["address"]
                    .as_str()
                    .ok_or("missing validator address")?;
                if address.len() != 40
                    || !address.bytes().all(|c| c.is_ascii_hexdigit())
                    || !addresses.insert(address.to_ascii_lowercase())
                    || member["voting_power"]
                        .as_str()
                        .ok_or("missing voting power")?
                        .parse::<u64>()?
                        == 0
                    || member["pub_key"]["type"] != "tendermint/PubKeyEd25519"
                    || member["pub_key"]["value"]
                        .as_str()
                        .is_none_or(str::is_empty)
                {
                    return Err("invalid committed validator member".into());
                }
                validators.push(member.clone());
            }
            if validators.len() == total {
                break;
            }
            if validators.len() > total {
                return Err("validator pages exceed committed total".into());
            }
        }
        if expected_total != Some(validators.len()) {
            return Err("incomplete committed validator set".into());
        }
        Ok(Self {
            schema: "paxeer/committed-native-evidence/1".into(),
            chain_id: 125,
            consensus_chain_id: "hyperpax_125-1".into(),
            height,
            block_hash,
            validators,
        })
    }
}

pub fn provision_account(config: &TenantAccount) -> Result<Value> {
    if config.schema != "paxeer/native-tenant-account/1"
        || config.tenant.is_empty()
        || config.tenant.len() > 128
        || !config
            .tenant
            .bytes()
            .all(|c| c.is_ascii_alphanumeric() || c == b'-' || c == b'_')
        || !config.key_file.is_absolute()
    {
        return Err("invalid per-tenant native account configuration".into());
    }
    let minimum = quantity(&config.minimum_balance)?;
    if minimum == [0; 32] {
        return Err("committed account funding must be positive".into());
    }
    let identity = account_identity(&scalar_file(&config.key_file)?)?;
    let evidence = NativeEvidence::fetch(&config.chain_rpc, &config.consensus_rpc)?;
    let balance = rpc(
        &config.chain_rpc,
        "eth_getBalance",
        json!([identity.address, format!("0x{:x}", evidence.height)]),
    )?;
    let balance = balance
        .as_str()
        .ok_or("missing committed account balance")?;
    if quantity(balance)? < minimum {
        return Err("tenant account lacks committed-chain funding".into());
    }
    Ok(
        json!({"schema":"paxeer/funded-native-account/1", "tenant":config.tenant, "identity":identity, "balance":balance, "evidence":evidence}),
    )
}

pub fn run_read_command(arguments: &[String], output: &mut impl Write) -> Result<()> {
    let value = match arguments {
        [command, config] if command == "account" => {
            let config: TenantAccount = serde_json::from_slice(&read_file(Path::new(config), 16384, false)?)?;
            provision_account(&config)?
        }
        [command, chain, consensus] if command == "evidence" => serde_json::to_value(NativeEvidence::fetch(chain, consensus)?)?,
        _ => return Err("usage: paxeer-native-tool account CONFIG | evidence CHAIN_RPC CONSENSUS_RPC; guarantor, checkpoint and settlement bootstrap are retired".into()),
    };
    serde_json::to_writer(&mut *output, &value)?;
    writeln!(output)?;
    Ok(())
}
