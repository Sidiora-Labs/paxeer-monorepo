package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/BurntSushi/toml"
	tmcli "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/spf13/viper"
)

const defaultConfigTemplate = `# This is a TOML config file.
# For more information, see https://github.com/toml-lang/toml

###############################################################################
###                           Client Configuration                            ###
###############################################################################

`

func SetClientConfig(key string, value string, configPath string, config *ClientConfig) error {
	switch key {
	case flags.FlagChainID:
		config.SetChainID(value)
	case flags.FlagKeyringBackend:
		config.SetKeyringBackend(value)
	case tmcli.OutputFlag:
		config.SetOutput(value)
	case flags.FlagNode:
		config.SetNode(value)
	case flags.FlagBroadcastMode:
		config.SetBroadcastMode(value)
	default:
		return errUnknownConfigKey(key)
	}

	confFile := filepath.Join(configPath, "client.toml")
	if err := writeConfigToFile(confFile, config); err != nil {
		return fmt.Errorf("could not write client config to the file: %v", err)
	}
	return nil
}

// writeConfigToFile encodes and validates config before atomically replacing configFilePath.
func writeConfigToFile(configFilePath string, config *ClientConfig) error {
	if config == nil {
		return fmt.Errorf("client config is nil")
	}

	var buffer bytes.Buffer
	buffer.WriteString(defaultConfigTemplate)
	encoder := toml.NewEncoder(&buffer)
	fields := []struct {
		key, value, comment string
	}{
		{flags.FlagChainID, config.ChainID, "# The network chain ID"},
		{flags.FlagKeyringBackend, config.KeyringBackend, "# The keyring's backend, where the keys are stored (os|file|kwallet|pass|test|memory)"},
		{tmcli.OutputFlag, config.Output, "# CLI output format (text|json)"},
		{flags.FlagNode, config.Node, "# <host>:<port> to Tendermint RPC interface for this chain"},
		{flags.FlagBroadcastMode, config.BroadcastMode, "# Transaction broadcasting mode (sync|async|block)"},
	}
	for _, field := range fields {
		if !utf8.ValidString(field.value) {
			return fmt.Errorf("client config %s is not valid UTF-8", field.key)
		}
		buffer.WriteString(field.comment + "\n")
		if err := encoder.Encode(map[string]string{field.key: field.value}); err != nil {
			return fmt.Errorf("could not encode client config: %w", err)
		}
	}

	v := viper.New()
	v.SetConfigType("toml")
	if err := v.ReadConfig(bytes.NewReader(buffer.Bytes())); err != nil {
		return fmt.Errorf("could not validate client config: %w", err)
	}
	var decoded ClientConfig
	if err := v.Unmarshal(&decoded); err != nil {
		return fmt.Errorf("could not decode client config: %w", err)
	}
	if decoded != *config {
		return fmt.Errorf("client config does not round-trip through TOML")
	}

	directory, err := os.Open(filepath.Dir(configFilePath))
	if err != nil {
		return fmt.Errorf("could not open client config directory: %w", err)
	}
	defer directory.Close()

	temporary, err := os.CreateTemp(filepath.Dir(configFilePath), ".client.toml-*")
	if err != nil {
		return fmt.Errorf("could not create temporary client config: %w", err)
	}
	defer os.Remove(temporary.Name())
	defer temporary.Close()

	if err := temporary.Chmod(0600); err != nil {
		return fmt.Errorf("could not secure temporary client config: %w", err)
	}
	if _, err := temporary.Write(buffer.Bytes()); err != nil {
		return fmt.Errorf("could not write temporary client config: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("could not sync temporary client config: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("could not close temporary client config: %w", err)
	}
	if err := os.Rename(temporary.Name(), configFilePath); err != nil {
		return fmt.Errorf("could not replace client config: %w", err)
	}
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("could not sync client config directory: %w", err)
	}

	return nil
}

// ensureConfigPath creates a directory configPath if it does not exist
func ensureConfigPath(configPath string) error {
	return os.MkdirAll(configPath, 0750)
}

// getClientConfig reads values from client.toml file and unmarshalls them into ClientConfig
func GetClientConfig(configPath string, v *viper.Viper) (*ClientConfig, error) {
	v.AddConfigPath(configPath)
	v.SetConfigName("client")
	v.SetConfigType("toml")

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	conf := new(ClientConfig)
	if err := v.Unmarshal(conf); err != nil {
		return nil, err
	}

	return conf, nil
}
