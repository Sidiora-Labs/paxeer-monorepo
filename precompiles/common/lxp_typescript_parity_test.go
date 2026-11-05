package common

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const LXP_RETRYABLE = "LXP_RETRYABLE"

func TestLXPRetryableMatchesTypeScriptSDK(t *testing.T) {
	source := lxpReadRepoFileForTest(t, "services/agent/sdk/typescript/src/revert.ts")
	declaration := regexp.MustCompile(`(?s)\bexport\s+const\s+` + LXP_RETRYABLE + `\s*:\s*ReadonlySet\s*<\s*string\s*>\s*=\s*new\s+Set\s*\(\s*\[([^\]]*)\]\s*\)\s*;`)
	sets := declaration.FindAllStringSubmatch(source, -1)
	require.Len(t, sets, 1, "expected exactly one literal TypeScript %s declaration", LXP_RETRYABLE)

	comments := regexp.MustCompile(`(?m)//[^\n]*|(?s:/\*.*?\*/)`)
	body := comments.ReplaceAllString(sets[0][1], "")
	literal := regexp.MustCompile(`^(?:"(LXP_[A-Z0-9]+(?:_[A-Z0-9]+)*)"|'(LXP_[A-Z0-9]+(?:_[A-Z0-9]+)*)')$`)
	sdkRetryable := map[string]bool{}
	entries := strings.Split(body, ",")
	for i, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" && i == len(entries)-1 {
			continue
		}
		match := literal.FindStringSubmatch(entry)
		require.NotNil(t, match, "unsupported %s member %q", LXP_RETRYABLE, entry)
		code := match[1] + match[2]
		require.False(t, sdkRetryable[code], "duplicate TypeScript retryable code %s", code)
		sdkRetryable[code] = true
	}
	require.NotEmpty(t, sdkRetryable, "no TypeScript retryable codes parsed")

	doc := lxpReadRepoFileForTest(t, "spec/layerx-merge/contracts/address-table.md")
	header := "| Code | Message | Emitting construct | Kernel or Go origin (file:line) | Retryable |"
	start := strings.Index(doc, header)
	end := strings.Index(doc, "## (c) Activation")
	require.True(t, start >= 0 && end > start, "section (b) error code registry not found")
	registryCodes := map[string]bool{}
	registryRetryable := map[string]bool{}
	codePattern := regexp.MustCompile(`^LXP_[A-Z0-9]+(?:_[A-Z0-9]+)*$`)
	for _, line := range strings.Split(doc[start:end], "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "| LXP_") {
			continue
		}
		cells := strings.Split(line, " | ")
		require.Len(t, cells, 5, "malformed registry row: %s", line)
		code := strings.TrimPrefix(cells[0], "| ")
		require.Regexp(t, codePattern, code, "invalid registry code")
		require.False(t, registryCodes[code], "duplicate registry code %s", code)
		registryCodes[code] = true
		retryable := strings.TrimSuffix(cells[4], " |")
		switch retryable {
		case "yes":
			registryRetryable[code] = true
		case "no":
		default:
			t.Fatalf("invalid Retryable value %q for %s", retryable, code)
		}
	}
	require.NotEmpty(t, registryRetryable, "no retryable registry rows parsed")

	registeredCodes := map[string]bool{}
	for _, entry := range lxpRevertTable {
		registeredCodes[entry.code] = true
	}
	for code := range sdkRetryable {
		assert.True(t, registeredCodes[code], "TypeScript retryable code %s is absent from lxpRevertTable", code)
	}
	assert.Equal(t, registryRetryable, sdkRetryable, "TypeScript %s must equal the registry codes whose Retryable column is yes", LXP_RETRYABLE)
}
