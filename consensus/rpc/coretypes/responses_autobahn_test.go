package coretypes_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
)

func TestAutobahnRPCSentinelsDistinct(t *testing.T) {
	sentinels := []struct {
		name    string
		err     error
		message string
	}{
		{
			name:    "ErrCommitNotAvailable",
			err:     coretypes.ErrCommitNotAvailable,
			message: "commit is not available under autobahn consensus",
		},
		{
			name:    "ErrConsensusStateNotAvailable",
			err:     coretypes.ErrConsensusStateNotAvailable,
			message: "consensus round state is not available under autobahn consensus",
		},
		{
			name:    "ErrEvidenceNotAccepted",
			err:     coretypes.ErrEvidenceNotAccepted,
			message: "evidence is not accepted under autobahn consensus",
		},
	}

	for i, sentinel := range sentinels {
		t.Run(sentinel.name, func(t *testing.T) {
			if sentinel.err == nil {
				t.Fatal("sentinel must not be nil")
			}
			if got := sentinel.err.Error(); got != sentinel.message {
				t.Errorf("message = %q, want %q", got, sentinel.message)
			}
			for _, missingBlock := range []error{
				coretypes.ErrHeightNotAvailable,
				coretypes.ErrHeightExceedsChainHead,
			} {
				if strings.Contains(sentinel.err.Error(), missingBlock.Error()) {
					t.Errorf("message %q contains light provider match %q", sentinel.err, missingBlock)
				}
				if errors.Is(sentinel.err, missingBlock) {
					t.Errorf("sentinel matches missing-block error %v", missingBlock)
				}
			}
			for j, other := range sentinels {
				if i != j && errors.Is(sentinel.err, other.err) {
					t.Errorf("sentinel matches %s", other.name)
				}
			}
		})
	}
}
