package projectionstats

import (
	"math/big"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPublicStatsCreationWithoutTradesOrHolderCoverage(t *testing.T) {
	f := newPublicFixture(t)
	first := f.real.Transactions[0]
	f.creation(first, 14)
	f.advance(first.BlockNumber, first.BlockHash)
	projector := f.projector()
	result, err := projector.RefreshOnce(f.ctx, time.Unix(f.real.Blocks["33963749"].Timestamp+60, 0))
	if err != nil || result.Pools != 1 || result.Appended != 1 || result.Prices != 0 {
		t.Fatalf("creation-only statistics: %+v %v", result, err)
	}
	frame := f.frame(f.real.Pool)
	if frame.frame.Payload.HolderCount != nil || frame.frame.Payload.SpotPriceWad != nil || frame.frame.Payload.TxCount24h != "0" || frame.frame.Payload.Buys24h != "0" {
		t.Fatalf("unmeasured creation: %+v", frame.frame.Payload)
	}
	if result.Anchor == nil || result.Anchor.Height != strconv.FormatInt(first.BlockNumber, 10) {
		t.Fatalf("creation anchor: %+v", result.Anchor)
	}
}

func TestPublicStatsReversesPostAnchorMintAndBurn(t *testing.T) {
	f := newPublicFixture(t)
	fills := f.ingestReal()
	f.advance(33963761, f.real.Blocks["33963761"].Hash)
	projector := f.projector()
	now := time.Unix(f.real.Blocks["33963749"].Timestamp+60, 0)
	result, err := projector.RefreshOnce(f.ctx, now)
	if err != nil || result.Appended != 1 {
		t.Fatalf("anchored statistics: %+v %v", result, err)
	}
	before := f.frame(f.real.Pool).wire
	owner := f.real.Transactions[0].From
	zero := "0x" + strings.Repeat("0", 40)
	later := realTransaction{Hash: "0x" + strings.Repeat("e", 64), From: owner, BlockNumber: 33963800, BlockHash: "0x" + strings.Repeat("d", 64)}
	amount := big.NewInt(1000)
	for i, pair := range [][2]string{{zero, owner}, {owner, zero}} {
		log := realLog{LogIndex: int64(i), Address: f.real.Token, Topics: []string{transferTopic, "0x000000000000000000000000" + pair[0][2:], "0x000000000000000000000000" + pair[1][2:]}, Data: encodeWords(amount)}
		f.rawLog(later, log)
		f.transfer(later, log)
	}
	f.advance(later.BlockNumber, later.BlockHash)
	result, err = projector.RefreshOnce(f.ctx, now.Add(time.Minute))
	if err != nil || result.Appended != 0 || result.Prices != 0 {
		t.Fatalf("post-anchor supply changes: %+v %v", result, err)
	}
	if after := f.frame(f.real.Pool).wire; string(after) != string(before) {
		t.Fatalf("historical statistics changed after balanced mint and burn: %s", after)
	}
	if fills["sell"].Position.Height != "33963749" {
		t.Fatalf("unexpected sale anchor: %+v", fills["sell"].Position)
	}
}
