defmodule Indexer.Repair2233472Test do
  use ExUnit.Case, async: false

  alias EthereumJSONRPC.{Blocks, Logs}
  alias EthereumJSONRPC.Block.ByNumber
  alias Explorer.Chain.{Block, Log}
  alias Explorer.Chain.Cache.CeloCoreContracts
  alias Indexer.Fetcher.Celo.EpochLogs
  alias Indexer.Helper

  @moduletag :capture_log
  @rpc_error %{"code" => -32000, "message" => "query timeout", "data" => "recorded batch element failure"}

  setup tags do
    keys = [
      {:tesla, :adapter},
      {:indexer, Indexer.Block.Realtime.Fetcher},
      {:explorer, :chain_identity},
      {:explorer, :celo},
      {:explorer, CeloCoreContracts}
    ]

    previous = Enum.map(keys, fn {app, key} -> {app, key, Application.fetch_env(app, key)} end)

    on_exit(fn ->
      for {app, key, previous_value} <- previous do
        case previous_value do
          {:ok, value} -> Application.put_env(app, key, value)
          :error -> Application.delete_env(app, key)
        end
      end
    end)

    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)
    Application.put_env(:indexer, Indexer.Block.Realtime.Fetcher, polling_period: 3_750)

    if tags[:pure_range_validation], do: :ok, else: start_rpc_fixture()
  end

  defp start_rpc_fixture do
    block = recorded_response("block_importer_import_1_pending.json")
    receipt = recorded_response("import_receipt_perform_1.json")
    templates = %{block: block, log: hd(receipt["logs"])}

    store =
      start_supervised!(
        {Agent,
         fn ->
           %{head: 2, safe_supported: true, error_id: nil, failed_batches: :always, batches: 0, requests: []}
         end}
      )

    bypass = Bypass.open()

    Bypass.expect(bypass, "POST", "/", fn conn ->
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      request = Jason.decode!(body)

      response =
        Agent.get_and_update(store, fn state ->
          state = %{state | requests: state.requests ++ [request]}

          if is_list(request) do
            state = %{state | batches: state.batches + 1}
            # JSON-RPC batches may arrive in a different order from the requests.
            {request |> Enum.map(&rpc_response(&1, state, templates)) |> Enum.reverse(), state}
          else
            {rpc_response(request, state, templates), state}
          end
        end)

      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.resp(200, Jason.encode!(response))
    end)

    %{rpc: Helper.json_rpc_named_arguments("http://localhost:#{bypass.port}"), store: store}
  end

  test "block-interval-genesis-division: heads 0 and 1 poll without samples and head 2 estimates committed timestamps",
       %{rpc: rpc, store: store} do
    for head <- [0, 1, 2] do
      Agent.update(store, &%{&1 | head: head, requests: []})
      expected = if head < 2, do: 3_750, else: 6_000
      assert {:ok, ^expected, ^head} = Helper.get_block_check_interval(rpc)

      requests = Agent.get(store, & &1.requests) |> List.flatten()

      assert Enum.map(requests, & &1["params"]) ==
               if(head < 2, do: [["safe", false]], else: [["safe", false], ["0x1", false], ["0x2", false]])

      assert Enum.all?(requests, &(&1["method"] == "eth_getBlockByNumber"))
      # Feed the returned committed head through the real RPC block decoder and schema.
      assert_committed_head_schema(head, rpc)
    end
  end

  test "block-interval-genesis-division: unset polling uses the existing 2000ms default", %{rpc: rpc, store: store} do
    for config <- [[polling_period: nil], nil] do
      Application.put_env(:indexer, Indexer.Block.Realtime.Fetcher, config)
      Agent.update(store, &%{&1 | head: 0, requests: []})
      assert Helper.get_block_check_interval(rpc) == {:ok, 2_000, 0}
      assert length(Agent.get(store, & &1.requests)) == 1
    end
  end

  test "block-interval-genesis-division: unsupported safe tag uses latest without sampling a future height", %{
    rpc: rpc,
    store: store
  } do
    Agent.update(store, &%{&1 | head: 0, safe_supported: false})
    assert Helper.get_block_check_interval(rpc) == {:ok, 3_750, 0}
    requests = Agent.get(store, & &1.requests) |> List.flatten()
    assert Enum.map(requests, & &1["params"]) == List.duplicate(["safe", false], 3) ++ [["latest", false]]
    assert_committed_head_schema(0, rpc)
  end

  for {label, range, size, expected} <- [
        {"exact multiple", 1..10, 5, [1..5, 6..10]},
        {"one element", 7..7, 5, [7..7]},
        {"partial final chunk", 1..11, 5, [1..5, 6..10, 11..11]},
        {"size one", 1..3, 1, [1..1, 2..2, 3..3]},
        {"larger than range", 1..3, 10, [1..3]}
      ] do
    test "range-chunk-extra-interval: #{label} reaches the log schema exactly once per height", %{
      rpc: rpc,
      store: store
    } do
      range = unquote(Macro.escape(range))
      expected = unquote(Macro.escape(expected))
      stream = Helper.range_chunk_every(range, unquote(size))
      assert Enum.to_list(stream) == expected

      logs =
        stream
        |> Enum.with_index()
        |> Enum.flat_map(fn {first..last//1, id} ->
          request = Logs.request(id, %{from_block: first, to_block: last})
          assert {:ok, responses} = Helper.repeated_batch_rpc_call([request], rpc, &inspect/1, 1)
          assert {:ok, params} = Logs.from_responses(responses)
          schema_logs(params)
        end)

      assert Enum.map(logs, & &1.block_number) == Enum.to_list(range)
      requests = Agent.get(store, & &1.requests) |> List.flatten()

      assert Enum.map(requests, fn %{"params" => [filter]} ->
               {EthereumJSONRPC.quantity_to_integer(filter["fromBlock"]),
                EthereumJSONRPC.quantity_to_integer(filter["toBlock"])}
             end) == Enum.map(expected, &{&1.first, &1.last})
    end
  end

  @tag :pure_range_validation
  test "range-chunk-extra-interval: rejects invalid chunk sizes and non-contiguous ranges" do
    for size <- [0, -1, 1.5, nil, "5"] do
      assert_raise ArgumentError, fn -> Helper.range_chunk_every(1..10, size) end
    end

    for range <- [10..1, 1..10//2, 1..0//1] do
      assert_raise ArgumentError, fn -> Helper.range_chunk_every(range, 5) end
    end
  end

  @tag :pure_range_validation
  test "range-chunk-extra-interval: integer arithmetic preserves large inclusive boundaries" do
    first = 9_007_199_254_740_993
    last = first + 9
    assert Enum.to_list(Helper.range_chunk_every(first..last, 5)) == [first..(first + 4), (first + 5)..last]
  end

  test "batch-rpc-error-return: exhausted mixed batch is a typed error preserving successes by id", %{
    rpc: rpc,
    store: store
  } do
    Agent.update(store, &%{&1 | error_id: 42})
    requests = [Logs.request(7, %{from_block: 1, to_block: 1}), Logs.request(42, %{from_block: 2, to_block: 2})]

    assert {:error, {:batch_rpc_error, responses}} = Helper.repeated_batch_rpc_call(requests, rpc, &inspect/1, 2)
    assert Enum.map(responses, & &1.id) == [42, 7]
    by_id = Map.new(responses, &{&1.id, &1})
    assert by_id[42].error == %{code: -32000, message: "query timeout", data: "recorded batch element failure"}
    assert {:ok, params} = Logs.from_responses([by_id[7]])
    assert [%Log{block_number: 1}] = schema_logs(params)
    assert {:error, [_]} = Logs.from_responses(responses)
    assert Agent.get(store, & &1.batches) == 2
    assert Agent.get(store, & &1.requests) == List.duplicate(Jason.decode!(Jason.encode!(requests)), 2)
  end

  test "batch-rpc-error-return: retry recovery returns only successful responses consumable by the schema", %{
    rpc: rpc,
    store: store
  } do
    Agent.update(store, &%{&1 | error_id: 42, failed_batches: 1})
    requests = [Logs.request(7, %{from_block: 1, to_block: 1}), Logs.request(42, %{from_block: 2, to_block: 2})]

    assert {:ok, responses} = Helper.repeated_batch_rpc_call(requests, rpc, &inspect/1, 2)
    assert Enum.map(responses, & &1.id) == [42, 7]
    assert Enum.all?(responses, &Map.has_key?(&1, :result))
    assert {:ok, params} = Logs.from_responses(responses)
    assert params |> schema_logs() |> Enum.map(& &1.block_number) |> Enum.sort() == [1, 2]
    assert Agent.get(store, & &1.batches) == 2
  end

  test "batch-rpc-error-return: real Celo epoch fetcher propagates an exhausted element error", %{
    rpc: rpc,
    store: store
  } do
    Application.put_env(:explorer, :chain_identity, {:optimism, :celo})
    Application.put_env(:explorer, :celo, l2_migration_block: nil)

    Application.put_env(:explorer, CeloCoreContracts,
      contracts: %{
        "addresses" => %{
          "GasPriceMinimum" => [
            %{"address" => "0x2265f9193ec79d072f3a617a22b27866968b88a6", "updated_at_block_number" => 0}
          ]
        }
      }
    )

    Agent.update(store, &%{&1 | error_id: 1})
    assert {:error, {:batch_rpc_error, responses}} = EpochLogs.fetch([%{number: 1}, %{number: 2}], rpc)
    assert Enum.map(responses, & &1.id) == [1, 0]
    assert %{error: %{code: -32000}} = Enum.find(responses, &(&1.id == 1))
    assert %{result: [_]} = Enum.find(responses, &(&1.id == 0))
    assert Agent.get(store, & &1.batches) == 3
  end

  defp assert_committed_head_schema(head, rpc) do
    request = ByNumber.request(%{id: 19, number: head}, false)
    assert {:ok, responses} = Helper.repeated_batch_rpc_call([request], rpc, &inspect/1, 1)
    fetched = Blocks.from_responses(responses, %{19 => %{number: head}})
    assert fetched.errors == []
    assert [params] = fetched.blocks_params
    changeset = Block.changeset(%Block{}, Map.put(params, :consensus, true))
    assert changeset.valid?, inspect(changeset.errors)
    assert %Block{number: ^head, consensus: true} = Ecto.Changeset.apply_changes(changeset)
  end

  defp schema_logs(params) do
    Enum.map(params, fn params ->
      changeset = Log.changeset(%Log{}, params)
      assert changeset.valid?, inspect(changeset.errors)
      Ecto.Changeset.apply_changes(changeset)
    end)
  end

  defp rpc_response(%{"id" => id, "params" => ["safe", false]}, %{safe_supported: false}, _templates) do
    %{"jsonrpc" => "2.0", "id" => id, "error" => %{"code" => -32602, "message" => "unsupported block tag"}}
  end

  defp rpc_response(%{"id" => id} = request, state, templates) do
    if id == state.error_id and (state.failed_batches == :always or state.batches <= state.failed_batches) do
      %{"jsonrpc" => "2.0", "id" => id, "error" => @rpc_error}
    else
      %{"jsonrpc" => "2.0", "id" => id, "result" => rpc_result(request, state, templates)}
    end
  end

  defp rpc_result(%{"method" => "eth_getBlockByNumber", "params" => [tag, false]}, state, templates) do
    height = if tag in ["safe", "latest"], do: state.head, else: EthereumJSONRPC.quantity_to_integer(tag)
    assert height <= state.head

    # Recorded block shape with heights/timestamps normalized to a newly started chain.
    Map.merge(templates.block, %{
      "number" => EthereumJSONRPC.integer_to_quantity(height),
      "timestamp" => EthereumJSONRPC.integer_to_quantity(1_500_000_000 + height * 12),
      "transactions" => []
    })
  end

  defp rpc_result(%{"method" => "eth_getLogs", "params" => [filter]}, _state, templates) do
    first = EthereumJSONRPC.quantity_to_integer(filter["fromBlock"])
    last = EthereumJSONRPC.quantity_to_integer(filter["toBlock"])
    assert first <= last

    for height <- first..last do
      Map.put(templates.log, "blockNumber", EthereumJSONRPC.integer_to_quantity(height))
    end
  end

  defp recorded_response(name) do
    path = Path.expand("../../../explorer/test/support/fixture/vcr_cassettes/#{name}", __DIR__)
    [record] = path |> File.read!() |> Jason.decode!()
    record["response"]["body"] |> Jason.decode!() |> Map.fetch!("result")
  end
end
