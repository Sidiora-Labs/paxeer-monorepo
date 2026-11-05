// Proves: layerxledger.PrecompileExecutor -> vm.EVM.Call/StaticCall and emitted log JSON -> decodePayerGrantIssued, AbiRegistry.decode_log/Store and pays_idempotency_key.
//
// The application registers the real 0x101A precompile and mounts its real stores.
// Calls, logs and reads below use that registration, including after closing and
// reopening the application's LevelDB. No kernel row or EVM log is fabricated.
//
// Repairs abi.json's missing register/openAccount/issueGrant identifier outputs:
// their implementations return bytes32, which clients must be able to decode.
// Repairs useSurface.ts's acceptance of nonempty PayerGrantIssued data and
// gateway.rs's acceptance of a noncanonical uint128 payment amount.
//
// Repairs modules/layerxledger/keeper/grants.go:14-35, Keeper.IssueGrant and
// Keeper.RevokeGrant, which submitted retired activities 0x00010007/0x00010008.
// Decision 2340 requires both and PrecompileExecutor.getPayerGrant to refuse
// with the registered LXP_NO_PAYER_GRANT reason (-214), without state or logs.
// draw refuses the same way; no PayerGrantIssued or PaymentDrawn is fabricated.
// Consumer grant discovery must therefore remain empty. PaymentExact and
// Transfer still travel through the real application and consumer decoders.
// modules/lxphost/ctx.go, branchCtx.checkPaymentReference (task 22.2.7), keeps
// committed references authoritative after reopening; duplicate payExact must
// refuse without changing balances or host state.
// The gateway uses pays_idempotency_key after receipt authentication; the proof
// calls that same matcher with application-produced logs, without a fake receipt.
// The wallet leg installs its repository-pinned dependencies explicitly so a
// fresh sandbox executes the consumer rather than depending on a prior install.
package layerxledger_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"math"
	"math/big"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmcfg "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

const (
	PayerGrantIssued = layerxledger.PayerGrantIssued
	PaymentDrawn     = layerxledger.PaymentDrawn
	PaymentExact     = layerxledger.PaymentExact
	Transfer         = layerxledger.Transfer
)

type ledgerPath struct {
	app *app.App
	ctx sdk.Context
	abi abi.ABI
}

func (p *ledgerPath) call(t *testing.T, caller common.Address, name string, args ...interface{}) ([]byte, []*ethtypes.Log, error) {
	t.Helper()
	input, err := p.abi.Pack(name, args...)
	require.NoError(t, err)
	db := state.NewDBImpl(p.ctx.WithEventManager(sdk.NewEventManager()), &p.app.EvmKeeper, false)
	block, err := p.app.EvmKeeper.GetVMBlockContext(db.Ctx(), core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	config := evmtypes.DefaultChainConfig().EthereumConfig(p.app.EvmKeeper.ChainID(p.ctx))
	evm := vm.NewEVM(*block, db, config, vm.Config{}, p.app.EvmKeeper.CustomPrecompiles(db.Ctx()))
	evm.TxContext.Origin = caller
	address := common.HexToAddress(layerxledger.LedgerAddress)
	require.Contains(t, p.app.EvmKeeper.CustomPrecompiles(db.Ctx()), address)
	var data []byte
	if p.abi.Methods[name].IsConstant() {
		data, _, err = evm.StaticCall(caller, address, input, 2_000_000)
	} else {
		data, _, err = evm.Call(caller, address, input, 2_000_000, uint256.NewInt(0))
	}
	logs := db.GetAllLogs()
	if err == nil {
		_, finalizeErr := db.Finalize()
		require.NoError(t, finalizeErr)
	}
	return data, logs, err
}

func (p *ledgerPath) success(t *testing.T, caller common.Address, name string, args ...interface{}) ([]interface{}, []*ethtypes.Log) {
	t.Helper()
	data, logs, err := p.call(t, caller, name, args...)
	require.NoError(t, err, "%s: %x", name, data)
	values, err := p.abi.Methods[name].Outputs.Unpack(data)
	require.NoError(t, err, name)
	return values, logs
}

func accountId(t *testing.T, p *ledgerPath, caller common.Address, owner, asset [32]byte) [32]byte {
	t.Helper()
	values, logs := p.success(t, caller, "accountId", owner, asset)
	require.Empty(t, logs)
	require.Len(t, values, 1)
	return values[0].([32]byte)
}

func getPayerGrant(t *testing.T, p *ledgerPath, caller common.Address, id [32]byte) {
	t.Helper()
	data, logs, err := p.call(t, caller, "getPayerGrant", id)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	assertLedgerRevert(t, data, "LXP_NO_PAYER_GRANT")
	require.Empty(t, logs)
}

func draw(t *testing.T, p *ledgerPath, caller common.Address, grant, recipient, purpose, contextHash [32]byte) []*ethtypes.Log {
	t.Helper()
	data, logs, err := p.call(t, caller, "draw", grant, recipient, big.NewInt(15), [32]byte{0x51}, uint64(0), purpose, contextHash)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	assertLedgerRevert(t, data, "LXP_NO_PAYER_GRANT")
	require.Empty(t, logs, "refused draw must not invent PaymentDrawn or Transfer")
	return logs
}

func payExact(t *testing.T, p *ledgerPath, caller common.Address, from, to, asset, reference [32]byte) []*ethtypes.Log {
	t.Helper()
	values, logs := p.success(t, caller, "payExact", from, to, asset, big.NewInt(15), uint64(p.ctx.BlockTime().UnixMilli())+60_000, reference)
	require.Empty(t, values)
	return logs
}

func proofLedgerLog(t *testing.T, contract abi.ABI, log *ethtypes.Log, name string, want map[string]interface{}) {
	t.Helper()
	require.Equal(t, common.HexToAddress(layerxledger.LedgerAddress), log.Address)
	event := contract.Events[name]
	require.NotEmpty(t, log.Topics)
	require.Equal(t, event.ID, log.Topics[0])
	fields := map[string]interface{}{}
	require.NoError(t, event.Inputs.NonIndexed().UnpackIntoMap(fields, log.Data))
	var indexed abi.Arguments
	for _, arg := range event.Inputs {
		if arg.Indexed {
			indexed = append(indexed, arg)
		}
	}
	require.NoError(t, abi.ParseTopicsIntoMap(fields, indexed, log.Topics[1:]))
	require.Equal(t, want, fields)
}

func TestProofLayerxledger0x101a(t *testing.T) {
	dir := t.TempDir()
	open := func(restarting bool) *ledgerPath {
		t.Helper()
		db, err := sdk.NewLevelDB("ledger-path", dir)
		require.NoError(t, err)
		encoding := app.MakeEncodingConfig()
		// All application stores (SC, receipt and wasm), as well as LevelDB,
		// must reopen the same home; SetupWithDB allocates a new home per call.
		a := app.New(db, nil, true, map[int64]bool{}, dir, 1, true,
			tmcfg.DefaultConfig().SetRoot(dir), encoding, nil, app.TestAppOpts{}, app.EmptyWasmOpts, nil)
		if !restarting {
			genesis, err := json.Marshal(app.NewDefaultGenesisState(encoding.Marshaler))
			require.NoError(t, err)
			_, err = a.InitChain(context.Background(), &abci.RequestInitChain{
				Time: time.Unix(1_800_000_000, 0).UTC(), ChainId: "pax-test",
				ConsensusParams: app.DefaultConsensusParams, AppStateBytes: genesis,
			})
			require.NoError(t, err)
		}
		header := tmproto.Header{ChainID: "pax-test", Height: 1, Time: time.Unix(1_800_000_000, 0).UTC()}
		var ctx sdk.Context
		if restarting {
			header.Height = a.LastBlockHeight() + 1
			ctx, _ = a.NewUncachedContext(false, header).CacheContext()
		} else {
			ctx = a.NewContext(false, header)
			a.UpgradeKeeper.SetDone(ctx, app.MergeUpgrade)
		}
		a.RefreshPrecompiles(ctx)
		return &ledgerPath{app: a, ctx: ctx, abi: layerxledger.GetABI()}
	}
	p := open(false)
	t.Cleanup(func() {
		if p != nil {
			require.NoError(t, p.app.Close())
		}
	})
	payerCaller := common.HexToAddress("0x2320000000000000000000000000000000000001")
	payeeCaller := common.HexToAddress("0x2320000000000000000000000000000000000002")
	bind := func(caller common.Address, tag byte) (ed25519.PrivateKey, ledgerkeeper.Actor) {
		t.Helper()
		var seed [32]byte
		seed[0] = tag
		private := ed25519.NewKeyFromSeed(seed[:])
		var key [32]byte
		copy(key[:], private.Public().(ed25519.PublicKey))
		message, err := evmtypes.LayerXBindMessage(p.app.EvmKeeper.ChainID(p.ctx), caller, 0)
		require.NoError(t, err)
		var signature [64]byte
		copy(signature[:], ed25519.Sign(private, message))
		_, err = p.app.EvmKeeper.BindLayerX(p.ctx, caller, key, signature)
		require.NoError(t, err)
		actor, err := p.app.LayerXLedgerKeeper.Actor(p.ctx, caller)
		require.NoError(t, err)
		return private, actor
	}
	private, payer := bind(payerCaller, 1)
	_, payee := bind(payeeCaller, 2)
	salt := [32]byte{0x41}
	material := append(append([]byte("LX:ASSET:v1"), payer.Authority.Actor[:]...), salt[:]...)
	asset := sha256.Sum256(material)
	values, _ := p.success(t, payerCaller, "register", asset, salt, "PATH", "Ledger path", uint8(6), big.NewInt(1000), uint8(ledgertypes.AssetIssuerNative), []byte{})
	require.Equal(t, []interface{}{asset}, values)
	from := accountId(t, p, payerCaller, payer.DIDKey, asset)
	to := accountId(t, p, payeeCaller, payee.DIDKey, asset)
	for _, entry := range []struct {
		caller common.Address
		id     [32]byte
	}{{payerCaller, from}, {payeeCaller, to}} {
		values, _ = p.success(t, entry.caller, "openAccount", asset)
		require.Equal(t, []interface{}{entry.id}, values)
	}
	p.success(t, payerCaller, "mint", asset, from, big.NewInt(250))
	reference := [32]byte{0x61}
	logs := payExact(t, p, payerCaller, from, to, asset, reference)
	require.Len(t, logs, 2)
	proofLedgerLog(t, p.abi, logs[0], Transfer, map[string]interface{}{
		"from": from, "to": to, "assetId": asset, "amount": big.NewInt(15), "kind": uint8(5),
	})
	proofLedgerLog(t, p.abi, logs[1], PaymentExact, map[string]interface{}{
		"paymentReference": reference, "from": from, "to": to, "assetId": asset, "amount": big.NewInt(15),
	})
	paymentLogs := logs
	assertBalances := func() {
		t.Helper()
		for id, amount := range map[[32]byte]int64{from: 235, to: 15} {
			values, _ := p.success(t, payerCaller, "balanceOf", id, asset)
			require.Equal(t, big.NewInt(amount), values[0])
			balance, err := p.app.LayerXLedgerKeeper.BalanceOf(p.ctx, id, asset)
			require.NoError(t, err)
			require.Equal(t, sdk.NewInt(amount), balance)
		}
	}
	assertBalances()
	require.NoError(t, p.app.SetSealedStateToCommit(p.ctx, p.ctx.MultiStore().(sdk.CacheMultiStore)))
	_, err := p.app.Commit(context.Background())
	require.NoError(t, err)
	require.NoError(t, p.app.Close())
	p = nil
	p = open(true)
	assertBalances()
	require.Equal(t, from, accountId(t, p, payerCaller, payer.DIDKey, asset))
	before := storeSnapshot(t, p.ctx.KVStore(p.app.GetKey(app.LayerXHostStoreKey)))
	data, duplicateLogs, duplicateErr := p.call(t, payerCaller, "payExact", from, to, asset, big.NewInt(15), uint64(p.ctx.BlockTime().UnixMilli())+60_000, reference)
	require.ErrorIs(t, duplicateErr, vm.ErrExecutionReverted, "a duplicate must never debit twice")
	assertLedgerRevert(t, data, "LXP_IDEMPOTENT_REPLAY")
	require.Empty(t, duplicateLogs)
	require.Equal(t, before, storeSnapshot(t, p.ctx.KVStore(p.app.GetKey(app.LayerXHostStoreKey))))
	assertBalances()
	data, logs, err = p.call(t, payeeCaller, "payExact", from, to, asset, big.NewInt(15), uint64(p.ctx.BlockTime().UnixMilli())+60_000, [32]byte{0x62})
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	assertLedgerRevert(t, data, "LXP_UNAUTHORIZED_DEBIT")
	require.Empty(t, logs)
	assertBalances()

	// Construct the signed producer request from the real accounts and keys.
	// The retired reader and draw must refuse even this well-formed grant.
	purpose := [32]byte{0x22}
	contextHash, err := codec.DomainHash(codec.DomainContextHash, purpose[:])
	require.NoError(t, err)
	body := append(append(append([]byte(nil), from[:]...), to[:]...), asset[:]...)
	body = binary.BigEndian.AppendUint64(body, 0)
	body = binary.BigEndian.AppendUint64(body, 40)
	body = binary.BigEndian.AppendUint64(body, 0)
	body = binary.BigEndian.AppendUint64(body, 100)
	body = append(body, 0)
	body = binary.BigEndian.AppendUint64(body, 0)
	body = binary.BigEndian.AppendUint64(body, uint64(p.ctx.BlockTime().UnixMilli())+60_000)
	body = append(body, purpose[:]...)
	body = append(body, 0)
	body = append(body, make([]byte, 32)...)
	body = binary.BigEndian.AppendUint64(body, 0)
	body = append(body, payer.DIDKey[:]...)
	grantID, err := codec.DomainHash(codec.DomainAuthorityHash, append([]byte("LXP:GRANT:v1"), body...))
	require.NoError(t, err)
	grant := append(append([]byte(nil), grantID[:]...), body...)
	grant = append(grant, ed25519.Sign(private, grantID[:])...)
	require.NoError(t, ledgertypes.ValidateIssueGrantPayload(grant))
	before = storeSnapshot(t, p.ctx.KVStore(p.app.GetKey(app.LayerXHostStoreKey)))
	assertRetired := func() []*ethtypes.Log {
		t.Helper()
		data, logs, err := p.call(t, payerCaller, "issueGrant", grant)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		assertLedgerRevert(t, data, "LXP_NO_PAYER_GRANT")
		require.Empty(t, logs, "retired issuance must not emit PayerGrantIssued")
		grantLogs := append([]*ethtypes.Log{}, logs...)
		data, logs, err = p.call(t, payerCaller, "revokeGrant", grantID, uint64(0))
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		assertLedgerRevert(t, data, "LXP_NO_PAYER_GRANT")
		require.Empty(t, logs, "retired revocation must not emit PayerGrantRevoked")
		getPayerGrant(t, p, payerCaller, grantID)
		require.Equal(t, before, storeSnapshot(t, p.ctx.KVStore(p.app.GetKey(app.LayerXHostStoreKey))))
		return grantLogs
	}
	grantLogs := assertRetired()
	refusedLogs := draw(t, p, payeeCaller, grantID, to, purpose, contextHash)
	require.Equal(t, before, storeSnapshot(t, p.ctx.KVStore(p.app.GetKey(app.LayerXHostStoreKey))))
	require.NoError(t, p.app.SetSealedStateToCommit(p.ctx, p.ctx.MultiStore().(sdk.CacheMultiStore)))
	_, err = p.app.Commit(context.Background())
	require.NoError(t, err)
	require.NoError(t, p.app.Close())
	p = nil
	p = open(true)
	before = storeSnapshot(t, p.ctx.KVStore(p.app.GetKey(app.LayerXHostStoreKey)))
	assertRetired()
	require.Equal(t, "PayerGrantIssued(bytes32)", p.abi.Events[PayerGrantIssued].Sig)
	require.Equal(t, "PaymentDrawn(bytes32,bytes32,bytes32,bytes32,uint128,bytes32,bytes32)", p.abi.Events[PaymentDrawn].Sig)
	getPayerGrant(t, p, payerCaller, [32]byte{0xff})
	assertBalances()

	root, err := filepath.Abs("../..")
	require.NoError(t, err)
	input, err := json.Marshal(map[string]interface{}{
		"paymentLogs": paymentLogs, "grantLogs": grantLogs, "refusedLogs": append([]*ethtypes.Log{}, refusedLogs...),
		"reference": common.Hash(reference).Hex(), "grantId": common.Hash(grantID).Hex(),
		"from": common.Hash(from).Hex(), "to": common.Hash(to).Hex(), "asset": common.Hash(asset).Hex(),
		"owner": payerCaller.Hex(), "store": filepath.Join(dir, "history.sqlite"),
	})
	require.NoError(t, err)
	for _, consumer := range []struct {
		name, directory, executable string
		args                        []string
	}{
		{"gateway", root, "cargo", []string{"test", "--manifest-path", "services/interop/Cargo.toml", "-p", "layerx-interop-gateway", "--lib", "proof_layerxledger_0x101a", "--", "--nocapture"}},
		{"indexer", root, "sh", []string{"tools/runtime/run-with-clock.sh", "cargo", "test", "--manifest-path", "services/platform/Cargo.toml", "-p", "layerx-indexer", "--test", "history_position", "proof_layerxledger_0x101a", "--", "--exact", "--nocapture"}},
		{"wallet", filepath.Join(root, "apps/wallet"), "node", []string{"--input-type=module", "-e", `import {readFileSync} from 'node:fs'; import {startVitest} from 'vitest/node'; const ledgerProof=JSON.parse(readFileSync(0,'utf8')); const ctx=await startVitest('test',['src/surfaces/WebDataView.test.tsx'],{watch:false,testNamePattern:'^proof_layerxledger_0x101a$',provide:{ledgerProof}}); if(!ctx) process.exit(1); await ctx.close();`}},
	} {
		t.Run(consumer.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
			defer cancel()
			if consumer.name == "wallet" {
				for _, install := range []struct {
					directory, executable string
					args                  []string
				}{
					{consumer.directory, "pnpm", []string{"install", "--ignore-workspace", "--frozen-lockfile", "--ignore-scripts"}},
					{filepath.Join(root, "services/agent/sdk/typescript"), "npm", []string{"ci", "--ignore-scripts", "--no-audit", "--no-fund"}},
				} {
					setup := exec.CommandContext(ctx, install.executable, install.args...)
					setup.Dir = install.directory
					output, err := setup.CombinedOutput()
					require.NoError(t, err, "repository consumer dependencies %s: %s", install.directory, output)
				}
			}
			cmd := exec.CommandContext(ctx, consumer.executable, consumer.args...)
			cmd.Dir = consumer.directory
			cmd.Stdin = bytes.NewReader(input)
			output, err := cmd.CombinedOutput()
			require.NoError(t, err, "real consumer %s: %s", consumer.name, output)
			require.Contains(t, string(output), "1 passed", "consumer test must execute")
		})
	}
}
