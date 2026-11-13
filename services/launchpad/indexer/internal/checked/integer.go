package checked

import "fmt"

// Integer is the set of integer representations used by the indexer.
type Integer interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

// Convert preserves an integer's value or returns a range error.
func Convert[To, From Integer](value From) (To, error) {
	converted := To(value)
	if (value < 0) != (converted < 0) || From(converted) != value {
		return 0, fmt.Errorf("integer %d is outside the range of %T", value, converted)
	}
	return converted, nil
}
