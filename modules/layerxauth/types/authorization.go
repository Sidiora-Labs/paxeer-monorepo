package types

import (
	"bytes"
	"math/big"
	"strings"

	"github.com/gogo/protobuf/proto"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz"
)

const (
	// layerXLedgerProtoPackage prefixes the full name of every layerxledger message.
	layerXLedgerProtoPackage = "paxprotocol.paxchain.layerxledger."

	// scopeOrdinalMax bounds the uint16 activity ordinals of lxp_authority_scope.
	scopeOrdinalMax = 0xffff

	scopeIDLength = 32
	scopeU128Bits = 128
)

var _ authz.Authorization = &LayerXScopeAuthorization{}

type ScopedActivity interface {
	LayerXScope() (moduleID uint16, ordinal uint16, assetID [32]byte, amount sdk.Int)
}

type AcceptResponse = authz.AcceptResponse

type grantorRevocationKey struct{}

type grantorRevocation struct {
	grantor  sdk.AccAddress
	sequence uint64
}

// WithGrantorRevocation records the grantor's current identity REVOCATION
// sequence on the context that Accept reads it from.
func WithGrantorRevocation(ctx sdk.Context, grantor sdk.AccAddress, sequence uint64) sdk.Context {
	return ctx.WithValue(grantorRevocationKey{}, grantorRevocation{grantor: grantor, sequence: sequence})
}

// MsgTypeURL implements Authorization.MsgTypeURL. A LayerX-scope grant from
// one grantor to one grantee authorizes exactly the message type it names.
func (a LayerXScopeAuthorization) MsgTypeURL() string {
	return a.MsgTypeUrl
}

// Accept implements Authorization.Accept. It refuses every layerxledger
// message, since the kernel ledger requires the account-holder authority kind,
// a message of any type other than the one the grant names, and a grant whose
// revocation epoch differs from the grantor's current REVOCATION sequence, as
// lxp_authority_is_live does.
func (a LayerXScopeAuthorization) Accept(ctx sdk.Context, msg sdk.Msg) (AcceptResponse, error) {
	if msg == nil {
		return authz.AcceptResponse{}, ErrAuthScope.Wrap("nil message")
	}
	if err := refuseLayerXLedger(msg); err != nil {
		return authz.AcceptResponse{}, err
	}
	if msgType := sdk.MsgTypeURL(msg); msgType != a.MsgTypeUrl {
		return authz.AcceptResponse{}, ErrAuthScope.Wrapf("message type %s, grant authorizes %s", msgType, a.MsgTypeUrl)
	}
	signers := msg.GetSigners()
	if len(signers) != 1 {
		return authz.AcceptResponse{}, ErrAuthScope.Wrapf("message has %d signers, want 1", len(signers))
	}
	current, ok := ctx.Value(grantorRevocationKey{}).(grantorRevocation)
	if !ok || !current.grantor.Equals(signers[0]) {
		return authz.AcceptResponse{}, ErrAuthRevoked.Wrap("grantor revocation sequence not established")
	}
	if current.sequence != a.RevocationEpoch {
		return authz.AcceptResponse{}, ErrAuthRevoked.Wrapf("revocation epoch %d, grantor revocation %d", a.RevocationEpoch, current.sequence)
	}
	activity, ok := msg.(ScopedActivity)
	if !ok {
		return authz.AcceptResponse{Accept: true}, nil
	}
	moduleID, ordinal, assetID, amount := activity.LayerXScope()
	if moduleID >= 64 || a.ModuleMask&(uint64(1)<<moduleID) == 0 {
		return authz.AcceptResponse{}, ErrAuthScope.Wrapf("module %d outside module mask %#x", moduleID, a.ModuleMask)
	}
	if uint32(ordinal) < a.OrdinalMin || uint32(ordinal) > a.OrdinalMax {
		return authz.AcceptResponse{}, ErrAuthScope.Wrapf("ordinal %d outside %d..%d", ordinal, a.OrdinalMin, a.OrdinalMax)
	}
	if !bytes.Equal(a.AssetId, assetID[:]) {
		return authz.AcceptResponse{}, ErrAuthScope.Wrap("activity asset differs from grant asset")
	}
	if amount.IsNil() || amount.IsNegative() || amount.BigInt().BitLen() > scopeU128Bits {
		return authz.AcceptResponse{}, ErrAuthScope.Wrap("activity amount is not a u128")
	}
	perActivity, err := parseScopeU128(a.MaxPerActivity)
	if err != nil {
		return authz.AcceptResponse{}, err
	}
	if amount.BigInt().Cmp(perActivity) > 0 {
		return authz.AcceptResponse{}, ErrAuthScope.Wrapf("activity amount %s above max per activity %s", amount, perActivity)
	}
	spent, err := parseScopeU128(a.SpentTotal)
	if err != nil {
		return authz.AcceptResponse{}, err
	}
	total, err := parseScopeU128(a.MaxTotal)
	if err != nil {
		return authz.AcceptResponse{}, err
	}
	spent.Add(spent, amount.BigInt())
	if spent.BitLen() > scopeU128Bits || spent.Cmp(total) > 0 {
		return authz.AcceptResponse{}, ErrAuthScope.Wrapf("spent total %s above max total %s", spent, total)
	}
	updated := a
	updated.SpentTotal = spent.String()
	return authz.AcceptResponse{Accept: true, Updated: &updated}, nil
}

// ValidateBasic implements Authorization.ValidateBasic with the stateless
// bounds of lxp_authority_scope that grant_scope_validate enforces.
func (a LayerXScopeAuthorization) ValidateBasic() error {
	if a.MsgTypeUrl == "" {
		return ErrAuthScope.Wrap("empty message type url")
	}
	if err := refuseLayerXLedgerName(strings.TrimPrefix(a.MsgTypeUrl, "/")); err != nil {
		return err
	}
	if a.ModuleMask == 0 {
		return ErrAuthScope.Wrap("zero module mask")
	}
	if a.ModuleMask&^uint64(0x3fe) != 0 {
		return ErrAuthScope.Wrapf("module mask %#x names unsupported modules", a.ModuleMask)
	}
	if a.Kind != 2 && a.Kind != 3 && a.Kind != 4 {
		return ErrAuthScope.Wrapf("unsupported authority kind %d", a.Kind)
	}
	if a.OrdinalMin > a.OrdinalMax {
		return ErrAuthScope.Wrapf("ordinal min %d above ordinal max %d", a.OrdinalMin, a.OrdinalMax)
	}
	if a.OrdinalMax > scopeOrdinalMax {
		return ErrAuthScope.Wrapf("ordinal max %d above %d", a.OrdinalMax, scopeOrdinalMax)
	}
	if len(a.AssetId) != 0 && len(a.AssetId) != scopeIDLength {
		return ErrNonCanonical.Wrapf("asset id length %d", len(a.AssetId))
	}
	if len(a.GrantId) != scopeIDLength {
		return ErrNonCanonical.Wrapf("grant id length %d", len(a.GrantId))
	}
	if len(a.PurposeHash) != 0 && len(a.PurposeHash) != scopeIDLength {
		return ErrNonCanonical.Wrapf("purpose hash length %d", len(a.PurposeHash))
	}
	if a.NotBefore >= a.NotAfter {
		return ErrAuthScope.Wrapf("not before %d is not before not after %d", a.NotBefore, a.NotAfter)
	}
	perActivity, err := parseScopeU128(a.MaxPerActivity)
	if err != nil {
		return err
	}
	total, err := parseScopeU128(a.MaxTotal)
	if err != nil {
		return err
	}
	if total.Sign() != 0 && perActivity.Cmp(total) > 0 {
		return ErrAuthScope.Wrapf("max per activity %s above max total %s", perActivity, total)
	}
	perPeriod, err := parseScopeU128(a.MaxPerPeriod)
	if err != nil {
		return err
	}
	if a.PeriodLength != 0 && perPeriod.Cmp(total) > 0 {
		return ErrAuthScope.Wrapf("max per period %s above max total %s", perPeriod, total)
	}
	return nil
}

// refuseLayerXLedger refuses every layerxledger message with
// LXP_UNAUTHORIZED_DEBIT (src/modules/asset/lx_asset_registry.c:147).
func refuseLayerXLedger(msg proto.Message) error {
	return refuseLayerXLedgerName(proto.MessageName(msg))
}

// refuseLayerXLedgerName refuses a full message name in the layerxledger
// package.
func refuseLayerXLedgerName(name string) error {
	if strings.HasPrefix(name, layerXLedgerProtoPackage) {
		return ErrUnauthorizedDebit.Wrapf("%s requires the account-holder authority", name)
	}
	return nil
}

// parseScopeU128 decodes a canonical decimal u128; the empty string is zero.
func parseScopeU128(value string) (*big.Int, error) {
	if value == "" {
		return new(big.Int), nil
	}
	if len(value) > 1 && value[0] == '0' {
		return nil, ErrNonCanonical.Wrapf("amount %q has a leading zero", value)
	}
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return nil, ErrNonCanonical.Wrapf("amount %q is not decimal", value)
		}
	}
	out, ok := new(big.Int).SetString(value, 10)
	if !ok || out.BitLen() > scopeU128Bits {
		return nil, ErrNonCanonical.Wrapf("amount %q is not a u128", value)
	}
	return out, nil
}
