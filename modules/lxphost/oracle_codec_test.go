//go:build cgo

package lxphost

import "testing"

func TestOracleCodecObjectDefinesEncoders(t *testing.T) {
	members := kernelArchiveMembers(t)
	codec, ok := members["lx_oracle_codec.o"]
	if !ok {
		t.Fatal("kernel archive does not contain lx_oracle_codec.o")
	}
	if _, ok := members["lx_oracle_adapter.o"]; ok {
		t.Error("kernel archive still contains lx_oracle_adapter.o")
	}
	for _, symbol := range []string{"lx_oracle_observation_encode", "lx_oracle_observation_sign"} {
		if !codec[symbol] {
			t.Errorf("kernel archive member lx_oracle_codec.o does not define %s", symbol)
		}
		for name, definitions := range members {
			if name != "lx_oracle_codec.o" && definitions[symbol] {
				t.Errorf("kernel archive member %s also defines %s", name, symbol)
			}
		}
	}
}
