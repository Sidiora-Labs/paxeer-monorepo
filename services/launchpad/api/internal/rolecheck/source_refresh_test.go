package rolecheck

import (
	"go/ast"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
)

func TestSourceRefreshStatementInventory(t *testing.T) {
	root, err := filepath.Abs(coordinatortest.RepositoryRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	statements, err := Statements()
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string]*sourceFile{}
	count, renewals, marks := 0, 0, 0
	for _, statement := range statements {
		if statement.Package != "services/launchpad/notifications" && statement.Package != "services/launchpad/discovery" {
			continue
		}
		count++
		t.Run(statement.ID, func(t *testing.T) {
			source := sources[statement.File]
			if source == nil {
				var err error
				source, err = openSource(root, statement.File)
				if err != nil {
					t.Fatal(err)
				}
				sources[statement.File] = source
			}
			fn := source.function(statement.Function)
			if fn == nil {
				t.Fatalf("statement function absent: %s", statement.Function)
			}
			registered := source.sqlExpressions(fn)
			var expressions []ast.Expr
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				if expression, ok := node.(ast.Expr); ok && registered[source.text(expression)] {
					expressions = append(expressions, expression)
				}
				return true
			})
			number, err := strconv.Atoi(statement.ID[strings.LastIndex(statement.ID, ".")+1:])
			if err != nil || number < 1 || number > len(expressions) {
				t.Fatalf("statement number absent: %s", statement.ID)
			}
			expression := expressions[number-1]
			if source.text(expression) != statement.Expression {
				t.Fatalf("statement %d expression differs: %s", number, source.text(expression))
			}
			if line := source.files.Position(expression.Pos()).Line; line != statement.Line {
				t.Fatalf("%s:%s line=%d, registered=%d", statement.File, statement.Function, line, statement.Line)
			}
			if statement.Function == "Store.RenewSources" {
				renewals++
				if statement.ID != "discovery/source_refresh.Store.RenewSources.1" || !slices.Equal(statement.Kinds, []string{"UPDATE"}) || !slices.Equal(statement.Tables, []string{"narrative_discovery.sources"}) || !slices.Equal(statement.Roles, []string{"api_rest_runtime"}) {
					t.Fatalf("source renewal authority: %+v", statement)
				}
			}
			if slices.Contains(statement.Tables, "narrative_discovery.consumer_events") {
				marks++
				if statement.ID != "discovery/store.Store.Apply.2" || statement.Function != "Store.Apply" {
					t.Fatalf("source event authority: %+v", statement)
				}
			}
		})
	}
	if count != 114 || renewals != 1 || marks != 1 {
		t.Fatalf("source statements=%d renewals=%d marks=%d", count, renewals, marks)
	}
}
