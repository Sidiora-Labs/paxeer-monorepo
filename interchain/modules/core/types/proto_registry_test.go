package types_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/27-interchain-accounts/controller/types"
	_ "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/27-interchain-accounts/host/types"
	_ "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/27-interchain-accounts/types"
	_ "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/types"
	_ "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/legacy/v100"
	_ "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/types"
	_ "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/03-connection/types"
	_ "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/04-channel/types"
	_ "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/05-port/types"
	_ "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/23-commitment/types"
	_ "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/types"
	_ "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/light-clients/06-solomachine/types"
	_ "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/light-clients/07-tendermint/types"
	_ "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/light-clients/09-localhost/types"
)

func TestInterchainProtoFilesRegistered(t *testing.T) {
	protoRoot := os.DirFS(filepath.Join("..", "..", "..", "proto"))
	files := 0
	err := fs.WalkDir(protoRoot, "ibc", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".proto") {
			return nil
		}

		files++
		if len(proto.FileDescriptor(path)) == 0 {
			t.Errorf("proto file %s has no registered generated Go descriptor", path)
		}
		return nil
	})
	require.NoError(t, err)
	require.Positive(t, files, "no interchain proto files found")
}
