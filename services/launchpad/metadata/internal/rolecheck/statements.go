package rolecheck

import "strings"

type Object struct {
	Kind    string
	Name    string
	Columns []string
	Rights  []string
}

type Statement struct {
	ID       string
	Package  string
	File     string
	Function string
	Call     int
	Kind     string
	Role     string
	SQL      string
	Objects  []Object
	Bindings map[string]string
}

const runtimeRole = "metadata_runtime"

func object(kind, name, columns, rights string) Object {
	return Object{Kind: kind, Name: name, Columns: strings.Fields(columns), Rights: strings.Fields(rights)}
}

var RuntimeStatements = []Statement{
	{ID: "CreateAsset", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/assets.go", Function: "CreateAsset", Call: 0, Kind: "INSERT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.assets", "hash id object_key pool scanner_engine scanner_revision size state", "INSERT"),
		},
	},
	{ID: "ReadAsset", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/assets.go", Function: "CreateAsset", Call: 1, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.assets", "hash id object_key pool size", "SELECT"),
		},
	},
	{ID: "TransitionAsset", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/assets.go", Function: "TransitionAsset", Call: 0, Kind: "UPDATE", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.assets", "id state", "SELECT UPDATE"),
		},
	},
	{ID: "AssetPool", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/assets.go", Function: "Promote", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.assets", "id pool", "SELECT"),
		},
	},
	{ID: "LockAsset", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/assets.go", Function: "Promote", Call: 2, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.assets", "hash id pool size state", "SELECT UPDATE"),
		},
	},
	{ID: "UpdateAssetVariants", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/assets.go", Function: "Promote", Call: 5, Kind: "UPDATE", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.asset_details_v2", "asset_id height mime variants width", "SELECT UPDATE"),
		},
	},
	{ID: "ApproveAsset", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/assets.go", Function: "Promote", Call: 6, Kind: "UPDATE", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.assets", "id object_key scanner_engine scanner_revision state", "SELECT UPDATE"),
		},
	},
	{ID: "ApprovedObject", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/assets.go", Function: "ApprovedReference", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.assets", "id object_key pool state", "SELECT"),
			object("relation", "metadata.documents", "blocked pool", "SELECT"),
		},
	},
	{ID: "ReserveNonce", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/v2.go", Function: "AuthorizeInTx", Call: 0, Kind: "INSERT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.v2_nonces", "digest nonce operation wallet", "INSERT"),
		},
	},
	{ID: "ReadNonce", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/v2.go", Function: "AuthorizeInTx", Call: 1, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.v2_nonces", "digest nonce operation wallet", "SELECT"),
		},
	},
	{ID: "LockPool", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/v2.go", Function: "applyV2InTx", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.pools", "creator pool token", "SELECT UPDATE"),
		},
	},
	{ID: "ReadBlocked", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/v2.go", Function: "applyV2InTx", Call: 1, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.documents", "blocked pool", "SELECT"),
		},
	},
	{ID: "ReadOperation", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/v2.go", Function: "applyV2InTx", Call: 2, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.v2_operations", "actor body_hash operation pool result", "SELECT"),
		},
	},
	{ID: "EnsureDocument", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/v2.go", Function: "applyV2InTx", Call: 3, Kind: "INSERT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.v2_documents", "pool", "INSERT"),
		},
	},
	{ID: "LockDocument", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/v2.go", Function: "applyV2InTx", Call: 4, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.v2_documents", "pool revision", "SELECT UPDATE"),
		},
	},
	{ID: "ReadLogoAsset", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/v2.go", Function: "applyV2InTx", Call: 5, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.assets", "id pool state", "SELECT UPDATE"),
		},
	},
	{ID: "UpdateDocument", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/v2.go", Function: "applyV2InTx", Call: 6, Kind: "UPDATE", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.v2_documents", "asset_id document pool revision updated_at", "SELECT UPDATE"),
		},
	},
	{ID: "UpdateSearchDocument", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/v2.go", Function: "applyV2InTx", Call: 7, Kind: "UPDATE", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.v2_documents", "pool search_document", "SELECT UPDATE"),
		},
	},
	{ID: "RecordOperation", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/v2.go", Function: "applyV2InTx", Call: 8, Kind: "INSERT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.v2_operations", "actor body_hash operation pool result revision", "INSERT"),
		},
	},
	{ID: "RecordOutbox", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/v2.go", Function: "applyV2InTx", Call: 9, Kind: "INSERT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.v2_outbox", "payload pool revision", "INSERT"),
			object("function", "pg_catalog.jsonb_build_object", "", "EXECUTE"),
		},
	},
	{ID: "ReadPublishedDocument", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/v2.go", Function: "readV2", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.documents", "blocked pool", "SELECT"),
			object("relation", "metadata.pools", "pool token", "SELECT"),
			object("relation", "metadata.v2_documents", "asset_id document pool revision updated_at", "SELECT"),
		},
	},
	{ID: "ReadPoolBlocked", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/v2.go", Function: "readCanonicalDocument", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.documents", "blocked pool", "SELECT"),
		},
	},
	{ID: "CreatePool", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/v2.go", Function: "patch", Call: 0, Kind: "INSERT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.pools", "creator pool token", "INSERT"),
		},
	},
	{ID: "ReadConnectedAsset", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "respondAsset", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.asset_details_v2", "asset_id mime variants", "SELECT"),
			object("relation", "metadata.assets", "hash id pool size state", "SELECT"),
			object("relation", "metadata.documents", "blocked pool", "SELECT"),
			object("relation", "metadata.pools", "creator pool token", "SELECT"),
		},
	},
	{ID: "ReadMediaPool", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "media", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.pools", "pool", "SELECT"),
		},
	},
	{ID: "ReadApprovedAsset", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "media", Call: 1, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.asset_details_v2", "asset_id height mime variants width", "SELECT"),
			object("relation", "metadata.assets", "hash id pool state", "SELECT"),
		},
	},
	{ID: "RejectAsset", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "upload", Call: 6, Kind: "UPDATE", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.asset_details_v2", "asset_id rejected_until", "SELECT UPDATE"),
			object("relation", "metadata.assets", "id scanner_engine scanner_revision state", "SELECT UPDATE"),
			object("function", "pg_catalog.clock_timestamp", "", "EXECUTE"),
		},
	},
	{ID: "ListRejectedAssets", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "ExpireRejected", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.asset_details_v2", "asset_id investigation_hold rejected_until", "SELECT"),
			object("relation", "metadata.assets", "id object_key state", "SELECT"),
			object("function", "pg_catalog.clock_timestamp", "", "EXECUTE"),
		},
	},
	{ID: "FindConnectedAsset", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "FindConnectedAsset", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.assets", "hash id object_key pool scanner_engine scanner_revision size state", "SELECT"),
		},
	},
	{ID: "connected.upload.2", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "upload", Call: 2, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.assets", "id", "SELECT"),
		},
	},
	{ID: "connected.upload.3", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "upload", Call: 3, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("function", "pg_catalog.hashtextextended", "", "EXECUTE"),
			object("function", "pg_catalog.pg_advisory_lock", "", "EXECUTE"),
		},
	},
	{ID: "connected.upload.4", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "upload", Call: 4, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("function", "pg_catalog.hashtextextended", "", "EXECUTE"),
			object("function", "pg_catalog.pg_advisory_unlock", "", "EXECUTE"),
		},
	},
	{ID: "CreateAssetRecord", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "CreateConnectedAsset", Call: 0, Kind: "INSERT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.assets", "hash id object_key pool scanner_engine scanner_revision size state", "INSERT"),
		},
	},
	{ID: "CreateAssetDetails", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "CreateConnectedAsset", Call: 2, Kind: "INSERT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.asset_details_v2", "asset_id height mime variants width", "INSERT"),
		},
	},
	{ID: "LockAssetDetails", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "CreateConnectedAsset", Call: 3, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.asset_details_v2", "asset_id height mime width", "SELECT UPDATE"),
		},
	},
	{ID: "ReadAccountAsset", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "getAccountAsset", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.account_assets_v2", "blocked hash id mime owner_wallet scanner_engine scanner_revision size state variants", "SELECT"),
		},
	},
	{ID: "CreateAccountAsset", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "uploadAccountAsset", Call: 0, Kind: "INSERT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.account_assets_v2", "hash height id mime owner_wallet size state width", "INSERT"),
		},
	},
	{ID: "UpdateAccountAssetModeration", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "syncAccountModeration", Call: 0, Kind: "UPDATE", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.account_assets_v2", "blocked id moderation_revision", "SELECT UPDATE"),
		},
	},
	{ID: "ApproveAccountAsset", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "ProcessAccountAsset", Call: 6, Kind: "UPDATE", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.account_assets_v2", "id scanner_engine scanner_revision state variants", "SELECT UPDATE"),
		},
	},
	{ID: "DeleteAccountAsset", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "purgeAccountAssets", Call: 1, Kind: "UPDATE", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.account_assets_v2", "deletion_id owner_wallet state", "SELECT UPDATE"),
		},
	},
	{ID: "FindPublication", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/publication.go", Function: "admitInTx", Call: 1, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.metadata_publication_intents_v3", "admission_response body_hash content_hash draft_id operation token", "SELECT"),
		},
	},
	{ID: "LockPublicationJob", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/publication.go", Function: "admitInTx", Call: 2, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.j", "attempt_count operation state", "SELECT UPDATE"),
			object("relation", "metadata.metadata_publication_intents_v3", "expires_at", "SELECT"),
			object("relation", "metadata.metadata_publication_jobs_v3", "", "SELECT"),
			object("function", "pg_catalog.clock_timestamp", "", "EXECUTE"),
		},
	},
	{ID: "CountQueued", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/publication.go", Function: "admitInTx", Call: 4, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.metadata_publication_jobs_v3", "state", "SELECT"),
			object("function", "pg_catalog.count", "", "EXECUTE"),
		},
	},
	{ID: "InsertPublicationIntent", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/publication.go", Function: "admitInTx", Call: 9, Kind: "INSERT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.metadata_publication_intents_v3", "accepted_at account_id admission_response auth_epoch body body_hash content_hash creation_tx_hash creator draft_id envelope envelope_digest expires_at metadata nonce operation pool request_id session_id token upload_refs", "INSERT"),
		},
	},
	{ID: "InsertPublicationJob", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/publication.go", Function: "admitInTx", Call: 10, Kind: "INSERT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.metadata_publication_jobs_v3", "grant_nonce next_at operation state updated_at", "INSERT"),
		},
	},
	{ID: "ReadPublication", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/publication.go", Function: "status", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.metadata_publication_intents_v3", "admission_response creator operation token", "SELECT"),
			object("relation", "metadata.metadata_publication_jobs_v3", "attempt_count last_error_code revision state", "SELECT"),
		},
	},
	{ID: "InsertPublicationGrant", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/publication.go", Function: "appendPublicationGrant", Call: 0, Kind: "INSERT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.metadata_publication_grants_v3", "account_id auth_epoch body_hash envelope envelope_digest nonce operation request_id response session_id", "INSERT"),
		},
	},
	{ID: "ClaimPublication", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/publication_worker.go", Function: "claimPublication", Call: 1, Kind: "UPDATE", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.j", "attempt_count fence lease_until next_at operation state", "SELECT UPDATE"),
			object("relation", "metadata.metadata_publication_intents_v3", "expires_at operation pool", "SELECT"),
			object("relation", "metadata.metadata_publication_jobs_v3", "attempt_count fence lease_owner lease_until state updated_at", "SELECT UPDATE"),
			object("function", "pg_catalog.clock_timestamp", "", "EXECUTE"),
		},
	},
	{ID: "ReadPublicationPayload", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/publication_worker.go", Function: "applyPublication", Call: 1, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.metadata_publication_grants_v3", "account_id auth_epoch body_hash envelope envelope_digest nonce operation request_id response session_id", "SELECT"),
			object("relation", "metadata.metadata_publication_intents_v3", "body body_hash content_hash creation_tx_hash creator expires_at metadata operation pool token", "SELECT"),
			object("relation", "metadata.metadata_publication_jobs_v3", "grant_nonce", "SELECT"),
			object("function", "pg_catalog.clock_timestamp", "", "EXECUTE"),
		},
	},
	{ID: "PublishJob", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/publication_worker.go", Function: "applyPublication", Call: 14, Kind: "UPDATE", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.metadata_publication_jobs_v3", "fence last_error_code lease_owner lease_until operation result revision state updated_at", "SELECT UPDATE"),
			object("function", "pg_catalog.clock_timestamp", "", "EXECUTE"),
		},
	},
	{ID: "ReadRawLog", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/canonical.go", Function: "rawEvent", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "indexer.indexer_raw_logs_v1", "block_hash chain_id data emitter height log_index topics transaction_hash transaction_index", "SELECT"),
		},
	},
	{ID: "ReadCanonicalCreation", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/canonical.go", Function: "ResolveTokenInTx", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "indexer.indexer_canonical_creations_v1", "block_hash block_timestamp creator factory height log_index nft_id optical payload pool provenance token transaction_hash transaction_index", "SELECT"),
		},
	},
	{ID: "CheckCanonicalBlock", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/canonical.go", Function: "ResolveTokenInTx", Call: 1, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "indexer.indexer_block_completeness_v1", "block_hash chain_id fetch_complete height required_decode_complete stream_id", "SELECT"),
		},
	},
	{ID: "CommittedBlock", Package: "shared/auth", File: "sdk/launchpad/shared/auth/contract.go", Function: "CommittedBlock", Call: 0, Kind: "SELECT", Role: runtimeRole,
		SQL: "SELECT height::text,block_hash,integrity FROM auth_committed_blocks AS heads WHERE chain_id=125 ORDER BY heads.height DESC LIMIT 1",
		Objects: []Object{
			object("relation", "metadata.auth_committed_blocks", "block_hash chain_id height integrity", "SELECT"),
		},
	},
	{ID: "IdentityGuard", Package: "shared/auth", File: "sdk/launchpad/shared/auth/http_authorization.go", Function: "ResolveTx", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("function", "users.identity_guard_v2", "access_expires_at account_id auth_epoch proof_at session_id wallet", "EXECUTE"),
		},
	},
	{ID: "ReadLedger", Package: "shared/auth", File: "sdk/launchpad/shared/auth/ledger.go", Function: "ExecuteValidatedTx", Call: 2, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.operation_ledger", "action actor body_digest envelope_digest nonce response response_digest", "SELECT"),
		},
	},
	{ID: "WriteLedger", Package: "shared/auth", File: "sdk/launchpad/shared/auth/ledger.go", Function: "ExecuteValidatedTx", Call: 3, Kind: "INSERT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.operation_ledger", "action actor body_digest envelope_digest nonce request_id response response_digest", "INSERT"),
		},
	},
	{ID: "WriteAudit", Package: "shared/auth", File: "sdk/launchpad/shared/auth/ledger.go", Function: "AppendAudit", Call: 0, Kind: "INSERT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.security_audit_outbox", "action actor_pseudonym policy_version request_digest request_id result", "INSERT"),
			object("sequence", "metadata.security_audit_outbox_id_seq", "", "USAGE"),
		},
	},
	{ID: "PublicationSearchPath", Package: "internal/app", File: "services/launchpad/metadata/internal/app/app.go", Function: "newRuntime", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("function", "pg_catalog.current_setting", "", "EXECUTE"),
		},
	},
	{ID: "PublicationIdentityPrivilege", Package: "internal/app", File: "services/launchpad/metadata/internal/app/app.go", Function: "newRuntime", Call: 1, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("function", "pg_catalog.has_function_privilege", "", "EXECUTE"),
			object("function", "pg_catalog.to_regprocedure", "", "EXECUTE"),
		},
	},
	{ID: "MaintenanceLeader", Package: "internal/app", File: "services/launchpad/metadata/internal/app/app.go", Function: "runMediaMaintenance", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("function", "pg_catalog.hashtextextended", "", "EXECUTE"),
			object("function", "pg_catalog.pg_try_advisory_lock", "", "EXECUTE"),
		},
	},
	{ID: "MaintenanceUnlock", Package: "internal/app", File: "services/launchpad/metadata/internal/app/app.go", Function: "runMediaMaintenance", Call: 1, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("function", "pg_catalog.hashtextextended", "", "EXECUTE"),
			object("function", "pg_catalog.pg_advisory_unlock", "", "EXECUTE"),
		},
	},
	{ID: "ScanPendingMedia", Package: "internal/app", File: "services/launchpad/metadata/internal/app/app.go", Function: "scanPendingMedia", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.asset_details_v2", "asset_id created_at mime rejected_until variants", "SELECT"),
			object("relation", "metadata.assets", "hash id object_key pool size state", "SELECT"),
			object("function", "pg_catalog.clock_timestamp", "", "EXECUTE"),
		},
	},
	{ID: "CheckReadinessAttestation", Package: "shared/db/production", File: "sdk/launchpad/shared/db/production/migrations.go", Function: "CheckReadiness", Call: 2, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("function", "migration_control.readiness", "interface_digest migrations reader_min_version registry_digest", "EXECUTE"),
		},
	},
	{ID: "CheckReadinessRecovery", Package: "shared/db/production", File: "sdk/launchpad/shared/db/production/migrations.go", Function: "CheckReadiness", Call: 4, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("function", "recovery.current_epoch", "epoch region", "EXECUTE"),
		},
	},
	{ID: "app.newRuntime.2.operation_ledger", Package: "internal/app", File: "services/launchpad/metadata/internal/app/app.go", Function: "newRuntime", Call: 2, Kind: "SELECT", Role: runtimeRole,
		Bindings: map[string]string{"cfg.MetadataSchema": "metadata", "table": "operation_ledger"},
		Objects: []Object{
			object("relation", "metadata.operation_ledger", "", "SELECT"),
		},
	},
	{ID: "app.newRuntime.2.security_audit_outbox", Package: "internal/app", File: "services/launchpad/metadata/internal/app/app.go", Function: "newRuntime", Call: 2, Kind: "SELECT", Role: runtimeRole,
		Bindings: map[string]string{"cfg.MetadataSchema": "metadata", "table": "security_audit_outbox"},
		Objects: []Object{
			object("relation", "metadata.security_audit_outbox", "", "SELECT"),
		},
	},
	{ID: "app.newRuntime.2.metadata_publication_intents_v3", Package: "internal/app", File: "services/launchpad/metadata/internal/app/app.go", Function: "newRuntime", Call: 2, Kind: "SELECT", Role: runtimeRole,
		Bindings: map[string]string{"cfg.MetadataSchema": "metadata", "table": "metadata_publication_intents_v3"},
		Objects: []Object{
			object("relation", "metadata.metadata_publication_intents_v3", "", "SELECT"),
		},
	},
	{ID: "app.newRuntime.2.metadata_publication_jobs_v3", Package: "internal/app", File: "services/launchpad/metadata/internal/app/app.go", Function: "newRuntime", Call: 2, Kind: "SELECT", Role: runtimeRole,
		Bindings: map[string]string{"cfg.MetadataSchema": "metadata", "table": "metadata_publication_jobs_v3"},
		Objects: []Object{
			object("relation", "metadata.metadata_publication_jobs_v3", "", "SELECT"),
		},
	},
	{ID: "app.newRuntime.2.metadata_publication_grants_v3", Package: "internal/app", File: "services/launchpad/metadata/internal/app/app.go", Function: "newRuntime", Call: 2, Kind: "SELECT", Role: runtimeRole,
		Bindings: map[string]string{"cfg.MetadataSchema": "metadata", "table": "metadata_publication_grants_v3"},
		Objects: []Object{
			object("relation", "metadata.metadata_publication_grants_v3", "", "SELECT"),
		},
	},
	{ID: "app.scanPendingMedia.1", Package: "internal/app", File: "services/launchpad/metadata/internal/app/app.go", Function: "scanPendingMedia", Call: 1, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("function", "pg_catalog.hashtextextended", "", "EXECUTE"),
			object("function", "pg_catalog.pg_try_advisory_lock", "", "EXECUTE"),
		},
	},
	{ID: "app.scanPendingMedia.2", Package: "internal/app", File: "services/launchpad/metadata/internal/app/app.go", Function: "scanPendingMedia", Call: 2, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("function", "pg_catalog.hashtextextended", "", "EXECUTE"),
			object("function", "pg_catalog.pg_advisory_unlock", "", "EXECUTE"),
		},
	},
	{ID: "app.scanPendingMedia.3", Package: "internal/app", File: "services/launchpad/metadata/internal/app/app.go", Function: "scanPendingMedia", Call: 3, Kind: "UPDATE", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.asset_details_v2", "asset_id rejected_until", "SELECT UPDATE"),
			object("function", "pg_catalog.clock_timestamp", "", "EXECUTE"),
		},
	},
	{ID: "app.scanPendingMedia.4", Package: "internal/app", File: "services/launchpad/metadata/internal/app/app.go", Function: "scanPendingMedia", Call: 4, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.assets", "id state", "SELECT"),
		},
	},
	{ID: "app.scanPendingMedia.5", Package: "internal/app", File: "services/launchpad/metadata/internal/app/app.go", Function: "scanPendingMedia", Call: 5, Kind: "UPDATE", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.asset_details_v2", "asset_id rejected_until", "SELECT UPDATE"),
			object("relation", "metadata.assets", "id scanner_engine scanner_revision state", "SELECT UPDATE"),
			object("function", "pg_catalog.clock_timestamp", "", "EXECUTE"),
		},
	},
	{ID: "app.scanPendingMedia.6", Package: "internal/app", File: "services/launchpad/metadata/internal/app/app.go", Function: "scanPendingMedia", Call: 6, Kind: "UPDATE", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.asset_details_v2", "asset_id height mime rejected_until variants width", "SELECT UPDATE"),
		},
	},
	{ID: "assets.Promote.1", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/assets.go", Function: "Promote", Call: 1, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.pools", "creator pool", "SELECT UPDATE"),
		},
	},
	{ID: "assets.Promote.3", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/assets.go", Function: "Promote", Call: 3, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.documents", "blocked pool", "SELECT UPDATE"),
		},
	},
	{ID: "assets.Promote.4", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/assets.go", Function: "Promote", Call: 4, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("function", "pg_catalog.to_regclass", "", "EXECUTE"),
		},
	},
	{ID: "connected.creator.0", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "creator", Call: 0, Kind: "INSERT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.pools", "creator pool token", "INSERT"),
		},
	},
	{ID: "connected.creator.1", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "creator", Call: 1, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.pools", "creator pool token", "SELECT UPDATE"),
		},
	},
	{ID: "connected.upload.0", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "upload", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("function", "pg_catalog.hashtextextended", "", "EXECUTE"),
			object("function", "pg_catalog.pg_advisory_lock", "", "EXECUTE"),
		},
	},
	{ID: "connected.upload.1", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "upload", Call: 1, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("function", "pg_catalog.hashtextextended", "", "EXECUTE"),
			object("function", "pg_catalog.pg_advisory_unlock", "", "EXECUTE"),
		},
	},
	{ID: "connected.upload.5", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "upload", Call: 5, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.assets", "id state", "SELECT"),
		},
	},
	{ID: "connected.asset.0", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "asset", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.pools", "pool token", "SELECT"),
		},
	},
	{ID: "connected.getMetadata.0", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "getMetadata", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.pools", "pool token", "SELECT"),
		},
	},
	{ID: "connected.uploadAccountAsset.1", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "uploadAccountAsset", Call: 1, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.account_assets_v2", "hash height id mime owner_wallet size state width", "SELECT"),
		},
	},
	{ID: "connected.uploadAccountAsset.2", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "uploadAccountAsset", Call: 2, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.account_assets_v2", "id state", "SELECT UPDATE"),
		},
	},
	{ID: "connected.ProcessAccountAsset.0", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "ProcessAccountAsset", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("function", "pg_catalog.hashtextextended", "", "EXECUTE"),
			object("function", "pg_catalog.pg_advisory_lock", "", "EXECUTE"),
		},
	},
	{ID: "connected.ProcessAccountAsset.1", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "ProcessAccountAsset", Call: 1, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("function", "pg_catalog.hashtextextended", "", "EXECUTE"),
			object("function", "pg_catalog.pg_advisory_unlock", "", "EXECUTE"),
		},
	},
	{ID: "connected.ProcessAccountAsset.2", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "ProcessAccountAsset", Call: 2, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.account_assets_v2", "blocked hash height id mime owner_wallet size state width", "SELECT"),
		},
	},
	{ID: "connected.ProcessAccountAsset.3", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "ProcessAccountAsset", Call: 3, Kind: "UPDATE", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.account_assets_v2", "id state", "SELECT UPDATE"),
		},
	},
	{ID: "connected.ProcessAccountAsset.4", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "ProcessAccountAsset", Call: 4, Kind: "UPDATE", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.account_assets_v2", "id rejected_until scanner_engine scanner_revision state", "SELECT UPDATE"),
			object("function", "pg_catalog.clock_timestamp", "", "EXECUTE"),
		},
	},
	{ID: "connected.ProcessAccountAsset.5", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "ProcessAccountAsset", Call: 5, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.account_assets_v2", "blocked id state", "SELECT UPDATE"),
		},
	},
	{ID: "connected.MaintainAccountAssets.0", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "MaintainAccountAssets", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.account_assets_v2", "created_at id state", "SELECT"),
		},
	},
	{ID: "connected.MaintainAccountAssets.1", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "MaintainAccountAssets", Call: 1, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.account_assets_v2", "id investigation_hold rejected_until state", "SELECT"),
			object("function", "pg_catalog.clock_timestamp", "", "EXECUTE"),
		},
	},
	{ID: "connected.MaintainAccountAssets.2", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "MaintainAccountAssets", Call: 2, Kind: "UPDATE", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.account_assets_v2", "id investigation_hold state", "SELECT UPDATE"),
		},
	},
	{ID: "connected.purgeAccountAssets.0", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "purgeAccountAssets", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.account_assets_v2", "investigation_hold owner_wallet", "SELECT"),
		},
	},
	{ID: "connected.purgeAccountAssets.2", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "purgeAccountAssets", Call: 2, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.account_assets_v2", "deletion_id id owner_wallet variants", "SELECT"),
		},
	},
	{ID: "connected.CreateConnectedAsset.1", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/connected.go", Function: "CreateConnectedAsset", Call: 1, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.assets", "hash id pool size", "SELECT UPDATE"),
		},
	},
	{ID: "publication.admitInTx.0", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/publication.go", Function: "admitInTx", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("function", "pg_catalog.hashtextextended", "", "EXECUTE"),
			object("function", "pg_catalog.pg_advisory_xact_lock", "", "EXECUTE"),
		},
	},
	{ID: "publication.admitInTx.3", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/publication.go", Function: "admitInTx", Call: 3, Kind: "UPDATE", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.metadata_publication_jobs_v3", "grant_nonce last_error_code next_at operation state updated_at", "SELECT UPDATE"),
			object("function", "pg_catalog.clock_timestamp", "", "EXECUTE"),
		},
	},
	{ID: "publication.admitInTx.5", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/publication.go", Function: "admitInTx", Call: 5, Kind: "INSERT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.pools", "creator pool token", "INSERT"),
		},
	},
	{ID: "publication.admitInTx.6", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/publication.go", Function: "admitInTx", Call: 6, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.pools", "creator pool token", "SELECT UPDATE"),
		},
	},
	{ID: "publication.admitInTx.7", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/publication.go", Function: "admitInTx", Call: 7, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.documents", "blocked pool revision", "SELECT"),
			object("relation", "metadata.v2_documents", "pool revision", "SELECT"),
		},
	},
	{ID: "publication.admitInTx.8", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/publication.go", Function: "admitInTx", Call: 8, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("function", "pg_catalog.clock_timestamp", "", "EXECUTE"),
		},
	},
	{ID: "production.Restrict.0", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/production.go", Function: "Restrict", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.pools", "creator pool", "SELECT UPDATE"),
		},
	},
	{ID: "production.Restrict.1", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/production.go", Function: "Restrict", Call: 1, Kind: "INSERT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.documents", "blocked moderation_revision pool", "INSERT SELECT UPDATE"),
		},
	},
	{ID: "publication_worker.claimPublication.0", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/publication_worker.go", Function: "claimPublication", Call: 0, Kind: "UPDATE", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.metadata_publication_intents_v3", "expires_at operation", "SELECT"),
			object("relation", "metadata.metadata_publication_jobs_v3", "attempt_count last_error_code lease_owner lease_until operation state updated_at", "SELECT UPDATE"),
			object("function", "pg_catalog.clock_timestamp", "", "EXECUTE"),
		},
	},
	{ID: "publication_worker.failPublication.0", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/publication_worker.go", Function: "failPublication", Call: 0, Kind: "UPDATE", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.metadata_publication_jobs_v3", "fence last_error_code lease_owner lease_until next_at operation state updated_at", "SELECT UPDATE"),
			object("function", "pg_catalog.clock_timestamp", "", "EXECUTE"),
			object("function", "pg_catalog.make_interval", "", "EXECUTE"),
		},
	},
	{ID: "publication_worker.applyPublication.0", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/publication_worker.go", Function: "applyPublication", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.metadata_publication_jobs_v3", "fence lease_owner lease_until operation state", "SELECT UPDATE"),
			object("function", "pg_catalog.clock_timestamp", "", "EXECUTE"),
		},
	},
	{ID: "publication_worker.applyPublication.2", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/publication_worker.go", Function: "applyPublication", Call: 2, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.operation_ledger", "action actor body_digest envelope_digest nonce request_id response", "SELECT"),
		},
	},
	{ID: "publication_worker.applyPublication.3", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/publication_worker.go", Function: "applyPublication", Call: 3, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.security_audit_outbox", "action request_digest request_id result", "SELECT"),
		},
	},
	{ID: "publication_worker.applyPublication.4", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/publication_worker.go", Function: "applyPublication", Call: 4, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.pools", "creator pool token", "SELECT UPDATE"),
		},
	},
	{ID: "publication_worker.applyPublication.5", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/publication_worker.go", Function: "applyPublication", Call: 5, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.documents", "blocked pool", "SELECT"),
		},
	},
	{ID: "publication_worker.applyPublication.6", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/publication_worker.go", Function: "applyPublication", Call: 6, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.v2_operations", "actor body_hash operation pool result", "SELECT"),
		},
	},
	{ID: "publication_worker.applyPublication.7", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/publication_worker.go", Function: "applyPublication", Call: 7, Kind: "INSERT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.v2_documents", "pool", "INSERT"),
		},
	},
	{ID: "publication_worker.applyPublication.8", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/publication_worker.go", Function: "applyPublication", Call: 8, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.v2_documents", "pool revision", "SELECT UPDATE"),
		},
	},
	{ID: "publication_worker.applyPublication.9", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/publication_worker.go", Function: "applyPublication", Call: 9, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.assets", "id pool state", "SELECT UPDATE"),
		},
	},
	{ID: "publication_worker.applyPublication.10", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/publication_worker.go", Function: "applyPublication", Call: 10, Kind: "UPDATE", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.v2_documents", "asset_id document pool revision updated_at", "SELECT UPDATE"),
			object("function", "pg_catalog.clock_timestamp", "", "EXECUTE"),
		},
	},
	{ID: "publication_worker.applyPublication.11", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/publication_worker.go", Function: "applyPublication", Call: 11, Kind: "UPDATE", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.v2_documents", "pool search_document", "SELECT UPDATE"),
		},
	},
	{ID: "publication_worker.applyPublication.12", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/publication_worker.go", Function: "applyPublication", Call: 12, Kind: "INSERT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.v2_operations", "actor body_hash operation pool result revision", "INSERT"),
		},
	},
	{ID: "publication_worker.applyPublication.13", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/publication_worker.go", Function: "applyPublication", Call: 13, Kind: "INSERT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.v2_outbox", "payload pool revision", "INSERT"),
			object("function", "pg_catalog.jsonb_build_object", "", "EXECUTE"),
		},
	},
	{ID: "legacy_reads.refreshModeration.0", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/legacy_reads.go", Function: "refreshModeration", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.pools", "pool token", "SELECT"),
		},
	},
	{ID: "legacy_reads.logoKey.0", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/legacy_reads.go", Function: "logoKey", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.asset_details_v2", "asset_id variants", "SELECT"),
			object("relation", "metadata.assets", "hash id pool state", "SELECT"),
		},
	},
	{ID: "edge.ReadTarget.0", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/edge.go", Function: "ReadTarget", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Bindings: map[string]string{"view": "\"chat\".\"safety_visibility_v2\""},
		Objects: []Object{
			object("relation", "chat.safety_visibility_v2", "revision status target_id target_type", "SELECT"),
		},
	},
	{ID: "edge.Ready.0", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/edge.go", Function: "Ready", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Bindings: map[string]string{"view": "\"chat\".\"safety_visibility_v2\""},
		Objects: []Object{
			object("relation", "chat.safety_visibility_v2", "revision status target_id target_type", "SELECT"),
		},
	},
	{ID: "v2.patch.1", Package: "internal/metadata", File: "services/launchpad/metadata/internal/metadata/v2.go", Function: "patch", Call: 1, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "metadata.pools", "creator pool token", "SELECT UPDATE"),
		},
	},
	{ID: "ledger.ExecuteValidatedTx.0", Package: "shared/auth", File: "sdk/launchpad/shared/auth/ledger.go", Function: "ExecuteValidatedTx", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("function", "pg_catalog.hashtextextended", "", "EXECUTE"),
			object("function", "pg_catalog.pg_advisory_xact_lock", "", "EXECUTE"),
		},
	},
	{ID: "ledger.ExecuteValidatedTx.1", Package: "shared/auth", File: "sdk/launchpad/shared/auth/ledger.go", Function: "ExecuteValidatedTx", Call: 1, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("function", "pg_catalog.clock_timestamp", "", "EXECUTE"),
		},
	},
	{ID: "contract.AuthorizeInTx.0", Package: "shared/auth", File: "sdk/launchpad/shared/auth/contract.go", Function: "AuthorizeInTx", Call: 0, Kind: "INSERT", Role: runtimeRole,
		Bindings: map[string]string{"qualified": "\"metadata\".\"v2_nonces\""},
		Objects: []Object{
			object("relation", "metadata.v2_nonces", "digest nonce operation wallet", "INSERT"),
		},
	},
	{ID: "contract.AuthorizeInTx.1", Package: "shared/auth", File: "sdk/launchpad/shared/auth/contract.go", Function: "AuthorizeInTx", Call: 1, Kind: "SELECT", Role: runtimeRole,
		Bindings: map[string]string{"qualified": "\"metadata\".\"v2_nonces\""},
		Objects: []Object{
			object("relation", "metadata.v2_nonces", "digest nonce operation wallet", "SELECT"),
		},
	},
	{ID: "migrations.CatalogFingerprint.0", Package: "shared/db/production", File: "sdk/launchpad/shared/db/production/migrations.go", Function: "CatalogFingerprint", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "pg_catalog.pg_attrdef", "adbin adnum adrelid", "SELECT"),
			object("relation", "pg_catalog.pg_attribute", "attidentity attisdropped attname attnotnull attnum attrelid atttypid atttypmod", "SELECT"),
			object("relation", "pg_catalog.pg_class", "oid relforcerowsecurity relkind relname relnamespace relowner relrowsecurity", "SELECT"),
			object("relation", "pg_catalog.pg_constraint", "conname conrelid oid", "SELECT"),
			object("relation", "pg_catalog.pg_namespace", "nspname oid", "SELECT"),
			object("relation", "pg_catalog.pg_proc", "oid proname pronamespace proowner", "SELECT"),
			object("relation", "pg_catalog.pg_trigger", "oid tgisinternal tgname tgrelid", "SELECT"),
			object("function", "pg_catalog.concat_ws", "", "EXECUTE"),
			object("function", "pg_catalog.format_type", "", "EXECUTE"),
			object("function", "pg_catalog.pg_get_constraintdef", "", "EXECUTE"),
			object("function", "pg_catalog.pg_get_expr", "", "EXECUTE"),
			object("function", "pg_catalog.pg_get_function_identity_arguments", "", "EXECUTE"),
			object("function", "pg_catalog.pg_get_functiondef", "", "EXECUTE"),
			object("function", "pg_catalog.pg_get_indexdef", "", "EXECUTE"),
			object("function", "pg_catalog.pg_get_triggerdef", "", "EXECUTE"),
			object("function", "pg_catalog.pg_get_userbyid", "", "EXECUTE"),
			object("function", "pg_catalog.pg_get_viewdef", "", "EXECUTE"),
		},
	},
	{ID: "migrations.CheckReadiness.0", Package: "shared/db/production", File: "sdk/launchpad/shared/db/production/migrations.go", Function: "CheckReadiness", Call: 0, Kind: "SET", Role: runtimeRole},
	{ID: "migrations.CheckReadiness.1", Package: "shared/db/production", File: "sdk/launchpad/shared/db/production/migrations.go", Function: "CheckReadiness", Call: 1, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "pg_catalog.pg_class", "oid relnamespace relowner", "SELECT"),
			object("relation", "pg_catalog.pg_namespace", "nspname nspowner oid", "SELECT"),
			object("relation", "pg_catalog.pg_roles", "oid rolbypassrls rolcreatedb rolcreaterole rolname rolsuper", "SELECT"),
			object("function", "pg_catalog.has_schema_privilege", "", "EXECUTE"),
			object("function", "pg_catalog.pg_has_role", "", "EXECUTE"),
		},
	},
	{ID: "migrations.CheckReadiness.3", Package: "shared/db/production", File: "sdk/launchpad/shared/db/production/migrations.go", Function: "CheckReadiness", Call: 3, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "pg_catalog.pg_namespace", "nspname nspowner", "SELECT"),
			object("function", "pg_catalog.has_schema_privilege", "", "EXECUTE"),
			object("function", "pg_catalog.pg_has_role", "", "EXECUTE"),
		},
	},
	{ID: "migrations.checkPrivilege.0", Package: "shared/db/production", File: "sdk/launchpad/shared/db/production/migrations.go", Function: "checkPrivilege", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("function", "pg_catalog.has_function_privilege", "", "EXECUTE"),
		},
	},
	{ID: "migrations.checkPrivilege.1", Package: "shared/db/production", File: "sdk/launchpad/shared/db/production/migrations.go", Function: "checkPrivilege", Call: 1, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("function", "pg_catalog.has_sequence_privilege", "", "EXECUTE"),
		},
	},
	{ID: "migrations.checkPrivilege.2", Package: "shared/db/production", File: "sdk/launchpad/shared/db/production/migrations.go", Function: "checkPrivilege", Call: 2, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("function", "pg_catalog.has_column_privilege", "", "EXECUTE"),
		},
	},
	{ID: "migrations.checkPrivilege.3", Package: "shared/db/production", File: "sdk/launchpad/shared/db/production/migrations.go", Function: "checkPrivilege", Call: 3, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("function", "pg_catalog.has_table_privilege", "", "EXECUTE"),
		},
	},
	{ID: "migrations.checkUnexpectedPrivileges.0", Package: "shared/db/production", File: "sdk/launchpad/shared/db/production/migrations.go", Function: "checkUnexpectedPrivileges", Call: 0, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "pg_catalog.pg_class", "oid relkind relname relnamespace", "SELECT"),
			object("relation", "pg_catalog.pg_namespace", "nspname oid", "SELECT"),
			object("function", "pg_catalog.has_table_privilege", "", "EXECUTE"),
		},
	},
	{ID: "migrations.checkUnexpectedPrivileges.1", Package: "shared/db/production", File: "sdk/launchpad/shared/db/production/migrations.go", Function: "checkUnexpectedPrivileges", Call: 1, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "pg_catalog.pg_attribute", "attisdropped attname attnum attrelid", "SELECT"),
			object("relation", "pg_catalog.pg_class", "oid relkind relname relnamespace", "SELECT"),
			object("relation", "pg_catalog.pg_namespace", "nspname oid", "SELECT"),
			object("function", "pg_catalog.has_column_privilege", "", "EXECUTE"),
		},
	},
	{ID: "migrations.checkUnexpectedPrivileges.2", Package: "shared/db/production", File: "sdk/launchpad/shared/db/production/migrations.go", Function: "checkUnexpectedPrivileges", Call: 2, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "pg_catalog.pg_namespace", "nspname oid", "SELECT"),
			object("relation", "pg_catalog.pg_proc", "oid proargtypes proname pronamespace", "SELECT"),
			object("function", "pg_catalog.has_function_privilege", "", "EXECUTE"),
			object("function", "pg_catalog.oidvectortypes", "", "EXECUTE"),
		},
	},
	{ID: "migrations.checkUnexpectedPrivileges.3", Package: "shared/db/production", File: "sdk/launchpad/shared/db/production/migrations.go", Function: "checkUnexpectedPrivileges", Call: 3, Kind: "SELECT", Role: runtimeRole,
		Objects: []Object{
			object("relation", "pg_catalog.pg_class", "oid relkind relname relnamespace", "SELECT"),
			object("relation", "pg_catalog.pg_namespace", "nspname oid", "SELECT"),
			object("function", "pg_catalog.has_sequence_privilege", "", "EXECUTE"),
		},
	},
}

var DevelopmentOnlyStatements = []string{
	"internal/metadata/dev_connected.go: development schemas and canonical/legacy projections",
	"internal/metadata/dev_v2.go: development indexer pool projection",
	"internal/metadata/legacy_publication.go: development token metadata and image publication",
	"internal/db/sqlcdb: development token_metadata/token_images and indexer.pools access",
	"internal/migrate: development schema installation",
	"internal/internaltest: test infrastructure",
}
