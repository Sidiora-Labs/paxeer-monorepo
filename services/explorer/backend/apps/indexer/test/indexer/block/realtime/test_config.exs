defmodule Indexer.Block.Realtime.TestConfig do
  @moduledoc false

  @keys [
    {:indexer, :fetch_rewards_way},
    {:indexer, Indexer.Fetcher.InternalTransaction.Supervisor},
    {:indexer, Indexer.Fetcher.UncleBlock.Supervisor},
    {:indexer, Indexer.Fetcher.Celo.EpochBlockOperations.Supervisor},
    {:indexer, Indexer.Fetcher.TokenInstance.Realtime.Supervisor},
    {:explorer, Explorer.Chain.Cache.CeloCoreContracts}
  ]

  def keys, do: @keys

  def preserve_env(keys \\ @keys) do
    previous = Enum.map(keys, fn {app, key} -> {app, key, Application.fetch_env(app, key)} end)

    ExUnit.Callbacks.on_exit(fn ->
      Enum.each(previous, fn
        {app, key, {:ok, value}} -> Application.put_env(app, key, value)
        {app, key, :error} -> Application.delete_env(app, key)
      end)
    end)
  end

  def block_fetcher(json_rpc_named_arguments) do
    %Indexer.Block.Fetcher{
      broadcast: false,
      callback_module: Indexer.Block.Realtime.Fetcher,
      json_rpc_named_arguments: json_rpc_named_arguments,
      task_supervisor: Indexer.TaskSupervisor
    }
  end
end
