package rolecheck

import (
	"path/filepath"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
)

func TestStreamRelayStatementLines(t *testing.T) {
	root, err := filepath.Abs(coordinatortest.RepositoryRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	statements, err := Statements()
	if err != nil {
		t.Fatal(err)
	}
	const file = "sdk/launchpad/shared/delivery/stream.go"
	source, err := openSource(root, file)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, statement := range statements {
		if statement.File != file {
			continue
		}
		count++
		t.Run(statement.ID, func(t *testing.T) {
			fn := source.function(statement.Function)
			if fn == nil {
				t.Fatalf("statement function absent: %s", statement.Function)
			}
			expression := source.expression(fn, statement.Expression)
			if expression == nil {
				t.Fatal("statement expression absent")
			}
			if line := source.files.Position(expression.Pos()).Line; line != statement.Line {
				t.Fatalf("%s:%s line=%d, registered=%d", file, statement.Function, line, statement.Line)
			}
		})
	}
	if count != 11 {
		t.Fatalf("stream relay statements=%d, want 11", count)
	}
}
