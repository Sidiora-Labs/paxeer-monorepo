package tasks

import (
	"fmt"
	"sync"

	sdk "github.com/sei-protocol/sei-chain/sei-cosmos/types"
)

// These are the deployed contract in node/params/config.go. The nested paxctl
// module cannot import the node's distinct SDK types.
const (
	feeDenom              = "uhpx"
	paxeerAccountPrefix   = "pax"
	paxeerValidatorPrefix = "paxvaloper"
	paxeerConsensusPrefix = "paxvalcons"
)

var paxeerAddressOnce sync.Once

func init() { ensureBech32() }

func ensureBech32() {
	paxeerAddressOnce.Do(func() {
		cfg := sdk.GetConfig()
		cfg.SetBech32PrefixForAccount(paxeerAccountPrefix, "paxpub")
		cfg.SetBech32PrefixForValidator(paxeerValidatorPrefix, "paxvaloperpub")
		cfg.SetBech32PrefixForConsensusNode(paxeerConsensusPrefix, "paxvalconspub")
		cfg.SetAddressVerifier(func(b []byte) error {
			if len(b) != 20 && len(b) != 32 {
				return fmt.Errorf("address length must be 20 or 32 bytes, got %d", len(b))
			}
			return nil
		})
	})
}
