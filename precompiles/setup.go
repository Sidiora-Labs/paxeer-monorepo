package precompiles

import (
	"bytes"
	"embed"
	"fmt"
	"sync"

	spottypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/addr"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/bank"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/distribution"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/feetoken"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/gov"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/ibc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/json"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/launchpad"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxanchor"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxbridge"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxcustody"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxexchange"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxverify"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/oracle"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/p256"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/perps"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/pointer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/pointerview"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/pricefeed"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/solo"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/spot"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/staking"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/wasmd"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/xweb"
	"github.com/ethereum/go-ethereum/accounts/abi"
	ecommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"golang.org/x/mod/semver"
)

// XWebUpgrade is the upgrade that brings the xweb precompile. A custom
// precompile set built for an earlier latest upgrade carries no entry at
// xweb.XWebAddress.
const XWebUpgrade = "v6.8"

// FeeTokenUpgrade is the upgrade that brings the fee-token precompile. A custom
// precompile set built for an earlier latest upgrade carries no entry at
// feetoken.FeeTokenAddress, and the entry carries a version at this upgrade so
// that execution below its height leaves the precompile out.
const FeeTokenUpgrade = "v6.7"

// MergeUpgrade is the plan that merges the LayerX kernel into the chain and
// brings the MergeAddressTable precompiles online. It equals the plan name the
// node registers, and the merge entries carry a version at this plan so that
// the keeper's activation entry for the plan withholds them below its height.
const MergeUpgrade = pcommon.MergeUpgrade

// MergeAddress is one precompile address the merge plan brings online and the
// name its precompile reports through GetName.
type MergeAddress struct {
	Address ecommon.Address
	Name    string
}

// MergeAddressTable lists the precompiles the merge plan brings online: the
// five construct addresses and the eight agent-function addresses.
var MergeAddressTable = []MergeAddress{
	{ecommon.HexToAddress("0x000000000000000000000000000000000000101A"), "layerxledger"},
	{ecommon.HexToAddress("0x000000000000000000000000000000000000101B"), "layerxpay"},
	{ecommon.HexToAddress("0x000000000000000000000000000000000000101C"), "layerxauth"},
	{ecommon.HexToAddress("0x000000000000000000000000000000000000101D"), "layerxprograms"},
	{ecommon.HexToAddress("0x000000000000000000000000000000000000101E"), "layerxchains"},
	{ecommon.HexToAddress("0x0000000000000000000000000000000000000901"), "orob"},
	{ecommon.HexToAddress("0x0000000000000000000000000000000000000902"), "clearing"},
	{ecommon.HexToAddress("0x0000000000000000000000000000000000000903"), "oracle aggregator"},
	{ecommon.HexToAddress("0x0000000000000000000000000000000000000904"), "pofq"},
	{ecommon.HexToAddress("0x0000000000000000000000000000000000000905"), "scheduler"},
	{ecommon.HexToAddress("0x0000000000000000000000000000000000000906"), "streams"},
	{ecommon.HexToAddress("0x0000000000000000000000000000000000000907"), "tee attestor"},
	{ecommon.HexToAddress("0x0000000000000000000000000000000000000908"), "eip712"},
}

// MergeConstructor builds the precompile served at one MergeAddressTable
// address from the precompile keepers and the merge plan name.
type MergeConstructor func(keepers utils.Keepers, planName string) (IPrecompile, error)

var (
	mergeConstructorsMtx sync.RWMutex
	mergeConstructors    = map[ecommon.Address]MergeConstructor{}
)

func MissingMergeConstructors() []MergeAddress {
	mergeConstructorsMtx.RLock()
	defer mergeConstructorsMtx.RUnlock()
	missing := make([]MergeAddress, 0, len(MergeAddressTable))
	for _, entry := range MergeAddressTable {
		if _, registered := mergeConstructors[entry.Address]; !registered {
			missing = append(missing, entry)
		}
	}
	return missing
}

// RegisterMergeConstructor records the constructor of the precompile served at
// a MergeAddressTable address. An address with no registered constructor is
// not served. It refuses an address outside the table, a nil constructor and
// a second constructor for the same address.
func RegisterMergeConstructor(address ecommon.Address, constructor MergeConstructor) error {
	if _, ok := mergeAddressName(address); !ok {
		return fmt.Errorf("merge constructor for %s outside the merge address table", address.Hex())
	}
	if constructor == nil {
		return fmt.Errorf("nil merge constructor for %s", address.Hex())
	}
	mergeConstructorsMtx.Lock()
	defer mergeConstructorsMtx.Unlock()
	if _, registered := mergeConstructors[address]; registered {
		return fmt.Errorf("merge constructor for %s already registered", address.Hex())
	}
	mergeConstructors[address] = constructor
	return nil
}

func mergeAddressName(address ecommon.Address) (string, bool) {
	for _, entry := range MergeAddressTable {
		if entry.Address == address {
			return entry.Name, true
		}
	}
	return "", false
}

// newMergePrecompiles builds, in MergeAddressTable order, the precompile of
// every address with a registered constructor, and refuses a precompile whose
// address or name differs from its table entry.
func newMergePrecompiles(keepers utils.Keepers) ([]IPrecompile, error) {
	mergeConstructorsMtx.RLock()
	defer mergeConstructorsMtx.RUnlock()
	built := make([]IPrecompile, 0, len(mergeConstructors))
	for _, entry := range MergeAddressTable {
		constructor, registered := mergeConstructors[entry.Address]
		if !registered {
			continue
		}
		p, err := constructor(keepers, MergeUpgrade)
		if err != nil {
			return nil, fmt.Errorf("merge precompile %s at %s: %w", entry.Name, entry.Address.Hex(), err)
		}
		if p == nil {
			return nil, fmt.Errorf("merge precompile %s at %s: constructor returned no precompile", entry.Name, entry.Address.Hex())
		}
		if p.Address() != entry.Address || p.GetName() != entry.Name {
			return nil, fmt.Errorf("merge precompile %s at %s: constructor built %s at %s",
				entry.Name, entry.Address.Hex(), p.GetName(), p.Address().Hex())
		}
		built = append(built, p)
	}
	return built, nil
}

var SetupMtx = &sync.Mutex{}
var Initialized = false

type PrecompileInfo struct {
	ABI     abi.ABI
	Address ecommon.Address
}

// PrecompileNamesToInfo is Populated by InitializePrecompiles
var PrecompileNamesToInfo = map[string]PrecompileInfo{}

//go:embed */abi.json perps/abi/abi.json pricefeed/abi/abi.json spot/abi/*.json
var precompileInfoABIs embed.FS

type precompileInfoSource struct {
	address string
	file    string
}

var precompileInfoSources = map[string]precompileInfoSource{
	"bank":            {bank.BankAddress, "bank/abi.json"},
	"wasmd":           {wasmd.WasmdAddress, "wasmd/abi.json"},
	"json":            {json.JSONAddress, "json/abi.json"},
	"addr":            {addr.AddrAddress, "addr/abi.json"},
	"staking":         {staking.StakingAddress, "staking/abi.json"},
	"gov":             {gov.GovAddress, "gov/abi.json"},
	"distribution":    {distribution.DistrAddress, "distribution/abi.json"},
	"oracle":          {oracle.OracleAddress, "oracle/abi.json"},
	"ibc":             {ibc.IBCAddress, "ibc/abi.json"},
	"pointer":         {pointer.PointerAddress, "pointer/abi.json"},
	"pointerview":     {pointerview.PointerViewAddress, "pointerview/abi.json"},
	"p256Verify":      {p256.P256VerifyAddress, "p256/abi.json"},
	"solo":            {solo.SoloAddress, "solo/abi.json"},
	"layerxVerify":    {layerxverify.LayerXVerifyAddress, "layerxverify/abi.json"},
	"layerxCustody":   {layerxcustody.LayerXCustodyAddress, "layerxcustody/abi.json"},
	"layerxAnchor":    {layerxanchor.LayerXAnchorAddress, "layerxanchor/abi.json"},
	"layerxExchange":  {layerxexchange.ExchangeAddress, "layerxexchange/abi.json"},
	"layerxexchange":  {layerxexchange.ExchangeAddress, "layerxexchange/abi.json"},
	"layerxBridge":    {layerxbridge.BridgeAddress, "layerxbridge/abi.json"},
	"launchpad":       {launchpad.LaunchpadAddress, "launchpad/abi.json"},
	"feeToken":        {feetoken.FeeTokenAddress, "feetoken/abi.json"},
	"xweb":            {xweb.XWebAddress, "xweb/abi.json"},
	"perps":           {perps.PerpsAddress, "perps/abi/abi.json"},
	"pricefeed":       {pricefeed.PriceFeedAddress, "pricefeed/abi/abi.json"},
	"spot_pecor":      {spottypes.PECORAddress, "spot/abi/pecor.json"},
	"spot_orders":     {spottypes.PECOROrdersAddress, "spot/abi/orders.json"},
	"spot_stoporders": {spottypes.PECORStopOrdersAddress, "spot/abi/stoporders.json"},
	"spot_quoter":     {spottypes.PECORQuoterAddress, "spot/abi/quoter.json"},
	"spot_vault":      {spottypes.PECORVaultAddress, "spot/abi/vault.json"},
}

var mergeAgentInfoABIs = map[string]string{
	"orob": `[
		{"type":"function","name":"resolveOffset","stateMutability":"pure","inputs":[{"name":"oraclePrice","type":"int256"},{"name":"offsetBps","type":"int16"}],"outputs":[{"name":"absolutePrice","type":"int256"}]},
		{"type":"function","name":"resolveOffsetBatch","stateMutability":"pure","inputs":[{"name":"oraclePrice","type":"int256"},{"name":"offsetsBps","type":"int16[]"}],"outputs":[{"name":"absolutePrices","type":"int256[]"}]},
		{"type":"function","name":"toOffset","stateMutability":"pure","inputs":[{"name":"oraclePrice","type":"int256"},{"name":"absolutePrice","type":"int256"}],"outputs":[{"name":"offsetBps","type":"int16"}]}
	]`,
	"clearing": `[{"type":"function","name":"computeClearing","stateMutability":"pure","inputs":[{"name":"oraclePrice","type":"int256"},{"name":"buyOrders","type":"tuple[]","components":[{"name":"offsetBps","type":"int16"},{"name":"size","type":"uint128"}]},{"name":"sellOrders","type":"tuple[]","components":[{"name":"offsetBps","type":"int16"},{"name":"size","type":"uint128"}]}],"outputs":[{"name":"clearingOffsetBps","type":"int16"},{"name":"clearingPrice","type":"int256"},{"name":"matchedVolume","type":"uint256"}]}]`,
	"oracle aggregator": `[
 {"type":"function","name":"aggregate","stateMutability":"pure","inputs":[{"name":"feeds","type":"tuple[]","components":[{"name":"price","type":"int256"},{"name":"confidence","type":"uint256"},{"name":"timestamp","type":"uint256"}]}],"outputs":[{"name":"price","type":"int256"},{"name":"confidence","type":"uint256"}]},
 {"type":"function","name":"submitPrice","stateMutability":"nonpayable","inputs":[{"name":"feedID","type":"bytes32"},{"name":"price","type":"int256"},{"name":"confidence","type":"uint256"}],"outputs":[]},
 {"type":"function","name":"getValidatorPrice","stateMutability":"view","inputs":[{"name":"feedID","type":"bytes32"}],"outputs":[{"name":"price","type":"int256"},{"name":"quorum","type":"uint256"},{"name":"timestamp","type":"uint256"}]}
 ]`,
	"pofq": `[
	{"type":"function","name":"scoreFill","stateMutability":"pure","inputs":[{"name":"fillPrice","type":"int256"},{"name":"oraclePrice","type":"int256"}],"outputs":[{"name":"score","type":"uint256"}]},
	{"type":"function","name":"scoreBatch","stateMutability":"pure","inputs":[{"name":"fills","type":"tuple[]","components":[{"name":"fillPrice","type":"int256"},{"name":"oraclePrice","type":"int256"},{"name":"size","type":"uint128"}]}],"outputs":[{"name":"avgScore","type":"uint256"},{"name":"totalVolume","type":"uint256"}]},
	{"type":"function","name":"updateRollingScore","stateMutability":"pure","inputs":[{"name":"currentScore","type":"uint256"},{"name":"currentWeight","type":"uint256"},{"name":"newScore","type":"uint256"},{"name":"newWeight","type":"uint256"},{"name":"decayBps","type":"uint16"}],"outputs":[{"name":"updatedScore","type":"uint256"},{"name":"updatedWeight","type":"uint256"}]}
]`,
	"scheduler": `[
{"type":"function","name":"schedule","stateMutability":"payable","inputs":[{"name":"target","type":"address"},{"name":"callData","type":"bytes"},{"name":"executeAtBlock","type":"uint64"},{"name":"gasLimit","type":"uint64"}],"outputs":[{"name":"jobId","type":"uint256"}]},
{"type":"function","name":"cancel","stateMutability":"nonpayable","inputs":[{"name":"jobId","type":"uint256"}],"outputs":[]},
{"type":"function","name":"reschedule","stateMutability":"nonpayable","inputs":[{"name":"jobId","type":"uint256"},{"name":"newBlock","type":"uint64"}],"outputs":[]},
{"type":"function","name":"getJob","stateMutability":"view","inputs":[{"name":"jobId","type":"uint256"}],"outputs":[{"name":"id","type":"uint256"},{"name":"creator","type":"address"},{"name":"target","type":"address"},{"name":"callData","type":"bytes"},{"name":"executeAtBlock","type":"uint64"},{"name":"gasLimit","type":"uint64"},{"name":"deposit","type":"uint256"},{"name":"active","type":"bool"}]},
{"type":"function","name":"pending","stateMutability":"view","inputs":[{"name":"creator","type":"address"}],"outputs":[{"name":"jobIds","type":"uint256[]"}]},
{"type":"event","name":"JobScheduled","anonymous":false,"inputs":[{"name":"jobId","type":"uint256","indexed":true},{"name":"creator","type":"address","indexed":true},{"name":"target","type":"address","indexed":false},{"name":"executeAtBlock","type":"uint64","indexed":false},{"name":"gasLimit","type":"uint64","indexed":false},{"name":"deposit","type":"uint256","indexed":false}]},
{"type":"event","name":"JobCancelled","anonymous":false,"inputs":[{"name":"jobId","type":"uint256","indexed":true},{"name":"creator","type":"address","indexed":true},{"name":"refund","type":"uint256","indexed":false}]},
{"type":"event","name":"JobRescheduled","anonymous":false,"inputs":[{"name":"jobId","type":"uint256","indexed":true},{"name":"newBlock","type":"uint64","indexed":false}]},
{"type":"event","name":"JobExecuted","anonymous":false,"inputs":[{"name":"jobId","type":"uint256","indexed":true},{"name":"creator","type":"address","indexed":true},{"name":"gasUsed","type":"uint64","indexed":false},{"name":"refund","type":"uint256","indexed":false}]},
{"type":"event","name":"JobFailed","anonymous":false,"inputs":[{"name":"jobId","type":"uint256","indexed":true},{"name":"creator","type":"address","indexed":true},{"name":"reason","type":"string","indexed":false}]}
]`,
	"streams": `[
 {"type":"function","name":"open","stateMutability":"nonpayable","inputs":[{"name":"payee","type":"address"},{"name":"token","type":"address"},{"name":"ratePerSecond","type":"uint256"},{"name":"startTime","type":"uint64"},{"name":"stopTime","type":"uint64"},{"name":"cap","type":"uint256"}],"outputs":[{"name":"streamId","type":"uint256"}]},
 {"type":"function","name":"settle","stateMutability":"nonpayable","inputs":[{"name":"streamId","type":"uint256"}],"outputs":[{"name":"paid","type":"uint256"}]},
 {"type":"function","name":"close","stateMutability":"nonpayable","inputs":[{"name":"streamId","type":"uint256"}],"outputs":[{"name":"finalPaid","type":"uint256"}]},
 {"type":"function","name":"updateRate","stateMutability":"nonpayable","inputs":[{"name":"streamId","type":"uint256"},{"name":"newRate","type":"uint256"}],"outputs":[]},
 {"type":"function","name":"accrued","stateMutability":"view","inputs":[{"name":"streamId","type":"uint256"}],"outputs":[{"name":"amount","type":"uint256"}]},
 {"type":"function","name":"getStream","stateMutability":"view","inputs":[{"name":"streamId","type":"uint256"}],"outputs":[{"name":"stream","type":"tuple","components":[{"name":"id","type":"uint256"},{"name":"payer","type":"address"},{"name":"payee","type":"address"},{"name":"token","type":"address"},{"name":"ratePerSecond","type":"uint256"},{"name":"cap","type":"uint256"},{"name":"startTime","type":"uint64"},{"name":"stopTime","type":"uint64"},{"name":"settled","type":"uint256"},{"name":"active","type":"bool"}]}]},
 {"type":"event","name":"StreamOpened","anonymous":false,"inputs":[{"name":"streamId","type":"uint256","indexed":true},{"name":"payer","type":"address","indexed":true},{"name":"payee","type":"address","indexed":true},{"name":"token","type":"address","indexed":false},{"name":"ratePerSecond","type":"uint256","indexed":false},{"name":"cap","type":"uint256","indexed":false}]},
 {"type":"event","name":"StreamSettled","anonymous":false,"inputs":[{"name":"streamId","type":"uint256","indexed":true},{"name":"payee","type":"address","indexed":true},{"name":"paid","type":"uint256","indexed":false},{"name":"committed","type":"uint256","indexed":false}]},
 {"type":"event","name":"StreamClosed","anonymous":false,"inputs":[{"name":"streamId","type":"uint256","indexed":true},{"name":"finalPaid","type":"uint256","indexed":false},{"name":"refund","type":"uint256","indexed":false}]},
 {"type":"event","name":"StreamRateUpdated","anonymous":false,"inputs":[{"name":"streamId","type":"uint256","indexed":true},{"name":"oldRate","type":"uint256","indexed":false},{"name":"newRate","type":"uint256","indexed":false}]}
 ]`,
	"tee attestor": `[
    {"type":"function","name":"verify","stateMutability":"view","inputs":[{"name":"family","type":"uint8"},{"name":"quote","type":"bytes"}],"outputs":[{"name":"att","type":"tuple","components":[{"name":"family","type":"uint8"},{"name":"mrtd","type":"bytes32"},{"name":"reportData","type":"bytes32"},{"name":"timestamp","type":"uint64"},{"name":"debug","type":"bool"}]}]},
    {"type":"function","name":"verifyAndExpect","stateMutability":"view","inputs":[{"name":"family","type":"uint8"},{"name":"quote","type":"bytes"},{"name":"expectedReportData","type":"bytes32"}],"outputs":[{"name":"att","type":"tuple","components":[{"name":"family","type":"uint8"},{"name":"mrtd","type":"bytes32"},{"name":"reportData","type":"bytes32"},{"name":"timestamp","type":"uint64"},{"name":"debug","type":"bool"}]}]},
    {"type":"function","name":"rootOf","stateMutability":"view","inputs":[{"name":"family","type":"uint8"},{"name":"index","type":"uint256"}],"outputs":[{"name":"root","type":"bytes"}]},
    {"type":"function","name":"rootCount","stateMutability":"view","inputs":[{"name":"family","type":"uint8"}],"outputs":[{"name":"count","type":"uint256"}]},
    {"type":"event","name":"AttestationVerified","anonymous":false,"inputs":[{"name":"family","type":"uint8","indexed":true},{"name":"mrtd","type":"bytes32","indexed":true},{"name":"reportData","type":"bytes32","indexed":false},{"name":"timestamp","type":"uint64","indexed":false},{"name":"debug","type":"bool","indexed":false}]}
]`,
	"eip712": `[
		{"type":"function","name":"hashTypedData","stateMutability":"pure","inputs":[{"name":"domainSeparator","type":"bytes32"},{"name":"structHash","type":"bytes32"}],"outputs":[{"name":"digest","type":"bytes32"}]},
		{"type":"function","name":"domainSeparator","stateMutability":"pure","inputs":[{"name":"name","type":"string"},{"name":"version","type":"string"},{"name":"chainId","type":"uint256"},{"name":"verifyingContract","type":"address"}],"outputs":[{"name":"separator","type":"bytes32"}]},
		{"type":"function","name":"recoverTypedSigner","stateMutability":"view","inputs":[{"name":"domainSeparator","type":"bytes32"},{"name":"structHash","type":"bytes32"},{"name":"signature","type":"bytes"}],"outputs":[{"name":"signer","type":"address"}]}
	]`,
}

type IPrecompile interface {
	vm.PrecompiledContract
	GetABI() abi.ABI
	GetName() string
	Address() ecommon.Address
}

func GetCustomPrecompiles(
	latestUpgrade string,
	keepers utils.Keepers,
) map[ecommon.Address]utils.VersionedPrecompiles {
	custom, err := GetCustomPrecompilesE(latestUpgrade, keepers)
	if err != nil {
		panic(err)
	}
	return custom
}

// GetCustomPrecompilesE includes launchpad.GetVersioned at the direct 0x1017
// address; surface runtime calls must reach this same registered implementation.
func GetCustomPrecompilesE(
	latestUpgrade string,
	keepers utils.Keepers,
) (map[ecommon.Address]utils.VersionedPrecompiles, error) {
	merge, err := newMergePrecompiles(keepers)
	if err != nil {
		return nil, err
	}
	custom := map[ecommon.Address]utils.VersionedPrecompiles{
		ecommon.HexToAddress(bank.BankAddress):                   bank.GetVersioned(latestUpgrade, keepers),
		ecommon.HexToAddress(wasmd.WasmdAddress):                 wasmd.GetVersioned(latestUpgrade, keepers),
		ecommon.HexToAddress(json.JSONAddress):                   json.GetVersioned(latestUpgrade, keepers),
		ecommon.HexToAddress(addr.AddrAddress):                   addr.GetVersioned(latestUpgrade, keepers),
		ecommon.HexToAddress(staking.StakingAddress):             staking.GetVersioned(latestUpgrade, keepers),
		ecommon.HexToAddress(gov.GovAddress):                     gov.GetVersioned(latestUpgrade, keepers),
		ecommon.HexToAddress(distribution.DistrAddress):          distribution.GetVersioned(latestUpgrade, keepers),
		ecommon.HexToAddress(oracle.OracleAddress):               oracle.GetVersioned(latestUpgrade, keepers),
		ecommon.HexToAddress(ibc.IBCAddress):                     ibc.GetVersioned(latestUpgrade, keepers),
		ecommon.HexToAddress(pointer.PointerAddress):             pointer.GetVersioned(latestUpgrade, keepers),
		ecommon.HexToAddress(pointerview.PointerViewAddress):     pointerview.GetVersioned(latestUpgrade, keepers),
		ecommon.HexToAddress(p256.P256VerifyAddress):             p256.GetVersioned(latestUpgrade, keepers),
		ecommon.HexToAddress(solo.SoloAddress):                   solo.GetVersioned(latestUpgrade, keepers),
		ecommon.HexToAddress(layerxverify.LayerXVerifyAddress):   layerxverify.GetVersioned(latestUpgrade, keepers),
		ecommon.HexToAddress(layerxcustody.LayerXCustodyAddress): layerxcustody.GetVersioned(latestUpgrade, keepers),
		ecommon.HexToAddress(layerxanchor.LayerXAnchorAddress):   layerxanchor.GetVersioned(latestUpgrade, keepers),
		ecommon.HexToAddress(layerxexchange.ExchangeAddress):     layerxexchange.GetVersioned(latestUpgrade, keepers),
		ecommon.HexToAddress(layerxbridge.BridgeAddress):         layerxbridge.GetVersioned(latestUpgrade, keepers),
		ecommon.HexToAddress(launchpad.LaunchpadAddress):         launchpad.GetVersioned(latestUpgrade, keepers),
	}
	if semver.Compare(latestUpgrade, FeeTokenUpgrade) >= 0 {
		versioned := feetoken.GetVersioned(latestUpgrade, keepers)
		versioned[FeeTokenUpgrade] = versioned[latestUpgrade]
		custom[ecommon.HexToAddress(feetoken.FeeTokenAddress)] = versioned
	}
	if semver.Compare(latestUpgrade, XWebUpgrade) >= 0 {
		custom[ecommon.HexToAddress(xweb.XWebAddress)] = xweb.GetVersioned(latestUpgrade, keepers)
	}
	for _, p := range merge {
		custom[p.Address()] = utils.VersionedPrecompiles{latestUpgrade: p, MergeUpgrade: p}
	}
	return custom, nil
}

func InitializePrecompiles(
	dryRun bool,
	keepers utils.Keepers,
) error {
	SetupMtx.Lock()
	defer SetupMtx.Unlock()
	if Initialized {
		panic("precompiles already initialized")
	}
	bankp, err := bank.NewPrecompile(keepers)
	if err != nil {
		return err
	}
	wasmdp, err := wasmd.NewPrecompile(keepers)
	if err != nil {
		return err
	}
	jsonp, err := json.NewPrecompile(keepers)
	if err != nil {
		return err
	}
	addrp, err := addr.NewPrecompile(keepers)
	if err != nil {
		return err
	}
	stakingp, err := staking.NewPrecompile(keepers)
	if err != nil {
		return err
	}
	govp, err := gov.NewPrecompile(keepers)
	if err != nil {
		return err
	}
	distrp, err := distribution.NewPrecompile(keepers)
	if err != nil {
		return err
	}
	oraclep, err := oracle.NewPrecompile(keepers)
	if err != nil {
		return err
	}
	ibcp, err := ibc.NewPrecompile(keepers)
	if err != nil {
		return err
	}
	pointerp, err := pointer.NewPrecompile(keepers)
	if err != nil {
		return err
	}
	pointerviewp, err := pointerview.NewPrecompile(keepers)
	if err != nil {
		return err
	}

	p256p, err := p256.NewPrecompile(keepers)
	if err != nil {
		return err
	}

	layerxverifyp, err := layerxverify.NewPrecompile(keepers)
	if err != nil {
		return err
	}

	layerxanchorp, err := layerxanchor.NewPrecompile(keepers)
	if err != nil {
		return err
	}

	layerxcustodyp, err := layerxcustody.NewPrecompile(keepers)
	if err != nil {
		return err
	}

	layerxexchangep, err := layerxexchange.NewPrecompile(keepers)
	if err != nil {
		return err
	}

	perpsp, err := perps.NewPrecompile(keepers)
	if err != nil {
		return err
	}

	pricefeedp, err := pricefeed.NewPrecompile(keepers)
	if err != nil {
		return err
	}

	spotABIs := spot.PrecompileABIs()

	// A dry run only records names and ABIs, so it builds the bridge and
	// launchpad precompiles without their keepers; a live run requires them.
	layerxbridgep, err := layerxbridge.NewPrecompile(keepers)
	if err != nil {
		if !dryRun {
			return err
		}
		layerxbridgep = layerxbridge.NewPrecompileWithKeeper(nil)
	}

	launchpadp, err := launchpad.NewPrecompile(keepers)
	if err != nil {
		if !dryRun {
			return err
		}
		if launchpadp, err = launchpad.NewPrecompileWithKeeper(nil, keepers.EVMK()); err != nil {
			return err
		}
	}

	feetokenp, err := feetoken.NewPrecompile(keepers)
	if err != nil {
		if !dryRun {
			return err
		}
		feetokenp = feetoken.NewPrecompileWithKeeper(nil)
	}

	xwebp, err := xweb.NewPrecompile(keepers)
	if err != nil {
		return err
	}

	PrecompileNamesToInfo[feetokenp.GetName()] = PrecompileInfo{ABI: feetokenp.GetABI(), Address: feetokenp.Address()}
	PrecompileNamesToInfo[bankp.GetName()] = PrecompileInfo{ABI: bankp.GetABI(), Address: bankp.Address()}
	PrecompileNamesToInfo[wasmdp.GetName()] = PrecompileInfo{ABI: wasmdp.GetABI(), Address: wasmdp.Address()}
	PrecompileNamesToInfo[jsonp.GetName()] = PrecompileInfo{ABI: jsonp.GetABI(), Address: jsonp.Address()}
	PrecompileNamesToInfo[addrp.GetName()] = PrecompileInfo{ABI: addrp.GetABI(), Address: addrp.Address()}
	PrecompileNamesToInfo[stakingp.GetName()] = PrecompileInfo{ABI: stakingp.GetABI(), Address: stakingp.Address()}
	PrecompileNamesToInfo[govp.GetName()] = PrecompileInfo{ABI: govp.GetABI(), Address: govp.Address()}
	PrecompileNamesToInfo[distrp.GetName()] = PrecompileInfo{ABI: distrp.GetABI(), Address: distrp.Address()}
	PrecompileNamesToInfo[oraclep.GetName()] = PrecompileInfo{ABI: oraclep.GetABI(), Address: oraclep.Address()}
	PrecompileNamesToInfo[ibcp.GetName()] = PrecompileInfo{ABI: ibcp.GetABI(), Address: ibcp.Address()}
	PrecompileNamesToInfo[pointerp.GetName()] = PrecompileInfo{ABI: pointerp.GetABI(), Address: pointerp.Address()}
	PrecompileNamesToInfo[pointerviewp.GetName()] = PrecompileInfo{ABI: pointerviewp.GetABI(), Address: pointerviewp.Address()}
	PrecompileNamesToInfo[p256p.GetName()] = PrecompileInfo{ABI: p256p.GetABI(), Address: p256p.Address()}
	PrecompileNamesToInfo[layerxverifyp.GetName()] = PrecompileInfo{ABI: layerxverifyp.GetABI(), Address: layerxverifyp.Address()}
	PrecompileNamesToInfo[layerxcustodyp.GetName()] = PrecompileInfo{ABI: layerxcustodyp.GetABI(), Address: layerxcustodyp.Address()}
	PrecompileNamesToInfo[layerxanchorp.GetName()] = PrecompileInfo{ABI: layerxanchorp.GetABI(), Address: layerxanchorp.Address()}
	PrecompileNamesToInfo[layerxexchangep.GetName()] = PrecompileInfo{ABI: layerxexchangep.GetABI(), Address: layerxexchangep.Address()}
	PrecompileNamesToInfo["layerxexchange"] = PrecompileInfo{ABI: layerxexchangep.GetABI(), Address: layerxexchangep.Address()}
	PrecompileNamesToInfo[perpsp.GetName()] = PrecompileInfo{ABI: perpsp.GetABI(), Address: perpsp.Address()}
	PrecompileNamesToInfo[pricefeedp.GetName()] = PrecompileInfo{ABI: pricefeedp.GetABI(), Address: pricefeedp.Address()}
	PrecompileNamesToInfo["spot_pecor"] = PrecompileInfo{ABI: spotABIs["spot_pecor"], Address: ecommon.HexToAddress(spottypes.PECORAddress)}
	PrecompileNamesToInfo["spot_orders"] = PrecompileInfo{ABI: spotABIs["spot_orders"], Address: ecommon.HexToAddress(spottypes.PECOROrdersAddress)}
	PrecompileNamesToInfo["spot_stoporders"] = PrecompileInfo{ABI: spotABIs["spot_stoporders"], Address: ecommon.HexToAddress(spottypes.PECORStopOrdersAddress)}
	PrecompileNamesToInfo["spot_quoter"] = PrecompileInfo{ABI: spotABIs["spot_quoter"], Address: ecommon.HexToAddress(spottypes.PECORQuoterAddress)}
	PrecompileNamesToInfo["spot_vault"] = PrecompileInfo{ABI: spotABIs["spot_vault"], Address: ecommon.HexToAddress(spottypes.PECORVaultAddress)}
	PrecompileNamesToInfo[layerxbridgep.GetName()] = PrecompileInfo{ABI: layerxbridgep.GetABI(), Address: layerxbridgep.Address()}
	PrecompileNamesToInfo[launchpadp.GetName()] = PrecompileInfo{ABI: launchpadp.GetABI(), Address: launchpadp.Address()}
	PrecompileNamesToInfo[xwebp.GetName()] = PrecompileInfo{ABI: xwebp.GetABI(), Address: xwebp.Address()}

	merge, err := newMergePrecompiles(keepers)
	if err != nil {
		return err
	}
	for _, p := range merge {
		PrecompileNamesToInfo[p.GetName()] = PrecompileInfo{ABI: p.GetABI(), Address: p.Address()}
	}

	mergeActive := false
	if provider, ok := keepers.(interface{ MergePlanActive() bool }); !dryRun && ok && len(merge) > 0 {
		mergeActive = provider.MergePlanActive()
	}

	if !dryRun {
		addPrecompileToVM(feetokenp)
		addPrecompileToVM(bankp)
		addPrecompileToVM(wasmdp)
		addPrecompileToVM(jsonp)
		addPrecompileToVM(addrp)
		addPrecompileToVM(stakingp)
		addPrecompileToVM(govp)
		addPrecompileToVM(distrp)
		addPrecompileToVM(oraclep)
		addPrecompileToVM(ibcp)
		addPrecompileToVM(pointerp)
		addPrecompileToVM(pointerviewp)
		addPrecompileToVM(p256p)
		addPrecompileToVM(layerxverifyp)
		addPrecompileToVM(layerxcustodyp)
		addPrecompileToVM(layerxanchorp)
		addPrecompileToVM(layerxexchangep)
		addPrecompileToVM(layerxbridgep)
		addPrecompileToVM(launchpadp)
		addPrecompileToVM(xwebp)
		if mergeActive {
			for _, p := range merge {
				addPrecompileToVM(p)
			}
		}
		Initialized = true
	}
	return nil
}

func GetPrecompileInfo(name string) PrecompileInfo {
	i, err := GetPrecompileInfoE(name)
	if err != nil {
		panic(err)
	}
	return i
}

func GetPrecompileInfoE(name string) (PrecompileInfo, error) {
	if info, ok := PrecompileNamesToInfo[name]; ok {
		return validatePrecompileInfo(name, info)
	}
	source, found := precompileInfoSources[name]
	if !found {
		for _, entry := range MergeAddressTable {
			if entry.Name == name {
				source = precompileInfoSource{entry.Address.Hex(), name + "/abi.json"}
				found = true
				break
			}
		}
	}
	if !found {
		return PrecompileInfo{}, fmt.Errorf("%s doesn't exist as a precompile", name)
	}
	var data []byte
	if inline, ok := mergeAgentInfoABIs[name]; ok {
		data = []byte(inline)
	} else {
		var err error
		data, err = precompileInfoABIs.ReadFile(source.file)
		if err != nil {
			return PrecompileInfo{}, fmt.Errorf("%s precompile ABI: %w", name, err)
		}
	}
	parsed, err := abi.JSON(bytes.NewReader(data))
	if err != nil {
		return PrecompileInfo{}, fmt.Errorf("%s precompile ABI: %w", name, err)
	}
	return validatePrecompileInfo(name, PrecompileInfo{ABI: parsed, Address: ecommon.HexToAddress(source.address)})
}

// A registry entry must carry a destination before a caller can build a call.
// Returning the zero address would silently turn it into an ordinary EVM send.
func validatePrecompileInfo(name string, info PrecompileInfo) (PrecompileInfo, error) {
	if info.Address == (ecommon.Address{}) {
		return PrecompileInfo{}, fmt.Errorf("%s precompile has no address", name)
	}
	return info, nil
}

// This function modifies global variable in `vm` module. It should only be called once
// per precompile during initialization
func addPrecompileToVM(p IPrecompile) {
	vm.PrecompiledContractsHomestead[p.Address()] = p
	vm.PrecompiledContractsByzantium[p.Address()] = p
	vm.PrecompiledContractsIstanbul[p.Address()] = p
	vm.PrecompiledContractsBerlin[p.Address()] = p
	vm.PrecompiledContractsCancun[p.Address()] = p
	vm.PrecompiledContractsBLS[p.Address()] = p
	vm.PrecompiledAddressesHomestead = append(vm.PrecompiledAddressesHomestead, p.Address())
	vm.PrecompiledAddressesByzantium = append(vm.PrecompiledAddressesByzantium, p.Address())
	vm.PrecompiledAddressesIstanbul = append(vm.PrecompiledAddressesIstanbul, p.Address())
	vm.PrecompiledAddressesBerlin = append(vm.PrecompiledAddressesBerlin, p.Address())
	vm.PrecompiledAddressesCancun = append(vm.PrecompiledAddressesCancun, p.Address())
}
