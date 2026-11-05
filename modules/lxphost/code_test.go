//go:build cgo

package lxphost

import "testing"

func TestCodeNamesMatchKernel(t *testing.T) {
	for value := int32(-2000); value <= 2000; value++ {
		if got, want := Code(value).Error(), kernelResultName(value); got != want {
			t.Errorf("Code(%d).Error() = %q, kernel = %q", value, got, want)
		}
	}
}
