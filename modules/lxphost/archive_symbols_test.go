//go:build cgo

package lxphost

import (
	"bytes"
	"debug/elf"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func kernelArchiveMembers(t *testing.T) map[string]map[string]bool {
	t.Helper()
	path := filepath.Join("..", "..", "build", "liblayerx.a")
	archive, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read kernel archive %s (build with make paxd-native-libs): %v", path, err)
	}
	const magic = "!<arch>\n"
	if !bytes.HasPrefix(archive, []byte(magic)) {
		t.Fatalf("kernel archive %s has invalid System V ar magic", path)
	}
	members := make(map[string]map[string]bool)
	var longNames []byte
	for offset := len(magic); offset < len(archive); {
		if len(archive)-offset < 60 {
			t.Fatalf("kernel archive %s has a truncated header at byte %d", path, offset)
		}
		header := archive[offset : offset+60]
		if string(header[58:60]) != "`\n" {
			t.Fatalf("kernel archive %s has an invalid member header at byte %d", path, offset)
		}
		name := strings.TrimSpace(string(header[:16]))
		size, err := strconv.ParseUint(strings.TrimSpace(string(header[48:58])), 10, 63)
		if err != nil || size > uint64(len(archive)-offset-60) {
			t.Fatalf("kernel archive %s has an invalid member size at byte %d: %q", path, offset, header[48:58])
		}
		offset += 60
		body := archive[offset : offset+int(size)]
		offset += int(size)
		if size%2 != 0 {
			if offset >= len(archive) || archive[offset] != '\n' {
				t.Fatalf("kernel archive %s has invalid member padding at byte %d", path, offset)
			}
			offset++
		}
		switch name {
		case "/", "/SYM64/":
			continue
		case "//":
			longNames = body
			continue
		}
		if strings.HasPrefix(name, "/") {
			index, err := strconv.ParseUint(name[1:], 10, 63)
			if err != nil || index >= uint64(len(longNames)) {
				t.Fatalf("kernel archive %s has an invalid long-name reference %q", path, name)
			}
			if index != 0 && longNames[index-1] != '\n' {
				t.Fatalf("kernel archive %s has a long-name reference inside an entry: %q", path, name)
			}
			entry := longNames[index:]
			end := bytes.Index(entry, []byte("/\n"))
			if end < 0 {
				t.Fatalf("kernel archive %s has an unterminated long name for %q", path, name)
			}
			name = string(entry[:end])
		} else {
			name = strings.TrimSuffix(name, "/")
		}
		if name == "" {
			t.Fatalf("kernel archive %s has an empty member name", path)
		}
		object, err := elf.NewFile(bytes.NewReader(body))
		if err != nil {
			t.Fatalf("read kernel archive %s member %s as ELF: %v", path, name, err)
		}
		symbols, err := object.Symbols()
		object.Close()
		if err != nil {
			t.Fatalf("read kernel archive %s member %s symbols: %v", path, name, err)
		}
		if members[name] == nil {
			members[name] = make(map[string]bool)
		}
		for _, symbol := range symbols {
			binding := elf.ST_BIND(symbol.Info)
			if (binding == elf.STB_GLOBAL || binding == elf.STB_WEAK) && symbol.Section != elf.SHN_UNDEF {
				members[name][symbol.Name] = true
			}
		}
	}
	return members
}

func kernelArchiveSymbols(t *testing.T) map[string]bool {
	t.Helper()
	symbols := make(map[string]bool)
	for _, member := range kernelArchiveMembers(t) {
		for symbol := range member {
			symbols[symbol] = true
		}
	}
	return symbols
}

func TestKernelArchiveSymbolsReadsLibrary(t *testing.T) {
	members := kernelArchiveMembers(t)
	symbols := kernelArchiveSymbols(t)
	for _, name := range []string{"lxp_kernel_create", "lxp_log_append"} {
		if !symbols[name] {
			t.Errorf("kernel archive does not define %s", name)
		}
	}
	if !members["lxp_log.o"]["lxp_log_append"] {
		t.Error("kernel archive member lxp_log.o does not define lxp_log_append")
	}
	for symbol := range symbols {
		if strings.HasPrefix(symbol, "lxp_projection_") || symbol == "lxp_log_replay_range" {
			t.Errorf("kernel archive defines excluded src/storage/lxp_projection.c symbol %s", symbol)
		}
	}
}
