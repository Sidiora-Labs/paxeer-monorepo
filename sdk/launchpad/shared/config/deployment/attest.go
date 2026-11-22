package deployment

import (
	"context"
	"errors"
	"math/big"
	"sort"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// Reader is satisfied by the frozen go-ethereum ethclient read methods. All
// state reads are hash pinned; it deliberately offers no signing/write methods.
// The integrator must supply a transport with bounded response sizes.
// Policies allowing parallel reads require a reader safe for concurrent use.
type Reader interface {
	ChainID(context.Context) (*big.Int, error)
	HeaderByHash(context.Context, common.Hash) (*types.Header, error)
	CodeAtHash(context.Context, common.Address, common.Hash) ([]byte, error)
	StorageAtHash(context.Context, common.Address, common.Hash, common.Hash) ([]byte, error)
	CallContractAtHash(context.Context, ethereum.CallMsg, common.Hash) ([]byte, error)
}

// ReportedHeaderReader is implemented by readers that can return the header the
// node itself identifies by the given hash. Chains whose consensus block hash is
// not the go-ethereum header hash (CometBFT-backed EVMs such as Paxeer) can only
// be verified this way; the node vouches for the identity instead of a local
// recomputation.
type ReportedHeaderReader interface {
	HeaderByReportedHash(context.Context, common.Hash) (*types.Header, error)
}

func headerIdentity(ctx context.Context, r Reader, block common.Hash) (*types.Header, error) {
	if ctx.Err() != nil {
		return nil, Issue{Code: valueBlockIdentityUnverified}
	}
	if block == (common.Hash{}) {
		return nil, Issue{Code: valueBlockIdentityUnverified}
	}
	if rr, ok := r.(ReportedHeaderReader); ok {
		h, err := rr.HeaderByReportedHash(ctx, block)
		if err != nil || h == nil || h.Number == nil || !h.Number.IsUint64() {
			return nil, Issue{Code: valueBlockIdentityUnverified}
		}
		return h, nil
	}
	h, err := r.HeaderByHash(ctx, block)
	if err != nil || h == nil || h.Number == nil || !h.Number.IsUint64() || h.Hash() != block {
		return nil, Issue{Code: valueBlockIdentityUnverified}
	}
	return h, nil
}

var ImplementationSlot = common.HexToHash("0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc")
var BeaconSlot = common.HexToHash("0xa3f0ad74e5423aebfd80d3ef4346578335a9a72aeaee59ff6cb3582b35133d50")

type Policy struct {
	Now     time.Time
	MaxAge  time.Duration
	Timeout time.Duration
	// ConcurrentReads bounds reads per attestation to at most sixteen; zero and one are serial.
	ConcurrentReads int
}

func (p Policy) validate() error {
	if p.Now.IsZero() || p.MaxAge <= 0 || p.MaxAge > time.Hour || p.Timeout <= 0 || p.Timeout > time.Minute || p.ConcurrentReads < 0 || p.ConcurrentReads > 16 {
		return Issue{Code: "attestation_policy_invalid"}
	}
	return nil
}

type Identity struct {
	Role                  string  `json:"role"`
	Address               string  `json:"address"`
	RuntimeHash           string  `json:"runtimeHash,omitempty"`
	ImplementationAddress string  `json:"implementationAddress,omitempty"`
	ImplementationHash    string  `json:"implementationHash,omitempty"`
	BeaconAddress         string  `json:"beaconAddress,omitempty"`
	Verified              bool    `json:"verified"`
	Issues                []Issue `json:"issues,omitempty"`
}
type ActivationAttestation struct {
	ManifestHash      string              `json:"manifestHash"`
	DeploymentVersion string              `json:"deploymentVersion"`
	BlockHash         string              `json:"blockHash"`
	BlockNumber       uint64              `json:"blockNumber"`
	ObservedAt        time.Time           `json:"observedAt"`
	BlockTime         time.Time           `json:"blockTime"`
	Identities        map[string]Identity `json:"identities"`
	ActualOwner       string              `json:"actualOwner,omitempty"`
	PendingOwner      string              `json:"pendingOwner,omitempty"`
	CreationFeeRaw    string              `json:"creationFeeRaw,omitempty"`
	PolicyValues      map[string]string   `json:"policyValues,omitempty"`
	SettlementAddress string              `json:"settlementAddress,omitempty"`
	USDLDecimals      int                 `json:"usdlDecimals"`
	Eligible          bool                `json:"eligible"`
	Issues            []Issue             `json:"issues,omitempty"`
}

func selector(name string) []byte { return crypto.Keccak256([]byte(name))[:4] }
func callWord(ctx context.Context, r Reader, address common.Address, name string, block common.Hash) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, Issue{Code: "identity_read_failed"}
	}
	b, e := r.CallContractAtHash(ctx, ethereum.CallMsg{To: &address, Data: selector(name)}, block)
	if e != nil || len(b) != 32 {
		return nil, Issue{Code: "identity_read_failed"}
	}
	return b, nil
}
func wordAddress(b []byte) (common.Address, error) {
	if len(b) != 32 {
		return common.Address{}, Issue{Code: "slot_malformed"}
	}
	for _, v := range b[:12] {
		if v != 0 {
			return common.Address{}, Issue{Code: "slot_malformed"}
		}
	}
	return common.BytesToAddress(b[12:]), nil
}
func implementationAt(ctx context.Context, r Reader, address common.Address, name string, block common.Hash) (common.Address, error) {
	b, e := callWord(ctx, r, address, name, block)
	if e != nil {
		return common.Address{}, e
	}
	return wordAddress(b)
}
func codeHash(ctx context.Context, r Reader, a common.Address, block common.Hash) (string, error) {
	if ctx.Err() != nil {
		return "", Issue{Code: valueCodeReadFailed}
	}
	b, e := r.CodeAtHash(ctx, a, block)
	if e != nil || len(b) == 0 || len(b) > 128<<10 {
		return "", Issue{Code: valueCodeReadFailed}
	}
	return crypto.Keccak256Hash(b).Hex(), nil
}

// Attest resolves identities, configuration and ownership from one committed
// block. Unknown code preserves observations but never selects an ABI for writes.
func Attest(ctx context.Context, r Reader, m Manifest, block common.Hash, p Policy) (ActivationAttestation, error) {
	if err := m.Validate(); err != nil {
		return ActivationAttestation{}, err
	}
	if err := p.validate(); err != nil {
		return ActivationAttestation{}, err
	}
	if r == nil || block == (common.Hash{}) {
		return ActivationAttestation{}, Issue{Code: valueIdentityUnverified}
	}
	ctx, cancel := context.WithTimeout(ctx, p.Timeout)
	defer cancel()
	if ctx.Err() != nil {
		return ActivationAttestation{}, Issue{Code: "chain_read_failed"}
	}
	chain, err := r.ChainID(ctx)
	if err != nil {
		return ActivationAttestation{}, Issue{Code: "chain_read_failed"}
	}
	if chain == nil || !chain.IsInt64() || chain.Int64() != ChainID {
		return ActivationAttestation{}, Issue{Code: "wrong_chain"}
	}
	h, err := headerIdentity(ctx, r, block)
	if err != nil {
		return ActivationAttestation{}, err
	}
	if h.Time > 1<<63-1 || p.Now.Unix() < 0 {
		return ActivationAttestation{}, Issue{Code: valueAttestationStale}
	}
	//nolint:gosec // Header time is bounded to the signed Unix range above.
	bt := time.Unix(int64(h.Time), 0).UTC()
	if bt.After(p.Now) || p.Now.Sub(bt) > p.MaxAge {
		return ActivationAttestation{}, Issue{Code: valueAttestationStale}
	}
	if h.Number.Uint64() < m.ActivationHeight {
		return ActivationAttestation{}, Issue{Code: "deployment_not_activated"}
	}
	hash, _ := m.Hash()
	a := ActivationAttestation{ManifestHash: hash, DeploymentVersion: m.DeploymentVersion, BlockHash: block.Hex(), BlockNumber: h.Number.Uint64(), ObservedAt: p.Now, BlockTime: bt, Identities: map[string]Identity{}, USDLDecimals: -1}
	names := make([]string, 0, len(m.Roles))
	for name := range m.Roles {
		names = append(names, name)
	}
	sort.Strings(names)
	identities := make([]Identity, len(names))
	reads := make([]func(), len(names))
	for i, name := range names {
		reads[i] = func() { identities[i] = attestIdentity(ctx, r, m, name, block) }
	}
	runAttestationReads(ctx, p.ConcurrentReads, reads)
	for i, name := range names {
		a.Identities[name] = identities[i]
		a.Issues = append(a.Issues, identities[i].Issues...)
	}
	// Do not decode getters for contracts whose runtime and implementation failed.
	governance := a.Identities[valuePoolBeacon].Verified
	configuration := a.Identities[valueProtocolConfig].Verified && a.Identities[valueUSDL].Verified
	getters := []attestationGetter{}
	if governance {
		addr := common.HexToAddress(m.Roles[valuePoolBeacon].Address)
		getters = append(getters, attestationGetter{address: addr, signature: "owner()"}, attestationGetter{address: addr, signature: "pendingOwner()"})
	}
	policySignatures := []string{"minFeeBps()", "maxFeeBps()", "baseFeeBps()", "protocolFeeBps()", "virtualUsdlDefault()", "virtualTokenDefault()", "feeDecayRate()", "volatilityWeight()", "concentrationWeight()"}
	policyStart := len(getters)
	if configuration {
		cfg := common.HexToAddress(m.Roles[valueProtocolConfig].Address)
		for _, signature := range policySignatures {
			getters = append(getters, attestationGetter{address: cfg, signature: signature})
		}
		getters = append(getters, attestationGetter{address: cfg, signature: "creationFee()"}, attestationGetter{address: cfg, signature: "usdlAddress()"}, attestationGetter{address: common.HexToAddress(m.Roles[valueUSDL].Address), signature: "decimals()"})
	}
	words := readAttestationGetters(ctx, r, block, p.ConcurrentReads, getters)
	if governance {
		owner, e1 := words[0].address()
		pending, e2 := words[1].address()
		if e1 != nil || e2 != nil {
			a.Issues = append(a.Issues, Issue{"governance_read_failed", valuePoolBeacon})
		} else {
			a.ActualOwner = owner.Hex()
			a.PendingOwner = pending.Hex()
			if owner != common.HexToAddress(m.Governance.ExpectedBeaconOwner) || pending != (common.Address{}) {
				a.Issues = append(a.Issues, Issue{valueGovernanceHandoffPending, valuePoolBeacon})
			}
		}
	}
	if configuration {
		a.PolicyValues = map[string]string{}
		for i, signature := range policySignatures {
			value, readErr := words[policyStart+i].value, words[policyStart+i].err
			if readErr != nil {
				a.Issues = append(a.Issues, Issue{Code: "configuration_policy_read_failed", Role: valueProtocolConfig})
			} else {
				a.PolicyValues[signature] = new(big.Int).SetBytes(value).String()
			}
		}
		creationStart := policyStart + len(policySignatures)
		fee, e1 := words[creationStart].value, words[creationStart].err
		usdl, e2 := words[creationStart+1].address()
		dec, e3 := words[creationStart+2].value, words[creationStart+2].err
		if e1 != nil || e2 != nil || e3 != nil {
			a.Issues = append(a.Issues, Issue{"creation_policy_read_failed", valueProtocolConfig})
		} else {
			a.SettlementAddress = usdl.Hex()
			a.CreationFeeRaw = new(big.Int).SetBytes(fee).String()
			d := new(big.Int).SetBytes(dec)
			if d.IsInt64() && d.Int64() <= 255 {
				a.USDLDecimals = int(d.Int64())
			}
			if a.CreationFeeRaw != CreationFeeRaw || a.USDLDecimals != 6 || usdl != common.HexToAddress(m.Roles[valueUSDL].Address) {
				a.Issues = append(a.Issues, Issue{valueCreationPolicyMismatch, valueProtocolConfig})
			}
		}
	}
	if m.Status == valuePendingBeaconAcceptance {
		a.Issues = append(a.Issues, Issue{"manifest_governance_pending", valuePoolBeacon})
	} else if m.Status != valueActive && m.Status != valueEligible {
		a.Issues = append(a.Issues, Issue{Code: valueManifestNotEligible})
	}
	if ctx.Err() != nil {
		return a, Issue{Code: "attestation_canceled"}
	}
	a.Eligible = len(a.Issues) == 0
	return a, nil
}
func hasImplementation(kind string) bool {
	return kind != valueDirect && kind != valueSurface && kind != valuePrecompile
}

// runtimeIdentity is the positive identity check of an attested role: the
// runtime hash for EVM code, the fixed precompile address for a precompile.
func runtimeIdentity(r Role, id Identity) bool {
	if r.Kind == valuePrecompile {
		return id.RuntimeHash == "" && validAddress(r.Address) && common.HexToAddress(r.Address) == common.HexToAddress(LaunchpadPrecompileAddress)
	}
	return r.AcceptsRuntimeHash(id.RuntimeHash)
}
func equalHash(a, b string) bool {
	return validHash(a) && validHash(b) && common.HexToHash(a) == common.HexToHash(b)
}

// ValidateFor rejects caller-supplied, stale or cross-deployment attestations.
func (a ActivationAttestation) ValidateFor(m Manifest, p Policy) error {
	if err := p.validate(); err != nil {
		return err
	}
	hash, err := m.Hash()
	if err != nil {
		return err
	}
	if a.ManifestHash != hash || a.DeploymentVersion != m.DeploymentVersion {
		return Issue{Code: valueAttestationRevisionMismatch}
	}
	if !validHash(a.BlockHash) || a.BlockNumber < m.ActivationHeight || a.ObservedAt.IsZero() || a.BlockTime.IsZero() || a.ObservedAt.After(p.Now) || a.BlockTime.After(a.ObservedAt) || p.Now.Sub(a.ObservedAt) > p.MaxAge || p.Now.Sub(a.BlockTime) > p.MaxAge {
		return Issue{Code: valueAttestationStale}
	}
	if !a.Eligible || len(a.Issues) > 0 {
		blocked := []error{Issue{Code: "activation_blocked"}}
		for _, issue := range a.Issues {
			blocked = append(blocked, issue)
		}
		return errors.Join(blocked...)
	}
	for name, r := range m.Roles {
		id, ok := a.Identities[name]
		if !ok || !id.Verified || len(id.Issues) > 0 || !validAddress(id.Address) || common.HexToAddress(id.Address) != common.HexToAddress(r.Address) || !runtimeIdentity(r, id) {
			return Issue{valueIdentityUnverified, name}
		}
		if hasImplementation(r.Kind) && (!validAddress(id.ImplementationAddress) || common.HexToAddress(id.ImplementationAddress) != common.HexToAddress(r.ImplementationAddress) || !equalHash(id.ImplementationHash, r.ImplementationHash)) {
			return Issue{valueIdentityUnverified, name}
		}
		if r.Kind == valueBeaconProxy && (!validAddress(id.BeaconAddress) || common.HexToAddress(id.BeaconAddress) != common.HexToAddress(r.BeaconAddress)) {
			return Issue{valueIdentityUnverified, name}
		}
	}
	var errs []error
	if m.Status != valueEligible && m.Status != valueActive {
		errs = append(errs, Issue{Code: valueManifestNotEligible})
	}
	if !validAddress(a.ActualOwner) || common.HexToAddress(a.ActualOwner) != common.HexToAddress(m.Governance.ExpectedBeaconOwner) || a.PendingOwner != (common.Address{}).Hex() {
		errs = append(errs, Issue{valueGovernanceHandoffPending, valuePoolBeacon})
	}
	if a.CreationFeeRaw != CreationFeeRaw || a.USDLDecimals != 6 || !validAddress(a.SettlementAddress) || common.HexToAddress(a.SettlementAddress) != common.HexToAddress(SettlementAddress) {
		errs = append(errs, Issue{Code: valueCreationPolicyMismatch})
	}
	return errors.Join(errs...)
}

// ValidateFamilyFor allows an unavailable optional contract/read role to disable
// only dependent families. Global governance and chain/deployment freshness still
// gate every executable family; creation additionally requires the fee policy.
func (a ActivationAttestation) ValidateFamilyFor(m Manifest, p Policy, roles []string, creation bool) error {
	if err := p.validate(); err != nil {
		return err
	}
	hash, err := m.Hash()
	if err != nil {
		return err
	}
	if hash != a.ManifestHash || m.DeploymentVersion != a.DeploymentVersion {
		return Issue{Code: valueAttestationRevisionMismatch}
	}
	if !validHash(a.BlockHash) || a.BlockNumber < m.ActivationHeight || a.ObservedAt.IsZero() || a.BlockTime.IsZero() || a.ObservedAt.After(p.Now) || a.BlockTime.After(a.ObservedAt) || p.Now.Sub(a.ObservedAt) > p.MaxAge || p.Now.Sub(a.BlockTime) > p.MaxAge {
		return Issue{Code: valueAttestationStale}
	}
	if m.Status != valueEligible && m.Status != valueActive {
		if m.Status == valuePendingBeaconAcceptance {
			return Issue{Code: "manifest_governance_pending", Role: valuePoolBeacon}
		}
		return Issue{Code: valueManifestNotEligible}
	}
	if !validAddress(a.ActualOwner) || common.HexToAddress(a.ActualOwner) != common.HexToAddress(m.Governance.ExpectedBeaconOwner) || a.PendingOwner != (common.Address{}).Hex() {
		return Issue{valueGovernanceHandoffPending, valuePoolBeacon}
	}
	for _, issue := range a.Issues {
		if issue.Code == "configuration_policy_read_failed" {
			for _, name := range roles {
				if name == issue.Role {
					return issue
				}
			}
		}
	}
	for _, name := range roles {
		r, ok := m.Roles[name]
		id, found := a.Identities[name]
		if !ok || !found || !id.Verified || len(id.Issues) != 0 || !validAddress(id.Address) || common.HexToAddress(r.Address) != common.HexToAddress(id.Address) || !runtimeIdentity(r, id) {
			return Issue{valueIdentityUnverified, name}
		}
		if hasImplementation(r.Kind) && (!validAddress(id.ImplementationAddress) || common.HexToAddress(id.ImplementationAddress) != common.HexToAddress(r.ImplementationAddress) || !equalHash(id.ImplementationHash, r.ImplementationHash)) {
			return Issue{valueIdentityUnverified, name}
		}
		if r.Kind == valueBeaconProxy && (!validAddress(id.BeaconAddress) || common.HexToAddress(id.BeaconAddress) != common.HexToAddress(r.BeaconAddress)) {
			return Issue{valueIdentityUnverified, name}
		}
	}
	for _, name := range roles {
		if name == valueUSDL && (a.USDLDecimals != 6 || !validAddress(a.SettlementAddress) || common.HexToAddress(a.SettlementAddress) != common.HexToAddress(SettlementAddress)) {
			return Issue{Code: "settlement_policy_mismatch", Role: valueUSDL}
		}
	}
	if creation && (a.CreationFeeRaw != CreationFeeRaw || a.USDLDecimals != 6) {
		return Issue{Code: valueCreationPolicyMismatch}
	}
	return nil
}
