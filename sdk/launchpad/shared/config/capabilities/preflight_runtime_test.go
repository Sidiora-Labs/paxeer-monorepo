package capabilities

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestPreflightPoolSurfaceRuntimeHashes(t *testing.T) {
	c := preflightFixtureOpenChain(t, true)
	pool, block := c.peer.epoch.Pool, c.peer.blocks[1]
	factory := c.engine.manifest.Roles["SidioraFactory"]
	current := append([]byte(nil), c.peer.codes[pool]...)
	staged := common.FromHex("0x3415600957600080fd5b363d3d373360601b36523d3d601436013d3d6110175af13d600060003e602e573d6000fd5b3d6000f3")
	if crypto.Keccak256Hash(current).Hex() != factory.RuntimeHash || crypto.Keccak256Hash(staged).Hex() != factory.StagedRuntimeHash {
		t.Fatal("surface bytecode must match both manifest runtime pins")
	}
	for _, reviewed := range []string{factory.RuntimeHash, factory.StagedRuntimeHash} {
		t.Run(reviewed, func(t *testing.T) {
			engine, err := NewPreflightEngine(c.reader, c.engine.manifest, reviewed, strings.Repeat("a", 64), time.Minute, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			revision := engine.Revision()
			configured, err := NewPreflightRevision(c.engine.manifest, reviewed, strings.Repeat("a", 64))
			if err != nil || configured != revision {
				t.Fatalf("configured revision differs from engine: %+v %v", configured, err)
			}
			for _, test := range []struct {
				name     string
				code     []byte
				accepted bool
			}{
				{"runtime", current, true},
				{"staged runtime", staged, true},
				{"unaccepted runtime", common.FromHex("0x6000"), false},
				{"empty runtime", nil, false},
				{"oversized runtime", make([]byte, (128<<10)+1), false},
			} {
				t.Run(test.name, func(t *testing.T) {
					c.peer.mu.Lock()
					c.peer.codes[pool] = test.code
					c.peer.mu.Unlock()
					state, err := engine.poolState(c.ctx, pool, block)
					if test.accepted {
						if err != nil || state.Address != pool.Hex() || state.ImplementationVersion != engine.manifest.DeploymentVersion+"/Pool/"+crypto.Keccak256Hash(test.code).Hex() {
							t.Fatalf("accepted surface state: %+v %v", state, err)
						}
					} else if reason, issue, ok := Refused(err); !ok || reason != IdentityUnverified || issue.Role != "Pool" || state != (PoolState{}) {
						t.Fatalf("unaccepted surface identity: %+v %v", state, err)
					}
					if engine.Revision() != revision || engine.poolRuntime != common.HexToHash(reviewed) {
						t.Fatal("pool observation changed reviewed configuration")
					}
				})
			}
		})
	}
}

func TestPreflightPoolHistoricalRuntimeHash(t *testing.T) {
	c := preflightFixtureOpenChain(t, false)
	pool, block := c.peer.epoch.Pool, c.peer.blocks[1]
	if c.engine.manifest.Roles["SidioraFactory"].Kind == "surface" {
		t.Fatal("historical manifest must retain proxy identity")
	}
	revision := c.engine.Revision()
	state, err := c.engine.poolState(c.ctx, pool, block)
	if err != nil || state.ImplementationVersion != c.engine.manifest.Roles["PoolBeacon"].ImplementationHash {
		t.Fatalf("reviewed historical runtime refused: %+v %v", state, err)
	}
	c.peer.mu.Lock()
	c.peer.codes[pool] = c.peer.codes[c.peer.address("SidioraFactory")]
	c.peer.mu.Unlock()
	if _, err := c.engine.poolState(c.ctx, pool, block); err == nil {
		t.Fatal("historical pool accepted factory runtime instead of reviewed pool runtime")
	} else if reason, _, ok := Refused(err); !ok || reason != IdentityUnverified {
		t.Fatalf("historical runtime refusal: %v", err)
	}
	if c.engine.Revision() != revision {
		t.Fatal("historical pool observation changed revision")
	}
}

func TestPreflightPoolRuntimeReadFailure(t *testing.T) {
	c := preflightFixtureOpenChain(t, true)
	ctx, cancel := context.WithCancel(c.ctx)
	cancel()
	state, err := c.engine.poolState(ctx, c.peer.epoch.Pool, c.peer.blocks[1])
	var issue deployment.Issue
	if !errors.As(err, &issue) || issue.Code != "pool_read_failed" || issue.Role != "Pool" || state != (PoolState{}) {
		t.Fatalf("pool runtime read failure: %+v %v", state, err)
	}
	if _, _, refused := Refused(err); refused {
		t.Fatal("pool read failure became an identity refusal")
	}
}

func TestPreflightPoolStagedRuntimeRequiresUpgrade(t *testing.T) {
	c := preflightFixtureOpenChain(t, true)
	manifest := deployment.Current()
	factory := manifest.Roles["SidioraFactory"]
	factory.StagedUpgrade = ""
	manifest.Roles["SidioraFactory"] = factory
	if factory.AcceptsRuntimeHash(factory.StagedRuntimeHash) || !factory.AcceptsRuntimeHash(factory.RuntimeHash) {
		t.Fatal("staged runtime acceptance must require an upgrade")
	}
	for _, reviewed := range []string{factory.RuntimeHash, factory.StagedRuntimeHash} {
		if _, err := NewPreflightEngine(c.reader, manifest, reviewed, strings.Repeat("a", 64), time.Minute, time.Second); err == nil {
			t.Fatal("manifest with an unqualified staged runtime admitted")
		}
		if _, err := NewPreflightRevision(manifest, reviewed, strings.Repeat("a", 64)); err == nil {
			t.Fatal("manifest with an unqualified staged runtime received a revision")
		}
	}
}

func TestPreflightPoolReviewedRuntimeRequired(t *testing.T) {
	c := preflightFixtureOpenChain(t, true)
	for _, runtime := range []string{"", "invalid", strings.Repeat("a", 64), "0x" + strings.Repeat("0", 64), "0x" + strings.Repeat("z", 64)} {
		t.Run(runtime, func(t *testing.T) {
			if _, err := NewPreflightEngine(c.reader, c.engine.manifest, runtime, strings.Repeat("a", 64), time.Minute, time.Second); err == nil {
				t.Fatal("missing or malformed reviewed pool runtime admitted")
			}
			if _, err := NewPreflightRevision(c.engine.manifest, runtime, strings.Repeat("a", 64)); err == nil {
				t.Fatal("missing or malformed reviewed pool runtime received a revision")
			}
		})
	}
}
