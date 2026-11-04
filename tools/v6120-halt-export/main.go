// Command v6120-halt-export adds the mainnet Launch-x contract code and
// storage the v6.12.0 upgrade reads to the halt export fixture. It applies
// the v6.12.0 plan on the export, replays every golden.json getter and
// refusal and the usdl.json reads at the halt height and after the plan,
// records every EVM code and storage key the run reads, fetches the keys the
// export lacks with eth_getCode and eth_getStorageAt at one height and repeats
// until a run reads nothing new.
//
// Run from the repository root:
//
//	go run ./tools/v6120-halt-export
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
)

const (
	defaultRPC = "https://api-mainnet-beta.paxeer.network"
	batchSize  = 100
	maxRounds  = 1024
)

type layoutGeneration struct {
	Generation                 int            `json:"generation"`
	SidioraFactory             common.Address `json:"sidioraFactory"`
	PoolRegistry               common.Address `json:"poolRegistry"`
	Treasury                   common.Address `json:"treasury"`
	PoolBeacon                 common.Address `json:"poolBeacon"`
	PoolProxyCodeHash          common.Hash    `json:"poolProxyCodeHash"`
	PoolImplementation         common.Address `json:"poolImplementation"`
	PoolImplementationCodeHash common.Hash    `json:"poolImplementationCodeHash"`
	PoolCount                  uint64         `json:"poolCount"`
}

type layoutFile struct {
	Height      uint64             `json:"height"`
	Treasury    common.Address     `json:"treasury"`
	Generations []layoutGeneration `json:"generations"`
	Storage     map[string][]struct {
		Slot  uint64 `json:"slot"`
		Field string `json:"field"`
	} `json:"storage"`
}

func (l layoutFile) slot(role, field string) (common.Hash, error) {
	for _, entry := range l.Storage[role] {
		if entry.Field == field {
			return common.BigToHash(new(big.Int).SetUint64(entry.Slot)), nil
		}
	}
	return common.Hash{}, fmt.Errorf("layout.json has no %s.%s", role, field)
}

// readKey is one EVM key a run reads: a storage word, or the code of an
// address when code is set.
type readKey struct {
	addr common.Address
	slot common.Hash
	code bool
}

func lessKey(a, b readKey) bool {
	if c := bytes.Compare(a.addr[:], b.addr[:]); c != 0 {
		return c < 0
	}
	if a.code != b.code {
		return a.code
	}
	return bytes.Compare(a.slot[:], b.slot[:]) < 0
}

// recorder collects the EVM store keys read through a recordingStore.
type recorder struct {
	evmStore string
	reads    map[readKey]bool
}

func (r *recorder) note(key []byte) {
	switch {
	case len(key) == 1+common.AddressLength+common.HashLength && key[0] == evmtypes.StateKeyPrefix[0]:
		r.reads[readKey{addr: common.BytesToAddress(key[1:21]), slot: common.BytesToHash(key[21:])}] = true
	case len(key) == 1+common.AddressLength && (key[0] == evmtypes.CodeKeyPrefix[0] ||
		key[0] == evmtypes.CodeHashKeyPrefix[0] || key[0] == evmtypes.CodeSizeKeyPrefix[0]):
		r.reads[readKey{addr: common.BytesToAddress(key[1:]), code: true}] = true
	}
}

type cacheMultiStore = sdk.CacheMultiStore

type recordingStore struct {
	cacheMultiStore
	rec *recorder
}

func (s recordingStore) GetKVStore(key sdk.StoreKey) sdk.KVStore {
	store := s.cacheMultiStore.GetKVStore(key)
	if key.Name() != s.rec.evmStore {
		return store
	}
	return recordingKV{KVStore: store, rec: s.rec}
}

func (s recordingStore) CacheMultiStore() sdk.CacheMultiStore {
	return recordingStore{cacheMultiStore: s.cacheMultiStore.CacheMultiStore(), rec: s.rec}
}

type recordingKV struct {
	sdk.KVStore
	rec *recorder
}

func (s recordingKV) Get(key []byte) []byte {
	s.rec.note(key)
	return s.KVStore.Get(key)
}

func (s recordingKV) Has(key []byte) bool {
	s.rec.note(key)
	return s.KVStore.Has(key)
}

type codeEntry struct {
	Address string `json:"address"`
	Code    []byte `json:"code"`
}

type stateEntry struct {
	Address string `json:"address"`
	Key     []byte `json:"key"`
	Value   []byte `json:"value"`
}

// replayCall is one recorded read replayed against the seeded chain. check is
// set when the record holds the result at the halt height.
type replayCall struct {
	from     sdk.AccAddress
	to       common.Address
	input    []byte
	reverted bool
	output   []byte
	check    bool
}

type goldenCall struct {
	From     *common.Address `json:"from"`
	To       common.Address  `json:"to"`
	Input    hexutil.Bytes   `json:"input"`
	Reverted bool            `json:"reverted"`
	Output   hexutil.Bytes   `json:"output"`
}

// readCalls lists every getter call and refusal of golden.json and the USDL
// totalSupply and balanceOf reads of usdl.json, sent from caller unless the
// record names a sender.
func readCalls(goldenPath, usdlPath string, caller sdk.AccAddress) ([]replayCall, error) {
	var golden struct {
		Contracts []struct {
			Calls []goldenCall `json:"calls"`
		} `json:"contracts"`
		Refusals []goldenCall `json:"refusals"`
	}
	bz, err := os.ReadFile(goldenPath)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(bz, &golden); err != nil {
		return nil, err
	}
	var usdl struct {
		Address  common.Address `json:"address"`
		Balances []struct {
			Address common.Address `json:"address"`
		} `json:"balances"`
	}
	if bz, err = os.ReadFile(usdlPath); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(bz, &usdl); err != nil {
		return nil, err
	}
	var calls []replayCall
	add := func(c goldenCall) {
		from := caller
		if c.From != nil {
			from = sdk.AccAddress(c.From.Bytes())
		}
		calls = append(calls, replayCall{from: from, to: c.To, input: c.Input, reverted: c.Reverted, output: c.Output,
			check: true})
	}
	for _, c := range golden.Contracts {
		for _, call := range c.Calls {
			add(call)
		}
	}
	for _, c := range golden.Refusals {
		add(c)
	}
	calls = append(calls, replayCall{from: caller, to: usdl.Address, input: crypto.Keccak256([]byte("totalSupply()"))[:4]})
	balanceOf := crypto.Keccak256([]byte("balanceOf(address)"))[:4]
	for _, b := range usdl.Balances {
		calls = append(calls, replayCall{from: caller, to: usdl.Address,
			input: append(append([]byte{}, balanceOf...), common.LeftPadBytes(b.Address.Bytes(), 32)...)})
	}
	return calls, nil
}

type fetcher struct {
	calls          []replayCall
	client         *rpc.Client
	block          rpc.BlockNumberOrHash
	a              *app.App
	base           sdk.Context
	done           map[readKey]bool
	codes          []codeEntry
	states         []stateEntry
	requestTimeout time.Duration
	binding        snapshotBinding
	publication    *publication
	raw            []byte
	heights        []byte
}

// fetch reads every key of keys the base state lacks from the RPC at the
// fetcher height and writes the nonzero results into the base state.
func (f *fetcher) fetch(ctx context.Context, keys []readKey) error {
	todo := f.pending(keys)
	sortReadKeys(todo)
	if f.requestTimeout <= 0 {
		return fmt.Errorf("request timeout must be positive")
	}
	for start := 0; start < len(todo); start += batchSize {
		chunk := todo[start:min(start+batchSize, len(todo))]
		results := make([]hexutil.Bytes, len(chunk))
		batch := make([]rpc.BatchElem, len(chunk))
		for i, k := range chunk {
			if k.code {
				batch[i] = rpc.BatchElem{Method: "eth_getCode", Args: []interface{}{k.addr, f.block}, Result: &results[i]}
			} else {
				batch[i] = rpc.BatchElem{Method: "eth_getStorageAt", Args: []interface{}{k.addr, k.slot, f.block},
					Result: &results[i]}
			}
		}
		request, cancel := context.WithTimeout(ctx, f.requestTimeout)
		err := f.client.BatchCallContext(request, batch)
		cancel()
		if err != nil {
			return fmt.Errorf("fetch batch: %w", err)
		}
		for i, k := range chunk {
			if batch[i].Error != nil {
				return fmt.Errorf("%s %s: %w", batch[i].Method, k.addr.Hex(), batch[i].Error)
			}
			if !k.code && len(results[i]) != common.HashLength {
				return fmt.Errorf("eth_getStorageAt %s %s: %d bytes", k.addr.Hex(), k.slot.Hex(), len(results[i]))
			}
		}
		for i, k := range chunk {
			f.done[k] = true
			if k.code {
				if len(results[i]) == 0 {
					continue
				}
				f.a.EvmKeeper.SetCode(f.base, k.addr, results[i])
				f.codes = append(f.codes, codeEntry{Address: k.addr.Hex(), Code: results[i]})
				continue
			}
			value := common.BytesToHash(results[i])
			if value == (common.Hash{}) {
				continue
			}
			f.a.EvmKeeper.SetState(f.base, k.addr, k.slot, value)
			f.states = append(f.states, stateEntry{Address: k.addr.Hex(), Key: k.slot.Bytes(), Value: value.Bytes()})
		}
		if err := f.save(); err != nil {
			return fmt.Errorf("checkpoint: %w", err)
		}
	}
	return nil
}

func sortReadKeys(keys []readKey) {
	sort.Slice(keys, func(i, j int) bool { return lessKey(keys[i], keys[j]) })
}

// replay sends every call to ctx and returns how many disagree with the
// recorded result.
func (f *fetcher) replay(ctx sdk.Context) (mismatches int) {
	for _, c := range f.calls {
		to := c.to
		out, err := func() (out []byte, err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("call panicked: %v", r)
				}
			}()
			return f.a.EvmKeeper.StaticCallEVM(ctx, c.from, &to, c.input)
		}()
		if c.check && ((err != nil) != c.reverted || (err == nil && !bytes.Equal(out, c.output))) {
			mismatches++
		}
	}
	return mismatches
}

// migrate replays the calls on a discarded branch of the base state at the
// halt height, applies the v6.12.0 plan at the height after the export on
// another and replays the calls after it, and returns the EVM keys all of it
// read, the plan's result and the calls at the halt height that disagree with
// golden.json, adding the code of every account whose storage was read so a
// contract read only by storage keeps its mainnet code.
func (f *fetcher) migrate() (reads []readKey, mismatches int, err error) {
	rec := &recorder{evmStore: evmtypes.StoreKey, reads: map[readKey]bool{}}
	hc, _ := f.base.CacheContext()
	mismatches = f.replay(hc.WithMultiStore(recordingStore{cacheMultiStore: hc.MultiStore().(sdk.CacheMultiStore),
		rec: rec}))
	cc, _ := f.base.CacheContext()
	cc = cc.WithMultiStore(recordingStore{cacheMultiStore: cc.MultiStore().(sdk.CacheMultiStore), rec: rec})
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("migration panicked: %v", r)
			}
		}()
		plan := upgradetypes.Plan{Name: app.V6120Upgrade, Height: cc.BlockHeight() + 1}
		if err = f.a.UpgradeKeeper.ScheduleUpgrade(cc, plan); err != nil {
			return
		}
		f.a.UpgradeKeeper.ApplyUpgrade(cc.WithBlockHeight(plan.Height), plan)
	}()
	if err == nil {
		f.replay(cc.WithBlockHeight(cc.BlockHeight() + 1))
	}
	codes := map[readKey]bool{}
	for k := range rec.reads {
		reads = append(reads, k)
		if code := (readKey{addr: k.addr, code: true}); !k.code && !rec.reads[code] && !codes[code] {
			codes[code] = true
			reads = append(reads, code)
		}
	}
	return reads, mismatches, err
}

func (f *fetcher) pending(reads []readKey) []readKey {
	var out []readKey
	seen := map[readKey]bool{}
	for _, k := range reads {
		if seen[k] {
			continue
		}
		seen[k] = true
		if f.done[k] {
			continue
		}
		if k.code && len(f.a.EvmKeeper.GetCode(f.base, k.addr)) > 0 {
			continue
		}
		if !k.code && f.a.EvmKeeper.GetState(f.base, k.addr, k.slot) != (common.Hash{}) {
			continue
		}
		out = append(out, k)
	}
	return out
}

// check refuses a base state whose Launch-x words disagree with layout.json.
func (f *fetcher) check(l layoutFile) error {
	treasurySlot, err := l.slot("SidioraFactory", "treasury")
	if err != nil {
		return err
	}
	allPools, err := l.slot("PoolRegistry", "_allPools")
	if err != nil {
		return err
	}
	evm := &f.a.EvmKeeper
	for _, g := range l.Generations {
		for _, addr := range []common.Address{g.SidioraFactory, g.PoolRegistry, g.Treasury, g.PoolBeacon,
			g.PoolImplementation} {
			if len(evm.GetCode(f.base, addr)) == 0 {
				return fmt.Errorf("generation %d: no code at %s", g.Generation, addr.Hex())
			}
		}
		if got := crypto.Keccak256Hash(evm.GetCode(f.base, g.PoolImplementation)); got != g.PoolImplementationCodeHash {
			return fmt.Errorf("generation %d: pool implementation %s code hash %s, layout.json %s", g.Generation,
				g.PoolImplementation.Hex(), got.Hex(), g.PoolImplementationCodeHash.Hex())
		}
		treasury := common.BytesToAddress(evm.GetState(f.base, g.SidioraFactory, treasurySlot).Bytes())
		if treasury != g.Treasury {
			return fmt.Errorf("generation %d: SidioraFactory treasury %s, layout.json %s", g.Generation,
				treasury.Hex(), g.Treasury.Hex())
		}
		if g.Generation == 2 && treasury != l.Treasury {
			return fmt.Errorf("generation 2: SidioraFactory treasury %s, layout.json treasury %s", treasury.Hex(),
				l.Treasury.Hex())
		}
		count := evm.GetState(f.base, g.PoolRegistry, allPools).Big()
		if !count.IsUint64() || count.Uint64() != g.PoolCount {
			return fmt.Errorf("generation %d: _allPools length %s, layout.json poolCount %d", g.Generation, count,
				g.PoolCount)
		}
		first := crypto.Keccak256Hash(allPools[:]).Big()
		for i := uint64(0); i < g.PoolCount; i++ {
			slot := common.BigToHash(new(big.Int).Add(first, new(big.Int).SetUint64(i)))
			pool := common.BytesToAddress(evm.GetState(f.base, g.PoolRegistry, slot).Bytes())
			code := evm.GetCode(f.base, pool)
			if len(code) == 0 || crypto.Keccak256Hash(code) != g.PoolProxyCodeHash {
				return fmt.Errorf("generation %d: pool %s code hash %s, layout.json %s", g.Generation, pool.Hex(),
					crypto.Keccak256Hash(code).Hex(), g.PoolProxyCodeHash.Hex())
			}
		}
	}
	return nil
}

// encodeExport appends codes and states to app_state.evm of the export.
func encodeExport(raw []byte, codes []codeEntry, states []stateEntry) ([]byte, error) {
	var doc, appState, evm map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(doc["app_state"], &appState); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(appState["evm"], &evm); err != nil {
		return nil, err
	}
	var allCodes []json.RawMessage
	var allStates []json.RawMessage
	if err := json.Unmarshal(evm["codes"], &allCodes); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(evm["states"], &allStates); err != nil {
		return nil, err
	}
	for _, c := range codes {
		bz, err := json.Marshal(c)
		if err != nil {
			return nil, err
		}
		allCodes = append(allCodes, bz)
	}
	for _, s := range states {
		bz, err := json.Marshal(s)
		if err != nil {
			return nil, err
		}
		allStates = append(allStates, bz)
	}
	var err error
	if evm["codes"], err = marshal(allCodes); err != nil {
		return nil, err
	}
	if evm["states"], err = marshal(allStates); err != nil {
		return nil, err
	}
	if appState["evm"], err = marshal(evm); err != nil {
		return nil, err
	}
	if doc["app_state"], err = marshal(appState); err != nil {
		return nil, err
	}
	out, err := marshal(doc)
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// withInitialHeight returns the genesis document raw with initial_height set
// to height.
func withInitialHeight(raw []byte, height uint64) ([]byte, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	var err error
	if doc["initial_height"], err = marshal(fmt.Sprint(height)); err != nil {
		return nil, err
	}
	return marshal(doc)
}

func marshal(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte{'\n'}), nil
}

func run() error {
	exportPath := flag.String("export", filepath.Join("node", "testdata", "v6120-halt-export.json"), "halt export")
	layoutPath := flag.String("layout", filepath.Join("modules", "launchpad", "keeper", "testdata", "v6120",
		"layout.json"), "layout.json")
	goldenPath := flag.String("golden", filepath.Join("modules", "launchpad", "keeper", "testdata", "v6120",
		"golden.json"), "golden.json")
	usdlPath := flag.String("usdl", filepath.Join("modules", "launchpad", "keeper", "testdata", "v6120",
		"usdl.json"), "usdl.json")
	rpcURL := flag.String("rpc", defaultRPC, "mainnet JSON-RPC")
	chainID := flag.Uint64("chain-id", defaultChainID, "expected EVM chain ID")
	blockHash := flag.String("block-hash", "", "required pinned snapshot block hash (0x + 64 hex digits)")
	requestTimeout := flag.Duration("request-timeout", defaultRequestTimeout, "deadline per RPC request")
	overallTimeout := flag.Duration("overall-timeout", defaultOverallTimeout, "overall export deadline")
	activationDone := flag.Int64("activation-done", 0,
		"done height of the activation (v6.9) plan in the source chain's upgrade store")
	v610Done := flag.Int64("v610-done", 0, "done height of the v6.10 plan in the source chain's upgrade store")
	flag.Parse()
	if *requestTimeout <= 0 || *overallTimeout <= 0 {
		return fmt.Errorf("request and overall timeouts must be positive")
	}
	ctx, cancel := context.WithTimeout(context.Background(), *overallTimeout)
	defer cancel()
	fmt.Fprintf(os.Stderr, "RPC request deadline %s, overall deadline %s\n", *requestTimeout, *overallTimeout)
	var pinned common.Hash
	if err := pinned.UnmarshalText([]byte(*blockHash)); err != nil || pinned == (common.Hash{}) {
		return fmt.Errorf("--block-hash must pin a nonzero 32-byte snapshot hash")
	}
	publication := newPublication(*exportPath)
	lock, err := os.OpenFile(*exportPath+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fmt.Errorf("export is locked: %w", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)

	var l layoutFile
	bz, err := os.ReadFile(*layoutPath)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(bz, &l); err != nil {
		return err
	}
	if len(l.Generations) != 2 {
		return fmt.Errorf("layout.json lists %d generations", len(l.Generations))
	}
	raw, recordRaw, savedHeights, err := publication.open()
	if err != nil {
		return err
	}
	doc, err := tmtypes.GenesisDocFromJSON(raw)
	if err != nil {
		return err
	}

	heights, err := marshal(map[string]int64{app.ActivationUpgrade: *activationDone, app.V610Upgrade: *v610Done})
	if err != nil {
		return err
	}
	heights = append(heights, '\n')
	if len(savedHeights) > 0 && !bytes.Equal(savedHeights, heights) {
		var saved, wanted map[string]int64
		if json.Unmarshal(savedHeights, &saved) != nil || json.Unmarshal(heights, &wanted) != nil || len(saved) != len(wanted) || saved[app.ActivationUpgrade] != *activationDone || saved[app.V610Upgrade] != *v610Done {
			return fmt.Errorf("upgrade heights sidecar does not match requested heights")
		}
	}
	binding := snapshotBinding{Version: 1, Height: l.Height, ChainID: *chainID, GenesisChain: doc.ChainID, BlockHash: pinned, LayoutDigest: digest(bz), ExportDigest: digest(raw), HeightsDigest: digest(heights)}
	record, err := readRecord(recordRaw, binding)
	if err != nil {
		return err
	}
	dialContext, dialCancel := context.WithTimeout(ctx, *requestTimeout)
	client, err := rpc.DialContext(dialContext, *rpcURL)
	dialCancel()
	if err != nil {
		return err
	}
	defer client.Close()
	if err := validateSnapshot(ctx, client, binding, *requestTimeout); err != nil {
		return err
	}
	height := record.Height
	if *activationDone <= 0 || *v610Done <= *activationDone || uint64(*v610Done) > height {
		return fmt.Errorf("done heights activation %d and v6.10 %d must rise and not pass the halt height %d",
			*activationDone, *v610Done, height)
	}
	// The export is a mainnet export: its chain starts at the block after the
	// halt height H, as paxd export writes it.
	if raw, err = withInitialHeight(raw, height+1); err != nil {
		return err
	}
	if doc, err = tmtypes.GenesisDocFromJSON(raw); err != nil {
		return err
	}

	home, err := os.MkdirTemp("", "v6120-halt-export")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(home) }()
	app.DefaultNodeHome = home
	a := app.SetupWithDefaultHome(true, true, false)
	defer a.Close()
	params := doc.ConsensusParams.ToProto()
	if _, err := a.InitChain(ctx, &abci.RequestInitChain{
		Time:            doc.GenesisTime,
		ChainId:         doc.ChainID,
		ConsensusParams: &params,
		AppStateBytes:   doc.AppState,
		InitialHeight:   doc.InitialHeight,
	}); err != nil {
		return err
	}
	base := a.NewContext(false, tmproto.Header{Height: doc.InitialHeight - 1, ChainID: doc.ChainID,
		Time: doc.GenesisTime})

	a.UpgradeKeeper.SetDone(base.WithBlockHeight(*activationDone), app.ActivationUpgrade)
	a.UpgradeKeeper.SetDone(base.WithBlockHeight(*v610Done), app.V610Upgrade)
	calls, err := readCalls(*goldenPath, *usdlPath, a.LaunchpadKeeper.ModuleAddress())
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "replaying %d calls at heights %d and %d\n", len(calls), height, height+1)
	f := &fetcher{calls: calls, client: client, block: rpc.BlockNumberOrHashWithHash(pinned, true), a: a, base: base, done: map[readKey]bool{},
		requestTimeout: *requestTimeout, binding: binding, publication: publication, raw: raw, heights: heights}
	for _, e := range record.Absent {
		if e.Slot == nil {
			f.done[readKey{addr: e.Address, code: true}] = true
		} else {
			f.done[readKey{addr: e.Address, slot: *e.Slot}] = true
		}
	}
	treasurySlot, err := l.slot("SidioraFactory", "treasury")
	if err != nil {
		return err
	}
	allPools, err := l.slot("PoolRegistry", "_allPools")
	if err != nil {
		return err
	}
	var seeds []readKey
	for _, g := range l.Generations {
		for _, addr := range []common.Address{g.SidioraFactory, g.PoolRegistry, g.Treasury, g.PoolBeacon,
			g.PoolImplementation} {
			seeds = append(seeds, readKey{addr: addr, code: true})
		}
		seeds = append(seeds, readKey{addr: g.SidioraFactory, slot: treasurySlot},
			readKey{addr: g.PoolRegistry, slot: allPools})
	}
	if err := f.save(); err != nil {
		return err
	}
	pending := seeds
	var migrateErr error
	for round := 1; len(pending) > 0; round++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if round > maxRounds {
			return fmt.Errorf("no fixed point after %d rounds", maxRounds)
		}
		if err := f.fetch(ctx, pending); err != nil {
			return err
		}
		var reads []readKey
		var mismatches int
		reads, mismatches, migrateErr = f.migrate()
		pending = f.pending(reads)
		fmt.Fprintf(os.Stderr, "round %d: %d reads, %d to fetch, %d calls differ from golden.json at %d, result %v\n",
			round, len(reads), len(pending), mismatches, height, migrateErr)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := f.check(l); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "height %d: adding %d codes and %d states\n", height, len(f.codes), len(f.states))
	if err := f.save(); err != nil {
		return err
	}
	if migrateErr != nil {
		return fmt.Errorf("v6.12.0 plan on the export at height %d: %w", height, migrateErr)
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
