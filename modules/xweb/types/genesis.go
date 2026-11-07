package types

import (
	"bytes"
	"encoding/binary"

	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
)

const (
	ProgramPendingCapacity    = 256
	ProgramCommittedCapacity  = 64
	ProgramPendingRecordBytes = 123
	ProgramAnswerHeaderBytes  = 40
	ProgramAnswerChunkBytes   = 1024
)

type ProgramPendingEntry struct {
	ProgramID Hash32 `json:"program_id"`
	RequestID uint64 `json:"request_id"`
	Record    []byte `json:"record"`
}

type ProgramAnswerEntry struct {
	ProgramID Hash32 `json:"program_id"`
	RequestID uint64 `json:"request_id"`
	Part      uint8  `json:"part"`
	Value     []byte `json:"value"`
}

type ProgramFeeDenomEntry struct {
	Asset Hash32 `json:"asset"`
	Denom string `json:"denom"`
}

// GenesisState is the whole xweb state. The default carries the documented
// parameters, no attestors, no requests and is paused: the module ships
// dormant and only governance registers attestors and unpauses it.
type GenesisState struct {
	Params                Params                 `json:"params"`
	Paused                bool                   `json:"paused"`
	Attestors             AttestorSet            `json:"attestors"`
	Nonce                 uint64                 `json:"nonce"`
	Requests              []Request              `json:"requests"`
	Results               []Result               `json:"results"`
	ProgramPending        []ProgramPendingEntry  `json:"program_pending,omitempty"`
	ProgramAnswers        []ProgramAnswerEntry   `json:"program_answers,omitempty"`
	ProgramFeeDenoms      []ProgramFeeDenomEntry `json:"program_fee_denoms_by_asset,omitempty"`
	ProgramPendingCount   uint64                 `json:"program_pending_count,omitempty"`
	ProgramCommittedCount uint64                 `json:"program_committed_count,omitempty"`
}

// DefaultAuthority is the gov module account.
func DefaultAuthority() string {
	return authtypes.NewModuleAddress(govtypes.ModuleName).String()
}

func DefaultGenesis() *GenesisState {
	return &GenesisState{Params: DefaultParams(DefaultAuthority()), Paused: true}
}

func (g GenesisState) Validate() error {
	if err := g.Params.Validate(); err != nil {
		return err
	}
	if err := g.Attestors.Validate(); err != nil {
		return err
	}
	requests := map[uint64]Request{}
	for _, request := range g.Requests {
		if err := request.Validate(); err != nil {
			return ErrInvalidGenesis.Wrap(err.Error())
		}
		if request.ID > g.Nonce {
			return ErrInvalidGenesis.Wrapf("request %d above nonce %d", request.ID, g.Nonce)
		}
		if _, duplicate := requests[request.ID]; duplicate {
			return ErrInvalidGenesis.Wrapf("duplicate request %d", request.ID)
		}
		requests[request.ID] = request
	}
	results := map[uint64]bool{}
	for _, result := range g.Results {
		if err := result.Validate(); err != nil {
			return ErrInvalidGenesis.Wrap(err.Error())
		}
		request, found := requests[result.RequestID]
		if !found || request.Status != StatusFulfilled {
			return ErrInvalidGenesis.Wrapf("result %d has no fulfilled request", result.RequestID)
		}
		if results[result.RequestID] {
			return ErrInvalidGenesis.Wrapf("duplicate result %d", result.RequestID)
		}
		results[result.RequestID] = true
	}
	for id, request := range requests {
		if request.Status == StatusFulfilled && !results[id] {
			return ErrInvalidGenesis.Wrapf("fulfilled request %d has no result", id)
		}
	}
	if g.ProgramPendingCount > ProgramPendingCapacity {
		return ErrInvalidGenesis.Wrapf("program pending count %d exceeds capacity %d", g.ProgramPendingCount, ProgramPendingCapacity)
	}
	if g.ProgramCommittedCount > ProgramCommittedCapacity {
		return ErrInvalidGenesis.Wrapf("program committed count %d exceeds capacity %d", g.ProgramCommittedCount, ProgramCommittedCapacity)
	}
	feeDenoms := make(map[Hash32]string, len(g.ProgramFeeDenoms))
	for _, entry := range g.ProgramFeeDenoms {
		if _, duplicate := feeDenoms[entry.Asset]; duplicate {
			return ErrInvalidGenesis.Wrapf("duplicate program fee asset %s", entry.Asset.Hex())
		}
		switch entry.Denom {
		case ProgramFeeDenomPAX, ProgramFeeDenomSID, ProgramFeeDenomUSDC, ProgramFeeDenomUSDL:
		default:
			return ErrInvalidGenesis.Wrapf("unmapped program fee denom %q", entry.Denom)
		}
		feeDenoms[entry.Asset] = entry.Denom
	}
	type programRequestKey struct {
		programID Hash32
		requestID uint64
	}
	programRecords := make(map[programRequestKey]bool, len(g.ProgramPending))
	var pending, committed uint64
	for _, entry := range g.ProgramPending {
		record := entry.Record
		if len(record) != ProgramPendingRecordBytes || record[0] != 1 ||
			!ProgramKind(record[1]) || record[122] > 1 {
			return ErrInvalidGenesis.Wrapf("malformed program record %s/%d", entry.ProgramID.Hex(), entry.RequestID)
		}
		key := programRequestKey{entry.ProgramID, entry.RequestID}
		if _, duplicate := programRecords[key]; duplicate {
			return ErrInvalidGenesis.Wrapf("duplicate program record %s/%d", entry.ProgramID.Hex(), entry.RequestID)
		}
		fulfilled := record[122] == 1
		programRecords[key] = fulfilled
		// Historical zero-fee pending fixtures have no denomination mapping.
		// Paid requests need it before settlement; fulfilled records retain it.
		if fulfilled || !bytes.Equal(record[98:114], make([]byte, 16)) {
			var asset Hash32
			copy(asset[:], record[34:66])
			if _, found := feeDenoms[asset]; !found {
				return ErrInvalidGenesis.Wrapf("program record %s/%d has no fee denom for %s",
					entry.ProgramID.Hex(), entry.RequestID, asset.Hex())
			}
		}
		if fulfilled {
			committed++
		} else {
			pending++
		}
	}
	if g.ProgramPendingCount != pending {
		return ErrInvalidGenesis.Wrapf("program pending count %d disagrees with %d records", g.ProgramPendingCount, pending)
	}
	if g.ProgramCommittedCount != committed {
		return ErrInvalidGenesis.Wrapf("program committed count %d disagrees with %d records", g.ProgramCommittedCount, committed)
	}
	type programAnswerKey struct {
		programRequestKey
		part uint8
	}
	answerParts := make(map[programAnswerKey][]byte, len(g.ProgramAnswers))
	var headers uint64
	for _, entry := range g.ProgramAnswers {
		key := programRequestKey{entry.ProgramID, entry.RequestID}
		if !programRecords[key] {
			return ErrInvalidGenesis.Wrapf("program answer %s/%d has no fulfilled record", entry.ProgramID.Hex(), entry.RequestID)
		}
		partKey := programAnswerKey{key, entry.Part}
		if _, duplicate := answerParts[partKey]; duplicate {
			return ErrInvalidGenesis.Wrapf("duplicate program answer %s/%d part %d", entry.ProgramID.Hex(), entry.RequestID, entry.Part)
		}
		answerParts[partKey] = entry.Value
		if entry.Part == 0 {
			if len(entry.Value) != ProgramAnswerHeaderBytes {
				return ErrInvalidGenesis.Wrapf("malformed program answer header %s/%d", entry.ProgramID.Hex(), entry.RequestID)
			}
			headers++
		}
	}
	if g.ProgramCommittedCount != headers {
		return ErrInvalidGenesis.Wrapf("program committed count %d disagrees with %d answer headers", g.ProgramCommittedCount, headers)
	}
	expectedParts := 0
	for key, fulfilled := range programRecords {
		if !fulfilled {
			continue
		}
		header := answerParts[programAnswerKey{key, 0}]
		// Ownership, uniqueness and header count above ensure one header per
		// fulfilled record. Decode the same lengths consumed by GetProgramResult.
		fullLength := binary.BigEndian.Uint32(header[32:36])
		responseLength := binary.BigEndian.Uint32(header[36:40])
		if responseLength > MaxResponseBytes || responseLength > fullLength {
			return ErrInvalidGenesis.Wrapf("invalid program answer lengths %s/%d: response %d, full %d",
				key.programID.Hex(), key.requestID, responseLength, fullLength)
		}
		expectedParts++
		for part, offset := uint8(1), uint32(0); offset < responseLength; part, offset = part+1, offset+ProgramAnswerChunkBytes {
			expected := min(uint32(ProgramAnswerChunkBytes), responseLength-offset)
			if chunk := answerParts[programAnswerKey{key, part}]; uint32(len(chunk)) != expected {
				return ErrInvalidGenesis.Wrapf("malformed program answer %s/%d part %d: want %d bytes",
					key.programID.Hex(), key.requestID, part, expected)
			}
			expectedParts++
		}
	}
	if len(answerParts) != expectedParts {
		return ErrInvalidGenesis.Wrap("unexpected program answer parts")
	}
	return nil
}
