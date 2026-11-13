package spot_test

import "testing"

func TestShareGoldenSnapshot(t *testing.T) {
	requireShareSnapshot(t)
}

func TestShareSupportedSelectors(t *testing.T) {
	requireNativeShareSupported(t)
}

func TestShareUnsupportedSelectors(t *testing.T) {
	requireNativeShareUnsupported(t)
}

func TestShareRevertBytes(t *testing.T) {
	requireNativeShareReverts(t)
}
