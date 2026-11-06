defmodule BlockScoutWeb.Repair.Repair22359Test do
  use BlockScoutWeb.ConnCase, async: false

  alias Explorer.Chain.Cache.Counters.{AddressesCount, AverageBlockTime}

  setup do
    configuration = Application.fetch_env!(:explorer, AverageBlockTime)
    Application.put_env(:explorer, AverageBlockTime, Keyword.put(configuration, :enabled, true))

    on_exit(fn -> Application.put_env(:explorer, AverageBlockTime, configuration) end)

    start_supervised!(AddressesCount)
    start_supervised!(AverageBlockTime)

    :ok
  end

  test "stats and chain page display the latest consensus block number", %{conn: conn} do
    insert(:block, number: 10, consensus: true)
    insert(:block, number: 11, consensus: true)
    insert(:block, number: 500, consensus: true)
    insert(:block, number: 600, consensus: false)

    assert json_response(get(conn, "/api/v2/stats"), 200)["total_blocks"] == "500"

    chain_response =
      conn
      |> init_test_session(%{})
      |> put_private(:phoenix_endpoint, BlockScoutWeb.Endpoint)
      |> Map.put(:secret_key_base, BlockScoutWeb.Endpoint.config(:secret_key_base))
      |> Phoenix.ConnTest.bypass_through(BlockScoutWeb.Routers.WebRouter, [:browser])
      |> Phoenix.ConnTest.dispatch(BlockScoutWeb.Routers.WebRouter, :get, "/")
      |> BlockScoutWeb.ChainController.call(BlockScoutWeb.ChainController.init(:show))

    assert chain_response.assigns.block_count == 500

    assert chain_response
           |> html_response(200)
           |> Floki.parse_document!()
           |> Floki.find("[data-selector='block-count']")
           |> Floki.text()
           |> String.trim() == "500"
  end

  test "stats and chain page display zero without consensus blocks", %{conn: conn} do
    insert(:block, number: 600, consensus: false)

    assert json_response(get(conn, "/api/v2/stats"), 200)["total_blocks"] == "0"

    chain_response =
      conn
      |> init_test_session(%{})
      |> put_private(:phoenix_endpoint, BlockScoutWeb.Endpoint)
      |> Map.put(:secret_key_base, BlockScoutWeb.Endpoint.config(:secret_key_base))
      |> Phoenix.ConnTest.bypass_through(BlockScoutWeb.Routers.WebRouter, [:browser])
      |> Phoenix.ConnTest.dispatch(BlockScoutWeb.Routers.WebRouter, :get, "/")
      |> BlockScoutWeb.ChainController.call(BlockScoutWeb.ChainController.init(:show))

    assert chain_response.assigns.block_count == 0

    assert chain_response
           |> html_response(200)
           |> Floki.parse_document!()
           |> Floki.find("[data-selector='block-count']")
           |> Floki.text()
           |> String.trim() == "0"
  end
end
