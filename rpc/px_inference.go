package evmrpc

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"math/big"
	"strconv"
	"strings"
	"time"

	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxpay"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

// F11 widths are frozen by queries.rs, rooms.rs and laps.rs in the ai-market producer.
const pxRoomWidth = 209 + 32*(64+128)
const pxDealWidth = 146 + 36*32 + 24
const pxPeerWidth = 32*4 + 8*5 + 2 + 1 + 13 + 1 + 4

type pxInferenceAPI struct{ account *pxAccountAPI }

// pxFixed refuses truncated, trailing and noncanonical producer records.
type pxFixed struct {
	data []byte
	bad  bool
}

func (r *pxFixed) take(n int) []byte {
	if n < 0 {
		r.bad = true
		return nil
	}
	if n > len(r.data) {
		r.bad = true
		return make([]byte, n)
	}
	value := r.data[:n]
	r.data = r.data[n:]
	return value
}
func (r *pxFixed) number(n int) uint64 {
	var v uint64
	for _, b := range r.take(n) {
		v = v<<8 | uint64(b)
	}
	return v
}
func (r *pxFixed) decimal(n int) string { return new(big.Int).SetBytes(r.take(n)).String() }
func (r *pxFixed) id() string {
	b := r.take(32)
	if bytes.Equal(b, make([]byte, 32)) {
		r.bad = true
	}
	return hex.EncodeToString(b)
}
func (r *pxFixed) end() error {
	if r.bad || len(r.data) != 0 {
		return pxUnavailable("invalid_inference_response")
	}
	return nil
}
func pxInferenceID(value string) ([32]byte, error) {
	var id [32]byte
	if !pxLowerHex(value, 32) || value == strings.Repeat("0", 64) {
		return id, pxInvalidParams()
	}
	b, _ := hex.DecodeString(value)
	copy(id[:], b)
	return id, nil
}
func pxOrders(r *pxFixed, n int) []string {
	ids := make([]string, 0)
	previous := ""
	ended := false
	for i := 0; i < n; i++ {
		b := r.take(32)
		if bytes.Equal(b, make([]byte, 32)) {
			ended = true
			continue
		}
		id := hex.EncodeToString(b)
		if ended || id <= previous {
			r.bad = true
		}
		ids = append(ids, id)
		previous = id
	}
	return ids
}

// DecodePxInferenceRoom decodes the fixed-width F11 RoomView answer.
func DecodePxInferenceRoom(answer []byte) (map[string]interface{}, error) {
	if len(answer) != pxRoomWidth {
		return nil, pxUnavailable("invalid_inference_response")
	}
	r := pxFixed{data: answer}
	v := map[string]interface{}{"room_id": r.id(), "host_key": r.id(), "model_digest": r.id(), "runtime_digest": r.id()}
	bond := r.decimal(16)
	canary := r.number(2)
	lap := r.number(8)
	window := r.number(8)
	slash := r.number(2)
	host := r.number(2)
	protocol := r.number(2)
	profile := map[string]interface{}{"minimum_bond": bond, "canary_ratio_bps": canary, "lap_size_tokens": strconv.FormatUint(lap, 10), "challenge_window_blocks": strconv.FormatUint(window, 10), "slash_fraction_bps": slash, "host_share_bps": host, "protocol_share_bps": protocol, "settlement_account": r.id()}
	if bond == "0" || lap == 0 || window == 0 || canary > 10000 || slash > 10000 || host+protocol > 10000 {
		r.bad = true
	}
	state := r.number(1)
	if state > 1 {
		r.bad = true
	}
	v["state"] = state
	v["opened_at_epoch"] = r.decimal(8)
	v["profile"] = profile
	asks := pxOrders(&r, 64)
	bids := pxOrders(&r, 128)
	v["open_asks"] = asks
	v["open_bids"] = bids
	if state == 1 && (len(asks) != 0 || len(bids) != 0) {
		r.bad = true
	}
	return v, r.end()
}

// DecodePxInferenceDeal decodes the fixed-width F11 DealView answer.
func DecodePxInferenceDeal(answer []byte) (map[string]interface{}, error) {
	if len(answer) != pxDealWidth {
		return nil, pxUnavailable("invalid_inference_response")
	}
	r := pxFixed{data: answer}
	v := map[string]interface{}{"room_id": r.id(), "deal_id": r.id(), "ask_id": r.id(), "agreed_price_per_thousand_tokens": r.decimal(16), "roster_digest": r.id()}
	count := r.number(1)
	state := r.number(1)
	if count == 0 || count > 32 || state > 1 || v["agreed_price_per_thousand_tokens"] == "0" {
		r.bad = true
	}
	v["state"] = state
	layers := make([]map[string]interface{}, 0)
	end := uint64(0)
	seen := map[string]bool{}
	for i := uint64(0); i < 32; i++ {
		if i >= count {
			if !bytes.Equal(r.take(36), make([]byte, 36)) {
				r.bad = true
			}
			continue
		}
		id := r.id()
		first := r.number(2)
		next := r.number(2)
		if first != end || next <= first || seen[id] {
			r.bad = true
		}
		seen[id] = true
		end = next
		layers = append(layers, map[string]interface{}{"bid_id": id, "first_layer": first, "end_layer": next})
	}
	v["layers"] = layers
	v["settled_tokens"] = r.decimal(8)
	v["settled_amount_owed"] = r.decimal(16)
	return v, r.end()
}

// DecodePxInferencePeer decodes the fixed-width F11 PeerView answer, including explicit availability tags.
func DecodePxInferencePeer(answer []byte) (map[string]interface{}, error) {
	if len(answer) != pxPeerWidth {
		return nil, pxUnavailable("invalid_inference_response")
	}
	r := pxFixed{data: answer}
	v := map[string]interface{}{"peer_key": r.id(), "worker": r.id(), "delegate": r.id(), "metadata": r.id()}
	var values [5]uint64
	for i, name := range []string{"generation", "key_version", "metadata_revision", "valid_from", "expiry"} {
		values[i] = r.number(8)
		v[name] = strconv.FormatUint(values[i], 10)
	}
	if values[0] == 0 || values[1] == 0 || values[2] == 0 || values[3] >= values[4] {
		r.bad = true
	}
	state := r.number(1)
	enrolled := r.number(1)
	tag := r.number(1)
	if state < 1 || state > 6 || enrolled > 1 {
		r.bad = true
	}
	v["state"] = state
	v["enrolled"] = enrolled == 1
	raw := r.take(13)
	v["capabilities"] = nil
	v["capabilities_status"] = tag
	if tag == 1 {
		c := pxFixed{data: raw}
		cap := map[string]interface{}{"accelerator_memory_mib": c.number(4), "system_memory_mib": c.number(4), "bandwidth_class": c.number(1), "region_code": string(c.take(2)), "attestation_family": c.number(1), "runtime_kind": c.number(1)}
		region := cap["region_code"].(string)
		if cap["bandwidth_class"].(uint64) > 4 || cap["runtime_kind"].(uint64) > 2 || len(region) != 2 || !pxAlpha(region[0]) || !pxAlpha(region[1]) {
			r.bad = true
		}
		v["capabilities"] = cap
	} else if tag != 4 || !bytes.Equal(raw, make([]byte, 13)) {
		r.bad = true
	}
	bond := r.number(1)
	if bond != 2 {
		r.bad = true
	}
	v["bond_status"] = bond
	v["fault_count"] = r.number(4)
	return v, r.end()
}
func pxAlpha(b byte) bool { return b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' }

// DecodePxInferenceReceipt decodes the fixed-width F11 ReceiptBatchView answer.
func DecodePxInferenceReceipt(answer []byte) (map[string]interface{}, error) {
	if len(answer) != 92 {
		return nil, pxUnavailable("invalid_inference_response")
	}
	r := pxFixed{data: answer}
	v := map[string]interface{}{"deal_id": r.id()}
	first := r.number(8)
	last := r.number(8)
	if last < first || last-first >= 64 {
		r.bad = true
	}
	v["first_lap_index"] = strconv.FormatUint(first, 10)
	v["last_lap_index"] = strconv.FormatUint(last, 10)
	v["cumulative_tokens"] = r.decimal(8)
	v["batch_digest"] = r.id()
	v["countersign_quorum"] = r.number(4)
	return v, r.end()
}

var pxPeerBindingABI = mustPxABI(`[{"type":"function","name":"peerBinding","inputs":[{"type":"bytes32"}],"outputs":[{"type":"uint8"},{"type":"bytes32"},{"type":"bytes32"},{"type":"uint64"}]}]`)

func (a *pxInferenceAPI) observe(ctx context.Context, method string, started time.Time, err error, recovered interface{}) {
	recordMetricsWithError(ctx, method, a.account.state.connectionType, started, err, recovered)
}
func (a *pxInferenceAPI) room(ctx context.Context, height int64, id [32]byte) (map[string]interface{}, error) {
	answer, err := a.account.programRead(ctx, height, 0x0b10, id[:])
	if err != nil {
		return nil, err
	}
	v, err := DecodePxInferenceRoom(answer)
	if err != nil {
		return nil, err
	}
	if v["room_id"] != hex.EncodeToString(id[:]) {
		return nil, pxUnavailable("invalid_inference_response")
	}
	payABI, err := layerxpay.GetABI()
	if err != nil {
		return nil, pxUnavailable("ask_stream_unavailable")
	}
	streams := make([]map[string]interface{}, 0)
	for _, ask := range v["open_asks"].([]string) {
		askID, _ := pxInferenceID(ask)
		values, err := a.account.call(ctx, height, common.HexToAddress("0x101b"), payABI, "askStreamOf", askID)
		if err != nil {
			return nil, pxUnavailable("ask_stream_unavailable")
		}
		stream := values[0].([32]byte)
		row := map[string]interface{}{"ask_id": ask, "stream_id": hex.EncodeToString(stream[:]), "stream": nil}
		if stream != ([32]byte{}) {
			values, err = a.account.call(ctx, height, common.HexToAddress("0x101b"), payABI, "getStream", stream)
			if err != nil {
				return nil, pxUnavailable("ask_stream_unavailable")
			}
			row["stream"] = pxStreamJSON(values[0])
		}
		streams = append(streams, row)
	}
	v["ask_streams"] = streams
	return v, nil
}
func pxStreamJSON(value interface{}) interface{} {
	stream := *abi.ConvertType(value, new(paytypes.StreamTuple)).(*paytypes.StreamTuple)
	authorities := make([]string, 0, len(stream.MeterAuthorities))
	for _, key := range stream.MeterAuthorities {
		authorities = append(authorities, hex.EncodeToString(key[:]))
	}
	return map[string]interface{}{"payer": hex.EncodeToString(stream.Payer[:]), "stream_account": hex.EncodeToString(stream.StreamAccount[:]), "recipient": hex.EncodeToString(stream.Recipient[:]), "asset_id": hex.EncodeToString(stream.AssetId[:]), "mode": stream.Mode, "rate": stream.Rate.String(), "rate_unit": strconv.FormatUint(stream.RateUnit, 10), "start_ms": strconv.FormatUint(stream.StartMs, 10), "last_accrual_ms": strconv.FormatUint(stream.LastAccrualMs, 10), "end_ms": strconv.FormatUint(stream.EndMs, 10), "total_cap": stream.TotalCap.String(), "accrued_total": stream.AccruedTotal.String(), "settled_total": stream.SettledTotal.String(), "remainder_carry": stream.RemainderCarry.String(), "cumulative_meter": strconv.FormatUint(stream.CumulativeMeter, 10), "meter_authorities": authorities, "underfunded": stream.Underfunded, "paused": stream.Paused, "closed": stream.Closed}
}
func (a *pxInferenceAPI) GetInferenceRoom(ctx context.Context, roomID string) (result map[string]interface{}, returnErr error) {
	start := time.Now()
	defer func() { a.observe(ctx, "px_getInferenceRoom", start, returnErr, recover()) }()
	id, err := pxInferenceID(roomID)
	if err != nil {
		return nil, err
	}
	height, err := a.account.height(ctx)
	if err != nil {
		return nil, err
	}
	return a.room(ctx, height, id)
}
func (a *pxInferenceAPI) GetInferenceRooms(ctx context.Context) (result []map[string]interface{}, returnErr error) {
	start := time.Now()
	defer func() { a.observe(ctx, "px_getInferenceRooms", start, returnErr, recover()) }()
	height, err := a.account.height(ctx)
	if err != nil {
		return nil, err
	}
	sections, err := a.account.programSections(ctx, height)
	if err != nil {
		return nil, err
	}
	ids, err := pxRoomIndex(sections[3])
	if err != nil {
		return nil, err
	}
	result = make([]map[string]interface{}, 0, len(ids))
	for _, id := range ids {
		v, err := a.room(ctx, height, id)
		if err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, nil
}
func (a *pxInferenceAPI) GetInferenceDeal(ctx context.Context, dealID string) (result map[string]interface{}, returnErr error) {
	start := time.Now()
	defer func() { a.observe(ctx, "px_getInferenceDeal", start, returnErr, recover()) }()
	id, err := pxInferenceID(dealID)
	if err != nil {
		return nil, err
	}
	height, err := a.account.height(ctx)
	if err != nil {
		return nil, err
	}
	answer, err := a.account.programRead(ctx, height, 0x0b11, id[:])
	if err != nil {
		return nil, err
	}
	result, err = DecodePxInferenceDeal(answer)
	if err == nil && result["deal_id"] != dealID {
		return nil, pxUnavailable("invalid_inference_response")
	}
	return result, err
}
func (a *pxInferenceAPI) GetInferencePeer(ctx context.Context, peerKey string) (result map[string]interface{}, returnErr error) {
	start := time.Now()
	defer func() { a.observe(ctx, "px_getInferencePeer", start, returnErr, recover()) }()
	id, err := pxInferenceID(peerKey)
	if err != nil {
		return nil, err
	}
	height, err := a.account.height(ctx)
	if err != nil {
		return nil, err
	}
	answer, err := a.account.programRead(ctx, height, 0x0b12, id[:])
	if err != nil {
		return nil, err
	}
	result, err = DecodePxInferencePeer(answer)
	if err != nil {
		return nil, err
	}
	if result["peer_key"] != peerKey {
		return nil, pxUnavailable("invalid_inference_response")
	}
	values, err := a.account.call(ctx, height, common.HexToAddress("0x0907"), pxPeerBindingABI, "peerBinding", id)
	if err != nil {
		return nil, pxUnavailable("peer_binding_unavailable")
	}
	model := values[1].([32]byte)
	runtime := values[2].([32]byte)
	result["attestation_binding"] = map[string]interface{}{"family": values[0], "model_digest": hex.EncodeToString(model[:]), "runtime_digest": hex.EncodeToString(runtime[:]), "block_height": strconv.FormatUint(values[3].(uint64), 10)}
	return result, nil
}
func (a *pxInferenceAPI) GetInferenceReceipts(ctx context.Context, dealID string) (result []map[string]interface{}, returnErr error) {
	start := time.Now()
	defer func() { a.observe(ctx, "px_getInferenceReceipts", start, returnErr, recover()) }()
	id, err := pxInferenceID(dealID)
	if err != nil {
		return nil, err
	}
	height, err := a.account.height(ctx)
	if err != nil {
		return nil, err
	}
	sections, err := a.account.programSections(ctx, height)
	if err != nil {
		return nil, err
	}
	digests, err := pxReceiptIndex(sections[5], id)
	if err != nil {
		return nil, err
	}
	result = make([]map[string]interface{}, 0, len(digests))
	for _, digest := range digests {
		payload := append(append([]byte{}, id[:]...), digest[:]...)
		answer, err := a.account.programRead(ctx, height, 0x0b13, payload)
		if err != nil {
			return nil, err
		}
		v, err := DecodePxInferenceReceipt(answer)
		if err != nil {
			return nil, err
		}
		if v["deal_id"] != dealID || v["batch_digest"] != hex.EncodeToString(digest[:]) {
			return nil, pxUnavailable("invalid_inference_response")
		}
		result = append(result, v)
	}
	return result, nil
}

func pxRoomIndex(section []byte) ([][32]byte, error) {
	ids := make([][32]byte, 0)
	if !bytes.HasSuffix(section, []byte("PAXAI/F11")) {
		return ids, nil
	}
	if len(section) < 13 {
		return nil, pxUnavailable("invalid_inference_response")
	}
	footer := len(section) - 13
	n := int(binary.BigEndian.Uint32(section[footer:]))
	if n > footer {
		return nil, pxUnavailable("invalid_inference_response")
	}
	book := section[footer-n : footer]
	if len(book) < 14 || string(book[:4]) != "F11B" || binary.BigEndian.Uint16(book[4:6]) != 1 {
		return nil, pxUnavailable("invalid_inference_response")
	}
	counts := [4]int{}
	caps := [4]int{16, 64, 128, 16}
	widths := [4]int{209, 193, 162, 1298}
	total := 14
	for i := range counts {
		counts[i] = int(binary.BigEndian.Uint16(book[6+2*i:]))
		if counts[i] > caps[i] {
			return nil, pxUnavailable("invalid_inference_response")
		}
		total += counts[i] * widths[i]
	}
	if len(book) != total {
		return nil, pxUnavailable("invalid_inference_response")
	}
	for i := 0; i < counts[0]; i++ {
		var id [32]byte
		copy(id[:], book[14+i*209:14+i*209+32])
		if id == ([32]byte{}) || i > 0 && bytes.Compare(ids[i-1][:], id[:]) >= 0 {
			return nil, pxUnavailable("invalid_inference_response")
		}
		ids = append(ids, id)
	}
	return ids, nil
}
func pxReceiptIndex(section []byte, deal [32]byte) ([][32]byte, error) {
	result := make([][32]byte, 0)
	// The control payload contains a canonical replay table then length-prefixed features.
	r := pxFixed{data: section}
	if r.number(2) != 1 || r.number(2) != 0 {
		return nil, pxUnavailable("invalid_inference_response")
	}
	actors := r.number(2)
	if actors > 43 {
		return nil, pxUnavailable("invalid_inference_response")
	}
	previousSlot := uint64(0)
	for i := uint64(0); i < actors; i++ {
		slot := r.number(2)
		if slot >= 43 || i > 0 && slot <= previousSlot {
			r.bad = true
		}
		previousSlot = slot
		r.id()
		if r.number(8) == 0 {
			r.bad = true
		}
		last := r.number(1)
		if last == 1 {
			r.take(120)
		} else if last != 0 {
			r.bad = true
		}
	}
	n := r.number(4)
	if n > uint64(len(r.data)) {
		return nil, pxUnavailable("invalid_inference_response")
	}
	features := r.take(int(n))
	if r.end() != nil {
		return nil, r.end()
	}
	if len(features) == 0 {
		return result, nil
	}
	r = pxFixed{data: features}
	r.take(8)
	seals := r.number(1)
	if seals > 8 {
		return nil, pxUnavailable("invalid_inference_response")
	}
	r.take(int(seals) * 185)
	if r.bad {
		return nil, r.end()
	}
	if !bytes.HasPrefix(r.data, []byte("LAPSETT1")) {
		return result, nil
	}
	r.take(8)
	count := r.number(1)
	if count == 0 || count > 16 {
		return nil, pxUnavailable("invalid_inference_response")
	}
	var previous []byte
	for i := uint64(0); i < count; i++ {
		row := r.take(188)
		if r.bad {
			return nil, r.end()
		}
		if i > 0 && bytes.Compare(previous, row[:32]) >= 0 {
			return nil, pxUnavailable("invalid_inference_response")
		}
		previous = row[:32]
		if bytes.Equal(row[:32], deal[:]) {
			var digest [32]byte
			copy(digest[:], row[56:88])
			result = append(result, digest)
		}
	}
	// Other policy extensions follow the settlement region and are owned by the producer.
	return result, nil
}
