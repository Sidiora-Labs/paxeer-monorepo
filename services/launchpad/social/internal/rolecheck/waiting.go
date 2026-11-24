package rolecheck

// WaitingStatement names the object and authority required by a runtime statement.
type WaitingStatement struct {
	Statement string
	Object    string
	Right     string
	Message   string
}

var Waiting = []WaitingStatement{}
