Code.require_file("../indexer/block/catchup/test_config.exs", __DIR__)
Code.require_file("../indexer/block/paxeer_import_case.exs", __DIR__)

defmodule Indexer.Repair2233552Test do
  use Explorer.DataCase, async: false
  use Indexer.Block.PaxeerImportCase

  alias Explorer.Chain.Cache.CeloCoreContracts
  alias Indexer.Block.Catchup.TestConfig

  @moduletag capture_log: true

  test "003: with single element range that is valid imports one block", context do
    assert_single_block_import(context)
  end

  test "003: Paxeer full-schema import uses configured Geth HTTP receipts traces and balances", context do
    assert_full_schema_import(context)
  end

  for previous <- [:configured, :absent, nil], outcome <- [:success, :assertion_failure] do
    test "030: Celo setup restores #{inspect(previous)} after #{outcome} with a real block import", context do
      recorded = Enum.find(context.blocks, &(&1["transactions"] != []))
      number = EthereumJSONRPC.quantity_to_integer(recorded["number"])

      previous =
        case unquote(previous) do
          :configured ->
            {:ok,
             [
               contracts: %{
                 "addresses" => %{
                   "Accounts" => [%{"address" => recorded["miner"], "updated_at_block_number" => number}]
                 }
               },
               retained_option: :preserve_me
             ]}

          :absent ->
            :error

          nil ->
            {:ok, nil}
        end

      restore(previous)
      assert Application.fetch_env(:explorer, CeloCoreContracts) == previous

      if unquote(previous) == :configured do
        assert CeloCoreContracts.get_address(:accounts, number) == {:ok, recorded["miner"]}
      end

      {pid, monitor} =
        spawn_monitor(fn ->
          :ok = ExUnit.OnExitHandler.register(self())
          assert :ok = TestConfig.setup_celo_core_contracts()
          assert {:error, :address_does_not_exist} = CeloCoreContracts.get_address(:accounts, number)
          assert_full_schema_import(context)

          if unquote(outcome) == :assertion_failure do
            flunk("deliberate assertion failure after the real Geth import")
          end
        end)

      on_exit(fn ->
        if Process.alive?(pid), do: Process.exit(pid, :kill)
        if :ets.member(ExUnit.Server, pid), do: ExUnit.OnExitHandler.run(pid, 5_000)
      end)

      assert_receive {:DOWN, ^monitor, :process, ^pid, reason}, 15_000
      assert :ok = ExUnit.OnExitHandler.run(pid, 5_000)

      case unquote(outcome) do
        :success ->
          assert reason == :normal

        :assertion_failure ->
          assert {%ExUnit.AssertionError{message: message}, _stack} = reason
          assert message == "deliberate assertion failure after the real Geth import"
      end

      assert Application.fetch_env(:explorer, CeloCoreContracts) == previous

      if unquote(previous) == :configured do
        assert CeloCoreContracts.get_address(:accounts, number) == {:ok, recorded["miner"]}
      end
    end
  end

  defp restore({:ok, value}), do: Application.put_env(:explorer, CeloCoreContracts, value)
  defp restore(:error), do: Application.delete_env(:explorer, CeloCoreContracts)
end
