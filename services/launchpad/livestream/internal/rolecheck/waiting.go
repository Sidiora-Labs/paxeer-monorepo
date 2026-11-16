package rolecheck

// WaitingStatement records the object and privilege a runtime statement needs.
// Message is PostgreSQL's response for that statement under RuntimeRole.
type WaitingStatement struct {
	Statement string
	Object    string
	Right     string
	Message   string
}

var Waiting = []WaitingStatement{
	{
		Statement: serviceInternal + "/streams/connected_routes:LifecycleStore.EditTx:2",
		Object:    "livestream.stream_operations.title",
		Right:     "UPDATE",
		Message:   "permission denied for table stream_operations",
	},
	{
		Statement: serviceInternal + "/streams/connected_routes:NewConnectedRuntime:1",
		Object:    "indexer.indexer_canonical_creations_v1.creator",
		Right:     "SELECT",
		Message:   "permission denied for view indexer_canonical_creations_v1",
	},
	{
		Statement: serviceInternal + "/streams/lifecycle:LifecycleStore.ExpireTerminalKeys:3",
		Object:    "livestream.stream_operations.idem_key",
		Right:     "UPDATE",
		Message:   "permission denied for table stream_operations",
	},
	{
		Statement: serviceInternal + "/streams/lifecycle:LifecycleStore.Observe:4",
		Object:    "livestream.stream_operations.terminal_at",
		Right:     "UPDATE",
		Message:   "permission denied for table stream_operations",
	},
	{
		Statement: serviceInternal + "/streams/lifecycle:LifecycleStore.Observe:6",
		Object:    "livestream.stream_inbox.outcome",
		Right:     "UPDATE",
		Message:   "permission denied for table stream_inbox",
	},
	{
		Statement: serviceInternal + "/streams/provision_failure:LifecycleStore.FailBeforeProvision:3",
		Object:    "livestream.stream_operations.terminal_at",
		Right:     "UPDATE",
		Message:   "permission denied for table stream_operations",
	},
	{
		Statement: serviceInternal + "/streams/provision_resolution:LifecycleStore.applyProvisionResolution:1",
		Object:    "livestream.stream_operations.terminal_at",
		Right:     "UPDATE",
		Message:   "permission denied for table stream_operations",
	},
	{
		Statement: serviceInternal + "/streams/provision_worker:LifecycleStore.finishUnallocated:1",
		Object:    "livestream.stream_operations.terminal_at",
		Right:     "UPDATE",
		Message:   "permission denied for table stream_operations",
	},
	{
		Statement: serviceInternal + "/streams/provision_worker:ProvisionWorker.RotationStep:3",
		Object:    "indexer.indexer_canonical_creations_v1.creator",
		Right:     "SELECT",
		Message:   "permission denied for view indexer_canonical_creations_v1",
	},
}
