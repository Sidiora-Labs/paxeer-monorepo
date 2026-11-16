package rolecheck

type WaitingStatement struct {
	Statement string
	Object    string
	Right     string
	Message   string
}

var Waiting = []WaitingStatement{
	{Statement: "prune.retained", Object: "ranking.snapshots", Right: "UPDATE", Message: "permission denied for table snapshots"},
}
