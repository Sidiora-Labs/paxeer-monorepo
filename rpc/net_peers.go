package evmrpc

import (
	"context"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/time"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

func (i *NetAPI) PeerCount(ctx context.Context) (count hexutil.Uint, err error) {
	startTime := time.Now()
	defer func() {
		recordMetricsWithError(ctx, "net_peerCount", i.connectionType, startTime, err, recover())
	}()
	info, err := i.tmClient.NetInfo(ctx)
	if err != nil {
		return 0, err
	}
	if info.NPeers < 0 {
		return 0, fmt.Errorf("consensus NetInfo returned negative peer count: %d", info.NPeers)
	}
	return hexutil.Uint(info.NPeers), nil
}
