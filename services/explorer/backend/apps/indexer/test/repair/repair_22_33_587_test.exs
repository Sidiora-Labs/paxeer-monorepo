Code.require_file("../indexer/block/realtime/paxeer_batch_case.exs", __DIR__)

defmodule Indexer.Repair2233587Test do
  use Explorer.DataCase, async: false
  use Indexer.Block.Realtime.PaxeerBatchCase

  alias Explorer.Chain.{Block, Log, TokenTransfer, Transaction}
  alias Explorer.Chain.PaxeerX.MarketEvent
  alias Indexer.Block.Fetcher
  alias Indexer.Block.Realtime.TestConfig

  @moduletag capture_log: true

  defmodule ReceiptNode do
    @behaviour Plug
    def init(options), do: options

    def call(conn, {node, observer}) do
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      requests = Jason.decode!(body)
      state = Agent.get(node, & &1)

      responses =
        Enum.map(requests, fn %{"jsonrpc" => "2.0", "id" => id, "method" => method, "params" => params} ->
          send(observer, {:receipt_rpc, method, params})

          result =
            case {method, params} do
              {"eth_getBlockByNumber", [quantity, true]} ->
                true = String.downcase(quantity) == String.downcase(state.block["number"])
                state.block

              {"eth_getTransactionReceipt", [hash]} ->
                Enum.find(state.receipts, &(&1["transactionHash"] == hash))

              {"eth_getBlockReceipts", [quantity]} ->
                true = String.downcase(quantity) == String.downcase(state.block["number"])
                state.receipts
            end

          %{jsonrpc: "2.0", id: id, result: result}
        end)

      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.send_resp(200, Jason.encode!(responses))
    end
  end

  setup context do
    TestConfig.preserve_env([
      {:ethereum_jsonrpc, :receipts_by_block?},
      {:ethereum_jsonrpc, :max_receipts_by_block},
      {:indexer, :block_transformer},
      {:explorer, :chain_type},
      {:indexer, Indexer.Fetcher.PaxeerX.PriceUpdates.Supervisor}
    ])

    Application.put_env(:ethereum_jsonrpc, :receipts_by_block?, false)
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

    # Retained node receipt and kernel-emitted event bytes, placed in the recorded
    # full-block envelope. Only identity fields are varied by the fault cases.
    cosmos = fixture("../../../ethereum_jsonrpc/test/support/fixture/paxeer_x/cosmos_originated_transaction.json")
    bridge = fixture("../../../explorer/test/explorer/chain/paxeer_x/fixtures/node_bridge_out.json")
    token_log = hd(cosmos["receipt"]["logs"])
    market_log = hd(bridge["case"]["receiptJson"]["logs"])

    logs =
      [token_log, market_log]
      |> Enum.with_index(fn log, index ->
        Map.merge(log, %{
          "blockHash" => block["hash"],
          "blockNumber" => block["number"],
          "transactionHash" => transaction["hash"],
          "transactionIndex" => transaction["transactionIndex"],
          "logIndex" => EthereumJSONRPC.integer_to_quantity(index)
        })
      end)

    receipt = Map.put(receipt, "logs", logs)
    node = start_supervised!({Agent, fn -> %{block: block, receipts: [receipt]} end}, id: :receipt_node)
    ref = make_ref()
    {:ok, _} = Application.ensure_all_started(:plug_cowboy)

    start_supervised!(
      {Plug.Cowboy, scheme: :http, plug: {ReceiptNode, {node, self()}}, options: [port: 0, ref: ref]},
      id: ref
    )

    rpc = [
      transport: EthereumJSONRPC.HTTP,
      transport_options: [
        http: EthereumJSONRPC.HTTP.HTTPoison,
        http_options: [timeout: 5_000, recv_timeout: 5_000],
        urls: ["http://127.0.0.1:#{:ranch.get_port(ref)}/"],
        fallback_urls: []
      ],
      variant: EthereumJSONRPC.PaxeerX
    ]

    # These are the production queues used by the realtime import callback.
    # Their metadata/balance work is outside the synchronous block import gate.
    for worker <- [Indexer.Fetcher.Token, Indexer.Fetcher.TokenBalance.Historical, Indexer.Fetcher.TokenBalance.Current] do
      task_supervisor = Module.concat(worker, TaskSupervisor)
      start_supervised!({Task.Supervisor, name: task_supervisor})

      pid =
        start_supervised!(
          {worker,
           [[json_rpc_named_arguments: rpc, flush_interval: 60_000, poll: false, max_concurrency: 1], [name: worker]]}
        )

      await_initialized(pid)
    end

    %{
      receipt_node: node,
      receipt: receipt,
      raw_block: block,
      number: EthereumJSONRPC.quantity_to_integer(block["number"]),
      fetcher: %{context.block_fetcher | json_rpc_named_arguments: rpc}
    }
  end

  test "invented-log-indices: conflicting logs quarantine the range and canonical retry imports committed identities",
       ctx do
    [token, market] = ctx.receipt["logs"]
    poisoned = %{ctx.receipt | "logs" => [token, %{market | "logIndex" => token["logIndex"]}]}
    put_receipts(ctx, [poisoned])
    key = {ctx.raw_block["hash"], 0}
    assert {:error, {:receipts, {:duplicate_log_index, ^key}}} = fetch(ctx)
    assert_empty_import()
    assert_receive {:receipt_rpc, "eth_getTransactionReceipt", [hash]}
    assert hash == ctx.receipt["transactionHash"]

    put_receipts(ctx, [ctx.receipt])
    assert {:ok, %{errors: [], inserted: inserted}} = fetch(ctx)
    assert length(inserted.logs) == 2
    assert_canonical_import(ctx)

    before = persisted_events()
    put_receipts(ctx, [poisoned])
    assert {:error, {:receipts, {:duplicate_log_index, ^key}}} = fetch(ctx)
    assert persisted_events() == before

    put_receipts(ctx, [ctx.receipt])
    assert {:ok, %{errors: []}} = fetch(ctx)
    assert persisted_events() == before
    assert_canonical_import(ctx)
  end

  test "invented-log-indices: log indices are unique across all transactions in a block", ctx do
    Application.put_env(:ethereum_jsonrpc, :receipts_by_block?, true)
    [transaction] = ctx.raw_block["transactions"]
    [token, market] = ctx.receipt["logs"]
    cosmos = fixture("../../../ethereum_jsonrpc/test/support/fixture/paxeer_x/cosmos_originated_transaction.json")
    second_hash = cosmos["transaction"]["hash"]
    second_transaction = %{transaction | "hash" => second_hash, "transactionIndex" => "0x1"}
    second_log = %{market | "transactionHash" => second_hash, "transactionIndex" => "0x1", "logIndex" => "0x0"}

    second_receipt = %{
      ctx.receipt
      | "transactionHash" => second_hash,
        "transactionIndex" => "0x1",
        "logs" => [second_log]
    }

    Agent.update(ctx.receipt_node, fn state ->
      %{state | block: %{state.block | "transactions" => [transaction, second_transaction]}}
    end)

    put_receipts(ctx, [%{ctx.receipt | "logs" => [token]}, second_receipt])
    key = {ctx.raw_block["hash"], 0}
    assert {:error, {:receipts, {:duplicate_log_index, ^key}}} = fetch(ctx)
    assert_empty_import()
  end

  test "invented-log-indices: byte-identical duplicate logs are rejected under the quarantine policy", ctx do
    [token, market] = ctx.receipt["logs"]
    put_receipts(ctx, [%{ctx.receipt | "logs" => [token, token, market]}])
    key = {ctx.raw_block["hash"], 0}
    assert {:error, {:receipts, {:duplicate_log_index, ^key}}} = fetch(ctx)
    assert_empty_import()
  end

  test "invented-log-indices: duplicate block receipts are rejected before their log-free contents can collapse", ctx do
    Application.put_env(:ethereum_jsonrpc, :receipts_by_block?, true)
    receipt = %{ctx.receipt | "logs" => []}
    put_receipts(ctx, [receipt, receipt])
    hash = receipt["transactionHash"]
    assert {:error, {:receipts, {:duplicate_receipt, ^hash}}} = fetch(ctx)
    assert_receive {:receipt_rpc, "eth_getBlockReceipts", [_]}
    assert_empty_import()
  end

  test "invented-log-indices: missing and foreign receipts cannot reach the import runner", ctx do
    Application.put_env(:ethereum_jsonrpc, :receipts_by_block?, true)
    hash = ctx.receipt["transactionHash"]
    put_receipts(ctx, [])
    assert {:error, {:receipts, {:missing_receipts, [^hash]}}} = fetch(ctx)
    assert_empty_import()

    foreign_hash = ctx.raw_block["parentHash"]
    put_receipts(ctx, [%{ctx.receipt | "transactionHash" => foreign_hash, "logs" => []}])
    assert {:error, {:receipts, {:unexpected_receipt, ^foreign_hash}}} = fetch(ctx)
    assert_empty_import()
  end

  test "invented-log-indices: receipt transaction positions must match the fetched block", ctx do
    put_receipts(ctx, [%{ctx.receipt | "transactionIndex" => "0x1"}])
    hash = ctx.receipt["transactionHash"]
    assert {:error, {:receipts, {:receipt_transaction_mismatch, ^hash}}} = fetch(ctx)
    assert_empty_import()
  end

  test "invented-log-indices: log block and transaction identity must agree with the canonical transaction", ctx do
    [token, market] = ctx.receipt["logs"]

    for {field, value} <- [
          {"blockHash", ctx.raw_block["parentHash"]},
          {"blockNumber", EthereumJSONRPC.integer_to_quantity(ctx.number + 1)},
          {"transactionHash", ctx.raw_block["parentHash"]},
          {"transactionIndex", "0x1"}
        ] do
      corrupted = Map.put(token, field, value)
      put_receipts(ctx, [%{ctx.receipt | "logs" => [corrupted, market]}])
      key = {corrupted["blockHash"], 0}
      assert {:error, {:receipts, {:log_transaction_mismatch, ^key}}} = fetch(ctx)
      assert_empty_import()
    end
  end

  test "invented-log-indices: unique out-of-order indices are preserved rather than sorted or regenerated", ctx do
    [token, market] = ctx.receipt["logs"]
    put_receipts(ctx, [%{ctx.receipt | "logs" => [%{market | "logIndex" => "0xb"}, %{token | "logIndex" => "0x8"}]}])
    assert {:ok, %{errors: []}} = fetch(ctx)
    assert Repo.all(from(l in Log, order_by: l.index, select: l.index)) == [8, 11]
    assert Repo.one!(TokenTransfer).log_index == 8
    assert Repo.one!(MarketEvent).log_index == 11
  end

  defp fixture(relative), do: __DIR__ |> Path.join(relative) |> File.read!() |> Jason.decode!()
  defp fetch(ctx), do: Fetcher.fetch_and_import_range(ctx.fetcher, ctx.number..ctx.number)
  defp put_receipts(ctx, receipts), do: Agent.update(ctx.receipt_node, &%{&1 | receipts: receipts})

  defp assert_empty_import do
    for schema <- [Block, Transaction, Log, TokenTransfer, MarketEvent], do: assert(Repo.aggregate(schema, :count) == 0)
  end

  defp assert_canonical_import(ctx) do
    assert Repo.aggregate(Block, :count) == 1
    assert Repo.aggregate(Transaction, :count) == 1
    assert Repo.aggregate(Log, :count) == 2
    assert Repo.aggregate(TokenTransfer, :count) == 1
    assert Repo.aggregate(MarketEvent, :count) == 1
    [token_log, market_log] = Repo.all(from(l in Log, order_by: l.index))
    transfer = Repo.one!(TokenTransfer)
    market = Repo.one!(MarketEvent)
    assert token_log.index == 0
    assert market_log.index == 1
    assert transfer.log_index == token_log.index
    assert market.log_index == market_log.index
    assert transfer.transaction_hash == token_log.transaction_hash
    assert market.transaction_hash == market_log.transaction_hash
    assert market.block_hash == market_log.block_hash
    assert market.block_number == ctx.number
    assert market.event_name == "BridgeOut"
    assert to_string(token_log.block_hash) == ctx.raw_block["hash"]
    assert to_string(token_log.transaction_hash) == ctx.receipt["transactionHash"]
    assert token_log.data.bytes == Base.decode16!(String.slice(hd(ctx.receipt["logs"])["data"], 2..-1//1), case: :mixed)
  end

  defp persisted_events do
    {Repo.all(from(l in Log, order_by: l.index)), Repo.all(TokenTransfer), Repo.all(MarketEvent)}
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
