package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmhttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	paxdcmd "github.com/Sidiora-Labs/Paxeer-X-Network/daemon/paxd/cmd"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	seqmodule "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/params"
	seqconfig "github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	servercmd "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/cmd"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/version"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	storekeys "github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func TestMain(m *testing.M) {
	if os.Getenv("COMMITTED_FIXTURE_EXEC_PAXD") == "1" {
		revision, err := exec.Command("git", "rev-parse", "HEAD").Output()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		version.Commit = strings.TrimSpace(string(revision))
		params.SetAddressPrefixes()
		root, _ := paxdcmd.NewRootCmd()
		root.SetArgs(os.Args[1:])
		if err := servercmd.Execute(root, ""); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestRecorderV6GenesisSelection(t *testing.T) {
	t.Run("V1 programs deploy encoding", assertRecorderV1ProgramsDeployEncoding)
	t.Run("V1 programs revert reason", assertRecorderV1ProgramsRevertReason)
	paxd, err := os.Executable()
	require.NoError(t, err)
	t.Setenv("COMMITTED_FIXTURE_EXEC_PAXD", "1")
	params.SetAddressPrefixes()
	contracts, _, _, err := v6LoadContracts()
	require.NoError(t, err)
	require.NotEmpty(t, contracts.Token.RuntimeCode)
	encoding := app.MakeEncodingConfig()
	for _, selection := range []struct {
		name   string
		filter string
		token  bool
	}{
		{"all", "", true},
		{"V6", vectorV6Name, true},
		{"unrelated", v1Name, false},
	} {
		t.Run(selection.name, func(t *testing.T) {
			if selection.filter != "" {
				require.NotNil(t, vectors[selection.filter])
			}
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			n, err := prepareHome(ctx, paxd)
			require.NoError(t, err)
			t.Cleanup(func() { os.RemoveAll(n.home) })
			public, actor, err := n.v1ProgramsGenesisIdentity()
			require.NoError(t, err)
			require.NotEqual(t, [32]byte{}, actor)
			require.Equal(t, public, [32]byte(n.programsAuthority.did.Public().(ed25519.PublicKey)))
			authority, err := lxphost.DIDID([]byte(evmtypes.LayerXDid(public)))
			require.NoError(t, err)
			require.Equal(t, authority, actor, "genesis must name the generated authority DID")
			account, err := n.command("keys", "show", "fixture", "--keyring-backend", "test", "--output", "json")
			require.NoError(t, err)
			var fixture struct {
				Address string `json:"address"`
			}
			require.NoError(t, json.Unmarshal(account, &fixture))
			payer, err := sdk.AccAddressFromBech32(fixture.Address)
			require.NoError(t, err)
			require.NotEqual(t, common.BytesToAddress(payer), crypto.PubkeyToAddress(n.programsAuthority.evm.PublicKey), "Programs must leave the fixture address for V4's first DID binding")
			path := filepath.Join(n.home, "config", "genesis.json")
			var original map[string]json.RawMessage
			require.NoError(t, readJSON(path, &original))
			var originalApp, originalEVM map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(original["app_state"], &originalApp))
			require.NoError(t, json.Unmarshal(originalApp["evm"], &originalEVM))
			var initial evmtypes.GenesisState
			require.NoError(t, encoding.Marshaler.UnmarshalAsJSON(originalApp["evm"], &initial))
			require.Empty(t, initial.Codes)
			unrelated := &evmtypes.Code{Address: common.HexToAddress("0x1234").Hex(), Code: contracts.Token.RuntimeCode}
			originalEVM["codes"], err = json.Marshal([]*evmtypes.Code{unrelated})
			require.NoError(t, err)
			originalApp["evm"], err = json.Marshal(originalEVM)
			require.NoError(t, err)
			original["app_state"], err = json.Marshal(originalApp)
			require.NoError(t, err)
			data, err := json.Marshal(original)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(path, data, 0600))

			standby, observer, err := n.prepareRecordingPeers(selection.filter)
			require.NoError(t, err)
			t.Cleanup(func() { os.RemoveAll(observer.home) })
			primary, err := os.ReadFile(path)
			require.NoError(t, err)
			for _, peer := range []*node{n, standby, observer} {
				data, err := os.ReadFile(filepath.Join(peer.home, "config", "genesis.json"))
				require.NoError(t, err)
				require.Equal(t, primary, data, "every peer must retain identical prepared genesis")
				require.Nil(t, peer.processDone)
				require.False(t, peer.bootstrapped)
				if peer.observer {
					var config struct {
						OCC  bool `toml:"occ-enabled"`
						Giga struct {
							Enabled bool `toml:"enabled"`
						} `toml:"giga_executor"`
					}
					_, err := toml.DecodeFile(filepath.Join(peer.home, "config", "app.toml"), &config)
					require.NoError(t, err)
					require.False(t, config.OCC, "observer must replay the sequential fixture synchronously")
					require.False(t, config.Giga.Enabled, "observer must replay through the recorder's SDK execution path")
				}
				for _, log := range []string{"paxd.log", "bootstrap.log"} {
					_, err := os.Stat(filepath.Join(peer.home, log))
					require.True(t, os.IsNotExist(err), "genesis must be prepared before startup")
				}
				var doc, state, evm map[string]json.RawMessage
				require.NoError(t, json.Unmarshal(data, &doc))
				require.JSONEq(t, string(original["chain_id"]), string(doc["chain_id"]))
				require.JSONEq(t, string(original["consensus_params"]), string(doc["consensus_params"]))
				require.NoError(t, json.Unmarshal(doc["app_state"], &state))
				assertRecorderProgramsGenesis(t, state[storekeys.LayerXHostStoreKey], public, actor)
				require.NoError(t, json.Unmarshal(state["evm"], &evm))
				for name, value := range originalEVM {
					if name != "codes" {
						require.JSONEq(t, string(value), string(evm[name]), "preserve EVM %s", name)
					}
				}
				var genesis evmtypes.GenesisState
				require.NoError(t, encoding.Marshaler.UnmarshalAsJSON(state["evm"], &genesis))
				require.NoError(t, genesis.Validate())
				tokens, others := 0, 0
				for _, code := range genesis.Codes {
					switch common.HexToAddress(code.Address) {
					case v6Token:
						tokens++
						require.Equal(t, []byte(contracts.Token.RuntimeCode), code.Code)
					case common.HexToAddress(unrelated.Address):
						others++
						require.Equal(t, unrelated.Code, code.Code)
					default:
						t.Fatalf("unexpected genesis code at %s", code.Address)
					}
				}
				require.Equal(t, 1, others, "preserve unrelated genesis code")
				wantTokens := 0
				if selection.token {
					wantTokens = 1
				}
				require.Equal(t, wantTokens, tokens)
				if selection.filter == "" {
					var sequencer struct {
						ActivationHeight uint64 `json:"activation_height"`
					}
					require.NoError(t, json.Unmarshal(state["sequencer"], &sequencer))
					require.Equal(t, uint64(3), sequencer.ActivationHeight)
					require.Equal(t, int64(2), n.bootstrapHeight)
					var gov govtypes.GenesisState
					require.NoError(t, encoding.Marshaler.UnmarshalAsJSON(state[govtypes.ModuleName], &gov))
					require.Len(t, gov.Proposals, 1)
					require.Equal(t, govtypes.StatusVotingPeriod, gov.Proposals[0].Status)
					proposal, ok := gov.Proposals[0].GetContent().(*upgradetypes.SoftwareUpgradeProposal)
					require.True(t, ok)
					require.Equal(t, app.MergeUpgrade, proposal.Plan.Name)
					require.Equal(t, int64(2), proposal.Plan.Height)
					require.JSONEq(t, string(state["sequencer"]), proposal.Plan.Info)
					require.Len(t, gov.Votes, 1)
					require.Equal(t, gov.Proposals[0].ProposalId, gov.Votes[0].ProposalId)
					require.Len(t, gov.Votes[0].Options, 1)
					require.Equal(t, govtypes.OptionYes, gov.Votes[0].Options[0].Option)
				}
			}
			if selection.token {
				require.EqualError(t, n.configureV6Genesis(), "V6 token address already has genesis code")
				after, err := os.ReadFile(path)
				require.NoError(t, err)
				require.Equal(t, primary, after, "duplicate token rejection must leave genesis intact")
			}
		})
	}
}

func assertRecorderProgramsGenesis(t *testing.T, data json.RawMessage, public, actor [32]byte) {
	t.Helper()
	var host lxphost.StoreGenesis
	require.NoError(t, json.Unmarshal(data, &host))
	require.Zero(t, host.LastSequence, "genesis schedules do not execute an activity")
	// Deploy needs both active and historical metering; calls also need the fee
	// schedule. All four module-9 records must precede the first peer block.
	require.Len(t, host.Records, 8)
	require.NoError(t, host.Validate())
	for i, record := range host.Records {
		if i > 0 {
			require.Less(t, bytes.Compare(host.Records[i-1].Key, record.Key), 0)
		}
		require.GreaterOrEqual(t, len(record.Key), 3)
		if record.Key[0] == 1 {
			require.Contains(t, []byte{1, 9}, record.Key[2])
			require.Zero(t, record.Key[1])
		} else {
			require.Equal(t, byte(6), record.Key[0])
		}
	}
	records := make(map[string][]byte)
	for _, record := range host.Records {
		if bytes.HasPrefix(record.Key, []byte{1, 0, 9}) {
			records[string(record.Key[3:])] = record.Value
		}
	}
	require.Len(t, records, 4, "retain every governance schedule")
	fee := records["progfee/active/v1"]
	require.Len(t, fee, 217)
	require.Equal(t, fee, records["progfee/history/v1/\x00\x00\x00\x01"])
	require.Equal(t, "LXFR1", string(fee[:5]))
	require.Equal(t, uint32(1), binary.BigEndian.Uint32(fee[5:9]))
	for offset := 9; offset < 65; offset += 8 {
		require.Equal(t, uint64(1), binary.BigEndian.Uint64(fee[offset:offset+8]))
	}
	material := append([]byte("LX:ASSET:v1"), actor[:]...)
	asset := sha256.Sum256(append(material, bytes.Repeat([]byte{0x55}, 32)...))
	require.Equal(t, asset[:], fee[65:97])
	for i, want := range []uint64{1000, 8, 1, 8, 1, 100, 1, 0, 1} {
		require.Equal(t, want, binary.BigEndian.Uint64(fee[97+i*8:105+i*8]))
	}
	require.Equal(t, actor[:], fee[169:201])
	require.Equal(t, make([]byte, 16), fee[201:])
	metering := records["progmet/active/v1"]
	require.Len(t, metering, 122)
	require.Equal(t, metering, records["progmet/history/v1/\x00\x00\x00\x01"])
	require.Equal(t, "LXMR1", string(metering[:5]))
	require.Equal(t, uint32(1), binary.BigEndian.Uint32(metering[5:9]))
	for i, want := range []uint64{1, 1, 1, 1, 1, 8, 8, 64, 8} {
		require.Equal(t, want, binary.BigEndian.Uint64(metering[9+i*8:17+i*8]))
	}
	require.Equal(t, uint64(1), binary.BigEndian.Uint64(metering[81:89]))
	require.Equal(t, byte(1), metering[89])
	require.Equal(t, actor[:], metering[90:])

	// Decode the actual genesis with the native host, including the fee asset
	// required at lxp_kernel_batch_snapshot_create before guest execution.
	key := sdk.NewKVStoreKey(storekeys.LayerXHostStoreKey)
	db := dbm.NewMemDB()
	defer db.Close()
	cms := store.NewCommitMultiStore(db)
	cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	require.NoError(t, cms.LoadLatestVersion())
	ctx := sdk.NewContext(cms, tmproto.Header{Height: 1, Time: time.UnixMilli(1)}, false)
	require.NoError(t, lxphost.InitStoreGenesis(ctx, key, host))
	native, err := app.NewHost(key, 125)
	require.NoError(t, err)
	defer native.Close()
	registered, err := native.ReadAsset(ctx, asset)
	require.NoError(t, err)
	require.Equal(t, actor, registered.IssuerDID)
	require.Equal(t, int64(1000000000), registered.SupplyCap.Int64())
	require.Equal(t, int64(500000000), registered.TotalUnits.Int64())
	require.False(t, registered.Paused)
	mainName, mainID, err := lxphost.MainAccountID(public)
	require.NoError(t, err)
	payerName, payerID, err := lxphost.AssetAccountID(public, asset)
	require.NoError(t, err)
	treasuryID, err := codec.DeriveAccountID([]byte("system:fees"))
	require.NoError(t, err)
	for _, want := range []struct {
		name      string
		id        [32]byte
		balance   int64
		kind      uint8
		authority bool
	}{
		{mainName, mainID, 100000000, 1, true},
		{payerName, payerID, 500000000, 14, true},
		{"system:fees", treasuryID, 0, 10, false},
	} {
		account, err := native.ReadAccount(ctx, want.name, want.id)
		require.NoError(t, err)
		require.Equal(t, want.balance, account.Balance.Int64())
		require.Equal(t, want.kind, account.Kind)
		accountAsset := asset
		if want.id == mainID {
			accountAsset = [32]byte{0x91}
		}
		require.Equal(t, accountAsset, account.AssetID)
		require.True(t, account.HasAsset)
		require.False(t, account.Frozen)
		require.Equal(t, want.authority, account.HasAuthorityKey)
		if want.authority {
			require.Equal(t, public, account.AuthorityKey)
		}
	}
}

func assertRecorderV1ProgramsRevertReason(t *testing.T) {
	stringType, err := abi.NewType("string", "", nil)
	require.NoError(t, err)
	reason := "LXP_ERR_VERSION_UNSUPPORTED: deploy: layerx kernel result -101"
	encoded, err := (abi.Arguments{{Type: stringType}}).Pack(reason)
	require.NoError(t, err)
	data := append(common.FromHex("0x08c379a0"), encoded...)
	recorded := v1ProgramsRevertError("deployment", data)
	require.Contains(t, recorded.Error(), "V1 Programs deployment reverted")
	require.Contains(t, recorded.Error(), reason)
	require.Contains(t, recorded.Error(), hex.EncodeToString(data))
	require.Contains(t, recorded.Error(), "ABI 1")
	require.Contains(t, recorded.Error(), "activity 0x00090001")
	call := v1ProgramsRevertError("call", data)
	require.Contains(t, call.Error(), "V1 Programs call reverted")
	require.Contains(t, call.Error(), reason)
	require.Contains(t, call.Error(), "activity 0x00090003")
	for _, malformed := range [][]byte{nil, data[:4], data[:68]} {
		recorded := v1ProgramsRevertError("deployment", malformed)
		require.Contains(t, recorded.Error(), "cannot decode revert data")
		require.NotNil(t, errors.Unwrap(recorded))
	}
}

func assertRecorderV1ProgramsDeployEncoding(t *testing.T) {
	principal := [32]byte{1}
	program, deploy, call, err := v1ProgramsRecipe(principal)
	require.NoError(t, err)
	method := v1ProgramsABI.Methods["deploy"]
	require.Equal(t, method.ID, deploy[:4])
	args, err := method.Inputs.Unpack(deploy[4:])
	require.NoError(t, err)
	require.Equal(t, [32]byte(program), args[0])
	require.Equal(t, uint16(1), args[1])
	require.Equal(t, uint8(1), args[2])
	require.Equal(t, principal, args[3])
	codeHash := args[4].([32]byte)
	iface, wasm := args[5].([]byte), args[6].([]byte)
	// Pin the import-free module and its interface: the fixture host has no
	// transfer accounts or assets for a transfer_402 import.
	require.Len(t, wasm, 88)
	require.Equal(t, "5af5abb220f57962dd60df53dbac84c1d06a7aa4154bcd3853dd62f2760f434a", hex.EncodeToString(codeHash[:]))
	require.Equal(t, sha256.Sum256(wasm), codeHash)
	require.Len(t, iface, 99)
	interfaceHash := sha256.Sum256(iface)
	require.Equal(t, "04f3fe02df679f83ec3d5556c9be235167411fdbecedb15a1427435f7b4e3f60", hex.EncodeToString(interfaceHash[:]))
	prefix := []byte("LayerX/program-interface/v1\x00")
	require.Equal(t, prefix, iface[:len(prefix)])
	require.Equal(t, codeHash[:], iface[len(prefix):len(prefix)+32])
	require.Equal(t, uint16(1), binary.BigEndian.Uint16(iface[len(prefix)+32:]))
	payload, err := lp.EncodeDeployPayload([32]byte(program), args[1].(uint16), args[2].(uint8), principal, codeHash, iface, wasm)
	require.NoError(t, err)
	require.Equal(t, uint16(1), binary.BigEndian.Uint16(payload[32:34]))
	require.Equal(t, uint32(len(wasm)), binary.BigEndian.Uint32(payload[100:104]))
	require.Equal(t, uint32(len(iface)), binary.BigEndian.Uint32(payload[104:108]))
	require.Equal(t, iface, payload[108:108+len(iface)])
	require.Equal(t, wasm, payload[108+len(iface):])
	callArgs, err := v1ProgramsABI.Methods["call"].Inputs.Unpack(call[4:])
	require.NoError(t, err)
	require.Equal(t, [32]byte(program), callArgs[0])
	require.Equal(t, uint16(1), callArgs[1])
	require.Equal(t, "layerx_call", callArgs[2])
}

func TestRecorderAllSelectedV6Executes(t *testing.T) {
	paxd, err := os.Executable()
	require.NoError(t, err)
	t.Setenv("COMMITTED_FIXTURE_EXEC_PAXD", "1")
	params.SetAddressPrefixes()
	require.NotNil(t, vectors[vectorV6Name])
	out := filepath.Join(t.TempDir(), "vectors")
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	var stderr bytes.Buffer
	err = run(ctx, []string{"record", "--paxd", paxd, "--out", out}, &stderr)
	require.NoError(t, err, "all registered recorders must finish: %s", stderr.String())
	for name := range vectors {
		data, err := os.ReadFile(filepath.Join(out, name+".json"))
		require.NoError(t, err, "selected recorder %s must emit its output", name)
		require.True(t, json.Valid(data), "recorder %s must emit JSON", name)
		var emitted struct {
			Name string `json:"name"`
		}
		require.NoError(t, json.Unmarshal(data, &emitted))
		require.Equal(t, name, emitted.Name)
	}
	var v v6Vector
	require.NoError(t, readJSON(filepath.Join(out, vectorV6Name+".json"), &v))
	require.NoError(t, verifyVectorV6(&v), "verify executed token repayment, DID binding, authorization and receipt signatures")
	require.Equal(t, "V6-sponsored-bind", v.Positive.ID)
	require.Equal(t, "V6-plain-transfer", v.Refusal.ID)
	require.Equal(t, evmtypes.LayerXDid([32]byte(v.DIDPublicKey)), v.BoundDID)
	for _, entry := range []*v6Case{&v.DelegateDeployment, &v.Positive, &v.Refusal} {
		require.NoError(t, entry.Receipt.Verify(v.ReceiptKeys))
		require.Equal(t, "committed", string(entry.Receipt.Rung))
		require.EqualValues(t, 1, entry.Receipt.Status)
	}
}

func TestRecorderHomePinsGeneratedReceiptKey(t *testing.T) {
	paxd, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("COMMITTED_FIXTURE_EXEC_PAXD", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	var previousKey []byte
	var previousHome string
	for run := 0; run < 2; run++ {
		n, err := prepareHome(ctx, paxd)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.RemoveAll(n.home) })
		if n.home == previousHome {
			t.Fatal("recorder reused its previous home")
		}
		previousHome = n.home
		info, err := os.Stat(n.home)
		if err != nil || info.Mode().Perm() != 0700 {
			t.Fatalf("temporary home must be private: %v", err)
		}
		keyInfo, err := os.Stat(n.receiptKeyFile)
		if err != nil || keyInfo.Mode().Perm() != 0600 {
			t.Fatalf("receipt key must have mode 0600: %v", err)
		}
		seedHex, err := os.ReadFile(n.receiptKeyFile)
		if err != nil {
			t.Fatal(err)
		}
		seed, err := hex.DecodeString(strings.TrimSpace(string(seedHex)))
		if err != nil || len(seed) != ed25519.SeedSize {
			t.Fatal("receipt key file must contain an ed25519 seed")
		}
		private := ed25519.NewKeyFromSeed(seed)
		public := private.Public().(ed25519.PublicKey)
		if bytes.Equal(public, previousKey) {
			t.Fatal("recorder reused the receipt key across homes")
		}
		previousKey = bytes.Clone(public)
		if !bytes.Equal(public, n.receiptPublicKey) {
			t.Fatal("generated public key differs from the configured signing seed")
		}
		var genesis struct {
			ChainID  string                     `json:"chain_id"`
			AppState map[string]json.RawMessage `json:"app_state"`
		}
		if err := readJSON(filepath.Join(n.home, "config", "genesis.json"), &genesis); err != nil {
			t.Fatal(err)
		}
		if genesis.ChainID != fixtureChainID {
			t.Fatalf("wrong fixture chain id: %s", genesis.ChainID)
		}
		var state struct {
			ReceiptKeys      seqkeeper.ReceiptKeys `json:"receipt_keys"`
			Leaders          [][]byte              `json:"leaders"`
			ActivationHeight uint64                `json:"activation_height"`
		}
		if err := json.Unmarshal(genesis.AppState["sequencer"], &state); err != nil {
			t.Fatal(err)
		}
		if len(state.ReceiptKeys) != 1 {
			t.Fatalf("want exactly one generated receipt pin, got %d", len(state.ReceiptKeys))
		}
		pin := state.ReceiptKeys[0]
		keyID := sha256.Sum256(public)
		if !bytes.Equal(pin.PublicKey, public) || !bytes.Equal(pin.KeyId, keyID[:]) {
			t.Fatal("genesis does not pin the generated receipt public key and its SHA-256 id")
		}
		if pin.FirstHeight != 1 || pin.LastHeight != uint64(1<<63-1) || state.ActivationHeight != 1 {
			t.Fatalf("unexpected receipt key height range [%d, %d] or activation %d", pin.FirstHeight, pin.LastHeight, state.ActivationHeight)
		}
		if err := (seqmodule.AppModuleBasic{}).ValidateGenesis(nil, nil, genesis.AppState["sequencer"]); err != nil {
			t.Fatalf("prepared sequencer genesis must be accepted by the real module: %v", err)
		}
		var consensus struct {
			PubKey struct {
				Value []byte `json:"value"`
			} `json:"pub_key"`
		}
		if err := readJSON(filepath.Join(n.home, "config", "priv_validator_key.json"), &consensus); err != nil {
			t.Fatal(err)
		}
		if len(state.Leaders) != 1 || !bytes.Equal(state.Leaders[0], consensus.PubKey.Value) {
			t.Fatal("genesis does not pin this node's consensus key as the sole leader")
		}
		if bytes.Equal(consensus.PubKey.Value, public) {
			t.Fatal("receipt and consensus signing keys must be distinct")
		}
		app := viper.New()
		app.SetConfigFile(filepath.Join(n.home, "config", "app.toml"))
		if err := app.ReadInConfig(); err != nil {
			t.Fatal(err)
		}
		cfg, err := seqconfig.ReadConfig(app)
		if err != nil {
			t.Fatalf("prepared sequencer configuration must be usable: %v", err)
		}
		if !cfg.Enable || cfg.ReceiptKeyFile != n.receiptKeyFile || cfg.WALDir != filepath.Join(n.home, "data", "sequencer-wal") {
			t.Fatalf("incorrect sequencer paths or enable flag: %+v", cfg)
		}
		if cfg.Role != seqconfig.RolePrimary || cfg.StandbyAddress == "" || cfg.ReplicationTimeout <= 0 {
			t.Fatalf("primary must have a standby and a positive replication timeout: %+v", cfg)
		}
		if cfg.ReplicationTimeout != 5*time.Second {
			t.Fatalf("replication timeout must be 5s, got %s", cfg.ReplicationTimeout)
		}
		if n.standby == nil || n.standby.home == n.home {
			t.Fatal("primary must have a distinct prepared standby home")
		}
		standbyApp := viper.New()
		standbyApp.SetConfigFile(filepath.Join(n.standby.home, "config", "app.toml"))
		if err := standbyApp.ReadInConfig(); err != nil {
			t.Fatal(err)
		}
		standbyCfg, err := seqconfig.ReadConfig(standbyApp)
		if err != nil {
			t.Fatalf("prepared standby configuration must be usable: %v", err)
		}
		if !standbyCfg.Enable || standbyCfg.Role != seqconfig.RoleStandby ||
			standbyCfg.StandbyAddress != cfg.StandbyAddress || standbyCfg.ReplicationTimeout != cfg.ReplicationTimeout ||
			standbyCfg.WALDir != filepath.Join(n.standby.home, "data", "sequencer-wal") ||
			standbyCfg.ReceiptKeyFile != n.standby.receiptKeyFile {
			t.Fatalf("standby must replicate with its own paths and matching address: %+v", standbyCfg)
		}
		standbySeedHex, err := os.ReadFile(standbyCfg.ReceiptKeyFile)
		if err != nil {
			t.Fatal(err)
		}
		standbySeed, err := hex.DecodeString(strings.TrimSpace(string(standbySeedHex)))
		if err != nil || len(standbySeed) != ed25519.SeedSize {
			t.Fatal("standby receipt key file must contain an ed25519 seed")
		}
		standbyPrivate := ed25519.NewKeyFromSeed(standbySeed)
		standbyPublic := standbyPrivate.Public().(ed25519.PublicKey)
		if !bytes.Equal(standbyPublic, n.standby.receiptPublicKey) || bytes.Equal(standbyPublic, public) {
			t.Fatal("standby must use its own generated receipt key")
		}
		clear(standbySeed)
		clear(standbyPrivate)
		primaryGenesis, err := os.ReadFile(filepath.Join(n.home, "config", "genesis.json"))
		if err != nil {
			t.Fatal(err)
		}
		standbyGenesis, err := os.ReadFile(filepath.Join(n.standby.home, "config", "genesis.json"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(primaryGenesis, standbyGenesis) {
			t.Fatal("standby must copy the primary genesis")
		}
		primaryID, err := tmconfig.DefaultConfig().SetRoot(n.home).LoadNodeKeyID()
		if err != nil {
			t.Fatal(err)
		}
		var standbyTM struct {
			P2P struct {
				PersistentPeers string `toml:"persistent-peers"`
			} `toml:"p2p"`
		}
		if _, err := toml.DecodeFile(filepath.Join(n.standby.home, "config", "config.toml"), &standbyTM); err != nil {
			t.Fatal(err)
		}
		if standbyTM.P2P.PersistentPeers != string(primaryID)+"@"+n.p2pAddress {
			t.Fatal("standby must peer with the primary")
		}
		if n.rpcURL != "" || n.standby.rpcURL != "" || n.consensusAddress != "127.0.0.4:0" || n.standby.consensusAddress != "127.0.0.4:0" || n.p2pAddress == n.standby.p2pAddress {
			t.Fatal("standby must have its own EVM, RPC and P2P listeners")
		}
		if _, err := os.Stat(filepath.Join(n.standby.home, "paxd.log")); !os.IsNotExist(err) {
			t.Fatal("preparing the standby home must not start paxd")
		}
		if filepath.Dir(cfg.ReceiptKeyFile) != filepath.Join(n.home, "config") || !filepath.IsAbs(cfg.WALDir) {
			t.Fatal("sequencer paths must refer to this temporary home")
		}
		var tm struct {
			Mode string `toml:"mode"`
		}
		if _, err := toml.DecodeFile(filepath.Join(n.home, "config", "config.toml"), &tm); err != nil {
			t.Fatal(err)
		}
		if tm.Mode != "validator" {
			t.Fatalf("single-node sequencer must run as a validator, got %q", tm.Mode)
		}
		if _, err := os.Stat(filepath.Join(n.home, "paxd.log")); !os.IsNotExist(err) {
			t.Fatal("preparing the home must not start paxd")
		}
		clear(seed)
		clear(private)
	}
}

func TestRecorderHomeStartsSequencerReceiptRPC(t *testing.T) {
	paxd, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("COMMITTED_FIXTURE_EXEC_PAXD", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	n, err := prepareHome(ctx, paxd)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(n.home) })
	stop, err := n.start()
	if err != nil {
		for _, peer := range []*node{n, n.standby} {
			var tail bytes.Buffer
			peer.printLogTail(&tail)
			t.Log(tail.String())
		}
		t.Fatalf("start prepared sequencer home: %v", err)
	}
	t.Cleanup(stop)
	probe, cancelProbe := context.WithTimeout(ctx, time.Second)
	defer cancelProbe()
	var receipt json.RawMessage
	if err := n.client.CallContext(probe, &receipt, "pax_sequencerReceipt", common.Hash{}); err != nil {
		t.Fatalf("primary sequencer receipt RPC must answer after start: %v", err)
	}
	if string(receipt) != "null" {
		t.Fatalf("unknown transaction must have no receipt, got %s", receipt)
	}
	for _, peer := range []*node{n, n.standby} {
		if peer.processDone == nil {
			t.Fatal("both primary and standby must have started")
		}
		select {
		case <-peer.processDone:
			t.Fatalf("paxd in %s exited before stop", peer.home)
		default:
		}
		if info, err := os.Stat(filepath.Join(peer.home, "paxd.log")); err != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("each paxd must have its own private log: %v", err)
		}
	}
	conn, err := net.DialTimeout("tcp", n.standbyAddress, time.Second)
	if err != nil {
		t.Fatalf("standby must serve the configured replication address: %v", err)
	}
	conn.Close()
	stop()
	for _, peer := range []*node{n, n.standby} {
		select {
		case <-peer.processDone:
		default:
			t.Fatalf("stop did not terminate paxd in %s", peer.home)
		}
	}
}

// TestStandbyLeaderIsInTheGenesisCommittee runs the real fixture through standby
// activation at height 2 and verifies a signed commit at height 3. Reproduce with:
//
//	CGO_ENABLED=1 go test -count=1 ./daemon/committed-fixture/ -run '^TestStandbyLeaderIsInTheGenesisCommittee$'
//
// The fixture starts its embedded paxd processes with:
//
//	paxd start --home <temporary-home> --chain-id committed-fixture
func TestStandbyLeaderIsInTheGenesisCommittee(t *testing.T) {
	paxd, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("COMMITTED_FIXTURE_EXEC_PAXD", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	n, err := prepareHome(ctx, paxd)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(n.home) })
	standby, err := n.prepareStandby()
	if err != nil {
		t.Fatal(err)
	}
	genesisPath := filepath.Join(n.home, "config", "genesis.json")
	genesis, err := tmtypes.GenesisDocFromFile(genesisPath)
	if err != nil {
		t.Fatal(err)
	}
	var app struct {
		Sequencer struct {
			Leaders          [][]byte              `json:"leaders"`
			ReceiptKeys      seqkeeper.ReceiptKeys `json:"receipt_keys"`
			ActivationHeight uint64                `json:"activation_height"`
		} `json:"sequencer"`
	}
	if err := json.Unmarshal(genesis.AppState, &app); err != nil {
		t.Fatal(err)
	}
	state := app.Sequencer
	if len(genesis.Validators) != 2 || len(state.Leaders) != 2 || state.ActivationHeight != 2 {
		t.Fatalf("want two validators and pinned leaders activating at 2, got %d, %d, %d",
			len(genesis.Validators), len(state.Leaders), state.ActivationHeight)
	}
	committee := make([]*tmtypes.Validator, 0, len(genesis.Validators))
	for _, validator := range genesis.Validators {
		committee = append(committee, tmtypes.NewValidator(validator.PubKey, validator.Power))
	}
	set := tmtypes.NewValidatorSet(committee)
	for _, leader := range state.Leaders {
		found := false
		for _, validator := range genesis.Validators {
			if bytes.Equal(leader, validator.PubKey.Bytes()) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("pinned leader %X is outside the genesis committee", leader)
		}
	}
	if err := state.ReceiptKeys.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(state.ReceiptKeys) != 2 || bytes.Equal(n.receiptPublicKey, standby.receiptPublicKey) {
		t.Fatal("primary and standby must retain distinct receipt keys")
	}
	for i, peer := range []*node{n, standby} {
		var key struct {
			Address string `json:"address"`
			PubKey  struct {
				Value []byte `json:"value"`
			} `json:"pub_key"`
		}
		if err := readJSON(filepath.Join(peer.home, "config", "priv_validator_key.json"), &key); err != nil {
			t.Fatal(err)
		}
		validator := genesis.Validators[i]
		wantPower := int64(7000000000)
		if peer == standby {
			wantPower = 1
		}
		if !bytes.Equal(validator.PubKey.Bytes(), key.PubKey.Value) ||
			!strings.EqualFold(hex.EncodeToString(validator.Address), key.Address) || validator.Power != wantPower ||
			!bytes.Equal(state.Leaders[i], key.PubKey.Value) {
			t.Fatalf("validator %d must use its own consensus key and power %d", i, wantPower)
		}
		if peer == n && validator.Power <= set.TotalVotingPower()*2/3 {
			t.Fatal("primary alone must retain the quorum")
		}
		if !bytes.Equal(state.ReceiptKeys[i].PublicKey, peer.receiptPublicKey) || bytes.Equal(peer.receiptPublicKey, key.PubKey.Value) {
			t.Fatalf("validator %d must retain its distinct pinned receipt key", i)
		}
		var cfg struct {
			Mode string `toml:"mode"`
		}
		if _, err := toml.DecodeFile(filepath.Join(peer.home, "config", "config.toml"), &cfg); err != nil {
			t.Fatal(err)
		}
		if cfg.Mode != "validator" {
			t.Fatalf("validator %d runs in mode %q", i, cfg.Mode)
		}
		data, err := os.ReadFile(filepath.Join(peer.home, "config", "genesis.json"))
		if err != nil {
			t.Fatal(err)
		}
		primaryData, err := os.ReadFile(genesisPath)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, primaryData) {
			t.Fatal("both validators must use the same genesis")
		}
	}
	t.Cleanup(func() {
		if t.Failed() {
			for _, peer := range []*node{n, standby} {
				var tail bytes.Buffer
				peer.printLogTail(&tail)
				t.Log(tail.String())
			}
		}
	})
	stop, err := n.start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(stop)
	client, err := tmhttp.New("http://" + n.consensusAddress)
	if err != nil {
		t.Fatal(err)
	}
	height := int64(state.ActivationHeight) + 1
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		status, err := client.Status(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if status.SyncInfo.LatestBlockHeight >= height {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("fixture did not commit past activation height %d: %v", state.ActivationHeight, ctx.Err())
		case <-ticker.C:
		}
	}
	commit, err := client.Commit(ctx, &height)
	if err != nil {
		t.Fatal(err)
	}
	if commit == nil || commit.Header == nil || commit.Commit == nil || commit.Header.Height != height ||
		!bytes.Equal(commit.Header.ValidatorsHash, set.Hash()) || !bytes.Equal(commit.Commit.BlockID.Hash, commit.Header.Hash()) {
		t.Fatal("committed block past activation must bind the genesis committee and header")
	}
	if err := set.VerifyCommit(fixtureChainID, commit.Commit.BlockID, height, commit.Commit); err != nil {
		t.Fatalf("verify committed block past standby activation: %v", err)
	}
	t.Logf("fixture committed block %d past standby ActivationHeight %d", height, state.ActivationHeight)
}
