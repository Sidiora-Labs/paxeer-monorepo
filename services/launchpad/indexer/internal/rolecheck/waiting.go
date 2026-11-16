package rolecheck

// WaitingStatement records a statement whose connected plan is refused by the
// current schema or grants.
type WaitingStatement struct {
	StatementID string
	Object      string
	Privilege   string
	Message     string
}

// Waiting is kept separate from RuntimeRoleStatements so each pending object
// is explicit and independently reviewable.
var Waiting = []WaitingStatement{
	{StatementID: "S010", Object: "indexer.indexer_block_completeness_v1", Privilege: "SELECT", Message: "ERROR: permission denied for view indexer_block_completeness_v1 (SQLSTATE 42501)"},
	{StatementID: "S011", Object: "indexer.indexer_raw_logs_v1", Privilege: "SELECT", Message: "ERROR: permission denied for view indexer_raw_logs_v1 (SQLSTATE 42501)"},
	{StatementID: "S012", Object: "indexer.indexer_block_completeness_v1", Privilege: "SELECT", Message: "ERROR: permission denied for view indexer_block_completeness_v1 (SQLSTATE 42501)"},
	{StatementID: "S013", Object: "indexer.indexer_block_completeness_v1", Privilege: "SELECT", Message: "ERROR: permission denied for view indexer_block_completeness_v1 (SQLSTATE 42501)"},
	{StatementID: "S021", Object: "indexer.backfill_jobs", Privilege: "UPDATE", Message: "ERROR: permission denied for table backfill_jobs (SQLSTATE 42501)"},
	{StatementID: "S022", Object: "indexer.backfill_jobs", Privilege: "UPDATE", Message: "ERROR: permission denied for table backfill_jobs (SQLSTATE 42501)"},
	{StatementID: "S023", Object: "indexer.backfill_jobs", Privilege: "SELECT", Message: "ERROR: permission denied for table backfill_jobs (SQLSTATE 42501)"},
	{StatementID: "S024", Object: "indexer.cursors", Privilege: "SELECT", Message: "ERROR: permission denied for table cursors (SQLSTATE 42501)"},
	{StatementID: "S025", Object: "indexer.pools.home_chain", Privilege: "SELECT", Message: "ERROR: column \"home_chain\" does not exist (SQLSTATE 42703)"},
	{StatementID: "S026", Object: "indexer.pools.home_chain", Privilege: "SELECT", Message: "ERROR: column \"home_chain\" does not exist (SQLSTATE 42703)"},
	{StatementID: "S027", Object: "indexer.pools.home_chain", Privilege: "SELECT", Message: "ERROR: column \"home_chain\" does not exist (SQLSTATE 42703)"},
	{StatementID: "S031", Object: "indexer.backfill_jobs", Privilege: "INSERT", Message: "ERROR: permission denied for table backfill_jobs (SQLSTATE 42501)"},
	{StatementID: "S041", Object: "indexer.backfill_jobs", Privilege: "UPDATE", Message: "ERROR: permission denied for table backfill_jobs (SQLSTATE 42501)"},
	{StatementID: "S042", Object: "indexer.cursors", Privilege: "INSERT, UPDATE", Message: "ERROR: permission denied for table cursors (SQLSTATE 42501)"},
	{StatementID: "S043", Object: "security.operator_sessions", Privilege: "USAGE", Message: "ERROR: permission denied for schema security (SQLSTATE 42501)"},
	{StatementID: "S044", Object: "security.authorization_decisions", Privilege: "USAGE", Message: "ERROR: permission denied for schema security (SQLSTATE 42501)"},
	{StatementID: "S096", Object: "indexer.launched_chains", Privilege: "INSERT", Message: "ERROR: relation \"indexer.launched_chains\" does not exist (SQLSTATE 42P01)"},
	{StatementID: "S097", Object: "indexer.launched_chains", Privilege: "SELECT", Message: "ERROR: relation \"indexer.launched_chains\" does not exist (SQLSTATE 42P01)"},
	{StatementID: "S098", Object: "indexer.pools.home_chain", Privilege: "UPDATE", Message: "ERROR: column \"home_chain\" does not exist (SQLSTATE 42703)"},
	{StatementID: "S099", Object: "indexer.chain_revenue_epochs", Privilege: "INSERT", Message: "ERROR: relation \"indexer.chain_revenue_epochs\" does not exist (SQLSTATE 42P01)"},
	{StatementID: "S100", Object: "indexer.chain_revenue_epochs", Privilege: "SELECT", Message: "ERROR: relation \"indexer.chain_revenue_epochs\" does not exist (SQLSTATE 42P01)"},
	{StatementID: "S101", Object: "indexer.chain_revenue_claims", Privilege: "INSERT", Message: "ERROR: relation \"indexer.chain_revenue_claims\" does not exist (SQLSTATE 42P01)"},
	{StatementID: "S102", Object: "indexer.chain_revenue_claims", Privilege: "SELECT", Message: "ERROR: relation \"indexer.chain_revenue_claims\" does not exist (SQLSTATE 42P01)"},
	{StatementID: "S103", Object: "indexer.launched_chains", Privilege: "UPDATE", Message: "ERROR: relation \"indexer.launched_chains\" does not exist (SQLSTATE 42P01)"},
	{StatementID: "S104", Object: "indexer.launched_chains", Privilege: "SELECT", Message: "ERROR: relation \"indexer.launched_chains\" does not exist (SQLSTATE 42P01)"},
	{StatementID: "S129", Object: "indexer.indexer_replay_authorizations", Privilege: "SELECT", Message: "ERROR: permission denied for table indexer_replay_authorizations (SQLSTATE 42501)"},
	{StatementID: "S163", Object: "indexer.launched_chains", Privilege: "UPDATE", Message: "ERROR: relation \"indexer.launched_chains\" does not exist (SQLSTATE 42P01)"},
}
