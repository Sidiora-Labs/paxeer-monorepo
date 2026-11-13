package featureacceptance

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strings"
)

const SpecPath = "spec/layerx-merge/spec.kvx"
const retirementTitle = "The removed launchpad production-system spec tree is retired"
const declarationDirectory = "tools/launchpad/production/feature-acceptance/"

var legacySection = regexp.MustCompile(`^(task\.(0\.[123]|[1-4]\.(0[1-9]|[1-4][0-9]|50|99)|9\.1)|req\.D[0-9]{2}_[0-9]{3}|coverage\.(PF(0[1-9]|[12][0-9]|30)|OC(0[1-9]|1[0-9]|2[0-5])))$`)

// SpecReference reports identity lineage, never execution or qualification.
type SpecReference struct {
	Spec    string `json:"spec"`
	Section string `json:"section"`
	Class   string `json:"class"`
	Reason  string `json:"reason"`
}

type ReferenceReport struct {
	Document         string          `json:"document"`
	References       []SpecReference `json:"references"`
	ReleaseQualified bool            `json:"release_qualified"`
}

// RequireCurrent keeps retired declarations out of qualification consumers.
func (report ReferenceReport) RequireCurrent() error {
	if len(report.References) == 0 {
		return fmt.Errorf("no specification references")
	}
	for _, ref := range report.References {
		if ref.Class != "current-reference" {
			return fmt.Errorf("retired release reference cannot qualify: %s", ref.Reason)
		}
	}
	return nil
}

type referenceGraph struct {
	sections map[string]bool
	retired  bool
}

func readReferenceGraph(root fs.FS) (referenceGraph, error) {
	source, err := root.Open(SpecPath)
	if err != nil {
		return referenceGraph{}, err
	}
	defer source.Close()
	graph := referenceGraph{sections: make(map[string]bool)}
	scanner := bufio.NewScanner(source)
	scanner.Buffer(make([]byte, 4096), 4*1024*1024)
	section := ""
	retiredTitle, retainedInputs := false, false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			if graph.sections[section] {
				return referenceGraph{}, fmt.Errorf("duplicate specification section %s", section)
			}
			graph.sections[section] = true
		} else if section == "decision.1803" && strings.HasPrefix(line, "title = ") && strings.Contains(line, retirementTitle) {
			retiredTitle = true
		} else if section == "decision.1803" && strings.HasPrefix(line, "value = ") && strings.Contains(line, "sdk/launchpad/protocol/contracts/v1/") {
			retainedInputs = true
		}
	}
	graph.retired = retiredTitle && retainedInputs
	if err := scanner.Err(); err != nil {
		return referenceGraph{}, err
	}
	if !graph.sections["meta"] {
		return referenceGraph{}, fmt.Errorf("authoritative graph has no meta section")
	}
	return graph, nil
}

func (graph referenceGraph) reference(section string) (SpecReference, error) {
	ref := SpecReference{Spec: SpecPath, Section: section}
	if graph.sections[section] {
		ref.Class = "current-reference"
		ref.Reason = "Declared in " + SpecPath
	} else {
		if !legacySection.MatchString(section) {
			return SpecReference{}, fmt.Errorf("unresolved current specification reference: %s", section)
		}
		if !graph.retired {
			return SpecReference{}, fmt.Errorf("missing retirement authority for %s", section)
		}
		ref.Class = "retired-reference"
		ref.Reason = "decision.1803 retires the launchpad production-system specification; " + section + " is absent from " + SpecPath
	}
	return ref, nil
}

// ReconcileReference reads the authoritative tracked source. Numeric neighbours
// are never substituted for an absent legacy identity.
func ReconcileReference(root fs.FS, section string) (SpecReference, error) {
	graph, err := readReferenceGraph(root)
	if err != nil {
		return SpecReference{}, err
	}
	return graph.reference(section)
}

type referenceGate struct {
	Command   string        `json:"verify_cmd"`
	Reference SpecReference `json:"reference"`
}

type referenceDocument struct {
	Task             string                   `json:"task"`
	Command          string                   `json:"verify_cmd"`
	Reference        *SpecReference           `json:"spec_reference"`
	References       []SpecReference          `json:"references"`
	Gates            map[string]referenceGate `json:"mandatory_gates"`
	ReleaseQualified bool                     `json:"release_qualified"`
}

// AuditReferenceDocument consumes the materialized Python producer's JSON and
// checks every reference against the graph before returning its report.
func AuditReferenceDocument(root fs.FS, document string) (ReferenceReport, error) {
	report := ReferenceReport{Document: document}
	graph, err := readReferenceGraph(root)
	if err != nil {
		return report, err
	}
	raw, err := fs.ReadFile(root, document)
	if err != nil {
		return report, err
	}
	var declaration referenceDocument
	if err = json.Unmarshal(raw, &declaration); err != nil {
		return report, err
	}
	if declaration.ReleaseQualified {
		return report, fmt.Errorf("declaration cannot establish release qualification")
	}
	seen := make(map[string]bool)
	check := func(ref SpecReference) error {
		expected, err := graph.reference(ref.Section)
		if err != nil {
			return err
		}
		if ref != expected {
			return fmt.Errorf("stale or invalid reference for %s", ref.Section)
		}
		if !seen[ref.Section] {
			report.References = append(report.References, ref)
			seen[ref.Section] = true
		}
		return nil
	}
	command := func(value, task string) error {
		expected := "python3 tools/launchpad/production/qualify.py --task " + task + " --spec " + SpecPath + " --require-current-source"
		if value != expected {
			return fmt.Errorf("task/source/current-source command mismatch: %s", task)
		}
		return nil
	}
	if declaration.Task != "" {
		if declaration.Reference == nil || declaration.Reference.Section != "task."+declaration.Task {
			return report, fmt.Errorf("task reference missing")
		}
		if err = command(declaration.Command, declaration.Task); err != nil {
			return report, err
		}
		if err = check(*declaration.Reference); err != nil {
			return report, err
		}
	}
	for _, ref := range declaration.References {
		if err = check(ref); err != nil {
			return report, err
		}
	}
	for id, gate := range declaration.Gates {
		if gate.Reference.Section != "task."+id {
			return report, fmt.Errorf("gate reference identity mismatch: %s", id)
		}
		if err = command(gate.Command, id); err != nil {
			return report, err
		}
		if err = check(gate.Reference); err != nil {
			return report, err
		}
	}
	sort.Slice(report.References, func(i, j int) bool { return report.References[i].Section < report.References[j].Section })
	if len(report.References) == 0 {
		return report, fmt.Errorf("reference report is empty")
	}
	return report, nil
}
