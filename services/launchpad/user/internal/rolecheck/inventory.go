package rolecheck

import _ "embed"

type Statement struct {
	ID        string            `json:"id"`
	File      string            `json:"file"`
	Function  string            `json:"function"`
	Call      int               `json:"call"`
	Bindings  map[string]string `json:"bindings"`
	Role      string            `json:"role"`
	Kind      string            `json:"kind"`
	Tables    []string          `json:"tables"`
	Sequences []string          `json:"sequences"`
	Functions []string          `json:"functions"`
	Columns   []string          `json:"columns"`
	SQL       string            `json:"sql"`
}

type Requirement struct {
	Object string `json:"object"`
	Right  string `json:"right"`
}

type WaitingEntry struct {
	Statement    string        `json:"statement"`
	Requirements []Requirement `json:"requirements"`
	Message      string        `json:"message"`
}

//go:embed statements.json
var statementsJSON []byte

//go:embed waiting.json
var waitingJSON []byte
