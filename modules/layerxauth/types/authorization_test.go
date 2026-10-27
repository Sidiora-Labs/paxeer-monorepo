package types

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	dbm "github.com/tendermint/tm-db"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz"
	authzkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz/keeper"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
)

const (
	layerXScopeTestEveryModuleMask = uint64(0x3fe)
	layerXScopeTestSendTypeURL     = "/cosmos.bank.v1beta1.MsgSend"
	layerXScopeTestOwnTypeURL      = "/paxprotocol.paxchain.layerxauth.LayerXScopeAuthorization"
)

func layerXScopeTestAuthorization() LayerXScopeAuthorization {
	return LayerXScopeAuthorization{
		ModuleMask:      0x80,
		OrdinalMin:      1,
		OrdinalMax:      8,
		AssetId:         bytes.Repeat([]byte{0xa1}, 32),
		MaxPerActivity:  "1000",
		MaxTotal:        "340282366920938463463374607431768211455",
		RevocationEpoch: 4,
		GrantId:         bytes.Repeat([]byte{0xc7}, 32),
		MsgTypeUrl:      layerXScopeTestSendTypeURL,
		Kind:            2,
		NotBefore:       100,
		NotAfter:        200,
	}
}

func layerXScopeTestAddress(fill byte) sdk.AccAddress {
	return sdk.AccAddress(bytes.Repeat([]byte{fill}, 20))
}

func layerXScopeTestSend(from sdk.AccAddress) sdk.Msg {
	return banktypes.NewMsgSend(from, layerXScopeTestAddress(0x0e), sdk.NewCoins(sdk.NewInt64Coin("upax", 1)))
}

func layerXScopeTestMultiSend(from sdk.AccAddress) sdk.Msg {
	coins := sdk.NewCoins(sdk.NewInt64Coin("upax", 1))
	return banktypes.NewMsgMultiSend(
		[]banktypes.Input{banktypes.NewInput(from, coins)},
		[]banktypes.Output{banktypes.NewOutput(layerXScopeTestAddress(0x0e), coins)},
	)
}

func TestLayerXScopeValidateBasic(t *testing.T) {
	if err := layerXScopeTestAuthorization().ValidateBasic(); err != nil {
		t.Fatalf("mask 0x80 ordinals 1-8: %v", err)
	}
	empty := layerXScopeTestAuthorization()
	empty.AssetId = nil
	empty.MaxPerActivity = ""
	empty.MaxTotal = ""
	if err := empty.ValidateBasic(); err != nil {
		t.Fatalf("empty asset and caps: %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*LayerXScopeAuthorization)
		want   error
	}{
		{"ordinal_min_above_max", func(a *LayerXScopeAuthorization) { a.OrdinalMin = 9 }, ErrAuthScope},
		{"zero_module_mask", func(a *LayerXScopeAuthorization) { a.ModuleMask = 0 }, ErrAuthScope},
		{"module_zero_bit", func(a *LayerXScopeAuthorization) { a.ModuleMask = 0x81 }, ErrAuthScope},
		{"ordinal_max_above_u16", func(a *LayerXScopeAuthorization) { a.OrdinalMax = 0x10000 }, ErrAuthScope},
		{"per_activity_above_total", func(a *LayerXScopeAuthorization) { a.MaxPerActivity = "11"; a.MaxTotal = "10" }, ErrAuthScope},
		{"asset_id_length", func(a *LayerXScopeAuthorization) { a.AssetId = make([]byte, 31) }, ErrNonCanonical},
		{"grant_id_missing", func(a *LayerXScopeAuthorization) { a.GrantId = nil }, ErrNonCanonical},
		{"grant_id_length", func(a *LayerXScopeAuthorization) { a.GrantId = make([]byte, 33) }, ErrNonCanonical},
		{"amount_leading_zero", func(a *LayerXScopeAuthorization) { a.MaxPerActivity = "01" }, ErrNonCanonical},
		{"amount_sign", func(a *LayerXScopeAuthorization) { a.MaxTotal = "-1" }, ErrNonCanonical},
		{"amount_above_u128", func(a *LayerXScopeAuthorization) { a.MaxTotal = "340282366920938463463374607431768211456" }, ErrNonCanonical},
		{"msg_type_url_empty", func(a *LayerXScopeAuthorization) { a.MsgTypeUrl = "" }, ErrAuthScope},
		{"msg_type_url_ledger", func(a *LayerXScopeAuthorization) {
			a.MsgTypeUrl = sdk.MsgTypeURL(&ledgertypes.MsgUpdateParams{})
		}, ErrUnauthorizedDebit},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			auth := layerXScopeTestAuthorization()
			tc.mutate(&auth)
			if err := auth.ValidateBasic(); !errors.Is(err, tc.want) {
				t.Fatalf("ValidateBasic = %v, want %v", err, tc.want)
			}
		})
	}

	t.Run("authz_grant_and_expiry", func(t *testing.T) {
		registry := cdctypes.NewInterfaceRegistry()
		authz.RegisterInterfaces(registry)
		RegisterInterfaces(registry)
		storeKey := sdk.NewKVStoreKey(authzkeeper.StoreKey)
		keeper := authzkeeper.NewKeeper(storeKey, codec.NewProtoCodec(registry), baseapp.NewMsgServiceRouter())
		expiration := time.Unix(1_900_000_000, 0).UTC()
		ctx := testutil.DefaultContext(storeKey, sdk.NewTransientStoreKey("transient_layerx_scope")).
			WithBlockHeader(tmproto.Header{Time: expiration.Add(-time.Second)})
		granter, grantee := layerXScopeTestAddress(0xd0), layerXScopeTestAddress(0xe0)
		auth := layerXScopeTestAuthorization()
		msgType := auth.MsgTypeURL()
		if msgType != layerXScopeTestSendTypeURL {
			t.Fatalf("MsgTypeURL = %q", msgType)
		}

		grantMsg, err := authz.NewMsgGrant(granter, grantee, &auth, expiration)
		if err != nil {
			t.Fatal(err)
		}
		if err := grantMsg.ValidateBasic(); err != nil {
			t.Fatalf("MsgGrant.ValidateBasic: %v", err)
		}
		bad := layerXScopeTestAuthorization()
		bad.ModuleMask = 0
		badGrant, err := authz.NewMsgGrant(granter, grantee, &bad, expiration)
		if err != nil {
			t.Fatal(err)
		}
		if err := badGrant.ValidateBasic(); !errors.Is(err, ErrAuthScope) {
			t.Fatalf("MsgGrant.ValidateBasic with zero mask = %v, want %v", err, ErrAuthScope)
		}

		if err := keeper.SaveGrant(ctx, grantee, granter, &auth, expiration); err != nil {
			t.Fatalf("SaveGrant: %v", err)
		}
		stored, storedExpiration := keeper.GetCleanAuthorization(ctx, grantee, granter, msgType)
		got, ok := stored.(*LayerXScopeAuthorization)
		if !ok {
			t.Fatalf("stored authorization = %T, want *LayerXScopeAuthorization", stored)
		}
		if !proto.Equal(got, &auth) {
			t.Fatalf("stored authorization = %v, want %v", got, &auth)
		}
		if !storedExpiration.Equal(expiration) {
			t.Fatalf("expiration = %v, want %v", storedExpiration, expiration)
		}

		expired := ctx.WithBlockHeader(tmproto.Header{Time: expiration.Add(time.Second)})
		if stored, _ := keeper.GetCleanAuthorization(expired, grantee, granter, msgType); stored != nil {
			t.Fatalf("authorization after expiration = %v, want nil", stored)
		}
		if remaining := keeper.GetAuthorizations(expired, grantee, granter); len(remaining) != 0 {
			t.Fatalf("grants after expiration = %d, want 0", len(remaining))
		}
	})
}

func TestLayerXScopeValidateBasicModuleMask(t *testing.T) {
	for bit := uint(0); bit < 64; bit++ {
		t.Run(fmt.Sprintf("bit_%d", bit), func(t *testing.T) {
			auth := layerXScopeTestAuthorization()
			auth.ModuleMask = uint64(1) << bit
			err := auth.ValidateBasic()
			if bit >= 1 && bit <= 9 {
				if err != nil {
					t.Fatalf("supported module mask %#x: %v", auth.ModuleMask, err)
				}
			} else if !errors.Is(err, ErrAuthScope) {
				t.Fatalf("unsupported module mask %#x: got %v, want %v", auth.ModuleMask, err, ErrAuthScope)
			}
		})
	}
	for _, mask := range []uint64{0, 0x3ff, 0x7fe, ^uint64(0)} {
		t.Run(fmt.Sprintf("invalid_mask_%x", mask), func(t *testing.T) {
			auth := layerXScopeTestAuthorization()
			auth.ModuleMask = mask
			if err := auth.ValidateBasic(); !errors.Is(err, ErrAuthScope) {
				t.Fatalf("module mask %#x: got %v, want %v", mask, err, ErrAuthScope)
			}
		})
	}

	auth := layerXScopeTestAuthorization()
	auth.ModuleMask = layerXScopeTestEveryModuleMask
	if err := auth.ValidateBasic(); err != nil {
		t.Fatalf("all supported modules: %v", err)
	}
}

func TestLayerXScopeValidateBasicKindAndPeriod(t *testing.T) {
	for _, kind := range []uint32{0, 1, 2, 3, 4, 5, 6, 7, 8, ^uint32(0)} {
		t.Run(fmt.Sprintf("kind_%d", kind), func(t *testing.T) {
			auth := layerXScopeTestAuthorization()
			auth.Kind = kind
			err := auth.ValidateBasic()
			if kind >= 2 && kind <= 4 {
				if err != nil {
					t.Fatalf("supported kind %d: %v", kind, err)
				}
			} else if !errors.Is(err, ErrAuthScope) {
				t.Fatalf("kind %d: got %v, want %v", kind, err, ErrAuthScope)
			}
		})
	}

	cases := []struct {
		name   string
		mutate func(*LayerXScopeAuthorization)
		want   error
	}{
		{"period_cap_below_total", func(a *LayerXScopeAuthorization) { a.MaxPerPeriod = "1999" }, nil},
		{"period_cap_equals_total", func(a *LayerXScopeAuthorization) { a.MaxPerPeriod = "2000" }, nil},
		{"period_cap_above_total", func(a *LayerXScopeAuthorization) { a.MaxPerPeriod = "2001" }, ErrAuthScope},
		{"period_cap_zero", func(a *LayerXScopeAuthorization) { a.MaxPerPeriod = "0" }, nil},
		{"period_cap_empty", func(a *LayerXScopeAuthorization) { a.MaxPerPeriod = "" }, nil},
		{"period_disabled", func(a *LayerXScopeAuthorization) { a.PeriodLength = 0; a.MaxPerPeriod = "2001" }, nil},
		{"period_length_max", func(a *LayerXScopeAuthorization) { a.PeriodLength = ^uint64(0) }, nil},
		{"period_cap_above_zero_total", func(a *LayerXScopeAuthorization) { a.MaxTotal = "0" }, ErrAuthScope},
		{"period_cap_above_empty_total", func(a *LayerXScopeAuthorization) { a.MaxTotal = "" }, ErrAuthScope},
		{"period_cap_u128_max", func(a *LayerXScopeAuthorization) {
			a.MaxTotal = "340282366920938463463374607431768211455"
			a.MaxPerPeriod = a.MaxTotal
		}, nil},
		{"period_cap_large_above_total", func(a *LayerXScopeAuthorization) {
			a.MaxTotal = "340282366920938463463374607431768211454"
			a.MaxPerPeriod = "340282366920938463463374607431768211455"
		}, ErrAuthScope},
		{"period_cap_above_u128", func(a *LayerXScopeAuthorization) { a.MaxPerPeriod = "340282366920938463463374607431768211456" }, ErrNonCanonical},
		{"period_cap_leading_zero", func(a *LayerXScopeAuthorization) { a.MaxPerPeriod = "01" }, ErrNonCanonical},
		{"period_cap_negative", func(a *LayerXScopeAuthorization) { a.MaxPerPeriod = "-1" }, ErrNonCanonical},
		{"period_cap_plus_sign", func(a *LayerXScopeAuthorization) { a.MaxPerPeriod = "+1" }, ErrNonCanonical},
		{"period_cap_not_decimal", func(a *LayerXScopeAuthorization) { a.MaxPerPeriod = "1.5" }, ErrNonCanonical},
		{"purpose_empty", func(a *LayerXScopeAuthorization) { a.PurposeHash = []byte{} }, nil},
		{"purpose_32_bytes", func(a *LayerXScopeAuthorization) { a.PurposeHash = bytes.Repeat([]byte{0x4a}, 32) }, nil},
		{"purpose_1_byte", func(a *LayerXScopeAuthorization) { a.PurposeHash = []byte{1} }, ErrNonCanonical},
		{"purpose_31_bytes", func(a *LayerXScopeAuthorization) { a.PurposeHash = make([]byte, 31) }, ErrNonCanonical},
		{"purpose_33_bytes", func(a *LayerXScopeAuthorization) { a.PurposeHash = make([]byte, 33) }, ErrNonCanonical},
		{"validity_equal", func(a *LayerXScopeAuthorization) { a.NotAfter = a.NotBefore }, ErrAuthScope},
		{"validity_reversed", func(a *LayerXScopeAuthorization) { a.NotAfter = a.NotBefore - 1 }, ErrAuthScope},
		{"validity_missing", func(a *LayerXScopeAuthorization) { a.NotBefore = 0; a.NotAfter = 0 }, ErrAuthScope},
		{"validity_zero_start", func(a *LayerXScopeAuthorization) { a.NotBefore = 0; a.NotAfter = 1 }, nil},
		{"validity_max_end", func(a *LayerXScopeAuthorization) { a.NotBefore = ^uint64(0) - 1; a.NotAfter = ^uint64(0) }, nil},
		{"validity_max_start", func(a *LayerXScopeAuthorization) { a.NotBefore = ^uint64(0); a.NotAfter = ^uint64(0) }, ErrAuthScope},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			auth := layerXScopeTestAuthorization()
			auth.PeriodLength = 60
			auth.MaxTotal = "2000"
			auth.MaxPerPeriod = "1500"
			tc.mutate(&auth)
			if err := auth.ValidateBasic(); !errors.Is(err, tc.want) {
				t.Fatalf("ValidateBasic = %v, want %v", err, tc.want)
			}
		})
	}

	t.Run("protobuf_round_trip", func(t *testing.T) {
		auth := layerXScopeTestAuthorization()
		auth.Kind = 4
		auth.PeriodLength = 60
		auth.MaxPerPeriod = "1500"
		auth.PurposeHash = bytes.Repeat([]byte{0x4a}, 32)
		auth.SpentTotal = "100"
		encoded, err := proto.Marshal(&auth)
		if err != nil {
			t.Fatal(err)
		}
		var decoded LayerXScopeAuthorization
		if err := proto.Unmarshal(encoded, &decoded); err != nil {
			t.Fatal(err)
		}
		if !proto.Equal(&auth, &decoded) {
			t.Fatalf("decoded authorization = %v, want %v", &decoded, &auth)
		}
		if err := decoded.ValidateBasic(); err != nil {
			t.Fatalf("decoded authorization: %v", err)
		}
	})
}

func TestLayerXScopeRevocationEpoch(t *testing.T) {
	ctx := testutil.DefaultContext(sdk.NewKVStoreKey("layerx_scope_revocation"), sdk.NewTransientStoreKey("transient_layerx_scope_revocation"))
	grantor := layerXScopeTestAddress(0xd1)
	auth := layerXScopeTestAuthorization()
	msg := layerXScopeTestSend(grantor)

	resp, err := auth.Accept(WithGrantorRevocation(ctx, grantor, 4), msg)
	if err != nil {
		t.Fatalf("Accept at revocation 4: %v", err)
	}
	if !resp.Accept || resp.Delete || resp.Updated != nil {
		t.Fatalf("Accept at revocation 4 = %+v, want accept without update", resp)
	}

	cases := []struct {
		name string
		ctx  sdk.Context
	}{
		{"grantor_revoked_to_5", WithGrantorRevocation(ctx, grantor, 5)},
		{"grantor_revocation_behind", WithGrantorRevocation(ctx, grantor, 3)},
		{"revocation_of_other_grantor", WithGrantorRevocation(ctx, layerXScopeTestAddress(0xd2), 4)},
		{"revocation_not_supplied", ctx},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := auth.Accept(tc.ctx, msg)
			if !errors.Is(err, ErrAuthRevoked) {
				t.Fatalf("Accept = %v, want %v", err, ErrAuthRevoked)
			}
			if resp.Accept || resp.Delete || resp.Updated != nil {
				t.Fatalf("refused Accept response = %+v, want zero", resp)
			}
		})
	}

	if _, err := auth.Accept(WithGrantorRevocation(ctx, grantor, 4), nil); !errors.Is(err, ErrAuthScope) {
		t.Fatalf("Accept nil message = %v, want %v", err, ErrAuthScope)
	}
	multiSend := layerXScopeTestMultiSend(grantor)
	resp, err = auth.Accept(WithGrantorRevocation(ctx, grantor, 4), multiSend)
	if !errors.Is(err, ErrAuthScope) {
		t.Fatalf("Accept %s under a MsgSend grant = %v, want %v", sdk.MsgTypeURL(multiSend), err, ErrAuthScope)
	}
	if resp.Accept || resp.Delete || resp.Updated != nil {
		t.Fatalf("refused Accept response = %+v, want zero", resp)
	}
}

func TestLayerXScopeRefusesLedger(t *testing.T) {
	ledgerMessages := []proto.Message{
		&ledgertypes.MsgUpdateParams{},
		&ledgertypes.MsgUpdateParamsResponse{},
		&ledgertypes.Params{},
	}
	for _, msg := range ledgerMessages {
		t.Run(proto.MessageName(msg), func(t *testing.T) {
			if err := refuseLayerXLedger(msg); !errors.Is(err, ErrUnauthorizedDebit) {
				t.Fatalf("refuseLayerXLedger = %v, want %v", err, ErrUnauthorizedDebit)
			}
		})
	}

	for _, msg := range []proto.Message{&MsgUpdateParams{}, &Params{}, &banktypes.MsgSend{}} {
		if err := refuseLayerXLedger(msg); err != nil {
			t.Fatalf("refuseLayerXLedger(%s) = %v, want nil", proto.MessageName(msg), err)
		}
	}

	auth := layerXScopeTestAuthorization()
	auth.ModuleMask = layerXScopeTestEveryModuleMask
	auth.OrdinalMin = 0
	auth.OrdinalMax = 0xffff
	if err := auth.ValidateBasic(); err != nil {
		t.Fatalf("every-module scope: %v", err)
	}
	ctx := testutil.DefaultContext(sdk.NewKVStoreKey("layerx_scope_ledger"), sdk.NewTransientStoreKey("transient_layerx_scope_ledger"))
	grantor := layerXScopeTestAddress(0xd3)
	resp, err := auth.Accept(WithGrantorRevocation(ctx, grantor, auth.RevocationEpoch), layerXScopeTestSend(grantor))
	if err != nil || !resp.Accept {
		t.Fatalf("every-module scope on a non-ledger message = %+v, %v; want accept", resp, err)
	}
}

type layerXScopeTestActivity struct {
	*banktypes.MsgSend
	moduleID uint16
	ordinal  uint16
	assetID  [32]byte
}

func (m *layerXScopeTestActivity) XXX_MessageName() string {
	return proto.MessageName(m.MsgSend)
}

func (m *layerXScopeTestActivity) LayerXScope() (uint16, uint16, [32]byte, sdk.Int) {
	return m.moduleID, m.ordinal, m.assetID, m.Amount[0].Amount
}

func layerXScopeTestScopedSend(grantor sdk.AccAddress, amount sdk.Int) *layerXScopeTestActivity {
	msg := &layerXScopeTestActivity{
		MsgSend:  banktypes.NewMsgSend(grantor, layerXScopeTestAddress(0x0e), sdk.Coins{{Denom: "upax", Amount: amount}}),
		moduleID: 1,
		ordinal:  5,
	}
	copy(msg.assetID[:], layerXScopeTestAuthorization().AssetId)
	return msg
}

func TestLayerXScopeAcceptScopedActivity(t *testing.T) {
	grantor := layerXScopeTestAddress(0xd5)
	auth := layerXScopeTestAuthorization()
	auth.ModuleMask = 1 << 1
	auth.OrdinalMin = 5
	auth.OrdinalMax = 6
	auth.MaxPerActivity = "10"
	auth.MaxTotal = "15"
	ctx := testutil.DefaultContext(sdk.NewKVStoreKey("layerx_scope_activity"), sdk.NewTransientStoreKey("transient_layerx_scope_activity"))
	ctx = WithGrantorRevocation(ctx, grantor, auth.RevocationEpoch)

	cases := []struct {
		name   string
		mutate func(*layerXScopeTestActivity)
		want   error
	}{
		{"inclusive_min_and_cap", func(m *layerXScopeTestActivity) {}, nil},
		{"inclusive_max", func(m *layerXScopeTestActivity) { m.ordinal = 6 }, nil},
		{"module_outside_mask", func(m *layerXScopeTestActivity) { m.moduleID = 2 }, ErrAuthScope},
		{"module_zero", func(m *layerXScopeTestActivity) { m.moduleID = 0 }, ErrAuthScope},
		{"module_64", func(m *layerXScopeTestActivity) { m.moduleID = 64 }, ErrAuthScope},
		{"module_max_u16", func(m *layerXScopeTestActivity) { m.moduleID = 0xffff }, ErrAuthScope},
		{"ordinal_below_min", func(m *layerXScopeTestActivity) { m.ordinal = 4 }, ErrAuthScope},
		{"ordinal_above_max", func(m *layerXScopeTestActivity) { m.ordinal = 7 }, ErrAuthScope},
		{"other_asset", func(m *layerXScopeTestActivity) { m.assetID[31] ^= 1 }, ErrAuthScope},
		{"amount_above_cap", func(m *layerXScopeTestActivity) { m.Amount[0].Amount = sdk.NewInt(11) }, ErrAuthScope},
		{"negative_amount", func(m *layerXScopeTestActivity) { m.Amount[0].Amount = sdk.NewInt(-1) }, ErrAuthScope},
		{"nil_amount", func(m *layerXScopeTestActivity) { m.Amount[0].Amount = sdk.Int{} }, ErrAuthScope},
		{"amount_above_u128", func(m *layerXScopeTestActivity) {
			m.Amount[0].Amount = sdk.NewIntFromUint64(1).Mul(sdk.NewIntFromUint64(1 << 63)).Mul(sdk.NewIntFromUint64(1 << 63)).MulRaw(4)
		}, ErrAuthScope},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := layerXScopeTestScopedSend(grantor, sdk.NewInt(10))
			tc.mutate(msg)
			if err := auth.ValidateBasic(); err != nil {
				t.Fatalf("ValidateBasic: %v", err)
			}
			before := proto.Clone(&auth)
			resp, err := auth.Accept(ctx, msg)
			if !errors.Is(err, tc.want) {
				t.Fatalf("Accept = %+v, %v; want %v", resp, err, tc.want)
			}
			if !proto.Equal(&auth, before) {
				t.Fatal("Accept mutated its original authorization")
			}
			if tc.want != nil {
				if resp.Accept || resp.Delete || resp.Updated != nil {
					t.Fatalf("refused response = %+v, want zero", resp)
				}
				return
			}
			updated, ok := resp.Updated.(*LayerXScopeAuthorization)
			if !resp.Accept || resp.Delete || !ok || updated.SpentTotal != "10" {
				t.Fatalf("accepted response = %+v, want updated spent total 10", resp)
			}
		})
	}

	t.Run("revoked_scoped_activity", func(t *testing.T) {
		resp, err := auth.Accept(WithGrantorRevocation(ctx, grantor, auth.RevocationEpoch+1), layerXScopeTestScopedSend(grantor, sdk.NewInt(10)))
		if !errors.Is(err, ErrAuthRevoked) || resp.Accept || resp.Delete || resp.Updated != nil {
			t.Fatalf("revoked scoped activity = %+v, %v; want %v and zero response", resp, err, ErrAuthRevoked)
		}
	})
}

func TestLayerXScopeAcceptSpendsTotal(t *testing.T) {
	grantor := layerXScopeTestAddress(0xd6)
	auth := layerXScopeTestAuthorization()
	auth.ModuleMask = 1 << 1
	auth.OrdinalMin = 5
	auth.OrdinalMax = 6
	auth.MaxPerActivity = "10"
	auth.MaxTotal = "15"
	ctx := testutil.DefaultContext(sdk.NewKVStoreKey("layerx_scope_spending"), sdk.NewTransientStoreKey("transient_layerx_scope_spending"))
	ctx = WithGrantorRevocation(ctx, grantor, auth.RevocationEpoch)

	resp, err := auth.Accept(ctx, layerXScopeTestScopedSend(grantor, sdk.NewInt(10)))
	if err != nil || !resp.Accept || resp.Delete {
		t.Fatalf("first activity = %+v, %v; want accept", resp, err)
	}
	updated, ok := resp.Updated.(*LayerXScopeAuthorization)
	if !ok || updated.SpentTotal != "10" || auth.SpentTotal != "" {
		t.Fatalf("first update = %v, original spent total = %q; want 10 and empty", resp.Updated, auth.SpentTotal)
	}
	want := auth
	want.SpentTotal = "10"
	if !proto.Equal(updated, &want) {
		t.Fatalf("first update = %v, want %v", updated, &want)
	}
	encoded, err := proto.Marshal(updated)
	if err != nil {
		t.Fatal(err)
	}
	var stored LayerXScopeAuthorization
	if err := proto.Unmarshal(encoded, &stored); err != nil {
		t.Fatal(err)
	}
	resp, err = stored.Accept(ctx, layerXScopeTestScopedSend(grantor, sdk.NewInt(6)))
	if !errors.Is(err, ErrAuthScope) || resp.Accept || resp.Delete || resp.Updated != nil || stored.SpentTotal != "10" {
		t.Fatalf("total 10 + 6 with cap 15 = %+v, %v, spent %s; want refusal without mutation", resp, err, stored.SpentTotal)
	}
	resp, err = stored.Accept(ctx, layerXScopeTestScopedSend(grantor, sdk.NewInt(5)))
	if err != nil || !resp.Accept || resp.Delete {
		t.Fatalf("total 10 + 5 with cap 15 = %+v, %v; want accept", resp, err)
	}
	exhausted, ok := resp.Updated.(*LayerXScopeAuthorization)
	if !ok || exhausted.SpentTotal != "15" || stored.SpentTotal != "10" {
		t.Fatalf("exact cap update = %v, prior spent %s; want 15 and 10", resp.Updated, stored.SpentTotal)
	}
	resp, err = exhausted.Accept(ctx, layerXScopeTestScopedSend(grantor, sdk.NewInt(1)))
	if !errors.Is(err, ErrAuthScope) || resp.Accept || resp.Delete || resp.Updated != nil || exhausted.SpentTotal != "15" {
		t.Fatalf("exhausted grant = %+v, %v; want refusal without mutation", resp, err)
	}

	cases := []struct {
		name   string
		mutate func(*LayerXScopeAuthorization)
		amount int64
		want   error
		spent  string
	}{
		{"zero_amount", func(a *LayerXScopeAuthorization) {}, 0, nil, "0"},
		{"explicit_zero_spent", func(a *LayerXScopeAuthorization) { a.SpentTotal = "0" }, 10, nil, "10"},
		{"spent_leading_zero", func(a *LayerXScopeAuthorization) { a.SpentTotal = "01" }, 1, ErrNonCanonical, ""},
		{"spent_negative", func(a *LayerXScopeAuthorization) { a.SpentTotal = "-1" }, 1, ErrNonCanonical, ""},
		{"spent_not_decimal", func(a *LayerXScopeAuthorization) { a.SpentTotal = "1.5" }, 1, ErrNonCanonical, ""},
		{"spent_above_u128", func(a *LayerXScopeAuthorization) { a.SpentTotal = "340282366920938463463374607431768211456" }, 1, ErrNonCanonical, ""},
		{"invalid_per_activity", func(a *LayerXScopeAuthorization) { a.MaxPerActivity = "01" }, 1, ErrNonCanonical, ""},
		{"invalid_total", func(a *LayerXScopeAuthorization) { a.MaxTotal = "+15" }, 1, ErrNonCanonical, ""},
		{"u128_sum_overflow", func(a *LayerXScopeAuthorization) {
			a.MaxTotal = "340282366920938463463374607431768211455"
			a.SpentTotal = a.MaxTotal
		}, 1, ErrAuthScope, ""},
		{"u128_exact_cap", func(a *LayerXScopeAuthorization) {
			a.MaxTotal = "340282366920938463463374607431768211455"
			a.SpentTotal = "340282366920938463463374607431768211454"
		}, 1, nil, "340282366920938463463374607431768211455"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			candidate := auth
			tc.mutate(&candidate)
			before := proto.Clone(&candidate)
			resp, err := candidate.Accept(ctx, layerXScopeTestScopedSend(grantor, sdk.NewInt(tc.amount)))
			if !errors.Is(err, tc.want) || !proto.Equal(&candidate, before) {
				t.Fatalf("Accept = %+v, %v; want %v without mutation", resp, err, tc.want)
			}
			if tc.want != nil {
				if resp.Accept || resp.Delete || resp.Updated != nil {
					t.Fatalf("refused response = %+v, want zero", resp)
				}
				return
			}
			updated, ok := resp.Updated.(*LayerXScopeAuthorization)
			if !resp.Accept || resp.Delete || !ok || updated.SpentTotal != tc.spent {
				t.Fatalf("Accept = %+v; want updated spent total %s", resp, tc.spent)
			}
		})
	}
}

func TestLayerXScopeGrantAndExec(t *testing.T) {
	registry := cdctypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	banktypes.RegisterInterfaces(registry)
	authz.RegisterInterfaces(registry)
	RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)

	authzKey := sdk.NewKVStoreKey(authzkeeper.StoreKey)
	accountKey := sdk.NewKVStoreKey(authtypes.StoreKey)
	bankKey := sdk.NewKVStoreKey(banktypes.StoreKey)
	paramsKey := sdk.NewKVStoreKey("params")
	paramsTransientKey := sdk.NewTransientStoreKey("transient_params")
	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db)
	for _, key := range []sdk.StoreKey{authzKey, accountKey, bankKey, paramsKey} {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	cms.MountStoreWithDB(paramsTransientKey, sdk.StoreTypeTransient, db)
	if err := cms.LoadLatestVersion(); err != nil {
		t.Fatal(err)
	}
	expiration := time.Unix(1_900_000_000, 0).UTC()
	ctx := sdk.NewContext(cms, tmproto.Header{Time: expiration.Add(-time.Hour)}, false)

	params := paramskeeper.NewKeeper(cdc, codec.NewLegacyAmino(), paramsKey, paramsTransientKey)
	const fundingModule = "layerx_scope_funding"
	accountKeeper := authkeeper.NewAccountKeeper(cdc, accountKey, params.Subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount,
		map[string][]string{fundingModule: {authtypes.Minter}})
	accountKeeper.SetParams(ctx, authtypes.DefaultParams())
	bankKeeper := bankkeeper.NewBaseKeeper(cdc, bankKey, accountKeeper, params.Subspace(banktypes.ModuleName), nil)
	bankKeeper.SetParams(ctx, banktypes.DefaultParams())

	router := baseapp.NewMsgServiceRouter()
	router.SetInterfaceRegistry(registry)
	banktypes.RegisterMsgServer(router, bankkeeper.NewMsgServerImpl(bankKeeper))
	keeper := authzkeeper.NewKeeper(authzKey, cdc, router)
	goCtx := sdk.WrapSDKContext(ctx)

	granter, grantee := layerXScopeTestAddress(0xd4), layerXScopeTestAddress(0xe4)
	recipient := layerXScopeTestAddress(0x0e)
	funds := sdk.NewCoins(sdk.NewInt64Coin("upax", 100))
	if err := bankKeeper.MintCoins(ctx, fundingModule, funds); err != nil {
		t.Fatalf("MintCoins: %v", err)
	}
	if err := bankKeeper.SendCoinsFromModuleToAccount(ctx, fundingModule, granter, funds); err != nil {
		t.Fatalf("SendCoinsFromModuleToAccount: %v", err)
	}

	auth := layerXScopeTestAuthorization()
	grantMsg, err := authz.NewMsgGrant(granter, grantee, &auth, expiration)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := keeper.Grant(goCtx, grantMsg); err != nil {
		t.Fatalf("Grant naming %s: %v", layerXScopeTestSendTypeURL, err)
	}
	stored, storedExpiration := keeper.GetCleanAuthorization(ctx, grantee, granter, layerXScopeTestSendTypeURL)
	if got, ok := stored.(*LayerXScopeAuthorization); !ok || !proto.Equal(got, &auth) {
		t.Fatalf("stored authorization = %v, want %v", stored, &auth)
	}
	if !storedExpiration.Equal(expiration) {
		t.Fatalf("expiration = %v, want %v", storedExpiration, expiration)
	}

	own := layerXScopeTestAuthorization()
	own.MsgTypeUrl = layerXScopeTestOwnTypeURL
	ownGrant, err := authz.NewMsgGrant(granter, grantee, &own, expiration)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := keeper.Grant(goCtx, ownGrant); !errors.Is(err, sdkerrors.ErrInvalidType) {
		t.Fatalf("Grant naming %s = %v, want %v", layerXScopeTestOwnTypeURL, err, sdkerrors.ErrInvalidType)
	}
	if stored, _ := keeper.GetCleanAuthorization(ctx, grantee, granter, layerXScopeTestOwnTypeURL); stored != nil {
		t.Fatalf("authorization under %s = %v, want nil", layerXScopeTestOwnTypeURL, stored)
	}

	execCtx := sdk.WrapSDKContext(WithGrantorRevocation(ctx, granter, auth.RevocationEpoch))
	send := authz.NewMsgExec(grantee, []sdk.Msg{layerXScopeTestSend(granter)})
	resp, err := keeper.Exec(execCtx, &send)
	if err != nil {
		t.Fatalf("Exec MsgSend under the grant: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("Exec results = %d, want 1", len(resp.Results))
	}
	if got := bankKeeper.GetBalance(ctx, granter, "upax"); !got.IsEqual(sdk.NewInt64Coin("upax", 99)) {
		t.Fatalf("granter balance = %v, want 99upax", got)
	}
	if got := bankKeeper.GetBalance(ctx, recipient, "upax"); !got.IsEqual(sdk.NewInt64Coin("upax", 1)) {
		t.Fatalf("recipient balance = %v, want 1upax", got)
	}

	multiSend := authz.NewMsgExec(grantee, []sdk.Msg{layerXScopeTestMultiSend(granter)})
	if _, err := keeper.Exec(execCtx, &multiSend); !errors.Is(err, sdkerrors.ErrUnauthorized) || !strings.Contains(err.Error(), "authorization not found") {
		t.Fatalf("Exec MsgMultiSend under a MsgSend grant = %v, want %v: authorization not found", err, sdkerrors.ErrUnauthorized)
	}
	if got := bankKeeper.GetBalance(ctx, granter, "upax"); !got.IsEqual(sdk.NewInt64Coin("upax", 99)) {
		t.Fatalf("granter balance after refused exec = %v, want 99upax", got)
	}
}
