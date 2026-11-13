# Bridge deploy contract

Source paths are relative to the repository root. Declaration tables identify the interfaces in the source files; runtime behavior is defined by their implementations.

## app-env

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.14.cli-bridge-paxeer-bridge-app-env-flag-check-j55</code> | <code>cli_command</code> | <code>services/bridge/deploy/cmd/paxeer-bridge-app-env/main.go</code> | <code>paxeer-bridge-app-env --check &lt;env-file&gt; &lt;records-directory&gt;</code> |
| <code>interface.14.cli-bridge-paxeer-bridge-app-env-j55</code> | <code>cli_command</code> | <code>services/bridge/deploy/cmd/paxeer-bridge-app-env/main.go</code> | <code>paxeer-bridge-app-env &lt;records-directory&gt;</code> |
| <code>interface.14.go-bridge-appenv-check-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/appenv/appenv.go</code> | <code>func Check(envPath string, records []Record) error</code> |
| <code>interface.14.go-bridge-appenv-envlines-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/appenv/appenv.go</code> | <code>func EnvLines(records []Record) []string</code> |
| <code>interface.14.go-bridge-appenv-envnames-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/appenv/appenv.go</code> | <code>func EnvNames() []string</code> |
| <code>interface.14.go-bridge-appenv-loadrecord-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/appenv/appenv.go</code> | <code>func LoadRecord(path string) (Record, error)</code> |
| <code>interface.14.go-bridge-appenv-record-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/appenv/appenv.go</code> | <code>type Record struct {</code> |
| <code>interface.14.go-bridge-appenv-run-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/appenv/appenv.go</code> | <code>func Run(args []string, w io.Writer) error</code> |

| Field | Rule |
|---|---|
| interface | services/bridge/deploy: deploy-evm-chain.sh, deploy-solana-program.sh, verify-evm-chain.sh and checklist.sh write and check deployment records; paxeer-bridge-proposals renders governance proposals; paxeer-bridge-app-env renders and checks the app env. |
| storage | Deployment records &lt;chain&gt;.json under PAXEER_BRIDGE_DEPLOYMENT_RECORD_DIR; attestors.json manifest; no chain state written by the tooling except the deployments themselves. |
| errors | Scripts refuse invalid input with a nonzero exit status; appenv.LoadRecord refuses a chain_id that differs from the registry and a zero vault. |
| events | None. |
| gas | Forge and the Solana CLI estimate gas; the scripts set none. |
| activation | Run by an operator per chain; the app env is rendered after the records exist. |
| invariants | The app env names exactly the vaults and program id of the deployment records; chain ids match chainconfig.Chains. |
| pause | Not applicable; the tooling does not pause contracts. |

| Requirement | Behavior |
| --- | --- |
| <code>req.14503</code> | EnvLines([]Record) returns NEXT_PUBLIC_BRIDGE_VAULT_<CHAIN>=<vault> for each EVM record in chainconfig.Chains order (services/bridge/deploy/chainconfig/config.go), the names the app reads at apps/bridge/src/lib/bridge/config.ts. |
| <code>req.14503</code> | LoadRecord refuses a record whose chain_id differs from the registry chain id of its chain name (services/bridge/deploy/chainconfig/config.go), naming the file and the field. |
| <code>req.14503</code> | LoadRecord refuses an EVM record whose vault is the zero address or not a 20-byte hex address (services/bridge/deploy/deploy-evm-chain.sh). |
| <code>req.14504</code> | Run([]string{dir}, w) writes one line per record found in dir in chainconfig.Chains order (services/bridge/deploy/chainconfig/config.go), whatever the file order. |
| <code>req.14504</code> | Run refuses a directory holding no *.json record and writes nothing (services/bridge/deploy/deploy-evm-chain.sh writes records into PAXEER_BRIDGE_DEPLOYMENT_RECORD_DIR). |
| <code>req.14505</code> | EnvLines emits NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID=<program_id> for the record at services/bridge/deploy/tests/fixtures/records/solana.json, the variable readConfig reads (apps/bridge/src/lib/bridge/config.ts). |
| <code>req.14507</code> | Check(envFile, records) returns an error naming NEXT_PUBLIC_BRIDGE_VAULT_ETHEREUM when the env file holds another address than the record at services/bridge/deploy/tests/fixtures/records/ethereum.json. |
| <code>req.14507</code> | Check returns nil when every record line is present with the same value, ignoring case of hex digits (services/bridge/deploy/tests/fixtures/records/ethereum.json). |

## chainconfig

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.14.fmt-bridge-deploy-overlay-j55</code> | <code>file_format</code> | <code>services/bridge/deploy/overlay.jq</code> | <code>{chains: {&lt;name&gt;: ChainOverlay}} applied by overlay.jq and chainconfig.Overlay.Apply</code> |
| <code>interface.14.go-bridge-chainconfig-asset-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/config.go</code> | <code>type Asset struct</code> |
| <code>interface.14.go-bridge-chainconfig-asset-pertx-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/config.go</code> | <code>func (a Asset) PerTx() (*big.Int, error) { return parseAmount(a.PerTxCap) }</code> |
| <code>interface.14.go-bridge-chainconfig-asset-total-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/config.go</code> | <code>func (a Asset) Total() (*big.Int, error) { return parseAmount(a.TotalCap) }</code> |
| <code>interface.14.go-bridge-chainconfig-assetcap-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/overlay.go</code> | <code>type AssetCap struct</code> |
| <code>interface.14.go-bridge-chainconfig-bigblockchain-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/config.go</code> | <code>const BigBlockChain = "hyperevm"</code> |
| <code>interface.14.go-bridge-chainconfig-bigblocks-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/config.go</code> | <code>type BigBlocks struct</code> |
| <code>interface.14.go-bridge-chainconfig-chain-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/config.go</code> | <code>type Chain struct</code> |
| <code>interface.14.go-bridge-chainconfig-chainbyname-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/config.go</code> | <code>func ChainByName(name string) (Chain, bool)</code> |
| <code>interface.14.go-bridge-chainconfig-chainconfig-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/config.go</code> | <code>type ChainConfig struct</code> |
| <code>interface.14.go-bridge-chainconfig-chainconfig-nativeasset-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/config.go</code> | <code>func (c *ChainConfig) NativeAsset() Asset { return c.Assets[0] }</code> |
| <code>interface.14.go-bridge-chainconfig-chainconfig-requiredeployable-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/validate.go</code> | <code>func (c *ChainConfig) RequireDeployable() error</code> |
| <code>interface.14.go-bridge-chainconfig-chainconfig-source-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/config.go</code> | <code>func (c *ChainConfig) Source() string { return c.path }</code> |
| <code>interface.14.go-bridge-chainconfig-chainconfig-validate-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/validate.go</code> | <code>func (c *ChainConfig) Validate() error</code> |
| <code>interface.14.go-bridge-chainconfig-chainoverlay-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/overlay.go</code> | <code>type ChainOverlay struct</code> |
| <code>interface.14.go-bridge-chainconfig-chains-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/config.go</code> | <code>func Chains() []Chain</code> |
| <code>interface.14.go-bridge-chainconfig-decodebase58-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/config.go</code> | <code>func DecodeBase58(value string) ([]byte, error) { return vectors.DecodeBase58(value) }</code> |
| <code>interface.14.go-bridge-chainconfig-environment-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/config.go</code> | <code>type Environment struct</code> |
| <code>interface.14.go-bridge-chainconfig-isplaceholder-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/config.go</code> | <code>func IsPlaceholder(value string) bool { return strings.HasPrefix(value, PlaceholderPrefix) }</code> |
| <code>interface.14.go-bridge-chainconfig-kind-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/config.go</code> | <code>type Kind string</code> |
| <code>interface.14.go-bridge-chainconfig-kindevm-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/config.go</code> | <code>KindEVM Kind = "evm"</code> |
| <code>interface.14.go-bridge-chainconfig-kindsolana-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/config.go</code> | <code>KindSolana Kind = "solana"</code> |
| <code>interface.14.go-bridge-chainconfig-load-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/config.go</code> | <code>func Load(path string) (*ChainConfig, error)</code> |
| <code>interface.14.go-bridge-chainconfig-loadchain-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/config.go</code> | <code>func LoadChain(chainsRoot, name string) (*ChainConfig, error)</code> |
| <code>interface.14.go-bridge-chainconfig-loadoverlay-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/overlay.go</code> | <code>func LoadOverlay(path string) (*Overlay, error)</code> |
| <code>interface.14.go-bridge-chainconfig-maxattestors-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/config.go</code> | <code>MaxAttestors = 64</code> |
| <code>interface.14.go-bridge-chainconfig-nativecoin-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/config.go</code> | <code>type NativeCoin struct</code> |
| <code>interface.14.go-bridge-chainconfig-nativeevmasset-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/config.go</code> | <code>NativeEVMAsset = "0x0000000000000000000000000000000000000000"</code> |
| <code>interface.14.go-bridge-chainconfig-overlay-apply-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/overlay.go</code> | <code>func (o *Overlay) Apply(c *ChainConfig) error</code> |
| <code>interface.14.go-bridge-chainconfig-overlay-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/overlay.go</code> | <code>type Overlay struct</code> |
| <code>interface.14.go-bridge-chainconfig-overlayenv-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/overlay.go</code> | <code>const OverlayEnv = "BRIDGE_DEPLOY_OVERLAY"</code> |
| <code>interface.14.go-bridge-chainconfig-path-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/config.go</code> | <code>func Path(repoRoot, name string) (string, error)</code> |
| <code>interface.14.go-bridge-chainconfig-placeholderprefix-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/config.go</code> | <code>PlaceholderPrefix = "PLACEHOLDER:"</code> |
| <code>interface.14.go-bridge-chainconfig-sidioraassetid-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/config.go</code> | <code>SidioraAssetID = "0x21f7b20a555199fa73A238B1a91FD0f549068fEe"</code> |
| <code>interface.14.go-bridge-chainconfig-sidioramint-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/config.go</code> | <code>SidioraMint = "5w3wVdJaESaJKyLmStM6Hv9UyUkmZ1b9DLQquAqqpump"</code> |
| <code>interface.14.go-bridge-chainconfig-solanachainid-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/config.go</code> | <code>SolanaChainID uint64 = 91600046870081</code> |
| <code>interface.14.go-bridge-chainconfig-solanahandle-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/config.go</code> | <code>func SolanaHandle(key string) (string, error)</code> |
| <code>interface.14.go-bridge-chainconfig-solanasection-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/config.go</code> | <code>type SolanaSection struct</code> |
| <code>interface.14.go-bridge-chainconfig-solanazeropubkey-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/config.go</code> | <code>SolanaZeroPubkey = "11111111111111111111111111111111"</code> |
| <code>interface.14.go-bridge-chainconfig-wrappedsolmint-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/chainconfig/config.go</code> | <code>WrappedSolMint = "So11111111111111111111111111111111111111112"</code> |

## ci

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.14.cli-bridge-tests-checklist-check-j55</code> | <code>cli_command</code> | <code>services/bridge/deploy/tests/checklist-check.sh</code> | <code>bash services/bridge/deploy/tests/checklist-check.sh</code> |
| <code>interface.14.cli-bridge-tests-deploy-scripts-check-j55</code> | <code>cli_command</code> | <code>services/bridge/deploy/tests/deploy-scripts-check.sh</code> | <code>bash services/bridge/deploy/tests/deploy-scripts-check.sh</code> |
| <code>interface.14.cli-bridge-tests-docs-check-j55</code> | <code>cli_command</code> | <code>services/bridge/deploy/tests/docs-check.sh</code> | <code>bash services/bridge/deploy/tests/docs-check.sh</code> |
| <code>interface.14.cli-bridge-tests-evm-dry-run-check-j55</code> | <code>cli_command</code> | <code>services/bridge/deploy/tests/evm-dry-run-check.sh</code> | <code>bash services/bridge/deploy/tests/evm-dry-run-check.sh</code> |
| <code>interface.14.cli-bridge-tests-solana-dry-run-check-j55</code> | <code>cli_command</code> | <code>services/bridge/deploy/tests/solana-dry-run-check.sh</code> | <code>bash services/bridge/deploy/tests/solana-dry-run-check.sh</code> |
| <code>interface.14.cli-bridge-tests-workflow-check-j55</code> | <code>cli_command</code> | <code>services/bridge/deploy/tests/workflow-check.sh</code> | <code>bash services/bridge/deploy/tests/workflow-check.sh</code> |


## conformance

| Requirement | Behavior |
| --- | --- |
| <code>req.14508</code> | The set of NEXT_PUBLIC_BRIDGE_VAULT_* and NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID names EnvLines can emit equals the set read in apps/bridge/src/lib/bridge/config.ts. |

## proposals

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.14.cli-bridge-paxeer-bridge-proposals-flag-manifest-j55</code> | <code>cli_command</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>-manifest &lt;path&gt; (default services/bridge/deploy/attestors.json)</code> |
| <code>interface.14.cli-bridge-paxeer-bridge-proposals-j55</code> | <code>cli_command</code> | <code>services/bridge/deploy/proposals/cmd/paxeer-bridge-proposals/main.go</code> | <code>paxeer-bridge-proposals [-manifest &lt;path&gt;] &lt;chain-configuration.json&gt; &lt;output-directory&gt;</code> |
| <code>interface.14.fmt-bridge-attestor-manifest-j55</code> | <code>file_format</code> | <code>services/bridge/deploy/attestors.json</code> | <code>{threshold, attestors[]} read by proposals.LoadAttestorManifest</code> |
| <code>interface.14.go-bridge-proposals-asset-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>type Asset struct</code> |
| <code>interface.14.go-bridge-proposals-assetreadback-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>type AssetReadback struct</code> |
| <code>interface.14.go-bridge-proposals-attestormanifest-check-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/manifest.go</code> | <code>func (m AttestorManifest) Check(cfg ChainConfig) error</code> |
| <code>interface.14.go-bridge-proposals-attestormanifest-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/manifest.go</code> | <code>type AttestorManifest struct</code> |
| <code>interface.14.go-bridge-proposals-attestormanifest-signers-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/manifest.go</code> | <code>func (m AttestorManifest) Signers() []types.Address20</code> |
| <code>interface.14.go-bridge-proposals-bundle-files-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>func (b Bundle) Files() ([]File, error)</code> |
| <code>interface.14.go-bridge-proposals-bundle-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>type Bundle struct</code> |
| <code>interface.14.go-bridge-proposals-bundle-proposalfiles-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>func (b Bundle) ProposalFiles() ([]File, error)</code> |
| <code>interface.14.go-bridge-proposals-bundle-proposals-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>func (b Bundle) Proposals() ([]Proposal, error)</code> |
| <code>interface.14.go-bridge-proposals-bundle-write-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>func (b Bundle) Write(dir string) ([]string, error)</code> |
| <code>interface.14.go-bridge-proposals-bundle-writeproposals-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>func (b Bundle) WriteProposals(dir string) ([]string, error)</code> |
| <code>interface.14.go-bridge-proposals-chainconfig-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>type ChainConfig struct</code> |
| <code>interface.14.go-bridge-proposals-commandname-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>CommandName = "paxeer-bridge-proposals"</code> |
| <code>interface.14.go-bridge-proposals-decodebody-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>func DecodeBody(body []byte) (sdk.Msg, error)</code> |
| <code>interface.14.go-bridge-proposals-decodeproposal-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>func DecodeProposal(body []byte) (*types.BridgeProposal, error)</code> |
| <code>interface.14.go-bridge-proposals-defaultmanifestpath-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>DefaultManifestPath = "services/bridge/deploy/attestors.json"</code> |
| <code>interface.14.go-bridge-proposals-evmnativeaddress-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>EVMNativeAddress = "0x0000000000000000000000000000000000000000"</code> |
| <code>interface.14.go-bridge-proposals-fielderror-error-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>func (e *FieldError) Error() string { return fmt.Sprintf("%s: %s: %s", e.File, e.Field, e.Msg) }</code> |
| <code>interface.14.go-bridge-proposals-fielderror-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>type FieldError struct</code> |
| <code>interface.14.go-bridge-proposals-file-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>type File struct</code> |
| <code>interface.14.go-bridge-proposals-fromchainconfig-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>func FromChainConfig(cfg *chainconfig.ChainConfig, authority, vault string) (ChainConfig, error)</code> |
| <code>interface.14.go-bridge-proposals-generate-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>func Generate(cfg ChainConfig, manifest AttestorManifest) (Bundle, error)</code> |
| <code>interface.14.go-bridge-proposals-loadattestormanifest-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/manifest.go</code> | <code>func LoadAttestorManifest(path string) (AttestorManifest, error)</code> |
| <code>interface.14.go-bridge-proposals-loadchainconfig-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>func LoadChainConfig(path string) (ChainConfig, error)</code> |
| <code>interface.14.go-bridge-proposals-openchainproposalfile-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>OpenChainProposalFile = "04-proposal-open-chain.json"</code> |
| <code>interface.14.go-bridge-proposals-proposal-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>type Proposal struct</code> |
| <code>interface.14.go-bridge-proposals-readback-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>type Readback struct</code> |
| <code>interface.14.go-bridge-proposals-readbackof-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>func ReadbackOf(cfg *chainconfig.ChainConfig, bundle Bundle) (Readback, error)</code> |
| <code>interface.14.go-bridge-proposals-registerchain-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>func RegisterChain(cfg ChainConfig) (types.MsgRegisterChain, error)</code> |
| <code>interface.14.go-bridge-proposals-registersidiorapair-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>func RegisterSidioraPair(cfg ChainConfig) (types.MsgRegisterSidioraPair, error)</code> |
| <code>interface.14.go-bridge-proposals-run-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>func Run(args []string, report io.Writer) error</code> |
| <code>interface.14.go-bridge-proposals-setattestors-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>func SetAttestors(cfg ChainConfig, manifest AttestorManifest) (types.MsgSetAttestors, error)</code> |
| <code>interface.14.go-bridge-proposals-setcaps-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>func SetCaps(cfg ChainConfig) ([]types.MsgSetCap, error)</code> |
| <code>interface.14.go-bridge-proposals-sidioraassetid-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>func SidioraAssetID() types.Address20 { return sidioraAssetID }</code> |
| <code>interface.14.go-bridge-proposals-sidioracapproposalfile-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>SidioraCapProposalFile = "05-proposal-sidiora-cap.json"</code> |
| <code>interface.14.go-bridge-proposals-solanachainid-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>SolanaChainID uint64 = 0x0000534f4c414e41</code> |
| <code>interface.14.go-bridge-proposals-solanareadback-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>type SolanaReadback struct</code> |
| <code>interface.14.go-bridge-proposals-wrappedsolmint-j55</code> | <code>go_exported</code> | <code>services/bridge/deploy/proposals/proposals.go</code> | <code>WrappedSOLMint = "So11111111111111111111111111111111111111112"</code> |

## records

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.14.fmt-bridge-evm-deployment-record-j55</code> | <code>file_format</code> | <code>services/bridge/deploy/deploy-evm-chain.sh</code> | <code>{chain, chain_id, kind, configuration, vault, deployer, code_hash, block, owner, configured_owner, ownership, threshold, attestors, caps[{asset, symbol, per_tx_cap, total_cap}]}</code> |
| <code>interface.14.fmt-bridge-solana-deployment-record-j55</code> | <code>file_format</code> | <code>services/bridge/deploy/tests/fixtures/records/solana.json</code> | <code>{chain, chain_id, kind, configuration, genesis_hash, program_id, program_data_account, owner, threshold, commitment, vault_authority, vault_handle, attestors, assets}</code> |

## scripts

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.14.cli-bridge-checklist-j55</code> | <code>cli_command</code> | <code>services/bridge/deploy/checklist.sh</code> | <code>checklist.sh &lt;chain&#124;--all&gt;</code> |
| <code>interface.14.cli-bridge-deploy-evm-chain-flag-all-j55</code> | <code>cli_command</code> | <code>services/bridge/deploy/deploy-evm-chain.sh</code> | <code>deploy-evm-chain.sh --all</code> |
| <code>interface.14.cli-bridge-deploy-evm-chain-flag-preflight-j55</code> | <code>cli_command</code> | <code>services/bridge/deploy/deploy-evm-chain.sh</code> | <code>deploy-evm-chain.sh --preflight &lt;chain&#124;--all&gt;</code> |
| <code>interface.14.cli-bridge-deploy-evm-chain-j55</code> | <code>cli_command</code> | <code>services/bridge/deploy/deploy-evm-chain.sh</code> | <code>deploy-evm-chain.sh [--preflight] &lt;chain&#124;--all&gt;</code> |
| <code>interface.14.cli-bridge-deploy-solana-program-j55</code> | <code>cli_command</code> | <code>services/bridge/deploy/deploy-solana-program.sh</code> | <code>deploy-solana-program.sh [--preflight]</code> |
| <code>interface.14.cli-bridge-overlay-j55</code> | <code>cli_command</code> | <code>services/bridge/deploy/overlay.sh</code> | <code>source services/bridge/deploy/overlay.sh</code> |
| <code>interface.14.cli-bridge-verify-evm-chain-j55</code> | <code>cli_command</code> | <code>services/bridge/deploy/verify-evm-chain.sh</code> | <code>verify-evm-chain.sh [--preflight] &lt;chain&#124;--all&gt;</code> |
| <code>interface.14.env-bridge-bridge-deploy-overlay-j55</code> | <code>env_var</code> | <code>services/bridge/deploy/chainconfig/overlay.go</code> | <code>BRIDGE_DEPLOY_OVERLAY</code> |
| <code>interface.14.env-bridge-paxeer-bridge-app-env-j55</code> | <code>env_var</code> | <code>services/bridge/deploy/checklist.sh</code> | <code>PAXEER_BRIDGE_APP_ENV</code> |
| <code>interface.14.env-bridge-paxeer-bridge-attestor-manifest-j55</code> | <code>env_var</code> | <code>services/bridge/deploy/checklist.sh</code> | <code>PAXEER_BRIDGE_ATTESTOR_MANIFEST</code> |
| <code>interface.14.env-bridge-paxeer-bridge-deployment-record-dir-j55</code> | <code>env_var</code> | <code>services/bridge/deploy/deploy-evm-chain.sh</code> | <code>PAXEER_BRIDGE_DEPLOYMENT_RECORD_DIR</code> |
| <code>interface.14.env-bridge-paxeer-bridge-deployment-record-j55</code> | <code>env_var</code> | <code>services/bridge/deploy/deploy-evm-chain.sh</code> | <code>PAXEER_BRIDGE_DEPLOYMENT_RECORD</code> |
| <code>interface.14.env-bridge-paxeer-bridge-evm-chains-root-j55</code> | <code>env_var</code> | <code>services/bridge/deploy/deploy-evm-chain.sh</code> | <code>PAXEER_BRIDGE_EVM_CHAINS_ROOT</code> |
| <code>interface.14.env-bridge-paxeer-bridge-governance-authority-j55</code> | <code>env_var</code> | <code>services/bridge/deploy/checklist.sh</code> | <code>PAXEER_BRIDGE_GOVERNANCE_AUTHORITY</code> |
| <code>interface.14.env-bridge-paxeer-bridge-paxeer-rpc-url-j55</code> | <code>env_var</code> | <code>services/bridge/deploy/checklist.sh</code> | <code>PAXEER_BRIDGE_PAXEER_RPC_URL</code> |
| <code>interface.14.env-bridge-paxeer-bridge-solana-admin-cli-j55</code> | <code>env_var</code> | <code>services/bridge/deploy/deploy-solana-program.sh</code> | <code>PAXEER_BRIDGE_SOLANA_ADMIN_CLI</code> |
| <code>interface.14.env-bridge-paxeer-bridge-solana-chains-root-j55</code> | <code>env_var</code> | <code>services/bridge/deploy/deploy-solana-program.sh</code> | <code>PAXEER_BRIDGE_SOLANA_CHAINS_ROOT</code> |
| <code>interface.14.env-bridge-paxeer-bridge-solana-executable-wait-seconds-j55</code> | <code>env_var</code> | <code>services/bridge/deploy/deploy-solana-program.sh</code> | <code>PAXEER_BRIDGE_SOLANA_EXECUTABLE_WAIT_SECONDS</code> |
| <code>interface.14.env-bridge-paxeer-bridge-solana-program-keypair-file-j55</code> | <code>env_var</code> | <code>services/bridge/deploy/deploy-solana-program.sh</code> | <code>PAXEER_BRIDGE_SOLANA_PROGRAM_KEYPAIR_FILE</code> |
| <code>interface.14.env-bridge-paxeer-bridge-vault-assets-j55</code> | <code>env_var</code> | <code>services/bridge/deploy/deploy-evm-chain.sh</code> | <code>PAXEER_BRIDGE_VAULT_ASSETS</code> |
| <code>interface.14.env-bridge-paxeer-bridge-vault-attestors-j55</code> | <code>env_var</code> | <code>services/bridge/deploy/deploy-evm-chain.sh</code> | <code>PAXEER_BRIDGE_VAULT_ATTESTORS</code> |
| <code>interface.14.env-bridge-paxeer-bridge-vault-owner-j55</code> | <code>env_var</code> | <code>services/bridge/deploy/deploy-evm-chain.sh</code> | <code>PAXEER_BRIDGE_VAULT_OWNER</code> |
| <code>interface.14.env-bridge-paxeer-bridge-vault-per-tx-caps-j55</code> | <code>env_var</code> | <code>services/bridge/deploy/deploy-evm-chain.sh</code> | <code>PAXEER_BRIDGE_VAULT_PER_TX_CAPS</code> |
| <code>interface.14.env-bridge-paxeer-bridge-vault-threshold-j55</code> | <code>env_var</code> | <code>services/bridge/deploy/deploy-evm-chain.sh</code> | <code>PAXEER_BRIDGE_VAULT_THRESHOLD</code> |
| <code>interface.14.env-bridge-paxeer-bridge-vault-total-caps-j55</code> | <code>env_var</code> | <code>services/bridge/deploy/deploy-evm-chain.sh</code> | <code>PAXEER_BRIDGE_VAULT_TOTAL_CAPS</code> |
| <code>interface.14.fmt-bridge-relayer-solana-example-j55</code> | <code>file_format</code> | <code>services/bridge/deploy/relayer-solana.example.json</code> | <code>example relayer configuration for the Solana leg</code> |

| Field | Rule |
|---|---|
| interface | services/bridge/deploy: deploy-evm-chain.sh, deploy-solana-program.sh, verify-evm-chain.sh and checklist.sh write and check deployment records; paxeer-bridge-proposals renders governance proposals; paxeer-bridge-app-env renders and checks the app env. |
| storage | Deployment records &lt;chain&gt;.json under PAXEER_BRIDGE_DEPLOYMENT_RECORD_DIR; attestors.json manifest; no chain state written by the tooling except the deployments themselves. |
| errors | Scripts refuse invalid input with a nonzero exit status; appenv.LoadRecord refuses a chain_id that differs from the registry and a zero vault. |
| events | None. |
| gas | Forge and the Solana CLI estimate gas; the scripts set none. |
| activation | Run by an operator per chain; the app env is rendered after the records exist. |
| invariants | The app env names exactly the vaults and program id of the deployment records; chain ids match chainconfig.Chains. |
| pause | Not applicable; the tooling does not pause contracts. |

