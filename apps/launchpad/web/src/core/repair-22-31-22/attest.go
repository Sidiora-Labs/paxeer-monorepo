package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/capabilities"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
)

type pinnedReadKey struct {
	method  string
	address common.Address
	slot    common.Hash
	data    string
}

type pinnedReadResult struct {
	value []byte
	err   error
}

type concurrentPinnedReader struct {
	*capabilities.PinnedRPCReader
	block common.Hash
	reads map[pinnedReadKey]pinnedReadResult
}

func prefetchCurrentDeployment(ctx context.Context, reader *capabilities.PinnedRPCReader, manifest deployment.Manifest, block common.Hash) (*concurrentPinnedReader, error) {
	reads := make(map[pinnedReadKey]pinnedReadResult)
	for _, role := range manifest.Roles {
		address := common.HexToAddress(role.Address)
		if role.Kind != "precompile" {
			reads[pinnedReadKey{method: "code", address: address}] = pinnedReadResult{}
		}
		if role.ImplementationAddress != "" {
			reads[pinnedReadKey{method: "code", address: common.HexToAddress(role.ImplementationAddress)}] = pinnedReadResult{}
		}
		if role.Kind == "erc1967" {
			reads[pinnedReadKey{method: "storage", address: address, slot: deployment.ImplementationSlot}] = pinnedReadResult{}
		}
	}
	for role, signatures := range map[string][]string{
		"PoolBeacon":     {"implementation()", "owner()", "pendingOwner()"},
		"ProtocolConfig": {"minFeeBps()", "maxFeeBps()", "baseFeeBps()", "protocolFeeBps()", "virtualUsdlDefault()", "virtualTokenDefault()", "feeDecayRate()", "volatilityWeight()", "concentrationWeight()", "creationFee()", "usdlAddress()"},
		"USDL":           {"decimals()"},
	} {
		for _, signature := range signatures {
			reads[pinnedReadKey{method: "call", address: common.HexToAddress(manifest.Roles[role].Address), data: string(crypto.Keccak256([]byte(signature))[:4])}] = pinnedReadResult{}
		}
	}
	var group sync.WaitGroup
	var mutex sync.Mutex
	keys := make([]pinnedReadKey, 0, len(reads))
	for key := range reads {
		keys = append(keys, key)
	}
	for _, key := range keys {
		group.Add(1)
		go func(key pinnedReadKey) {
			defer group.Done()
			var result pinnedReadResult
			switch key.method {
			case "code":
				result.value, result.err = reader.CodeAtHash(ctx, key.address, block)
			case "storage":
				result.value, result.err = reader.StorageAtHash(ctx, key.address, key.slot, block)
			case "call":
				result.value, result.err = reader.CallContractAtHash(ctx, ethereum.CallMsg{To: &key.address, Data: []byte(key.data)}, block)
			}
			mutex.Lock()
			reads[key] = result
			mutex.Unlock()
		}(key)
	}
	group.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &concurrentPinnedReader{reader, block, reads}, nil
}

func (reader *concurrentPinnedReader) read(ctx context.Context, block common.Hash, key pinnedReadKey) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result, ok := reader.reads[key]
	if block != reader.block || !ok {
		return nil, fmt.Errorf("read outside prefetched current deployment: %s %s", key.method, key.address)
	}
	return append([]byte(nil), result.value...), result.err
}

func (reader *concurrentPinnedReader) CodeAtHash(ctx context.Context, address common.Address, block common.Hash) ([]byte, error) {
	return reader.read(ctx, block, pinnedReadKey{method: "code", address: address})
}

func (reader *concurrentPinnedReader) StorageAtHash(ctx context.Context, address common.Address, slot, block common.Hash) ([]byte, error) {
	return reader.read(ctx, block, pinnedReadKey{method: "storage", address: address, slot: slot})
}

func (reader *concurrentPinnedReader) CallContractAtHash(ctx context.Context, call ethereum.CallMsg, block common.Hash) ([]byte, error) {
	if call.To == nil || call.From != (common.Address{}) || call.Gas != 0 || call.GasPrice != nil || call.GasFeeCap != nil || call.GasTipCap != nil || call.Value != nil || len(call.AccessList) != 0 {
		return nil, fmt.Errorf("call outside prefetched current deployment")
	}
	return reader.read(ctx, block, pinnedReadKey{method: "call", address: *call.To, data: string(call.Data)})
}

func authorityFromCurrentDeployment() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	endpoint := os.Getenv("LAUNCHPAD_AUTHORITY_TEST_RPC")
	if endpoint == "" {
		endpoint = "https://api1.mainnet-beta.paxeer.network"
	}
	rpcURL, err := url.Parse(endpoint)
	if err != nil {
		return err
	}
	if rpcURL.Scheme != "https" || rpcURL.Host == "" || rpcURL.User != nil || rpcURL.RawQuery != "" || rpcURL.Fragment != "" {
		return fmt.Errorf("reviewed HTTPS RPC endpoint required")
	}
	rpcURL.Scheme = "wss"
	client, err := rpc.DialContext(ctx, rpcURL.String())
	if err != nil {
		return err
	}
	defer client.Close()
	reader, err := capabilities.NewPinnedRPCReader(client)
	if err != nil {
		return err
	}
	number, err := reader.BlockNumber(ctx)
	if err != nil {
		return err
	}
	var head struct {
		Hash common.Hash `json:"hash"`
	}
	if err = client.CallContext(ctx, &head, "eth_getBlockByNumber", fmt.Sprintf("0x%x", number), false); err != nil {
		return err
	}
	manifest := deployment.Current()
	readContext, cancelReads := context.WithTimeout(ctx, 10*time.Second)
	defer cancelReads()
	concurrentReader, err := prefetchCurrentDeployment(readContext, reader, manifest, head.Hash)
	if err != nil {
		return fmt.Errorf("current deployment reads: %w", err)
	}
	policy := deployment.Policy{Now: time.Now().UTC(), MaxAge: time.Minute, Timeout: 10 * time.Second}
	attestation, err := deployment.Attest(readContext, concurrentReader, manifest, head.Hash, policy)
	if err != nil {
		return fmt.Errorf("current deployment attestation: %w (issues=%v)", err, attestation.Issues)
	}
	if err = attestation.ValidateFor(manifest, policy); err != nil {
		return err
	}
	schema, err := os.ReadFile("interfaces/v2/types.gen.go")
	if err != nil {
		return err
	}
	schemaHash := sha256.Sum256(schema)
	engine, err := capabilities.NewPreflightEngine(reader, manifest, manifest.Roles["PoolBeacon"].ImplementationHash, hex.EncodeToString(schemaHash[:]), time.Minute, 10*time.Second)
	if err != nil {
		return err
	}
	endpoints := deployment.PublicEndpoints{RPC: endpoint, Explorer: "https://explorer.paxeer.network", AllowedRPCs: []string{endpoint}, AllowedExplorers: []string{"https://explorer.paxeer.network"}}
	export, err := deployment.ExportRevision(manifest, attestation, policy, endpoints, engine.Revision(), manifest.Addresses(), manifest.Addresses())
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(struct {
		SchemaVersion int                              `json:"schemaVersion"`
		SerialExport  deployment.SerialExport          `json:"serialExport"`
		Attestation   deployment.ActivationAttestation `json:"attestation"`
		CommittedHead map[string]string                `json:"committedHead"`
	}{2, export, attestation, map[string]string{"number": fmt.Sprint(attestation.BlockNumber), "hash": attestation.BlockHash}})
	if err != nil {
		return err
	}
	var response interfaces.ConnectedNetworkAuthorityResponse
	if err = json.Unmarshal(encoded, &response); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(response)
}

func main() {
	if err := authorityFromCurrentDeployment(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
