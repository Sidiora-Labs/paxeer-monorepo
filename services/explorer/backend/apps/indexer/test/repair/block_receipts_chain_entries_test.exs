Code.require_file("../indexer/block/realtime/paxeer_batch_case.exs", __DIR__)

defmodule Indexer.BlockReceiptsChainEntriesTest do
  use Explorer.DataCase, async: false
  use Indexer.Block.Realtime.PaxeerBatchCase

  alias EthereumJSONRPC.{Logs, Receipt}
  alias Explorer.Chain.{Block, Log, Transaction}
  alias Indexer.Block.Fetcher
  alias Indexer.Block.Fetcher.Receipts
  alias Indexer.Block.Realtime.{PaxeerBatchCase.Transcript, TestConfig}

  @moduletag capture_log: true

  setup context do
    TestConfig.preserve_env([
      {:ethereum_jsonrpc, :receipts_by_block?},
      {:ethereum_jsonrpc, :max_receipts_by_block},
      {:indexer, :block_transformer},
      {:explorer, :chain_type},
      {:indexer, Indexer.Fetcher.PaxeerX.PriceUpdates.Supervisor}
    ])

    Application.put_env(:ethereum_jsonrpc, :receipts_by_block?, true)
    Application.put_env(:ethereum_jsonrpc, :max_receipts_by_block, 100)
    Application.put_env(:indexer, :block_transformer, Indexer.Transform.Blocks.Base)
    Application.put_env(:explorer, :chain_type, :paxeer_x)
    Application.put_env(:indexer, Indexer.Fetcher.PaxeerX.PriceUpdates.Supervisor, disabled?: true)

    block = Enum.find(context.blocks, &(&1["transactions"] != []))
    [transaction] = block["transactions"]

    receipt =
      Enum.find_value(context.fixture["exchanges"], fn
        %{"request" => %{"method" => "eth_getTransactionReceipt"}, "response" => %{"result" => result}} -> result
        _ -> nil
      end)

    cosmos = fixture("../../../ethereum_jsonrpc/test/support/fixture/paxeer_x/cosmos_originated_transaction.json")
    bridge = fixture("../../../explorer/test/explorer/chain/paxeer_x/fixtures/node_bridge_out.json")
    second = %{transaction | "hash" => cosmos["transaction"]["hash"], "transactionIndex" => "0x1"}
    block = %{block | "transactions" => [transaction, second]}
    [token_log] = cosmos["receipt"]["logs"]
    market_log = hd(bridge["case"]["receiptJson"]["logs"])

    receipts =
      for {tx, log, index} <- [{transaction, token_log, 0}, {second, market_log, 1}] do
        log =
          Map.merge(log, %{
            "blockHash" => block["hash"],
            "blockNumber" => block["number"],
            "transactionHash" => tx["hash"],
            "transactionIndex" => tx["transactionIndex"],
            "logIndex" => EthereumJSONRPC.integer_to_quantity(index)
          })

        Map.merge(receipt, %{
          "transactionHash" => tx["hash"],
          "transactionIndex" => tx["transactionIndex"],
          "logs" => [log]
        })
      end

    chain_hash = block["parentHash"]

    chain_logs =
      for index <- 2..4 do
        Map.merge(token_log, %{
          "blockHash" => block["hash"],
          "blockNumber" => block["number"],
          "transactionHash" => chain_hash,
          "transactionIndex" => "0x2",
          "logIndex" => EthereumJSONRPC.integer_to_quantity(index)
        })
      end

    chain_receipt =
      Map.merge(cosmos["receipt"], %{
        "blockHash" => block["hash"],
        "blockNumber" => block["number"],
        "transactionHash" => chain_hash,
        "transactionIndex" => "0x2",
        "logs" => chain_logs
      })

    %{
      raw_block: block,
      receipts: receipts,
      chain_receipt: chain_receipt,
      number: EthereumJSONRPC.quantity_to_integer(block["number"])
    }
  end

  test "block receipts omit unlisted chain entries and all their logs before import", ctx do
    {fetcher, node} = replay(ctx, ctx.receipts ++ [ctx.chain_receipt])
    transactions = transactions(fetcher, ctx.number)
    assert {:ok, %{receipts: receipts, logs: logs}} = Receipts.fetch(fetcher, transactions)
    assert receipts == Enum.map(ctx.receipts, &(&1 |> Receipt.to_elixir() |> Receipt.elixir_to_params()))

    expected_logs = ctx.receipts |> Enum.flat_map(& &1["logs"]) |> Logs.to_elixir() |> Logs.elixir_to_params()
    assert logs == expected_logs
    assert length(receipts) == 2
    assert length(logs) == 2
    refute Enum.any?(logs, &(&1.transaction_hash == ctx.chain_receipt["transactionHash"]))
    assert Enum.map(Receipts.put(transactions, receipts), & &1.hash) == Enum.map(transactions, & &1.hash)

    start_token_workers(fetcher.json_rpc_named_arguments)
    assert {:ok, %{errors: [], inserted: inserted}} = Fetcher.fetch_and_import_range(fetcher, ctx.number..ctx.number)
    assert length(inserted.transactions) == 2
    assert length(inserted.logs) == 2
    assert Repo.aggregate(Block, :count) == 1
    assert Repo.aggregate(Transaction, :count) == 2
    assert Repo.aggregate(Log, :count) == 2
    assert Repo.all(from(l in Log, order_by: l.index, select: l.index)) == [0, 1]
    assert_block_receipts_requested(node)
  end

  test "an unlisted receipt of type 0x2 refuses the block range", ctx do
    extra = Map.put(ctx.chain_receipt, "type", "0x2")
    {fetcher, node} = replay(ctx, ctx.receipts ++ [extra])
    hash = extra["transactionHash"]

    assert {:error, {:receipts, {:unexpected_receipt, ^hash}}} =
             Fetcher.fetch_and_import_range(fetcher, ctx.number..ctx.number)

    assert_empty_import()
    assert_block_receipts_requested(node)
  end

  test "a chain entry cannot replace a missing listed receipt", ctx do
    [first, missing] = ctx.receipts
    {fetcher, node} = replay(ctx, [first, ctx.chain_receipt])
    hash = missing["transactionHash"]

    assert {:error, {:receipts, {:missing_receipts, [^hash]}}} =
             Fetcher.fetch_and_import_range(fetcher, ctx.number..ctx.number)

    assert_empty_import()
    assert_block_receipts_requested(node)
  end

  test "duplicate listed receipts with the chain marker still refuse the range", ctx do
    [first, second] = ctx.receipts
    first = Map.put(first, "type", "0xffffffff")
    {fetcher, node} = replay(ctx, [first, first, second, ctx.chain_receipt])
    hash = first["transactionHash"]

    assert {:error, {:receipts, {:duplicate_receipt, ^hash}}} =
             Fetcher.fetch_and_import_range(fetcher, ctx.number..ctx.number)

    assert_empty_import()
    assert_block_receipts_requested(node)
  end

  test "a listed receipt with a wrong transaction index still refuses the range", ctx do
    [first, second] = ctx.receipts
    first = Map.merge(first, %{"type" => "0xffffffff", "transactionIndex" => "0x2"})
    {fetcher, node} = replay(ctx, [first, second, ctx.chain_receipt])
    hash = first["transactionHash"]

    assert {:error, {:receipts, {:receipt_transaction_mismatch, ^hash}}} =
             Fetcher.fetch_and_import_range(fetcher, ctx.number..ctx.number)

    assert_empty_import()
    assert_block_receipts_requested(node)
  end

  test "a listed receipt with the chain marker retains its receipt and logs", ctx do
    [first, second] = ctx.receipts
    first = Map.put(first, "type", "0xffffffff")
    {fetcher, node} = replay(ctx, [first, second, ctx.chain_receipt])
    assert {:ok, %{receipts: [receipt, _], logs: logs}} = Receipts.fetch(fetcher, transactions(fetcher, ctx.number))
    assert receipt.transaction_hash == first["transactionHash"]
    assert receipt.type == EthereumJSONRPC.PaxeerX.stored_cosmos_transaction_type()
    assert Enum.map(logs, & &1.transaction_hash) == Enum.map(ctx.receipts, & &1["transactionHash"])
    assert_block_receipts_requested(node)
  end

  test "transaction receipt fetching retains unlisted chain entries for validation", ctx do
    Application.put_env(:ethereum_jsonrpc, :receipts_by_block?, false)
    {fetcher, node} = replay(ctx, [ctx.chain_receipt | tl(ctx.receipts)])
    hash = ctx.chain_receipt["transactionHash"]

    assert {:error, {:receipts, {:unexpected_receipt, ^hash}}} =
             Fetcher.fetch_and_import_range(fetcher, ctx.number..ctx.number)

    assert_empty_import()
    calls = Indexer.Block.Realtime.PaxeerBatchCase.calls(node) |> List.flatten()
    assert Enum.count(calls, &(&1["method"] == "eth_getTransactionReceipt")) == 2
    refute Enum.any?(calls, &(&1["method"] == "eth_getBlockReceipts"))
  end

  defp fixture(relative), do: __DIR__ |> Path.join(relative) |> File.read!() |> Jason.decode!()

  defp replay(ctx, receipts) do
    block = ctx.raw_block
    block_key = Transcript.key("eth_getBlockByNumber", [block["number"], true])
    receipts_key = Transcript.key("eth_getBlockReceipts", [block["number"]])

    recordings = %{
      block_key => %{"jsonrpc" => "2.0", "result" => block},
      receipts_key => %{"jsonrpc" => "2.0", "result" => receipts}
    }

    recordings =
      Enum.zip(block["transactions"], receipts)
      |> Enum.reduce(recordings, fn {transaction, receipt}, acc ->
        Map.put(acc, Transcript.key("eth_getTransactionReceipt", [transaction["hash"]]), %{
          "jsonrpc" => "2.0",
          "result" => receipt
        })
      end)

    node = start_supervised!({Agent, fn -> %{head: ctx.number, calls: []} end}, id: make_ref())
    listener = make_ref()
    dispatch = :cowboy_router.compile([{:_, [{"/", Transcript, %{node: node, recordings: recordings}}]}])
    {:ok, _} = :cowboy.start_clear(listener, [ip: {127, 0, 0, 1}, port: 0], %{env: %{dispatch: dispatch}})
    on_exit(fn -> :cowboy.stop_listener(listener) end)
    url = "http://127.0.0.1:#{:ranch.get_port(listener)}"
    rpc = Keyword.update!(ctx.arguments, :transport_options, &Keyword.put(&1, :urls, [url]))
    {%{ctx.block_fetcher | json_rpc_named_arguments: rpc}, node}
  end

  defp transactions(fetcher, number) do
    assert {:ok, blocks} = EthereumJSONRPC.fetch_blocks_by_range(number..number, fetcher.json_rpc_named_arguments)
    blocks.transactions_params
  end

  defp assert_block_receipts_requested(node) do
    calls = Indexer.Block.Realtime.PaxeerBatchCase.calls(node) |> List.flatten()
    assert Enum.any?(calls, &(&1["method"] == "eth_getBlockReceipts"))
    refute Enum.any?(calls, &(&1["method"] == "eth_getTransactionReceipt"))
  end

  defp assert_empty_import do
    for schema <- [Block, Transaction, Log], do: assert(Repo.aggregate(schema, :count) == 0)
  end

  defp start_token_workers(rpc) do
    for worker <- [Indexer.Fetcher.Token, Indexer.Fetcher.TokenBalance.Historical, Indexer.Fetcher.TokenBalance.Current] do
      start_supervised!({Task.Supervisor, name: Module.concat(worker, TaskSupervisor)})

      pid =
        start_supervised!(
          {worker,
           [[json_rpc_named_arguments: rpc, flush_interval: 60_000, poll: false, max_concurrency: 1], [name: worker]]}
        )

      await_initialized(pid)
    end
  end

  defp await_initialized(pid, attempts \\ 500) do
    state = :sys.get_state(pid)

    if state.init_task in [:complete, nil] do
      :ok
    else
      assert attempts > 0, "production queue did not initialize: #{inspect(state)}"
      Process.sleep(10)
      await_initialized(pid, attempts - 1)
    end
  end
end
