package account

import (
	"encoding/json"
	"errors"
	"fmt"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

var ErrUnrepresentable = errors.New("UNREPRESENTABLE")

func AdaptV2Watchlist(input Envelope[[]WireWatchEntry]) ([]byte, error) {
	items := make([]v2.WatchlistEntry, len(input.Data))
	for i, entry := range input.Data {
		items[i] = v2WatchEntry(entry)
	}
	return marshalV2Envelope("WatchlistGetResponse", input, v2.WatchlistGetData{Items: items})
}

func AdaptV2WatchPut(input Envelope[WireWatchEntry]) ([]byte, error) {
	return marshalV2Envelope("WatchlistPutResponse", input, v2.WatchlistPutData{Entry: v2WatchEntry(input.Data)})
}

func AdaptV2ProfilePatch(input Envelope[PublicProfile]) ([]byte, error) {
	profile := input.Data
	return marshalV2Envelope("ProfilePatchResponse", input, v2.ProfilePatchData{Profile: v2.Profile{
		Wallet:        profile.Wallet,
		DisplayName:   profile.DisplayName,
		Bio:           profile.Bio,
		AvatarAssetId: profile.AvatarAssetID,
		BannerAssetId: profile.BannerAssetID,
		Website:       profile.Website,
		Revision:      v2.UInt256(profile.Revision),
	}})
}

func v2WatchEntry(entry WireWatchEntry) v2.WatchlistEntry {
	return v2.WatchlistEntry{
		Wallet:       entry.Wallet,
		PoolAddress:  entry.PoolAddress,
		AddedAt:      entry.AddedAt,
		Order:        v2.SafeInteger(entry.Order),
		AlertEnabled: entry.AlertEnabled,
	}
}

func marshalV2Envelope[T, U any](schema string, input Envelope[T], data U) ([]byte, error) {
	body, err := json.Marshal(Envelope[U]{
		RequestID:   input.RequestID,
		Data:        data,
		Watermark:   input.Watermark,
		Revision:    input.Revision,
		Quality:     input.Quality,
		NextCursor:  input.NextCursor,
		GeneratedAt: input.GeneratedAt,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %w", schema, ErrUnrepresentable, err)
	}
	if err := v2.ValidateJSON(schema, body); err != nil {
		return nil, fmt.Errorf("%s: %w: %w", schema, ErrUnrepresentable, err)
	}
	return body, nil
}
