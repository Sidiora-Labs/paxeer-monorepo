package httpapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/store"
)

const legacyPageMaximum = 500
const legacyOffsetMaximum = 10000
const creatorPageMaximum = 100
const whalePageMaximum = 100

type historyPage struct {
	Limit  int
	Offset int
	After  *store.HistoryPosition
	Head   *store.HistoryPosition
}
type historyCursor struct {
	Version int                    `json:"v"`
	Scope   string                 `json:"scope"`
	After   store.HistoryPosition  `json:"after"`
	Head    *store.HistoryPosition `json:"head,omitempty"`
}

func boundedPage(q url.Values, def, maximum, minimum int, scope string) (p historyPage, err error) {
	p.Limit = def
	for _, field := range []string{"limit", "offset", "cursor"} {
		if values, ok := q[field]; ok && (len(values) != 1 || values[0] == "") {
			return p, errors.New("invalid pagination")
		}
	}
	if value, ok := q["limit"]; ok {
		p.Limit, err = strconv.Atoi(value[0])
		if err != nil {
			return p, err
		}
	}
	if value, ok := q["offset"]; ok {
		p.Offset, err = strconv.Atoi(value[0])
		if err != nil {
			return p, err
		}
	}
	if p.Limit < minimum || p.Limit > maximum || p.Offset < 0 || p.Offset > legacyOffsetMaximum {
		return p, errors.New("pagination outside bounds; use a cursor for deep history")
	}
	if raw, ok := q["cursor"]; ok {
		if _, offset := q["offset"]; offset || len(raw[0]) > 2048 {
			return p, errors.New("invalid cursor pagination")
		}
		body, e := base64.RawURLEncoding.Strict().DecodeString(raw[0])
		if e != nil {
			return p, e
		}
		var c historyCursor
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&c) != nil || decoder.Decode(new(any)) != io.EOF || c.Version != 1 || c.Scope != scope || !validHistoryPosition(c.After) || (c.Head != nil && (!validHistoryPosition(*c.Head) || positionBefore(*c.Head, c.After))) {
			return p, errors.New("invalid history cursor")
		}
		p.After, p.Head = &c.After, c.Head
	}
	return p, nil
}
func validHistoryPosition(p store.HistoryPosition) bool {
	return p.Timestamp >= 0 && len(p.ID) > 0 && len(p.ID) <= 256 && !strings.ContainsAny(p.ID, "\x00\r\n")
}
func positionBefore(a, b store.HistoryPosition) bool {
	return a.Timestamp < b.Timestamp || (a.Timestamp == b.Timestamp && a.ID < b.ID)
}
func nextHistoryCursor(scope string, after store.HistoryPosition, head *store.HistoryPosition) string {
	raw, _ := json.Marshal(historyCursor{Version: 1, Scope: scope, After: after, Head: head})
	return base64.RawURLEncoding.EncodeToString(raw)
}
