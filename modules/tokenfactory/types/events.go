package types

const AttributePaused = "paused"

// event types
// nolint
const (
	EventTypeSetIssuerDid      = "set_issuer_did"
	AttributeIssuerDid         = "issuer_did"
	AttributeAmount            = "amount"
	AttributeCreator           = "creator"
	AttributeSubdenom          = "subdenom"
	AttributeNewTokenDenom     = "new_token_denom" //nolint:gosec
	AttributeUpdatedTokenDenom = "updated_token_denom"
	AttributeMintToAddress     = "mint_to_address"
	AttributeBurnFromAddress   = "burn_from_address"
	AttributeDenom             = "denom"
	AttributeNewAdmin          = "new_admin"
	AttributeDenomMetadata     = "denom_metadata"
	AttributeAllowList         = "denom_allow_list"
)
