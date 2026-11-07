package cli

import (
	"regexp"
	"testing"

	"github.com/spf13/cobra"
)

func TestEVMTxUsageNamesRegisteredFlags(t *testing.T) {
	flagToken := regexp.MustCompile(`--([a-zA-Z0-9][a-zA-Z0-9-]*)`)
	documentedFlags := 0
	var visit func(*testing.T, *cobra.Command)
	visit = func(t *testing.T, cmd *cobra.Command) {
		for _, match := range flagToken.FindAllStringSubmatch(cmd.Use, -1) {
			documentedFlags++
			name := match[1]
			if cmd.LocalFlags().Lookup(name) == nil && cmd.InheritedFlags().Lookup(name) == nil {
				t.Errorf("%s usage names unregistered flag --%s: %q", cmd.CommandPath(), name, cmd.Use)
			}
		}
		for _, child := range cmd.Commands() {
			t.Run(child.Name(), func(t *testing.T) {
				visit(t, child)
			})
		}
	}

	cmd := GetTxCmd()
	if len(cmd.Commands()) == 0 {
		t.Fatal("GetTxCmd has no transaction subcommands")
	}
	visit(t, cmd)
	if documentedFlags == 0 {
		t.Fatal("no flags found in evm tx usage strings")
	}
}
