package types

import "testing"

func TestRetirementStepNames(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  string
		want string
	}{
		{name: "MergeUpgrade", got: MergeUpgrade, want: "v7.0"},
		{name: "StoreDeleteUpgrade", got: StoreDeleteUpgrade, want: "v7.0-anchor-store-delete"},
		{name: "EventAnchorRetired", got: EventAnchorRetired, want: "layerx_anchor_retired"},
		{name: "AttributeReturned", got: AttributeReturned, want: "returned_bond"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
			}
		})
	}
}
