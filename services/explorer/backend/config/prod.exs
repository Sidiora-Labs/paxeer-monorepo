import Config

# This is build-time configuration written into the release's sys.config.
# Explorer.Repo ssl_opts belong exclusively to apps/explorer/config/runtime/prod.exs.

# Do not print debug messages in production
config :logger, level: :info

config :logger, :error,
  level: :error,
  path: Path.absname("logs/prod/error.log"),
  rotate: %{max_bytes: 52_428_800, keep: 19}

config :logger, :account,
  path: Path.absname("logs/prod/account.log"),
  rotate: %{max_bytes: 52_428_800, keep: 19},
  metadata_filter: [fetcher: :account]
