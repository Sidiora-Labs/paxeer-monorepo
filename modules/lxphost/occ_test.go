//go:build cgo

package lxphost

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"reflect"
	"sync"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/tasks"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/occ"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/utils/tracing"
)

func TestHostOCCMatchesSequential(t *testing.T) {
	const blockTime = int64(1700000000000)
	const accountCount, transferCount, workers = 8, 32, 8
	const initialBalance, supplyCap = int64(1000), int64(1000000)
	var owners [accountCount - 1]KernelAuthority
	for i := range owners {
		owners[i] = executeActivityAuthority(t, [32]byte{byte(0x40 + i)})
	}
	registration, asset := executeActivityAssetPayload(owners[0], 0x61)
	registration = append([]byte(nil), registration...)
	binary.BigEndian.PutUint64(registration[len(registration)-10:len(registration)-2], uint64(supplyCap))
	var accounts [accountCount][32]byte
	_, issuance, err := IssuanceAccountID(asset)
	if err != nil {
		t.Fatal(err)
	}
	accounts[0] = issuance
	for i := range owners {
		_, id, err := AssetAccountID(owners[i].VerifiedKey, asset)
		if err != nil {
			t.Fatal(err)
		}
		accounts[i+1] = id
	}
	supplyPayload := func(account [32]byte, amount uint64) []byte {
		payload := append([]byte{0, 1}, asset[:]...)
		payload = append(payload, account[:]...)
		payload = binary.BigEndian.AppendUint64(payload, 0)
		return binary.BigEndian.AppendUint64(payload, amount)
	}
	newFixture := func(pooled bool) (*testChain, *Host, [accountCount - 1]uint64, uint64) {
		chain := newTestChain(t)
		hostWorkers := 0
		if pooled {
			hostWorkers = 4
		}
		host := poolTestHost(t, chain.key, hostWorkers)
		var sequences [accountCount - 1]uint64
		global := uint64(1)
		run := func(owner int, kind uint32, payload []byte) {
			receipt, err := ExecuteActivity(chain.context(1, blockTime, global), host, owners[owner], kind, payload, sequences[owner])
			if err != nil || receipt.ResultCode != 0 {
				t.Fatalf("fixture activity %d: receipt=%+v error=%v", global, receipt, err)
			}
			sequences[owner]++
			global++
		}
		run(0, assetRegisterType, registration)
		for i := range owners {
			run(i, assetAccountOpenType, append([]byte{0, 1}, asset[:]...))
			run(0, assetMintType, supplyPayload(accounts[i+1], uint64(initialBalance)))
		}
		chain.commit()
		if rows := accountRows(t, chain); len(rows) != accountCount {
			t.Fatalf("fixture has %d accounts, want %d", len(rows), accountCount)
		}
		return chain, host, sequences, global
	}
	sequential, serialHost, sequences, firstSequence := newFixture(false)
	concurrent, pooledHost, concurrentSequences, concurrentFirst := newFixture(true)
	if sequences != concurrentSequences || firstSequence != concurrentFirst || !reflect.DeepEqual(sequential.dump(), concurrent.dump()) {
		t.Fatal("sequential and concurrent fixtures differ")
	}
	type transfer struct {
		owner    int
		kind     uint32
		sequence uint64
		payload  []byte
		from, to [32]byte
		amount   int64
	}
	var transfers [transferCount]transfer
	var expectedBalances [accountCount]int64
	expectedBalances[0] = supplyCap - (accountCount-1)*initialBalance
	for i := 1; i < accountCount; i++ {
		expectedBalances[i] = initialBalance
	}
	var canaryPayloads [transferCount][]byte
	var canaryKeys [transferCount][]byte
	for i := range transfers {
		account := 1 + i%(accountCount-1)
		from, to, owner, kind := account, 0, account-1, uint32(0x0001000b)
		if i%2 == 1 {
			from, to, owner, kind = 0, account, 0, assetMintType
			payload, id := executeActivityAssetPayload(owners[owner], byte(0x80+i))
			canaryPayloads[i] = payload
			canaryKeys[i] = moduleKVKey(1, append([]byte("asset:"), id[:]...))
		}
		amount := int64(i + 1)
		transfers[i] = transfer{owner: owner, kind: kind, sequence: sequences[owner],
			payload: supplyPayload(accounts[account], uint64(amount)), from: accounts[from], to: accounts[to], amount: amount}
		sequences[owner]++
		expectedBalances[from] -= amount
		expectedBalances[to] += amount
	}
	runTransfer := func(ctx sdk.Context, host *Host, index int) (Receipt, error) {
		tx := transfers[index]
		receipt, err := ExecuteActivity(WithSequence(ctx, firstSequence+uint64(index)), host, owners[tx.owner], tx.kind, tx.payload, tx.sequence)
		if err != nil {
			return Receipt{}, err
		}
		if receipt.ResultCode != 0 {
			return Receipt{}, fmt.Errorf("transfer %d returned %v", index, receipt.ResultCode)
		}
		if len(receipt.Encoded) == 0 || len(receipt.Transfers) != 1 {
			return Receipt{}, fmt.Errorf("transfer %d has no encoded receipt or transfer leg", index)
		}
		leg := receipt.Transfers[0]
		if leg.Asset != asset || leg.From != tx.from || leg.To != tx.to || !leg.Amount.Equal(sdk.NewInt(tx.amount)) {
			return Receipt{}, fmt.Errorf("transfer %d has unexpected leg %+v", index, leg)
		}
		return receipt, nil
	}
	var expected, actual [transferCount]Receipt
	for i := range transfers {
		branch, write := sequential.context(2, blockTime+1000, firstSequence+uint64(i)).CacheContext()
		receipt, err := runTransfer(branch, serialHost, i)
		if err != nil {
			t.Fatalf("sequential transfer %d: %v", i, err)
		}
		expected[i] = receipt
		write()
	}
	var attempts [transferCount]int
	var discarded [transferCount]bool
	var invariantFailures [transferCount]string
	var firstWave sync.WaitGroup
	firstWave.Add(workers)
	assetKey := moduleKVKey(1, append([]byte("asset:"), asset[:]...))
	deliver := func(ctx sdk.Context, _ abci.RequestDeliverTxV2, _ sdk.Tx, _ [32]byte) (response abci.ResponseDeliverTx) {
		defer func() {
			if recovered := recover(); recovered != nil {
				if _, ok := recovered.(occ.Abort); !ok {
					panic(recovered)
				}
			}
		}()
		i := ctx.TxIndex()
		attempts[i]++
		if i < workers && attempts[i] == 1 {
			ctx.KVStore(concurrent.key).Get([]byte{lastSequenceKey})
			firstWave.Done()
			firstWave.Wait()
		}
		fail := func(err error) abci.ResponseDeliverTx {
			return abci.ResponseDeliverTx{Code: 1, Log: err.Error()}
		}
		var aborted Receipt
		abortedHere := false
		if i%2 == 1 && !discarded[i] {
			parent := ctx.KVStore(concurrent.key)
			before := append([]byte(nil), parent.Get(assetKey)...)
			branch, _ := ctx.CacheContext()
			var err error
			aborted, err = runTransfer(branch, pooledHost, i)
			if err != nil {
				return fail(err)
			}
			if bytes.Equal(before, branch.KVStore(concurrent.key).Get(assetKey)) {
				invariantFailures[i] = "aborted mint did not change its 0x01 asset row"
			}
			canary, err := ExecuteActivity(WithSequence(branch, firstSequence+uint64(i)+1), pooledHost, owners[transfers[i].owner],
				assetRegisterType, canaryPayloads[i], transfers[i].sequence+1)
			if err != nil || canary.ResultCode != 0 {
				return fail(fmt.Errorf("aborted canary %d: result=%v error=%v", i, canary.ResultCode, err))
			}
			if branch.KVStore(concurrent.key).Get(canaryKeys[i]) == nil {
				invariantFailures[i] = "aborted incarnation did not write its unique 0x01 row"
			}
			if parent.Get(canaryKeys[i]) != nil || !bytes.Equal(before, parent.Get(assetKey)) {
				invariantFailures[i] = "discarded CacheContext leaked 0x01 writes to its parent"
			}
			discarded[i], abortedHere = true, true
		}
		branch, write := ctx.CacheContext()
		receipt, err := runTransfer(branch, pooledHost, i)
		if err != nil {
			return fail(err)
		}
		if abortedHere && !reflect.DeepEqual(aborted, receipt) {
			invariantFailures[i] = "aborted and retried incarnation receipts differ"
		}
		write()
		actual[i] = receipt
		return abci.ResponseDeliverTx{Data: receipt.Encoded}
	}
	requests := make([]*sdk.DeliverTxEntry, transferCount)
	for i := range requests {
		requests[i] = &sdk.DeliverTxEntry{AbsoluteIndex: i, Request: abci.RequestDeliverTxV2{Tx: []byte{byte(i)}}}
	}
	scheduler := tasks.NewScheduler(workers, tracing.NewTracingInfo(nil, false), deliver)
	responses, err := scheduler.ProcessAll(concurrent.context(2, blockTime+1000, firstSequence), requests)
	if err != nil || len(responses) != transferCount {
		t.Fatalf("OCC returned %d responses: %v", len(responses), err)
	}
	totalAttempts, totalDiscarded := 0, 0
	for i, response := range responses {
		totalAttempts += attempts[i]
		if response.Code != 0 {
			t.Fatalf("OCC transfer %d: %s", i, response.Log)
		}
		if invariantFailures[i] != "" {
			t.Fatalf("OCC transfer %d: %s", i, invariantFailures[i])
		}
		if discarded[i] != (i%2 == 1) {
			t.Fatalf("transfer %d: discarded=%t", i, discarded[i])
		}
		if discarded[i] {
			totalDiscarded++
			if concurrent.get(canaryKeys[i]) != nil {
				t.Fatalf("aborted 0x01 row %x survived", canaryKeys[i])
			}
		}
		if !reflect.DeepEqual(expected[i], actual[i]) || !bytes.Equal(expected[i].Encoded, response.Data) {
			t.Fatalf("transfer %d: sequential and OCC receipts differ", i)
		}
	}
	if totalAttempts <= transferCount || totalDiscarded != transferCount/2 {
		t.Fatalf("OCC did not exercise conflicts and aborts: attempts=%d discarded=%d", totalAttempts, totalDiscarded)
	}
	for i, account := range accounts {
		for _, view := range []struct {
			chain *testChain
			host  *Host
		}{{sequential, serialHost}, {concurrent, pooledHost}} {
			balance, err := view.host.ReadBalance(view.chain.context(2, blockTime+1000, firstSequence+transferCount), account, asset)
			if err != nil || !balance.Equal(sdk.NewInt(expectedBalances[i])) {
				t.Fatalf("account %d: balance=%v want=%d error=%v", i, balance, expectedBalances[i], err)
			}
		}
	}
	sequentialHash, concurrentHash := sequential.commit(), concurrent.commit()
	if !bytes.Equal(sequentialHash, concurrentHash) || !reflect.DeepEqual(sequential.dump(), concurrent.dump()) {
		t.Fatal("sequential and OCC store contents or hashes differ")
	}
	if !bytes.Equal(sequential.cms.LastCommitID().Hash, concurrent.cms.LastCommitID().Hash) {
		t.Fatal("sequential and OCC committed multistore hashes differ")
	}
	t.Logf("matched %d transfers across %d accounts; %d scheduler attempts and %d discarded incarnations", transferCount, accountCount, totalAttempts, totalDiscarded)
}
