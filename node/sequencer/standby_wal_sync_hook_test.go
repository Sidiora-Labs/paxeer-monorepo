package sequencer_test

import (
	"context"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/stretchr/testify/require"
)

func TestReplicatorReportsWALSyncDuration(t *testing.T) {
	for _, hooked := range []bool{true, false} {
		name := "nil_hook"
		if hooked {
			name = "with_hook"
		}
		t.Run(name, func(t *testing.T) {
			sender := utils.NewSigner()
			recipient := utils.NewSigner().EvmAddress
			l := listen(t)
			p := newPrimary(t, l.Addr().String(), replicationTimeout, sender)
			sb := newStandby(t, p, sender)
			sb.serve(t, l)

			var durations []time.Duration
			var onWALSync func(time.Duration)
			if hooked {
				onWALSync = func(d time.Duration) {
					durations = append(durations, d)
				}
			}
			p.repl.Close()
			repl, err := sequencer.NewReplicator(sequencer.ReplicatorConfig{
				WAL: p.wal,
				Dial: func(ctx context.Context) (net.Conn, error) {
					if hooked {
						require.Len(t, durations, 1)
					}
					var d net.Dialer
					return d.DialContext(ctx, "tcp", l.Addr().String())
				},
				ReplicationTimeout: replicationTimeout,
				OnWALSync:          onWALSync,
			})
			require.NoError(t, err)
			t.Cleanup(repl.Close)

			_, tx := transfer(t, sender, 0, recipient, big.NewInt(onePaxInWei))
			open := p.open()
			accept := p.accept(tx)
			started := time.Now()
			ack, err := repl.Release(context.Background(), open, accept)
			elapsed := time.Since(started)
			require.NoError(t, err)
			require.Equal(t, sequencer.Ack{Status: sequencer.AckOK, LastSeq: accept.Seq}, ack)
			require.Equal(t, accept.Seq, sb.s.LastSeq())
			require.NoError(t, sb.s.Err())
			requireStandbyState(t, p, sb, sender.EvmAddress, recipient)
			if hooked {
				require.Len(t, durations, 1)
				require.GreaterOrEqual(t, durations[0], time.Duration(0))
				require.LessOrEqual(t, durations[0], elapsed)
			} else {
				require.Empty(t, durations)
			}

			ack, err = repl.Release(context.Background(), &wal.Lease{Epoch: 1, ExpiryMs: uint64(p.openMs)})
			require.NoError(t, err)
			require.Equal(t, sequencer.Ack{Status: sequencer.AckOK, LastSeq: accept.Seq}, ack)
			if hooked {
				require.Len(t, durations, 2)
				require.GreaterOrEqual(t, durations[1], time.Duration(0))
			} else {
				require.Empty(t, durations)
			}
		})
	}
}
