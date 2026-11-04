package wasm_test

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"strings"
	"testing"

	epochwasm "github.com/Sidiora-Labs/Paxeer-X-Network/modules/epoch/client/wasm"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/artifacts/cw1155"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/artifacts/cw721"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/artifacts/native"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/client/wasm"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/client/wasm/bindings"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	oraclewasm "github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/client/wasm"
	tokenfactorywasm "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/client/wasm"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/wasmbinding"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	ethabi "github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func repairM1241Setup() (*keeper.Keeper, sdk.Context, *wasmbinding.QueryPlugin, sdk.AccAddress, common.Address) {
	app := testkeeper.EVMTestApp
	k := &app.EvmKeeper
	ctx, _ := app.GetContextForDeliverTx(nil).CacheContext()
	account := sdk.AccAddress(append(make([]byte, 19), 71))
	evm := common.Address{71}
	k.SetAddressMapping(ctx, account, evm)
	plugin := wasmbinding.NewQueryPlugin(
		oraclewasm.NewOracleWasmQueryHandler(&app.OracleKeeper),
		epochwasm.NewEpochWasmQueryHandler(&app.EpochKeeper),
		tokenfactorywasm.NewTokenFactoryWasmQueryHandler(&app.TokenFactoryKeeper),
		wasm.NewEVMQueryHandler(k), app.StakingKeeper,
	)
	return k, ctx, plugin, account, evm
}

func repairM1241JSON(t *testing.T, query bindings.PaxEVMQuery) []byte {
	t.Helper()
	bz, err := json.Marshal(query)
	require.NoError(t, err)
	return bz
}

// Start with the production binding's JSON, then alter a field as an untrusted
// CosmWasm contract can. The real dispatcher performs decoding and routing.
func repairM1241Field(t *testing.T, query bindings.PaxEVMQuery, field string, value json.RawMessage) []byte {
	t.Helper()
	var envelope map[string]map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(repairM1241JSON(t, query), &envelope))
	require.Len(t, envelope, 1)
	for _, request := range envelope {
		if value == nil {
			delete(request, field)
		} else {
			request[field] = value
		}
	}
	bz, err := json.Marshal(envelope)
	require.NoError(t, err)
	return bz
}

func TestRepairM_1241_OptionalIntegersJSON(t *testing.T) {
	_, ctx, plugin, account, _ := repairM1241Setup()
	a := account.String()
	one := sdk.OneInt()
	cases := []struct {
		name, field string
		query       bindings.PaxEVMQuery
	}{
		{"erc20_transfer", "amount", bindings.PaxEVMQuery{ERC20TransferPayload: &bindings.ERC20TransferPayloadRequest{Recipient: a, Amount: &one}}},
		{"erc20_transfer_from", "amount", bindings.PaxEVMQuery{ERC20TransferFromPayload: &bindings.ERC20TransferFromPayloadRequest{Owner: a, Recipient: a, Amount: &one}}},
		{"erc20_approve", "amount", bindings.PaxEVMQuery{ERC20ApprovePayload: &bindings.ERC20ApprovePayloadRequest{Spender: a, Amount: &one}}},
		{"erc1155_transfer", "amount", bindings.PaxEVMQuery{ERC1155TransferPayload: &bindings.ERC1155TransferPayloadRequest{From: a, Recipient: a, TokenID: "1", Amount: &one}}},
		{"erc721_royalty", "sale_price", bindings.PaxEVMQuery{ERC721RoyaltyInfo: &bindings.ERC721RoyaltyInfoRequest{Caller: a, TokenID: "1", SalePrice: &one}}},
		{"erc1155_royalty", "sale_price", bindings.PaxEVMQuery{ERC1155RoyaltyInfo: &bindings.ERC1155RoyaltyInfoRequest{Caller: a, TokenID: "1", SalePrice: &one}}},
	}
	overflow := new(big.Int).Lsh(big.NewInt(1), 256).String()
	invalid := []struct {
		name  string
		value json.RawMessage
		typed bool
	}{
		{"omitted", nil, true}, {"null", json.RawMessage(`null`), true},
		{"negative", json.RawMessage(`"-1"`), true},
		{"overflow", json.RawMessage(`"` + overflow + `"`), false},
	}
	for _, tc := range cases {
		for _, bad := range invalid {
			t.Run(tc.name+"/"+bad.name, func(t *testing.T) {
				res, err := plugin.HandleEVMQuery(ctx, repairM1241Field(t, tc.query, tc.field, bad.value))
				require.Error(t, err)
				require.Nil(t, res)
				if bad.typed {
					require.ErrorIs(t, err, sdkerrors.ErrInvalidRequest)
					require.Contains(t, err.Error(), tc.field)
				}
			})
		}
	}
	batch := bindings.PaxEVMQuery{ERC1155BatchTransferPayload: &bindings.ERC1155BatchTransferPayloadRequest{From: a, Recipient: a, TokenIDs: []string{"1", "2"}, Amounts: []*sdk.Int{&one, &one}}}
	for name, value := range map[string]json.RawMessage{
		"omitted": nil, "null": json.RawMessage(`null`),
		"first_null": json.RawMessage(`[null,"1"]`), "second_null": json.RawMessage(`["1",null]`),
		"negative": json.RawMessage(`["1","-1"]`), "overflow": json.RawMessage(`["1","` + overflow + `"]`),
	} {
		t.Run("batch/"+name, func(t *testing.T) {
			res, err := plugin.HandleEVMQuery(ctx, repairM1241Field(t, batch, "amounts", value))
			require.Error(t, err)
			require.Nil(t, res)
			if name == "first_null" || name == "second_null" || name == "negative" {
				require.ErrorIs(t, err, sdkerrors.ErrInvalidRequest)
			}
		})
	}
}

func TestRepairM_1241_UninitializedIntegers(t *testing.T) {
	k, ctx, _, account, _ := repairM1241Setup()
	h := wasm.NewEVMQueryHandler(k)
	a := account.String()
	uninitialized := sdk.Int{}
	// A mutated constructor input can exceed the SDK constructor's bound.
	backing := big.NewInt(1)
	oversized := sdk.NewIntFromBigInt(backing)
	backing.Lsh(backing, 256)
	for name, amount := range map[string]*sdk.Int{"nil": nil, "uninitialized": &uninitialized, "oversized": &oversized} {
		calls := []func() ([]byte, error){
			func() ([]byte, error) { return h.HandleERC20TransferPayload(ctx, a, amount) },
			func() ([]byte, error) { return h.HandleERC20TransferFromPayload(ctx, a, a, amount) },
			func() ([]byte, error) { return h.HandleERC20ApprovePayload(ctx, a, amount) },
			func() ([]byte, error) { return h.HandleERC1155TransferPayload(ctx, a, a, "1", amount) },
			func() ([]byte, error) {
				return h.HandleERC1155BatchTransferPayload(ctx, a, a, []string{"1"}, []*sdk.Int{amount})
			},
			func() ([]byte, error) { return h.HandleERC721RoyaltyInfo(ctx, a, "", "1", amount) },
			func() ([]byte, error) { return h.HandleERC1155RoyaltyInfo(ctx, a, "", "1", amount) },
		}
		t.Run(name, func(t *testing.T) {
			for _, call := range calls {
				res, err := call()
				require.ErrorIs(t, err, sdkerrors.ErrInvalidRequest)
				require.Nil(t, res)
			}
		})
	}
}

func TestRepairM_1241_TokenIDsJSON(t *testing.T) {
	_, ctx, plugin, account, _ := repairM1241Setup()
	a := account.String()
	one := sdk.OneInt()
	cases := []struct {
		name, field string
		query       bindings.PaxEVMQuery
	}{
		{"erc721_owner", "token_id", bindings.PaxEVMQuery{ERC721Owner: &bindings.ERC721OwnerRequest{Caller: a, TokenID: "1"}}},
		{"erc721_transfer", "token_id", bindings.PaxEVMQuery{ERC721TransferPayload: &bindings.ERC721TransferPayloadRequest{From: a, Recipient: a, TokenID: "1"}}},
		{"erc721_approve", "token_id", bindings.PaxEVMQuery{ERC721ApprovePayload: &bindings.ERC721ApprovePayloadRequest{Spender: a, TokenID: "1"}}},
		{"erc721_approved", "token_id", bindings.PaxEVMQuery{ERC721Approved: &bindings.ERC721ApprovedRequest{Caller: a, TokenID: "1"}}},
		{"erc721_uri", "token_id", bindings.PaxEVMQuery{ERC721Uri: &bindings.ERC721UriRequest{Caller: a, TokenID: "1"}}},
		{"erc721_royalty", "token_id", bindings.PaxEVMQuery{ERC721RoyaltyInfo: &bindings.ERC721RoyaltyInfoRequest{Caller: a, TokenID: "1", SalePrice: &one}}},
		{"erc1155_transfer", "token_id", bindings.PaxEVMQuery{ERC1155TransferPayload: &bindings.ERC1155TransferPayloadRequest{From: a, Recipient: a, TokenID: "1", Amount: &one}}},
		{"erc1155_batch_transfer", "token_ids", bindings.PaxEVMQuery{ERC1155BatchTransferPayload: &bindings.ERC1155BatchTransferPayloadRequest{From: a, Recipient: a, TokenIDs: []string{"1", "2"}, Amounts: []*sdk.Int{&one, &one}}}},
		{"erc1155_balance", "token_id", bindings.PaxEVMQuery{ERC1155BalanceOf: &bindings.ERC1155BalanceOfRequest{Caller: a, Account: a, TokenID: "1"}}},
		{"erc1155_batch_balance", "token_ids", bindings.PaxEVMQuery{ERC1155BalanceOfBatch: &bindings.ERC1155BalanceOfBatchRequest{Caller: a, Accounts: []string{a, a}, TokenIDs: []string{"1", "2"}}}},
		{"erc1155_uri", "token_id", bindings.PaxEVMQuery{ERC1155Uri: &bindings.ERC1155UriRequest{Caller: a, TokenID: "1"}}},
		{"erc1155_supply", "token_id", bindings.PaxEVMQuery{ERC1155TotalSupplyForToken: &bindings.ERC1155TotalSupplyForTokenRequest{Caller: a, TokenID: "1"}}},
		{"erc1155_exists", "token_id", bindings.PaxEVMQuery{ERC1155TokenExists: &bindings.ERC1155TokenExistsRequest{Caller: a, TokenID: "1"}}},
		{"erc1155_royalty", "token_id", bindings.PaxEVMQuery{ERC1155RoyaltyInfo: &bindings.ERC1155RoyaltyInfoRequest{Caller: a, TokenID: "1", SalePrice: &one}}},
	}
	for _, tc := range cases {
		for _, bad := range []string{"-1", "-" + strings.Repeat("9", 77), new(big.Int).Lsh(big.NewInt(1), 256).String(), "", "invalid"} {
			positions := 1
			if tc.field == "token_ids" {
				positions = 2
			}
			for i := 0; i < positions; i++ {
				t.Run(tc.name+"/"+bad+"/"+string(rune('0'+i)), func(t *testing.T) {
					var value any = bad
					if tc.field == "token_ids" {
						ids := []string{"1", "2"}
						ids[i] = bad
						value = ids
					}
					encoded, err := json.Marshal(value)
					require.NoError(t, err)
					res, err := plugin.HandleEVMQuery(ctx, repairM1241Field(t, tc.query, tc.field, encoded))
					require.ErrorIs(t, err, sdkerrors.ErrInvalidRequest)
					require.Contains(t, err.Error(), "token ID")
					require.Nil(t, res)
				})
			}
		}
	}
}

func TestRepairM_1241_Uint256PayloadBoundaries(t *testing.T) {
	_, ctx, plugin, account, evm := repairM1241Setup()
	a := account.String()
	erc20, err := native.NativeMetaData.GetAbi()
	require.NoError(t, err)
	erc721, err := cw721.Cw721MetaData.GetAbi()
	require.NoError(t, err)
	erc1155, err := cw1155.Cw1155MetaData.GetAbi()
	require.NoError(t, err)
	maximum := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	// Derive zero from its ABI word just as the consumer does. A plain
	// big.Int zero has a nil backing slice, unlike an ABI-decoded zero.
	zero := new(big.Int).SetBytes(common.Hash{}.Bytes())
	for _, value := range []*big.Int{zero, maximum} {
		amount := sdk.NewIntFromBigInt(value)
		id := value.String()
		cases := []struct {
			name, method string
			abi          *ethabi.ABI
			query        bindings.PaxEVMQuery
			args         []any
		}{
			{"erc20_transfer", "transfer", erc20, bindings.PaxEVMQuery{ERC20TransferPayload: &bindings.ERC20TransferPayloadRequest{Recipient: a, Amount: &amount}}, []any{evm, value}},
			{"erc20_transfer_from", "transferFrom", erc20, bindings.PaxEVMQuery{ERC20TransferFromPayload: &bindings.ERC20TransferFromPayloadRequest{Owner: a, Recipient: a, Amount: &amount}}, []any{evm, evm, value}},
			{"erc20_approve", "approve", erc20, bindings.PaxEVMQuery{ERC20ApprovePayload: &bindings.ERC20ApprovePayloadRequest{Spender: a, Amount: &amount}}, []any{evm, value}},
			{"erc721_transfer", "transferFrom", erc721, bindings.PaxEVMQuery{ERC721TransferPayload: &bindings.ERC721TransferPayloadRequest{From: a, Recipient: a, TokenID: id}}, []any{evm, evm, value}},
			{"erc721_approve", "approve", erc721, bindings.PaxEVMQuery{ERC721ApprovePayload: &bindings.ERC721ApprovePayloadRequest{Spender: a, TokenID: id}}, []any{evm, value}},
			{"erc1155_transfer", "safeTransferFrom", erc1155, bindings.PaxEVMQuery{ERC1155TransferPayload: &bindings.ERC1155TransferPayloadRequest{From: a, Recipient: a, TokenID: id, Amount: &amount}}, []any{evm, evm, value, value, []byte("0x0")}},
			{"erc1155_batch", "safeBatchTransferFrom", erc1155, bindings.PaxEVMQuery{ERC1155BatchTransferPayload: &bindings.ERC1155BatchTransferPayloadRequest{From: a, Recipient: a, TokenIDs: []string{"0", id}, Amounts: []*sdk.Int{&amount, &amount}}}, []any{evm, evm, []*big.Int{zero, value}, []*big.Int{value, value}, []byte("0x0")}},
		}
		for _, tc := range cases {
			t.Run(tc.name+"/"+id, func(t *testing.T) {
				res, err := plugin.HandleEVMQuery(ctx, repairM1241JSON(t, tc.query))
				require.NoError(t, err)
				var payload bindings.ERCPayloadResponse
				require.NoError(t, json.Unmarshal(res, &payload))
				calldata, err := base64.StdEncoding.DecodeString(payload.EncodedPayload)
				require.NoError(t, err)
				method, err := tc.abi.MethodById(calldata[:4])
				require.NoError(t, err)
				require.Equal(t, tc.method, method.Name)
				args, err := method.Inputs.Unpack(calldata[4:])
				require.NoError(t, err)
				require.Equal(t, tc.args, args)
			})
		}
	}
}

func TestRepairM_1241_RoyaltyBoundaries(t *testing.T) {
	k, ctx, plugin, account, _ := repairM1241Setup()
	contract := common.Address{72}
	// Committed EVM fixture: return (address(0), calldataload(36)). The second
	// ABI argument is the sale price; execution returns its exact uint256 word.
	code, err := hex.DecodeString("600060005260243560205260406000f3")
	require.NoError(t, err)
	k.SetCode(ctx, contract, code)
	maximum := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	for _, value := range []*big.Int{new(big.Int), maximum} {
		amount := sdk.NewIntFromBigInt(value)
		for _, query := range []bindings.PaxEVMQuery{
			{ERC721RoyaltyInfo: &bindings.ERC721RoyaltyInfoRequest{Caller: account.String(), ContractAddress: contract.Hex(), TokenID: value.String(), SalePrice: &amount}},
			{ERC1155RoyaltyInfo: &bindings.ERC1155RoyaltyInfoRequest{Caller: account.String(), ContractAddress: contract.Hex(), TokenID: value.String(), SalePrice: &amount}},
		} {
			res, err := plugin.HandleEVMQuery(ctx, repairM1241JSON(t, query))
			require.NoError(t, err)
			var royalty bindings.ERC721RoyaltyInfoResponse
			require.NoError(t, json.Unmarshal(res, &royalty))
			require.Equal(t, "", royalty.Receiver)
			require.NotNil(t, royalty.RoyaltyAmount)
			require.Equal(t, value, royalty.RoyaltyAmount.BigInt())
		}
	}
}

func TestRepairM_1241_InterfaceIDsJSON(t *testing.T) {
	k, ctx, plugin, account, _ := repairM1241Setup()
	contract := common.Address{73}
	// Committed ERC165 fixture: compare the entire bytes4 ABI word with
	// 0x12345678 followed by zero padding, then return the equality as a bool.
	code, err := hex.DecodeString("6004357f12345678000000000000000000000000000000000000000000000000000000001460005260206000f3")
	require.NoError(t, err)
	k.SetCode(ctx, contract, code)
	query := func(id string) bindings.PaxEVMQuery {
		return bindings.PaxEVMQuery{SupportsInterface: &bindings.SupportsInterfaceRequest{Caller: account.String(), ContractAddress: contract.Hex(), InterfaceID: id}}
	}
	for _, id := range []string{"", "0x", "12", "1234", "123456", "1234567800", "0012345678", strings.Repeat("00", 32), "0x123", "zzzzzzzz"} {
		t.Run("invalid/"+id, func(t *testing.T) {
			res, err := plugin.HandleEVMQuery(ctx, repairM1241JSON(t, query(id)))
			require.Error(t, err)
			require.Nil(t, res)
			decoded, decodeErr := hex.DecodeString(strings.TrimPrefix(id, "0x"))
			if decodeErr == nil && len(decoded) != 4 {
				require.ErrorIs(t, err, sdkerrors.ErrInvalidRequest)
				require.Contains(t, err.Error(), "exactly four bytes")
			}
		})
	}
	for _, id := range []string{"12345678", "0x12345678", "12345679", "78563412", "00000000", "ffffffff", "0xABCDEF01"} {
		t.Run("exact/"+id, func(t *testing.T) {
			res, err := plugin.HandleEVMQuery(ctx, repairM1241JSON(t, query(id)))
			require.NoError(t, err)
			var response bindings.SupportsInterfaceResponse
			require.NoError(t, json.Unmarshal(res, &response))
			require.Equal(t, strings.TrimPrefix(id, "0x") == "12345678", response.Supported)
		})
	}
	// Invalid IDs must fail even when there is no contract to execute.
	k.SetCode(ctx, contract, nil)
	res, err := plugin.HandleEVMQuery(ctx, repairM1241JSON(t, query("1234567800")))
	require.ErrorIs(t, err, sdkerrors.ErrInvalidRequest)
	require.Nil(t, res)
}
