//go:build cgo

package lxphost

/*
#include <stdlib.h>
#include "layerx/lxp_module_ctx.h"
#include "layerx/programs.h"
#include "layerx/lxp_governance.h"

typedef struct lxphost_read_state {
    lxp_kernel kernel;
    lxp_state_store store;
    lxp_state_journal journal;
    lx_account_registry accounts;
    lxp_identity_store identities;
    uint64_t parameters;
} lxphost_read_state;

static void *lxphost_program_read_context(lxphost_read_state *state)
{
    lxp_module_ctx *ctx = calloc(1U, sizeof(*ctx));
    if (ctx == NULL) return NULL;
    ctx->kernel = &state->kernel;
    ctx->module_id = LXP_MODULE_PROGRAMS;
    return ctx;
}

static int32_t lxphost_program_account(void *reader,
    const uint8_t program[32], const uint8_t *seed, size_t seed_length,
    const uint8_t id[32], lx_programs_account_binding *binding, lx_account *out)
{
    lxp_module_ctx *ctx = reader;
    lx_account *account = NULL;
    int32_t status;
    if (program != NULL) {
        uint8_t derived[32];
        status = lxp_programs_account_derive(program, seed, seed_length, derived);
        if (status == LXP_OK)
            status = lxp_programs_account_lookup_id(ctx, derived, binding, &account);
    } else {
        status = lxp_programs_account_lookup_id(ctx, id, binding, &account);
    }
    if (status == LXP_OK) *out = *account;
    return status;
}

static int32_t lxphost_program_bytes(void *reader, const uint8_t *key,
    size_t key_length, const uint8_t **bytes, size_t *length)
{
    return lxp_ctx_kv_get(reader, key, key_length, bytes, length);
}

static int32_t lxphost_program_active(void *reader, const uint8_t id[32])
{
    return lxp_programs_program_active(reader, id);
}

static int32_t lxphost_program_profile(void *reader, const uint8_t id[32],
    uint8_t profile[33])
{
    return lxp_programs_account_profile_read(reader, id, profile);
}

static int32_t lxphost_program_wind_down(void *reader, const uint8_t id[32],
    lx_programs_wind_down_view *view)
{
    return lxp_programs_wind_down_read(reader, id, view);
}

static int32_t lxphost_program_exit_route(void *reader, const uint8_t program[32],
    const uint8_t account[32], lx_programs_exit_route_view *route)
{
    return lxp_programs_exit_route_read(reader, program, account, route);
}
*/
import "C"

import (
	"crypto/sha256"
	"encoding/binary"
	"unsafe"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

type ProgramRecord struct {
	Policy     uint8
	Authority  [32]byte
	CodeHash   [32]byte
	ABIVersion uint16
	Version    uint32
	Active     bool
}

type ProgramInterface struct {
	Version  uint32
	Digest   [32]byte
	Encoding []byte
}

type ProgramAccount struct {
	RecordVersion           uint8
	ProgramID               [32]byte
	AccountID               [32]byte
	AssetID                 [32]byte
	Seed                    []byte
	RegisteredSequence      uint64
	RegistrationEventDigest [32]byte
}

type AccountProfile struct {
	Profile   uint8
	Principal [32]byte
}

type ValueAccount struct {
	Balance sdk.Int
	Frozen  bool
}

type WindDown struct {
	ProgramID             [32]byte
	Status                uint8
	ExitProgram           [32]byte
	Deadline              uint64
	EffectiveSequence     uint64
	ValueAccountCount     uint16
	LiveValueAccountCount uint16
}

type ExitRoute struct {
	ProgramID   [32]byte
	AccountID   [32]byte
	AssetID     [32]byte
	Destination [32]byte
	Seed        []byte
}

func (h *Host) withProgramRead(ctx sdk.Context, read func(unsafe.Pointer) error) error {
	return h.withRead(ctx, func(state *C.lxphost_read_state) error {
		module := C.lxphost_program_read_context(state)
		if module == nil {
			return Code(C.LXP_ERR_ARENA_EXHAUSTED)
		}
		defer C.free(unsafe.Pointer(module))
		return read(module)
	})
}

func programReadBytes(ctx unsafe.Pointer, key []byte) ([]byte, error) {
	var value *C.uint8_t
	var length C.size_t
	if err := result(C.lxphost_program_bytes(ctx, bytesPtr(key), C.size_t(len(key)), &value, &length)); err != nil {
		return nil, err
	}
	if length > C.LXP_MODULE_MAX_VALUE_BYTES {
		return nil, codeFatalInvariant
	}
	return C.GoBytes(unsafe.Pointer(value), C.int(length)), nil
}

func programKey(prefix string, id [32]byte) []byte {
	return append([]byte(prefix), id[:]...)
}

func (h *Host) ReadProgram(ctx sdk.Context, id [32]byte) (ProgramRecord, error) {
	var out ProgramRecord
	err := h.withProgramRead(ctx, func(module unsafe.Pointer) error {
		if id == ([32]byte{}) {
			return codeNonCanonical
		}
		record, err := programReadBytes(module, programKey("program\x00", id))
		if err != nil {
			return err
		}
		if len(record) != 71 || record[0] > 1 {
			return codeFatalInvariant
		}
		out.Policy = record[0]
		copy(out.Authority[:], record[1:33])
		copy(out.CodeHash[:], record[33:65])
		out.ABIVersion = binary.BigEndian.Uint16(record[65:67])
		out.Version = binary.BigEndian.Uint32(record[67:71])
		if (out.Policy == 0) != (out.Authority == ([32]byte{})) || out.Version == 0 || out.ABIVersion == 0 {
			return codeFatalInvariant
		}
		status := C.lxphost_program_active(module, bytes32(&id))
		if status != C.LXP_OK && status != C.LXP_ERR_PROGRAM_REFUSED {
			return result(status)
		}
		out.Active = status == C.LXP_OK
		return nil
	})
	if err != nil {
		return ProgramRecord{}, err
	}
	return out, nil
}

func (h *Host) ReadProgramInterface(ctx sdk.Context, id [32]byte) (ProgramInterface, error) {
	var out ProgramInterface
	err := h.withProgramRead(ctx, func(module unsafe.Pointer) error {
		if id == ([32]byte{}) {
			return codeNonCanonical
		}
		record, err := programReadBytes(module, programKey("interface\x00", id))
		if err != nil {
			return err
		}
		if len(record) < 72 || len(record) > 72+952 || string(record[:32]) != string(id[:]) ||
			int(binary.BigEndian.Uint32(record[68:72])) != len(record)-72 {
			return codeFatalInvariant
		}
		out.Version = binary.BigEndian.Uint32(record[32:36])
		copy(out.Digest[:], record[36:68])
		out.Encoding = record[72:]
		if out.Version == 0 || len(out.Encoding) == 0 || sha256.Sum256(out.Encoding) != out.Digest {
			return codeFatalInvariant
		}
		return nil
	})
	if err != nil {
		return ProgramInterface{}, err
	}
	return out, nil
}

func (h *Host) ReadProgramStateHead(ctx sdk.Context, id [32]byte) ([]byte, error) {
	var out []byte
	err := h.withProgramRead(ctx, func(module unsafe.Pointer) error {
		if id == ([32]byte{}) {
			return codeNonCanonical
		}
		var err error
		out, err = programReadBytes(module, append(programKey("progstor", id), 1))
		if err != nil {
			return err
		}
		if len(out) != 38 || binary.BigEndian.Uint16(out[:2]) != 1 {
			return codeFatalInvariant
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func readProgramAccount(binding *C.lx_programs_account_binding) ProgramAccount {
	return ProgramAccount{
		RecordVersion:           uint8(binding.record_version),
		ProgramID:               *(*[32]byte)(unsafe.Pointer(&binding.program_id[0])),
		AccountID:               *(*[32]byte)(unsafe.Pointer(&binding.account_id[0])),
		AssetID:                 *(*[32]byte)(unsafe.Pointer(&binding.asset_id[0])),
		Seed:                    C.GoBytes(unsafe.Pointer(&binding.seed[0]), C.int(binding.seed_length)),
		RegisteredSequence:      uint64(binding.registered_sequence),
		RegistrationEventDigest: *(*[32]byte)(unsafe.Pointer(&binding.registration_event_digest[0])),
	}
}

func (h *Host) ReadProgramAccount(ctx sdk.Context, programID, assetID [32]byte, seed []byte) (ProgramAccount, error) {
	var out ProgramAccount
	err := h.withProgramRead(ctx, func(module unsafe.Pointer) error {
		var binding C.lx_programs_account_binding
		var account C.lx_account
		if err := result(C.lxphost_program_account(module, bytes32(&programID), bytesPtr(seed), C.size_t(len(seed)), nil, &binding, &account)); err != nil {
			return err
		}
		out = readProgramAccount(&binding)
		if out.ProgramID != programID || string(out.Seed) != string(seed) {
			return Code(C.LXP_ERR_CONTEXT_MISMATCH)
		}
		if out.AssetID != assetID {
			return Code(C.LXP_ERR_ASSET_MISMATCH)
		}
		return nil
	})
	if err != nil {
		return ProgramAccount{}, err
	}
	return out, nil
}

func (h *Host) ReadProgramAccountByID(ctx sdk.Context, id [32]byte) (ProgramAccount, error) {
	var out ProgramAccount
	err := h.withProgramRead(ctx, func(module unsafe.Pointer) error {
		var binding C.lx_programs_account_binding
		var account C.lx_account
		if err := result(C.lxphost_program_account(module, nil, nil, 0, bytes32(&id), &binding, &account)); err != nil {
			return err
		}
		out = readProgramAccount(&binding)
		return nil
	})
	if err != nil {
		return ProgramAccount{}, err
	}
	return out, nil
}

func (h *Host) ReadAccountProfile(ctx sdk.Context, id [32]byte) (AccountProfile, error) {
	var out AccountProfile
	err := h.withProgramRead(ctx, func(module unsafe.Pointer) error {
		var profile [33]C.uint8_t
		if err := result(C.lxphost_program_profile(module, bytes32(&id), &profile[0])); err != nil {
			return err
		}
		out.Profile = uint8(profile[0])
		out.Principal = *(*[32]byte)(unsafe.Pointer(&profile[1]))
		return nil
	})
	if err != nil {
		return AccountProfile{}, err
	}
	return out, nil
}

func (h *Host) ReadValueAccount(ctx sdk.Context, programID, accountID [32]byte) (ValueAccount, error) {
	out := ValueAccount{Balance: sdk.ZeroInt()}
	err := h.withProgramRead(ctx, func(module unsafe.Pointer) error {
		var binding C.lx_programs_account_binding
		var account C.lx_account
		if err := result(C.lxphost_program_account(module, nil, nil, 0, bytes32(&accountID), &binding, &account)); err != nil {
			return err
		}
		if *(*[32]byte)(unsafe.Pointer(&binding.program_id[0])) != programID {
			return Code(C.LXP_ERR_CONTEXT_MISMATCH)
		}
		if account.kind != C.LX_ACCOUNT_MODULE_VALUE || !bool(account.has_asset) ||
			*(*[32]byte)(unsafe.Pointer(&account.asset_id[0])) != *(*[32]byte)(unsafe.Pointer(&binding.asset_id[0])) {
			return codeFatalInvariant
		}
		out.Balance = readAmount(account.balance)
		out.Frozen = bool(account.frozen)
		return nil
	})
	if err != nil {
		return ValueAccount{Balance: sdk.ZeroInt()}, err
	}
	return out, nil
}

func (h *Host) ReadWindDown(ctx sdk.Context, id [32]byte) (WindDown, error) {
	var out WindDown
	err := h.withProgramRead(ctx, func(module unsafe.Pointer) error {
		var view C.lx_programs_wind_down_view
		if err := result(C.lxphost_program_wind_down(module, bytes32(&id), &view)); err != nil {
			return err
		}
		out = WindDown{ProgramID: id, Status: uint8(view.status),
			ExitProgram: *(*[32]byte)(unsafe.Pointer(&view.exit_program[0])),
			Deadline:    uint64(view.deadline), EffectiveSequence: uint64(view.effective_sequence),
			ValueAccountCount: uint16(view.value_account_count), LiveValueAccountCount: uint16(view.live_value_account_count)}
		return nil
	})
	if err != nil {
		return WindDown{}, err
	}
	return out, nil
}

func (h *Host) ReadExitRoute(ctx sdk.Context, programID, accountID [32]byte) (ExitRoute, error) {
	var out ExitRoute
	err := h.withProgramRead(ctx, func(module unsafe.Pointer) error {
		var route C.lx_programs_exit_route_view
		if err := result(C.lxphost_program_exit_route(module, bytes32(&programID), bytes32(&accountID), &route)); err != nil {
			return err
		}
		out = ExitRoute{ProgramID: programID, AccountID: accountID,
			AssetID:     *(*[32]byte)(unsafe.Pointer(&route.asset_id[0])),
			Destination: *(*[32]byte)(unsafe.Pointer(&route.destination[0])),
			Seed:        C.GoBytes(unsafe.Pointer(&route.seed[0]), C.int(route.seed_length))}
		return nil
	})
	if err != nil {
		return ExitRoute{}, err
	}
	return out, nil
}

func (h *Host) ReadSandbox(ctx sdk.Context, leaseID [32]byte, kind uint8) ([]byte, error) {
	var out []byte
	err := h.withProgramRead(ctx, func(module unsafe.Pointer) error {
		if kind > 2 {
			return codeNonCanonical
		}
		var err error
		out, err = programReadBytes(module, append([]byte{'s', kind}, leaseID[:]...))
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (h *Host) ReadRetirementControl(ctx sdk.Context) ([]byte, error) {
	var out []byte
	err := h.withProgramRead(ctx, func(module unsafe.Pointer) error {
		var err error
		out, err = programReadBytes(module, []byte("progretire/v1"))
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
