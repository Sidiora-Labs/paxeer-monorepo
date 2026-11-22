package receipt

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum/common"
)

//go:embed deployment_source.json
var approvedDeploymentSource []byte

// historicalEmitterEpochs restores roles omitted by the normalized historical
// manifest from its exact, approved source document. Code identities are never
// borrowed from the successor's native surfaces.
func historicalEmitterEpochs(manifest deployment.Manifest, catalogJSON []byte) ([]RPCCodeEpoch, error) {
	if manifest.ActivationHeight >= deployment.Current().ActivationHeight {
		return nil, nil
	}
	sum := sha256.Sum256(approvedDeploymentSource)
	if hex.EncodeToString(sum[:]) != manifest.SourceManifestSHA256 {
		return nil, qualificationRefusal("Treasury", valueErc1967, common.Address{}, "coverage")
	}
	var source struct {
		Deployment struct {
			Records map[string]struct {
				Address     string `json:"address"`
				RuntimeHash string `json:"runtimeHash"`
				Status      string `json:"status"`
				Height      uint64 `json:"blockNumber"`
			} `json:"records"`
		} `json:"_deployment"`
	}
	if json.Unmarshal(approvedDeploymentSource, &source) != nil {
		return nil, qualificationRefusal("Treasury", valueErc1967, common.Address{}, "coverage")
	}
	if _, exists := manifest.Roles["Treasury"]; exists {
		return nil, nil
	}
	proxy, ok := source.Deployment.Records["Treasury_proxy"]
	implementation, implOK := source.Deployment.Records["Treasury_impl"]
	if !ok || !implOK || proxy.Status != "confirmed" || implementation.Status != "confirmed" || proxy.Height > manifest.ActivationHeight || implementation.Height > manifest.ActivationHeight {
		return nil, qualificationRefusal("Treasury", valueErc1967, common.HexToAddress(proxy.Address), "coverage")
	}
	identity := deployment.Role{Address: proxy.Address, Kind: valueErc1967, RuntimeHash: proxy.RuntimeHash, ImplementationAddress: implementation.Address, ImplementationHash: implementation.RuntimeHash}
	if !validRPCIdentity(identity) {
		return nil, qualificationRefusal("Treasury", identity.Kind, common.HexToAddress(identity.Address), "identity")
	}
	var catalog decode.Catalog
	if json.Unmarshal(catalogJSON, &catalog) != nil || catalog.ManifestSHA256 != manifest.SourceManifestSHA256 {
		return nil, qualificationRefusal("Treasury", identity.Kind, common.HexToAddress(identity.Address), "template")
	}
	var revision string
	for _, contract := range catalog.Contracts {
		if contract.Name == "Treasury" {
			if revision != "" {
				return nil, qualificationRefusal("Treasury", identity.Kind, common.HexToAddress(identity.Address), "template")
			}
			revision = contract.ABIRevision
		}
	}
	// A trimmed runtime catalog intentionally leaves this non-manifest role
	// opaque. Only catalogs that decode Treasury require its source epoch.
	if revision == "" {
		return nil, nil
	}
	return []RPCCodeEpoch{{Deployment: decode.Deployment{Address: common.HexToAddress(identity.Address), Role: "Treasury", ABIRevision: revision, From: decode.Order{Block: manifest.ActivationHeight}}, Identity: identity}}, nil
}
