package ogrender

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/ogrender/fonts"
	"github.com/fogleman/gg"
)

func TestStrictRendererRequiresCommittedFontsAndFittingLabels(t *testing.T) {
	if _, err := NewStrict(""); err == nil {
		t.Fatal("missing font authority accepted")
	}
	if _, err := NewStrict(t.TempDir()); err == nil {
		t.Fatal("missing font faces accepted")
	}
	var missing *Renderer
	if _, err := missing.Render(Input{}); err == nil {
		t.Fatal("missing renderer accepted")
	}
	directory := filepath.Join(t.TempDir(), "fonts")
	if err := fonts.Materialize(directory); err != nil {
		t.Fatal(err)
	}
	renderer, err := NewStrict(directory)
	if err != nil {
		t.Fatal(err)
	}
	data, err := renderer.Render(Input{Title: "Canonical portfolio", Multiple: "1.25x", Pnl: "+25 USDL", PnlPositive: true, Holdings: "100 tokens", Footer: "sidiora.fun"})
	if err != nil {
		t.Fatal(err)
	}
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width != 1200 || config.Height != 630 {
		t.Fatal(config, err)
	}
	for _, input := range []Input{{Title: strings.Repeat("a", 4097)}, {Multiple: strings.Repeat("9", 300)}, {Pnl: strings.Repeat("9", 300)}, {Footer: strings.Repeat("w", 300)}} {
		if _, err := renderer.Render(input); err == nil {
			t.Fatal("unbounded card label accepted")
		}
	}
	if got := truncate("abc", 1); got != "a" {
		t.Fatal(got)
	}
	if err = os.Remove(filepath.Join(directory, fontBold)); err != nil {
		t.Fatal(err)
	}
	if _, err = renderer.Render(Input{}); err == nil {
		t.Fatal("removed font authority accepted")
	}
	if err = renderer.fitFace(gg.NewContext(1, 1), fontBold, 30, "amount"); err == nil {
		t.Fatal("missing face accepted")
	}
}
