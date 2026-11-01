package exportcheck

import (
	"bytes"
	"fmt"
	"go/importer"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"strings"
)

func PackageExports(importPath string) (map[string]bool, error) {
	lookup := func(path string) (io.ReadCloser, error) {
		cmd := exec.Command("go", "list", "-export", "-f", "{{.Export}}", "--", path)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		output, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("go list %q: %w: %s", path, err, strings.TrimSpace(stderr.String()))
		}
		exportPath := strings.TrimSpace(string(output))
		if exportPath == "" {
			return nil, fmt.Errorf("package %q has no compiled export file", path)
		}
		file, err := os.Open(exportPath)
		if err != nil {
			return nil, fmt.Errorf("open export data for %q: %w", path, err)
		}
		info, err := file.Stat()
		if err != nil {
			file.Close()
			return nil, fmt.Errorf("stat export data for %q: %w", path, err)
		}
		if info.Size() == 0 {
			file.Close()
			return nil, fmt.Errorf("package %q has an empty compiled export file", path)
		}
		return file, nil
	}

	pkg, err := importer.ForCompiler(token.NewFileSet(), "gc", lookup).Import(importPath)
	if err != nil {
		return nil, fmt.Errorf("read compiled exports for %q: %w", importPath, err)
	}
	exports := make(map[string]bool)
	for _, name := range pkg.Scope().Names() {
		object := pkg.Scope().Lookup(name)
		if !object.Exported() {
			continue
		}
		exports[name] = true
		typeName, ok := object.(*types.TypeName)
		if !ok {
			continue
		}
		named, ok := types.Unalias(typeName.Type()).(*types.Named)
		if !ok {
			continue
		}
		for i := 0; i < named.NumMethods(); i++ {
			method := named.Method(i)
			if method.Exported() {
				exports[name+"."+method.Name()] = true
			}
		}
	}
	return exports, nil
}
