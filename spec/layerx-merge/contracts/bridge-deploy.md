# Bridge deploy contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## app-env

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cli-bridge-paxeer-bridge-app-env-flag-check-j55</code> | <code>cli_command</code> | <code>new</code> | <code>services/bridge/deploy/cmd/paxeer-bridge-app-env/main.go</code> (planned) | <code>paxeer-bridge-app-env --check &lt;env-file&gt; &lt;records-directory&gt;</code> |
| <code>interface.14.cli-bridge-paxeer-bridge-app-env-j55</code> | <code>cli_command</code> | <code>new</code> | <code>services/bridge/deploy/cmd/paxeer-bridge-app-env/main.go</code> (planned) | <code>paxeer-bridge-app-env &lt;records-directory&gt;</code> |
| <code>interface.14.go-bridge-appenv-check-j55</code> | <code>go_exported</code> | <code>new</code> | <code>services/bridge/deploy/appenv/appenv.go</code> (planned) | <code>func Check(envPath string, records []Record) error</code> |
| <code>interface.14.go-bridge-appenv-envlines-j55</code> | <code>go_exported</code> | <code>new</code> | <code>services/bridge/deploy/appenv/appenv.go</code> (planned) | <code>func EnvLines(records []Record) []string</code> |
| <code>interface.14.go-bridge-appenv-envnames-j55</code> | <code>go_exported</code> | <code>new</code> | <code>services/bridge/deploy/appenv/appenv.go</code> (planned) | <code>func EnvNames() []string</code> |
| <code>interface.14.go-bridge-appenv-loadrecord-j55</code> | <code>go_exported</code> | <code>new</code> | <code>services/bridge/deploy/appenv/appenv.go</code> (planned) | <code>func LoadRecord(path string) (Record, error)</code> |
| <code>interface.14.go-bridge-appenv-record-j55</code> | <code>go_exported</code> | <code>new</code> | <code>services/bridge/deploy/appenv/appenv.go</code> (planned) | <code>type Record struct { Chain string; ChainID uint64; Kind string; Vault string; ProgramID string }</code> |
| <code>interface.14.go-bridge-appenv-run-j55</code> | <code>go_exported</code> | <code>new</code> | <code>services/bridge/deploy/appenv/appenv.go</code> (planned) | <code>func Run(args []string, w io.Writer) error</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cli-bridge-paxeer-bridge-app-env-flag-check-j55</code> | notes | created by task 14.53.5; exits 1 naming each differing variable |
| <code>interface.14.cli-bridge-paxeer-bridge-app-env-j55</code> | notes | created by task 14.53.2; prints the bridge app NEXT_PUBLIC_BRIDGE_* lines |
| <code>interface.14.go-bridge-appenv-check-j55</code> | notes | created by task 14.53.5; drift check of an app env file |
| <code>interface.14.go-bridge-appenv-envlines-j55</code> | notes | created by task 14.53.1; app environment lines in registry order |
| <code>interface.14.go-bridge-appenv-envnames-j55</code> | notes | created by task 14.53.6; every app variable name appenv writes |
| <code>interface.14.go-bridge-appenv-loadrecord-j55</code> | notes | created by task 14.53.1; reads and checks one deployment record |
| <code>interface.14.go-bridge-appenv-record-j55</code> | notes | created by task 14.53.1; deployment record fields the app needs |
| <code>interface.14.go-bridge-appenv-run-j55</code> | notes | created by task 14.53.2; command body of paxeer-bridge-app-env |

Contract context <code>contract_context.14.bridge-deploy</code>:

| Field | Frozen rule |
|---|---|
| interface | services/bridge/deploy: deploy-evm-chain.sh, deploy-solana-program.sh, verify-evm-chain.sh and checklist.sh write and check deployment records; paxeer-bridge-proposals renders governance proposals; paxeer-bridge-app-env renders and checks the app env. |
| storage | Deployment records &lt;chain&gt;.json under PAXEER_BRIDGE_DEPLOYMENT_RECORD_DIR; attestors.json manifest; no chain state written by the tooling except the deployments themselves. |
| errors | Each script exits 1 naming the failing file, variable or field; appenv.LoadRecord refuses a chain_id that differs from the registry and a zero vault. |
| events | None. |
| gas | Forge and the Solana CLI estimate gas; the scripts set none. |
| activation | Run by an operator per chain; the app env is rendered after the records exist. |
| invariants | The app env names exactly the vaults and program id of the deployment records; chain ids match chainconfig.Chains. |
| pause | Not applicable; the tooling does not pause contracts. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.14.503.1</code> | req | 14503.1 |
| <code>acceptance.14.503.1</code> | scenario | Given the record at services/bridge/deploy/tests/fixtures/records/ethereum.json, when LoadRecord and EnvLines run, then the output is NEXT_PUBLIC_BRIDGE_VAULT_ETHEREUM=0x7a3e5c81b04d296f8e1a7c35d92b6f04e8c1a37d |
| <code>acceptance.14.503.1</code> | test | services/bridge/deploy/appenv/appenv_test.go::TestEnvLinesFromEVMRecords (planned) |
| <code>acceptance.14.503.1</code> | qualification | UNRUN |
| <code>req.14503</code> | ac_1 | EnvLines([]Record) returns NEXT_PUBLIC_BRIDGE_VAULT_&lt;CHAIN&gt;=&lt;vault&gt; for each EVM record in chainconfig.Chains order (services/bridge/deploy/chainconfig/config.go:126), the names the app reads at apps/bridge/src/lib/bridge/config.ts:38-45. |
| <code>acceptance.14.503.2</code> | req | 14503.2 |
| <code>acceptance.14.503.2</code> | scenario | Given a copy of ethereum.json with chain_id 2, when LoadRecord runs, then it returns an error naming chain_id |
| <code>acceptance.14.503.2</code> | test | services/bridge/deploy/appenv/appenv_test.go::TestRefusesARecordWithAnotherChainID (planned) |
| <code>acceptance.14.503.2</code> | qualification | UNRUN |
| <code>req.14503</code> | ac_2 | LoadRecord refuses a record whose chain_id differs from the registry chain id of its chain name (services/bridge/deploy/chainconfig/config.go:108-118), naming the file and the field. |
| <code>acceptance.14.503.3</code> | req | 14503.3 |
| <code>acceptance.14.503.3</code> | scenario | Given a copy of ethereum.json with vault 0x0000000000000000000000000000000000000000, when LoadRecord runs, then it returns an error naming vault |
| <code>acceptance.14.503.3</code> | test | services/bridge/deploy/appenv/appenv_test.go::TestRefusesAZeroVault (planned) |
| <code>acceptance.14.503.3</code> | qualification | UNRUN |
| <code>req.14503</code> | ac_3 | LoadRecord refuses an EVM record whose vault is the zero address or not a 20-byte hex address (services/bridge/deploy/deploy-evm-chain.sh:330). |
| <code>acceptance.14.504.1</code> | req | 14504.1 |
| <code>acceptance.14.504.1</code> | scenario | Given a temporary directory with base.json and ethereum.json records, when Run is called with that directory, then the output lists ethereum before base |
| <code>acceptance.14.504.1</code> | test | services/bridge/deploy/appenv/appenv_test.go::TestRunWritesEveryRecordInChainOrder (planned) |
| <code>acceptance.14.504.1</code> | qualification | UNRUN |
| <code>req.14504</code> | ac_1 | Run([]string{dir}, w) writes one line per record found in dir in chainconfig.Chains order (services/bridge/deploy/chainconfig/config.go:126), whatever the file order. |
| <code>acceptance.14.504.2</code> | req | 14504.2 |
| <code>acceptance.14.504.2</code> | scenario | Given an empty temporary directory, when Run is called with it, then it returns an error naming the directory and the writer is empty |
| <code>acceptance.14.504.2</code> | test | services/bridge/deploy/appenv/appenv_test.go::TestRunRefusesAnEmptyDirectory (planned) |
| <code>acceptance.14.504.2</code> | qualification | UNRUN |
| <code>req.14504</code> | ac_2 | Run refuses a directory holding no *.json record and writes nothing (services/bridge/deploy/deploy-evm-chain.sh:12 writes records into PAXEER_BRIDGE_DEPLOYMENT_RECORD_DIR). |
| <code>acceptance.14.505.1</code> | req | 14505.1 |
| <code>acceptance.14.505.1</code> | scenario | Given services/bridge/deploy/tests/fixtures/records/solana.json, when LoadRecord and EnvLines run, then the output is NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID followed by the record's program_id |
| <code>acceptance.14.505.1</code> | test | services/bridge/deploy/appenv/appenv_test.go::TestSolanaRecordEmitsTheProgramID (planned) |
| <code>acceptance.14.505.1</code> | qualification | UNRUN |
| <code>req.14505</code> | ac_1 | EnvLines emits NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID=&lt;program_id&gt; for the record at services/bridge/deploy/tests/fixtures/records/solana.json:1, the variable readConfig reads (apps/bridge/src/lib/bridge/config.ts:30). |
| <code>acceptance.14.507.1</code> | req | 14507.1 |
| <code>acceptance.14.507.1</code> | scenario | Given an env file with NEXT_PUBLIC_BRIDGE_VAULT_ETHEREUM=0x1111111111111111111111111111111111111111, when Check runs against ethereum.json, then it returns an error naming the variable |
| <code>acceptance.14.507.1</code> | test | services/bridge/deploy/appenv/appenv_test.go::TestCheckReportsAVaultThatDiffers (planned) |
| <code>acceptance.14.507.1</code> | qualification | UNRUN |
| <code>req.14507</code> | ac_1 | Check(envFile, records) returns an error naming NEXT_PUBLIC_BRIDGE_VAULT_ETHEREUM when the env file holds another address than the record at services/bridge/deploy/tests/fixtures/records/ethereum.json:6. |
| <code>acceptance.14.507.2</code> | req | 14507.2 |
| <code>acceptance.14.507.2</code> | scenario | Given an env file produced by EnvLines from the same records, when Check runs, then it returns nil |
| <code>acceptance.14.507.2</code> | test | services/bridge/deploy/appenv/appenv_test.go::TestCheckAcceptsAMatchingEnv (planned) |
| <code>acceptance.14.507.2</code> | qualification | UNRUN |
| <code>req.14507</code> | ac_2 | Check returns nil when every record line is present with the same value, ignoring case of hex digits (services/bridge/deploy/tests/fixtures/records/ethereum.json:6). |
| <code>capability.14.53.1</code> | title | Add an appenv package that turns EVM deployment records into the app's vault variables |
| <code>capability.14.53.1</code> | reqs | 14503.1<br>14503.2<br>14503.3 |
| <code>capability.14.53.2</code> | title | Add the paxeer-bridge-app-env command that writes the app env from a records directory |
| <code>capability.14.53.2</code> | reqs | 14504.1<br>14504.2 |
| <code>capability.14.53.3</code> | title | Emit the Solana program id line from the Solana deployment record |
| <code>capability.14.53.3</code> | reqs | 14505.1 |
| <code>capability.14.53.5</code> | title | Add a --check mode that reports app env vaults differing from the deployment records |
| <code>capability.14.53.5</code> | reqs | 14507.1<br>14507.2 |
| <code>resolution.17.959</code> | title | decision.1445 binds: The app's vault variables come from deployment records |
| <code>resolution.17.959</code> | applies_to | decision.1445<br>task.14.53.1<br>task.14.53.2<br>task.14.53.3<br>task.14.58.1 |
| <code>resolution.17.959</code> | source | decision.1445 in fragments/J55/decision.kvx; tasks from affects |

## chainconfig

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.fmt-bridge-deploy-overlay-j55</code> | <code>file_format</code> | <code>stable</code> | <code>services/bridge/deploy/overlay.jq:1</code> | <code>{chains: {&lt;name&gt;: ChainOverlay}} applied by overlay.jq and chainconfig.Overlay.Apply</code> |
| <code>interface.14.go-bridge-chainconfig-asset-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/config.go:196</code> | <code>type Asset struct</code> |
| <code>interface.14.go-bridge-chainconfig-asset-pertx-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/config.go:206</code> | <code>func (a Asset) PerTx() (*big.Int, error) { return parseAmount(a.PerTxCap) }</code> |
| <code>interface.14.go-bridge-chainconfig-asset-total-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/config.go:209</code> | <code>func (a Asset) Total() (*big.Int, error) { return parseAmount(a.TotalCap) }</code> |
| <code>interface.14.go-bridge-chainconfig-assetcap-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/overlay.go:37</code> | <code>type AssetCap struct</code> |
| <code>interface.14.go-bridge-chainconfig-bigblockchain-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/config.go:123</code> | <code>const BigBlockChain = "hyperevm"</code> |
| <code>interface.14.go-bridge-chainconfig-bigblocks-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/config.go:179</code> | <code>type BigBlocks struct</code> |
| <code>interface.14.go-bridge-chainconfig-chain-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/config.go:99</code> | <code>type Chain struct</code> |
| <code>interface.14.go-bridge-chainconfig-chainbyname-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/config.go:133</code> | <code>func ChainByName(name string) (Chain, bool)</code> |
| <code>interface.14.go-bridge-chainconfig-chainconfig-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/config.go:213</code> | <code>type ChainConfig struct</code> |
| <code>interface.14.go-bridge-chainconfig-chainconfig-nativeasset-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/config.go:237</code> | <code>func (c *ChainConfig) NativeAsset() Asset { return c.Assets[0] }</code> |
| <code>interface.14.go-bridge-chainconfig-chainconfig-requiredeployable-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/validate.go:84</code> | <code>func (c *ChainConfig) RequireDeployable() error</code> |
| <code>interface.14.go-bridge-chainconfig-chainconfig-source-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/config.go:233</code> | <code>func (c *ChainConfig) Source() string { return c.path }</code> |
| <code>interface.14.go-bridge-chainconfig-chainconfig-validate-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/validate.go:21</code> | <code>func (c *ChainConfig) Validate() error</code> |
| <code>interface.14.go-bridge-chainconfig-chainoverlay-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/overlay.go:25</code> | <code>type ChainOverlay struct</code> |
| <code>interface.14.go-bridge-chainconfig-chains-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/config.go:126</code> | <code>func Chains() []Chain</code> |
| <code>interface.14.go-bridge-chainconfig-decodebase58-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/config.go:299</code> | <code>func DecodeBase58(value string) ([]byte, error) { return vectors.DecodeBase58(value) }</code> |
| <code>interface.14.go-bridge-chainconfig-environment-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/config.go:160</code> | <code>type Environment struct</code> |
| <code>interface.14.go-bridge-chainconfig-isplaceholder-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/config.go:284</code> | <code>func IsPlaceholder(value string) bool { return strings.HasPrefix(value, PlaceholderPrefix) }</code> |
| <code>interface.14.go-bridge-chainconfig-kind-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/config.go:40</code> | <code>type Kind string</code> |
| <code>interface.14.go-bridge-chainconfig-kindevm-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/config.go:44</code> | <code>KindEVM Kind = "evm"</code> |
| <code>interface.14.go-bridge-chainconfig-kindsolana-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/config.go:46</code> | <code>KindSolana Kind = "solana"</code> |
| <code>interface.14.go-bridge-chainconfig-load-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/config.go:242</code> | <code>func Load(path string) (*ChainConfig, error)</code> |
| <code>interface.14.go-bridge-chainconfig-loadchain-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/config.go:275</code> | <code>func LoadChain(chainsRoot, name string) (*ChainConfig, error)</code> |
| <code>interface.14.go-bridge-chainconfig-loadoverlay-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/overlay.go:44</code> | <code>func LoadOverlay(path string) (*Overlay, error)</code> |
| <code>interface.14.go-bridge-chainconfig-maxattestors-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/config.go:85</code> | <code>MaxAttestors = 64</code> |
| <code>interface.14.go-bridge-chainconfig-nativecoin-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/config.go:153</code> | <code>type NativeCoin struct</code> |
| <code>interface.14.go-bridge-chainconfig-nativeevmasset-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/config.go:63</code> | <code>NativeEVMAsset = "0x0000000000000000000000000000000000000000"</code> |
| <code>interface.14.go-bridge-chainconfig-overlay-apply-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/overlay.go:70</code> | <code>func (o *Overlay) Apply(c *ChainConfig) error</code> |
| <code>interface.14.go-bridge-chainconfig-overlay-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/overlay.go:18</code> | <code>type Overlay struct</code> |
| <code>interface.14.go-bridge-chainconfig-overlayenv-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/overlay.go:15</code> | <code>const OverlayEnv = "BRIDGE_DEPLOY_OVERLAY"</code> |
| <code>interface.14.go-bridge-chainconfig-path-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/config.go:143</code> | <code>func Path(repoRoot, name string) (string, error)</code> |
| <code>interface.14.go-bridge-chainconfig-placeholderprefix-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/config.go:53</code> | <code>PlaceholderPrefix = "PLACEHOLDER:"</code> |
| <code>interface.14.go-bridge-chainconfig-sidioraassetid-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/config.go:77</code> | <code>SidioraAssetID = "0x21f7b20a555199fa73A238B1a91FD0f549068fEe"</code> |
| <code>interface.14.go-bridge-chainconfig-sidioramint-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/config.go:70</code> | <code>SidioraMint = "5w3wVdJaESaJKyLmStM6Hv9UyUkmZ1b9DLQquAqqpump"</code> |
| <code>interface.14.go-bridge-chainconfig-solanachainid-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/config.go:59</code> | <code>SolanaChainID uint64 = 91600046870081</code> |
| <code>interface.14.go-bridge-chainconfig-solanahandle-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/config.go:289</code> | <code>func SolanaHandle(key string) (string, error)</code> |
| <code>interface.14.go-bridge-chainconfig-solanasection-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/config.go:188</code> | <code>type SolanaSection struct</code> |
| <code>interface.14.go-bridge-chainconfig-solanazeropubkey-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/config.go:81</code> | <code>SolanaZeroPubkey = "11111111111111111111111111111111"</code> |
| <code>interface.14.go-bridge-chainconfig-wrappedsolmint-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/config.go:67</code> | <code>WrappedSolMint = "So11111111111111111111111111111111111111112"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.fmt-bridge-deploy-overlay-j55</code> | notes | private overlay shape; ChainOverlay at services/bridge/deploy/chainconfig/overlay.go:25 |
| <code>interface.14.go-bridge-chainconfig-asset-j55</code> | notes | exported from package chainconfig |
| <code>interface.14.go-bridge-chainconfig-asset-pertx-j55</code> | notes | exported from package chainconfig, method of Asset |
| <code>interface.14.go-bridge-chainconfig-asset-total-j55</code> | notes | exported from package chainconfig, method of Asset |
| <code>interface.14.go-bridge-chainconfig-assetcap-j55</code> | notes | exported from package chainconfig |
| <code>interface.14.go-bridge-chainconfig-bigblockchain-j55</code> | notes | exported from package chainconfig |
| <code>interface.14.go-bridge-chainconfig-bigblocks-j55</code> | notes | exported from package chainconfig |
| <code>interface.14.go-bridge-chainconfig-chain-j55</code> | notes | exported from package chainconfig |
| <code>interface.14.go-bridge-chainconfig-chainbyname-j55</code> | notes | exported from package chainconfig |
| <code>interface.14.go-bridge-chainconfig-chainconfig-j55</code> | notes | exported from package chainconfig |
| <code>interface.14.go-bridge-chainconfig-chainconfig-nativeasset-j55</code> | notes | exported from package chainconfig, method of ChainConfig |
| <code>interface.14.go-bridge-chainconfig-chainconfig-requiredeployable-j55</code> | notes | exported from package chainconfig, method of ChainConfig |
| <code>interface.14.go-bridge-chainconfig-chainconfig-source-j55</code> | notes | exported from package chainconfig, method of ChainConfig |
| <code>interface.14.go-bridge-chainconfig-chainconfig-validate-j55</code> | notes | exported from package chainconfig, method of ChainConfig |
| <code>interface.14.go-bridge-chainconfig-chainoverlay-j55</code> | notes | exported from package chainconfig |
| <code>interface.14.go-bridge-chainconfig-chains-j55</code> | notes | exported from package chainconfig |
| <code>interface.14.go-bridge-chainconfig-decodebase58-j55</code> | notes | exported from package chainconfig |
| <code>interface.14.go-bridge-chainconfig-environment-j55</code> | notes | exported from package chainconfig |
| <code>interface.14.go-bridge-chainconfig-isplaceholder-j55</code> | notes | exported from package chainconfig |
| <code>interface.14.go-bridge-chainconfig-kind-j55</code> | notes | exported from package chainconfig |
| <code>interface.14.go-bridge-chainconfig-kindevm-j55</code> | notes | exported from package chainconfig |
| <code>interface.14.go-bridge-chainconfig-kindsolana-j55</code> | notes | exported from package chainconfig |
| <code>interface.14.go-bridge-chainconfig-load-j55</code> | notes | exported from package chainconfig |
| <code>interface.14.go-bridge-chainconfig-loadchain-j55</code> | notes | exported from package chainconfig |
| <code>interface.14.go-bridge-chainconfig-loadoverlay-j55</code> | notes | exported from package chainconfig |
| <code>interface.14.go-bridge-chainconfig-maxattestors-j55</code> | notes | exported from package chainconfig |
| <code>interface.14.go-bridge-chainconfig-nativecoin-j55</code> | notes | exported from package chainconfig |
| <code>interface.14.go-bridge-chainconfig-nativeevmasset-j55</code> | notes | exported from package chainconfig |
| <code>interface.14.go-bridge-chainconfig-overlay-apply-j55</code> | notes | exported from package chainconfig, method of Overlay |
| <code>interface.14.go-bridge-chainconfig-overlay-j55</code> | notes | exported from package chainconfig |
| <code>interface.14.go-bridge-chainconfig-overlayenv-j55</code> | notes | exported from package chainconfig |
| <code>interface.14.go-bridge-chainconfig-path-j55</code> | notes | exported from package chainconfig |
| <code>interface.14.go-bridge-chainconfig-placeholderprefix-j55</code> | notes | exported from package chainconfig |
| <code>interface.14.go-bridge-chainconfig-sidioraassetid-j55</code> | notes | exported from package chainconfig |
| <code>interface.14.go-bridge-chainconfig-sidioramint-j55</code> | notes | exported from package chainconfig |
| <code>interface.14.go-bridge-chainconfig-solanachainid-j55</code> | notes | exported from package chainconfig |
| <code>interface.14.go-bridge-chainconfig-solanahandle-j55</code> | notes | exported from package chainconfig |
| <code>interface.14.go-bridge-chainconfig-solanasection-j55</code> | notes | exported from package chainconfig |
| <code>interface.14.go-bridge-chainconfig-solanazeropubkey-j55</code> | notes | exported from package chainconfig |
| <code>interface.14.go-bridge-chainconfig-wrappedsolmint-j55</code> | notes | exported from package chainconfig |

## ci

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cli-bridge-tests-checklist-check-j55</code> | <code>cli_command</code> | <code>stable</code> | <code>services/bridge/deploy/tests/checklist-check.sh:16</code> | <code>bash services/bridge/deploy/tests/checklist-check.sh</code> |
| <code>interface.14.cli-bridge-tests-deploy-scripts-check-j55</code> | <code>cli_command</code> | <code>stable</code> | <code>services/bridge/deploy/tests/deploy-scripts-check.sh:37</code> | <code>bash services/bridge/deploy/tests/deploy-scripts-check.sh</code> |
| <code>interface.14.cli-bridge-tests-docs-check-j55</code> | <code>cli_command</code> | <code>stable</code> | <code>services/bridge/deploy/tests/docs-check.sh:23</code> | <code>bash services/bridge/deploy/tests/docs-check.sh</code> |
| <code>interface.14.cli-bridge-tests-evm-dry-run-check-j55</code> | <code>cli_command</code> | <code>stable</code> | <code>services/bridge/deploy/tests/evm-dry-run-check.sh:47</code> | <code>bash services/bridge/deploy/tests/evm-dry-run-check.sh</code> |
| <code>interface.14.cli-bridge-tests-solana-dry-run-check-j55</code> | <code>cli_command</code> | <code>stable</code> | <code>services/bridge/deploy/tests/solana-dry-run-check.sh:50</code> | <code>bash services/bridge/deploy/tests/solana-dry-run-check.sh</code> |
| <code>interface.14.cli-bridge-tests-workflow-check-j55</code> | <code>cli_command</code> | <code>stable</code> | <code>services/bridge/deploy/tests/workflow-check.sh:14</code> | <code>bash services/bridge/deploy/tests/workflow-check.sh</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cli-bridge-tests-checklist-check-j55</code> | notes | offline check script run by the deploy-tooling leg |
| <code>interface.14.cli-bridge-tests-deploy-scripts-check-j55</code> | notes | offline check script run by the deploy-tooling leg |
| <code>interface.14.cli-bridge-tests-docs-check-j55</code> | notes | offline check script run by the deploy-tooling leg |
| <code>interface.14.cli-bridge-tests-evm-dry-run-check-j55</code> | notes | offline dry-run check script |
| <code>interface.14.cli-bridge-tests-solana-dry-run-check-j55</code> | notes | offline dry-run check script |
| <code>interface.14.cli-bridge-tests-workflow-check-j55</code> | notes | offline check script run by the deploy-tooling leg |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.14.506.1</code> | req | 14506.1 |
| <code>acceptance.14.506.1</code> | scenario | Given .github/workflows/bridge-test.yml with the bridge-app job and filter from wiring J55.1, when bash services/bridge/deploy/tests/workflow-check.sh runs, then it exits 0 |
| <code>acceptance.14.506.1</code> | test | services/bridge/deploy/tests/workflow-check.sh::workflow-check (existing) |
| <code>acceptance.14.506.1</code> | qualification | UNRUN |
| <code>req.14506</code> | ac_1 | workflow-check.sh passes against the workflow once it carries the apps/bridge/** filter and a bridge-app job running pnpm install --frozen-lockfile and pnpm test in apps/bridge; at HEAD FILTERS at services/bridge/deploy/tests/workflow-check.sh:50 and LEGS at services/bridge/deploy/tests/workflow-check.sh:61 admit only four legs. |
| <code>capability.14.53.4</code> | title | Accept the bridge-app leg and the apps/bridge path filter in workflow-check.sh |
| <code>capability.14.53.4</code> | reqs | 14506.1 |
| <code>resolution.17.955</code> | title | decision.1441 binds: The services/bridge/deploy test suite needs no re-targeting task |
| <code>resolution.17.955</code> | applies_to | decision.1441<br>task.14.53.4<br>task.14.58.1 |
| <code>resolution.17.955</code> | source | decision.1441 in fragments/J55/decision.kvx; tasks from affects |

## conformance

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.14.508.1</code> | req | 14508.1 |
| <code>acceptance.14.508.1</code> | scenario | Given apps/bridge/src/lib/bridge/config.ts read from the repository root, when the test extracts every process.env.NEXT_PUBLIC_BRIDGE_VAULT_* and NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID name and compares them with EnvNames(), then the sets are equal |
| <code>acceptance.14.508.1</code> | test | services/bridge/deploy/appenv/appenv_test.go::TestEnvNamesMatchTheBridgeAppConfig (planned) |
| <code>acceptance.14.508.1</code> | qualification | UNRUN |
| <code>req.14508</code> | ac_1 | The set of NEXT_PUBLIC_BRIDGE_VAULT_* and NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID names EnvLines can emit equals the set read in apps/bridge/src/lib/bridge/config.ts:30-58. |
| <code>capability.14.53.6</code> | title | Pin the env names appenv writes to the names the bridge app reads |
| <code>capability.14.53.6</code> | reqs | 14508.1 |

## proposals

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cli-bridge-paxeer-bridge-proposals-flag-manifest-j55</code> | <code>cli_command</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:630</code> | <code>-manifest &lt;path&gt; (default services/bridge/deploy/attestors.json)</code> |
| <code>interface.14.cli-bridge-paxeer-bridge-proposals-j55</code> | <code>cli_command</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/cmd/paxeer-bridge-proposals/main.go:19</code> | <code>paxeer-bridge-proposals [-manifest &lt;path&gt;] &lt;chain-configuration.json&gt; &lt;output-directory&gt;</code> |
| <code>interface.14.fmt-bridge-attestor-manifest-j55</code> | <code>file_format</code> | <code>stable</code> | <code>services/bridge/deploy/attestors.json:1</code> | <code>{threshold, attestors[]} read by proposals.LoadAttestorManifest</code> |
| <code>interface.14.go-bridge-proposals-asset-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:133</code> | <code>type Asset struct</code> |
| <code>interface.14.go-bridge-proposals-assetreadback-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:850</code> | <code>type AssetReadback struct</code> |
| <code>interface.14.go-bridge-proposals-attestormanifest-check-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/manifest.go:72</code> | <code>func (m AttestorManifest) Check(cfg ChainConfig) error</code> |
| <code>interface.14.go-bridge-proposals-attestormanifest-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/manifest.go:17</code> | <code>type AttestorManifest struct</code> |
| <code>interface.14.go-bridge-proposals-attestormanifest-signers-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/manifest.go:57</code> | <code>func (m AttestorManifest) Signers() []types.Address20</code> |
| <code>interface.14.go-bridge-proposals-bundle-files-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:439</code> | <code>func (b Bundle) Files() ([]File, error)</code> |
| <code>interface.14.go-bridge-proposals-bundle-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:235</code> | <code>type Bundle struct</code> |
| <code>interface.14.go-bridge-proposals-bundle-proposalfiles-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:464</code> | <code>func (b Bundle) ProposalFiles() ([]File, error)</code> |
| <code>interface.14.go-bridge-proposals-bundle-proposals-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:495</code> | <code>func (b Bundle) Proposals() ([]Proposal, error)</code> |
| <code>interface.14.go-bridge-proposals-bundle-write-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:544</code> | <code>func (b Bundle) Write(dir string) ([]string, error)</code> |
| <code>interface.14.go-bridge-proposals-bundle-writeproposals-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:554</code> | <code>func (b Bundle) WriteProposals(dir string) ([]string, error)</code> |
| <code>interface.14.go-bridge-proposals-chainconfig-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:147</code> | <code>type ChainConfig struct</code> |
| <code>interface.14.go-bridge-proposals-commandname-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:55</code> | <code>CommandName = "paxeer-bridge-proposals"</code> |
| <code>interface.14.go-bridge-proposals-decodebody-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:1027</code> | <code>func DecodeBody(body []byte) (sdk.Msg, error)</code> |
| <code>interface.14.go-bridge-proposals-decodeproposal-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:1010</code> | <code>func DecodeProposal(body []byte) (*types.BridgeProposal, error)</code> |
| <code>interface.14.go-bridge-proposals-defaultmanifestpath-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:52</code> | <code>DefaultManifestPath = "services/bridge/deploy/attestors.json"</code> |
| <code>interface.14.go-bridge-proposals-evmnativeaddress-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:48</code> | <code>EVMNativeAddress = "0x0000000000000000000000000000000000000000"</code> |
| <code>interface.14.go-bridge-proposals-fielderror-error-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:124</code> | <code>func (e *FieldError) Error() string { return fmt.Sprintf("%s: %s: %s", e.File, e.Field, e.Msg) }</code> |
| <code>interface.14.go-bridge-proposals-fielderror-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:118</code> | <code>type FieldError struct</code> |
| <code>interface.14.go-bridge-proposals-file-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:225</code> | <code>type File struct</code> |
| <code>interface.14.go-bridge-proposals-fromchainconfig-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:763</code> | <code>func FromChainConfig(cfg *chainconfig.ChainConfig, authority, vault string) (ChainConfig, error)</code> |
| <code>interface.14.go-bridge-proposals-generate-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:246</code> | <code>func Generate(cfg ChainConfig, manifest AttestorManifest) (Bundle, error)</code> |
| <code>interface.14.go-bridge-proposals-loadattestormanifest-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/manifest.go:30</code> | <code>func LoadAttestorManifest(path string) (AttestorManifest, error)</code> |
| <code>interface.14.go-bridge-proposals-loadchainconfig-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:170</code> | <code>func LoadChainConfig(path string) (ChainConfig, error)</code> |
| <code>interface.14.go-bridge-proposals-openchainproposalfile-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:63</code> | <code>OpenChainProposalFile = "04-proposal-open-chain.json"</code> |
| <code>interface.14.go-bridge-proposals-proposal-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:482</code> | <code>type Proposal struct</code> |
| <code>interface.14.go-bridge-proposals-readback-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:825</code> | <code>type Readback struct</code> |
| <code>interface.14.go-bridge-proposals-readbackof-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:868</code> | <code>func ReadbackOf(cfg *chainconfig.ChainConfig, bundle Bundle) (Readback, error)</code> |
| <code>interface.14.go-bridge-proposals-registerchain-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:302</code> | <code>func RegisterChain(cfg ChainConfig) (types.MsgRegisterChain, error)</code> |
| <code>interface.14.go-bridge-proposals-registersidiorapair-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:283</code> | <code>func RegisterSidioraPair(cfg ChainConfig) (types.MsgRegisterSidioraPair, error)</code> |
| <code>interface.14.go-bridge-proposals-run-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:627</code> | <code>func Run(args []string, report io.Writer) error</code> |
| <code>interface.14.go-bridge-proposals-setattestors-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:337</code> | <code>func SetAttestors(cfg ChainConfig, manifest AttestorManifest) (types.MsgSetAttestors, error)</code> |
| <code>interface.14.go-bridge-proposals-setcaps-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:367</code> | <code>func SetCaps(cfg ChainConfig) ([]types.MsgSetCap, error)</code> |
| <code>interface.14.go-bridge-proposals-sidioraassetid-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:115</code> | <code>func SidioraAssetID() types.Address20 { return sidioraAssetID }</code> |
| <code>interface.14.go-bridge-proposals-sidioracapproposalfile-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:68</code> | <code>SidioraCapProposalFile = "05-proposal-sidiora-cap.json"</code> |
| <code>interface.14.go-bridge-proposals-solanachainid-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:40</code> | <code>SolanaChainID uint64 = 0x0000534f4c414e41</code> |
| <code>interface.14.go-bridge-proposals-solanareadback-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:840</code> | <code>type SolanaReadback struct</code> |
| <code>interface.14.go-bridge-proposals-wrappedsolmint-j55</code> | <code>go_exported</code> | <code>stable</code> | <code>services/bridge/deploy/proposals/proposals.go:44</code> | <code>WrappedSOLMint = "So11111111111111111111111111111111111111112"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cli-bridge-paxeer-bridge-proposals-flag-manifest-j55</code> | notes | attestor-set manifest the bundle is checked against |
| <code>interface.14.cli-bridge-paxeer-bridge-proposals-j55</code> | notes | writes the numbered proposal bodies of one chain; Run is proposals.Run |
| <code>interface.14.fmt-bridge-attestor-manifest-j55</code> | notes | committed attestor-set manifest |
| <code>interface.14.go-bridge-proposals-asset-j55</code> | notes | exported from package proposals |
| <code>interface.14.go-bridge-proposals-assetreadback-j55</code> | notes | exported from package proposals |
| <code>interface.14.go-bridge-proposals-attestormanifest-check-j55</code> | notes | exported from package proposals, method of AttestorManifest |
| <code>interface.14.go-bridge-proposals-attestormanifest-j55</code> | notes | exported from package proposals |
| <code>interface.14.go-bridge-proposals-attestormanifest-signers-j55</code> | notes | exported from package proposals, method of AttestorManifest |
| <code>interface.14.go-bridge-proposals-bundle-files-j55</code> | notes | exported from package proposals, method of Bundle |
| <code>interface.14.go-bridge-proposals-bundle-j55</code> | notes | exported from package proposals |
| <code>interface.14.go-bridge-proposals-bundle-proposalfiles-j55</code> | notes | exported from package proposals, method of Bundle |
| <code>interface.14.go-bridge-proposals-bundle-proposals-j55</code> | notes | exported from package proposals, method of Bundle |
| <code>interface.14.go-bridge-proposals-bundle-write-j55</code> | notes | exported from package proposals, method of Bundle |
| <code>interface.14.go-bridge-proposals-bundle-writeproposals-j55</code> | notes | exported from package proposals, method of Bundle |
| <code>interface.14.go-bridge-proposals-chainconfig-j55</code> | notes | exported from package proposals |
| <code>interface.14.go-bridge-proposals-commandname-j55</code> | notes | exported from package proposals |
| <code>interface.14.go-bridge-proposals-decodebody-j55</code> | notes | exported from package proposals |
| <code>interface.14.go-bridge-proposals-decodeproposal-j55</code> | notes | exported from package proposals |
| <code>interface.14.go-bridge-proposals-defaultmanifestpath-j55</code> | notes | exported from package proposals |
| <code>interface.14.go-bridge-proposals-evmnativeaddress-j55</code> | notes | exported from package proposals |
| <code>interface.14.go-bridge-proposals-fielderror-error-j55</code> | notes | exported from package proposals, method of FieldError |
| <code>interface.14.go-bridge-proposals-fielderror-j55</code> | notes | exported from package proposals |
| <code>interface.14.go-bridge-proposals-file-j55</code> | notes | exported from package proposals |
| <code>interface.14.go-bridge-proposals-fromchainconfig-j55</code> | notes | exported from package proposals |
| <code>interface.14.go-bridge-proposals-generate-j55</code> | notes | exported from package proposals |
| <code>interface.14.go-bridge-proposals-loadattestormanifest-j55</code> | notes | exported from package proposals |
| <code>interface.14.go-bridge-proposals-loadchainconfig-j55</code> | notes | exported from package proposals |
| <code>interface.14.go-bridge-proposals-openchainproposalfile-j55</code> | notes | exported from package proposals |
| <code>interface.14.go-bridge-proposals-proposal-j55</code> | notes | exported from package proposals |
| <code>interface.14.go-bridge-proposals-readback-j55</code> | notes | exported from package proposals |
| <code>interface.14.go-bridge-proposals-readbackof-j55</code> | notes | exported from package proposals |
| <code>interface.14.go-bridge-proposals-registerchain-j55</code> | notes | exported from package proposals |
| <code>interface.14.go-bridge-proposals-registersidiorapair-j55</code> | notes | exported from package proposals |
| <code>interface.14.go-bridge-proposals-run-j55</code> | notes | exported from package proposals |
| <code>interface.14.go-bridge-proposals-setattestors-j55</code> | notes | exported from package proposals |
| <code>interface.14.go-bridge-proposals-setcaps-j55</code> | notes | exported from package proposals |
| <code>interface.14.go-bridge-proposals-sidioraassetid-j55</code> | notes | exported from package proposals |
| <code>interface.14.go-bridge-proposals-sidioracapproposalfile-j55</code> | notes | exported from package proposals |
| <code>interface.14.go-bridge-proposals-solanachainid-j55</code> | notes | exported from package proposals |
| <code>interface.14.go-bridge-proposals-solanareadback-j55</code> | notes | exported from package proposals |
| <code>interface.14.go-bridge-proposals-wrappedsolmint-j55</code> | notes | exported from package proposals |

## records

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.fmt-bridge-evm-deployment-record-j55</code> | <code>file_format</code> | <code>stable</code> | <code>services/bridge/deploy/deploy-evm-chain.sh:330</code> | <code>{chain, chain_id, kind, configuration, vault, deployer, code_hash, block, owner, configured_owner, ownership, threshold, attestors, caps[{asset, symbol, per_tx_cap, total_cap}]}</code> |
| <code>interface.14.fmt-bridge-solana-deployment-record-j55</code> | <code>file_format</code> | <code>stable</code> | <code>services/bridge/deploy/tests/fixtures/records/solana.json:7</code> | <code>{chain, chain_id, kind, configuration, genesis_hash, program_id, program_data_account, owner, threshold, commitment, vault_authority, vault_handle, attestors, assets}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.fmt-bridge-evm-deployment-record-j55</code> | notes | fixture at services/bridge/deploy/tests/fixtures/records/ethereum.json |
| <code>interface.14.fmt-bridge-solana-deployment-record-j55</code> | notes | written by deploy-solana-program.sh; fixture line shown |

## scripts

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cli-bridge-checklist-j55</code> | <code>cli_command</code> | <code>stable</code> | <code>services/bridge/deploy/checklist.sh:72</code> | <code>checklist.sh &lt;chain&#124;--all&gt;</code> |
| <code>interface.14.cli-bridge-deploy-evm-chain-flag-all-j55</code> | <code>cli_command</code> | <code>stable</code> | <code>services/bridge/deploy/deploy-evm-chain.sh:84</code> | <code>deploy-evm-chain.sh --all</code> |
| <code>interface.14.cli-bridge-deploy-evm-chain-flag-preflight-j55</code> | <code>cli_command</code> | <code>stable</code> | <code>services/bridge/deploy/deploy-evm-chain.sh:75</code> | <code>deploy-evm-chain.sh --preflight</code> |
| <code>interface.14.cli-bridge-deploy-evm-chain-j55</code> | <code>cli_command</code> | <code>stable</code> | <code>services/bridge/deploy/deploy-evm-chain.sh:69</code> | <code>deploy-evm-chain.sh [--preflight] &lt;chain&#124;--all&gt;</code> |
| <code>interface.14.cli-bridge-deploy-solana-program-j55</code> | <code>cli_command</code> | <code>stable</code> | <code>services/bridge/deploy/deploy-solana-program.sh:103</code> | <code>deploy-solana-program.sh [--preflight]</code> |
| <code>interface.14.cli-bridge-overlay-j55</code> | <code>cli_command</code> | <code>stable</code> | <code>services/bridge/deploy/overlay.sh:1</code> | <code>source services/bridge/deploy/overlay.sh</code> |
| <code>interface.14.cli-bridge-verify-evm-chain-j55</code> | <code>cli_command</code> | <code>stable</code> | <code>services/bridge/deploy/verify-evm-chain.sh:43</code> | <code>verify-evm-chain.sh [--preflight] &lt;chain&#124;--all&gt;</code> |
| <code>interface.14.env-bridge-bridge-deploy-overlay-j55</code> | <code>env_var</code> | <code>stable</code> | <code>services/bridge/deploy/chainconfig/overlay.go:15</code> | <code>BRIDGE_DEPLOY_OVERLAY</code> |
| <code>interface.14.env-bridge-paxeer-bridge-app-env-j55</code> | <code>env_var</code> | <code>new</code> | <code>services/bridge/deploy/checklist.sh</code> (planned) | <code>PAXEER_BRIDGE_APP_ENV</code> |
| <code>interface.14.env-bridge-paxeer-bridge-attestor-manifest-j55</code> | <code>env_var</code> | <code>stable</code> | <code>services/bridge/deploy/checklist.sh:25</code> | <code>PAXEER_BRIDGE_ATTESTOR_MANIFEST</code> |
| <code>interface.14.env-bridge-paxeer-bridge-deployment-record-dir-j55</code> | <code>env_var</code> | <code>stable</code> | <code>services/bridge/deploy/deploy-evm-chain.sh:12</code> | <code>PAXEER_BRIDGE_DEPLOYMENT_RECORD_DIR</code> |
| <code>interface.14.env-bridge-paxeer-bridge-deployment-record-j55</code> | <code>env_var</code> | <code>stable</code> | <code>services/bridge/deploy/deploy-evm-chain.sh:12</code> | <code>PAXEER_BRIDGE_DEPLOYMENT_RECORD</code> |
| <code>interface.14.env-bridge-paxeer-bridge-evm-chains-root-j55</code> | <code>env_var</code> | <code>stable</code> | <code>services/bridge/deploy/deploy-evm-chain.sh:33</code> | <code>PAXEER_BRIDGE_EVM_CHAINS_ROOT</code> |
| <code>interface.14.env-bridge-paxeer-bridge-governance-authority-j55</code> | <code>env_var</code> | <code>stable</code> | <code>services/bridge/deploy/checklist.sh:24</code> | <code>PAXEER_BRIDGE_GOVERNANCE_AUTHORITY</code> |
| <code>interface.14.env-bridge-paxeer-bridge-paxeer-rpc-url-j55</code> | <code>env_var</code> | <code>stable</code> | <code>services/bridge/deploy/checklist.sh:22</code> | <code>PAXEER_BRIDGE_PAXEER_RPC_URL</code> |
| <code>interface.14.env-bridge-paxeer-bridge-solana-admin-cli-j55</code> | <code>env_var</code> | <code>stable</code> | <code>services/bridge/deploy/deploy-solana-program.sh:23</code> | <code>PAXEER_BRIDGE_SOLANA_ADMIN_CLI</code> |
| <code>interface.14.env-bridge-paxeer-bridge-solana-chains-root-j55</code> | <code>env_var</code> | <code>stable</code> | <code>services/bridge/deploy/deploy-solana-program.sh:28</code> | <code>PAXEER_BRIDGE_SOLANA_CHAINS_ROOT</code> |
| <code>interface.14.env-bridge-paxeer-bridge-solana-executable-wait-seconds-j55</code> | <code>env_var</code> | <code>stable</code> | <code>services/bridge/deploy/deploy-solana-program.sh:26</code> | <code>PAXEER_BRIDGE_SOLANA_EXECUTABLE_WAIT_SECONDS</code> |
| <code>interface.14.env-bridge-paxeer-bridge-solana-program-keypair-file-j55</code> | <code>env_var</code> | <code>stable</code> | <code>services/bridge/deploy/deploy-solana-program.sh:25</code> | <code>PAXEER_BRIDGE_SOLANA_PROGRAM_KEYPAIR_FILE</code> |
| <code>interface.14.env-bridge-paxeer-bridge-vault-assets-j55</code> | <code>env_var</code> | <code>stable</code> | <code>services/bridge/deploy/deploy-evm-chain.sh:249</code> | <code>PAXEER_BRIDGE_VAULT_ASSETS</code> |
| <code>interface.14.env-bridge-paxeer-bridge-vault-attestors-j55</code> | <code>env_var</code> | <code>stable</code> | <code>services/bridge/deploy/deploy-evm-chain.sh:247</code> | <code>PAXEER_BRIDGE_VAULT_ATTESTORS</code> |
| <code>interface.14.env-bridge-paxeer-bridge-vault-owner-j55</code> | <code>env_var</code> | <code>stable</code> | <code>services/bridge/deploy/deploy-evm-chain.sh:246</code> | <code>PAXEER_BRIDGE_VAULT_OWNER</code> |
| <code>interface.14.env-bridge-paxeer-bridge-vault-per-tx-caps-j55</code> | <code>env_var</code> | <code>stable</code> | <code>services/bridge/deploy/deploy-evm-chain.sh:250</code> | <code>PAXEER_BRIDGE_VAULT_PER_TX_CAPS</code> |
| <code>interface.14.env-bridge-paxeer-bridge-vault-threshold-j55</code> | <code>env_var</code> | <code>stable</code> | <code>services/bridge/deploy/deploy-evm-chain.sh:248</code> | <code>PAXEER_BRIDGE_VAULT_THRESHOLD</code> |
| <code>interface.14.env-bridge-paxeer-bridge-vault-total-caps-j55</code> | <code>env_var</code> | <code>stable</code> | <code>services/bridge/deploy/deploy-evm-chain.sh:251</code> | <code>PAXEER_BRIDGE_VAULT_TOTAL_CAPS</code> |
| <code>interface.14.fmt-bridge-relayer-solana-example-j55</code> | <code>file_format</code> | <code>stable</code> | <code>services/bridge/deploy/relayer-solana.example.json:1</code> | <code>example relayer configuration for the Solana leg</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cli-bridge-checklist-j55</code> | notes | compares a deployment record, the chain configuration and the live Paxeer registration |
| <code>interface.14.cli-bridge-deploy-evm-chain-flag-all-j55</code> | notes | deploys the eight EVM chains in registry order, stopping at the first failure |
| <code>interface.14.cli-bridge-deploy-evm-chain-flag-preflight-j55</code> | notes | runs every offline check and stops before the first chain call |
| <code>interface.14.cli-bridge-deploy-evm-chain-j55</code> | notes | deploys one PaxeerXVault and writes its deployment record |
| <code>interface.14.cli-bridge-deploy-solana-program-j55</code> | notes | deploys and initialises the Solana custody program and writes its deployment record |
| <code>interface.14.cli-bridge-overlay-j55</code> | notes | sourced by the deploy scripts to apply the private overlay through overlay.jq |
| <code>interface.14.cli-bridge-verify-evm-chain-j55</code> | notes | submits the deployed vault source for explorer verification |
| <code>interface.14.env-bridge-bridge-deploy-overlay-j55</code> | notes | private deploy overlay path read by chainconfig.LoadOverlay and overlay.sh |
| <code>interface.14.env-bridge-paxeer-bridge-app-env-j55</code> | notes | created by task 14.58.1; app env file checklist.sh compares with the record |
| <code>interface.14.env-bridge-paxeer-bridge-attestor-manifest-j55</code> | notes | attestor manifest path the checklist compares against |
| <code>interface.14.env-bridge-paxeer-bridge-deployment-record-dir-j55</code> | notes | directory --all writes &lt;chain&gt;.json records into |
| <code>interface.14.env-bridge-paxeer-bridge-deployment-record-j55</code> | notes | path of the deployment record a deploy script writes or checklist.sh reads |
| <code>interface.14.env-bridge-paxeer-bridge-evm-chains-root-j55</code> | notes | optional chains root replacing contracts/bridge/evm/chains |
| <code>interface.14.env-bridge-paxeer-bridge-governance-authority-j55</code> | notes | governance authority the checklist expects on the proposals |
| <code>interface.14.env-bridge-paxeer-bridge-paxeer-rpc-url-j55</code> | notes | Paxeer endpoint checklist.sh reads the registration from |
| <code>interface.14.env-bridge-paxeer-bridge-solana-admin-cli-j55</code> | notes | path of the Solana custody admin CLI binary |
| <code>interface.14.env-bridge-paxeer-bridge-solana-chains-root-j55</code> | notes | optional chains root replacing contracts/bridge/solana/chains |
| <code>interface.14.env-bridge-paxeer-bridge-solana-executable-wait-seconds-j55</code> | notes | bound on the wait for the program to become executable |
| <code>interface.14.env-bridge-paxeer-bridge-solana-program-keypair-file-j55</code> | notes | program keypair file the deployment uses |
| <code>interface.14.env-bridge-paxeer-bridge-vault-assets-j55</code> | notes | asset list passed to the forge deploy script |
| <code>interface.14.env-bridge-paxeer-bridge-vault-attestors-j55</code> | notes | attestor list passed to the forge deploy script |
| <code>interface.14.env-bridge-paxeer-bridge-vault-owner-j55</code> | notes | owner passed to the forge deploy script |
| <code>interface.14.env-bridge-paxeer-bridge-vault-per-tx-caps-j55</code> | notes | per-transaction caps passed to the forge deploy script |
| <code>interface.14.env-bridge-paxeer-bridge-vault-threshold-j55</code> | notes | threshold passed to the forge deploy script |
| <code>interface.14.env-bridge-paxeer-bridge-vault-total-caps-j55</code> | notes | total caps passed to the forge deploy script |
| <code>interface.14.fmt-bridge-relayer-solana-example-j55</code> | notes | example only; the relayer configuration schema belongs to domain 11 |

Contract context <code>contract_context.14.bridge-deploy</code>:

| Field | Frozen rule |
|---|---|
| interface | services/bridge/deploy: deploy-evm-chain.sh, deploy-solana-program.sh, verify-evm-chain.sh and checklist.sh write and check deployment records; paxeer-bridge-proposals renders governance proposals; paxeer-bridge-app-env renders and checks the app env. |
| storage | Deployment records &lt;chain&gt;.json under PAXEER_BRIDGE_DEPLOYMENT_RECORD_DIR; attestors.json manifest; no chain state written by the tooling except the deployments themselves. |
| errors | Each script exits 1 naming the failing file, variable or field; appenv.LoadRecord refuses a chain_id that differs from the registry and a zero vault. |
| events | None. |
| gas | Forge and the Solana CLI estimate gas; the scripts set none. |
| activation | Run by an operator per chain; the app env is rendered after the records exist. |
| invariants | The app env names exactly the vaults and program id of the deployment records; chain ids match chainconfig.Chains. |
| pause | Not applicable; the tooling does not pause contracts. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.14.519.1</code> | req | 14519.1 |
| <code>acceptance.14.519.1</code> | scenario | Given the base fixture record and an app env file with another vault, when bash services/bridge/deploy/tests/checklist-check.sh runs its new app-env cases, then the mismatch case fails naming the variable, the match case passes and the script exits 0 |
| <code>acceptance.14.519.1</code> | test | services/bridge/deploy/tests/checklist-check.sh::checklist-check (existing) |
| <code>acceptance.14.519.1</code> | qualification | UNRUN |
| <code>req.14519</code> | ac_1 | checklist.sh fails with a FAIL line naming NEXT_PUBLIC_BRIDGE_VAULT_&lt;CHAIN&gt; when PAXEER_BRIDGE_APP_ENV names an env file whose vault differs from the record read at services/bridge/deploy/checklist.sh:175, and passes when they match; at HEAD the checklist never reads the app env. |
| <code>capability.14.58.1</code> | title | Compare the bridge app env with the deployment record in checklist.sh |
| <code>capability.14.58.1</code> | reqs | 14519.1 |
