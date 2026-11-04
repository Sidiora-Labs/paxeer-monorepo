defmodule Explorer.Repair.Repair2233493Test do
  @moduledoc """
  Task 22.33.493, audit 22.30.151 repair record:
  * migration-cache-key-suffix: the three "cold migration flag" cases drive
    MigrationStatus persistence through the real cache, including restart.
  * minimum-gap-never-clears: "repaired gap advances the persisted frontier
    across scanner restarts" drives recorded block schema ingestion and the
    scanner's persisted LastFetchedCounter consumer.
  * unbounded-uniswap-pools: "evicted pool metadata is refetched without losing
    imported actions" replays the committed Blockscout client Swap recording
    through HTTP eth_call, the production transformer, schema and import runner;
    "concurrent pool insertions stay within capacity" proves the write bound.

  Baseline: the runner ledger contained no prior gates. No unchanged-tree test
  was run because the ordered instructions require complete implementation,
  then hf-build and hf-done, and prohibit running verify by hand. The gates
  record the candidate revision and private evidence. No out-of-path wiring
  changes are required for the cache repairs. Spec and GOTCHA ledgers are outside the allowed scope.
  Wiring request (explorer/indexer domain 14): services/explorer/backend/apps/indexer/lib/indexer/transform/transaction_actions.ex must support chain 125 before a recorded chain-125 Uniswap event can traverse parse_uniswap_v3; this cache test retains the committed supported-chain Swap recording.
  Gotcha: the pool/log recording lives outside the builder mounts, so the
  relevant recorded Swap response is embedded below with its source path.
  Qualification attempt 1 at b29ef0b5bcf2ae4b8e55e8c970ea57a9c5dbc5e0:
  five cases passed; pool RPC reads failed before HTTP because the fixture
  omitted the transport's required :http_options. Supply the real transport's
  full options; keep every assertion unchanged. Evidence: /out/logs/verify-1.log.
  Continued attempt at 81d51280fa73ce72e4bee30ef401562b3c277d2f:
  four of six cases passed; scanner restart lost SQL ownership and the pool
  replay returned no actions. This continuation explicitly shares the SQL
  checkout, supervises the scanner's block-number cache, and primes the actual
  pool cache before proving metadata refetch through the loopback transport.
  Continuation gate at 64f8767dd322fbcbd4acf64e827efb905497ac88:
  five of six cases passed; the pool replay reached the parser with token
  decimals loaded as Decimal structs from the database, which its integer
  bound rejects. Seed the token metadata cache with recorded RPC integers.
  Evidence: /out/logs/verify-1.log.
  Wiring request (explorer/indexer domain 14): services/explorer/backend/apps/indexer/lib/indexer/transform/transaction_actions.ex must normalize database Token.decimals before comparing it to the integer decimals limit.
  """

  use Explorer.DataCase, async: false

  alias Ecto.{Changeset, Multi}
  alias EthereumJSONRPC.Log
  alias Explorer.Chain
  alias Explorer.Chain.{Block, Import, TransactionAction}
  alias Explorer.Chain.Cache.{BackgroundMigrations, BlockNumber, ChainId, MinMissingBlockNumber}
  alias Explorer.Chain.Cache.TransactionActionUniswapPools, as: Pools
  alias Explorer.Chain.Cache.TransactionActionTokensData
  alias Explorer.Chain.Cache.Counters.LastFetchedCounter
  alias Explorer.Chain.Import.Runner.TransactionActions, as: ActionsRunner
  alias Explorer.Migrator.MigrationStatus

  alias Explorer.Migrator.HeavyDbIndexOperation.{
    DropTransactionsCreatedContractAddressHashWithPendingIndex,
    DropTransactionsFromAddressHashWithPendingIndex,
    DropTransactionsToAddressHashWithPendingIndex
  }

  @pool "0x8fd4596d4e7788a71f82daf4119d069a84e7d3f3"
  @token0 "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2"
  @token1 "0xf0a1fcf349b5a6f8987c225f94ccb28bef25a19b"
  @factory "0x1f98431c8ad98523631ae4a59f267346ea31f984"
  @pressure_pools [
    "0x88e6a0c2ddd26feeb64f039a2c41296fcb3f5640",
    "0x8ad599c3a0ff1de082011efddc58f1908eb6e6d8"
  ]
  @table :transaction_actions_uniswap_pools_cache

  # Source: services/explorer/services/libs/blockscout-client/crate/tests/recorded/
  # eth_blockscout_com/_api_v2_addresses_0x8FD4596d4E7788a71F82dAf4119D069a84E7d3f3_logs.json
  @swap Jason.decode!(
          ~S|{"block_hash":"0x2ebed1c3d4350037c2998042754b170f2c8dc381e9bda1653e839a299959e2c4","block_number":20077994,"data":"0xffffffffffffffffffffffffffffffffffffffffffffffffff9189753a8056de0000000000000000000000000000000000000000003d3da97c028ac02a77e4cb000000000000000000000000000000000000bec97b8d5d2f116085f2f224b569000000000000000000000000000000000000000000008a03f24683c2ced9d7020000000000000000000000000000000000000000000000000000000000034b81","index":311,"topics":["0xc42079f94a6350d7e6235f29174924f928cc2ac818eb64fed8004e115fbcca67","0x0000000000000000000000003fc91a3afd70395cd496c647d5a6cc9d4b2b7fad","0x0000000000000000000000003fc91a3afd70395cd496c647d5a6cc9d4b2b7fad",null],"transaction_hash":"0x2ff8028e8adfa590dfe7bf0318e30727d39c1169cf96c3b8a0463a64f5dc0b67","address":"0x8FD4596d4E7788a71F82dAf4119D069a84E7d3f3","decoded":{"method_call":"Swap(address indexed sender, address indexed recipient, int256 amount0, int256 amount1, uint160 sqrtPriceX96, uint128 liquidity, int24 tick)","method_id":"c42079f9","parameters":[{"indexed":true,"name":"sender","type":"address","value":"0x3fC91A3afd70395Cd496C647d5a6CC9D4B2b7FAD"},{"indexed":true,"name":"recipient","type":"address","value":"0x3fC91A3afd70395Cd496C647d5a6CC9D4B2b7FAD"},{"indexed":false,"name":"amount0","type":"int256","value":"-31092585829214498"},{"indexed":false,"name":"amount1","type":"int256","value":"74035665787553053536675019"},{"indexed":false,"name":"sqrtPriceX96","type":"uint160","value":"3869620922913981435538763113870697"},{"indexed":false,"name":"liquidity","type":"uint128","value":"651759372654110433728258"},{"indexed":false,"name":"tick","type":"int24","value":"215937"}]}}|
        )

  setup_all do
    suspend_application_scanner()
    :ok
  end

  setup do
    :ok = Ecto.Adapters.SQL.Sandbox.mode(Repo, {:shared, self()})

    cache = Enum.map(BackgroundMigrations.cache_keys(), &{&1, ConCache.get(BackgroundMigrations.cache_name(), &1)})

    on_exit(fn ->
      Enum.each(cache, fn {key, value} -> BackgroundMigrations.set(key, value) end)
    end)

    :ok
  end

  for {migration, key} <- [
        {DropTransactionsCreatedContractAddressHashWithPendingIndex,
         :heavy_indexes_drop_transactions_created_contract_address_hash_with_pending_index_finished},
        {DropTransactionsFromAddressHashWithPendingIndex,
         :heavy_indexes_drop_transactions_from_address_hash_with_pending_index_finished},
        {DropTransactionsToAddressHashWithPendingIndex,
         :heavy_indexes_drop_transactions_to_address_hash_with_pending_index_finished}
      ] do
    @migration migration
    @key key
    test "cold migration flag #{@key} recovers persisted status" do
      for {status, expected} <- [{"started", false}, {"completed", true}] do
        assert {:ok, %MigrationStatus{status: ^status}} =
                 MigrationStatus.set_status(@migration.migration_name(), status)

        restart_cache(BackgroundMigrations)
        assert ConCache.get(BackgroundMigrations.cache_name(), @key) == nil
        assert apply(BackgroundMigrations, String.to_existing_atom("get_#{@key}"), []) == expected
        assert ConCache.get(BackgroundMigrations.cache_name(), @key) == expected
        assert apply(BackgroundMigrations, String.to_existing_atom("get_#{@key}"), []) == expected
      end
    end
  end

  test "repaired gap advances the persisted frontier across scanner restarts" do
    configure(:explorer, MinMissingBlockNumber, batch_size: 1)
    assert :ok = Supervisor.terminate_child(Explorer.Supervisor, BlockNumber.child_id())
    start_supervised!(BlockNumber)
    on_exit(fn -> Supervisor.restart_child(Explorer.Supervisor, BlockNumber.child_id()) end)

    recording =
      Path.expand("../support/fixture/vcr_cassettes/import_block_perform_1_integer.json", __DIR__)
      |> File.read!()
      |> Jason.decode!()
      |> hd()
      |> get_in(["response", "body"])
      |> Jason.decode!()
      |> Map.fetch!("result")

    params = recording |> EthereumJSONRPC.Block.to_elixir() |> EthereumJSONRPC.Block.elixir_to_params()
    insert(:address, hash: params.miner_hash)
    insert(:block, number: 0, hash: params.parent_hash)
    second = insert(:block, number: 2, parent_hash: params.hash)
    insert(:block, number: 3, parent_hash: second.hash)
    BlockNumber.set_max(3)

    first_pid = start_supervised!(MinMissingBlockNumber)
    await_counter(1)
    stop_supervised!(MinMissingBlockNumber)
    refute Process.alive?(first_pid)

    changeset = Block.changeset(%Block{}, Map.put(params, :consensus, true))
    assert changeset.valid?, inspect(changeset.errors)
    assert {:ok, %Block{number: 1}} = Repo.insert(changeset)
    assert Chain.fetch_min_missing_block_cache(0, 2) == nil

    # A successful partial batch must retain the old gap until the full scan finishes.
    assert MinMissingBlockNumber.fetch_min_missing_block(-1) == 0
    assert Decimal.to_integer(LastFetchedCounter.get("min_missing_block_number")) == 1
    assert MinMissingBlockNumber.fetch_min_missing_block(0) == 1
    assert Decimal.to_integer(LastFetchedCounter.get("min_missing_block_number")) == 1

    repaired_pid = start_supervised!(MinMissingBlockNumber)
    await_counter(3)
    stop_supervised!(MinMissingBlockNumber)
    refute Process.alive?(repaired_pid)

    recovered_pid = start_supervised!(MinMissingBlockNumber)
    assert recovered_pid != repaired_pid
    assert Decimal.to_integer(LastFetchedCounter.get("min_missing_block_number")) == 3
    await_counter(3)
    assert Chain.fetch_min_missing_block_cache(0, 2) == nil

    stop_supervised!(MinMissingBlockNumber)
    second |> Changeset.change(refetch_needed: true) |> Repo.update!()
    start_supervised!(MinMissingBlockNumber)
    await_counter(2)
  end

  test "evicted pool metadata is refetched without losing imported actions" do
    configure(:explorer, Pools, max_cache_size: 2)
    restart_cache(Pools)
    configure(:indexer, Indexer.Fetcher.TransactionAction.Supervisor, enabled: true)

    configure(:indexer, Indexer.Fetcher.TransactionAction,
      aave_v3_pool: nil,
      uniswap_v3_factory: @factory,
      uniswap_v3_nft_position_manager: "0xc36442b4a4522e871399cd717abdd847ab11fe88"
    )

    chain_id = ConCache.get(ChainId.cache_name(), :id)
    ChainId.set_id(1)
    assert ChainId.get_id() == 1
    on_exit(fn -> ChainId.set_id(chain_id) end)

    # Pool immutable values are also present in the committed verified bytecode
    # fixture for this pool: WETH, token1, fee 500 and the Uniswap V3 factory.
    insert(:token, contract_address: insert(:address, hash: @token0), symbol: "WETH", decimals: 18)
    insert(:token, contract_address: insert(:address, hash: @token1), symbol: "MAGA", decimals: 18)
    insert(:transaction, hash: @swap["transaction_hash"])

    # The RPC metadata producer caches integer decimals. Database token rows
    # use Decimal, which the transformer's integer bound does not accept.
    for {address, symbol} <- [{@token0, "WETH"}, {@token1, "MAGA"}] do
      previous = TransactionActionTokensData.fetch_from_cache(address)
      on_exit(fn -> TransactionActionTokensData.put_to_cache(address, previous) end)
      assert TransactionActionTokensData.put_to_cache(address, %{symbol: symbol, decimals: 18})
      assert TransactionActionTokensData.fetch_from_cache(address) == %{symbol: symbol, decimals: 18}
    end

    server = Bypass.open()
    observer = self()

    Bypass.expect(server, "POST", "/", fn conn ->
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      requests = Jason.decode!(body)
      assert is_list(requests)

      responses =
        Enum.map(requests, fn request ->
          assert %{"jsonrpc" => "2.0", "id" => id, "method" => "eth_call", "params" => [call, "latest"]} = request
          to = String.downcase(call["to"])
          data = call["data"]
          send(observer, {:eth_call, to, data})

          result =
            case {to, data} do
              {@pool, "0x0dfe1681"} ->
                abi_address(@token0)

              {@pool, "0xd21220a7"} ->
                abi_address(@token1)

              {@pool, "0xddca3f43"} ->
                "0x" <> String.pad_leading("1f4", 64, "0")

              {@factory, "0x1698ee82" <> arguments} ->
                assert arguments ==
                         String.trim_leading(abi_address(@token0), "0x") <>
                           String.trim_leading(abi_address(@token1), "0x") <> String.pad_leading("1f4", 64, "0")

                abi_address(@pool)
            end

          %{jsonrpc: "2.0", id: id, result: result}
        end)

      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.resp(200, Jason.encode!(responses))
    end)

    configure(:explorer, :json_rpc_named_arguments,
      transport: EthereumJSONRPC.HTTP,
      transport_options: [
        urls: ["http://127.0.0.1:#{server.port}"],
        fallback_urls: [],
        http: EthereumJSONRPC.HTTP.HTTPoison,
        http_options: []
      ],
      variant: EthereumJSONRPC.Geth
    )

    log =
      %{
        "address" => @swap["address"],
        "blockHash" => @swap["block_hash"],
        "blockNumber" => EthereumJSONRPC.integer_to_quantity(@swap["block_number"]),
        "data" => @swap["data"],
        "logIndex" => EthereumJSONRPC.integer_to_quantity(@swap["index"]),
        "topics" => @swap["topics"],
        "transactionHash" => @swap["transaction_hash"],
        "transactionIndex" => "0x0"
      }
      |> Log.to_elixir()
      |> Log.elixir_to_params()

    assert Pools.fetch_from_cache(@pool) == nil
    assert Pools.put_to_cache(@pool, [@token0, @token1])
    assert %{transaction_actions: [action]} = Indexer.Transform.TransactionActions.parse([log])
    refute_received {:eth_call, _, _}

    for pool <- @pressure_pools do
      assert Pools.put_to_cache(pool, [@token0, @token1])
      assert :ets.info(@table, :size) <= 2
    end

    assert Pools.fetch_from_cache(@pool) == nil
    assert %{transaction_actions: [^action]} = Indexer.Transform.TransactionActions.parse([log])
    assert_pool_calls()
    assert Pools.fetch_from_cache(@pool) == [@token0, @token1]
    assert action.type == "swap"
    assert action.data.amount0 == "74035665.787553053536675019"
    assert action.data.amount1 == "0.031092585829214498"
    assert action.data.block_number == @swap["block_number"]
    assert action.hash == @swap["transaction_hash"]

    changeset = TransactionAction.changeset(%TransactionAction{}, action)
    assert changeset.valid?, inspect(changeset.errors)
    changes = Map.take(Map.from_struct(Changeset.apply_changes(changeset)), Map.keys(changeset.changes))

    assert {:ok, %{insert_transaction_actions: [imported]}} =
             Multi.new() |> ActionsRunner.run([changes], %{timestamps: Import.timestamps()}) |> Repo.transaction()

    assert %{transaction_actions: [^action]} = Indexer.Transform.TransactionActions.parse([log])
    refute_received {:eth_call, _, _}

    for pool <- @pressure_pools do
      assert Pools.put_to_cache(pool, [@token0, @token1])
      assert :ets.info(@table, :size) <= 2
    end

    assert :ets.info(@table, :size) == 2
    assert Pools.fetch_from_cache(@pool) == nil
    assert %{transaction_actions: [^action]} = Indexer.Transform.TransactionActions.parse([log])
    assert_pool_calls()
    assert :ets.info(@table, :size) == 2
    assert Repo.get_by!(TransactionAction, hash: imported.hash, log_index: imported.log_index).data == imported.data
  end

  test "concurrent pool insertions stay within capacity" do
    configure(:explorer, Pools, max_cache_size: 2)
    restart_cache(Pools)

    # Updating a cached negative result must not create a second eviction entry.
    assert Pools.put_to_cache(@pool, [])
    assert Pools.put_to_cache(@pool, [@token0, @token1])
    assert Pools.put_to_cache(hd(@pressure_pools), [])
    assert Pools.put_to_cache(List.last(@pressure_pools), [])
    assert Pools.fetch_from_cache(@pool) == nil
    assert Pools.fetch_from_cache(hd(@pressure_pools)) == []

    1..64
    |> Task.async_stream(
      fn n ->
        address = "0x" <> (n |> Integer.to_string(16) |> String.pad_leading(40, "0"))
        assert Pools.put_to_cache(address, [@token0, @token1])
        assert :ets.info(@table, :size) <= 2
      end,
      max_concurrency: 16
    )
    |> Enum.each(fn result -> assert {:ok, _} = result end)

    assert :ets.info(@table, :size) == 2
    restart_cache(Pools)
    assert :ets.info(@table, :size) == 0
    assert Pools.fetch_from_cache(@pool) == nil
    assert Pools.put_to_cache(@pool, [@token0, @token1])
    assert Pools.fetch_from_cache(@pool) == [@token0, @token1]
  end

  defp abi_address("0x" <> address), do: "0x" <> String.pad_leading(address, 64, "0")

  defp assert_pool_calls do
    for selector <- ["0x0dfe1681", "0xd21220a7", "0xddca3f43"] do
      assert_received {:eth_call, @pool, ^selector}
    end

    assert_received {:eth_call, @factory, "0x1698ee82" <> _}
    refute_received {:eth_call, _, _}
  end

  defp restart_cache(module) do
    id = if module == BackgroundMigrations, do: module.child_id(), else: module
    assert :ok = Supervisor.terminate_child(Explorer.Supervisor, id)
    assert {:ok, pid} = Supervisor.restart_child(Explorer.Supervisor, id)
    pid
  end

  defp configure(app, key, value) do
    original = Application.fetch_env(app, key)
    Application.put_env(app, key, value)

    on_exit(fn ->
      case original do
        {:ok, config} -> Application.put_env(app, key, config)
        :error -> Application.delete_env(app, key)
      end

      if key == Pools, do: restart_cache(Pools)
    end)
  end

  defp suspend_application_scanner do
    case Process.whereis(MinMissingBlockNumber) do
      nil ->
        :ok

      _ ->
        assert :ok = Supervisor.terminate_child(Explorer.Supervisor, MinMissingBlockNumber)

        on_exit(fn ->
          :ok = Ecto.Adapters.SQL.Sandbox.mode(Repo, :auto)
          Supervisor.restart_child(Explorer.Supervisor, MinMissingBlockNumber)
        end)
    end
  end

  defp await_counter(expected, attempts \\ 100)

  defp await_counter(expected, 0) do
    assert Decimal.to_integer(LastFetchedCounter.get("min_missing_block_number")) == expected
  end

  defp await_counter(expected, attempts) do
    if Decimal.to_integer(LastFetchedCounter.get("min_missing_block_number")) != expected do
      Process.sleep(10)
      await_counter(expected, attempts - 1)
    end
  end
end
