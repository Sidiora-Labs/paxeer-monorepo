unless Code.ensure_loaded?(Indexer.Transform.Stability.Validators) do
  Code.require_file(Path.expand("../../../indexer/lib/indexer/transform/stability/validators.ex", __DIR__))
end

defmodule Explorer.Repair.Repair2233590Test do
  @moduledoc """
  Task 22.33.590 closes audit 22.30.160 / validator-replay-count with:
  * "duplicate recorded ranges reconcile instead of incrementing"
  * "restart replay recovers the persisted canonical total"
  * "late validator metadata recovers retained blocks on an empty retry"
  * "consensus changes remove orphaned contributions"
  * "counter reconciliation rolls back with its import transaction"
  * "concurrent range replays preserve the canonical total"

  The committed consecutive-block RPC recording passes through the real HTTP
  transport, block decoder, block schema, validator schema and import runner.
  Recovery stops and reopens the actual repository pool against committed rows.
  The isolated schema copies production block tables and the Stability validator
  migrations, independent of the qualification builder's selected chain.

  Baseline: no scoped changes or earlier gate ledger existed. The explicit
  implementation-first gate order supersedes the older unchanged-tree verify
  step; no baseline test result is claimed. hf-build and hf-done retain revision,
  command, exit code and private evidence. No out-of-path wiring is required.
  The legacy Stability runner tests assume additive counters without persisted
  blocks; their expectations conflict with this audit's replay-safe contract.
  """

  use ExUnit.Case, async: false

  alias Ecto.Multi
  alias Ecto.Adapters.SQL.Sandbox
  alias Explorer.Chain.{Address, Block, Import}
  alias Explorer.Chain.Import.Runner.Stability.Validators
  alias Explorer.Chain.Stability.Validator
  alias Explorer.Repo

  @recording Path.expand("../../../ethereum_jsonrpc/test/support/fixture/paxeer_x/consecutive_blocks.json", __DIR__)
  @external_resource @recording
  @miner "0xb3ad5a835b418958a9142d8681bfb4b81d2cf270"

  setup do
    original_chain_type = Application.fetch_env!(:explorer, :chain_type)
    Application.put_env(:explorer, :chain_type, :stability)
    original_repo = Repo.get_dynamic_repo()
    schema = "repair_2233590_#{System.unique_integer([:positive])}"
    Repo.query!("CREATE SCHEMA #{schema}")

    on_exit(fn ->
      Application.put_env(:explorer, :chain_type, original_chain_type)
      Repo.put_dynamic_repo(original_repo)
      Repo.query!("DROP SCHEMA #{schema} CASCADE")
    end)

    for table <- ["addresses", "blocks"] do
      Repo.query!("CREATE TABLE #{schema}.#{table} (LIKE public.#{table} INCLUDING ALL)")
    end

    # Schema from priv/stability/migrations/20240203091010_add_stability_validators.exs
    # and 20250602163043_add_validator_blocks_validated_counter.exs.
    Repo.query!("""
    CREATE TABLE #{schema}.validators_stability (
      address_hash bytea PRIMARY KEY,
      state integer DEFAULT 0,
      blocks_validated integer NOT NULL,
      inserted_at timestamp NOT NULL,
      updated_at timestamp NOT NULL
    )
    """)

    repo = open_repo(schema)
    {:ok, blocks: fetch_recorded_blocks(), schema: schema, repo: repo}
  end

  test "duplicate recorded ranges reconcile instead of incrementing", %{blocks: blocks} do
    store_validator(19)
    persist_blocks(blocks)
    changes = validator_changes(blocks)
    assert [change] = changes
    assert change.blocks_validated == 2

    assert_total(run_changes(changes), 2)
    persist_blocks(blocks)
    assert_total(run_changes(changes), 2)
    assert_total(run_changes(changes ++ changes), 2)
    assert Repo.aggregate(Block, :count) == 2
    assert Repo.get!(Validator, @miner).blocks_validated == 2
  end

  test "restart replay recovers the persisted canonical total", ctx do
    store_validator(0)
    persist_blocks(ctx.blocks)
    assert_total(run_changes(validator_changes(ctx.blocks)), 2)
    old_repo = ctx.repo
    reopen_repo(ctx.schema, old_repo)

    assert Repo.aggregate(Block, :count) == 2
    assert Repo.get!(Validator, @miner).blocks_validated == 2
    replay = fetch_recorded_blocks()
    persist_blocks(replay)
    assert_total(run_changes(validator_changes(replay)), 2)
    assert Repo.get!(Validator, @miner).state == :probation
  end

  test "late validator metadata recovers retained blocks on an empty retry", ctx do
    persist_blocks(ctx.blocks)
    assert {:ok, %{stability_validators: []}} = run_changes(validator_changes(ctx.blocks))
    assert Repo.get(Validator, @miner) == nil
    assert Repo.aggregate(Block, :count) == 2
    reopen_repo(ctx.schema, ctx.repo)

    store_validator(0)
    assert Repo.get!(Validator, @miner).blocks_validated == 0
    assert_total(run_changes([]), 2)
    assert_total(run_changes(validator_changes(ctx.blocks)), 2)
    assert Repo.aggregate(Block, :count) == 2
  end

  test "consensus changes remove orphaned contributions", %{blocks: blocks} do
    store_validator(0)
    persist_blocks(blocks)
    assert_total(run_changes(validator_changes(blocks)), 2)

    for {block, expected} <- Enum.zip(blocks, [1, 0]) do
      {:ok, hash} = Explorer.Chain.Hash.Full.cast(block.hash)
      Repo.get!(Block, hash) |> Block.changeset(%{consensus: false}) |> Repo.update!()
      assert_total(run_changes([]), expected)
    end

    assert Repo.aggregate(Block, :count) == 2
    assert_total(run_changes(validator_changes(blocks)), 0)
    persist_blocks(blocks)
    assert_total(run_changes(validator_changes(blocks)), 2)
  end

  test "counter reconciliation rolls back with its import transaction", %{blocks: blocks} do
    store_validator(0)
    persist_blocks(blocks)

    multi =
      validator_multi(validator_changes(blocks))
      |> Multi.run(:reject_range, fn _, _ -> {:error, :rejected} end)

    assert {:error, :reject_range, :rejected, %{stability_validators: [%Validator{blocks_validated: 2}]}} =
             Repo.transaction(multi)

    assert Repo.get!(Validator, @miner).blocks_validated == 0
    assert_total(run_changes(validator_changes(blocks)), 2)
  end

  test "concurrent range replays preserve the canonical total", %{blocks: blocks, repo: repo} do
    store_validator(0)
    persist_blocks(blocks)
    changes = validator_changes(blocks)

    tasks =
      for _ <- 1..4 do
        Task.async(fn ->
          Repo.put_dynamic_repo(repo)
          run_changes(changes)
        end)
      end

    for task <- tasks, do: assert_total(Task.await(task, 10_000), 2)
    assert Repo.get!(Validator, @miner).blocks_validated == 2
  end

  defp open_repo(schema) do
    repo = start_supervised!({Repo, name: nil, pool_size: 4, parameters: [search_path: schema]})
    Sandbox.mode(repo, :auto)
    Repo.put_dynamic_repo(repo)
    repo
  end

  defp reopen_repo(schema, old_repo) do
    monitor = Process.monitor(old_repo)
    assert :ok = stop_supervised(Repo)
    assert_receive {:DOWN, ^monitor, :process, ^old_repo, :shutdown}
    refute Process.alive?(old_repo)
    repo = open_repo(schema)
    refute repo == old_repo
    repo
  end

  defp fetch_recorded_blocks do
    recorded = @recording |> File.read!() |> Jason.decode!()
    bypass = Bypass.open()
    owner = self()

    Bypass.expect_once(bypass, "POST", "/", fn conn ->
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      requests = Jason.decode!(body)

      assert Enum.map(requests, &Map.take(&1, ["jsonrpc", "method", "params"])) == [
               %{"jsonrpc" => "2.0", "method" => "eth_getBlockByNumber", "params" => ["0x1714EFD", true]},
               %{"jsonrpc" => "2.0", "method" => "eth_getBlockByNumber", "params" => ["0x1714EFE", true]}
             ]

      replies =
        requests
        |> Enum.reverse()
        |> Enum.map(fn %{"id" => id, "params" => [number, true]} ->
          %{"jsonrpc" => "2.0", "id" => id, "result" => Map.fetch!(recorded, String.downcase(number))}
        end)

      send(owner, {:served_recording, Enum.map(replies, & &1["result"]["hash"])})

      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.resp(200, Jason.encode!(replies))
    end)

    arguments = [
      transport: EthereumJSONRPC.HTTP,
      transport_options: [
        http: EthereumJSONRPC.HTTP.HTTPoison,
        urls: ["http://127.0.0.1:#{bypass.port}/"],
        http_options: [timeout: 2_000, recv_timeout: 2_000]
      ]
    ]

    assert {:ok, fetched} = EthereumJSONRPC.fetch_blocks_by_range(24_203_005..24_203_006, arguments)
    assert fetched.errors == []
    assert_receive {:served_recording, hashes}
    assert MapSet.new(hashes) == MapSet.new(recorded, fn {_, block} -> block["hash"] end)
    assert Enum.map(fetched.blocks_params, & &1.miner_hash) == [@miner, @miner]
    Enum.map(fetched.blocks_params, &Map.put(&1, :consensus, true))
  end

  defp persist_blocks(blocks) do
    address = Address.changeset(%Address{}, %{hash: @miner})
    assert address.valid?, inspect(address.errors)
    Repo.insert_all(Address, [Map.merge(address.changes, Import.timestamps())], on_conflict: :nothing)

    for params <- blocks do
      changeset = Block.changeset(%Block{}, params)
      assert changeset.valid?, inspect(changeset.errors)
      row = Map.merge(changeset.changes, Import.timestamps())
      assert {1, nil} = Repo.insert_all(Block, [row], on_conflict: {:replace, [:consensus]}, conflict_target: :hash)
    end
  end

  defp store_validator(count) do
    changeset = Validator.changeset(%Validator{}, %{address_hash: @miner, blocks_validated: count, state: :probation})
    assert changeset.valid?, inspect(changeset.errors)
    assert {1, nil} = Validator.insert_validators([Validator.append_timestamps(changeset.changes)])
  end

  defp validator_changes(blocks) do
    blocks
    |> Indexer.Transform.Stability.Validators.parse()
    |> Enum.map(fn params ->
      changeset = Validator.changeset(%Validator{}, params)
      assert changeset.valid?, inspect(changeset.errors)
      changeset.changes
    end)
  end

  defp validator_multi(changes), do: Validators.run(Multi.new(), changes, %{timestamps: Import.timestamps()})
  defp run_changes(changes), do: changes |> validator_multi() |> Repo.transaction()

  defp assert_total(result, count) do
    assert {:ok, %{stability_validators: [%Validator{} = validator]}} = result
    assert to_string(validator.address_hash) == @miner
    assert validator.blocks_validated == count
    assert validator.state == :probation
    assert Repo.get!(Validator, @miner).blocks_validated == count
  end
end
