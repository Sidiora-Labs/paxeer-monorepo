package rolecheck

type WaitingStatement struct {
	Object    string
	Privilege string
	Message   string
}

var WaitingStatements = map[string]WaitingStatement{
	"internal.auth.Identity.ownerMutation.0":    {Object: "users.identity_owner_requests", Privilege: "DELETE", Message: "permission denied for table identity_owner_requests"},
	"internal.auth.Identity.CollectExpired.0.4": {Object: "users.identity_challenge_requests", Privilege: "DELETE", Message: "permission denied for table identity_challenge_requests"},
	"internal.auth.Identity.CollectExpired.0.5": {Object: "users.identity_owner_requests", Privilege: "DELETE", Message: "permission denied for table identity_owner_requests"},
	"internal.auth.Identity.CollectExpired.0.6": {Object: "users.identity_wallet_links", Privilege: "DELETE", Message: "permission denied for table identity_wallet_links"},
	"auth.identity-challenge-request-delete":    {Object: "users.identity_challenge_requests", Privilege: "DELETE", Message: "permission denied for table identity_challenge_requests"},
}
