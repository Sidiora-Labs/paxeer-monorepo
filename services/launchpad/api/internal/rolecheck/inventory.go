// Package rolecheck describes SQL used by the API's connected database adapters.
package rolecheck

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
)

//go:embed statements.json waiting.json coverage.json
var documents embed.FS

// Statement binds a SQL expression to its source and runtime authorities.
// Columns includes column references, write targets and result labels as spelled
// in SQL; CTEs are separate from stored relations. Sequences lists explicit
// sequence references (identity allocation remains part of the owning insert).
type Statement struct {
	ID         string            `json:"id"`
	File       string            `json:"file"`
	Package    string            `json:"package"`
	Function   string            `json:"function"`
	Line       int               `json:"line"`
	Expression string            `json:"expression"`
	SQL        string            `json:"sql"`
	Bindings   map[string]string `json:"bindings"`
	Roles      []string          `json:"roles"`
	Kinds      []string          `json:"kinds"`
	Tables     []string          `json:"tables"`
	Sequences  []string          `json:"sequences"`
	Functions  []string          `json:"functions"`
	Columns    []string          `json:"columns"`
	CTEs       []string          `json:"ctes"`
}

// WaitingStatement names database objects and privileges needed by a statement.
type WaitingStatement struct {
	Statement string `json:"statement"`
	Role      string `json:"role"`
	Object    string `json:"object"`
	Right     string `json:"right"`
	SQLState  string `json:"sqlstate"`
	Message   string `json:"message"`
}

// SourceScope identifies the connected SQL functions in a source file. An empty
// Functions list includes all functions except those in ExcludedFunctions.
type SourceScope struct {
	File              string   `json:"file"`
	Functions         []string `json:"functions,omitempty"`
	ExcludedFunctions []string `json:"excluded_functions,omitempty"`
}

type Exclusion struct {
	Package string   `json:"package"`
	Paths   []string `json:"paths"`
	Reason  string   `json:"reason"`
}

type Coverage struct {
	Sources    []SourceScope `json:"sources"`
	Exclusions []Exclusion   `json:"exclusions"`
}

func readDocument(name string, into any) error {
	raw, err := documents.ReadFile(name)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(into); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("%s: trailing JSON", name)
	}
	return nil
}

func Statements() ([]Statement, error) {
	var entries []Statement
	err := readDocument("statements.json", &entries)
	return entries, err
}

func WaitingList() ([]WaitingStatement, error) {
	var entries []WaitingStatement
	err := readDocument("waiting.json", &entries)
	return entries, err
}
