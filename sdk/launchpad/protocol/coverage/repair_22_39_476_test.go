package coverage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/protocol/catalog"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestReviewedMeasurementCapturesAndReplaysCommittedProducerReceipt(t *testing.T) {
	producer := repair28StartProducer(t)
	client, err := DialMeasurementRPC(context.Background(), producer.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	bundles, err := catalog.Bundles()
	if err != nil {
		t.Fatal(err)
	}
	var deployments []catalog.Deployment
	var enabled []string
	var tokenABI abi.ABI
	for _, bundle := range bundles {
		if address, ok := producer.Addresses[bundle.Name]; ok {
			deployments = append(deployments, catalog.Deployment{ChainID: 125, Address: address, Family: bundle.Name, Version: bundle.ABIHash, Attested: true})
			enabled = append(enabled, bundle.Name)
		}
		if bundle.Name == "SidioraERC20" {
			deployments = append(deployments, catalog.Deployment{ChainID: 125, Address: producer.Addresses["USDL"], Family: bundle.Name, Version: bundle.ABIHash, Attested: true})
			tokenABI, err = abi.JSON(bytes.NewReader(bundle.ABI))
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	directReceipt, err := client.TransactionReceipt(context.Background(), producer.Direct)
	if err != nil {
		t.Fatal(err)
	}
	vaultRegistry, vaultABIs, vaultAddresses := sourceRegistry(t)
	vault := vaultAddresses["PECORVault"]
	vaultToken := vaultAddresses["SidioraERC20"]
	nativeDeposit := eventLog(t, vault, vaultABIs["PECORVault"].Events["NativeDeposit"], map[string]any{
		"depositor": producer.Account, "amount": big.NewInt(5), "newReserve": big.NewInt(5),
	})
	reserveSync := eventLog(t, vault, vaultABIs["PECORVault"].Events["ReservesSync"], map[string]any{
		"token": vaultToken, "oldReserves": big.NewInt(4), "newReserves": big.NewInt(4),
	})
	vaultResult := correlateInput(t, vaultRegistry, nativeDeposit, reserveSync)
	vaultResult.BlockHash = directReceipt.BlockHash
	previousDiagnostics := len(vaultResult.Diagnostics)
	vaultResult.reconcileReserves(context.Background(), &RPCSource{RPC: client, Registry: vaultRegistry})
	vaultDiagnostics := vaultResult.Diagnostics[previousDiagnostics:]
	if !containsDiagnostic(vaultDiagnostics, "unknown_vault_asset") || !containsDiagnostic(vaultDiagnostics, "vault_reserve_unavailable") {
		t.Fatalf("uncommitted vault state supplied reserve authority: %v", vaultDiagnostics)
	}
	getterInput, err := tokenABI.Pack("totalSupply")
	if err != nil {
		t.Fatal(err)
	}
	token := producer.Addresses["SidioraERC20"]
	getterOutput, err := client.CallContractAtHash(context.Background(), ethereum.CallMsg{To: &token, Data: getterInput}, directReceipt.BlockHash)
	if err != nil {
		t.Fatal(err)
	}
	zeroDigest := strings.Repeat("0", 64)
	manifest := MeasurementManifest{
		SchemaVersion:        1,
		ChainID:              125,
		SourceManifestSHA256: zeroDigest,
		InterfaceSHA256:      zeroDigest,
		Deployments:          deployments,
		EnabledFamilies:      enabled,
		PoolRegistry:         producer.Addresses["PoolRegistry"],
		Receipts: []ReceiptRequest{
			{TransactionHash: producer.Created, Scenario: 1, ExpectedCanonicalSHA256: zeroDigest, ExpectedProjectionSHA256: zeroDigest},
			{TransactionHash: producer.Funded, Scenario: 2, ExpectedCanonicalSHA256: zeroDigest, ExpectedProjectionSHA256: zeroDigest},
			{TransactionHash: producer.Direct, Scenario: 3, ExpectedCanonicalSHA256: zeroDigest, ExpectedProjectionSHA256: zeroDigest},
			{TransactionHash: producer.Routed, Scenario: 4, ExpectedCanonicalSHA256: zeroDigest, ExpectedProjectionSHA256: zeroDigest},
			{TransactionHash: producer.Forged, Scenario: 5, ExpectedCanonicalSHA256: zeroDigest, ExpectedProjectionSHA256: zeroDigest},
			{TransactionHash: producer.Orphan, Scenario: 6, ExpectedCanonicalSHA256: zeroDigest, ExpectedProjectionSHA256: zeroDigest},
			{TransactionHash: producer.Sold, Scenario: 7, ExpectedCanonicalSHA256: zeroDigest, ExpectedProjectionSHA256: zeroDigest},
		},
		Getters:                  []GetterRequest{{BlockHash: directReceipt.BlockHash, Address: token, Input: getterInput, ExpectedOutputSHA256: measurementDigest(getterOutput)}},
		ExpectedProjectionSHA256: zeroDigest,
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	reviewed := measurementDigest(raw)
	input, err := ReadMeasurementManifest(raw, reviewed)
	if err != nil {
		t.Fatal(err)
	}
	registry, _, err := MeasurementRegistry(input)
	if err != nil {
		t.Fatal(err)
	}
	capture, err := CaptureHistorical(context.Background(), client, input, reviewed)
	if err != nil {
		t.Fatal(err)
	}
	if len(capture.Receipts) != 7 || capture.Receipts[0].Receipt.TxHash != producer.Created || capture.Receipts[1].Receipt.TxHash != producer.Funded || capture.Receipts[5].Receipt.TxHash != producer.Orphan || capture.Receipts[6].Receipt.TxHash != producer.Sold || capture.Receipts[0].Header.Hash() != capture.Receipts[0].Receipt.BlockHash || len(capture.Getters) != 1 || !bytes.Equal(capture.Getters[0].Output, getterOutput) {
		t.Fatalf("committed capture differs: %+v", capture)
	}
	for _, record := range capture.Receipts {
		var observed []string
		for _, log := range record.Receipt.Logs {
			decoded := registry.Decode(125, *log)
			observed = append(observed, decoded.EmitterRole+":"+decoded.Signature)
		}
		t.Logf("committed scenario %d events: %v", record.Scenario, observed)
	}
	source := &RPCSource{RPC: client, Registry: registry, PoolRegistry: producer.Addresses["PoolRegistry"], Traces: &RPCTraceSource{RPC: client.Client()}, NativeReceipts: &RPCNativeReceiptSource{RPC: client}}
	for _, item := range []struct{ event, caller string }{
		{"MarketCreated", "SidioraFactory"},
		{"FeeFlow", "FeeAccumulator"},
		{"FeeFlow", "Account"},
		{"Swap", "SidioraPool"},
	} {
		selector, ok := TypedSelector(item.event)
		if !ok {
			t.Fatalf("reviewed %s emitter selector absent", item.event)
		}
		caller := producer.Addresses[item.caller]
		if item.caller == "Account" {
			caller = producer.Account
		}
		source.Permissions = append(source.Permissions, PermissionEpoch{ChainID: 125, Emitter: producer.Addresses["EventEmitter"], Caller: caller, Selector: selector})
	}
	orphanRecord := capture.Receipts[5]
	orphanTransaction := new(types.Transaction)
	if err := orphanTransaction.UnmarshalBinary(orphanRecord.SignedTransaction); err != nil {
		t.Fatal(err)
	}
	orphanResult, err := (&Correlator{Registry: registry, Source: source}).Correlate(context.Background(), orphanRecord.Header, orphanTransaction, orphanRecord.Receipt)
	if err != nil {
		t.Fatal(err)
	}
	if len(orphanResult.Effects) != 0 || !containsDiagnostic(orphanResult.Diagnostics, "mirror_native_correlation_conflict:FeeFlow") {
		t.Fatalf("unmatched reviewed fee mirror affected economics: %+v", orphanResult)
	}
	for _, item := range []struct {
		index int
		role  string
		event string
	}{
		{0, "SidioraFactory", "MarketCreated"},
		{2, "FeeAccumulator", "FeeRecorded"},
	} {
		record := capture.Receipts[item.index]
		transaction := new(types.Transaction)
		if err := transaction.UnmarshalBinary(record.SignedTransaction); err != nil {
			t.Fatal(err)
		}
		correlated, err := (&Correlator{Registry: registry, Source: source}).Correlate(context.Background(), record.Header, transaction, record.Receipt)
		if err != nil {
			t.Fatal(err)
		}
		matched := false
		for _, effect := range correlated.Effects {
			if effect.Role == item.role && effect.Event == item.event && len(effect.Companions) == 1 {
				matched = true
			}
		}
		if !matched {
			t.Fatalf("%s native effect lacks its authenticated companion: %+v", item.event, correlated.Diagnostics)
		}
		if item.index == 0 {
			creation, err := correlated.ResolveCreation(record.Receipt.TxHash)
			if err != nil || creation.Creator != producer.Account || creation.Pool != producer.Addresses["SidioraPool"] || creation.Token != producer.Addresses["SidioraERC20"] || creation.NFTID == nil || creation.NFTID.Sign() <= 0 {
				t.Fatalf("committed creation identity differs: %+v %v", creation, err)
			}
			for _, field := range []string{"nftId", "creator", "token", "pool", "optical"} {
				changed := *correlated
				changed.Effects = append([]Effect(nil), correlated.Effects...)
				for i := range changed.Effects {
					if changed.Effects[i].Kind == effectMarketAttachment {
						changed.Effects[i].Fields = copyFields(changed.Effects[i].Fields)
						changed.Effects[i].Fields[field] = "different field type"
					}
				}
				changed.seal = receiptSeal(&changed)
				if _, err := changed.ResolveCreation(record.Receipt.TxHash); err == nil {
					t.Fatalf("creation with noncanonical %s accepted", field)
				}
			}
		}
	}
	soldRecord := capture.Receipts[6]
	soldTransaction := new(types.Transaction)
	if err := soldTransaction.UnmarshalBinary(soldRecord.SignedTransaction); err != nil {
		t.Fatal(err)
	}
	soldResult, err := (&Correlator{Registry: registry, Source: source}).Correlate(context.Background(), soldRecord.Header, soldTransaction, soldRecord.Receipt)
	if err != nil {
		t.Fatal(err)
	}
	matchedSale := false
	var saleFill Effect
	for _, effect := range soldResult.Effects {
		if effect.Kind == effectCoreFill && effect.Role == roleEventEmitter && effect.Fields["isBuy"] == false && len(effect.Companions) == 1 && effect.Companions[0].Emitter == producer.Addresses["Router"] {
			matchedSale = true
			saleFill = effect
		}
	}
	if !matchedSale {
		t.Fatalf("committed sale lacks its routed pool fill: %v", soldResult.Diagnostics)
	}
	unregistered := *source
	unregistered.PoolRegistry = producer.Addresses["SidioraERC20"]
	withoutPool, err := (&Correlator{Registry: registry, Source: &unregistered}).Correlate(context.Background(), soldRecord.Header, soldTransaction, soldRecord.Receipt)
	if err != nil {
		t.Fatal(err)
	}
	if !containsDiagnostic(withoutPool.Diagnostics, "pool_registration_unavailable") {
		t.Fatalf("sale with an unregistered pool acquired economic authority: %v", withoutPool.Diagnostics)
	}
	withoutPermission := *source
	withoutPermission.Permissions = nil
	if _, _, _, err := withoutPermission.CallerAt(context.Background(), saleFill.Identity, saleFill.Position); err == nil {
		t.Fatal("sale mirror without a selector permission acquired a caller")
	}
	replay, err := ReplayHistorical(context.Background(), source, input, capture)
	if err != nil {
		t.Fatal(err)
	}
	if !replay.ByteIdentical || replay.FirstSHA256 != replay.SecondSHA256 || replay.ReceiptCount != 7 || replay.Scenarios[1] != 1 || replay.Scenarios[6] != 1 || replay.Scenarios[7] != 1 || replay.UniqueRawLogs == 0 {
		t.Fatalf("committed receipt replay differs: %+v", replay)
	}
	hasBlocker := func(prefix string) bool {
		for _, blocker := range replay.Blockers {
			if strings.HasPrefix(blocker, prefix) {
				return true
			}
		}
		return false
	}
	if !hasBlocker("scenario_1_expected_canonical_mismatch:") || !hasBlocker("scenario_6_expected_canonical_mismatch:") || !hasBlocker("expected_projection_mismatch") || !hasBlocker("missing_reviewed_scenario:12") {
		t.Fatalf("incomplete reviewed corpus diagnostics differ: %v", replay.Blockers)
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	burst, err := MeasureDecoderBurst(cancelled, registry, capture)
	if err != context.Canceled || burst == nil || burst.UniqueHistoricalLogs == 0 {
		t.Fatalf("canceled decoder replay: burst=%+v err=%v", burst, err)
	}
	transferCapture := &HistoricalCapture{SchemaVersion: 1, ChainID: 125, ManifestSHA256: reviewed, Receipts: []CapturedReceipt{capture.Receipts[1]}}
	transferCapture.seal = captureSeal(transferCapture)
	deadline, stop := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop()
	partial, err := MeasureDecoderBurst(deadline, registry, transferCapture)
	if err != context.DeadlineExceeded || partial == nil || partial.Attempted == 0 || partial.Matched != partial.Attempted || partial.Decoded != partial.Attempted || partial.Mismatched != 0 {
		t.Fatalf("bounded decoder replay: burst=%+v err=%v", partial, err)
	}
	t.Logf("bounded decoder replay matched %d committed log observations", partial.Matched)
	getterSource := &RPCSource{RPC: client, Registry: registry, PoolRegistry: producer.Addresses["PoolRegistry"]}
	if _, err := getterSource.ReserveAt(context.Background(), directReceipt.BlockHash, token, token); err == nil {
		t.Fatal("token admitted as a vault reserve")
	}
	if _, err := getterSource.WrappedNativeAt(context.Background(), directReceipt.BlockHash, token); err == nil {
		t.Fatal("token admitted as a wrapped asset vault")
	}
	if _, err := getterSource.call(context.Background(), directReceipt.BlockHash, token, "SidioraERC20", "balanceOf", producer.Account); err != nil {
		t.Fatalf("pinned reviewed token balance unavailable: %v", err)
	}
	for _, item := range []struct {
		block   common.Hash
		family  string
		method  string
		address common.Address
	}{
		{common.Hash{}, "SidioraERC20", "totalSupply", token},
		{common.HexToHash("0x9876"), "SidioraERC20", "totalSupply", token},
		{directReceipt.BlockHash, "UnknownFamily", "totalSupply", token},
		{directReceipt.BlockHash, "SidioraNFT", "name", token},
		{directReceipt.BlockHash, "SidioraERC20", "unknownMethod", token},
	} {
		if _, err := getterSource.call(context.Background(), item.block, item.address, item.family, item.method); err == nil {
			t.Fatal("unreviewed pinned getter admitted")
		}
	}
	if _, err := getterSource.PoolAt(context.Background(), directReceipt.BlockHash, common.HexToHash("0xff"), common.Address{}); err == nil {
		t.Fatal("non-address pool identity admitted")
	}
	var nonAddressID common.Hash
	nonAddressID[0] = 1
	if _, err := getterSource.PoolAt(context.Background(), directReceipt.BlockHash, nonAddressID, common.Address{}); err == nil {
		t.Fatal("pool identity with a non-address prefix admitted")
	}
	zeroPool := common.BytesToHash(make([]byte, 20))
	if _, err := getterSource.PoolAt(context.Background(), directReceipt.BlockHash, zeroPool, common.Address{}); err == nil {
		t.Fatal("zero pool identity admitted")
	}
	if _, err := getterSource.PoolAt(context.Background(), directReceipt.BlockHash, common.Hash{}, common.HexToAddress("0xdead")); err == nil {
		t.Fatal("unregistered pool admitted")
	}
	if _, err := getterSource.PoolAt(context.Background(), directReceipt.BlockHash, common.HexToHash("0x1234"), producer.Addresses["SidioraPool"]); err == nil {
		t.Fatal("different pool id admitted")
	}
	if _, _, _, err := getterSource.CallerAt(context.Background(), Identity{ChainID: 1}, Position{}); err == nil {
		t.Fatal("caller admitted on another chain")
	}
	if _, _, _, err := getterSource.CallerAt(context.Background(), Identity{ChainID: 125, TransactionHash: common.HexToHash("0x9876")}, Position{}); err == nil {
		t.Fatal("unknown caller transaction admitted")
	}
	if _, err := (&RPCTraceSource{RPC: client.Client()}).TraceTransactionInBlock(context.Background(), common.Hash{}, producer.Direct); err == nil {
		t.Fatal("unbound execution trace admitted")
	}
	if _, err := getterSource.call(context.Background(), common.HexToHash("0x9876"), token, "SidioraERC20", "totalSupply"); err == nil {
		t.Fatal("uncommitted header supplied a pinned getter")
	}
	if _, err := getterSource.call(context.Background(), directReceipt.BlockHash, token, "SidioraERC20", "ownerOf", big.NewInt(0)); err == nil {
		t.Fatal("unreviewed token method supplied a getter")
	}
	readVariant := func(value MeasurementManifest) (MeasurementManifest, string) {
		t.Helper()
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		hash := measurementDigest(encoded)
		selected, err := ReadMeasurementManifest(encoded, hash)
		if err != nil {
			t.Fatal(err)
		}
		return selected, hash
	}
	wrongGetter := manifest
	wrongGetter.Getters = append([]GetterRequest(nil), manifest.Getters...)
	wrongGetter.Getters[0].ExpectedOutputSHA256 = zeroDigest
	selected, hash := readVariant(wrongGetter)
	if _, err := CaptureHistorical(context.Background(), client, selected, hash); err == nil || !strings.Contains(err.Error(), "getter expected output differs") {
		t.Fatalf("different pinned getter output accepted: %v", err)
	}
	unknownGetter := manifest
	unknownGetter.Getters = append([]GetterRequest(nil), manifest.Getters...)
	unknownGetter.Getters[0].BlockHash = common.HexToHash("0x9876")
	selected, hash = readVariant(unknownGetter)
	if _, err := CaptureHistorical(context.Background(), client, selected, hash); err == nil || !strings.Contains(err.Error(), "eth_getBlockByHash") {
		t.Fatalf("uncommitted getter block accepted: %v", err)
	}
	revertingGetter := manifest
	revertingGetter.Getters = append([]GetterRequest(nil), manifest.Getters...)
	nftABI, err := catalog.Bundles()
	if err != nil {
		t.Fatal(err)
	}
	for _, bundle := range nftABI {
		if bundle.Name == "SidioraNFT" {
			parsed, err := abi.JSON(bytes.NewReader(bundle.ABI))
			if err != nil {
				t.Fatal(err)
			}
			revertingGetter.Getters[0].Input, err = parsed.Pack("ownerOf", big.NewInt(0))
			if err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if len(revertingGetter.Getters[0].Input) < 4 {
		t.Fatal("reviewed NFT owner getter absent")
	}
	revertingGetter.Getters[0].Address = producer.Addresses["SidioraNFT"]
	selected, hash = readVariant(revertingGetter)
	if _, err := CaptureHistorical(context.Background(), client, selected, hash); err == nil || !strings.Contains(err.Error(), "eth_call_at_hash") {
		t.Fatalf("reverting historical getter accepted: %v", err)
	}
	unknownReceipt := manifest
	unknownReceipt.Receipts = append([]ReceiptRequest(nil), manifest.Receipts...)
	unknownReceipt.Receipts[0].TransactionHash = common.HexToHash("0x9876")
	selected, hash = readVariant(unknownReceipt)
	if _, err := CaptureHistorical(context.Background(), client, selected, hash); err == nil || !strings.Contains(err.Error(), "eth_getTransactionByHash") {
		t.Fatalf("unknown signed transaction accepted: %v", err)
	}
	duplicateReceipt := manifest
	duplicateReceipt.Receipts = append([]ReceiptRequest(nil), manifest.Receipts...)
	duplicateReceipt.Receipts[1].TransactionHash = duplicateReceipt.Receipts[0].TransactionHash
	rawDuplicate, err := json.Marshal(duplicateReceipt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReadMeasurementManifest(rawDuplicate, measurementDigest(rawDuplicate)); err == nil {
		t.Fatal("duplicate reviewed receipt admitted")
	}
	copyCapture := func() *HistoricalCapture {
		t.Helper()
		selected := *capture
		selected.Receipts = append([]CapturedReceipt(nil), capture.Receipts...)
		selected.Getters = append([]CapturedGetter(nil), capture.Getters...)
		return &selected
	}
	duplicate := copyCapture()
	duplicate.Receipts = append(duplicate.Receipts, duplicate.Receipts[0])
	duplicate.seal = captureSeal(duplicate)
	duplicatedReplay, err := ReplayHistorical(context.Background(), source, input, duplicate)
	if err != nil || !duplicatedReplay.ByteIdentical || duplicatedReplay.UniqueRawLogs != replay.UniqueRawLogs || duplicatedReplay.Scenarios[1] != 2 {
		t.Fatalf("duplicate historical observation changed raw identity: %+v %v", duplicatedReplay, err)
	}
	if burst, err := MeasureDecoderBurst(cancelled, registry, duplicate); err != context.Canceled || burst == nil || burst.UniqueHistoricalLogs != replay.UniqueRawLogs {
		t.Fatalf("duplicate decoder corpus changed raw log count: %+v %v", burst, err)
	}
	containsBlocker := func(result *ReplayMeasurement, prefix string) bool {
		for _, blocker := range result.Blockers {
			if strings.HasPrefix(blocker, prefix) {
				return true
			}
		}
		return false
	}
	missingBaseline := copyCapture()
	missingBaseline.Receipts = []CapturedReceipt{capture.Receipts[2]}
	missingBaseline.seal = captureSeal(missingBaseline)
	baselineReplay, err := ReplayHistorical(context.Background(), source, input, missingBaseline)
	if err != nil || !containsBlocker(baselineReplay, "native_projection:") {
		t.Fatalf("sale without prior native balance lost its baseline diagnostic: %v %v", baselineReplay, err)
	}
	missingPool := *source
	missingPool.PoolRegistry = producer.Addresses["SidioraERC20"]
	poolReplay, err := ReplayHistorical(context.Background(), &missingPool, input, capture)
	if err != nil || !containsBlocker(poolReplay, "creation_resolution:") {
		t.Fatalf("creation without registered pool acquired attachment: %v %v", poolReplay, err)
	}
	withoutTokenDeployment := manifest
	withoutTokenDeployment.Deployments = append([]catalog.Deployment(nil), manifest.Deployments...)
	for i, deployment := range withoutTokenDeployment.Deployments {
		if deployment.Address == producer.Addresses["USDL"] {
			withoutTokenDeployment.Deployments = append(withoutTokenDeployment.Deployments[:i], withoutTokenDeployment.Deployments[i+1:]...)
			break
		}
	}
	selected, hash = readVariant(withoutTokenDeployment)
	missingDeployment := copyCapture()
	missingDeployment.ManifestSHA256 = hash
	missingDeployment.seal = captureSeal(missingDeployment)
	selectorReplay, err := ReplayHistorical(context.Background(), source, selected, missingDeployment)
	if err != nil || !containsBlocker(selectorReplay, "transaction_selector:selector origin unregistered") {
		t.Fatalf("unregistered settlement selector gained authority: %v %v", selectorReplay, err)
	}
	badGetter := copyCapture()
	badGetter.Getters[0].Output = []byte{1}
	badGetter.seal = captureSeal(badGetter)
	getterReplay, err := ReplayHistorical(context.Background(), source, input, badGetter)
	if err != nil {
		t.Fatal(err)
	}
	getterBlocked := false
	for _, blocker := range getterReplay.Blockers {
		if strings.HasPrefix(blocker, "getter_selector:") {
			getterBlocked = true
		}
	}
	if !getterBlocked {
		t.Fatal("noncanonical captured getter return admitted")
	}
	badTransaction := copyCapture()
	badTransaction.Receipts[0].SignedTransaction = []byte{1}
	badTransaction.seal = captureSeal(badTransaction)
	if _, err := ReplayHistorical(context.Background(), source, input, badTransaction); err == nil {
		t.Fatal("invalid captured signed transaction admitted")
	}
	if _, err := ReplayHistorical(cancelled, source, input, capture); err != context.Canceled {
		t.Fatalf("canceled historical replay continued: %v", err)
	}
	if _, err := CaptureHistorical(cancelled, client, input, reviewed); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled historical capture continued: %v", err)
	}
	alteredReceipt := copyCapture()
	changedReceipt := *alteredReceipt.Receipts[0].Receipt
	changedReceipt.Status = types.ReceiptStatusFailed
	alteredReceipt.Receipts[0].Receipt = &changedReceipt
	alteredReceipt.seal = captureSeal(alteredReceipt)
	if _, err := ReplayHistorical(context.Background(), source, input, alteredReceipt); err == nil {
		t.Fatal("changed committed receipt entered historical replay")
	}

	changed := input
	changed.ChainID = 1
	if _, _, err := MeasurementRegistry(changed); err == nil {
		t.Fatal("modified reviewed input admitted")
	}
	client.Close()
	producer.stop()
	capture.Receipts[0].Scenario = 2
	if _, err := ReplayHistorical(context.Background(), source, input, capture); err == nil {
		t.Fatal("modified committed capture admitted")
	}
	if _, err := CaptureHistorical(context.Background(), client, input, zeroDigest); err == nil {
		t.Fatal("unreviewed capture identity admitted")
	}
}
