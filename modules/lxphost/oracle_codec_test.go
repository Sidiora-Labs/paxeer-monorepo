//go:build cgo

package lxphost

import "testing"

func TestOracleCodecObjectDefinesEncoders(t *testing.T) {
	members := kernelArchiveMembers(t)
	codec, ok := members["lx_oracle_codec.o"]
	if !ok {
		t.Fatal("kernel archive does not contain lx_oracle_codec.o")
	}
	adapter, ok := members["lx_oracle_adapter.o"]
	if !ok {
		t.Fatal("kernel archive does not contain lx_oracle_adapter.o")
	}
	for _, symbol := range []string{"lx_oracle_observation_encode", "lx_oracle_observation_sign"} {
		if !codec[symbol] {
			t.Errorf("kernel archive member lx_oracle_codec.o does not define %s", symbol)
		}
		if adapter[symbol] {
			t.Errorf("kernel archive member lx_oracle_adapter.o still defines %s", symbol)
		}
	}
}
