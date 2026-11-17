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
var Waiting = []WaitingStatement{}
