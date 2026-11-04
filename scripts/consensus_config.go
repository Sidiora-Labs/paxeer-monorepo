package scripts

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
)

func ConfigureConsensus(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var original map[string]any
	if _, err = toml.Decode(string(data), &original); err != nil {
		return err
	}
	consensus, ok := original["consensus"].(map[string]any)
	if !ok {
		return fmt.Errorf("missing [consensus] section")
	}
	desired := map[string]string{
		"unsafe-vote-timeout-override":          `"2s"`,
		"unsafe-commit-timeout-override":        `"2s"`,
		"unsafe-bypass-commit-timeout-override": "false",
	}
	section := regexp.MustCompile(`^\s*\[([^\]]+)\]\s*(?:#.*)?$`)
	assignment := regexp.MustCompile(`^\s*(?:#\s*)?([A-Za-z0-9_-]+)\s*=`)
	seen := make(map[string]int)
	current := ""
	lines := strings.Split(string(data), "\n")
	for index, line := range lines {
		if match := section.FindStringSubmatch(line); match != nil {
			current = match[1]
			continue
		}
		if current != "consensus" {
			continue
		}
		match := assignment.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		if value, exists := desired[match[1]]; exists {
			seen[match[1]]++
			lines[index] = match[1] + " = " + value
		}
	}
	for key := range desired {
		if seen[key] != 1 {
			return fmt.Errorf("[consensus] requires exactly one intended key %s (found %d)", key, seen[key])
		}
	}
	// Decode and compare the entire tree, refusing edits inside TOML multiline strings.
	consensus["unsafe-vote-timeout-override"] = "2s"
	consensus["unsafe-commit-timeout-override"] = "2s"
	consensus["unsafe-bypass-commit-timeout-override"] = false
	updated := strings.Join(lines, "\n")
	var decoded map[string]any
	if _, err = toml.Decode(updated, &decoded); err != nil {
		return err
	}
	if !reflect.DeepEqual(original, decoded) {
		return fmt.Errorf("consensus edit would change unrelated settings")
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	return publishFile(path, bytes.NewBufferString(updated), info.Mode().Perm())
}
