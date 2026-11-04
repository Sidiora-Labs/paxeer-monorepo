package spot_test

import "testing"

func TestRepairM_708_ShareGoldenSnapshot(t *testing.T) {
	requireShareSnapshot(t)
}

func TestRepairM_708_ShareSupportedSelectors(t *testing.T) {
	requireNativeShareSupported(t)
}

func TestRepairM_708_ShareUnsupportedSelectors(t *testing.T) {
	requireNativeShareUnsupported(t)
}

func TestRepairM_708_ShareRevertBytes(t *testing.T) {
	requireNativeShareReverts(t)
}
