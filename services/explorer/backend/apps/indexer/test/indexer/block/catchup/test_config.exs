defmodule Indexer.Block.Catchup.TestConfig do
  @moduledoc false

  import ExUnit.Callbacks, only: [on_exit: 1]

  alias Explorer.Chain.Cache.CeloCoreContracts
  alias Indexer.Block.Catchup.Fetcher
  alias Indexer.Fetcher.Celo.EpochBlockOperations
  alias Indexer.Fetcher.OnDemand.ContractCreator

  def setup_celo_core_contracts do
    preserve_env(:explorer, [CeloCoreContracts])

    Application.put_env(:explorer, CeloCoreContracts,
      contracts: %{
        "addresses" => %{
          "Accounts" => [],
          "Election" => [],
          "EpochRewards" => [],
          "FeeHandler" => [],
          "GasPriceMinimum" => [],
          "GoldToken" => [],
          "Governance" => [],
          "LockedGold" => [],
          "Reserve" => [],
          "StableToken" => [],
          "Validators" => []
        }
      }
    )
  end

  def setup_fetcher_import do
    preserve_env(:indexer, [:last_block, EpochBlockOperations.Supervisor])
    Application.put_env(:indexer, :last_block, 0)
    Application.put_env(:indexer, EpochBlockOperations.Supervisor, disabled?: true)

    {:ok, _pid} = ContractCreator.start_link([[], []])
    :ok
  end

  def setup_fetcher_task do
    preserve_env(:indexer, [:block_ranges, Fetcher, EpochBlockOperations.Supervisor])
    Application.put_env(:indexer, EpochBlockOperations.Supervisor, disabled?: true)
    setup_celo_core_contracts()
  end

  defp preserve_env(application, keys) do
    prior = Enum.map(keys, &{&1, Application.fetch_env(application, &1)})

    on_exit(fn ->
      Enum.each(prior, fn
        {key, {:ok, value}} -> Application.put_env(application, key, value)
        {key, :error} -> Application.delete_env(application, key)
      end)
    end)
  end
end
