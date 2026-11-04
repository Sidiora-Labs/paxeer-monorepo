package layerx

import (
	"encoding/json"
	"runtime"
	"sync"
)

type SecretBytes struct {
	mu        sync.Mutex
	value     []byte
	destroyed bool
}

func NewSecretBytes(value []byte) (*SecretBytes, error) {
	if len(value) == 0 {
		return nil, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	secret := &SecretBytes{value: append([]byte(nil), value...)}
	runtime.SetFinalizer(secret, (*SecretBytes).Destroy)
	return secret, nil
}

// Expose lends an isolated copy for the duration of consumer. The callback may
// re-enter Expose or call Destroy. Destroy wipes stored material and prevents new
// borrows immediately; active borrows remain valid until their callbacks return.
// Mutations affect only the borrow, which is wiped even if the callback panics.
// Consumers must not retain the slice or access it after returning.
func (secret *SecretBytes) Expose(consumer func([]byte) error) error {
	if secret == nil || consumer == nil {
		return newSDKError(ErrorInvalidArgument, RetryNever)
	}
	secret.mu.Lock()
	if secret.destroyed {
		secret.mu.Unlock()
		return newSDKError(ErrorInvalidArgument, RetryNever)
	}
	borrow := append([]byte(nil), secret.value...)
	secret.mu.Unlock()
	defer func() {
		for index := range borrow {
			borrow[index] = 0
		}
	}()
	return consumer(borrow)
}

func (secret *SecretBytes) Destroy() {
	if secret == nil {
		return
	}
	secret.mu.Lock()
	defer secret.mu.Unlock()
	for index := range secret.value {
		secret.value[index] = 0
	}
	secret.value = nil
	secret.destroyed = true
	runtime.SetFinalizer(secret, nil)
}

func (*SecretBytes) String() string   { return "[REDACTED]" }
func (*SecretBytes) GoString() string { return "SecretBytes([REDACTED])" }

func (*SecretBytes) MarshalJSON() ([]byte, error) {
	return json.Marshal("[REDACTED]")
}
