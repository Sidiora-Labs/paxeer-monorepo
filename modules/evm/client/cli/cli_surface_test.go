package cli

import (
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestEVMCLISurfaceFrozen(t *testing.T) {
	const txFlags = "account-number broadcast-mode dry-run fee-account fees from gas gas-adjustment gas-prices generate-only keyring-backend keyring-dir ledger node note offline output sequence sign-mode timeout-height yes"
	const queryFlags = "height node output"
	want := []struct {
		path  string
		flags string
	}{
		{"tx evm", ""},
		{"tx evm add-erc-native-pointer", txFlags},
		{"tx evm associate-address", txFlags + " evm-rpc"},
		{"tx evm associate-contract-address", txFlags},
		{"tx evm bind-erc-native-pointer", txFlags + " deposit"},
		{"tx evm call-contract", txFlags + " evm-rpc gas-fee-cap gas-limit nonce value"},
		{"tx evm call-precompile", txFlags + " evm-rpc gas-fee-cap gas-limit nonce value"},
		{"tx evm deploy", txFlags + " evm-rpc gas-fee-cap gas-limit nonce"},
		{"tx evm deploy-wpax", txFlags + " evm-rpc gas-fee-cap gas-limit nonce"},
		{"tx evm erc20-send", txFlags + " evm-rpc gas-fee-cap gas-limit nonce"},
		{"tx evm native-associate", txFlags},
		{"tx evm native-send", txFlags},
		{"tx evm print-claim", txFlags},
		{"tx evm print-claim-by-sender", txFlags},
		{"tx evm print-claim-specific", txFlags},
		{"tx evm register-cw-pointer", txFlags},
		{"tx evm register-evm-pointer", txFlags + " cw-address evm-rpc gas-fee-cap gas-limit"},
		{"tx evm send", txFlags + " evm-rpc gas-fee-cap gas-limit nonce"},
		{"query evm", ""},
		{"query evm erc1155-payload", queryFlags},
		{"query evm erc20", queryFlags},
		{"query evm erc20-payload", queryFlags},
		{"query evm erc721-payload", queryFlags},
		{"query evm evm-addr", queryFlags},
		{"query evm pax-addr", queryFlags},
		{"query evm payload", queryFlags},
		{"query evm pointee", queryFlags},
		{"query evm pointer", queryFlags},
		{"query evm pointer-version", queryFlags},
		{"query evm tx", "evm-rpc"},
	}

	if FlagGas != "gas-limit" {
		t.Errorf("changed FlagGas: got %q, want %q", FlagGas, "gas-limit")
	}
	if FlagRPC != "evm-rpc" {
		t.Errorf("changed FlagRPC: got %q, want %q", FlagRPC, "evm-rpc")
	}

	got := make(map[string]map[string]bool)
	var visit func(string, *cobra.Command)
	visit = func(parent string, cmd *cobra.Command) {
		path := parent + " " + cmd.Name()
		if _, exists := got[path]; exists {
			t.Errorf("duplicate command %q", path)
		}
		flagNames := make(map[string]bool)
		collect := func(flag *pflag.Flag) {
			flagNames[flag.Name] = true
		}
		cmd.LocalFlags().VisitAll(collect)
		cmd.InheritedFlags().VisitAll(collect)
		got[path] = flagNames
		for _, child := range cmd.Commands() {
			visit(path, child)
		}
	}
	visit("tx", GetTxCmd())
	visit("query", GetQueryCmd(""))

	for _, entry := range want {
		t.Run(entry.path, func(t *testing.T) {
			flagNames, exists := got[entry.path]
			if !exists {
				t.Errorf("removed command %q", entry.path)
				return
			}
			delete(got, entry.path)
			for _, name := range strings.Fields(entry.flags) {
				if !flagNames[name] {
					t.Errorf("removed flag %q from command %q", "--"+name, entry.path)
				}
				delete(flagNames, name)
			}
			var added []string
			for name := range flagNames {
				added = append(added, name)
			}
			sort.Strings(added)
			for _, name := range added {
				t.Errorf("added flag %q to command %q", "--"+name, entry.path)
			}
		})
	}
	var added []string
	for path := range got {
		added = append(added, path)
	}
	sort.Strings(added)
	for _, path := range added {
		t.Errorf("added command %q", path)
	}
}
