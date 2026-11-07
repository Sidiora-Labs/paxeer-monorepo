//go:build gofuzz

package tests

import (
	"bytes"
	"context"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/p2p/conn"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/scope"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/tcp"
)

func FuzzP2PSecretConnection(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte("secret connection"))
	f.Add(bytes.Repeat([]byte{0xff}, 1024))
	f.Add(bytes.Repeat([]byte{0x01}, 1025))
	f.Add(bytes.Repeat([]byte{0x00, 0xff}, 2048))
	f.Fuzz(fuzz)
}

func fuzz(t *testing.T, data []byte) {
	if len(data) == 0 {
		return
	}

	dataRead := make([]byte, len(data))
	c1, c2 := tcp.TestPipe()
	defer c1.Close()
	defer c2.Close()
	err := scope.Run(t.Context(), func(ctx context.Context, s scope.Scope) error {
		for _, c := range []tcp.Conn{c1, c2} {
			s.SpawnBg(func() error {
				err := c.Run(ctx)
				if ctx.Err() != nil {
					return nil
				}
				return err
			})
		}
		var fooConn, barConn *conn.SecretConnection
		// Make connections from both sides in parallel.
		if err := scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
			s.Spawn(func() error {
				var err error
				fooConn, err = conn.MakeSecretConnection(ctx, c1)
				return err
			})
			var err error
			barConn, err = conn.MakeSecretConnection(ctx, c2)
			return err
		}); err != nil {
			return err
		}
		// Run Write in a separate goroutine because if data is greater than 1024
		// bytes, each Write must be followed by Read (see io.Pipe documentation).
		return scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
			s.Spawn(func() error {
				// Copy data because Write modifies the slice.
				dataToWrite := bytes.Clone(data)
				if err := fooConn.Write(ctx, dataToWrite); err != nil {
					return err
				}
				return fooConn.Flush(ctx)
			})
			return barConn.Read(ctx, dataRead)
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, dataRead) {
		t.Fatal("bytes written != read")
	}
}
