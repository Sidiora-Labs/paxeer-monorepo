package common

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLXPRetryableMatchesPythonSDK(t *testing.T) {
	source := lxpReadRepoFileForTest(t, "services/agent/sdk/python/layerx_sdk/revert.py")
	declaration := regexp.MustCompile(`(?m)^LXP_RETRYABLE[^\n=]*=\s*MappingProxyType\(\{`).FindStringIndex(source)
	require.NotNil(t, declaration, "Python SDK LXP_RETRYABLE map declaration not found")
	body := source[declaration[1]:]
	end := strings.Index(body, "\n})")
	require.NotEqual(t, -1, end, "Python SDK LXP_RETRYABLE map terminator not found")
	body = body[:end]

	entryPattern := regexp.MustCompile(`^"(LXP_[A-Z0-9_]+)":\s*(True|False),?\s*$`)
	pythonRetryable := map[string]bool{}
	pythonCodes := map[string]struct{}{}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
		if line == "" {
			continue
		}
		match := entryPattern.FindStringSubmatch(line)
		require.NotNil(t, match, "invalid Python SDK LXP_RETRYABLE entry: %s", line)
		code := match[1]
		_, duplicate := pythonRetryable[code]
		require.False(t, duplicate, "duplicate Python SDK LXP_RETRYABLE code %s", code)
		pythonRetryable[code] = match[2] == "True"
		pythonCodes[code] = struct{}{}
	}
	require.NotEmpty(t, pythonRetryable, "Python SDK LXP_RETRYABLE map is empty")

	goCodes := map[string]struct{}{}
	for _, entry := range lxpRevertTable {
		_, duplicate := goCodes[entry.code]
		require.False(t, duplicate, "duplicate lxpRevertTable code %s", entry.code)
		goCodes[entry.code] = struct{}{}
	}
	require.Equal(t, goCodes, pythonCodes, "Python SDK and lxpRevertTable must have identical code sets")

	doc := lxpReadRepoFileForTest(t, "spec/layerx-merge/contracts/address-table.md")
	start := strings.Index(doc, "| Code | Message | Emitting construct | Kernel or Go origin (file:line) | Retryable |")
	end = strings.Index(doc, "## (c) Activation")
	require.True(t, start >= 0 && end > start, "section (b) code table not found")
	registryRetryable := map[string]bool{}
	for _, line := range strings.Split(doc[start:end], "\n") {
		if !strings.HasPrefix(line, "| LXP_") {
			continue
		}
		cells := strings.Split(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(line), "| "), " |"), " | ")
		require.Len(t, cells, 5, line)
		code := cells[0]
		_, duplicate := registryRetryable[code]
		require.False(t, duplicate, "duplicate registry code %s", code)
		require.Contains(t, []string{"yes", "no"}, cells[4], "invalid registry Retryable value for %s", code)
		registryRetryable[code] = cells[4] == "yes"
	}
	require.NotEmpty(t, registryRetryable, "registry code table is empty")
	require.Equal(t, registryRetryable, pythonRetryable, "Python SDK must contain exactly the registry codes and their Retryable values")
}
