package main

import noderuntime "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/test/e2e/node/runtime"

// Config is the application configuration.
type Config = noderuntime.Config

// LoadConfig loads the configuration from disk.
func LoadConfig(file string) (*Config, error) {
	return noderuntime.LoadConfig(file)
}
