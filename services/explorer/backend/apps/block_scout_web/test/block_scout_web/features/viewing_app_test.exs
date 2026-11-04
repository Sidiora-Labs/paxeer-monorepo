defmodule BlockScoutWeb.ViewingAppTest do
  @moduledoc """
  The former server-rendered loading bar is replaced by the indexing-status API
  and the V2 block channels consumed by IndexingBlocksAlert and IntTxsIndexingStatus
  in apps/explorer/ui/snippets. These five scenarios cover the same indexing
  transitions without depending on the retired layout's DOM selectors.
  """

  use BlockScoutWeb.ConnCase, async: false

  import Phoenix.ChannelTest

  alias BlockScoutWeb.Counters.{BlocksIndexedCounter, InternalTransactionsIndexedCounter}
  alias Explorer.Chain.{Block, PendingBlockOperation}
  alias Explorer.Chain.Cache.BlockNumber
  alias Explorer.Chain.Cache.Counters.{BlocksCount, PendingBlockOperationCount}
  alias Explorer.Repo

  setup do
    settings = [
      {:indexer, Indexer.Supervisor, enabled: true},
      {:indexer, Indexer.Fetcher.InternalTransaction.Supervisor, disabled?: false},
      {:indexer, Indexer.Fetcher.InternalTransaction, indexing_finished_threshold: 1},
      {:indexer, :block_ranges, "0..latest"},
      {:indexer, :trace_first_block, 0}
    ]

    for {app, key, value} <- settings do
      previous = Application.fetch_env(app, key)
      Application.put_env(app, key, value)

      on_exit(fn ->
        case previous do
          {:ok, old} -> Application.put_env(app, key, old)
          :error -> Application.delete_env(app, key)
        end
      end)
    end

    for {cache, key} <- [
          {BlockNumber, :min},
          {BlockNumber, :max},
          {BlocksCount, :count},
          {PendingBlockOperationCount, :count}
        ] do
      previous = ConCache.get(cache.cache_name(), key)
      on_exit(fn -> cache.set(key, previous) end)
    end

    :ok
  end

  test "shows blocks indexed percentage", %{conn: conn} do
    pending_chain(5..9)
    status = indexing_status(conn)

    assert status["indexed_blocks_ratio"] == "0.50"
    assert status["finished_indexing_blocks"] == false
    assert status["finished_indexing"] == false
    assert status["indexed_internal_transactions_ratio"] == nil
  end

  test "shows internal transactions loading after blocks finish", %{conn: conn} do
    pending_chain(0..9)
    status = indexing_status(conn)

    assert status["indexed_blocks_ratio"] == "1.00"
    assert status["finished_indexing_blocks"] == true
    assert status["finished_indexing"] == false
    assert status["indexed_internal_transactions_ratio"] == "0.90"
  end

  test "updates blocks indexed percentage", %{conn: conn} do
    pending_chain(5..9)
    join_indexing("indexing")

    assert indexing_status(conn)["indexed_blocks_ratio"] == "0.50"
    BlocksIndexedCounter.calculate_blocks_indexed_and_broadcast()
    assert_push("index_status", %{ratio: "0.50", finished: false})

    insert(:block, number: 4)
    refresh_counters()

    assert indexing_status(conn)["indexed_blocks_ratio"] == "0.60"
    BlocksIndexedCounter.calculate_blocks_indexed_and_broadcast()
    assert_push("index_status", %{ratio: "0.60", finished: false})
  end

  test "updates when blocks are fully indexed", %{conn: conn} do
    pending_chain(1..9)
    join_indexing("indexing")
    assert indexing_status(conn)["indexed_blocks_ratio"] == "0.90"

    insert(:block, number: 0)
    refresh_counters()
    BlocksIndexedCounter.calculate_blocks_indexed_and_broadcast()
    assert_push("index_status", %{ratio: "1.00", finished: true})

    status = indexing_status(conn)
    assert status["indexed_blocks_ratio"] == "1.00"
    assert status["finished_indexing_blocks"] == true
    assert status["finished_indexing"] == false
    assert status["indexed_internal_transactions_ratio"] == "0.90"
  end

  test "removes indexing indication when chain is indexed", %{conn: conn} do
    pending_chain(0..9)
    join_indexing("indexing_internal_transactions")
    assert indexing_status(conn)["finished_indexing"] == false

    InternalTransactionsIndexedCounter.calculate_internal_transactions_indexed()
    assert_push("index_status", %{ratio: "0.90", finished: false})

    Repo.delete_all(PendingBlockOperation)
    refresh_counters()
    InternalTransactionsIndexedCounter.calculate_internal_transactions_indexed()
    assert_push("index_status", %{ratio: "1.00", finished: true})

    status = indexing_status(conn)
    assert status["finished_indexing_blocks"] == true
    assert status["finished_indexing"] == true
    assert status["indexed_internal_transactions_ratio"] == "1.00"
  end

  defp pending_chain(range) do
    [block | _] = Enum.map(range, &insert(:block, number: &1))
    :transaction |> insert(input: Explorer.Factory.contract_code_info().tx_input) |> with_block(block)
    insert(:pending_block_operation, block_hash: block.hash, block_number: block.number)
    refresh_counters()
  end

  defp refresh_counters do
    BlockNumber.set_min(Repo.aggregate(Block, :min, :number))
    BlockNumber.set_max(Repo.aggregate(Block, :max, :number))
    BlocksCount.set_count(Repo.aggregate(Block, :count, :hash))
    PendingBlockOperationCount.set_count(Repo.aggregate(PendingBlockOperation, :count, :block_hash))
  end

  defp indexing_status(conn) do
    conn |> get("/api/v2/main-page/indexing-status") |> json_response(200)
  end

  defp join_indexing(channel) do
    assert {:ok, %{}, _socket} =
             BlockScoutWeb.V2.UserSocket
             |> socket("indexing-feature", %{})
             |> subscribe_and_join("blocks:#{channel}")
  end
end
