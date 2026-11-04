package privval

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	tmnet "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/net"
)

// Socket errors.
var (
	ErrDialRetryMax      = errors.New("dialed maximum retries")
	ErrPeerNotAuthorized = errors.New("socket peer key is missing or unauthorized")
)

// SocketDialer dials a remote address and returns a net.Conn or an error.
type SocketDialer func() (net.Conn, error)

// DialTCPFn dials the given tcp addr, using the given timeoutReadWrite and
// privKey for the authenticated encryption handshake. Exactly one authorized
// remote public key is required; an absent pin fails closed.
func DialTCPFn(addr string, timeoutReadWrite time.Duration, privKey crypto.PrivKey, peers ...crypto.PubKey) SocketDialer {
	// Copy configuration so callers cannot change the pin after construction.
	var peer crypto.PubKey
	valid := len(peers) == 1
	if valid {
		peer = peers[0]
		valid = peer != (crypto.PubKey{})
	}
	return func() (net.Conn, error) {
		if !valid {
			return nil, ErrPeerNotAuthorized
		}
		protocol, address := tmnet.ProtocolAndAddress(addr)
		if protocol != "tcp" {
			return nil, fmt.Errorf("unsupported TCP dial protocol %q", protocol)
		}
		conn, err := net.DialTimeout(protocol, address, timeoutReadWrite)
		if err != nil {
			return nil, err
		}
		if err = conn.SetDeadline(time.Now().Add(timeoutReadWrite)); err != nil {
			_ = conn.Close()
			return nil, err
		}
		secretConn, err := MakeSecretConnection(conn, privKey)
		if err != nil {
			_ = conn.Close()
			return nil, err
		}
		if secretConn.RemotePubKey() != peer {
			_ = secretConn.Close()
			return nil, ErrPeerNotAuthorized
		}
		return secretConn, nil
	}
}

// DialUnixFn dials the given unix socket.
func DialUnixFn(addr string) SocketDialer {
	return func() (net.Conn, error) {
		unixAddr := &net.UnixAddr{Name: addr, Net: "unix"}
		return net.DialUnix("unix", nil, unixAddr)
	}
}
