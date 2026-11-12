package decode

import (
	"fmt"
	"sort"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

type CapabilityPolicy struct {
	Family          string `json:"family"`
	ProjectionReady bool   `json:"projectionReady"`
	PublicEnabled   bool   `json:"publicEnabled"`
}

type FamilyAvailability struct {
	Family            string     `json:"family"`
	DeploymentPresent bool       `json:"deploymentPresent"`
	ABISupported      bool       `json:"abiSupported"`
	ProjectionReady   bool       `json:"projectionReady"`
	PublicEnabled     bool       `json:"publicEnabled"`
	CoverageReady     bool       `json:"coverageReady"`
	Available         bool       `json:"available"`
	Missing           []string   `json:"missing"`
	Coverage          []Coverage `json:"coverage"`
}

func optionalFamily(role, name string) string {
	switch role {
	case valuePECOR:
		return valuePecor
	case valuePECOROrders:
		return valueOrders
	case valuePECORVault:
		return valueVault
	case valueMetaAGRouter, "MetaAGQuoter", "SidioraAdapter", "VaultAdapter":
		return valueRouting
	case valueTransactionTracker:
		return "tracker"
	case valuePriceOracle, valueOracleHub:
		return valueOracle
	case valueEventEmitter:
		switch name {
		case valuePecorSwap:
			return valuePecor
		case valuePecorOrderCreated, valuePecorOrderLifecycle:
			return valueOrders
		case valueVaultFlow:
			return valueVault
		case valueBestRouteSwap:
			return valueRouting
		case valuePriceUpdated, valueCircuitBreakerTriggered, valueOracleAdapterLifecycle:
			return valueOracle
		}
	}
	return ""
}

func eventFamily(role, name string) string {
	if f := optionalFamily(role, name); f != "" {
		return f
	}
	return family(name)
}

func nativeRoles(f string) []string {
	switch f {
	case valuePecor:
		return []string{valuePECOR}
	case valueOrders:
		return []string{valuePECOROrders}
	case valueVault:
		return []string{valuePECORVault}
	case valueRouting:
		return []string{valueMetaAGRouter}
	case "tracker":
		return []string{valueTransactionTracker}
	case valueOracle:
		return []string{valuePriceOracle, valueOracleHub}
	}
	return nil
}

func mirrorNames(f string) []string {
	switch f {
	case valuePecor:
		return []string{valuePecorSwap}
	case valueOrders:
		return []string{valuePecorOrderCreated, valuePecorOrderLifecycle}
	case valueVault:
		return []string{valueVaultFlow}
	case valueRouting:
		return []string{valueBestRouteSwap}
	case valueOracle:
		return []string{valuePriceUpdated, valueCircuitBreakerTriggered, valueOracleAdapterLifecycle}
	}
	return nil
}

func (r *Registry) OptionalAvailability(at Order, policies []CapabilityPolicy) ([]FamilyAvailability, error) {
	if r == nil {
		return nil, fmt.Errorf("registry required")
	}
	seen := map[string]bool{}
	out := make([]FamilyAvailability, 0, len(policies))
	for _, p := range policies {
		roles := nativeRoles(p.Family)
		if len(roles) == 0 || seen[p.Family] {
			return nil, fmt.Errorf("invalid or duplicate optional family %q", p.Family)
		}
		seen[p.Family] = true
		a := FamilyAvailability{Family: p.Family, ProjectionReady: p.ProjectionReady, PublicEnabled: p.PublicEnabled, ABISupported: true, CoverageReady: true, Missing: []string{}, Coverage: []Coverage{}}
		active := map[string]bool{}
		mirrors := map[string]bool{}
		for _, epochs := range r.epochs {
			for _, epoch := range epochs {
				d := epoch.deployment
				if at.less(d.From) || (d.Until != nil && !at.less(*d.Until)) {
					continue
				}
				if d.Optional && d.Enabled && optionalFamily(d.Role, "") == p.Family {
					active[d.Role] = true
				}
				for _, ev := range epoch.events {
					if optionalFamily(d.Role, ev.Name) != p.Family {
						continue
					}
					state := valueUnsupported
					if (d.Optional && d.Enabled) || (d.Role == valueEventEmitter && optionalEnabled(d, ev.Name)) {
						state = valueOptionalDeployed
					}
					a.Coverage = append(a.Coverage, Coverage{d.Role, d.ABIRevision, ev.Name, ev.Sig, state, authority(d.Role, ev.Name), p.Family})
					if d.Role == valueEventEmitter && state != valueUnsupported {
						mirrors[ev.Name] = true
					}
				}
			}
		}
		for _, role := range roles {
			if !active[role] {
				a.Missing = append(a.Missing, "deployment:"+role)
			}
		}
		a.DeploymentPresent = len(active) == len(roles)
		for _, role := range roles {
			found := false
			for _, coverage := range r.coverage {
				if coverage.Role == role {
					found = true
					break
				}
			}
			if !found {
				a.ABISupported = false
				a.Missing = append(a.Missing, "abi:"+role)
			}
		}
		for _, name := range mirrorNames(p.Family) {
			if !mirrors[name] {
				a.Missing = append(a.Missing, "mirror:"+name)
				a.CoverageReady = false
			}
		}
		if !p.ProjectionReady {
			a.Missing = append(a.Missing, "projection")
		}
		if !p.PublicEnabled {
			a.Missing = append(a.Missing, "public_flag")
		}
		a.Available = a.DeploymentPresent && a.ABISupported && a.CoverageReady && a.ProjectionReady && a.PublicEnabled
		sort.Strings(a.Missing)
		sort.Slice(a.Coverage, func(i, j int) bool {
			return a.Coverage[i].Role+a.Coverage[i].ABIRevision+a.Coverage[i].Signature < a.Coverage[j].Role+a.Coverage[j].ABIRevision+a.Coverage[j].Signature
		})
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Family < out[j].Family })
	return out, nil
}

type OptionalFailure struct {
	Identity RawIdentity `json:"identity"`
	Family   string      `json:"family"`
	Reason   string      `json:"reason"`
}
type OptionalBatch struct {
	Core     []*DecodedEnvelope `json:"core"`
	Optional []*DecodedEnvelope `json:"optional"`
	Failures []OptionalFailure  `json:"failures"`
}

func (r *Registry) DecodeOptionalBatch(logs []ethtypes.Log) (*OptionalBatch, error) {
	if r == nil {
		return nil, fmt.Errorf("registry required")
	}
	unique, err := DeduplicateRaw(logs)
	if err != nil {
		return nil, err
	}
	out := &OptionalBatch{Core: []*DecodedEnvelope{}, Optional: []*DecodedEnvelope{}, Failures: []OptionalFailure{}}
	for _, l := range unique {
		e, decodeErr := r.Decode(l)
		f := optionalFamily(e.EmitterRole, e.EventName)
		if f == "" && e.EmitterRole == "" {
			for _, epoch := range r.epochs[l.Address] {
				d := epoch.deployment
				if !e.Raw.Order.less(d.From) && (d.Until == nil || e.Raw.Order.less(*d.Until)) {
					f = optionalFamily(d.Role, "")
					break
				}
			}
		}
		if f != "" {
			out.Optional = append(out.Optional, e)
			if decodeErr != nil || e.Status != valueDecoded {
				out.Failures = append(out.Failures, OptionalFailure{e.Raw.Identity, f, e.FailureReason})
			}
			continue
		}
		if decodeErr != nil {
			return nil, decodeErr
		}
		out.Core = append(out.Core, e)
	}
	return out, nil
}

type OracleTelemetry struct {
	Identity    RawIdentity `json:"identity"`
	Emitter     string      `json:"emitter"`
	ABIRevision string      `json:"abiRevision"`
	Event       string      `json:"event"`
	Token       *string     `json:"token"`
	RawValue    *string     `json:"rawValue"`
	Scale       *uint8      `json:"scale"`
	ObservedAt  *string     `json:"observedAt"`
	SourceID    *string     `json:"sourceId"`
	Confidence  *string     `json:"confidence"`
	Indicative  bool        `json:"indicative"`
	Status      string      `json:"status"`
}

func (r *Registry) DecodeOracle(l ethtypes.Log) (*OracleTelemetry, error) {
	e, err := r.Decode(l)
	if err != nil {
		return nil, err
	}
	if optionalFamily(e.EmitterRole, e.EventName) != valueOracle || e.Event == nil || e.Status != valueDecoded {
		return nil, fmt.Errorf("approved enabled oracle event required")
	}
	get := func(name string) *string {
		v, ok := e.Event.Args[name].(string)
		if !ok {
			return nil
		}
		return &v
	}
	t := &OracleTelemetry{Identity: e.Raw.Identity, Emitter: e.Raw.Emitter, ABIRevision: e.ABIRevision, Event: e.EventName, Token: get("token"), RawValue: get("price"), ObservedAt: get("timestamp"), SourceID: get("sourceId"), Confidence: get("confidence"), Indicative: true, Status: "telemetry"}
	if e.EmitterRole == valuePriceOracle && e.EventName == valuePriceUpdated {
		scale := uint8(18)
		t.Scale = &scale
	}
	if t.RawValue == nil {
		t.Status = "lifecycle"
	} else if t.Scale == nil || t.ObservedAt == nil {
		t.Status = "incomplete_units_or_time"
	}
	return t, nil
}
