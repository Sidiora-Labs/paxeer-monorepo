package rolecheck

type WaitingStatement struct {
	Object    string
	Privilege string
	Message   string
}

var WaitingStatements = map[string]WaitingStatement{}
