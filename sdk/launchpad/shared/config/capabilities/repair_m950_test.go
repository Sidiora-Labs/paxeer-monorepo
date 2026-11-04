package capabilities

import (
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
)

func TestRepairM_950_HistoricalSolidityManifest(t *testing.T) {
	manifest := historicalPreflightManifest(t)
	approved, ok := deployment.Approved(32_226_215)
	if !ok || manifest.ActivationHeight != 32_226_215 || manifest.DeploymentVersion == deployment.Current().DeploymentVersion {
		t.Fatal("Solidity fixture did not select the approved historical epoch")
	}
	for name, role := range approved.Roles {
		if manifest.Roles[name] != role {
			t.Fatalf("historical fixture rewrote approved %s", name)
		}
	}
	c := repair38OpenChain(t, false)
	if want, err := manifest.Hash(); err != nil {
		t.Fatal(err)
	} else if got, err := c.engine.manifest.Hash(); err != nil || got != want {
		t.Fatalf("historical producer uses another deployment: %s want %s: %v", got, want, err)
	}
	block, pool := c.peer.blocks[1], c.peer.epoch.Pool
	for _, family := range []Family{PlainQuote, Trading} {
		request := c.request(family, block)
		request.Pool = pool
		produced, err := c.engine.Preflight(c.ctx, request)
		if err != nil || !produced.Supported || produced.Pool == nil || produced.Quote == nil || produced.Pool.ImplementationVersion != manifest.Roles["PoolBeacon"].ImplementationHash {
			t.Fatalf("historical beacon producer refused %s: %+v %v", family, produced, err)
		}
		consumed := c.connected(request)
		if !consumed.Supported || !consumed.Pool.Present || consumed.Pool.Value == nil || consumed.Pool.Value.ImplementationVersion != produced.Pool.ImplementationVersion || !consumed.Quote.Present || consumed.Quote.Value == nil || consumed.Quote.Value.AmountOut != produced.Quote.AmountOut {
			t.Fatalf("historical %s was lost at the HTTP consumer: %+v", family, consumed)
		}
	}
	c.peer.mu.Lock()
	defer c.peer.mu.Unlock()
	if c.peer.seen["storage/"+pool.Hex()+"/"+deployment.BeaconSlot.Hex()] == 0 {
		t.Fatal("historical pool was not verified through its actual beacon slot")
	}
}

func TestRepairM_950_HistoricalDrift(t *testing.T) {
	assertRecordedPreflightDrift(t, false)
}

func TestRepairM_950_CurrentNativeDrift(t *testing.T) {
	assertCurrentNativePreflightDrift(t)
}

func TestRepairM_950_CurrentNativeCreation(t *testing.T) {
	c := repair38OpenChain(t, true)
	current := deployment.Current()
	for _, path := range []string{"Router", "SidioraFactory"} {
		t.Run(path, func(t *testing.T) {
			child := *c
			child.t = t
			request := child.request(Creation, c.peer.blocks[1])
			request.CreationPath = path
			produced, err := c.engine.Preflight(c.ctx, request)
			if err != nil || !produced.Supported || produced.CreationCost == nil {
				t.Fatalf("native %s creation producer refused: %+v %v", path, produced, err)
			}
			cost := produced.CreationCost
			if current.Roles[path].Kind != "surface" || cost.Spender != deployment.LaunchpadPrecompileAddress || cost.ProtocolFeeRaw != deployment.CreationFeeRaw || !cost.GasEstimateAvailable || cost.EstimatedGasUnits != 200_000 || cost.EstimatedGasRaw != "200000000000000" {
				t.Fatalf("native creation did not use the real surface and dispatcher payment authority: %+v", cost)
			}
			consumed := child.connected(request)
			if !consumed.Supported || !consumed.CreationCost.Present || consumed.CreationCost.Value == nil || consumed.CreationCost.Value.Spender != cost.Spender || consumed.CreationCost.Value.ProtocolFeeRaw != cost.ProtocolFeeRaw || consumed.CreationCost.Value.EstimatedGasRaw != cost.EstimatedGasRaw {
				t.Fatalf("native payment disclosure was lost at the HTTP consumer: %+v", consumed)
			}
		})
	}
}
