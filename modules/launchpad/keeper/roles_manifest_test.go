package keeper

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"github.com/ethereum/go-ethereum/common"
)

func TestGeneration2RolesMatchManifest(t *testing.T) {
	const manifestPath = "../../../contracts/launchpad/deployments/paxeer-x-network-addresses.json"
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read deployment manifest: %v", err)
	}
	var manifest map[string]json.RawMessage
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode deployment manifest: %v", err)
	}
	manifestAddress := func(t *testing.T, key string) common.Address {
		t.Helper()
		raw, ok := manifest[key]
		if !ok {
			t.Fatalf("deployment manifest is missing %q", key)
		}
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			t.Fatalf("decode manifest address %q: %v", key, err)
		}
		if !common.IsHexAddress(value) {
			t.Fatalf("manifest %q is not a valid address: %q", key, value)
		}
		address := common.HexToAddress(value)
		if address == (common.Address{}) {
			t.Fatalf("manifest %q is the zero address", key)
		}
		return address
	}

	surfaceRoles := map[string]common.Address{
		types.RoleFactory:        generation2Roles.SidioraFactory,
		types.RoleRouter:         generation2Roles.Router,
		types.RoleQuoter:         generation2Roles.Quoter,
		types.RoleFeesRouter:     generation2Roles.FeesRouter,
		types.RoleFeeAccumulator: generation2Roles.FeeAccumulator,
		types.RolePoolRegistry:   generation2Roles.PoolRegistry,
		types.RoleProtocolConfig: generation2Roles.ProtocolConfig,
		types.RoleFeeRights:      generation2Roles.SidioraNFT,
		types.RoleTreasury:       generation2Roles.Treasury,
	}
	if len(Generation2Surfaces) != len(surfaceRoles) {
		t.Errorf("Generation2Surfaces has %d roles, want %d", len(Generation2Surfaces), len(surfaceRoles))
	}
	for role, compiled := range surfaceRoles {
		t.Run(role, func(t *testing.T) {
			key := role + "_proxy"
			want := manifestAddress(t, key)
			surface, ok := Generation2Surfaces[role]
			if !ok {
				t.Errorf("Generation2Surfaces is missing role %q", role)
			} else if surface != want {
				t.Errorf("Generation2Surfaces[%q] = %s, manifest %s = %s", role, surface.Hex(), key, want.Hex())
			}
			if compiled != want {
				t.Errorf("generation2Roles %s = %s, manifest %s = %s", role, compiled.Hex(), key, want.Hex())
			}
		})
	}

	for key, compiled := range map[string]common.Address{
		types.RoleOpticalRegistry + "_proxy": generation2Roles.OpticalRegistry,
		"EventEmitter_proxy":                 generation2Roles.EventEmitter,
		"PoolBeacon":                         generation2Roles.PoolBeacon,
	} {
		t.Run(key, func(t *testing.T) {
			want := manifestAddress(t, key)
			if compiled != want {
				t.Errorf("generation2Roles %s = %s, manifest = %s", key, compiled.Hex(), want.Hex())
			}
		})
	}
}
