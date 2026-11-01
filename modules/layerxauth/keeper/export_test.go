package keeper

import (
	"crypto/ed25519"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

type harness struct {
	ctx       sdk.Context
	keeper    *Keeper
	host      *lxphost.Host
	key       *sdk.KVStoreKey
	hostKey   *sdk.KVStoreKey
	authority string
}

type Harness = harness
type AuthViewFixture = authViewFixture

var (
	HarnessFactory       func(*testing.T) *Harness
	ExecuteActor         func(*testing.T, *AuthViewFixture) Actor
	ExecutePayload       func(*testing.T, *AuthViewFixture, string) ([]byte, [32]byte)
	ExecuteSignedReceipt func(*testing.T, lxphost.Receipt) []byte
	NewAuthViewFixture   = newAuthViewFixture
	AuthViewStores       = authViewStores
)

const CapabilityAuthorizationTypeURL = capabilityAuthorizationTypeURL

func newHarness(t *testing.T) *harness                        { t.Helper(); return HarnessFactory(t) }
func authExecuteActor(t *testing.T, f *authViewFixture) Actor { t.Helper(); return ExecuteActor(t, f) }
func authExecutePayload(t *testing.T, f *authViewFixture, method string) ([]byte, [32]byte) {
	t.Helper()
	return ExecutePayload(t, f, method)
}
func authExecuteSignedReceipt(t *testing.T, receipt lxphost.Receipt) []byte {
	t.Helper()
	return ExecuteSignedReceipt(t, receipt)
}

func NewHarness(ctx sdk.Context, k *Keeper, host *lxphost.Host, key, hostKey *sdk.KVStoreKey, authority string) *Harness {
	return &harness{ctx: ctx, keeper: k, host: host, key: key, hostKey: hostKey, authority: authority}
}
func (h *harness) Context() *sdk.Context     { return &h.ctx }
func (h *harness) Keeper() *Keeper           { return h.keeper }
func (h *harness) Host() *lxphost.Host       { return h.host }
func (h *harness) StoreKey() *sdk.KVStoreKey { return h.key }
func (h *harness) HostKey() *sdk.KVStoreKey  { return h.hostKey }
func (h *harness) Authority() string         { return h.authority }

func (k *Keeper) TestCodec() codec.BinaryCodec        { return k.cdc }
func (k *Keeper) TestStoreKey() sdk.StoreKey          { return k.storeKey }
func (k *Keeper) TestHost() *lxphost.Host             { return k.host }
func (k *Keeper) TestAuthzKeeper() *types.AuthzKeeper { return &k.authzKeeper }
func (k *Keeper) TestEVMKeeper() types.EVMKeeper      { return k.evmKeeper }
func (k *Keeper) TestAuthority() string               { return k.authority }
func (k *Keeper) Execute(ctx sdk.Context, actor Actor, payload []byte, key [32]byte) (Result, error) {
	return k.execute(ctx, actor, payload, key)
}

func (f *authViewFixture) Private() ed25519.PrivateKey { return f.private }
func (f *authViewFixture) Public() *[32]byte           { return &f.public }
func (f *authViewFixture) Delegate() *[32]byte         { return &f.delegate }
func (f *authViewFixture) CapabilityKey() *[32]byte    { return &f.capabilityKey }
func (f *authViewFixture) DIDID() *[32]byte            { return &f.didID }
func (f *authViewFixture) SessionID() *[32]byte        { return &f.sessionID }
func (f *authViewFixture) CapabilityID() *[32]byte     { return &f.capabilityID }
func (f *authViewFixture) Asset() *[32]byte            { return &f.asset }
func (f *authViewFixture) Purpose() *[32]byte          { return &f.purpose }
func (f *authViewFixture) Timestamp() uint64           { return f.timestamp }
func (f *authViewFixture) GrantBody(kind byte) []byte  { return f.grantBody(kind) }
