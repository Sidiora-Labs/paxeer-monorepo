package bringup

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

var xwebSymbols = []string{"SID", "PAX", "USDC", "USDL"}

type XwebAssetInput struct {
	ID       string
	Decimals int
}
type XwebOptions struct {
	Gateway, Node, ReceiptNode, SIDPerPAX, PAXUSD, Seeds string
	Assets                                               map[string]XwebAssetInput
}
type XwebAsset struct {
	ID    string `json:"asset_id"`
	Price string `json:"price"`
}

func XwebEnvironment(get func(string) string) (XwebOptions, error) {
	o := XwebOptions{Gateway: get("XWEB_GATEWAY_ENDPOINT"), Node: get("XWEB_EVM_ENDPOINT"), ReceiptNode: get("XWEB_RECEIPT_KEYS_ENDPOINT"), SIDPerPAX: get("XWEB_SID_PER_PAX"), PAXUSD: get("XWEB_PAX_USD_PRICE"), Seeds: get("XWEB_SEEDS"), Assets: map[string]XwebAssetInput{}}
	if o.Node == "" {
		o.Node = "http://127.0.0.1:8545"
	}
	if o.ReceiptNode == "" {
		o.ReceiptNode = o.Node
	}
	if o.SIDPerPAX == "" {
		o.SIDPerPAX = "3.114"
	}
	for _, sym := range xwebSymbols {
		text := get("XWEB_DECIMALS_" + sym)
		n, e := strconv.Atoi(text)
		if e != nil || strconv.Itoa(n) != text || n < 0 || n > 38 {
			return o, errors.New("xweb asset decimals must be explicit and bounded")
		}
		id := strings.ToLower(strings.TrimPrefix(get("XWEB_ASSET_ID_"+sym), "0x"))
		o.Assets[sym] = XwebAssetInput{id, n}
	}
	return o, o.validate()
}
func (o XwebOptions) validate() error {
	if o.Gateway == "" || protectedURL(o.Gateway) != nil {
		return errors.New("hosted gateway endpoint is not configured (XWEB_GATEWAY_ENDPOINT)")
	}
	if upgradeRoot(o.Node) != nil || upgradeRoot(o.ReceiptNode) != nil || o.Gateway == o.Node || o.Gateway == o.ReceiptNode {
		return errors.New("xweb requires separate hosted gateway and node root transports")
	}
	if len(o.Assets) != 4 {
		return errors.New("four explicit xweb assets required")
	}
	seen := map[string]bool{}
	for _, sym := range xwebSymbols {
		a, ok := o.Assets[sym]
		if !ok || !upgradeDigest.MatchString(a.ID) || a.ID == strings.Repeat(a.ID[:1], 64) || seen[a.ID] || a.Decimals < 0 || a.Decimals > 38 {
			return errors.New("xweb asset identity or decimals refused")
		}
		seen[a.ID] = true
	}
	return nil
}
func XwebPrices(ctx context.Context, client *http.Client, o XwebOptions) (map[string]XwebAsset, error) {
	if e := o.validate(); e != nil {
		return nil, e
	}
	sid, ok := new(big.Rat).SetString(o.SIDPerPAX)
	if !ok || sid.Sign() <= 0 {
		return nil, errors.New("XWEB_SID_PER_PAX is malformed")
	}
	if o.PAXUSD != "" {
		usd, ok := new(big.Rat).SetString(o.PAXUSD)
		if !ok || usd.Cmp(big.NewRat(336, 25)) != 0 {
			return nil, errors.New("XWEB_PAX_USD_PRICE differs from the approved 13.44")
		}
	}
	raw, e := callRPC(ctx, client, o.Gateway, "px_listAssets", []any{}, "", true)
	if e != nil {
		return nil, e
	}
	inventory, e := readCommittedAssetInventory(raw)
	if e != nil {
		return nil, e
	}
	prices := map[string]XwebAsset{}
	for _, sym := range xwebSymbols {
		a := o.Assets[sym]
		found := 0
		for _, record := range inventory.Assets {
			if record.ID != a.ID {
				continue
			}
			found++
			if record.Symbol != sym || record.Decimals == nil || *record.Decimals != a.Decimals || record.Paused == nil || *record.Paused || record.Verification != "authenticated_committed_snapshot" {
				return nil, errors.New("xweb asset contradicts authenticated committed inventory")
			}
		}
		if found != 1 {
			return nil, errors.New("xweb asset is missing or duplicated in hosted inventory")
		}
		amount := big.NewRat(1, 1000)
		if sym == "PAX" || sym == "SID" {
			amount = big.NewRat(1, 13440)
		}
		if sym == "SID" {
			amount.Mul(amount, sid)
		}
		amount.Mul(amount, new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(a.Decimals)), nil)))
		price := new(big.Int).Add(amount.Num(), new(big.Int).Sub(amount.Denom(), big.NewInt(1)))
		price.Div(price, amount.Denom())
		if price.Sign() <= 0 || price.BitLen() > 128 {
			return nil, errors.New("xweb price exceeds loader amount bounds")
		}
		prices[sym] = XwebAsset{a.ID, price.String()}
	}
	return prices, nil
}
func xwebTagged(domain string, public []byte) []byte {
	digest := sha256.New()
	for _, part := range [][]byte{[]byte(domain), []byte("beta"), public} {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(part)))
		digest.Write(size[:])
		digest.Write(part)
	}
	return digest.Sum(nil)
}
func RegisterXweb(ctx context.Context, client *http.Client, gateway, path string, output io.Writer) error {
	if gateway == "" || protectedURL(gateway) != nil {
		return errors.New("explicit protected hosted gateway required")
	}
	info, e := os.Lstat(path)
	if e != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 256 {
		return errors.New("XWEB_REGISTER_KEY_FILE must be a protected regular file")
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || st.Uid != uint32(os.Geteuid()) {
		return errors.New("registration key has a different owner")
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		return errors.New("registration key unreadable")
	}
	seed, e := hex.DecodeString(strings.TrimSpace(string(raw)))
	if e != nil || len(seed) != ed25519.SeedSize {
		return errors.New("registration key must contain 32 hexadecimal bytes")
	}
	key := ed25519.NewKeyFromSeed(seed)
	public := key.Public().(ed25519.PublicKey)
	encoded := hex.EncodeToString(public)
	subject := "beta." + hex.EncodeToString(xwebTagged("layerx-register-subject-v1", public))[:32]
	signature := hex.EncodeToString(ed25519.Sign(key, xwebTagged("layerx-register-binding-v1", public)))
	result, e := callRPC(ctx, client, gateway, "px_register", []string{encoded, signature}, "", true)
	if e != nil {
		return e
	}
	var principal struct {
		Tenant  string   `json:"tenant"`
		Subject string   `json:"sub"`
		Signers []string `json:"allowed_signer_public_keys"`
	}
	if uniqueJSON(result) != nil || json.Unmarshal(result, &principal) != nil || principal.Tenant != "beta" || principal.Subject != subject || len(principal.Signers) != 1 || principal.Signers[0] != encoded {
		return errors.New("px_register answered a principal that is not this key")
	}
	_, e = fmt.Fprintf(output, "registered public_key=%s did=did:layerx:%s sub=%s tenant=beta\n", encoded, encoded, subject)
	return e
}

// RenderXweb loads node trust through its root RPC and publishes loader input
// only after asset metadata has been verified at the separate hosted gateway.
func RenderXweb(ctx context.Context, client *http.Client, o XwebOptions, kind, directory string, names []string, output io.Writer) error {
	if kind != "serve" && kind != "attestor" {
		return errors.New("invalid xweb renderer kind")
	}
	if len(names) == 0 || len(strings.Fields(o.Seeds)) == 0 {
		return errors.New("xweb serving names and seeds required")
	}
	seen := map[string]bool{}
	for _, name := range names {
		if !edgeDNS.MatchString(name) || seen[name] {
			return errors.New("invalid or duplicate xweb serving name")
		}
		seen[name] = true
	}
	prices, e := XwebPrices(ctx, client, o)
	if e != nil {
		return e
	}
	raw, e := callRPC(ctx, client, o.ReceiptNode, "px_getReceiptKeys", []any{}, "", false)
	if e != nil {
		return e
	}
	keys, e := XwebReceiptHistory(raw)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(directory, 0755); e != nil {
		return e
	}
	count := len(names)
	if kind == "attestor" {
		count = 4
	}
	for index := 0; index < count; index++ {
		name := ""
		listen := "127.0.0.1:8482"
		dataDir := "/var/lib/x-websearch"
		node := o.Node
		peers := []string{}
		if kind == "serve" {
			name = names[index]
			for _, peer := range names {
				if peer != name {
					peers = append(peers, "https://"+peer)
				}
			}
		} else {
			name = fmt.Sprintf("attestor-%d", index+1)
			listen = "[::]:8480"
			dataDir = "/data/state"
			node = "https://" + names[index%len(names)]
			for peer := 0; peer < 4; peer++ {
				if peer != index {
					peers = append(peers, fmt.Sprintf("http://127.0.0.1:%d", 8491+peer))
				}
			}
		}
		config := map[string]any{"listen": listen, "data_dir": dataDir, "seeds": strings.Fields(o.Seeds), "crawl": map[string]any{"pages_per_cycle": 1000, "pages_per_host": 100, "max_depth": 3, "politeness_delay_ms": 1000}, "fetch": map[string]any{"connect_timeout_ms": 3000, "total_timeout_ms": 10000, "max_body_bytes": 2097152, "max_redirects": 3, "allow_loopback": false}, "assets": prices, "gateway": map[string]any{"endpoint": o.Gateway, "receipt_keys": keys}, "evm": map[string]any{"endpoint": node, "chain_id": 125, "confirmations": 12}, "kernel_network_id": 125, "peers": peers}
		encoded, e := json.MarshalIndent(config, "", "  ")
		if e != nil {
			return e
		}
		if e = edgeAtomic(filepath.Join(directory, name+".json"), append(encoded, '\n'), 0644); e != nil {
			return e
		}
		if _, e = fmt.Fprintln(output, "rendered", name); e != nil {
			return e
		}
	}
	return nil
}
