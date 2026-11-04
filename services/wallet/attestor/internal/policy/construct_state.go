package policy

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	"github.com/ethereum/go-ethereum/common"
)

// ConstructPin is supplied by the trusted header and address-binding reader,
// never by a signing request. All witnesses must belong to this single pin.
// The reader must refresh the pin for each evaluation, including after restart.
type ConstructPin struct {
	ChainID      uint64
	Account      common.Address
	BlockHash    common.Hash
	StateRoot    [32]byte
	ActorDID     string
	AuthorityKey [32]byte
}

// ConstructState owns verified, immutable account and budget records.
type ConstructState struct {
	pin      ConstructPin
	main     [32]byte
	accounts map[[32]byte]*codec.Account
	budgets  map[[32]byte]constructBudget
}

type constructBudget struct {
	owner, account, asset, source [32]byte
	native, closed, revoked       bool
}

// NewConstructState verifies native witnesses against an independently trusted
// pin. A raw getBudget/eth_call result is deliberately not an accepted input.
func NewConstructState(pin ConstructPin, witnesses [][]byte) (*ConstructState, error) {
	if pin.ChainID == 0 || pin.Account == (common.Address{}) || pin.BlockHash == (common.Hash{}) || pin.StateRoot == ([32]byte{}) || pin.ActorDID == "" || pin.AuthorityKey == ([32]byte{}) {
		return nil, errors.New("construct state requires a trusted chain, block, signer and authority binding")
	}
	main, err := codec.DeriveAccountID([]byte("agent:" + pin.ActorDID + ":main"))
	if err != nil {
		return nil, err
	}
	state := &ConstructState{pin: pin, main: main, accounts: map[[32]byte]*codec.Account{}, budgets: map[[32]byte]constructBudget{}}
	for _, raw := range witnesses {
		w, err := verify.StateProof(raw, pin.StateRoot)
		if err != nil {
			return nil, fmt.Errorf("construct state proof: %w", err)
		}
		switch {
		case w.AccountPath != nil:
			var id [32]byte
			copy(id[:], w.Key[1:])
			a, err := codec.DecodeAccountValue(id, w.Value)
			if err != nil {
				return nil, err
			}
			if _, exists := state.accounts[id]; exists {
				return nil, errors.New("duplicate construct account proof")
			}
			state.accounts[id] = a
		case w.ModuleID == 3 && len(w.Key) == 39 && bytes.Equal(w.Key[:7], []byte("budget:")):
			id, b, err := decodeConstructBudget(w.Value)
			if err != nil || !bytes.Equal(id[:], w.Key[7:]) {
				return nil, errors.New("invalid construct budget proof")
			}
			if _, exists := state.budgets[id]; exists {
				return nil, errors.New("duplicate construct budget proof")
			}
			state.budgets[id] = b
		default:
			return nil, errors.New("unsupported construct witness")
		}
	}
	owner := state.accounts[main]
	if owner == nil || owner.Kind != 1 || !owner.HasAuthorityKey || owner.AuthorityKey != pin.AuthorityKey || owner.Frozen {
		return nil, errors.New("construct pin does not prove the bound owner account")
	}
	return state, nil
}

func decodeConstructBudget(raw []byte) ([32]byte, constructBudget, error) {
	var id [32]byte
	var b constructBudget
	if len(raw) < 278 || raw[0] != 0 || (raw[1] != 1 && raw[1] != 2) || raw[275] > 1 || raw[276] > 1 || raw[277] > 16 {
		return id, b, errors.New("invalid budget record")
	}
	b.native = raw[1] == 2
	size := 278 + 32*int(raw[277])
	if b.native {
		size += 32
	}
	if len(raw) != size || bytes.Equal(raw[162:178], make([]byte, 16)) || binary.BigEndian.Uint64(raw[242:250]) == 0 || binary.BigEndian.Uint64(raw[258:266]) <= binary.BigEndian.Uint64(raw[250:258]) || raw[274] < 1 || raw[274] > 2 || (raw[274] == 1 && !bytes.Equal(raw[194:210], make([]byte, 16))) {
		return id, b, errors.New("invalid budget terms")
	}
	copy(id[:], raw[2:34])
	copy(b.owner[:], raw[34:66])
	copy(b.account[:], raw[66:98])
	copy(b.asset[:], raw[98:130])
	if id == ([32]byte{}) {
		return id, b, errors.New("missing budget id")
	}
	b.source = b.owner
	if b.native {
		copy(b.source[:], raw[size-32:])
		if b.source == ([32]byte{}) {
			return id, b, errors.New("missing budget source")
		}
	}
	for i := 1; i < int(raw[277]); i++ {
		offset := 278 + 32*i
		if bytes.Compare(raw[offset-32:offset], raw[offset:offset+32]) >= 0 {
			return id, b, errors.New("non-canonical budget delegates")
		}
	}
	b.closed = raw[275] != 0
	b.revoked = raw[276] != 0
	return id, b, nil
}

// ConstructAsset uses the same ledger key as layerxledger.send and grants.
// Even the chain's native asset ID is distinct from EVM value in wei.
func ConstructAsset(asset [32]byte) string { return hex.EncodeToString(asset[:]) }

func (s *ConstructState) account(id, asset [32]byte) (*codec.Account, bool, error) {
	a := s.accounts[id]
	if a == nil || !a.HasAsset || a.AssetID != asset || a.Frozen {
		return nil, false, refuse(CodeMissingField, "unresolved construct account %x for asset %x", id, asset)
	}
	owned := strings.HasPrefix(string(a.Name), "agent:"+s.pin.ActorDID+":")
	return a, owned, nil
}

func (ctx Context) constructState() (*ConstructState, error) {
	s := ctx.ConstructState
	if s == nil && ctx.readConstructState != nil {
		var err error
		s, err = ctx.readConstructState()
		if err != nil {
			return nil, refuse(CodeMissingField, "verified construct state: %v", err)
		}
	}
	if s == nil || ctx.ChainID == nil || !ctx.ChainID.IsUint64() || s.pin.ChainID != ctx.ChainID.Uint64() || s.pin.Account != ctx.Account {
		return nil, refuse(CodeMissingField, "no pinned verified construct state for signer and chain")
	}
	return s, nil
}
