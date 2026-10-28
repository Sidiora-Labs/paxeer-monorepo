defmodule BlockScoutWeb.API.V2.PaxeerX.CapabilitiesControllerTest do
  use BlockScoutWeb.ConnCase, async: false

  import Mox

  alias Explorer.Chain.PaxeerX.Capabilities

  @head "0x16c1320"

  # `EthereumJSONRPC.integer_to_quantity/1` spells the height back in uppercase
  # hexadecimal, which is what the probe requests carry.
  @head_quantity "0x16C1320"

  @code_addresses [
    "0x0000000000000000000000000000000000001015",
    "0x0000000000000000000000000000000000001016",
    "0x0000000000000000000000000000000000001017"
  ]

  @addr_precompile "0x0000000000000000000000000000000000001004"
  @view_calls %{
    "0x0000000000000000000000000000000000001015" => "0xb71d031c" <> String.duplicate("0", 64),
    "0x0000000000000000000000000000000000001016" => "0xb187bd26"
  }
  @view_word "0x" <> String.duplicate("0", 64)

  @unified_account_answer "0x" <> String.duplicate("0", 320)

  @bytecode "0x60806040523480156100105760006000fd"

  @surfaces ~w(addr exchange bridge launchpad)

  test "reports every surface absent while nothing has probed the node", %{conn: conn} do
    response =
      conn
      |> get("/api/v2/paxeer-x/capabilities")
      |> json_response(200)

    for surface <- @surfaces do
      assert response[surface] == false
    end
  end

  test "answers with the four surfaces and checked_at", %{conn: conn} do
    response =
      conn
      |> get("/api/v2/paxeer-x/capabilities")
      |> json_response(200)

    assert Enum.sort(Map.keys(response) -- ["price_feed"]) == Enum.sort(["checked_at" | @surfaces])
    refute Map.has_key?(response, "custody")
    refute Map.has_key?(response, "anchor")
  end

  test "reports the surfaces of the last probe", %{conn: conn} do
    set_mox_global()

    stub(EthereumJSONRPC.Mox, :json_rpc, fn
      %{method: "eth_blockNumber"}, _options ->
        {:ok, @head}

      requests, _options when is_list(requests) ->
        {:ok, Enum.map(requests, &answer/1)}
    end)

    start_supervised!(Capabilities)

    assert {:ok, _snapshot} = Capabilities.refresh()

    response =
      conn
      |> get("/api/v2/paxeer-x/capabilities")
      |> json_response(200)

    for surface <- @surfaces do
      assert response[surface] == true
    end
  end

  defp answer(%{id: id, method: "eth_getCode", params: [address, block]}) do
    assert address in @code_addresses
    assert address == "0x0000000000000000000000000000000000001017"
    assert block == @head_quantity

    %{id: id, jsonrpc: "2.0", result: @bytecode}
  end

  defp answer(%{
         id: id,
         method: "eth_call",
         params: [%{to: to, data: data}, block]
       })
       when is_map_key(@view_calls, to) do
    assert data == Map.fetch!(@view_calls, to)
    assert block == @head_quantity

    %{id: id, jsonrpc: "2.0", result: @view_word}
  end

  defp answer(%{
         id: id,
         method: "eth_call",
         params: [%{to: @addr_precompile, data: _data}, block]
       }) do
    assert block == @head_quantity

    %{id: id, jsonrpc: "2.0", result: @unified_account_answer}
  end
end
