package discovery

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
)

func TestAdaptV2AlertCreateMatchesFrozenSchema(t *testing.T) {
	const wallet = "0x1234567890abcdef1234567890abcdef12345678"
	const requestID = "c9e42506-f847-4ec1-9c54-e320c98c1884"
	const owner = "verified-account"
	now := time.Date(2026, 10, 26, 12, 34, 56, 789000000, time.UTC)
	chainID, assetAddress, currency, provider := "125", wallet, "USD", "market-provider"
	decimals := v2.SafeInteger(18)
	threshold := v2.InputPrice{
		Numerator: "900719925474099312345678901234567890", Denominator: "1000000000000000001",
		BaseAsset:  v2.InputAssetRef{Kind: "erc20", ChainId: &chainID, Address: &assetAddress, Decimals: &decimals},
		QuoteAsset: v2.InputAssetRef{Kind: "fiat_display", CurrencyCode: &currency},
		Source:     v2.InputPriceSource{Kind: "external_display", AmountBasis: "provider_display", ProviderId: &provider, ObservedAt: now.Format("2006-01-02T15:04:05.000Z")},
	}
	body := v2.AlertCreateBody{PoolAddress: wallet, Direction: "crosses_above", Threshold: threshold, CooldownSeconds: 300}
	marshal := func(v any) []byte {
		t.Helper()
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	ids := map[string]bool{}
	for _, direction := range []string{"crosses_above", "crosses_below"} {
		for _, cooldown := range []v2.SafeInteger{60, 300, 86400} {
			t.Run(fmt.Sprintf("%s/%d", direction, cooldown), func(t *testing.T) {
				input := body
				input.Direction, input.CooldownSeconds = direction, cooldown
				raw := marshal(input)
				if err := v2.ValidateJSON("AlertCreateBody", raw); err != nil {
					t.Fatalf("invalid frozen input: %v", err)
				}
				rule, err := RuleFromV2AlertCreate(raw, owner)
				if err != nil {
					t.Fatal(err)
				}
				if err := rule.Validate(); err != nil {
					t.Fatalf("invalid domain rule: %v", err)
				}
				id, err := uuid.Parse(rule.ID)
				if err != nil || id.Version() != 4 || id.Variant() != uuid.RFC4122 || ids[rule.ID] {
					t.Fatalf("expected fresh RFC 4122 UUID v4, got %q (%v)", rule.ID, err)
				}
				ids[rule.ID] = true
				if rule.OwnerAccountID != owner || rule.PoolAddress != input.PoolAddress || rule.Direction != direction || rule.Cooldown != time.Duration(cooldown)*time.Second || rule.Threshold != string(threshold.Numerator)+"/"+string(threshold.Denominator) {
					t.Fatalf("rule did not preserve input: %+v", rule)
				}
				if rule.Revision != 1 || rule.Disabled || rule.Metric != "price" || rule.Hysteresis != "0" || !reflect.DeepEqual(rule.Channels, []string{"inapp"}) {
					t.Fatalf("incorrect rule defaults: %+v", rule)
				}
				for _, disabled := range []bool{false, true} {
					rule.Disabled = disabled
					response, err := AdaptV2AlertCreate(rule, wallet, input.Threshold, requestID, now)
					if err != nil {
						t.Fatal(err)
					}
					encoded := marshal(response)
					if err := v2.ValidateJSON("AlertCreateResponse", encoded); err != nil {
						t.Fatalf("invalid frozen response: %v: %s", err, encoded)
					}
					var decoded v2.AlertCreateResponse
					if err := json.Unmarshal(encoded, &decoded); err != nil {
						t.Fatal(err)
					}
					alert := decoded.Data.Alert
					state := "enabled"
					if disabled {
						state = "disabled"
					}
					if alert.AlertId != rule.ID || alert.Wallet != wallet || alert.PoolAddress != input.PoolAddress || alert.Direction != direction || alert.CooldownSeconds != cooldown || alert.State != state {
						t.Fatalf("incorrect frozen alert: %+v", alert)
					}
					if string(marshal(alert.Threshold)) != string(marshal(input.Threshold)) {
						t.Fatalf("threshold lost precision or context: %+v", alert.Threshold)
					}
					if decoded.RequestId != requestID || decoded.GeneratedAt != now.Format("2006-01-02T15:04:05.000Z") || decoded.Quality != "complete" || decoded.Revision == nil || *decoded.Revision != "1" || decoded.Watermark != nil || decoded.NextCursor != nil {
						t.Fatalf("incorrect envelope: %+v", decoded)
					}
					var fields map[string]json.RawMessage
					if err := json.Unmarshal(encoded, &fields); err != nil {
						t.Fatal(err)
					}
					for _, field := range []string{"watermark", "nextCursor"} {
						if string(fields[field]) != "null" {
							t.Errorf("%s must be explicit null: %s", field, fields[field])
						}
					}
				}
			})
		}
	}

	t.Run("reject invalid bodies", func(t *testing.T) {
		valid := string(marshal(body))
		cases := map[string]string{
			"empty": "", "null": "null", "array": "[]", "missing fields": "{}",
			"trailing value":         valid + " {}",
			"unknown field":          strings.TrimSuffix(valid, "}") + `,"wallet":"` + wallet + `"}`,
			"unknown nested field":   strings.Replace(valid, `"numerator":`, `"extra":true,"numerator":`, 1),
			"missing nullable field": strings.Replace(valid, `"sourceEventId":null,`, "", 1),
			"wrong case":             strings.Replace(valid, `"poolAddress"`, `"PoolAddress"`, 1),
			"bad pool":               strings.Replace(valid, wallet, "not-an-address", 1),
			"bad direction":          strings.Replace(valid, "crosses_above", "above", 1),
			"missing cooldown":       strings.Replace(valid, `"cooldownSeconds":300,`, "", 1),
			"null cooldown":          strings.Replace(valid, `"cooldownSeconds":300`, `"cooldownSeconds":null`, 1),
			"short cooldown":         strings.Replace(valid, `"cooldownSeconds":300`, `"cooldownSeconds":59`, 1),
			"long cooldown":          strings.Replace(valid, `"cooldownSeconds":300`, `"cooldownSeconds":86401`, 1),
			"fractional cooldown":    strings.Replace(valid, `"cooldownSeconds":300`, `"cooldownSeconds":60.5`, 1),
			"overflow cooldown":      strings.Replace(valid, `"cooldownSeconds":300`, `"cooldownSeconds":9223372036854775807`, 1),
			"zero denominator":       strings.Replace(valid, `"denominator":"1000000000000000001"`, `"denominator":"0"`, 1),
			"negative numerator":     strings.Replace(valid, `"numerator":"900719925474099312345678901234567890"`, `"numerator":"-1"`, 1),
		}
		for name, raw := range cases {
			t.Run(name, func(t *testing.T) {
				if _, err := RuleFromV2AlertCreate([]byte(raw), owner); !errors.Is(err, ErrInvalid) {
					t.Fatalf("expected ErrInvalid, got %v for %s", err, raw)
				}
			})
		}
		if _, err := RuleFromV2AlertCreate(marshal(body), ""); !errors.Is(err, ErrInvalid) {
			t.Fatalf("empty owner accepted: %v", err)
		}
	})

	t.Run("reject invalid output", func(t *testing.T) {
		rule, err := RuleFromV2AlertCreate(marshal(body), owner)
		if err != nil {
			t.Fatal(err)
		}
		for name, mutate := range map[string]func(*AlertRule){
			"invalid rule":        func(r *AlertRule) { r.Revision = 0 },
			"invalid UUID":        func(r *AlertRule) { r.ID = "rule" },
			"fractional second":   func(r *AlertRule) { r.Cooldown += time.Nanosecond },
			"different threshold": func(r *AlertRule) { r.Threshold = "1" },
			"different metric":    func(r *AlertRule) { r.Metric = "volume" },
		} {
			t.Run(name, func(t *testing.T) {
				invalid := rule
				mutate(&invalid)
				if _, err := AdaptV2AlertCreate(invalid, wallet, threshold, requestID, now); !errors.Is(err, ErrInvalid) {
					t.Fatalf("expected ErrInvalid, got %v", err)
				}
			})
		}
		if _, err := AdaptV2AlertCreate(rule, "not-a-wallet", threshold, requestID, now); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid wallet accepted: %v", err)
		}
		if _, err := AdaptV2AlertCreate(rule, wallet, threshold, "not-a-UUID", now); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid request ID accepted: %v", err)
		}
		invalid := threshold
		invalid.Denominator = "0"
		if _, err := AdaptV2AlertCreate(rule, wallet, invalid, requestID, now); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid price accepted: %v", err)
		}
	})

	t.Run("zero threshold", func(t *testing.T) {
		input := body
		input.Threshold.Numerator = "0"
		rule, err := RuleFromV2AlertCreate(marshal(input), owner)
		if err != nil {
			t.Fatal(err)
		}
		response, err := AdaptV2AlertCreate(rule, wallet, input.Threshold, requestID, now)
		if err != nil || response.Data.Alert.Threshold.Numerator != "0" {
			t.Fatalf("zero threshold did not round trip: %+v, %v", response, err)
		}
	})
}
