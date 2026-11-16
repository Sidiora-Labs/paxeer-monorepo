package rolecheck

// WaitingStatement names the object and authority required by a runtime statement.
type WaitingStatement struct {
	Statement string
	Object    string
	Right     string
	Message   string
}

var Waiting = []WaitingStatement{
	{"app/app.go:socialConnectedRuntime.stageSafetyOutbox:1", "chat.safety_outbox", "UPDATE (row lock)", "permission denied for table safety_outbox"},
	{"app/app.go:socialConnectedRuntime.stageSafetyOutbox:2", "chat.safety_outbox.published_at", "UPDATE", "permission denied for table safety_outbox"},
	{"db/sqlcdb/query.sql.go:Queries.GetPoolCreator:1", "indexer.pools", "SELECT", "permission denied for table pools"},
	{"interactions/contract.go:PGPolicy.profiles:1", "users.account_state", "USAGE on schema users; SELECT on wallet, profile, deleted, revision", "permission denied for schema users"},
	{"interactions/erasure.go:Store.EraseBatch:3", "chat.interaction_follows", "UPDATE (row lock)", "permission denied for table interaction_follows"},
	{"interactions/erasure.go:Store.EraseBatch:7", "chat.interaction_reactions", "UPDATE (row lock)", "permission denied for table interaction_reactions"},
	{"interactions/erasure.go:Store.EraseBatch:9", "chat.interaction_notification_intents", "DELETE; UPDATE (row lock)", "permission denied for table interaction_notification_intents"},
	{"interactions/erasure.go:Store.EraseBatch:10", "chat.interaction_callout_refs", "DELETE; UPDATE (row lock)", "permission denied for table interaction_callout_refs"},
	{"interactions/erasure.go:Store.EraseBatch:11", "chat.interaction_mutations", "DELETE; UPDATE (row lock)", "permission denied for table interaction_mutations"},
	{"interactions/erasure.go:Store.EraseBatch:13", "chat.interaction_blocks", "UPDATE (row lock)", "permission denied for table interaction_blocks"},
	{"interactions/erasure.go:Store.EraseBatch:14", "chat.interaction_mutes", "UPDATE (row lock)", "permission denied for table interaction_mutes"},
	{"interactions/policy.go:PGPolicy.Check:9", "chat.interaction_callout_authority", "UPDATE (row lock)", "permission denied for table interaction_callout_authority"},
}
