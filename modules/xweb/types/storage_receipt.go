package types

import (
	"net/url"
	"strings"
	"unicode/utf8"

	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

const maxStorageReceiptURLBytes = 512

var ErrInvalidPayload = sdkerrors.Register(ModuleName, 26, "invalid storage receipt payload")

type StorageReceiptPayload struct {
	ObjectDigest [32]byte
	BaseURL      string
}

func ValidateStorageReceiptPayload(payload StorageReceiptPayload) error {
	if payload.ObjectDigest == ([32]byte{}) {
		return ErrInvalidPayload.Wrap("zero object digest")
	}
	if len(payload.BaseURL) == 0 || len(payload.BaseURL) > maxStorageReceiptURLBytes {
		return ErrInvalidPayload.Wrapf("base URL is %d bytes, want 1 to %d", len(payload.BaseURL), maxStorageReceiptURLBytes)
	}
	if !utf8.ValidString(payload.BaseURL) {
		return ErrInvalidPayload.Wrap("base URL is not UTF-8")
	}
	base, err := url.Parse(payload.BaseURL)
	if err != nil {
		return ErrInvalidPayload.Wrapf("base URL: %v", err)
	}
	if base.Scheme != "https" || base.Hostname() == "" {
		return ErrInvalidPayload.Wrap("base URL must have an https scheme and a host")
	}
	if base.User != nil || strings.ContainsAny(payload.BaseURL, "?#") {
		return ErrInvalidPayload.Wrap("base URL carries userinfo, a query or a fragment")
	}
	if strings.HasSuffix(payload.BaseURL, "/") {
		return ErrInvalidPayload.Wrap("base URL has a trailing slash")
	}
	return nil
}

func EncodeStorageReceiptPayload(payload StorageReceiptPayload) ([]byte, error) {
	if err := ValidateStorageReceiptPayload(payload); err != nil {
		return nil, err
	}
	raw := make([]byte, len(payload.ObjectDigest)+len(payload.BaseURL))
	copy(raw, payload.ObjectDigest[:])
	copy(raw[len(payload.ObjectDigest):], payload.BaseURL)
	return raw, nil
}

func DecodeStorageReceiptPayload(raw []byte) (StorageReceiptPayload, error) {
	var payload StorageReceiptPayload
	if len(raw) < len(payload.ObjectDigest) {
		return StorageReceiptPayload{}, ErrInvalidPayload.Wrapf("payload is %d bytes, shorter than the object digest", len(raw))
	}
	copy(payload.ObjectDigest[:], raw[:len(payload.ObjectDigest)])
	payload.BaseURL = string(raw[len(payload.ObjectDigest):])
	if err := ValidateStorageReceiptPayload(payload); err != nil {
		return StorageReceiptPayload{}, err
	}
	return payload, nil
}
