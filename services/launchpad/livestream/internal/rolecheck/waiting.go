package rolecheck

// WaitingStatement records the object and privilege a runtime statement needs.
// Message is PostgreSQL's response for that statement under RuntimeRole.
type WaitingStatement struct {
	Statement string
	Object    string
	Right     string
	Message   string
}

var Waiting = []WaitingStatement{}
