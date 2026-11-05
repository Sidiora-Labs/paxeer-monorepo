package main

import (
	"fmt"
	"strings"
)

// buildBrief composes the shared, IDE-neutral operating brief from
// workflow.kvx. Every per-IDE adapter wraps this same body.
func buildBrief(wf *Doc) string {
	var b strings.Builder
	name := wf.Str("meta", "name")
	fmt.Fprintf(&b, "# %s\n\n", name)
	b.WriteString("This file is a generated pointer. The single workflow source is `spec/workflow.kvx`; edit it and run `hyperfide spec render`, never this file.\n")
	if owners := wf.Str("render", "generated_owners"); owners != "" {
		fmt.Fprintf(&b, "%s\n", owners)
	}

	if active := wf.Str("meta", "active_feature"); active != "" {
		b.WriteString("\n## Active feature\n\n")
		fmt.Fprintf(&b, "`spec/%s/spec.kvx` — read its `[meta]` status, `[req.*]` acceptance criteria, and the `[task.*]` list (status + wave + requires). Work one task at a time in wave order; update `status` in the kvx as you go.\n", active)
	}

	b.WriteString("\n## The loop (every session)\n\n")
	for i, kv := range wf.OrderedKV("loop", "") {
		fmt.Fprintf(&b, "%d. %s\n", i+1, kv[1])
	}

	var sections []string
	for _, section := range []string{"principle", "hard_rules", "status_values", "human", "asks", "publication", "memory"} {
		if wf.Has(section) {
			sections = append(sections, "`["+section+"]`")
		}
	}
	if len(sections) > 0 {
		b.WriteString("\n## Where the rules are\n\n")
		fmt.Fprintf(&b, "Principles, hard rules and policies live only in `spec/workflow.kvx`: %s. Read them there.\n", strings.Join(sections, ", "))
	}

	return b.String()
}

// adapter renders one IDE pointer file from the shared brief + a banner.
func adapter(label, brief, banner string) string {
	owned := "<!-- hyperfide-owned: generated-spec v1 -->\n"
	cmt := "<!-- " + banner + " -->\n\n"
	switch label {
	case "cursor":
		// Cursor rules: .mdc with YAML frontmatter; alwaysApply keeps it on.
		fm := "---\n" +
			"description: Ion portable spec workflow (task-driven + cortex)\n" +
			"alwaysApply: true\n" +
			"---\n\n"
		return owned + fm + cmt + brief
	case "windsurf_workflow":
		return owned + cmt + brief
	case "kiro":
		// Kiro steering: frontmatter inclusion=always.
		fm := "---\n" +
			"inclusion: always\n" +
			"---\n\n"
		return owned + fm + cmt + brief
	default:
		// windsurf_rules, claude (CLAUDE.md), codex (AGENTS.md), copilot — plain md.
		return owned + cmt + brief
	}
}

// humanize turns a snake_case kvx key into a Title Cased label.
func humanize(k string) string {
	parts := strings.Split(k, "_")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}
