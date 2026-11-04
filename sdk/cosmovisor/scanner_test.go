package cosmovisor_test

import (
	"bufio"
	"fmt"
	"io"
	"net/url"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/cosmovisor"

	"github.com/stretchr/testify/require"
)

func TestWaitForInfo(t *testing.T) {
	cases := map[string]struct {
		write         []string
		expectUpgrade *cosmovisor.UpgradeInfo
		expectErr     bool
	}{
		"no match": {
			write: []string{"some", "random\ninfo\n"},
		},
		"match name with no info": {
			write: []string{"first line\n", `UPGRADE "myname" NEEDED at height: 123: `, "\nnext line\n"},
			expectUpgrade: &cosmovisor.UpgradeInfo{
				Name: "myname",
				Info: "",
			},
		},
		"match name with info": {
			write: []string{"first line\n", `UPGRADE "take2" NEEDED at height: 123:   DownloadData here!`, "\nnext line\n"},
			expectUpgrade: &cosmovisor.UpgradeInfo{
				Name: "take2",
				Info: "DownloadData here!",
			},
		},
	}

	spacedJSON := fmt.Sprintf(`{"binaries": {"%s": "https://example.com/paxd?checksum=sha256:abcd"}}`, cosmovisor.OSArch())
	cases["x/upgrade spaced JSON with logger suffix"] = struct {
		write         []string
		expectUpgrade *cosmovisor.UpgradeInfo
		expectErr     bool
	}{
		write:         []string{`ERR UPGRADE "v2" NEEDED at height: 123: ` + spacedJSON + " module=x/upgrade\n"},
		expectUpgrade: &cosmovisor.UpgradeInfo{Name: "v2", Info: spacedJSON},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r, w := io.Pipe()
			scan := bufio.NewScanner(r)

			// write all info in separate routine
			go func() {
				for _, line := range tc.write {
					n, err := w.Write([]byte(line))
					require.NoError(t, err)
					require.Equal(t, len(line), n)
				}
				w.Close()
			}()

			// now scan the info
			info, err := cosmovisor.WaitForUpdate(scan)
			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expectUpgrade, info)
			if name == "x/upgrade spaced JSON with logger suffix" {
				download, err := cosmovisor.GetDownloadURL(info)
				require.NoError(t, err)
				parsed, err := url.ParseRequestURI(download)
				require.NoError(t, err)
				require.Equal(t, "https", parsed.Scheme)
				require.Equal(t, "example.com", parsed.Host)
				require.Equal(t, "sha256:abcd", parsed.Query().Get("checksum"))
			}
		})
	}
}
