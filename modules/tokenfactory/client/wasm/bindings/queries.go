package bindings

import "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"

type PaxTokenFactoryQuery struct {
	// queries the tokenfactory authority metadata
	DenomAuthorityMetadata *types.QueryDenomAuthorityMetadataRequest `json:"denom_authority_metadata,omitempty"`
	// queries the tokenfactory denoms from a creator
	DenomsFromCreator *types.QueryDenomsFromCreatorRequest `json:"denoms_from_creator,omitempty"`
	DenomPaused       *types.QueryDenomPausedRequest       `json:"denom_paused,omitempty"`
}

// QueryDenomPausedResponse is the response consumed by wasm callers, sharing
// the keeper's paused and issuer_did wire fields without a second representation.
type QueryDenomPausedResponse = types.QueryDenomPausedResponse

// Validate rejects ambiguous query variants before a handler can select one.
// An empty query retains the router's ErrUnknownPaxTokenFactoryQuery response.
func (q PaxTokenFactoryQuery) Validate() error {
	variants := 0
	if q.DenomAuthorityMetadata != nil {
		variants++
	}
	if q.DenomsFromCreator != nil {
		variants++
	}
	if q.DenomPaused != nil {
		variants++
	}
	if variants > 1 {
		return types.ErrParsingPaxTokenFactoryQuery
	}
	return nil
}
