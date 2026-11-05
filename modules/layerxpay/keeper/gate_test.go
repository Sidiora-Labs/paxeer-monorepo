package keeper

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/stretchr/testify/require"
)

func payGateMethods(t *testing.T, views bool) []string {
	t.Helper()
	raw, err := os.ReadFile("../../../precompiles/layerxpay/abi.json")
	require.NoError(t, err)
	var entries []struct {
		Type            string `json:"type"`
		Name            string `json:"name"`
		StateMutability string `json:"stateMutability"`
	}
	require.NoError(t, json.Unmarshal(raw, &entries))
	var methods []string
	for _, entry := range entries {
		if entry.Type == "function" && (entry.StateMutability == "view" || entry.StateMutability == "pure") == views {
			methods = append(methods, entry.Name)
		}
	}
	require.NotEmpty(t, methods)
	return methods
}

func payGateEnabledRecord() []byte {
	flag := make([]byte, 32)
	flag[31] = 1
	return flag
}

func TestCheckWritePauseExitPaths(t *testing.T) {
	h := newPayHarness(t)
	params := types.Params{Paused: true, EscrowEnabled: true, BudgetEnabled: true, StreamEnabled: true, ServiceEnabled: true}
	h.keeper.SetParams(h.ctx, params)
	for _, moduleID := range []uint16{types.EscrowModuleID, types.BudgetModuleID, types.StreamModuleID, types.ServiceModuleID} {
		key, err := types.KernelModuleEnableKey(moduleID)
		require.NoError(t, err)
		h.ctx.KVStore(h.hostKey).Set(key, payGateEnabledRecord())
	}
	exits := map[string]bool{
		"escrowRelease": true, "escrowTimeout": true,
		"budgetClose": true, "budgetDefund": true, "budgetRevoke": true,
		"streamPause": true, "streamClose": true,
		"serviceOfferWithdraw": true, "serviceCommitAbandon": true,
	}
	require.Len(t, exits, 9)
	seenExits := 0
	for _, method := range payGateMethods(t, false) {
		t.Run(method, func(t *testing.T) {
			require.Contains(t, types.MethodActivity, method)
			moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
			hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
			err := h.keeper.CheckWrite(h.ctx, method)
			if exits[method] {
				seenExits++
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, govtypes.ErrPausedScope)
				require.Contains(t, err.Error(), "LXP_PAUSED_SCOPE")
				require.EqualError(t, err, govtypes.CheckPauseScope(types.ModuleName, true, method).Error())
			}
			require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
			require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
		})
	}
	require.Equal(t, 9, seenExits)
	key, err := types.KernelModuleEnableKey(types.EscrowModuleID)
	require.NoError(t, err)
	h.ctx.KVStore(h.hostKey).Delete(key)
	require.ErrorIs(t, h.keeper.CheckWrite(h.ctx, "escrowOpen"), govtypes.ErrPausedScope)
	h.ctx.KVStore(h.hostKey).Set(key, payGateEnabledRecord())
	for method := range exits {
		t.Run(method+"/kernel_still_required", func(t *testing.T) {
			key, err := types.KernelModuleEnableKey(types.MethodActivity[method].Module)
			require.NoError(t, err)
			h.ctx.KVStore(h.hostKey).Delete(key)
			err = h.keeper.CheckWrite(h.ctx, method)
			require.ErrorIs(t, err, types.ErrKernelGateMismatch)
			require.Contains(t, err.Error(), method)
			h.ctx.KVStore(h.hostKey).Set(key, payGateEnabledRecord())
		})
	}
	params.Paused = false
	h.keeper.SetParams(h.ctx, params)
	for _, method := range payGateMethods(t, false) {
		require.NoError(t, h.keeper.CheckWrite(h.ctx, method), method)
	}
}

func TestCheckWriteFamilyDisabled(t *testing.T) {
	methods := payGateMethods(t, false)
	for _, family := range []struct {
		name     string
		moduleID uint16
		enabled  types.Params
		disabled types.Params
	}{
		{"escrow", types.EscrowModuleID, types.Params{EscrowEnabled: true}, types.Params{BudgetEnabled: true, StreamEnabled: true, ServiceEnabled: true}},
		{"budget", types.BudgetModuleID, types.Params{BudgetEnabled: true}, types.Params{EscrowEnabled: true, StreamEnabled: true, ServiceEnabled: true}},
		{"stream", types.StreamModuleID, types.Params{StreamEnabled: true}, types.Params{EscrowEnabled: true, BudgetEnabled: true, ServiceEnabled: true}},
		{"service", types.ServiceModuleID, types.Params{ServiceEnabled: true}, types.Params{EscrowEnabled: true, BudgetEnabled: true, StreamEnabled: true}},
	} {
		t.Run(family.name, func(t *testing.T) {
			h := newPayHarness(t)
			key, err := types.KernelModuleEnableKey(family.moduleID)
			require.NoError(t, err)
			for _, otherID := range []uint16{types.EscrowModuleID, types.BudgetModuleID, types.StreamModuleID, types.ServiceModuleID} {
				otherKey, err := types.KernelModuleEnableKey(otherID)
				require.NoError(t, err)
				h.ctx.KVStore(h.hostKey).Set(otherKey, payGateEnabledRecord())
			}
			h.ctx.KVStore(h.storeKey).Set(key, payGateEnabledRecord())
			for _, method := range methods {
				if !strings.HasPrefix(method, family.name) {
					continue
				}
				t.Run(method, func(t *testing.T) {
					require.Contains(t, types.MethodActivity, method)
					require.Equal(t, family.moduleID, types.MethodActivity[method].Module)
					for _, paused := range []bool{false, true} {
						params := family.disabled
						params.Paused = paused
						h.keeper.SetParams(h.ctx, params)
						for _, present := range []bool{false, true} {
							if present {
								h.ctx.KVStore(h.hostKey).Set(key, payGateEnabledRecord())
							} else {
								h.ctx.KVStore(h.hostKey).Delete(key)
							}
							err := h.keeper.CheckWrite(h.ctx, method)
							require.ErrorIs(t, err, types.ErrFamilyDisabled)
							require.Contains(t, err.Error(), method)
						}
					}
					h.keeper.SetParams(h.ctx, family.enabled)
					nonCanonical := payGateEnabledRecord()
					nonCanonical[0] = 1
					for _, record := range []struct {
						name  string
						value []byte
					}{
						{"absent", nil}, {"zero", make([]byte, 32)},
						{"short", []byte{1}}, {"long", append(payGateEnabledRecord(), 0)},
						{"invalid_boolean", append(make([]byte, 31), 2)}, {"noncanonical", nonCanonical},
						{"enabled", payGateEnabledRecord()},
					} {
						t.Run(record.name, func(t *testing.T) {
							if record.value == nil {
								h.ctx.KVStore(h.hostKey).Delete(key)
							} else {
								h.ctx.KVStore(h.hostKey).Set(key, record.value)
							}
							moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
							hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
							err := h.keeper.CheckWrite(h.ctx, method)
							if record.name == "enabled" {
								require.NoError(t, err)
							} else {
								require.ErrorIs(t, err, types.ErrKernelGateMismatch)
								require.Contains(t, err.Error(), method)
							}
							require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
							require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
						})
					}
				})
			}
		})
	}
}

func TestCheckWriteViewsUngated(t *testing.T) {
	h := newPayHarness(t)
	views := payGateMethods(t, true)
	require.Len(t, views, 17)
	for _, view := range views {
		require.NotContains(t, types.MethodActivity, view)
	}
	for mask := 0; mask < 32; mask++ {
		t.Run(fmt.Sprintf("params_%02d", mask), func(t *testing.T) {
			h.keeper.SetParams(h.ctx, types.Params{
				Paused: mask&1 != 0, EscrowEnabled: mask&2 != 0, BudgetEnabled: mask&4 != 0,
				StreamEnabled: mask&8 != 0, ServiceEnabled: mask&16 != 0,
			})
			for _, record := range [][]byte{nil, make([]byte, 32), payGateEnabledRecord()} {
				for _, moduleID := range []uint16{types.EscrowModuleID, types.BudgetModuleID, types.StreamModuleID, types.ServiceModuleID} {
					key, err := types.KernelModuleEnableKey(moduleID)
					require.NoError(t, err)
					if record == nil {
						h.ctx.KVStore(h.hostKey).Delete(key)
					} else {
						h.ctx.KVStore(h.hostKey).Set(key, record)
					}
				}
				moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
				hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
				for _, view := range views {
					require.NoError(t, h.keeper.CheckWrite(h.ctx, view), view)
				}
				require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
				require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
			}
		})
	}
}
