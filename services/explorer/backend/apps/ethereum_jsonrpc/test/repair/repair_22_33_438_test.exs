defmodule EthereumJSONRPC.Repair2233438Test do
  use ExUnit.Case, async: false

  @behaviour :cowboy_handler

  alias EthereumJSONRPC.{Blocks, HTTP}

  @fixture Path.expand("../support/fixture/paxeer_x/new_head_full.json", __DIR__)
  @head_hash "0x63ef5cd005e73dc5bbd69846df6eff253a97c21185c0286e8c493256ebe4329d"
  @head_number 24_203_006

  setup_all do
    {:ok, _} = Application.ensure_all_started(:cowboy)
    {:ok, _} = Application.ensure_all_started(:httpoison)
    :ok
  end

  setup do
    %{"new_head" => head, "block" => block} = @fixture |> File.read!() |> Jason.decode!()
    listener = make_ref()
    dispatch = :cowboy_router.compile([{:_, [{"/", __MODULE__, {self(), block}}]}])

    {:ok, _} =
      :cowboy.start_clear(listener, %{socket_opts: [ip: {127, 0, 0, 1}, port: 0]}, %{env: %{dispatch: dispatch}})

    on_exit(fn -> :cowboy.stop_listener(listener) end)

    arguments = [
      transport: HTTP,
      transport_options: [
        http: HTTP.HTTPoison,
        http_options: [recv_timeout: 5_000, timeout: 5_000],
        urls: ["http://127.0.0.1:#{:ranch.get_port(listener)}"]
      ]
    ]

    {:ok, arguments: arguments, head: head, block: block}
  end

  test "finalized-block-tag: block helper imports the recorded committed head over HTTP", context do
    assert context.head["hash"] == @head_hash
    assert context.head["number"] == "0x1714efe"
    assert context.block["hash"] == context.head["hash"]
    assert context.block["number"] == context.head["number"]

    assert {:ok, %Blocks{blocks_params: [block], errors: []} = finalized} =
             EthereumJSONRPC.fetch_block_by_tag("finalized", context.arguments)

    assert_request("finalized")
    assert block.number == @head_number
    assert block.hash == @head_hash
    assert block.parent_hash == context.block["parentHash"]
    assert block.state_root == context.block["stateRoot"]
    assert block.receipts_root == context.block["receiptsRoot"]
    assert block.transactions_root == context.block["transactionsRoot"]
    assert finalized.transactions_params == []
    assert finalized.block_second_degree_relations_params == []

    for tag <- ~w(safe latest) do
      assert EthereumJSONRPC.fetch_block_by_tag(tag, context.arguments) == {:ok, finalized}
      assert_request(tag)
    end
  end

  test "finalized-block-tag: number helper consumes the same committed head for all BFT head tags", context do
    for tag <- ~w(finalized safe latest) do
      assert EthereumJSONRPC.fetch_block_number_by_tag(tag, context.arguments) == {:ok, @head_number}
      assert_request(tag)
    end
  end

  test "finalized-block-tag: the shared builder preserves finalized and an independent request id", context do
    request = EthereumJSONRPC.Block.ByTag.request(%{id: 438, tag: "finalized"})
    assert {:ok, [%{id: 438, result: block}]} = EthereumJSONRPC.json_rpc([request], context.arguments)
    assert block == context.block
    assert_request("finalized", 438)
  end

  @impl :cowboy_handler
  def init(req, {owner, block} = state) do
    {:ok, body, req} = :cowboy_req.read_body(req)
    requests = Jason.decode!(body)
    send(owner, {:rpc_request, :cowboy_req.method(req), requests})

    responses =
      Enum.map(requests, fn %{
                              "jsonrpc" => "2.0",
                              "id" => id,
                              "method" => "eth_getBlockByNumber",
                              "params" => [tag, false]
                            }
                            when tag in ~w(finalized safe latest) ->
        %{jsonrpc: "2.0", id: id, result: block}
      end)

    req = :cowboy_req.reply(200, %{"content-type" => "application/json"}, Jason.encode!(responses), req)
    {:ok, req, state}
  end

  defp assert_request(tag, id \\ 0) do
    expected = [%{"jsonrpc" => "2.0", "id" => id, "method" => "eth_getBlockByNumber", "params" => [tag, false]}]
    assert_receive {:rpc_request, "POST", ^expected}, 1_000
  end
end
