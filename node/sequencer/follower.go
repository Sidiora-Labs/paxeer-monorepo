package sequencer

import (
	"fmt"
	"net/url"

	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
)

// NewFollowerReceiptOption selects HTTP receipt forwarding for a configured
// follower. The RPC consumer verifies the forwarded JSON against its receipt.
func NewFollowerReceiptOption(cfg Config) (evmrpc.SequencerReceiptOption, bool) {
	if cfg.Enable || cfg.ReceiptForwardURL == "" {
		return evmrpc.SequencerReceiptOption{}, false
	}
	forwardURL, err := url.Parse(cfg.ReceiptForwardURL)
	if err != nil {
		panic(fmt.Errorf("%w: %s: %v", ErrConfigValue, flagReceiptForwardURL, err))
	}
	if (forwardURL.Scheme != "http" && forwardURL.Scheme != "https") || forwardURL.Host == "" {
		panic(fmt.Errorf("%w: %s must be an http or https URL with a host", ErrConfigValue, flagReceiptForwardURL))
	}
	return evmrpc.SequencerReceiptOption{ForwardURL: forwardURL}, true
}
