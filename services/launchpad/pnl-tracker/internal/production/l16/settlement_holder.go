package l16

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

const nativeExecutionAddress = "0x0000000000000000000000000000000000001017"

var launchpadModuleAddress = func() string {
	digest := sha256.Sum256([]byte("launchpad"))
	return "0x" + hex.EncodeToString(digest[:20])
}()

func settlementAssetHolder(fill accounting.FillV1) string {
	if fill.ActualEmitter == nativeExecutionAddress {
		return launchpadModuleAddress
	}
	return fill.PoolAddress
}
