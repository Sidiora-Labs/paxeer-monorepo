package account

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
)

func TestAdaptV2WatchlistMatchesFrozenSchema(t *testing.T) {
	at := time.Date(2026, time.October, 26, 12, 34, 56, 789000000, time.UTC)
	id := uuid.MustParse("00000000-0000-4000-8000-000000000123")
	wallet := "0x0000000000000000000000000000000000000001"
	pools := []string{
		"0x0000000000000000000000000000000000000003",
		"0x0000000000000000000000000000000000000002",
	}
	entries := make([]WireWatchEntry, len(pools))
	for i, pool := range pools {
		entry, err := WatchProjection(wallet, WatchEntry{Pool: pool, AddedAt: at}, 7+i, i == 0)
		if err != nil {
			t.Fatal(err)
		}
		entries[i] = entry
	}
	input, err := Response(id, entries, "42", at)
	if err != nil {
		t.Fatal(err)
	}
	cursor := "opaque-watchlist-cursor"
	input.NextCursor = &cursor
	revision := v2.PositiveUInt256("42")
	wantItems := []v2.WatchlistEntry{
		{Wallet: wallet, PoolAddress: pools[0], AddedAt: "2026-10-26T12:34:56.789Z", Order: 7, AlertEnabled: true},
		{Wallet: wallet, PoolAddress: pools[1], AddedAt: "2026-10-26T12:34:56.789Z", Order: 8, AlertEnabled: false},
	}

	t.Run("page", func(t *testing.T) {
		raw, err := AdaptV2Watchlist(input)
		if err != nil {
			t.Fatal(err)
		}
		if err := v2.ValidateJSON("WatchlistGetResponse", raw); err != nil {
			t.Fatal(err)
		}
		got, err := v2.DecodeWatchlistGetResponse(raw)
		if err != nil {
			t.Fatal(err)
		}
		want := v2.WatchlistGetResponse{
			RequestId: id.String(), Data: v2.WatchlistGetData{Items: wantItems},
			Revision: &revision, Quality: "complete", NextCursor: &cursor,
			GeneratedAt: "2026-10-26T12:34:56.789Z",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("watchlist = %#v; want %#v", got, want)
		}
	})

	t.Run("put", func(t *testing.T) {
		put, err := Response(id, entries[0], "42", at)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := AdaptV2WatchPut(put)
		if err != nil {
			t.Fatal(err)
		}
		if err := v2.ValidateJSON("WatchlistPutResponse", raw); err != nil {
			t.Fatal(err)
		}
		got, err := v2.DecodeWatchlistPutResponse(raw)
		if err != nil {
			t.Fatal(err)
		}
		want := v2.WatchlistPutResponse{
			RequestId: id.String(), Data: v2.WatchlistPutData{Entry: wantItems[0]},
			Revision: &revision, Quality: "complete", GeneratedAt: "2026-10-26T12:34:56.789Z",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("watch put = %#v; want %#v", got, want)
		}
	})

	t.Run("empty", func(t *testing.T) {
		empty, err := Response(id, []WireWatchEntry(nil), "0", at)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := AdaptV2Watchlist(empty)
		if err != nil {
			t.Fatal(err)
		}
		if err := v2.ValidateJSON("WatchlistGetResponse", raw); err != nil {
			t.Fatal(err)
		}
		got, err := v2.DecodeWatchlistGetResponse(raw)
		if err != nil {
			t.Fatal(err)
		}
		want := v2.WatchlistGetResponse{
			RequestId: id.String(), Data: v2.WatchlistGetData{Items: []v2.WatchlistEntry{}},
			Quality: "complete", GeneratedAt: "2026-10-26T12:34:56.789Z",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("empty watchlist = %#v; want %#v", got, want)
		}
	})

	t.Run("unrepresentable", func(t *testing.T) {
		invalid := input
		invalid.Data = append([]WireWatchEntry(nil), entries...)
		invalid.Data[0].PoolAddress = "invalid"
		raw, err := AdaptV2Watchlist(invalid)
		assertV2Unrepresentable(t, "WatchlistGetResponse", raw, err)
		put, err := Response(id, invalid.Data[0], "42", at)
		if err != nil {
			t.Fatal(err)
		}
		raw, err = AdaptV2WatchPut(put)
		assertV2Unrepresentable(t, "WatchlistPutResponse", raw, err)
		invalid = input
		invalid.Watermark = make(chan int)
		raw, err = AdaptV2Watchlist(invalid)
		assertV2Unrepresentable(t, "WatchlistGetResponse", raw, err)
	})
}

func TestAdaptV2ProfilePatchMatchesFrozenSchema(t *testing.T) {
	at := time.Date(2026, time.October, 26, 12, 34, 56, 789000000, time.UTC)
	id := uuid.MustParse("00000000-0000-4000-8000-000000000123")
	avatar := "00000000-0000-4000-8000-000000000456"
	banner := "00000000-0000-4000-8000-000000000789"
	website := "https://example.com/profile"
	profile := PublicProfile{
		Wallet:      "0x0000000000000000000000000000000000000001",
		DisplayName: "Account name", Bio: "Plain text biography",
		AvatarAssetID: &avatar, BannerAssetID: &banner, Website: &website, Revision: "42",
	}
	for _, nullable := range []bool{false, true} {
		name := "populated"
		if nullable {
			name = "nullable"
		}
		t.Run(name, func(t *testing.T) {
			value := profile
			var revision *v2.PositiveUInt256
			if nullable {
				value.AvatarAssetID, value.BannerAssetID, value.Website = nil, nil, nil
				value.Revision = "0"
			} else {
				r := v2.PositiveUInt256("42")
				revision = &r
			}
			input, err := Response(id, value, value.Revision, at)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := AdaptV2ProfilePatch(input)
			if err != nil {
				t.Fatal(err)
			}
			if err := v2.ValidateJSON("ProfilePatchResponse", raw); err != nil {
				t.Fatal(err)
			}
			got, err := v2.DecodeProfilePatchResponse(raw)
			if err != nil {
				t.Fatal(err)
			}
			want := v2.ProfilePatchResponse{
				RequestId: id.String(), Revision: revision, Quality: "complete",
				GeneratedAt: "2026-10-26T12:34:56.789Z",
				Data: v2.ProfilePatchData{Profile: v2.Profile{
					Wallet: value.Wallet, DisplayName: value.DisplayName, Bio: value.Bio,
					AvatarAssetId: value.AvatarAssetID, BannerAssetId: value.BannerAssetID,
					Website: value.Website, Revision: v2.UInt256(value.Revision),
				}},
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("profile patch = %#v; want %#v", got, want)
			}
		})
	}
	t.Run("unrepresentable", func(t *testing.T) {
		profile.Wallet = "invalid"
		input, err := Response(id, profile, profile.Revision, at)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := AdaptV2ProfilePatch(input)
		assertV2Unrepresentable(t, "ProfilePatchResponse", raw, err)
	})
}

func assertV2Unrepresentable(t *testing.T, schema string, raw []byte, err error) {
	t.Helper()
	if raw != nil || !errors.Is(err, ErrUnrepresentable) || !strings.Contains(err.Error(), schema) {
		t.Fatalf("%s: bytes = %s, error = %v; want nil bytes and schema-wrapped ErrUnrepresentable", schema, raw, err)
	}
}
