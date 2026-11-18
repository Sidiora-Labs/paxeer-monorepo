package rolecheck

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:generate go run ./cmd/rolecheckgen -root ../../../../.. -select services/launchpad/social/internal/app/app.go:socialConnectedRuntime.publicResource

// WriteStatementLocations renews inventory locations from the SQL call sites.
// Object descriptions remain bound to identical SQL; changed SQL requires a new
// description of its dependencies.
func WriteStatementLocations(root string, selections ...string) error {
	path := filepath.Join(root, "services/launchpad/social/internal/rolecheck/statements.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var statements []Statement
	if err = json.Unmarshal(raw, &statements); err != nil {
		return err
	}
	locations := map[string]bool{}
	for _, s := range statements {
		locations[fmt.Sprintf("%s:%s:%d", s.File, s.Function, s.Ordinal)] = true
	}
	sources := &sourceReader{root: root, fset: token.NewFileSet(), files: map[string]*ast.File{}}
	for i := range statements {
		s := &statements[i]
		file, err := sources.read(s.File)
		if err != nil {
			return err
		}
		var matches []Statement
		for _, declaration := range file.Decls {
			fn, ok := declaration.(*ast.FuncDecl)
			if !ok {
				continue
			}
			for ordinal, call := range queryCalls(fn) {
				sql, err := sources.evaluate(call.Args[1], s.File, fn, call.Pos(), s.Bindings, 0)
				if err != nil || sql != s.SQL {
					continue
				}
				match := *s
				match.Function, match.Ordinal = functionName(fn), ordinal+1
				matches = append(matches, match)
			}
		}
		// A still-current location disambiguates repeated identical SQL.
		current := false
		for _, match := range matches {
			if match.Function == s.Function && match.Ordinal == s.Ordinal {
				current = true
				break
			}
		}
		if current {
			continue
		}
		unclaimed := matches[:0]
		for _, match := range matches {
			if !locations[fmt.Sprintf("%s:%s:%d", match.File, match.Function, match.Ordinal)] {
				unclaimed = append(unclaimed, match)
			}
		}
		matches = unclaimed
		if len(matches) != 1 {
			return fmt.Errorf("%s: expected one SQL location, found %d", s.ID, len(matches))
		}
		prefix, _, ok := strings.Cut(s.ID, ":")
		if !ok {
			return fmt.Errorf("%s: invalid statement identity", s.ID)
		}
		*s = matches[0]
		s.ID = fmt.Sprintf("%s:%s:%d", prefix, s.Function, s.Ordinal)
		locations[fmt.Sprintf("%s:%s:%d", s.File, s.Function, s.Ordinal)] = true
	}
	for _, selection := range selections {
		filePath, name, ok := strings.Cut(selection, ":")
		if !ok {
			return fmt.Errorf("invalid source selection %s", selection)
		}
		file, err := sources.read(filePath)
		if err != nil {
			return err
		}
		found := false
		for _, declaration := range file.Decls {
			fn, ok := declaration.(*ast.FuncDecl)
			if !ok || functionName(fn) != name {
				continue
			}
			found = true
			for ordinal, call := range queryCalls(fn) {
				key := fmt.Sprintf("%s:%s:%d", filePath, name, ordinal+1)
				if locations[key] {
					continue
				}
				sql, err := sources.evaluate(call.Args[1], filePath, fn, call.Pos(), nil, 0)
				if err != nil {
					return fmt.Errorf("%s: %w", key, err)
				}
				command := exec.Command("python3", filepath.Join(filepath.Dir(path), "describe_select.py"))
				command.Stdin = strings.NewReader(sql)
				description, err := command.Output()
				if err != nil {
					return fmt.Errorf("%s: describe SQL: %w", key, err)
				}
				var s Statement
				if err = json.Unmarshal(description, &s); err != nil {
					return err
				}
				s.ID = fmt.Sprintf("%s:%s:%d", strings.TrimPrefix(filePath, "services/launchpad/social/internal/"), name, ordinal+1)
				s.File, s.Function, s.Ordinal = filePath, name, ordinal+1
				s.Role, s.SQL, s.Bindings = RuntimeRole, sql, map[string]string{}
				statements = append(statements, s)
				locations[key] = true
			}
		}
		if !found {
			return fmt.Errorf("%s: function absent", selection)
		}
	}
	seen := map[string]bool{}
	for _, s := range statements {
		if seen[s.ID] {
			return fmt.Errorf("duplicate statement identity %s", s.ID)
		}
		seen[s.ID] = true
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(file)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	writeErr := encoder.Encode(statements)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}
