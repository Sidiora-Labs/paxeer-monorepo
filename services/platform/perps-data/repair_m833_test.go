package perpsdata

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestRepairM_833_MissingSetupFails(t *testing.T) {
	inputs := []string{e2eDatabaseEnv, e2eRPCEnv, e2eCometEnv}
	for _, journey := range []struct {
		name string
		run  func(*testing.T)
	}{
		{"TestEndToEnd", TestEndToEnd},
		{"TestEndToEndChainMarkets", TestEndToEndChainMarkets},
		{"TestEndToEndGraphQLMarkets", TestEndToEndGraphQLMarkets},
	} {
		for present := 0; present < (1<<len(inputs))-1; present++ {
			t.Run(fmt.Sprintf("%s/inputs_%03b", journey.name, present), func(t *testing.T) {
				for i, name := range inputs {
					value := ""
					if present&(1<<i) != 0 {
						value = "unused-setup-input"
					}
					t.Setenv(name, value)
				}

				// An independent Go test runner lets the real journey fail setup
				// without failing its parent or requiring a subprocess or test double.
				outputPath := filepath.Join(t.TempDir(), "setup.log")
				output, err := os.Create(outputPath)
				if err != nil {
					t.Fatal(err)
				}
				stdout := os.Stdout
				var runs int
				var failed, skipped, returned bool
				passed := func() bool {
					os.Stdout = output
					defer func() {
						os.Stdout = stdout
						if err := output.Close(); err != nil {
							t.Errorf("close setup output: %v", err)
						}
					}()
					return testing.RunTests(regexp.MatchString, []testing.InternalTest{{
						Name: "TestRepairM_833_SetupProbe",
						F: func(probe *testing.T) {
							runs++
							defer func() {
								failed, skipped = probe.Failed(), probe.Skipped()
							}()
							journey.run(probe)
							returned = true
						},
					}})
				}()
				if runs != 1 || passed || !failed || skipped || returned {
					t.Fatalf("%s with inputs %03b: runs=%d passed=%t failed=%t skipped=%t returned=%t; want one fatal setup failure without a skip", journey.name, present, runs, passed, failed, skipped, returned)
				}
				raw, err := os.ReadFile(outputPath)
				if err != nil {
					t.Fatal(err)
				}
				want := fmt.Sprintf("%s requires %s, %s and %s to all be set", journey.name, inputs[0], inputs[1], inputs[2])
				if !strings.Contains(string(raw), want) {
					t.Fatalf("setup diagnostic = %q, want %q before opening any dependency", raw, want)
				}
			})
		}
	}
}
