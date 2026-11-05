package repotest_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"golang.org/x/mod/modfile"
)

type makefileRunSelector struct {
	command  string
	expr     string
	packages []string
}

func makefileRunSelectors(source string) ([]makefileRunSelector, error) {
	source = strings.ReplaceAll(source, "\r\n", "\n")
	source = strings.ReplaceAll(source, "\\\n", " ")
	goTest := regexp.MustCompile(`(?:^|[\s;(&|@])(?:go|\$\(GO\)|\$\{GO\})\s+test\b`)
	var selectors []makefileRunSelector
	for _, line := range strings.Split(source, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		for _, loc := range goTest.FindAllStringIndex(line, -1) {
			command := strings.TrimSpace(line[loc[0]:])
			words, err := makefileTestWords(line[loc[1]:])
			if err != nil {
				return nil, fmt.Errorf("%s: %w", command, err)
			}
			selector := makefileRunSelector{command: command}
			hasRun := false
			for i := 0; i < len(words); i++ {
				if words[i] == "-args" {
					break
				}
				flag := strings.TrimPrefix(words[i], "-")
				flag = strings.TrimPrefix(flag, "-")
				name, value, equal := strings.Cut(flag, "=")
				if strings.HasPrefix(words[i], "-") && name == "run" {
					if !equal {
						i++
						if i == len(words) {
							return nil, fmt.Errorf("%s: -run has no argument", command)
						}
						value = words[i]
					}
					selector.expr, hasRun = value, true
				}
			}
			if !hasRun {
				continue
			}
			for i := 0; i < len(words); i++ {
				word := words[i]
				if word == "-args" {
					break
				}
				if !strings.HasPrefix(word, "-") {
					selector.packages = append(selector.packages, word)
					continue
				}
				name, _, equal := strings.Cut(strings.TrimLeft(word, "-"), "=")
				if equal {
					continue
				}
				switch name {
				case "a", "asan", "c", "cover", "failfast", "fullpath", "i", "json", "msan", "n", "race", "short", "v", "work", "x", "trimpath", "buildvcs":
				case "run", "bench", "benchtime", "blockprofile", "blockprofilerate", "count", "covermode", "coverpkg", "coverprofile", "cpu", "cpuprofile", "fuzz", "fuzzminimizetime", "fuzztime", "list", "memprofile", "memprofilerate", "mutexprofile", "mutexprofilefraction", "o", "outputdir", "parallel", "shuffle", "skip", "timeout", "trace", "vet", "asmflags", "buildmode", "compiler", "gccgoflags", "gcflags", "installsuffix", "ldflags", "mod", "modfile", "overlay", "p", "pgo", "pkgdir", "tags", "toolexec":
					i++
					if i == len(words) {
						return nil, fmt.Errorf("%s: %s has no argument", command, word)
					}
				default:
					return nil, fmt.Errorf("%s: cannot determine whether %s takes an argument", command, word)
				}
			}
			if len(selector.packages) == 0 {
				selector.packages = []string{"."}
			}
			selectors = append(selectors, selector)
		}
	}
	return selectors, nil
}

func makefileTestWords(command string) ([]string, error) {
	var words []string
	var word strings.Builder
	var quote byte
	started := false
	flush := func() {
		if started {
			words = append(words, strings.ReplaceAll(word.String(), "$$", "$"))
			word.Reset()
			started = false
		}
	}
	for i := 0; i < len(command); i++ {
		c := command[i]
		if c == '\\' && quote != '\'' {
			if i+1 == len(command) {
				return nil, fmt.Errorf("trailing shell escape")
			}
			next := command[i+1]
			if quote == 0 || strings.ContainsRune("$`\"\\", rune(next)) {
				i++
				c = next
			}
		} else if quote != 0 {
			if c == quote {
				quote = 0
				continue
			}
		} else {
			switch c {
			case '\'', '"':
				quote, started = c, true
				continue
			case ' ', '\t':
				flush()
				continue
			case ';', '&', '|', ')', '<', '>':
				flush()
				return words, nil
			case '#':
				if !started {
					return words, nil
				}
			}
		}
		started = true
		word.WriteByte(c)
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated shell quote")
	}
	flush()
	return words, nil
}

func makefileSelectorTestNames(pattern string) ([]string, error) {
	if !strings.HasPrefix(pattern, ".") && !filepath.IsAbs(pattern) {
		data, err := os.ReadFile("go.mod")
		if err != nil {
			return nil, err
		}
		module := modfile.ModulePath(data)
		if pattern != module && !strings.HasPrefix(pattern, module+"/") {
			return nil, fmt.Errorf("package pattern %q is not in root module %q", pattern, module)
		}
		pattern = "." + strings.TrimPrefix(pattern, module)
	}
	dirs := []string{filepath.Clean(pattern)}
	if strings.Contains(pattern, "...") {
		prefix, _, _ := strings.Cut(pattern, "...")
		root := filepath.Clean(prefix)
		expression := regexp.QuoteMeta(filepath.ToSlash(filepath.Clean(pattern)))
		expression = strings.ReplaceAll(expression, `/\.\.\.`, `(?:/.*)?`)
		expression = strings.ReplaceAll(expression, `\.\.\.`, `.*`)
		match := regexp.MustCompile("^" + expression + "$")
		dirs = nil
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !entry.IsDir() {
				return nil
			}
			if path != root {
				if strings.HasPrefix(entry.Name(), ".") || strings.HasPrefix(entry.Name(), "_") || entry.Name() == "vendor" || entry.Name() == "testdata" {
					return filepath.SkipDir
				}
				if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
					return filepath.SkipDir
				} else if !os.IsNotExist(err) {
					return err
				}
			}
			if match.MatchString(filepath.ToSlash(path)) {
				dirs = append(dirs, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	var names []string
	for _, dir := range dirs {
		packages, err := parser.ParseDir(token.NewFileSet(), dir, func(info fs.FileInfo) bool {
			return strings.HasSuffix(info.Name(), "_test.go")
		}, 0)
		if err != nil {
			return nil, err
		}
		for _, pkg := range packages {
			for _, file := range pkg.Files {
				for _, decl := range file.Decls {
					fn, ok := decl.(*ast.FuncDecl)
					if !ok || fn.Recv != nil || !strings.HasPrefix(fn.Name.Name, "Test") || fn.Name.Name == "TestMain" {
						continue
					}
					suffix, _ := utf8.DecodeRuneInString(strings.TrimPrefix(fn.Name.Name, "Test"))
					if !unicode.IsLower(suffix) {
						names = append(names, fn.Name.Name)
					}
				}
			}
		}
	}
	return names, nil
}

func makefileSelectorMatches(selector makefileRunSelector) (bool, error) {
	expression, err := regexp.Compile(selector.expr)
	if err != nil {
		return false, err
	}
	matched := false
	for _, pattern := range selector.packages {
		names, err := makefileSelectorTestNames(pattern)
		if err != nil {
			return false, fmt.Errorf("%s: %w", pattern, err)
		}
		for _, name := range names {
			matched = matched || expression.MatchString(name)
		}
	}
	return matched, nil
}

func TestMakefileRunSelectorsMatchTests(t *testing.T) {
	for _, path := range []string{"Makefile", "chain.mk"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		selectors, err := makefileRunSelectors(string(data))
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for _, selector := range selectors {
			matched, err := makefileSelectorMatches(selector)
			if err != nil {
				t.Errorf("%s: command %q, -run %q: %v", path, selector.command, selector.expr, err)
			} else if !matched {
				t.Errorf("%s: command %q: -run %q matches no Test function in %v", path, selector.command, selector.expr, selector.packages)
			}
		}
	}

	t.Run("guard verification command", func(t *testing.T) {
		for _, command := range []string{
			"go test -count=1 ./tools/repotest/ -run '^TestMakefileRunSelectorsMatchTests$'",
			"go test -count 1 \\\n\t./tools/repotest/... -run=\"^TestMakefileRunSelectorsMatchTests$$\"",
		} {
			selectors, err := makefileRunSelectors(command)
			if err != nil || len(selectors) != 1 {
				t.Fatalf("parse %q: selectors=%v, error=%v", command, selectors, err)
			}
			selector := selectors[0]
			if selector.expr != "^TestMakefileRunSelectorsMatchTests$" || len(selector.packages) != 1 {
				t.Fatalf("incorrectly parsed command %q: %+v", command, selector)
			}
			matched, err := makefileSelectorMatches(selector)
			if err != nil || !matched {
				t.Fatalf("guard cannot match its verification command %q: %v", command, err)
			}
			selector.expr = "^TestMakefileRunSelectorsMatchTests_NoSuchTest$"
			matched, err = makefileSelectorMatches(selector)
			if err != nil || matched {
				t.Fatalf("guard must reject unmatched -run %q: matched=%v, error=%v", selector.expr, matched, err)
			}
		}
	})
}
