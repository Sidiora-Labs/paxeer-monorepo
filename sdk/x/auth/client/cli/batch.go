package cli

import (
	"bufio"
	"io"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	authclient "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/client"
)

// maxBatchTxLineBytes bounds each JSON transaction in both batch commands at
// 16 MiB, including its newline. Unlike Scanner's 64 KiB default, this permits
// large transaction bodies while keeping input buffering bounded.
const maxBatchTxLineBytes = 16 * 1024 * 1024

func newBatchTxScanner(cfg client.TxConfig, r io.Reader) *authclient.BatchScanner {
	scanner := authclient.NewBatchScanner(cfg, r)
	scanner.Buffer(make([]byte, bufio.MaxScanTokenSize), maxBatchTxLineBytes)
	return scanner
}
