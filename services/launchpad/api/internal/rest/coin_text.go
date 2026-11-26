package rest

func coinText(name, symbol string) (string, string) {
	return truncateRunes(name, 100), truncateRunes(symbol, 16)
}
