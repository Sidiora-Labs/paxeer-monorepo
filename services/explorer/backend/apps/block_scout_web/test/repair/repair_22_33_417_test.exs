Code.require_file("../block_scout_web/controllers/api/v2/home_feed_scarcity.exs", __DIR__)

defmodule BlockScoutWeb.Repair.Repair2233417Test do
  @moduledoc """
  Repair record for audit 22.30.144, finding home-feed-scarcity-coverage.
  Proving cases: "home-feed-scarcity-coverage: 0 EVM transactions",
  "home-feed-scarcity-coverage: 1 EVM transactions" and
  "home-feed-scarcity-coverage: 2 EVM transactions".
  The main-page suite runs the same assertions and retains its three-EVM
  reservation assertion unchanged. The committed price-feed block transaction
  and execution-result recordings pass through CosmosRest's real decoders and
  PriceFeed persistence before requests traverse the production route and view.
  To exercise scarcity, eight local submissions reuse the recorded payload with
  deterministic distinct hashes and positions across four factory blocks; EVM
  factory transactions tie submission positions in older and newer blocks.
  These topology variants do not claim to be additional recorded chain blocks.
  No out-of-path wiring is required. Baseline verification was not run because
  the ordered bounded gates require implementation before build and hf-done
  and prohibit a separate verification run; no baseline failures are claimed.
  The runner's private evidence record names the verified candidate revision.
  """

  use BlockScoutWeb.ConnCase

  for evm_count <- 0..2 do
    test "home-feed-scarcity-coverage: #{evm_count} EVM transactions", %{conn: conn} do
      BlockScoutWeb.API.V2.HomeFeedScarcity.assert_page(conn, unquote(evm_count))
    end
  end
end
