package app

import (
	"errors"
	"fmt"
)

var ErrSequencedBlockHalt = errors.New("sequenced block halt")

type sequencedHaltError struct {
	error
	deliveryErr error
}

func (e sequencedHaltError) Unwrap() error {
	return e.error
}

func (e sequencedHaltError) Cause() error {
	return e.deliveryErr
}

func sequencedHalt(height int64, err error) error {
	return sequencedHaltError{
		error:       fmt.Errorf("%w at height %d: %w", ErrSequencedBlockHalt, height, err),
		deliveryErr: err,
	}
}
