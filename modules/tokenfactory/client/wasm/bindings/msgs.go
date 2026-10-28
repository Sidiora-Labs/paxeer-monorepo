package bindings

type SetDenomPaused struct {
	Denom  string `json:"denom"`
	Paused bool   `json:"paused"`
}
