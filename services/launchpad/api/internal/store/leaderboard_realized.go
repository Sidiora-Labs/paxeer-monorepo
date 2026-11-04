package store

import (
	"context"
	"encoding/json"
	"math/big"
	"sort"
	"strconv"
	"strings"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/jackc/pgx/v5"
)

// traderFill is one committed fill seen from the wallet whose inventory it
// moves: the recipient of a buy, the payer of a sell.
type traderFill struct {
	Wallet, Pool, Side   string
	TokenRaw, PaidRaw    string
	USDLRaw              string
	Timestamp            int64
	AttributionAvailable bool
	BasisUnknown         bool
}

// traderRealizedRows loads, for every wallet/pool pair with a sell inside
// (from, windowEnd], that pair's whole fill history up to windowEnd in
// canonical chain order. Earlier buys are needed for the cost basis of the
// in-window disposals.
func traderRealizedRows(ctx context.Context, tx pgx.Tx, commits, projectionVersion string, from, windowEnd int64) ([]traderFill, error) {
	rows, err := tx.Query(ctx, `WITH f AS (SELECT CASE WHEN fill->>'side'='buy' THEN COALESCE(fill->'attribution'->>'recipient',fill->'attribution'->>'trader',fill->'attribution'->>'transactionSender') ELSE COALESCE(fill->'attribution'->>'payer',fill->'attribution'->>'trader',fill->'attribution'->>'transactionSender') END AS wallet,pool,fill,(fill->>'blockTimestampSeconds')::bigint AS ts FROM `+commits+` WHERE projection_version=$1 AND (fill->>'blockTimestampSeconds')::bigint <= $3) SELECT fill FROM f WHERE (wallet,pool) IN (SELECT wallet,pool FROM f WHERE fill->>'side'='sell' AND ts > $2) ORDER BY wallet,pool,(fill->'position'->>'height')::numeric,(fill->'position'->>'transactionIndex')::numeric,(fill->'position'->>'logIndex')::numeric`, projectionVersion, from, windowEnd)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	fills := []accounting.FillV1{}
	type movementScope struct {
		Wallet string `json:"wallet"`
		Token  string `json:"token"`
		Height int64  `json:"height"`
	}
	scopes := map[string]movementScope{}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		fill, decodeErr := accounting.DecodeFillV1(raw)
		if decodeErr != nil {
			return nil, ErrDataIntegrity
		}
		fills = append(fills, fill)
		wallet := traderInventoryOwner(fill)
		height, _ := strconv.ParseInt(fill.Position.Height, 10, 64)
		key := wallet + "|" + fill.Token.Address
		if old, ok := scopes[key]; !ok || old.Height < height {
			scopes[key] = movementScope{wallet, fill.Token.Address, height}
		}
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(fills) == 0 {
		return []traderFill{}, nil
	}
	scopeList := make([]movementScope, 0, len(scopes))
	for _, scope := range scopes {
		scopeList = append(scopeList, scope)
	}
	rawScopes, err := json.Marshal(scopeList)
	if err != nil {
		return nil, err
	}
	rows, err = tx.Query(ctx, `SELECT DISTINCT m.payload FROM indexer.indexer_token_transfers_v1 m JOIN jsonb_to_recordset($1::jsonb) AS s(wallet text,token text,height bigint) ON m.token=s.token AND (m.from_address=s.wallet OR m.to_address=s.wallet) AND m.height<=s.height WHERE m.chain_id=125`, rawScopes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	movements := []traderTokenTransfer{}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var movement traderTokenTransfer
		if err = json.Unmarshal(raw, &movement); err != nil {
			return nil, ErrDataIntegrity
		}
		movements = append(movements, movement)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return traderProjectionRows(fills, movements)
}

// traderTokenTransfer is the canonical indexer TransferFact payload, read from
// its versioned view in the same repeatable-read transaction as the fills.
type traderTokenTransfer struct {
	Token      string                    `json:"token"`
	From       string                    `json:"from"`
	To         string                    `json:"to"`
	AmountRaw  string                    `json:"amountRaw"`
	Position   accounting.SourcePosition `json:"position"`
	Provenance accounting.Provenance     `json:"provenance"`
}

func traderInventoryOwner(f accounting.FillV1) string {
	owner := f.Attribution.Payer
	if f.Side == "buy" {
		owner = f.Attribution.Recipient
	}
	if owner != nil {
		return *owner
	}
	if f.Attribution.Trader != nil {
		return *f.Attribution.Trader
	}
	return f.Attribution.TransactionSender
}

// traderProjectionRows admits basis only when every movement can be tied to a
// fill's receipt evidence. An unresolved movement permanently excludes that
// wallet/pool position; later buys cannot establish the unrelated tokens' cost.
func traderProjectionRows(fills []accounting.FillV1, movements []traderTokenTransfer) ([]traderFill, error) {
	groups := map[string][]accounting.FillV1{}
	keys := []string{}
	byOwner := map[string][]traderTokenTransfer{}
	identities := map[string]bool{}
	for _, m := range movements {
		amount, err := accounting.Uint(m.AmountRaw)
		if err != nil || accounting.ValidateSourcePosition(m.Position) != nil {
			return nil, ErrDataIntegrity
		}
		if _, err = Address(m.Token); err != nil {
			return nil, ErrDataIntegrity
		}
		for _, a := range []string{m.From, m.To} {
			if a != "0x"+strings.Repeat("0", 40) {
				if _, err = Address(a); err != nil {
					return nil, ErrDataIntegrity
				}
			}
		}
		id := m.Position.TransactionHash + ":" + m.Position.LogIndex
		if identities[id] {
			return nil, ErrDataIntegrity
		}
		identities[id] = true
		if amount.Sign() == 0 || m.From == m.To {
			continue
		}
		byOwner[m.From+"|"+m.Token] = append(byOwner[m.From+"|"+m.Token], m)
		byOwner[m.To+"|"+m.Token] = append(byOwner[m.To+"|"+m.Token], m)
	}
	for _, f := range fills {
		if accounting.ValidateFillV1(f) != nil {
			return nil, ErrDataIntegrity
		}
		key := traderInventoryOwner(f) + "|" + f.PoolAddress
		if _, ok := groups[key]; !ok {
			keys = append(keys, key)
		}
		groups[key] = append(groups[key], f)
	}
	sort.Strings(keys)
	out := []traderFill{}
	for _, key := range keys {
		history := groups[key]
		sort.SliceStable(history, func(i, j int) bool {
			c, _ := accounting.CompareSourcePosition(history[i].Position, history[j].Position)
			return c < 0
		})
		owner := traderInventoryOwner(history[0])
		moves := append([]traderTokenTransfer(nil), byOwner[owner+"|"+history[0].Token.Address]...)
		sort.SliceStable(moves, func(i, j int) bool {
			c, _ := accounting.CompareSourcePosition(moves[i].Position, moves[j].Position)
			return c < 0
		})
		movementByID := map[string]traderTokenTransfer{}
		for _, m := range moves {
			movementByID[m.Position.TransactionHash+":"+m.Position.LogIndex] = m
		}
		matched := map[string]bool{}
		proven := map[string]bool{}
		for _, f := range history {
			if f.Token.Address != history[0].Token.Address {
				return nil, ErrDataIntegrity
			}
			kind, from, to := "output-transfer", f.PoolAddress, owner
			if f.Side == "sell" {
				kind, from, to = "input-transfer", owner, f.PoolAddress
			}
			refs := []accounting.EvidenceRef{}
			for _, ref := range f.Attribution.Evidence {
				if ref.Kind == kind {
					refs = append(refs, ref)
				}
			}
			if len(refs) != 1 || refs[0].LogIndex == nil {
				continue
			}
			ref := refs[0]
			m, found := movementByID[ref.ArtifactID+":"+*ref.LogIndex]
			if !found || ref.ArtifactID != f.Position.TransactionHash || ref.ArtifactSHA256 != f.Provenance.ReceiptSHA256 {
				continue
			}
			c, err := accounting.CompareSourcePosition(m.Position, f.Position)
			if err != nil {
				return nil, ErrDataIntegrity
			}
			if c >= 0 || m.Position.Height != f.Position.Height || m.Position.TransactionIndex != f.Position.TransactionIndex || m.Position.BlockHash != f.Position.BlockHash || m.From != from || m.To != to || m.AmountRaw != f.TokenAmountRaw || m.Provenance.ReceiptSHA256 != ref.ArtifactSHA256 {
				continue
			}
			id := m.Position.TransactionHash + ":" + m.Position.LogIndex
			if matched[id] {
				return nil, ErrDataIntegrity
			}
			matched[id], proven[f.SourceFillID] = true, true
		}
		unknown, next := false, 0
		for _, f := range history {
			for next < len(moves) {
				m := moves[next]
				c, err := accounting.CompareSourcePosition(m.Position, f.Position)
				if err != nil {
					return nil, ErrDataIntegrity
				}
				if c > 0 {
					break
				}
				if !matched[m.Position.TransactionHash+":"+m.Position.LogIndex] {
					unknown = true
				}
				next++
			}
			unknown = unknown || !proven[f.SourceFillID]
			timestamp, _ := strconv.ParseInt(f.BlockTimestampSeconds, 10, 64)
			out = append(out, traderFill{Wallet: owner, Pool: f.PoolAddress, Side: f.Side, TokenRaw: f.TokenAmountRaw, PaidRaw: f.PaidInputRaw, USDLRaw: f.USDLAmountRaw, Timestamp: timestamp, AttributionAvailable: f.Attribution.Quality != "unavailable", BasisUnknown: unknown})
		}
	}
	return out, nil
}

// RealizedTraderPnL ranks wallets by realised PnL on disposals inside
// (from, windowEnd]. fills must be grouped by wallet and pool and ordered by
// chain position inside each group.
//
// Per wallet and pool the known inventory K and its capital basis C follow the
// committed fills: a buy adds tokenAmountRaw to K and paidInputRaw (USDL paid
// including the fee) to C. A sell of q tokens for usdlAmountRaw proceeds P
// disposes basis c = floor(C*q/K) (c = C when q = K, proportional_floor_v1)
// and realises P - c. A sell larger than K moves tokens whose cost basis the
// fills do not contain (transferred in); its profit cannot be known, so a
// wallet with such a disposal inside the window is left off the ranking and
// counted in the returned unknown-basis total instead.
func RealizedTraderPnL(fills []traderFill, from, windowEnd int64) ([]LeaderboardEntry, int, error) {
	type state struct{ known, cost *big.Int }
	type wallet struct {
		realized              *big.Int
		disposals             int
		unknownBasis, partial bool
	}
	positions := map[string]*state{}
	wallets := map[string]*wallet{}
	order := []string{}
	for _, f := range fills {
		address, err := Address(f.Wallet)
		if err != nil || (f.Side != "buy" && f.Side != "sell") {
			return nil, 0, ErrDataIntegrity
		}
		token, okToken := new(big.Int).SetString(f.TokenRaw, 10)
		paid, okPaid := new(big.Int).SetString(f.PaidRaw, 10)
		usdl, okUSDL := new(big.Int).SetString(f.USDLRaw, 10)
		if !okToken || !okPaid || !okUSDL || token.Sign() < 0 || paid.Sign() < 0 || usdl.Sign() < 0 {
			return nil, 0, ErrDataIntegrity
		}
		key := address + "|" + f.Pool
		p := positions[key]
		if p == nil {
			p = &state{new(big.Int), new(big.Int)}
			positions[key] = p
		}
		if f.Side == "buy" {
			p.known.Add(p.known, token)
			p.cost.Add(p.cost, paid)
			continue
		}
		inWindow := f.Timestamp > from && f.Timestamp <= windowEnd
		w := wallets[address]
		if inWindow && w == nil {
			w = &wallet{realized: new(big.Int)}
			wallets[address] = w
			order = append(order, address)
		}
		if f.BasisUnknown || token.Cmp(p.known) > 0 {
			// Unresolved provenance or disposal beyond fill-known inventory.
			if inWindow {
				w.unknownBasis = true
			}
			p.known.SetInt64(0)
			p.cost.SetInt64(0)
			continue
		}
		basis := new(big.Int).Set(p.cost)
		if token.Cmp(p.known) != 0 {
			basis.Mul(p.cost, token)
			basis.Quo(basis, p.known)
		}
		p.known.Sub(p.known, token)
		p.cost.Sub(p.cost, basis)
		if inWindow {
			w.realized.Add(w.realized, new(big.Int).Sub(usdl, basis))
			w.disposals++
			if !f.AttributionAvailable {
				w.partial = true
			}
		}
	}
	entries := []LeaderboardEntry{}
	excluded := 0
	for _, address := range order {
		w := wallets[address]
		if w.unknownBasis {
			excluded++
			continue
		}
		if w.disposals == 0 {
			continue
		}
		entry := LeaderboardEntry{Wallet: address, MetricRaw: new(big.Int).Abs(w.realized).String(), MetricSigned: w.realized.String(), Quality: "complete", signed: w.realized}
		if w.partial {
			entry.Quality = "partial"
		}
		if len(entry.MetricRaw) > 156 {
			return nil, 0, ErrDataIntegrity
		}
		entries = append(entries, entry)
	}
	return entries, excluded, nil
}

func realizedDigestSuffix(excluded int) string {
	return "|unknown-basis:" + strconv.Itoa(excluded)
}
