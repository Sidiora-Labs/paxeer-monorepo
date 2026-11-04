package privval

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	tmnet "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/net"
)

// IsConnTimeout returns a boolean indicating whether the error is known to
// report that a connection timeout occurred. This detects both fundamental
// network timeouts, as well as ErrConnTimeout errors.
func IsConnTimeout(err error) bool {
	var timeout timeoutError
	return errors.As(err, &timeout) && timeout.Timeout()
}

// NewSignerListener creates a new SignerListenerEndpoint using the corresponding listen address
func NewSignerListener(listenAddr string, identities ...SocketIdentity) (*SignerListenerEndpoint, error) {
	protocol, address := tmnet.ProtocolAndAddress(listenAddr)
	if protocol != "unix" && protocol != "tcp" { //nolint:goconst
		return nil, fmt.Errorf("unsupported address family %q, want unix or tcp", protocol)
	}

	var key ed25519.SecretKey
	if protocol == "tcp" {
		if len(identities) != 1 || identities[0].PeerKey == (ed25519.PublicKey{}) {
			return nil, ErrPeerNotAuthorized
		}
		var err error
		key, err = LoadOrGenSocketIdentity(identities[0].KeyFile)
		if err != nil {
			return nil, err
		}
	}

	ln, err := net.Listen(protocol, address)
	if err != nil {
		return nil, err
	}

	var listener net.Listener
	switch protocol {
	case "unix":
		listener = NewUnixListener(ln)
	case "tcp":
		listener = NewTCPListener(ln, key, identities[0].PeerKey)
	default:
		panic("invalid protocol: " + protocol) // semantically unreachable
	}

	return NewSignerListenerEndpoint(listener), nil
}

// SocketIdentity configures a stable local TCP identity and the authorized remote peer.
// KeyFile must live in a directory controlled by the operator.
type SocketIdentity struct {
	KeyFile string
	PeerKey ed25519.PublicKey
}

// LoadOrGenSocketIdentity loads a private identity, or atomically installs a new
// owner-only identity without replacing an existing key. Corrupt or inaccessible
// files are errors, never a reason to rotate the identity silently.
func LoadOrGenSocketIdentity(path string) (ed25519.SecretKey, error) {
	if path == "" {
		return ed25519.SecretKey{}, fmt.Errorf("socket identity path is required")
	}
	load := func() (ed25519.SecretKey, error) {
		info, err := os.Lstat(path)
		if err != nil {
			return ed25519.SecretKey{}, err
		}
		if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
			return ed25519.SecretKey{}, fmt.Errorf("socket identity must be an owner-only regular file")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return ed25519.SecretKey{}, err
		}
		defer clear(data)
		return ed25519.SecretKeyFromSecretBytes(data)
	}
	key, err := load()
	if !errors.Is(err, os.ErrNotExist) {
		return key, err
	}
	key = ed25519.GenerateSecretKey()
	data := key.SecretBytes()
	defer clear(data)
	file, err := os.CreateTemp(filepath.Dir(path), ".signer-identity-*")
	if err != nil {
		return ed25519.SecretKey{}, err
	}
	defer os.Remove(file.Name())
	_, err = file.Write(data)
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return ed25519.SecretKey{}, err
	}
	if closeErr != nil {
		return ed25519.SecretKey{}, closeErr
	}
	// Linking publishes only the complete key, and cannot overwrite a key
	// concurrently installed by another process.
	if err := os.Link(file.Name(), path); err != nil {
		if errors.Is(err, os.ErrExist) {
			return load()
		}
		return ed25519.SecretKey{}, err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return ed25519.SecretKey{}, err
	}
	err = dir.Sync()
	closeErr = dir.Close()
	if err != nil {
		return ed25519.SecretKey{}, err
	}
	if closeErr != nil {
		return ed25519.SecretKey{}, closeErr
	}
	return key, nil
}
