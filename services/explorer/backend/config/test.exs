import Config

config :explorer, Explorer.Chain.PaxeerX.PriceFeed, update_count_24h_cache_period: 0

# Print only warnings and errors during test
config :logger, level: :warn

config :logger, :error, path: Path.absname("logs/test/error.log")
