package bindings

import (
	"encoding/json"
	"fmt"
)

type SetDenomPaused struct {
	Denom  string `json:"denom"`
	Paused bool   `json:"paused"`
}

// UnmarshalJSON requires an explicit pause decision: a missing or null boolean
// must never turn malformed contract output into an unpause operation.
func (m *SetDenomPaused) UnmarshalJSON(data []byte) error {
	var wire struct {
		Denom  *string `json:"denom"`
		Paused *bool   `json:"paused"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if wire.Denom == nil || *wire.Denom == "" || wire.Paused == nil {
		return fmt.Errorf("set_denom_paused requires denom and paused")
	}
	*m = SetDenomPaused{Denom: *wire.Denom, Paused: *wire.Paused}
	return nil
}
