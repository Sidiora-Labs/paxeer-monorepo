package rolecheck

type WaitingStatement struct {
	Statement string
	Object    string
	Right     string
	Message   string
}

var WaitingStatements = []WaitingStatement{
	{Statement: "LockPool", Object: "metadata.pools", Right: "UPDATE", Message: "ERROR: permission denied for table pools (SQLSTATE 42501)"},
	{Statement: "assets.Promote.1", Object: "metadata.pools", Right: "UPDATE", Message: "ERROR: permission denied for table pools (SQLSTATE 42501)"},
	{Statement: "connected.creator.1", Object: "metadata.pools", Right: "UPDATE", Message: "ERROR: permission denied for table pools (SQLSTATE 42501)"},
	{Statement: "publication.admitInTx.6", Object: "metadata.pools", Right: "UPDATE", Message: "ERROR: permission denied for table pools (SQLSTATE 42501)"},
	{Statement: "production.Restrict.0", Object: "metadata.pools", Right: "UPDATE", Message: "ERROR: permission denied for table pools (SQLSTATE 42501)"},
	{Statement: "publication_worker.applyPublication.4", Object: "metadata.pools", Right: "UPDATE", Message: "ERROR: permission denied for table pools (SQLSTATE 42501)"},
	{Statement: "v2.patch.1", Object: "metadata.pools", Right: "UPDATE", Message: "ERROR: permission denied for table pools (SQLSTATE 42501)"},
	{Statement: "UpdateSearchDocument", Object: "metadata.v2_documents.search_document", Right: "UPDATE", Message: "ERROR: permission denied for table v2_documents (SQLSTATE 42501)"},
	{Statement: "publication_worker.applyPublication.11", Object: "metadata.v2_documents.search_document", Right: "UPDATE", Message: "ERROR: permission denied for table v2_documents (SQLSTATE 42501)"},
	{Statement: "ApproveAsset", Object: "metadata.assets.object_key", Right: "UPDATE", Message: "ERROR: permission denied for table assets (SQLSTATE 42501)"},
	{Statement: "edge.ReadTarget.0", Object: "chat.safety_visibility_v2", Right: "SELECT", Message: "ERROR: relation \"chat.safety_visibility_v2\" does not exist (SQLSTATE 42P01)"},
	{Statement: "edge.Ready.0", Object: "chat.safety_visibility_v2", Right: "SELECT", Message: "ERROR: relation \"chat.safety_visibility_v2\" does not exist (SQLSTATE 42P01)"},
}
