// Package scaffold renders the developer commands without invoking a compiler.
package scaffold

import (
	"bytes"
	_ "embed"
	"fmt"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"unicode"
	"unicode/utf8"

	"github.com/spf13/cobra"
)

//go:embed scenario.go.tmpl
var scenarioTemplate string

func validName(name string) bool {
	r, _ := utf8.DecodeRuneInString(name)
	return token.IsIdentifier(name) && unicode.IsUpper(r)
}

// RenderScenario leaves constructor arguments and contract operations explicit.
func RenderScenario(w io.Writer, name string) error {
	if !validName(name) {
		return fmt.Errorf("contract name %q must be an exported Go identifier", name)
	}
	t, err := template.New("scenario").Parse(scenarioTemplate)
	if err != nil {
		return err
	}
	return t.Execute(w, struct{ Name, Lower string }{name, strings.ToLower(name)})
}

const factoryStart = "// DO NOT EDIT BELOW THIS LINE - AUTO-GENERATED CONTENT"
const factoryEnd = "// DO NOT EDIT ABOVE THIS LINE - AUTO-GENERATED CONTENT"

// RenderFactory preserves manual registrations and all consumer functions.
func RenderFactory(source []byte, names []string) ([]byte, error) {
	start := bytes.Index(source, []byte(factoryStart))
	end := bytes.Index(source, []byte(factoryEnd))
	if start < 0 || end <= start {
		return nil, fmt.Errorf("scenario factory markers are missing or unordered")
	}
	start += len(factoryStart)
	var entries strings.Builder
	entries.WriteString("\n")
	seen := make(map[string]bool)
	for _, name := range names {
		if !validName(name) || seen[strings.ToLower(name)] {
			return nil, fmt.Errorf("invalid or repeated contract name %q", name)
		}
		seen[strings.ToLower(name)] = true
		fmt.Fprintf(&entries, "\t%s: New%sScenario,\n", name, name)
	}
	entries.WriteString("\n\t")
	result := append([]byte(nil), source[:start]...)
	result = append(result, entries.String()...)
	return append(result, source[end:]...), nil
}

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "scaffold", Short: "Generate contract scenario scaffolding"}
	cmd.AddCommand(&cobra.Command{
		Use: "scenario ContractName OutputFile", Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var content bytes.Buffer
			if err := RenderScenario(&content, args[0]); err != nil {
				return err
			}
			f, err := os.OpenFile(args[1], os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
			if os.IsExist(err) {
				cmd.Println("Preserving existing scenario: " + args[1])
				return nil
			} // Preserve implemented custom operations.
			if err != nil {
				return err
			}
			_, writeErr := f.Write(content.Bytes())
			closeErr := f.Close()
			if writeErr != nil {
				return writeErr
			}
			return closeErr
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use: "factory FactoryFile [ContractName...]", Args: cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			source, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			result, err := RenderFactory(source, args[1:])
			if err != nil {
				return err
			}
			f, err := os.CreateTemp(filepath.Dir(args[0]), ".factory-*")
			if err != nil {
				return err
			}
			defer os.Remove(f.Name())
			if err := f.Chmod(0644); err != nil {
				f.Close()
				return err
			}
			if _, err := f.Write(result); err != nil {
				f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
			return os.Rename(f.Name(), args[0])
		},
	})
	return cmd
}
