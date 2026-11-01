package utils

import (
	"math/big"
	"reflect"
	"testing"

	launchpadkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	layerxbridgekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxbridge/keeper"
	layerxcustodykeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/keeper"
	layerxexchangekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxexchange/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

func TestPrecompileKeeperMethodSetsFrozen(t *testing.T) {
	var _ Keepers = (*EmptyKeepers)(nil)

	for _, tt := range []struct {
		name    string
		iface   reflect.Type
		methods map[string]reflect.Type
	}{
		{
			name:  "Keepers",
			iface: reflect.TypeFor[Keepers](),
			methods: map[string]reflect.Type{
				"AccountK":        reflect.TypeFor[func() AccountKeeper](),
				"AnchorK":         reflect.TypeFor[func() AnchorKeeper](),
				"BankK":           reflect.TypeFor[func() BankKeeper](),
				"BankMS":          reflect.TypeFor[func() BankMsgServer](),
				"ChannelK":        reflect.TypeFor[func() ChannelKeeper](),
				"ClientK":         reflect.TypeFor[func() ClientKeeper](),
				"ConnectionK":     reflect.TypeFor[func() ConnectionKeeper](),
				"DistributionK":   reflect.TypeFor[func() DistributionKeeper](),
				"EVMK":            reflect.TypeFor[func() EVMKeeper](),
				"GovK":            reflect.TypeFor[func() GovKeeper](),
				"GovMS":           reflect.TypeFor[func() GovMsgServer](),
				"LaunchpadK":      reflect.TypeFor[func() *launchpadkeeper.Keeper](),
				"LayerXBridgeK":   reflect.TypeFor[func() *layerxbridgekeeper.Keeper](),
				"LayerXCustodyK":  reflect.TypeFor[func() *layerxcustodykeeper.Keeper](),
				"LayerXExchangeK": reflect.TypeFor[func() *layerxexchangekeeper.Keeper](),
				"OracleK":         reflect.TypeFor[func() OracleKeeper](),
				"StakingK":        reflect.TypeFor[func() StakingKeeper](),
				"StakingQ":        reflect.TypeFor[func() StakingQuerier](),
				"TransferK":       reflect.TypeFor[func() TransferKeeper](),
				"TxConfig":        reflect.TypeFor[func() client.TxConfig](),
				"WasmdK":          reflect.TypeFor[func() WasmdKeeper](),
				"WasmdVK":         reflect.TypeFor[func() WasmdViewKeeper](),
			},
		},
		{
			name:  "EVMKeeper",
			iface: reflect.TypeFor[EVMKeeper](),
			methods: map[string]reflect.Type{
				"BindLayerX":                  reflect.TypeFor[func(sdk.Context, common.Address, [32]byte, [64]byte) (uint64, error)](),
				"GetBalance":                  reflect.TypeFor[func(sdk.Context, sdk.AccAddress) *big.Int](),
				"GetBaseDenom":                reflect.TypeFor[func(sdk.Context) string](),
				"GetCode":                     reflect.TypeFor[func(sdk.Context, common.Address) []byte](),
				"GetCodeHash":                 reflect.TypeFor[func(sdk.Context, common.Address) common.Hash](),
				"GetCosmosGasLimitFromEVMGas": reflect.TypeFor[func(sdk.Context, uint64) uint64](),
				"GetERC1155CW1155Pointer":     reflect.TypeFor[func(sdk.Context, string) (common.Address, uint16, bool)](),
				"GetERC20CW20Pointer":         reflect.TypeFor[func(sdk.Context, string) (common.Address, uint16, bool)](),
				"GetERC20NativePointer":       reflect.TypeFor[func(sdk.Context, string) (common.Address, uint16, bool)](),
				"GetERC721CW721Pointer":       reflect.TypeFor[func(sdk.Context, string) (common.Address, uint16, bool)](),
				"GetEVMAddress":               reflect.TypeFor[func(sdk.Context, sdk.AccAddress) (common.Address, bool)](),
				"GetEVMAddressByLayerXDid":    reflect.TypeFor[func(sdk.Context, [32]byte) (common.Address, bool)](),
				"GetEVMAddressOrDefault":      reflect.TypeFor[func(sdk.Context, sdk.AccAddress) common.Address](),
				"GetEVMGasLimitFromCtx":       reflect.TypeFor[func(sdk.Context) uint64](),
				"GetLayerXBindNonce":          reflect.TypeFor[func(sdk.Context, common.Address) uint64](),
				"GetLayerXDid":                reflect.TypeFor[func(sdk.Context, common.Address) ([32]byte, bool)](),
				"GetPaxAddress":               reflect.TypeFor[func(sdk.Context, common.Address) (sdk.AccAddress, bool)](),
				"GetPaxAddressOrDefault":      reflect.TypeFor[func(sdk.Context, common.Address) sdk.AccAddress](),
				"GetPriorityNormalizer":       reflect.TypeFor[func(sdk.Context) sdk.Dec](),
				"SetAddressMapping":           reflect.TypeFor[func(sdk.Context, sdk.AccAddress, common.Address)](),
				"SetCode":                     reflect.TypeFor[func(sdk.Context, common.Address, []byte)](),
				"SetERC1155CW1155Pointer":     reflect.TypeFor[func(sdk.Context, string, common.Address) error](),
				"SetERC20CW20Pointer":         reflect.TypeFor[func(sdk.Context, string, common.Address) error](),
				"SetERC20NativePointer":       reflect.TypeFor[func(sdk.Context, string, common.Address) error](),
				"SetERC721CW721Pointer":       reflect.TypeFor[func(sdk.Context, string, common.Address) error](),
				"UnbindLayerX":                reflect.TypeFor[func(sdk.Context, common.Address) ([32]byte, uint64, error)](),
				"UpsertERCCW1155Pointer":      reflect.TypeFor[func(sdk.Context, *vm.EVM, string, utils.ERCMetadata) (common.Address, error)](),
				"UpsertERCCW20Pointer":        reflect.TypeFor[func(sdk.Context, *vm.EVM, string, utils.ERCMetadata) (common.Address, error)](),
				"UpsertERCCW721Pointer":       reflect.TypeFor[func(sdk.Context, *vm.EVM, string, utils.ERCMetadata) (common.Address, error)](),
				"UpsertERCNativePointer":      reflect.TypeFor[func(sdk.Context, *vm.EVM, string, utils.ERCMetadata) (common.Address, error)](),
			},
		},
		{
			name:  "BankKeeper",
			iface: reflect.TypeFor[BankKeeper](),
			methods: map[string]reflect.Type{
				"GetAllBalances":   reflect.TypeFor[func(sdk.Context, sdk.AccAddress) sdk.Coins](),
				"GetBalance":       reflect.TypeFor[func(sdk.Context, sdk.AccAddress, string) sdk.Coin](),
				"GetDenomMetaData": reflect.TypeFor[func(sdk.Context, string) (banktypes.Metadata, bool)](),
				"GetSupply":        reflect.TypeFor[func(sdk.Context, string) sdk.Coin](),
				"GetWeiBalance":    reflect.TypeFor[func(sdk.Context, sdk.AccAddress) sdk.Int](),
				"LockedCoins":      reflect.TypeFor[func(sdk.Context, sdk.AccAddress) sdk.Coins](),
				"SendCoins":        reflect.TypeFor[func(sdk.Context, sdk.AccAddress, sdk.AccAddress, sdk.Coins) error](),
				"SendCoinsAndWei":  reflect.TypeFor[func(sdk.Context, sdk.AccAddress, sdk.AccAddress, sdk.Int, sdk.Int) error](),
				"SpendableCoins":   reflect.TypeFor[func(sdk.Context, sdk.AccAddress) sdk.Coins](),
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for name, want := range tt.methods {
				got, ok := tt.iface.MethodByName(name)
				if !ok {
					t.Errorf("%s.%s removed: want %v", tt.name, name, want)
				} else if got.Type != want {
					t.Errorf("%s.%s changed: got %v, want %v", tt.name, name, got.Type, want)
				}
			}
			for i := 0; i < tt.iface.NumMethod(); i++ {
				got := tt.iface.Method(i)
				if _, ok := tt.methods[got.Name]; !ok {
					t.Errorf("%s.%s added: %v", tt.name, got.Name, got.Type)
				}
			}
		})
	}
}
