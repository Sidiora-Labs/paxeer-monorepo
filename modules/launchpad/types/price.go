package types

import "fmt"

func (m Market) PriceHistory(fromHeight, toHeight uint64) ([]PricePoint, error) {
	if len(m.PriceSnapshots) != SnapshotSlots || m.SnapshotIndex >= SnapshotSlots ||
		m.SnapshotCount > SnapshotSlots ||
		(len(m.SnapshotHeights) != 0 && len(m.SnapshotHeights) != SnapshotSlots) {
		return nil, fmt.Errorf("%w: snapshot ring", ErrInvalidMarket)
	}
	if fromHeight > toHeight {
		return nil, ErrInvalidRange
	}

	var start uint64
	if m.SnapshotCount == SnapshotSlots {
		start = m.SnapshotIndex
	}
	points := make([]PricePoint, 0, m.SnapshotCount)
	for i := uint64(0); i < m.SnapshotCount; i++ {
		slot := (start + i) % SnapshotSlots
		var height uint64
		if len(m.SnapshotHeights) != 0 {
			height = m.SnapshotHeights[slot]
		}
		if height >= fromHeight && height <= toHeight {
			points = append(points, PricePoint{Height: height, Price: m.PriceSnapshots[slot]})
		}
	}
	return points, nil
}
