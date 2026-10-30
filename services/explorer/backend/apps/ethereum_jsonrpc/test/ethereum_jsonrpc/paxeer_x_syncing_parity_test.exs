defmodule EthereumJSONRPC.PaxeerXSyncingParityTest do
  use ExUnit.Case, async: true

  @behaviour :cowboy_handler

  alias EthereumJSONRPC.{HTTP, PaxeerX}

  setup_all do
    {:ok, _} = Application.ensure_all_started(:cowboy)
    {:ok, _} = Application.ensure_all_started(:httpoison)
    :ok
  end

  test "syncing_object_reports_not_synced" do
    json_rpc_named_arguments =
      syncing_endpoint(%{
        "startingBlock" => "0x0",
        "currentBlock" => "0x1714efd",
        "highestBlock" => "0x1714efe"
      })

    assert {:ok, {:syncing, 24_203_005, 24_203_006}} =
             PaxeerX.fetch_syncing_status(json_rpc_named_arguments)

    assert_receive {:syncing_request, request}
    assert request == %{"jsonrpc" => "2.0", "id" => 0, "method" => "eth_syncing", "params" => []}
  end

  test "syncing_false_reports_synced" do
    json_rpc_named_arguments = syncing_endpoint(false)

    assert {:ok, :synced} = PaxeerX.fetch_syncing_status(json_rpc_named_arguments)

    assert_receive {:syncing_request, request}
    assert request == %{"jsonrpc" => "2.0", "id" => 0, "method" => "eth_syncing", "params" => []}
  end

  @impl :cowboy_handler
  def init(req, {test_pid, result} = state) do
    {:ok, body, req} = :cowboy_req.read_body(req)
    request = Jason.decode!(body)
    send(test_pid, {:syncing_request, request})

    response = Jason.encode!(%{"jsonrpc" => "2.0", "id" => Map.fetch!(request, "id"), "result" => result})
    req = :cowboy_req.reply(200, %{"content-type" => "application/json"}, response, req)
    {:ok, req, state}
  end

  defp syncing_endpoint(result) do
    listener = make_ref()
    dispatch = :cowboy_router.compile([{:_, [{"/", __MODULE__, {self(), result}}]}])

    {:ok, _} =
      :cowboy.start_clear(
        listener,
        %{socket_opts: [ip: {127, 0, 0, 1}, port: 0]},
        %{env: %{dispatch: dispatch}}
      )

    on_exit(fn -> :cowboy.stop_listener(listener) end)

    [
      transport: HTTP,
      transport_options: [
        http: HTTP.HTTPoison,
        http_options: [recv_timeout: 5_000, timeout: 5_000],
        urls: ["http://127.0.0.1:#{:ranch.get_port(listener)}"]
      ]
    ]
  end
end
