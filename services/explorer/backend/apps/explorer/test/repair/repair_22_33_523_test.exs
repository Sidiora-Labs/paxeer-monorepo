# The repaired suites retain their original malformed, identity, consensus and
# boundary vectors. Requiring them makes those checks part of this focused gate.
for file <-
      ~w(account_binding_test.exs anchor_test.exs receipt_test.exs capabilities_test.exs capabilities_boot_test.exs market_event_test.exs) do
  Code.require_file(Path.expand("../explorer/chain/paxeer_x/#{file}", __DIR__))
end

Code.require_file(Path.expand("../explorer/chain/paxeer_x/repair_gateway.exs", __DIR__))
Code.require_file(Path.expand("../explorer/chain/paxeer_x/repair_price_feed.exs", __DIR__))

defmodule Explorer.Repair.Repair2233523Test do
  use Explorer.DataCase, async: false

  test "price_feed_recovery_concurrency uses independent PostgreSQL sessions" do
    Explorer.Chain.PaxeerX.Repair2233523.PriceFeedCases.price_feed_concurrent_compaction()
  end

  test "price_feed_recovery_concurrency reconstructs the production fetcher" do
    Explorer.Chain.PaxeerX.Repair2233523.PriceFeedCases.price_feed_restarted_fetcher()
  end

  test "receipt_gateway_transport_proof reads gateway envelopes and recovers key discovery" do
    Explorer.Chain.PaxeerX.Repair2233523.Gateway.receipt_gateway_transport_proof()
  end
end
