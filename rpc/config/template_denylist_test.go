package config_test

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/BurntSushi/toml"
	"github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestConfigTemplateRendersDenyListArray(t *testing.T) {
	for _, tc := range []struct {
		name     string
		denyList []string
		wantLine string
	}{
		{
			name:     "populated",
			denyList: []string{"debug_traceBlockByNumber", "eth_sign"},
			wantLine: `deny_list = ["debug_traceBlockByNumber", "eth_sign"]`,
		},
		{
			name:     "empty",
			denyList: []string{},
			wantLine: "deny_list = []",
		},
		{
			name:     "nil",
			wantLine: "deny_list = []",
		},
		{
			name:     "escaped strings",
			denyList: []string{`method"quote`, `method\slash`},
			wantLine: `deny_list = ["method\"quote", "method\\slash"]`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.DefaultConfig
			cfg.DenyList = tc.denyList
			tmpl, err := template.New("evm").Parse(config.ConfigTemplate)
			require.NoError(t, err)
			var rendered bytes.Buffer
			require.NoError(t, tmpl.Execute(&rendered, struct{ EVM config.Config }{EVM: cfg}))
			require.Contains(t, rendered.String(), "\n"+tc.wantLine+"\n")

			var decoded struct {
				EVM struct {
					DenyList []string `toml:"deny_list"`
				} `toml:"evm"`
			}
			_, err = toml.Decode(rendered.String(), &decoded)
			require.NoError(t, err)
			want := tc.denyList
			if want == nil {
				want = []string{}
			}
			require.Equal(t, want, decoded.EVM.DenyList)

			options := viper.New()
			options.SetConfigType("toml")
			require.NoError(t, options.ReadConfig(bytes.NewReader(rendered.Bytes())))
			roundTrip, err := config.ReadConfig(options)
			require.NoError(t, err)
			require.Equal(t, want, roundTrip.DenyList)
		})
	}
}
