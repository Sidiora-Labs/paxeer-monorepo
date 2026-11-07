defmodule EthereumJSONRPC.Repair2233439Test do
  @moduledoc """
  Repair record 22.33.439: block-tag-error-keys is proven by the single HTTP
  invalid-parameter tests and the batch HTTP canonical-error test below.
  Recorded block data comes from support/fixture/paxeer_x/new_head_full.json;
  JSON-RPC error transcripts exercise the audited -32602 wire shape.

  Baseline verification was not run: the task's explicit gate order requires
  complete implementation before hf-build and verification only via hf-done.
  No pre-change test failure is claimed. The runner records the verified revision.

  Wiring request (domain 14): services/explorer/backend/apps/ethereum_jsonrpc/lib/ethereum_jsonrpc/ipc.ex
  must normalize single-response errors with EthereumJSONRPC.HTTP.standardize_error/1.
  Wiring request (domain 14): services/explorer/backend/apps/ethereum_jsonrpc/lib/ethereum_jsonrpc/web_socket/web_socket_client.ex
  must normalize JSON-RPC errors with EthereumJSONRPC.HTTP.standardize_error/1.
  The existing string-key match remains compatible with these transports.
  """

  use ExUnit.Case, async: false

  @behaviour :cowboy_handler

  alias EthereumJSONRPC.Block.ByTag
  alias EthereumJSONRPC.HTTP

  @block_fixture Path.expand("../support/fixture/paxeer_x/new_head_full.json", __DIR__)
  @invalid_params ~s({"jsonrpc":"2.0","id":0,"error":{"code":-32602,"message":"invalid argument 0: hex string without 0x prefix"}})
  @invalid_params_data ~s({"jsonrpc":"2.0","id":0,"error":{"code":-32602,"message":"Invalid params","data":{"tag":"safe"}}})

  setup_all do
    {:ok, _} = Application.ensure_all_started(:cowboy)
    {:ok, _} = Application.ensure_all_started(:httpoison)
    :ok
  end

  test "block-tag-error-keys: single HTTP invalid parameters become invalid_tag" do
    result = rpc(@invalid_params)

    assert result == {:error, %{code: -32602, message: "invalid argument 0: hex string without 0x prefix"}}
    assert ByTag.number_from_result(result) == {:error, :invalid_tag}
    assert_request()
  end

  test "block-tag-error-keys: single HTTP invalid parameters with data become invalid_tag" do
    result = rpc(@invalid_params_data)

    assert result == {:error, %{code: -32602, message: "Invalid params", data: %{"tag" => "safe"}}}
    assert ByTag.number_from_result(result) == {:error, :invalid_tag}
    assert_request()
  end

  test "block-tag-error-keys: batch HTTP canonical errors become invalid_tag" do
    arguments = endpoint("[" <> @invalid_params_data <> "]")
    request = ByTag.request(%{id: 0, tag: "safe"})

    assert {:ok, [%{id: 0, jsonrpc: "2.0", error: error}]} = EthereumJSONRPC.json_rpc([request], arguments)
    assert error == %{code: -32602, message: "Invalid params", data: %{"tag" => "safe"}}
    assert ByTag.number_from_result({:error, error}) == {:error, :invalid_tag}
    assert_request(true)
  end

  test "HTTP errors other than invalid parameters retain their code message and data" do
    response =
      ~s({"jsonrpc":"2.0","id":0,"error":{"code":-32000,"message":"header not found","data":{"code":-32602,"number":"0x1714efe"}}})

    result = rpc(response)

    assert result ==
             {:error, %{code: -32000, message: "header not found", data: %{"code" => -32602, "number" => "0x1714efe"}}}

    assert ByTag.number_from_result(result) == result
    assert_request()
  end

  test "HTTP recorded block still yields its block number" do
    %{"block" => block} = @block_fixture |> File.read!() |> Jason.decode!()
    response = Jason.encode!(%{jsonrpc: "2.0", id: 0, result: block})
    result = rpc(response)

    assert result == {:ok, block}
    assert ByTag.number_from_result(result) == {:ok, 24_203_006}
    assert_request()
  end

  test "HTTP missing block still yields not_found" do
    result = rpc(~s({"jsonrpc":"2.0","id":0,"result":null}))

    assert result == {:ok, nil}
    assert ByTag.number_from_result(result) == {:error, :not_found}
    assert_request()
  end

  @impl :cowboy_handler
  def init(req, {owner, response} = state) do
    {:ok, body, req} = :cowboy_req.read_body(req)
    send(owner, {:rpc_request, :cowboy_req.method(req), Jason.decode!(body)})
    req = :cowboy_req.reply(200, %{"content-type" => "application/json"}, response, req)
    {:ok, req, state}
  end

  defp rpc(response) do
    request = ByTag.request(%{id: 0, tag: "safe"})
    EthereumJSONRPC.json_rpc(request, endpoint(response))
  end

  defp assert_request(batch? \\ false) do
    expected = %{"jsonrpc" => "2.0", "id" => 0, "method" => "eth_getBlockByNumber", "params" => ["safe", false]}
    expected = if batch?, do: [expected], else: expected
    assert_receive {:rpc_request, "POST", ^expected}, 1_000
  end

  defp endpoint(response) do
    listener = make_ref()
    dispatch = :cowboy_router.compile([{:_, [{"/", __MODULE__, {self(), response}}]}])

    {:ok, _} =
      :cowboy.start_clear(listener, %{socket_opts: [ip: {127, 0, 0, 1}, port: 0]}, %{env: %{dispatch: dispatch}})

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
