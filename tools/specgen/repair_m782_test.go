package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestRepairM_782_QuotedValues(t *testing.T) {
	cases := []struct {
		name, token, want string
	}{
		{"escaped_quote_before_hash", `"say \"hello # world\" now"`, `say "hello # world" now`},
		{"escaped_quote_before_comma", `"say \", next"`, `say ", next`},
		{"hash", `"literal # hash"`, `literal # hash`},
		{"backslashes", `"C:\\spec\\"`, `C:\spec\`},
		{"backslash_then_quote", `"slash \\\"# and comma, remain"`, `slash \"# and comma, remain`},
		{"escaped_controls", `"one\ntwo\tthree\rfour"`, "one\ntwo\tthree\rfour"},
		{"unicode", `"\u03bb \"#\" 雪"`, "λ \"#\" 雪"},
		{"empty", `""`, ""},
		{"bare", `bare value`, "bare value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "quoted.kvx")
			src := "[req.1] # section comment\ntitle = " + tc.token + " # outside \"quote\"\nac_1 = " + tc.token + " # discarded\n"
			if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
				t.Fatal(err)
			}
			doc, err := ParseFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if got := doc.Raw("req.1", "title"); got != tc.token {
				t.Errorf("Raw = %q, want %q", got, tc.token)
			}
			if got := doc.Str("req.1", "title"); got != tc.want {
				t.Errorf("Str = %q, want %q", got, tc.want)
			}
			wantKV := [][2]string{{"ac_1", tc.want}}
			if got := doc.OrderedKV("req.1", "ac_"); !reflect.DeepEqual(got, wantKV) {
				t.Errorf("OrderedKV = %q, want %q", got, wantKV)
			}
			var wantList []string
			if tc.want != "" {
				wantList = []string{tc.want}
			}
			if got := doc.List("req.1", "title"); !reflect.DeepEqual(got, wantList) {
				t.Errorf("scalar List = %q, want %q", got, wantList)
			}
			want := "<!-- repair -->\n\n# Requirements\n\n## Requirement 1: " + tc.want + "\n\n### Acceptance Criteria\n\n1. " + tc.want + "\n\n"
			if got := renderRequirements(doc, "repair"); got != want {
				t.Errorf("renderRequirements = %q, want %q", got, want)
			}
		})
	}
}

func TestRepairM_782_Lists(t *testing.T) {
	cases := []struct {
		name, token string
		want        []string
	}{
		{"escaped_quote_and_comma", `["first \", still first # hash", "second"]`, []string{`first ", still first # hash`, "second"}},
		{"backslash_before_delimiter", `["C:\\", "after", "# hash"]`, []string{`C:\`, "after", "# hash"}},
		{"backslash_quote_and_hash", `["a\\\"#,b", "c\\\\", bare]`, []string{`a\"#,b`, `c\\`, "bare"}},
		{"empty_elements", `["", "kept", ""]`, []string{"kept"}},
		{"empty_list", `[]`, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := parse(strings.NewReader("[design.vector]\nitems = "+tc.token+" # outside\n"), "list.kvx")
			if err != nil {
				t.Fatal(err)
			}
			if got := doc.Raw("design.vector", "items"); got != tc.token {
				t.Errorf("Raw = %q, want %q", got, tc.token)
			}
			if !doc.IsList("design.vector", "items") {
				t.Fatal("bracketed value must be a list")
			}
			if got := doc.List("design.vector", "items"); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("List = %q, want %q", got, tc.want)
			}
			want := "<!-- repair -->\n\n# Design\n\n"
			for _, item := range tc.want {
				want += "- " + item + "\n"
			}
			want += "\n"
			got, err := renderDesign(doc, "", "repair")
			if err != nil || got != want {
				t.Errorf("renderDesign = %q, %v; want %q", got, err, want)
			}
		})
	}
	for n := 0; n <= 8; n++ {
		t.Run("backslash_parity_"+strconv.Itoa(n), func(t *testing.T) {
			want := []string{strings.Repeat(`\`, n) + `"#,inside`, "tail" + strings.Repeat(`\`, n), "last"}
			var tokens []string
			for _, value := range want {
				tokens = append(tokens, strconv.Quote(value))
			}
			src := "[vector]\nitems = [" + strings.Join(tokens, ", ") + "] # outside\n"
			doc, err := parse(strings.NewReader(src), "parity.kvx")
			if err != nil {
				t.Fatal(err)
			}
			if got := doc.List("vector", "items"); !reflect.DeepEqual(got, want) {
				t.Errorf("List = %q, want %q", got, want)
			}
		})
	}
}

func TestRepairM_782_ProducerToRenderer(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("spec/workflow.kvx", `[meta]
name = "Spec \"#\" workflow"
active_feature = "fixture"
[status_values]
task = ["pending", "in_progress", "done"]
[render]
banner = "repair"
markdown_mirror = true
[adapters]
agents = "AGENTS.md"
[loop]
first = "Keep \"# and ,\" in C:\\spec\\"
`)
	write("spec/fixture/spec.kvx", `[req.1]
title = "Keep \"# and ,\" in C:\\spec\\" # actual comment
story = "Decode \"quotes\" once"
ac_1 = "Preserve \\\"#,comma and \\slashes" # actual comment
[design]
overview = "Literal \"# overview\""
[design.vector]
items = ["first \", still first # hash", "C:\\", "last"] # actual comment
[task.1]
title = "Render \"# task\""
status = "done"
wave = 1
requires = []
reqs = ["1"]
touches = ["C:\\spec\\", "a\",b#c"]
verify_cmd = "printf \"# quoted\""
do_1 = "Keep \"# and ,\" in C:\\spec\\"
`)
	w := &writer{}
	if err := run(root, w); err != nil {
		t.Fatal(err)
	}
	if w.written != 4 || w.stale != 0 {
		t.Fatalf("writer = %+v, want four outputs and no stale files", w)
	}
	wants := map[string]string{
		"spec/fixture/requirements.md": "<!-- repair -->\n\n# Requirements\n\n## Requirement 1: Keep \"# and ,\" in C:\\spec\\\n\n**User Story:** Decode \"quotes\" once\n\n### Acceptance Criteria\n\n1. Preserve \\\"#,comma and \\slashes\n\n",
		"spec/fixture/design.md":       "<!-- repair -->\n\n# Design\n\nLiteral \"# overview\"\n\n- first \", still first # hash\n- C:\\\n- last\n\n",
		"spec/fixture/tasks.md":        "<!-- repair -->\n\n# Implementation Plan\n\n## Tasks\n\n- [x] 1. Render \"# task\"\n  - Keep \"# and ,\" in C:\\spec\\\n  - _Requirements: 1_\n\n## Task Dependency Graph\n\n```json\n{\n  \"waves\": [\n    { \"id\": 1,  \"tasks\": [\"1\"] }\n  ]\n}\n```\n",
	}
	for rel, want := range wants {
		got, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("%s = %q, want %q", rel, got, want)
		}
	}
	got, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# Spec \"#\" workflow\n", "1. Keep \"# and ,\" in C:\\spec\\\n"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("adapter missing decoded content %q: %s", want, got)
		}
	}
}

func TestRepairM_782_RejectMalformedStrings(t *testing.T) {
	cases := []string{
		`"unfinished`,
		`"dangling\"`,
		`"bad\q"`,
		`"bad\x0"`,
		`"valid" trailing`,
		`["good", "bad\q"]`,
		`["good", "unfinished]`,
	}
	for _, token := range cases {
		t.Run(token, func(t *testing.T) {
			doc, err := parse(strings.NewReader("[vector]\nvalue = "+token+"\n"), "malformed.kvx")
			if err == nil || doc != nil {
				t.Fatalf("malformed token %q accepted: doc=%v err=%v", token, doc, err)
			}
			if !strings.Contains(err.Error(), `malformed.kvx line 2: invalid value for "value"`) {
				t.Fatalf("error lacks source context: %v", err)
			}
		})
	}
}
