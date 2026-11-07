package types

import "testing"

func TestErrModuleRetiredCode19(t *testing.T) {
	if got := ErrModuleRetired.Codespace(); got != "layerxanchor" {
		t.Errorf("ErrModuleRetired codespace = %q, want %q", got, "layerxanchor")
	}
	if got := ErrModuleRetired.ABCICode(); got != 19 {
		t.Errorf("ErrModuleRetired ABCI code = %d, want 19", got)
	}
	if got := ErrModuleRetired.Error(); got != "LXP_MODULE_DISABLED: layerxanchor retired" {
		t.Errorf("ErrModuleRetired text = %q, want %q", got, "LXP_MODULE_DISABLED: layerxanchor retired")
	}
}
