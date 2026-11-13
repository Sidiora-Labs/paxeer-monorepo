package processor

import (
	"fmt"
	"math"
)

func chainLaunchNumber(args map[string]any) (int64, error) {
	n, err := asInt64(args["chain"])
	if err != nil || n < 0 || n > math.MaxUint32 {
		return 0, fmt.Errorf("invalid chain registry number")
	}
	return n, nil
}
