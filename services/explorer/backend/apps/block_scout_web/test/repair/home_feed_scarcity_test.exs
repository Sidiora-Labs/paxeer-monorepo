Code.require_file("../block_scout_web/controllers/api/v2/home_feed_scarcity.exs", __DIR__)

defmodule BlockScoutWeb.HomeFeedScarcityTest do
  @moduledoc "Home feed scarcity."

  use BlockScoutWeb.ConnCase

  for evm_count <- 0..2 do
    test "home-feed-scarcity-coverage: #{evm_count} EVM transactions", %{conn: conn} do
      BlockScoutWeb.API.V2.HomeFeedScarcity.assert_page(conn, unquote(evm_count))
    end
  end
end
