defmodule Indexer.Repair2233456Test do
  @moduledoc """
  Task 22.33.456 repair record (audit 22.30.148):
  * coin-refresh-acknowledges-error: failed refresh retries its original work and survives reopening after import.
  * creator-pending-key-shape: repeated creator requests share searches and one block import.
  * internal-trace-page-count: cursor pages refill using only address and direction eligible traces.
  * erc404-current-balance-kind: current and historic ERC-404 requests use the same ABI operations.
  * balance-notification-address-key: notifications compare each holder with its own previous balance.

  Recorded JSON-RPC block and call-tracer shapes are normalized to the isolated
  chain below. The loopback peer serves exact request/response transcripts; the
  production HTTP transport, decoders, BalanceReader, schemas, import runners,
  BufferedTask queue and event publisher execute unchanged.

  Baseline verification was not run: the explicit gate order requires complete
  implementation before hf-build/hf-done and prohibits a manual verify run.
  No baseline failure is claimed. The runner ledger records the exact qualified
  revision and private evidence. No out-of-scope files are changed.
  Wiring request (domain 14): services/explorer/backend/apps/explorer/lib/explorer/chain/import/runner/address/token_balances.ex should distinguish fetched ERC-404 id values from transfer placeholders; the on-demand path persists authoritative responses through TokenBalance.changeset/2.
  """
  use ExUnit.Case, async: false
  use Explorer.DataCase

  alias Explorer.Chain.Address.{CoinBalance, CurrentTokenBalance, TokenBalance}
  alias Explorer.Chain.Cache.{BlockNumber, Counters.AverageBlockTime}
  alias Explorer.Chain.Events.Subscriber
  alias Explorer.Chain.Wei
  alias Explorer.PagingOptions
  alias Indexer.BufferedTask
  alias Indexer.Fetcher.OnDemand.CoinBalance, as: CoinFetcher
  alias Indexer.Fetcher.OnDemand.ContractCreator, as: CreatorFetcher
  alias Indexer.Fetcher.OnDemand.InternalTransaction, as: TraceFetcher
  alias Indexer.Fetcher.OnDemand.TokenBalance, as: TokenFetcher
  alias Indexer.Fetcher.InternalTransaction, as: InternalFetcher

  @moduletag :capture_log
  @other "0x4200000000000000000000000000000000000015"
  @third "0x6df83a19647a398d48e77a6835f4a28eb7e2f7c0"

  setup do
    keys = [
      {:explorer, :json_rpc_named_arguments},
      {:indexer, :json_rpc_named_arguments},
      {:explorer, AverageBlockTime},
      {:indexer, TokenFetcher},
      {:ethereum_jsonrpc, EthereumJSONRPC.Geth},
      {:indexer, Indexer.Fetcher.InternalTransaction.Supervisor}
    ]

    saved = Enum.map(keys, fn {app, key} -> {app, key, Application.fetch_env(app, key)} end)

    on_exit(fn ->
      Enum.each(saved, fn
        {app, key, {:ok, value}} -> Application.put_env(app, key, value)
        {app, key, :error} -> Application.delete_env(app, key)
      end)
    end)

    store = start_supervised!({Agent, fn -> %{transcript: %{}, status: 200, requests: []} end})
    server = Bypass.open()

    Bypass.expect(server, "POST", "/", fn conn ->
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      request = Jason.decode!(body)

      {status, response} =
        Agent.get_and_update(store, fn state ->
          response = fn request ->
            %{"id" => id, "jsonrpc" => "2.0", "method" => method, "params" => params} = request
            result = Map.fetch!(state.transcript, {method, params})
            Map.merge(result, %{"id" => id, "jsonrpc" => "2.0"})
          end

          result = if is_list(request), do: request |> Enum.map(response) |> Enum.reverse(), else: response.(request)
          {{state.status, result}, %{state | requests: state.requests ++ List.wrap(request)}}
        end)

      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.resp(status, Jason.encode!(response))
    end)

    rpc = [
      variant: EthereumJSONRPC.Geth,
      transport: EthereumJSONRPC.HTTP,
      transport_options: [
        http: EthereumJSONRPC.HTTP.HTTPoison,
        urls: ["http://127.0.0.1:#{server.port}/"],
        fallback_urls: [],
        http_options: [timeout: 1_000, recv_timeout: 1_000]
      ]
    ]

    Application.put_env(:explorer, :json_rpc_named_arguments, rpc)
    Application.put_env(:indexer, :json_rpc_named_arguments, rpc)
    Application.put_env(:explorer, AverageBlockTime, enabled: false)
    Application.put_env(:indexer, TokenFetcher, fallback_threshold_in_blocks: 1)

    Application.put_env(:ethereum_jsonrpc, EthereumJSONRPC.Geth,
      tracer: "call_tracer",
      debug_trace_timeout: "5s",
      block_traceable?: true
    )

    %{store: store, rpc: rpc, server: server}
  end

  test "failed refresh retries its original work and survives reopening after import", %{
    store: store,
    rpc: rpc,
    server: server
  } do
    address = insert(:address)
    insert(:block, number: 2)
    insert(:block, number: 4)
    assert BlockNumber.get_max() == 4
    hash = to_string(address.hash)

    transcript(store, %{
      {"eth_getBalance", [hash, "0x2"]} => %{"result" => "0x2a"},
      {"eth_getBlockByNumber", ["0x2", true]} => %{"result" => recorded_block(2)}
    })

    entries = [{:fetch_and_import, 2, hash}, {:fetch_and_import_daily_balances, 2, hash}]
    :ok = Bypass.down(server)
    before = System.monotonic_time(:millisecond)
    assert {:retry, ^entries} = CoinFetcher.run(entries, rpc)
    assert System.monotonic_time(:millisecond) - before >= 1_000
    assert Repo.aggregate(CoinBalance, :count) == 0

    start_supervised!({Task.Supervisor, name: CoinFetcher.TaskSupervisor})
    original = start_coin_fetcher(rpc)
    await(fn -> GenServer.call(original, :state).init_task == :complete end)
    assert :ok = BufferedTask.buffer(original, entries, false)
    send(original, :flush)

    await(fn ->
      state = GenServer.call(original, :state)
      Enum.to_list(state.bound_queue) == entries and state.task_ref_to_batch == %{}
    end)

    :ok = Bypass.up(server)
    send(original, :flush)
    await(fn -> Repo.aggregate(CoinBalance, :count) == 1 end)
    await(fn -> BufferedTask.debug_count(original) == %{buffer: 0, tasks: 0} end)

    assert %CoinBalance{block_number: 2, value: %Wei{value: value}, value_fetched_at: fetched_at} =
             Repo.one!(CoinBalance)

    assert Decimal.equal?(value, 42)
    refute is_nil(fetched_at)
    assert Repo.aggregate(Explorer.Chain.Address.CoinBalanceDaily, :count) == 1
    assert Enum.all?(requests(store, "eth_getBalance"), &(&1["params"] == [hash, "0x2"]))

    assert :ok = stop_supervised(CoinFetcher)
    refute Process.alive?(original)
    reopened = start_coin_fetcher(rpc)
    refute reopened == original
    await(fn -> GenServer.call(reopened, :state).init_task == :complete end)
    assert Repo.one!(CoinBalance).value == %Wei{value: value}
    assert BufferedTask.debug_count(reopened) == %{buffer: 0, tasks: 0}
  end

  test "coin refresh retains each failed operation from a partial RPC batch", %{store: store, rpc: rpc} do
    first = insert(:address)
    second = insert(:address)
    insert(:block, number: 4)
    assert BlockNumber.get_max() == 4
    first_hash = to_string(first.hash)
    second_hash = to_string(second.hash)
    failed_entries = [{:fetch_and_import, 2, second_hash}, {:fetch_and_import_daily_balances, 2, second_hash}]

    transcript(store, %{
      {"eth_getBalance", [first_hash, "0x2"]} => %{"result" => "0x2a"},
      {"eth_getBalance", [second_hash, "0x2"]} => %{
        "error" => %{"code" => -32_000, "message" => "historical state temporarily unavailable"}
      }
    })

    assert {:retry, ^failed_entries} = CoinFetcher.run([{:fetch_and_update, 2, first_hash} | failed_entries], rpc)
    assert Repo.get!(Explorer.Chain.Address, first.hash).fetched_coin_balance == %Wei{value: Decimal.new(42)}
    assert Repo.aggregate(CoinBalance, :count) == 0

    transcript(store, %{
      {"eth_getBalance", [second_hash, "0x2"]} => %{"result" => "0x2b"},
      {"eth_getBlockByNumber", ["0x2", true]} => %{"result" => recorded_block(2)}
    })

    assert :ok = CoinFetcher.run(failed_entries, rpc)
    assert Repo.one!(CoinBalance).address_hash == second.hash
    assert Repo.one!(CoinBalance).value == %Wei{value: Decimal.new(43)}
  end

  test "repeated creator requests share searches and one block import", %{store: store, rpc: rpc} do
    blocks = for number <- 0..4, do: insert(:block, number: number, refetch_needed: number == 3)

    contracts =
      for _ <- 1..2 do
        insert(:address, contract_code: "0x1234")
        |> Repo.preload([:contract_creation_transaction, :contract_creation_internal_transaction])
      end

    assert BlockNumber.get_max() == 4

    transcript(
      store,
      Map.new(
        for address <- contracts, {block, nonce} <- [{"0x2", "0x0"}, {"0x3", "0x1"}] do
          {{"eth_getTransactionCount", [to_string(address.hash), block]}, %{"result" => nonce}}
        end
      )
    )

    creator = start_supervised!(CreatorFetcher)
    assert :ets.info(:contract_creator_lookup, :owner) == creator
    :sys.suspend(creator)

    for address <- contracts do
      assert :ok = CreatorFetcher.trigger_fetch(address)
      for _ <- 1..20, do: assert(:ignore == CreatorFetcher.trigger_fetch(address))
    end

    :sys.resume(creator)
    :sys.get_state(creator)

    for address <- contracts do
      hash = to_string(address.hash)
      assert :ets.lookup(:contract_creator_lookup, hash) == [{hash, {:pending_import, 3}}]
      for _ <- 1..20, do: assert(:ignore == CreatorFetcher.trigger_fetch(address))
    end

    assert length(requests(store, "eth_getTransactionCount")) == 4
    [{"pending_blocks", pending}] = :ets.lookup(:contract_creator_lookup, "pending_blocks")

    assert Enum.sort(pending) ==
             Enum.sort(Enum.map(contracts, &%{block_number: 3, address_hash_string: to_string(&1.hash)}))

    assert [%{from_number: 3, to_number: 3}] = Repo.all(Explorer.Utility.MissingBlockRange)

    Application.put_env(:indexer, Indexer.Fetcher.InternalTransaction.Supervisor, disabled?: false)
    start_supervised!({Task.Supervisor, name: InternalFetcher.TaskSupervisor})

    importer =
      start_supervised!(
        {InternalFetcher,
         [
           [json_rpc_named_arguments: rpc, poll: false, flush_interval: 60_000, max_concurrency: 0],
           [name: InternalFetcher]
         ]}
      )

    await(fn -> GenServer.call(importer, :state).init_task == :complete end)
    assert BufferedTask.debug_count(importer) == %{buffer: 0, tasks: 0}
    imported = %{blocks: [Enum.at(blocks, 3)]}
    assert :ok = imported |> CreatorFetcher.async_update_cache_of_contract_creator_on_demand() |> Task.await()
    assert :ets.lookup(:contract_creator_lookup, "pending_blocks") == [{"pending_blocks", []}]
    for address <- contracts, do: assert([] == :ets.lookup(:contract_creator_lookup, to_string(address.hash)))
    assert GenServer.call(importer, :state).current_front_buffer == [[3]]
    assert :ok = imported |> CreatorFetcher.async_update_cache_of_contract_creator_on_demand() |> Task.await()
    assert GenServer.call(importer, :state).current_front_buffer == [[3]]
    assert :ok = stop_supervised(CreatorFetcher)
    assert :ets.info(:contract_creator_lookup) == :undefined
    restarted = start_supervised!(CreatorFetcher)
    assert :ets.info(:contract_creator_lookup, :owner) == restarted
  end

  test "cursor pages refill using only address and direction eligible traces", %{store: store} do
    address = insert(:address)
    hash = to_string(address.hash)
    id = insert(:address_id_to_address_hash, address_hash: address.hash).address_id

    for number <- 1..3 do
      block = insert(:block, number: number)
      insert(:transaction) |> with_block(block, index: 0)

      insert(:deleted_internal_transactions_address_placeholder,
        address_id: id,
        block_number: number,
        count_tos: 20,
        count_froms: 20
      )
    end

    traces = %{
      3 => trace([call(@other, hash), call(@other, @third), call(@other, hash), call(hash, @third)]),
      2 => trace([call(@other, hash)]),
      1 => trace([call(@other, hash)])
    }

    transcript(
      store,
      Map.new(traces, fn {number, traces} ->
        {{"debug_traceBlockByNumber",
          [EthereumJSONRPC.integer_to_quantity(number), %{"tracer" => "callTracer", "timeout" => "5s"}]},
         %{"result" => traces}}
      end)
    )

    result =
      TraceFetcher.fetch_by_address(address.hash,
        direction: :to,
        from_block: 1,
        paging_options: %PagingOptions{key: {3, 0, 2}, page_size: 3}
      )

    assert Enum.map(result, &{&1.block_number, &1.index}) == [{3, 1}, {2, 1}, {1, 1}]
    assert Enum.all?(result, &(&1.to_address_hash == address.hash and &1.block.number == &1.block_number))
    assert Enum.map(requests(store, "debug_traceBlockByNumber"), &hd(&1["params"])) == ["0x3", "0x2", "0x1"]

    result =
      TraceFetcher.fetch_by_address(address.hash,
        direction: :to_address_hash,
        from_block: 1,
        paging_options: %PagingOptions{key: {3, 0, 1}, page_size: 2}
      )

    assert Enum.map(result, &{&1.block_number, &1.index}) == [{2, 1}, {1, 1}]
  end

  test "current and historic ERC-404 requests use the same ABI operations", %{store: store} do
    insert(:block, number: 100)
    address = insert(:address)
    token = insert(:token, type: "ERC-404")

    for id <- [nil, Decimal.new(7)] do
      insert(:address_current_token_balance,
        address: address,
        token_contract_address_hash: token.contract_address_hash,
        token_type: "ERC-404",
        token_id: id,
        block_number: 1,
        value: 0
      )
    end

    transcript(
      store,
      Map.new(
        for block <- ["0x64", "0x2"], {id, value} <- [{nil, 42}, {7, 3}] do
          {{"eth_call",
            [%{"to" => to_string(token.contract_address_hash), "data" => calldata(address.hash, id)}, block]},
           %{"result" => uint256(value)}}
        end
      )
    )

    Subscriber.to(:address_token_balances, :on_demand)

    assert :ok =
             TokenFetcher.run(
               [
                 {:fetch, address.hash},
                 {:historic_fetch, {address.hash, token.contract_address_hash, "ERC-404", nil, 2}},
                 {:historic_fetch, {address.hash, token.contract_address_hash, "ERC-404", Decimal.new(7), 2}}
               ],
               nil
             )

    current = Repo.all(CurrentTokenBalance)
    historical = Repo.all(TokenBalance)

    for balances <- [current, historical] do
      assert length(balances) == 2
      assert Enum.find(balances, &is_nil(&1.token_id)).value == Decimal.new(42)
      assert Enum.find(balances, &(not is_nil(&1.token_id))).value == Decimal.new(3)
    end

    assert length(requests(store, "eth_call")) == 4
    assert Enum.all?(current, &(&1.block_number == 100 and not is_nil(&1.value_fetched_at)))
    assert Enum.all?(historical, &(&1.block_number == 2 and not is_nil(&1.value_fetched_at)))

    assert_receive {:chain_event, :address_token_balances, :on_demand, notified}, 1_000
    assert Enum.sort_by(notified, & &1.token_id) == Enum.sort_by(historical, & &1.token_id)

    transcript(store, %{
      {"eth_call", [%{"to" => to_string(token.contract_address_hash), "data" => calldata(address.hash, 7)}, "0x2"]} =>
        %{"result" => uint256(4)}
    })

    assert :ok =
             TokenFetcher.run(
               [{:historic_fetch, {address.hash, token.contract_address_hash, "ERC-404", Decimal.new(7), 2}}],
               nil
             )

    historical = Repo.all(TokenBalance)
    assert length(historical) == 2
    assert Enum.find(historical, &(not is_nil(&1.token_id))).value == Decimal.new(4)
    assert Enum.find(historical, &is_nil(&1.token_id)).value == Decimal.new(42)
  end

  test "notifications compare each holder with its own previous balance", %{store: store} do
    insert(:block, number: 100)
    holders = [insert(:address), insert(:address)]
    token = insert(:token, type: "ERC-20")

    for {holder, old} <- Enum.zip(holders, [10, 20]) do
      insert(:address_current_token_balance,
        address: holder,
        token_contract_address_hash: token.contract_address_hash,
        token_type: "ERC-20",
        block_number: 1,
        value: old
      )
    end

    transcript(
      store,
      Map.new(holders, fn holder ->
        {{"eth_call",
          [%{"to" => to_string(token.contract_address_hash), "data" => calldata(holder.hash, nil)}, "0x64"]},
         %{"result" => uint256(20)}}
      end)
    )

    Subscriber.to(:address_current_token_balances, :on_demand)
    assert :ok = TokenFetcher.run(Enum.map(holders, &{:fetch, &1.hash}), nil)
    changed_hash = to_string(hd(holders).hash)

    assert_receive {:chain_event, :address_current_token_balances, :on_demand,
                    %{address_hash: ^changed_hash, address_current_token_balances: [changed]}},
                   1_000

    assert changed.address_hash == hd(holders).hash
    assert changed.value == Decimal.new(20)
    refute_receive {:chain_event, :address_current_token_balances, :on_demand, _}, 100
    assert length(requests(store, "eth_call")) == 2
    assert Enum.all?(Repo.all(CurrentTokenBalance), &(&1.value == Decimal.new(20) and &1.block_number == 100))
  end

  defp start_coin_fetcher(rpc) do
    start_supervised!(
      {CoinFetcher,
       [[json_rpc_named_arguments: rpc, poll: false, flush_interval: 60_000, max_concurrency: 1], [name: CoinFetcher]]}
    )
  end

  defp transcript(store, values), do: Agent.update(store, &%{&1 | transcript: values})

  defp requests(store, method),
    do: Agent.get(store, fn state -> Enum.filter(state.requests, &(&1["method"] == method)) end)

  defp await(condition, attempts \\ 500) do
    if condition.() do
      :ok
    else
      assert attempts > 0, "production component did not reach the expected state"
      Process.sleep(10)
      await(condition, attempts - 1)
    end
  end

  defp calldata(hash, nil), do: "0x70a08231" <> String.pad_leading(String.slice(to_string(hash), 2..-1//1), 64, "0")

  defp calldata(hash, id),
    do:
      "0x00fdd58e" <>
        String.pad_leading(String.slice(to_string(hash), 2..-1//1), 64, "0") <>
        String.pad_leading(Integer.to_string(id, 16), 64, "0")

  defp uint256(value), do: "0x" <> String.pad_leading(String.downcase(Integer.to_string(value, 16)), 64, "0")

  defp call(from, to) do
    %{
      "type" => "CALL",
      "from" => from,
      "to" => to,
      "value" => "0x0",
      "gas" => "0xe9a3c",
      "gasUsed" => "0x4a28",
      "input" => "0x",
      "output" => "0x"
    }
  end

  defp trace(calls) do
    [
      %{
        "txHash" => "0x32b17f27ddb546eab3c4c33f31eb22c1cb992d4ccc50dae26922805b717efe5c",
        "result" => Map.put(call(@other, @third), "calls", calls)
      }
    ]
  end

  defp recorded_block(number) do
    path =
      Path.expand("../../../explorer/test/support/fixture/vcr_cassettes/block_importer_import_1_pending.json", __DIR__)

    [record] = path |> File.read!() |> Jason.decode!()
    block = record["response"]["body"] |> Jason.decode!() |> Map.fetch!("result")
    Map.merge(block, %{"number" => EthereumJSONRPC.integer_to_quantity(number), "transactions" => []})
  end
end
