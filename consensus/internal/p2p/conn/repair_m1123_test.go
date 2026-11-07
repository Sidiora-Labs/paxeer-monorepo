package conn

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/scope"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/tcp"
)

func repairM1123RunTransport(ctx context.Context, conn tcp.Conn) error {
	err := conn.Run(ctx)
	// Canceling pending TCP reads closes the sockets. Treat a disconnect as
	// cancellation only after the test scope has already been canceled.
	if ctx.Err() != nil && IsDisconnect(err) {
		return utils.IgnoreCancel(ctx.Err())
	}
	return utils.IgnoreCancel(err)
}

func TestRepairM_1123_MaxPayloadRoundTrip(t *testing.T) {
	for _, channelID := range []ChannelID{127, 128, 255, 16383, 16384, math.MaxUint16} {
		for _, payloadSize := range []int{120, DefaultMConnConfig().MaxPacketMsgPayloadSize, 16384} {
			t.Run(fmt.Sprintf("channel_%d/payload_%d", channelID, payloadSize), func(t *testing.T) {
				rng := utils.TestRng()
				want := utils.GenBytes(rng, payloadSize)
				ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
				defer cancel()
				clientConn, serverConn := tcp.TestPipe()
				defer clientConn.Close()
				defer serverConn.Close()
				cfg := DefaultMConnConfig()
				cfg.MaxPacketMsgPayloadSize = payloadSize
				cfg.SendRate = 0
				cfg.RecvRate = 0
				descriptors := []*ChannelDescriptor{{ID: channelID, RecvMessageCapacity: payloadSize}}
				client := NewMConnection(clientConn, descriptors, cfg)
				server := NewMConnection(serverConn, descriptors, cfg)
				err := scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
					s.SpawnBgNamed("client transport", func() error { return repairM1123RunTransport(ctx, clientConn) })
					s.SpawnBgNamed("server transport", func() error { return repairM1123RunTransport(ctx, serverConn) })
					s.SpawnBgNamed("client connection", func() error { return utils.IgnoreCancel(client.Run(ctx)) })
					s.SpawnBgNamed("server connection", func() error { return utils.IgnoreCancel(server.Run(ctx)) })
					if err := client.Send(ctx, channelID, bytes.Clone(want)); err != nil {
						return fmt.Errorf("client.Send(): %w", err)
					}
					gotChannel, got, err := server.Recv(ctx)
					if err != nil {
						return fmt.Errorf("server.Recv(): %w", err)
					}
					if gotChannel != channelID || !bytes.Equal(got, want) {
						return fmt.Errorf("server received channel %d and %d bytes, want channel %d and matching %d-byte payload", gotChannel, len(got), channelID, len(want))
					}
					if err := server.Send(ctx, gotChannel, got); err != nil {
						return fmt.Errorf("server.Send(): %w", err)
					}
					gotChannel, got, err = client.Recv(ctx)
					if err != nil {
						return fmt.Errorf("client.Recv(): %w", err)
					}
					if gotChannel != channelID || !bytes.Equal(got, want) {
						return fmt.Errorf("client received channel %d and %d bytes, want channel %d and matching %d-byte payload", gotChannel, len(got), channelID, len(want))
					}
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestRepairM_1123_OversizedPayloadRejected(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	clientConn, serverConn := tcp.TestPipe()
	defer clientConn.Close()
	defer serverConn.Close()
	cfg := DefaultMConnConfig()
	cfg.SendRate = 0
	cfg.RecvRate = 0
	descriptors := []*ChannelDescriptor{{ID: math.MaxUint16}}
	server := NewMConnection(serverConn, descriptors, cfg)
	cfg.MaxPacketMsgPayloadSize++
	client := NewMConnection(clientConn, descriptors, cfg)
	err := scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
		s.SpawnBgNamed("client transport", func() error { return repairM1123RunTransport(ctx, clientConn) })
		s.SpawnBgNamed("server transport", func() error { return repairM1123RunTransport(ctx, serverConn) })
		s.SpawnBgNamed("client connection", func() error { return utils.IgnoreCancel(client.Run(ctx)) })
		s.SpawnNamed("bounded receiver", func() error {
			if err := server.Run(ctx); !errors.Is(err, errMsgTooLarge) {
				return fmt.Errorf("server.Run() = %v, want errMsgTooLarge", err)
			}
			return nil
		})
		return client.Send(ctx, math.MaxUint16, make([]byte, cfg.MaxPacketMsgPayloadSize))
	})
	if err != nil {
		t.Fatal(err)
	}
}
