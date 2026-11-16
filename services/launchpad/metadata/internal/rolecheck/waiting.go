package rolecheck

type WaitingStatement struct {
	Statement string
	Object    string
	Right     string
	Message   string
}

var WaitingStatements = []WaitingStatement{}
