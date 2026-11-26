package metadata

import (
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgtype"
)

func chainCoinText(value string) string {
	if len(value) > 64<<10 || !utf8.ValidString(value) || strings.ContainsRune(value, 0) {
		return ""
	}
	return value
}

func carriedCoinText(value string, limit int) string {
	value = chainCoinText(value)
	count := 0
	for offset := range value {
		if count == limit {
			return value[:offset]
		}
		count++
	}
	return value
}

func coinTextPtr(value pgtype.Text, limit int) *string {
	if !value.Valid {
		return nil
	}
	text := carriedCoinText(value.String, limit)
	return &text
}
