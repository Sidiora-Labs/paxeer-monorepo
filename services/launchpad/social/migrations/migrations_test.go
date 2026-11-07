package migrations

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestEveryReferencedChatRelationIsCreated(t *testing.T) {
	created := make(map[string]bool)
	declaration := regexp.MustCompile(`(?i)\bCREATE\s+(?:OR\s+REPLACE\s+)?(?:TABLE|FUNCTION|VIEW)\s+(?:IF\s+NOT\s+EXISTS\s+)?chat\s*\.\s*([a-z_][a-z0-9_]*)\b`)
	migrations, err := fs.Glob(FS, "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) == 0 {
		t.Fatal("no embedded SQL migrations found")
	}
	for _, name := range migrations {
		data, err := FS.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		var up strings.Builder
		inUp := false
		for _, line := range strings.Split(string(data), "\n") {
			switch strings.TrimSpace(line) {
			case "-- +goose Up":
				inUp = true
			case "-- +goose Down":
				inUp = false
			default:
				if inUp && !strings.HasPrefix(strings.TrimSpace(line), "--") {
					up.WriteString(line)
					up.WriteByte('\n')
				}
			}
		}
		for _, match := range declaration.FindAllStringSubmatch(up.String(), -1) {
			created[strings.ToLower(match[1])] = true
		}
	}

	reference := regexp.MustCompile(`(?i)\b(?:FROM|JOIN|INTO|UPDATE|TABLE)\s+(?:IF\s+NOT\s+EXISTS\s+)?chat\s*\.\s*([a-z_][a-z0-9_]*)\b|\bchat\s*\.\s*([a-z_][a-z0-9_]*)\s*\(`)
	referenced := make(map[string][]string)
	files := 0
	fset := token.NewFileSet()
	err = filepath.WalkDir(filepath.Join("..", "internal"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		files++
		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			sql, err := strconv.Unquote(literal.Value)
			if err != nil {
				t.Errorf("%s: %v", fset.Position(literal.Pos()), err)
				return true
			}
			for _, match := range reference.FindAllStringSubmatch(sql, -1) {
				name := match[1]
				if name == "" {
					name = match[2]
				}
				name = strings.ToLower(name)
				referenced[name] = append(referenced[name], fset.Position(literal.Pos()).String())
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files == 0 || len(referenced) == 0 {
		t.Fatalf("scan found %d Go files and %d chat relations", files, len(referenced))
	}
	names := make([]string, 0, len(referenced))
	for name := range referenced {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			if !created[name] {
				t.Errorf("chat.%s is referenced at %s but never created in an embedded goose Up section", name, strings.Join(referenced[name], ", "))
			}
		})
	}
}
