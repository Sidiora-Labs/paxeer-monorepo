# Platform CLI contract

The platform CLI and emulator are implemented under [tools/platform](../../../tools/platform). The CLI uses signed EVM transactions for program writes and `eth_call` for simulations and reads. `program call` uses `--evm-key-file`; `program simulate` supports `--from`. Both target the program construct at `0x101D`.

The tables identify code declarations and configuration names. Activation, authentication and transport registration are defined by the linked implementations.

## cli-cli-commands

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `struct Cli {` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `A2a(A2aCommand),` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Serve {` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `asset: Option<String>,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `authorization_file: PathBuf,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `environment: Option<String>,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `gateway_credential: String,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `key: Option<String>,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `listen: String,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `read_only: bool,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `source_account: Option<String>,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Start,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Status,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Stop,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Account(AccountCommand),` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Create {` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `display_name: Option<String>,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `email: Option<String>,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `idempotency_key: Option<String>,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `initial_amount: String,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Get {` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `did: Option<String>,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Auth(AuthCommand),` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Delete {` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Set {` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Status {` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Emulator(EmulatorCommand),` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Provision {` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `force: bool,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Up(EmulatorUpArgs),` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `listen: Option<String>,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `network_id: Option<u32>,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `prefund: Vec<String>,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `protocol_version: Option<u16>,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `sequencer_seed_file: PathBuf,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `time_ms: Option<u64>,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Environment(EnvironmentCommand),` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Current,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `List,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Use {` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `endpoint: Option<String>,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `name: String,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `sequencer_trust_anchor: Option<String>,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `sequencer_trust_anchor_file: Option<PathBuf>,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Faucet(faucet::FaucetArgs),` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `key: Option<String>,` | [tools/platform/cli/src/faucet.rs](../../../tools/platform/cli/src/faucet.rs) |
| `cli_command` | `gateway_credential: Option<String>,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Install(InstallCommand),` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `A2a(InstallA2aArgs),` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `rotate: bool,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `token_stdin: bool,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `well_known: Option<PathBuf>,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Mcp(InstallMcpArgs),` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `daemon_binding: Option<PathBuf>,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `host: Vec<String>,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `json: bool,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Key(KeyCommand),` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Default { name: String },` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Delete { name: String },` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Import {` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Show { name: String },` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Mcp(McpCommand),` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `daemon_binding: PathBuf,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `New(NewArgs),` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `directory: PathBuf,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Payment(PaymentCommand),` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Test {` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `amount: String,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `currency: String,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `from: String,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `idempotency_key: String,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `to: String,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Program(ProgramCommand),` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Bindings {` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `code_hash: String,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `deployment_proof: PathBuf,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `digest: String,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `historical: bool,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `interface: PathBuf,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `output: PathBuf,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `trust_history: PathBuf,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Build {` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `artifact: Option<PathBuf>,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `manifest_path: PathBuf,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Call(ProgramExecutionArgs),` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `pub abi_version: u16,` | [tools/platform/cli/src/programs.rs](../../../tools/platform/cli/src/programs.rs) |
| `cli_command` | `pub access_declaration: Option<String>,` | [tools/platform/cli/src/programs.rs](../../../tools/platform/cli/src/programs.rs) |
| `cli_command` | `calldata: Option<String>,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `capabilities: Vec<String>,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `pub entrypoint: String,` | [tools/platform/cli/src/programs.rs](../../../tools/platform/cli/src/programs.rs) |
| `cli_command` | `evm_key_file: Option<PathBuf>,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `from: Option<String>,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `fee_limit: String,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `fuel: u64,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `pub memory_bytes: u64,` | [tools/platform/cli/src/programs.rs](../../../tools/platform/cli/src/programs.rs) |
| `cli_command` | `pub output_bytes: u64,` | [tools/platform/cli/src/programs.rs](../../../tools/platform/cli/src/programs.rs) |
| `cli_command` | `pub output_values: u64,` | [tools/platform/cli/src/programs.rs](../../../tools/platform/cli/src/programs.rs) |
| `cli_command` | `program_id: String,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `pub response_capacity: u32,` | [tools/platform/cli/src/programs.rs](../../../tools/platform/cli/src/programs.rs) |
| `cli_command` | `pub storage_read_bytes: u64,` | [tools/platform/cli/src/programs.rs](../../../tools/platform/cli/src/programs.rs) |
| `cli_command` | `pub storage_write_bytes: u64,` | [tools/platform/cli/src/programs.rs](../../../tools/platform/cli/src/programs.rs) |
| `cli_command` | `pub table_elements: u64,` | [tools/platform/cli/src/programs.rs](../../../tools/platform/cli/src/programs.rs) |
| `cli_command` | `Deploy {` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `artifact: PathBuf,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `interface: Option<PathBuf>,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `previous_state_root: String,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `upgrade_authority: Option<String>,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Discover { program_id: String },` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Interface(ProgramInterfaceCommand),` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Publish {` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Registry(RegistryCommand),` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Get { program_id: String },` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `MirrorSource {` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `archive: PathBuf,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `plan: PathBuf,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `source_uri: String,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `VerifySource {` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `source_digest: String,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Simulate(ProgramExecutionArgs),` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Upgrade {` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `allow_breaking_interface: bool,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `clear_interface: bool,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `migration_hook: Option<PathBuf>,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `old_hash: String,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `WindDown(ProgramWindDownCommand),` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Deprecate {` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `deadline_batch: u64,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `exit_program: String,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Exit {` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `account: String,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Route {` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `asset: String,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `destination: String,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `seed: String,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Tombstone {` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Receipt(ReceiptCommand),` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Get { id: String },` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Verify(VerifyReceiptArgs),` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `batch_id: String,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `receipt: PathBuf,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `resulting_state_root: String,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `sequencer_public_key: String,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Register(register::RegisterArgs),` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `key: Option<String>,` | [tools/platform/cli/src/register.rs](../../../tools/platform/cli/src/register.rs) |
| `cli_command` | `rpc: Option<String>,` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Token(wallet::TokenCommand),` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Burn {` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `amount: String,` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `asset: String,` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `fee_limit: String,` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `key: Option<String>,` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `receipt_policy: std::path::PathBuf,` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `timeout_seconds: u64,` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `wait: Commitment,` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `Create {` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `decimals: u8,` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `name: String,` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `salt: String,` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `supply_cap: String,` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `symbol: String,` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `Info { asset: String },` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `List {` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `cursor: Option<String>,` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `limit: Option<u16>,` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `Mint {` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `to: String,` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `Transfer(TransferArgs),` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `fee_limit: Option<String>,` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `receipt_policy: Option<std::path::PathBuf>,` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `Wallet(wallet::WalletCommand),` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `Balance {` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `asset: Option<String>,` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `did: Option<String>,` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `Derive(crate::wallet_derive::DeriveArgs),` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `bind: bool,` | [tools/platform/cli/src/wallet_derive.rs](../../../tools/platform/cli/src/wallet_derive.rs) |
| `cli_command` | `export_private_keys: Option<PathBuf>,` | [tools/platform/cli/src/wallet_derive.rs](../../../tools/platform/cli/src/wallet_derive.rs) |
| `cli_command` | `index: u32,` | [tools/platform/cli/src/wallet_derive.rs](../../../tools/platform/cli/src/wallet_derive.rs) |
| `cli_command` | `mnemonic_file: Option<PathBuf>,` | [tools/platform/cli/src/wallet_derive.rs](../../../tools/platform/cli/src/wallet_derive.rs) |
| `cli_command` | `passphrase_file: Option<PathBuf>,` | [tools/platform/cli/src/wallet_derive.rs](../../../tools/platform/cli/src/wallet_derive.rs) |
| `cli_command` | `History {` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `Import {` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `List,` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `OpenAccount {` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `Receipt {` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `Send(TransferArgs),` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `Watch {` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `account_id: Option<String>,` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `topic: SubscriptionTopic,` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `cli_command` | `Workspace(workspace::WorkspaceArgs),` | [tools/platform/cli/src/main.rs](../../../tools/platform/cli/src/main.rs) |
| `cli_command` | `All(RunArgs),` | [tools/platform/cli/src/workspace.rs](../../../tools/platform/cli/src/workspace.rs) |
| `cli_command` | `all: bool,` | [tools/platform/cli/src/workspace.rs](../../../tools/platform/cli/src/workspace.rs) |
| `cli_command` | `allow_production: bool,` | [tools/platform/cli/src/workspace.rs](../../../tools/platform/cli/src/workspace.rs) |
| `cli_command` | `ci: bool,` | [tools/platform/cli/src/workspace.rs](../../../tools/platform/cli/src/workspace.rs) |
| `cli_command` | `dry_run: bool,` | [tools/platform/cli/src/workspace.rs](../../../tools/platform/cli/src/workspace.rs) |
| `cli_command` | `environment: Option<String>,` | [tools/platform/cli/src/workspace.rs](../../../tools/platform/cli/src/workspace.rs) |
| `cli_command` | `fail_fast: bool,` | [tools/platform/cli/src/workspace.rs](../../../tools/platform/cli/src/workspace.rs) |
| `cli_command` | `modules: Vec<String>,` | [tools/platform/cli/src/workspace.rs](../../../tools/platform/cli/src/workspace.rs) |
| `cli_command` | `yes: bool,` | [tools/platform/cli/src/workspace.rs](../../../tools/platform/cli/src/workspace.rs) |
| `cli_command` | `Build(RunArgs),` | [tools/platform/cli/src/workspace.rs](../../../tools/platform/cli/src/workspace.rs) |
| `cli_command` | `Doctor(SelectionArgs),` | [tools/platform/cli/src/workspace.rs](../../../tools/platform/cli/src/workspace.rs) |
| `cli_command` | `Install(RunArgs),` | [tools/platform/cli/src/workspace.rs](../../../tools/platform/cli/src/workspace.rs) |
| `cli_command` | `Modules,` | [tools/platform/cli/src/workspace.rs](../../../tools/platform/cli/src/workspace.rs) |
| `cli_command` | `Test(RunArgs),` | [tools/platform/cli/src/workspace.rs](../../../tools/platform/cli/src/workspace.rs) |

## cli-config-keys

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `pub current_environment: String,` | [tools/platform/cli/src/config.rs](../../../tools/platform/cli/src/config.rs) |
| `config_key` | `pub default_key: Option<String>,` | [tools/platform/cli/src/config.rs](../../../tools/platform/cli/src/config.rs) |
| `config_key` | `pub environments: BTreeMap<String, Environment>,` | [tools/platform/cli/src/config.rs](../../../tools/platform/cli/src/config.rs) |
| `config_key` | `pub keys: BTreeMap<String, KeyMetadata>,` | [tools/platform/cli/src/config.rs](../../../tools/platform/cli/src/config.rs) |
| `config_key` | `version: u16,` | [tools/platform/cli/src/config.rs](../../../tools/platform/cli/src/config.rs) |
| `config_key` | `pub endpoint: String,` | [tools/platform/cli/src/config.rs](../../../tools/platform/cli/src/config.rs) |
| `config_key` | `pub network_id: u32,` | [tools/platform/cli/src/config.rs](../../../tools/platform/cli/src/config.rs) |
| `config_key` | `pub sequencer_trust_anchor: Option<String>,` | [tools/platform/cli/src/config.rs](../../../tools/platform/cli/src/config.rs) |
| `config_key` | `pub did: String,` | [tools/platform/cli/src/config.rs](../../../tools/platform/cli/src/config.rs) |
| `config_key` | `pub public_key: String,` | [tools/platform/cli/src/config.rs](../../../tools/platform/cli/src/config.rs) |
| `config_key` | `checkpoint_context_digest: Option<String>,` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `config_key` | `first_batch: u64,` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `config_key` | `last_batch: u64,` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `config_key` | `network_id: u32,` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `config_key` | `protocol_version: u16,` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `config_key` | `sequencer_id: String,` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |
| `config_key` | `sequencer_key: String,` | [tools/platform/cli/src/wallet.rs](../../../tools/platform/cli/src/wallet.rs) |

## cli-env-vars

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `CARGO` | [tools/platform/cli/src/programs.rs](../../../tools/platform/cli/src/programs.rs) |
| `env_var` | `HOME` | [tools/platform/cli/src/config.rs](../../../tools/platform/cli/src/config.rs) |
| `env_var` | `LAYERX_API_TOKEN` | [tools/platform/cli/src/a2a.rs](../../../tools/platform/cli/src/a2a.rs) |
| `env_var` | `LAYERX_AUTH_TOKEN` | [tools/platform/cli/src/a2a.rs](../../../tools/platform/cli/src/a2a.rs) |
| `env_var` | `LAYERX_BETA_REGISTRY_PUBLICATION_KEY_FILE` | [tools/platform/cli/src/programs.rs](../../../tools/platform/cli/src/programs.rs) |
| `env_var` | `LAYERX_BETA_REGISTRY_PUBLICATION_TOKEN_FILE` | [tools/platform/cli/src/programs.rs](../../../tools/platform/cli/src/programs.rs) |
| `env_var` | `LAYERX_BETA_REGISTRY_URL` | [tools/platform/cli/src/programs.rs](../../../tools/platform/cli/src/programs.rs) |
| `env_var` | `LAYERX_CLIENT_CERTIFICATE_FILE` | [tools/platform/cli/src/http.rs](../../../tools/platform/cli/src/http.rs) |
| `env_var` | `LAYERX_CLIENT_PRIVATE_KEY_FILE` | [tools/platform/cli/src/http.rs](../../../tools/platform/cli/src/http.rs) |
| `env_var` | `LAYERX_CONFIG` | [tools/platform/cli/src/config.rs](../../../tools/platform/cli/src/config.rs) |
| `env_var` | `LAYERX_CREDENTIAL_PASSPHRASE` | [tools/platform/cli/src/file_store.rs](../../../tools/platform/cli/src/file_store.rs) |
| `env_var` | `LAYERX_CREDENTIAL_STORE` | [tools/platform/cli/src/credential.rs](../../../tools/platform/cli/src/credential.rs) |
| `env_var` | `LAYERX_ENDPOINT` | [tools/platform/cli/src/workspace.rs](../../../tools/platform/cli/src/workspace.rs) |
| `env_var` | `LAYERX_ENVIRONMENT` | [tools/platform/cli/src/workspace.rs](../../../tools/platform/cli/src/workspace.rs) |
| `env_var` | `LAYERX_GATEWAY_KEY_ID` | [tools/platform/cli/src/toolset.rs](../../../tools/platform/cli/src/toolset.rs) |
| `env_var` | `LAYERX_INSTALL_ROOT` | [tools/platform/cli/src/install/mod.rs](../../../tools/platform/cli/src/install/mod.rs) |
| `env_var` | `LAYERX_NETWORK_ID` | [tools/platform/cli/src/workspace.rs](../../../tools/platform/cli/src/workspace.rs) |
| `env_var` | `LAYERX_PROGRAM_SDK` | [tools/platform/cli/src/scaffold.rs](../../../tools/platform/cli/src/scaffold.rs) |
| `env_var` | `LAYERX_REPO_ROOT` | [tools/platform/cli/src/workspace.rs](../../../tools/platform/cli/src/workspace.rs) |
| `env_var` | `LAYERX_TOKEN` | [tools/platform/cli/src/a2a.rs](../../../tools/platform/cli/src/a2a.rs) |
| `env_var` | `NO_COLOR` | [tools/platform/cli/src/workspace.rs](../../../tools/platform/cli/src/workspace.rs) |
| `env_var` | `PATH` | [tools/platform/cli/src/workspace.rs](../../../tools/platform/cli/src/workspace.rs) |
| `env_var` | `PATHEXT` | [tools/platform/cli/src/workspace.rs](../../../tools/platform/cli/src/workspace.rs) |
| `env_var` | `XDG_CONFIG_HOME` | [tools/platform/cli/src/config.rs](../../../tools/platform/cli/src/config.rs) |

## cli-error-codes

| Kind | Declaration | Source |
|---|---|---|
| `error_code` | `return failure(identifier, -32001, "the task was not found");` | [tools/platform/cli/src/a2a.rs](../../../tools/platform/cli/src/a2a.rs) |
| `error_code` | `return failure(identifier, -32600, "the message did not name a method");` | [tools/platform/cli/src/a2a.rs](../../../tools/platform/cli/src/a2a.rs) |
| `error_code` | `_ => failure(identifier, -32601, "the method is not implemented"),` | [tools/platform/cli/src/a2a.rs](../../../tools/platform/cli/src/a2a.rs) |
| `error_code` | `return failure(identifier, -32602, "the request carried no message");` | [tools/platform/cli/src/a2a.rs](../../../tools/platform/cli/src/a2a.rs) |
| `error_code` | `return failure(identifier, -32603, "the task ledger is unavailable");` | [tools/platform/cli/src/a2a.rs](../../../tools/platform/cli/src/a2a.rs) |
| `error_code` | `return failure(Value::Null, -32700, "the message is not valid JSON");` | [tools/platform/cli/src/a2a.rs](../../../tools/platform/cli/src/a2a.rs) |
| `error_code` | `"code": "gateway_response_invalid",` | [tools/platform/cli/src/http.rs](../../../tools/platform/cli/src/http.rs) |
| `error_code` | `"code": "gateway_response_unreadable",` | [tools/platform/cli/src/http.rs](../../../tools/platform/cli/src/http.rs) |
| `error_code` | `"failure": {"code": "gateway_transport_unavailable", "detail": error.to_string()},` | [tools/platform/cli/src/http.rs](../../../tools/platform/cli/src/http.rs) |

## cli-file-formats

| Kind | Declaration | Source |
|---|---|---|
| `file_format` | `const HEADER: &[u8; 8] = b"LXVAULT1";` | [tools/platform/cli/src/file_store.rs](../../../tools/platform/cli/src/file_store.rs) |

## cli-http-routes

| Kind | Declaration | Source |
|---|---|---|
| `http_route` | `const CARD_ROUTES: [&str; 2] = ["/.well-known/agent-card.json", "/.well-known/agent.json"];` | [tools/platform/cli/src/a2a.rs](../../../tools/platform/cli/src/a2a.rs) |
| `http_route` | `"POST" if request.path == "/" => {` | [tools/platform/cli/src/a2a.rs](../../../tools/platform/cli/src/a2a.rs) |

## cli-jsonrpc-methods

| Kind | Declaration | Source |
|---|---|---|
| `jsonrpc_method` | `"message/send" => send(runtime, tools, tasks, identifier, &parameters),` | [tools/platform/cli/src/a2a.rs](../../../tools/platform/cli/src/a2a.rs) |
| `jsonrpc_method` | `"message/stream" \| "tasks/resubscribe" \| "tasks/pushNotificationConfig/set" => failure(` | [tools/platform/cli/src/a2a.rs](../../../tools/platform/cli/src/a2a.rs) |
| `jsonrpc_method` | `"tasks/cancel" => cancel(tasks, identifier, &parameters),` | [tools/platform/cli/src/a2a.rs](../../../tools/platform/cli/src/a2a.rs) |
| `jsonrpc_method` | `"tasks/get" => fetch(tasks, identifier, &parameters),` | [tools/platform/cli/src/a2a.rs](../../../tools/platform/cli/src/a2a.rs) |

## cli-rust-pub-items

| Kind | Declaration | Source |
|---|---|---|
| `rust_pub_item` | `pub struct Client` | [tools/platform/cli/src/http.rs](../../../tools/platform/cli/src/http.rs) |
| `rust_pub_item` | `pub fn delete(&self, path: &str) -> Result<Value, String>` | [tools/platform/cli/src/http.rs](../../../tools/platform/cli/src/http.rs) |
| `rust_pub_item` | `pub fn get(&self, path: &str) -> Result<Value, String>` | [tools/platform/cli/src/http.rs](../../../tools/platform/cli/src/http.rs) |
| `rust_pub_item` | `pub fn get_with_body(&self, path: &str, selector: &Value) -> Result<Value, String>` | [tools/platform/cli/src/http.rs](../../../tools/platform/cli/src/http.rs) |
| `rust_pub_item` | `pub fn new_gateway(endpoint: &str, credential: Zeroizing<String>) -> Result<Self, String>` | [tools/platform/cli/src/http.rs](../../../tools/platform/cli/src/http.rs) |
| `rust_pub_item` | `pub fn new(endpoint: &str, token: Option<Zeroizing<String>>) -> Result<Self, String>` | [tools/platform/cli/src/http.rs](../../../tools/platform/cli/src/http.rs) |
| `rust_pub_item` | `pub fn post(` | [tools/platform/cli/src/http.rs](../../../tools/platform/cli/src/http.rs) |
| `rust_pub_item` | `pub fn post_publication(` | [tools/platform/cli/src/http.rs](../../../tools/platform/cli/src/http.rs) |
| `rust_pub_item` | `pub fn post_sensitive<T: DeserializeOwned>(` | [tools/platform/cli/src/http.rs](../../../tools/platform/cli/src/http.rs) |
| `rust_pub_item` | `pub fn post_stateful(` | [tools/platform/cli/src/http.rs](../../../tools/platform/cli/src/http.rs) |
| `rust_pub_item` | `pub fn validate_idempotency_key(value: &str) -> Result<(), String>` | [tools/platform/cli/src/http.rs](../../../tools/platform/cli/src/http.rs) |
| `rust_pub_item` | `pub fn validate_resource_id(value: &str, name: &str) -> Result<(), String>` | [tools/platform/cli/src/http.rs](../../../tools/platform/cli/src/http.rs) |
| `rust_pub_item` | `pub fn call(&self, method: &str, params: &Value) -> Result<Value, String>` | [tools/platform/cli/src/rpc.rs](../../../tools/platform/cli/src/rpc.rs) |
| `rust_pub_item` | `pub fn decode_response(method: &str, response: &Value) -> Result<Value, String>` | [tools/platform/cli/src/rpc.rs](../../../tools/platform/cli/src/rpc.rs) |
| `rust_pub_item` | `pub fn new(url: &str, credential: Option<zeroize::Zeroizing<String>>) -> Result<Self, String>` | [tools/platform/cli/src/rpc.rs](../../../tools/platform/cli/src/rpc.rs) |
| `rust_pub_item` | `pub fn new_session(url: &str, session: zeroize::Zeroizing<String>) -> Result<Self, String>` | [tools/platform/cli/src/rpc.rs](../../../tools/platform/cli/src/rpc.rs) |
| `rust_pub_item` | `pub fn request(method: &str, params: &Value) -> Result<Value, String>` | [tools/platform/cli/src/rpc.rs](../../../tools/platform/cli/src/rpc.rs) |
| `rust_pub_item` | `pub struct RpcClient` | [tools/platform/cli/src/rpc.rs](../../../tools/platform/cli/src/rpc.rs) |
| `rust_pub_item` | `pub fn subscribe(&self, params: &Value, timeout: std::time::Duration) -> Result<Value, String>` | [tools/platform/cli/src/rpc.rs](../../../tools/platform/cli/src/rpc.rs) |
| `rust_pub_item` | `pub enum AssetOperation<'a>` | [tools/platform/cli/src/wallet_encoding.rs](../../../tools/platform/cli/src/wallet_encoding.rs) |
| `rust_pub_item` | `pub struct AssetRegistration<'a>` | [tools/platform/cli/src/wallet_encoding.rs](../../../tools/platform/cli/src/wallet_encoding.rs) |
| `rust_pub_item` | `pub fn encode(&self) -> Result<Vec<u8>, String>` | [tools/platform/cli/src/wallet_encoding.rs](../../../tools/platform/cli/src/wallet_encoding.rs) |
| `rust_pub_item` | `pub fn native_issuer_id(did: &str) -> Result<[u8; 32], String>` | [tools/platform/cli/src/wallet_encoding.rs](../../../tools/platform/cli/src/wallet_encoding.rs) |
| `rust_pub_item` | `pub const fn ordinal(&self) -> u16` | [tools/platform/cli/src/wallet_encoding.rs](../../../tools/platform/cli/src/wallet_encoding.rs) |
| `rust_pub_item` | `pub struct Authorization` | [tools/platform/cli/src/wallet_send.rs](../../../tools/platform/cli/src/wallet_send.rs) |
| `rust_pub_item` | `pub fn authorization_message(&self) -> Result<Vec<u8>, String>` | [tools/platform/cli/src/wallet_send.rs](../../../tools/platform/cli/src/wallet_send.rs) |
| `rust_pub_item` | `pub fn disclosure(&self) -> Value` | [tools/platform/cli/src/wallet_send.rs](../../../tools/platform/cli/src/wallet_send.rs) |
| `rust_pub_item` | `pub fn encode(&self) -> Result<Vec<u8>, String>` | [tools/platform/cli/src/wallet_send.rs](../../../tools/platform/cli/src/wallet_send.rs) |
| `rust_pub_item` | `pub fn prepare(` | [tools/platform/cli/src/wallet_send.rs](../../../tools/platform/cli/src/wallet_send.rs) |
| `rust_pub_item` | `pub struct Send` | [tools/platform/cli/src/wallet_send.rs](../../../tools/platform/cli/src/wallet_send.rs) |
| `rust_pub_item` | `pub fn confirmation(&self) -> Value` | [tools/platform/cli/src/wallet_signing.rs](../../../tools/platform/cli/src/wallet_signing.rs) |
| `rust_pub_item` | `pub fn from_send(` | [tools/platform/cli/src/wallet_signing.rs](../../../tools/platform/cli/src/wallet_signing.rs) |
| `rust_pub_item` | `pub fn new(` | [tools/platform/cli/src/wallet_signing.rs](../../../tools/platform/cli/src/wallet_signing.rs) |
| `rust_pub_item` | `pub struct PreparedPayment` | [tools/platform/cli/src/wallet_signing.rs](../../../tools/platform/cli/src/wallet_signing.rs) |
| `rust_pub_item` | `pub fn sign(self, key: &EvmSigningKey) -> Result<(Vec<u8>, [u8; 32]), String> {` | [tools/platform/cli/src/wallet_signing.rs](../../../tools/platform/cli/src/wallet_signing.rs) |
| `rust_pub_item` | `pub fn sign_with_id(self, key: &EvmSigningKey) -> Result<(Vec<u8>, [u8; 32]), String> {` | [tools/platform/cli/src/wallet_signing.rs](../../../tools/platform/cli/src/wallet_signing.rs) |
| `rust_pub_item` | `pub struct SigningFacts<'a>` | [tools/platform/cli/src/wallet_signing.rs](../../../tools/platform/cli/src/wallet_signing.rs) |

## emulator-cli-commands

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `"--listen" => {` | [tools/platform/emulator/src/main.rs](../../../tools/platform/emulator/src/main.rs) |
| `cli_command` | `"--network-id" => {` | [tools/platform/emulator/src/main.rs](../../../tools/platform/emulator/src/main.rs) |
| `cli_command` | `if arguments.next().as_deref() != Some("up") {` | [tools/platform/emulator/src/main.rs](../../../tools/platform/emulator/src/main.rs) |
| `cli_command` | `"--prefund" => config.prefunds.push(parse_prefund(&value)?),` | [tools/platform/emulator/src/main.rs](../../../tools/platform/emulator/src/main.rs) |
| `cli_command` | `"--protocol-version" => {` | [tools/platform/emulator/src/main.rs](../../../tools/platform/emulator/src/main.rs) |
| `cli_command` | `"--sequencer-seed-file" => {` | [tools/platform/emulator/src/main.rs](../../../tools/platform/emulator/src/main.rs) |
| `cli_command` | `"--time-ms" => {` | [tools/platform/emulator/src/main.rs](../../../tools/platform/emulator/src/main.rs) |

## emulator-env-vars

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `OPT_LEVEL` | [tools/platform/emulator/build.rs](../../../tools/platform/emulator/build.rs) |
| `env_var` | `OUT_DIR` | [tools/platform/emulator/build.rs](../../../tools/platform/emulator/build.rs) |

## emulator-error-codes

| Kind | Declaration | Source |
|---|---|---|
| `error_code` | `return refusal(trace, 400, "invalid_did", "did is not a canonical DID");` | [tools/platform/emulator/src/main.rs](../../../tools/platform/emulator/src/main.rs) |
| `error_code` | `refusal(trace, 400, "invalid_request", &error)` | [tools/platform/emulator/src/main.rs](../../../tools/platform/emulator/src/main.rs) |
| `error_code` | `Err(error) => return refusal(trace, 400, "invalid_snapshot", &error),` | [tools/platform/emulator/src/main.rs](../../../tools/platform/emulator/src/main.rs) |
| `error_code` | `return "LXP_ERR_UNKNOWN".into();` | [tools/platform/emulator/src/main.rs](../../../tools/platform/emulator/src/main.rs) |
| `error_code` | `return refusal(trace, 404, "not_found", "route does not exist");` | [tools/platform/emulator/src/main.rs](../../../tools/platform/emulator/src/main.rs) |
| `error_code` | `Err(error) => refusal(trace, 503, "snapshot_encoding_failed", &error),` | [tools/platform/emulator/src/main.rs](../../../tools/platform/emulator/src/main.rs) |

## emulator-file-formats

| Kind | Declaration | Source |
|---|---|---|
| `file_format` | `const RECOVERY_SNAPSHOT_MAGIC: &[u8; 8] = b"LXEMR001";` | [tools/platform/emulator/src/main.rs](../../../tools/platform/emulator/src/main.rs) |

## emulator-http-routes

| Kind | Declaration | Source |
|---|---|---|
| `http_route` | `("GET", "/__emulator/snapshot") => export_snapshot(emulator, trace),` | [tools/platform/emulator/src/main.rs](../../../tools/platform/emulator/src/main.rs) |
| `http_route` | `("GET", "/healthz") => health(emulator, trace),` | [tools/platform/emulator/src/main.rs](../../../tools/platform/emulator/src/main.rs) |
| `http_route` | `("GET", path) if path.starts_with("/v1/dids/") && path.ends_with("/sequence") => {` | [tools/platform/emulator/src/main.rs](../../../tools/platform/emulator/src/main.rs) |
| `http_route` | `("GET", path) if path.starts_with("/v1/journeys/") => {` | [tools/platform/emulator/src/main.rs](../../../tools/platform/emulator/src/main.rs) |
| `http_route` | `("GET", path) if path.starts_with("/v1/receipts/") => {` | [tools/platform/emulator/src/main.rs](../../../tools/platform/emulator/src/main.rs) |
| `http_route` | `("GET", "/v1/sequencer") => sequencer_identity(emulator, trace),` | [tools/platform/emulator/src/main.rs](../../../tools/platform/emulator/src/main.rs) |
| `http_route` | `("GET", "/v1/state") => inspect(emulator, trace),` | [tools/platform/emulator/src/main.rs](../../../tools/platform/emulator/src/main.rs) |
| `http_route` | `("POST", "/__emulator/accounts/prefund") => prefund(emulator, request, trace),` | [tools/platform/emulator/src/main.rs](../../../tools/platform/emulator/src/main.rs) |
| `http_route` | `("POST", "/__emulator/faults") => inject_fault(emulator, request, trace),` | [tools/platform/emulator/src/main.rs](../../../tools/platform/emulator/src/main.rs) |
| `http_route` | `("POST", "/__emulator/time/advance") => update_time(emulator, request, trace, true),` | [tools/platform/emulator/src/main.rs](../../../tools/platform/emulator/src/main.rs) |
| `http_route` | `("POST", "/__emulator/time/set") => update_time(emulator, request, trace, false),` | [tools/platform/emulator/src/main.rs](../../../tools/platform/emulator/src/main.rs) |
| `http_route` | `("PUT", "/__emulator/snapshot") => import_snapshot(emulator, &request.body, trace),` | [tools/platform/emulator/src/main.rs](../../../tools/platform/emulator/src/main.rs) |

## emulator-rust-pub-items

| Kind | Declaration | Source |
|---|---|---|
| `rust_pub_item` | `pub fn run(arguments: impl IntoIterator<Item = String>) -> Result<(), String>` | [tools/platform/emulator/src/main.rs](../../../tools/platform/emulator/src/main.rs) |

## release-cli-commands

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `fn run() -> Result<(), String> {` | [tools/platform/release/src/main.rs](../../../tools/platform/release/src/main.rs) |
| `cli_command` | `if arguments.len() > 3 \|\| !matches!(mode, "--check" \| "--plan") {` | [tools/platform/release/src/main.rs](../../../tools/platform/release/src/main.rs) |
| `cli_command` | `Some("manifest") => run_manifest(&arguments[1..]),` | [tools/platform/release/src/main.rs](../../../tools/platform/release/src/main.rs) |
| `cli_command` | `"--output",` | [tools/platform/release/src/main.rs](../../../tools/platform/release/src/main.rs) |
| `cli_command` | `"--registries",` | [tools/platform/release/src/main.rs](../../../tools/platform/release/src/main.rs) |
| `cli_command` | `"--release-dir",` | [tools/platform/release/src/main.rs](../../../tools/platform/release/src/main.rs) |
| `cli_command` | `"--rollback-version",` | [tools/platform/release/src/main.rs](../../../tools/platform/release/src/main.rs) |
| `cli_command` | `"--workflow",` | [tools/platform/release/src/main.rs](../../../tools/platform/release/src/main.rs) |
| `cli_command` | `Some("verify") => run_verify(&arguments[1..]),` | [tools/platform/release/src/main.rs](../../../tools/platform/release/src/main.rs) |
| `cli_command` | `const FETCH_FLAG: &str = "--fetch";` | [tools/platform/release/src/main.rs](../../../tools/platform/release/src/main.rs) |
| `cli_command` | `&["--manifest", "--from", FETCH_FLAG, "--source-revision"],` | [tools/platform/release/src/main.rs](../../../tools/platform/release/src/main.rs) |

## release-config-keys

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `pub artifacts: Vec<ArtifactManifestEntry>,` | [tools/platform/release/src/main.rs](../../../tools/platform/release/src/main.rs) |
| `config_key` | `pub images: Vec<ImageManifestEntry>,` | [tools/platform/release/src/main.rs](../../../tools/platform/release/src/main.rs) |
| `config_key` | `pub rollback_version: Option<String>,` | [tools/platform/release/src/main.rs](../../../tools/platform/release/src/main.rs) |
| `config_key` | `pub schema: String,` | [tools/platform/release/src/main.rs](../../../tools/platform/release/src/main.rs) |
| `config_key` | `pub source_digest: String,` | [tools/platform/release/src/main.rs](../../../tools/platform/release/src/main.rs) |
| `config_key` | `pub source_revision: String,` | [tools/platform/release/src/main.rs](../../../tools/platform/release/src/main.rs) |
| `config_key` | `pub tag: String,` | [tools/platform/release/src/main.rs](../../../tools/platform/release/src/main.rs) |
| `config_key` | `pub version: String,` | [tools/platform/release/src/main.rs](../../../tools/platform/release/src/main.rs) |

## release-env-vars

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `LAYERX_RELEASE_DISTRIBUTION` | [tools/platform/release/src/main.rs](../../../tools/platform/release/src/main.rs) |
| `env_var` | `LAYERX_RELEASE_PACKAGES` | [tools/platform/release/src/main.rs](../../../tools/platform/release/src/main.rs) |
| `env_var` | `LAYERX_RELEASE_REGISTRY` | [tools/platform/release/src/main.rs](../../../tools/platform/release/src/main.rs) |

## release-file-formats

| Kind | Declaration | Source |
|---|---|---|
| `file_format` | `const ARTIFACT_MANIFEST_SCHEMA: &str = "layerx/artifact-manifest/1";` | [tools/platform/release/src/main.rs](../../../tools/platform/release/src/main.rs) |
