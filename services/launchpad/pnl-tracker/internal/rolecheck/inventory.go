package rolecheck

import (
	"embed"
	"encoding/json"
	"fmt"
)

// Statement identifies a runtime SQL call and the identifiers it uses.
type Statement struct {
	ID        string            `json:"id"`
	File      string            `json:"file"`
	Function  string            `json:"function"`
	Call      int               `json:"call"`
	Variant   string            `json:"variant,omitempty"`
	Role      string            `json:"role"`
	Kind      string            `json:"kind"`
	Bindings  map[string]string `json:"bindings,omitempty"`
	Tables    []string          `json:"tables"`
	Sequences []string          `json:"sequences"`
	Functions []string          `json:"functions"`
	Columns   []string          `json:"columns"`
	SHA256    string            `json:"sha256"`
}

// WaitingStatement describes an object or privilege needed by a runtime call.
type WaitingStatement struct {
	Statement string `json:"statement"`
	Object    string `json:"object"`
	Right     string `json:"right"`
	Message   string `json:"message"`
}

type omittedStatement struct {
	File     string `json:"file"`
	Function string `json:"function"`
	Reason   string `json:"reason"`
}

//go:embed statements.json waiting.json omitted.json
var documents embed.FS

func readDocument[T any](name string) ([]T, error) {
	raw, err := documents.ReadFile(name)
	if err != nil {
		return nil, err
	}
	var entries []T
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return entries, nil
}

// Statements returns the connected pnl runtime's statement inventory.
func Statements() ([]Statement, error) { return readDocument[Statement]("statements.json") }

// Waiting returns the runtime's outstanding schema and grant requirements.
func Waiting() ([]WaitingStatement, error) { return readDocument[WaitingStatement]("waiting.json") }
