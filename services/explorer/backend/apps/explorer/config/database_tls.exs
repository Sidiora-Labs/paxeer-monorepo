import Config

ssl_opts = [
  verify: :verify_peer,
  cacertfile:
    System.get_env("DATABASE_SSL_CA_CERT_FILE", "/etc/ssl/certs/ca-certificates.crt")
    |> String.to_charlist(),
  customize_hostname_check: [match_fun: :public_key.pkix_verify_hostname_match_fun(:https)]
]

for repo <- [
      Explorer.Repo,
      # Configures API the database
      Explorer.Repo.Replica1,

      # Feature dependent repos
      Explorer.Repo.Account,
      Explorer.Repo.BridgedTokens,
      Explorer.Repo.ShrunkInternalTransactions,
      Explorer.Repo.EventNotifications,

      # Chain-type dependent repos
      Explorer.Repo.Arbitrum,
      Explorer.Repo.Beacon,
      Explorer.Repo.Blackfort,
      Explorer.Repo.Celo,
      Explorer.Repo.Filecoin,
      Explorer.Repo.Mud,
      Explorer.Repo.Optimism,
      Explorer.Repo.PolygonEdge,
      Explorer.Repo.PolygonZkevm,
      Explorer.Repo.RSK,
      Explorer.Repo.Scroll,
      Explorer.Repo.Shibarium,
      Explorer.Repo.Stability,
      Explorer.Repo.Suave,
      Explorer.Repo.Zilliqa,
      Explorer.Repo.ZkSync,
      Explorer.Repo.Neon
    ] do
  config :explorer, repo, ssl_opts: ssl_opts
end

