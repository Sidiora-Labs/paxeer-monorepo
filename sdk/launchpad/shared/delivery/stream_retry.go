package delivery

import (
	"fmt"
	"time"
)

// StreamRetryPassError reports a failed pass and preserves its underlying causes.
// RetriedOnly means every failure returned its frame to pending. Attempts is
// the largest attempt count of those frames; Quarantined reports any frame set aside.
type StreamRetryPassError struct {
	RetriedOnly bool
	Attempts    int
	Quarantined bool
	cause       error
}

func (e *StreamRetryPassError) Error() string { return e.cause.Error() }
func (e *StreamRetryPassError) Unwrap() error { return e.cause }

func streamRetryDelay(delay time.Duration, attempts int) time.Duration {
	ceiling := max(5*time.Second, delay)
	for attempts > 1 && delay < ceiling {
		if delay > ceiling/2 {
			return ceiling
		}
		delay *= 2
		attempts--
	}
	return delay
}

func streamRetryPublicationError(l *streamPublicationLease, message string, cause error) error {
	identity := fmt.Sprintf("%s (topic=%q partition=%d sequence=%d attempts=%d)", message, l.topic, l.partition, l.sequence, l.attempts)
	if cause == nil {
		return fmt.Errorf("%s", identity)
	}
	return fmt.Errorf("%s: %w", identity, cause)
}
