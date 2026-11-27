package quoter

import (
	"os"
	"testing"
)

func TestLocalChainQuotesThroughAFollowerNode(t *testing.T) {
	endpoint, key := os.Getenv("LOCAL_NODE_FOLLOWER_RPC"), os.Getenv("LOCAL_NODE_KEY")
	if endpoint == "" || key == "" {
		t.Skip("LOCAL_NODE_FOLLOWER_RPC and LOCAL_NODE_KEY select a funded local follower")
	}
	runLocalChainQuotes(t, endpoint, key)
}
