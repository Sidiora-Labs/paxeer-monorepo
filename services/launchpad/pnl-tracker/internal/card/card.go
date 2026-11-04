// Package card reads archived legacy PnL cards and formats their snapshots.
// New public disclosures must use owner-approved v3 publications; legacy card
// identifiers and short codes are not v3 share identifiers.
package card

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/ogrender"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/pnlmath"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/store"
)

// ErrNoPosition is retained for callers of the retired legacy mint interface.
var ErrNoPosition = errors.New("card: no position for owner/pool")

// ErrMintRetired rejects legacy publication without creating a card or referral.
var ErrMintRetired = errors.New("card: legacy minting retired; use owner-approved /v3/me/portfolio/shares publications")

// Position is the numeric subset of a position embedded in a snapshot
// (pnl.ts: Omit<UserPosition,'userAddress'|'poolAddress'|'tokenAddress'>).
type Position struct {
	TotalUsdlSpent    string `json:"totalUsdlSpent"`
	TotalTokensBought string `json:"totalTokensBought"`
	TotalUsdlReceived string `json:"totalUsdlReceived"`
	TotalTokensSold   string `json:"totalTokensSold"`
	AvgCostBasis      string `json:"avgCostBasis"`
	CurrentHoldings   string `json:"currentHoldings"`
	RealizedPnlUsdl   string `json:"realizedPnlUsdl"`
	FirstBuyTs        *int64 `json:"firstBuyTs"`
	LastTradeTs       int64  `json:"lastTradeTs"`
	TradeCount        int    `json:"tradeCount"`
}

// Market is the captured market context (pnl.ts CardSnapshot.market). Nullable.
type Market struct {
	PriceWad          *string `json:"priceWad"`
	MarketCapUsdl     *string `json:"marketCapUsdl"`
	PriceChange24hBps *string `json:"priceChange24hBps"`
}

// Snapshot is the immutable card payload (pnl.ts CardSnapshot).
type Snapshot struct {
	Version      int      `json:"version"`
	OwnerAddress string   `json:"ownerAddress"`
	PoolAddress  string   `json:"poolAddress"`
	TokenAddress string   `json:"tokenAddress"`
	TokenSymbol  string   `json:"tokenSymbol,omitempty"`
	TokenName    string   `json:"tokenName,omitempty"`
	Position     Position `json:"position"`
	Market       Market   `json:"market"`
	CapturedAt   int64    `json:"capturedAt"`
}

// Minted is the archived hydrate response (pnl.ts MintedCard).
// ShareURL is empty: archived cards have no supported public landing contract.
type Minted struct {
	CardID    string   `json:"cardId"`
	ShortCode string   `json:"shortCode"`
	ShareURL  string   `json:"shareUrl"`
	OgURL     string   `json:"ogUrl"`
	Snapshot  Snapshot `json:"snapshot"`
	CreatedAt int64    `json:"createdAt,omitempty"`
}

// Service reads archived cards. Its legacy mint interface is retired.
type Service struct {
	store    *store.Store
	ogOrigin string
}

// New builds an archive Service. The legacy share origin and clock parameters
// are retained for source compatibility; archives do not issue share links.
func New(st *store.Store, shareOrigin, ogOrigin string, now func() int64) *Service {
	return &Service{store: st, ogOrigin: ogOrigin}
}

func (s *Service) ogURL(cardID string) string {
	return s.ogOrigin + "/pnl/cards/" + cardID + "/og.png"
}

// Mint is retired. A wallet/pool pair cannot express owner approval, selected
// disclosure, expiry or revocation, so it must never create a v3 publication or
// emit a link to the v3 public landing page.
func (s *Service) Mint(ctx context.Context, owner, pool string) (*Minted, error) {
	return nil, ErrMintRetired
}

// Get hydrates a minted card by id, or (nil, nil) when unknown.
func (s *Service) Get(ctx context.Context, cardID string) (*Minted, error) {
	row, err := s.store.GetCard(ctx, cardID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	return s.DecodeArchive(*row)
}

// DecodeArchive hydrates a stored legacy disclosure without issuing a share URL.
func (s *Service) DecodeArchive(row store.CardRow) (*Minted, error) {
	var snap Snapshot
	if err := json.Unmarshal(row.Snapshot, &snap); err != nil {
		return nil, fmt.Errorf("card: decode snapshot: %w", err)
	}
	return &Minted{
		CardID:    row.CardID,
		ShortCode: row.ShortCode,
		OgURL:     s.ogURL(row.CardID),
		Snapshot:  snap,
		CreatedAt: row.CreatedAt,
	}, nil
}

// wad is the 1e18 fixed-point scale used by price/cost basis.
var wad = new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

// BuildRenderInput maps a snapshot to the flat, formatted ogrender.Input. The
// multiple and signed-PnL strings are derived with math/big so the displayed
// figures match the client's computeMultiple/computeTotalPnlUsdl (no float).
func BuildRenderInput(snap Snapshot, footer string) ogrender.Input {
	priceWad := ""
	if snap.Market.PriceWad != nil {
		priceWad = *snap.Market.PriceWad
	}
	pnlText, positive := formatSignedUsdl(snap.Position.RealizedPnlUsdl, snap.Position.AvgCostBasis, snap.Position.CurrentHoldings, priceWad)
	return ogrender.Input{
		Title:       titleFor(snap),
		Multiple:    formatMultiple(snap.Position, priceWad),
		Pnl:         pnlText,
		PnlPositive: positive,
		Holdings:    formatHoldings(snap.Position.CurrentHoldings, snap.TokenSymbol),
		Footer:      footer,
	}
}

func titleFor(snap Snapshot) string {
	if snap.TokenName != "" {
		return snap.TokenName
	}
	if snap.TokenSymbol != "" {
		return "$" + snap.TokenSymbol
	}
	return snap.PoolAddress
}

// formatMultiple renders (received + unrealized) / spent as "N.NNx".
func formatMultiple(p Position, priceWad string) string {
	spent, ok := new(big.Int).SetString(orZero(p.TotalUsdlSpent), 10)
	if !ok || spent.Sign() == 0 {
		return "0.00x"
	}
	received, _ := new(big.Int).SetString(orZero(p.TotalUsdlReceived), 10)
	unrealizedStr, err := pnlmath.HoldingValue(priceWad, p.CurrentHoldings)
	if err != nil {
		unrealizedStr = "0"
	}
	unrealized, _ := new(big.Int).SetString(orZero(unrealizedStr), 10)

	// milli = (received + unrealized) * 1000 / spent.
	num := new(big.Int).Add(received, unrealized)
	num.Mul(num, big.NewInt(1000))
	milli := num.Quo(num, spent)
	whole := new(big.Int).Quo(milli, big.NewInt(1000))
	frac := new(big.Int).Mod(milli, big.NewInt(1000))
	cents := new(big.Int).Quo(frac, big.NewInt(10)) // two-digit precision
	return fmt.Sprintf("%s.%02dx", whole.String(), cents.Int64())
}

// formatSignedUsdl renders the total PnL as "+$D.CC" / "-$D.CC" and its sign.
func formatSignedUsdl(realized, avgCost, holdings, priceWad string) (string, bool) {
	total, err := pnlmath.TotalPnl(realized, avgCost, holdings, priceWad)
	if err != nil {
		return "$0.00", true
	}
	v, _ := new(big.Int).SetString(total, 10)
	if v == nil {
		return "$0.00", true
	}
	positive := v.Sign() >= 0
	abs := new(big.Int).Abs(v)
	million := big.NewInt(1_000_000)
	dollars := new(big.Int).Quo(abs, million)
	rem := new(big.Int).Mod(abs, million)
	cents := new(big.Int).Quo(rem, big.NewInt(10_000))
	sign := "+"
	if !positive {
		sign = "-"
	}
	return fmt.Sprintf("%s$%s.%02d", sign, dollars.String(), cents.Int64()), positive
}

// formatHoldings renders a token balance (18-dec raw) as "N.NN SYMBOL held".
func formatHoldings(holdings, symbol string) string {
	v, ok := new(big.Int).SetString(orZero(holdings), 10)
	if !ok {
		v = new(big.Int)
	}
	whole := new(big.Int).Quo(v, wad)
	frac := new(big.Int).Mod(v, wad)
	// two decimals: frac / 1e16.
	cents := new(big.Int).Quo(frac, new(big.Int).Exp(big.NewInt(10), big.NewInt(16), nil))
	sym := symbol
	if sym == "" {
		sym = "tokens"
	}
	return fmt.Sprintf("%s.%02d %s held", whole.String(), cents.Int64(), sym)
}

func orZero(s string) string {
	if s == "" {
		return "0"
	}
	return s
}
