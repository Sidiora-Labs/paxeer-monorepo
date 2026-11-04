Code.require_file("../indexer/block/catchup/test_config.exs", __DIR__)

defmodule Indexer.Repair2233551Test do
  use Explorer.DataCase, async: false

  alias EthereumJSONRPC.Blocks
  alias Explorer.Chain
  alias Explorer.Chain.Cache.CeloCoreContracts
  alias Indexer.Block.Catchup.{Fetcher, TestConfig}
  alias Indexer.Fetcher.Celo.EpochBlockOperations
  alias Indexer.Fetcher.OnDemand.ContractCreator
  alias Indexer.Transform.Addresses
  alias Indexer.Transform.Blocks, as: TransformBlocks

  @moduletag capture_log: true

  @keys [
    {:explorer, CeloCoreContracts},
    {:indexer, :last_block},
    {:indexer, :block_ranges},
    {:indexer, Fetcher},
    {:indexer, EpochBlockOperations.Supervisor}
  ]
  @fixture Path.expand("../../../ethereum_jsonrpc/test/support/fixture/paxeer_x/consecutive_blocks.json", __DIR__)
  @timeout 5_000

  setup do
    prior = snapshot()
    on_exit(fn -> restore(prior) end)

    recorded = @fixture |> File.read!() |> Jason.decode!() |> Map.values() |> Enum.sort_by(& &1["number"])
    %{recorded: recorded}
  end

  for prior <- [:configured, :absent, nil], outcome <- [:success, :assertion_failure] do
    test "001: Celo contracts are restored exactly with #{inspect(prior)} configuration after #{outcome}", %{
      recorded: recorded
    } do
      [block | _] = recorded
      number = EthereumJSONRPC.quantity_to_integer(block["number"])

      contracts = [
        contracts: %{
          "addresses" => %{
            "Accounts" => [%{"address" => block["miner"], "updated_at_block_number" => number}]
          }
        },
        retained_option: :preserve_me
      ]

      prior =
        case unquote(prior) do
          :configured -> {:ok, contracts}
          :absent -> :error
          nil -> {:ok, nil}
        end

      restore([{{:explorer, CeloCoreContracts}, prior}])
      before_setup = snapshot()

      run_lifecycle(
        fn ->
          assert :ok = TestConfig.setup_celo_core_contracts()
          assert {:error, :address_does_not_exist} = CeloCoreContracts.get_address(:accounts, number)
          import_recorded_blocks(recorded)
        end,
        unquote(outcome)
      )

      assert snapshot() == before_setup

      if unquote(prior) == :configured do
        assert CeloCoreContracts.get_address(:accounts, number) == {:ok, block["miner"]}
      end
    end
  end

  for setup <- [:setup_fetcher_import, :setup_fetcher_task],
      prior <- [:configured, :absent, nil],
      outcome <- [:success, :assertion_failure] do
    test "002: #{setup} restores every affected key with #{inspect(prior)} configuration after #{outcome}", %{
      recorded: recorded
    } do
      seed(unquote(prior), unquote(setup))
      before_setup = snapshot()

      run_lifecycle(
        fn ->
          assert :ok = apply(TestConfig, unquote(setup), [])
          assert EpochBlockOperations.Supervisor.disabled?()

          case unquote(setup) do
            :setup_fetcher_import ->
              assert Application.fetch_env!(:indexer, :last_block) == 0

            :setup_fetcher_task ->
              Application.put_env(:indexer, Fetcher, batch_size: 1, concurrency: 10)
              Application.put_env(:indexer, :block_ranges, "0..1")
              assert Fetcher.blocks_batch_size() == 1
              assert Fetcher.blocks_concurrency() == 10
              assert {:error, :address_does_not_exist} = CeloCoreContracts.get_address(:accounts, 0)
          end

          import_recorded_blocks(recorded)
        end,
        unquote(outcome)
      )

      assert snapshot() == before_setup
      refute EpochBlockOperations.Supervisor.disabled?()

      if unquote(prior) == :configured do
        assert Fetcher.blocks_batch_size() == 7
        assert Fetcher.blocks_concurrency() == 2
      end
    end
  end

  defp run_lifecycle(body, outcome) do
    # Use ExUnit's real callback registry and runner for an isolated test process.
    {pid, ref} =
      spawn_monitor(fn ->
        :ok = ExUnit.OnExitHandler.register(self())

        try do
          body.()

          if outcome == :assertion_failure do
            flunk("intentional assertion failure after the real import")
          end
        rescue
          error in ExUnit.AssertionError -> exit({:assertion_failure, error})
        after
          if creator = Process.whereis(ContractCreator) do
            :ok = GenServer.stop(creator)
          end
        end
      end)

    assert_receive {:DOWN, ^ref, :process, ^pid, reason}, @timeout
    assert :ok = ExUnit.OnExitHandler.run(pid, @timeout)

    assert Process.whereis(ContractCreator) == nil

    case outcome do
      :success ->
        assert reason == :normal

      :assertion_failure ->
        assert {:assertion_failure, %ExUnit.AssertionError{message: message}} = reason
        assert message == "intentional assertion failure after the real import"
    end
  end

  defp import_recorded_blocks(recorded) do
    requests =
      recorded
      |> Enum.with_index()
      |> Map.new(fn {block, id} -> {id, %{number: block["number"]}} end)

    responses = recorded |> Enum.with_index() |> Enum.map(fn {block, id} -> %{id: id, result: block} end)
    decoded = Blocks.from_responses(responses, requests)
    assert decoded.errors == []
    params = decoded.blocks_params |> TransformBlocks.transform_blocks() |> Enum.map(&Map.put(&1, :consensus, true))

    assert {:ok, %{blocks: imported}} =
             Chain.import(%{
               addresses: %{params: Addresses.extract_addresses(%{blocks: params})},
               blocks: %{params: params},
               broadcast: false
             })

    assert Enum.sort(Enum.map(imported, & &1.number)) == [24_203_005, 24_203_006]

    for block <- imported do
      assert %{consensus: true, hash: hash} = Repo.get_by!(Chain.Block, number: block.number, consensus: true)
      assert hash == block.hash
    end
  end

  defp seed(prior, setup) do
    configured = [
      {{:explorer, CeloCoreContracts}, {:ok, [contracts: %{"addresses" => %{}}, retained_option: :preserve_me]}},
      {{:indexer, :last_block}, {:ok, 42}},
      {{:indexer, :block_ranges}, {:ok, "5..42"}},
      {{:indexer, Fetcher}, {:ok, [batch_size: 7, concurrency: 2, retained_option: :preserve_me]}},
      {{:indexer, EpochBlockOperations.Supervisor}, {:ok, [disabled?: false, retained_option: :preserve_me]}}
    ]

    affected =
      case setup do
        :setup_fetcher_import ->
          [{:indexer, :last_block}, {:indexer, EpochBlockOperations.Supervisor}]

        :setup_fetcher_task ->
          [
            {:indexer, :block_ranges},
            {:indexer, Fetcher},
            {:indexer, EpochBlockOperations.Supervisor},
            {:explorer, CeloCoreContracts}
          ]
      end

    restore(
      Enum.map(configured, fn {key, value} ->
        if key in affected do
          case prior do
            :configured -> {key, value}
            :absent -> {key, :error}
            nil -> {key, {:ok, nil}}
          end
        else
          {key, value}
        end
      end)
    )
  end

  defp snapshot do
    Enum.map(@keys, fn {app, key} -> {{app, key}, Application.fetch_env(app, key)} end)
  end

  defp restore(values) do
    Enum.each(values, fn
      {{app, key}, {:ok, value}} -> Application.put_env(app, key, value)
      {{app, key}, :error} -> Application.delete_env(app, key)
    end)
  end
end
