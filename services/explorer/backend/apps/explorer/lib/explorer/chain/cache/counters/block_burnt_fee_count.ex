defmodule Explorer.Chain.Cache.Counters.BlockBurntFeeCount do
  @moduledoc """
  Caches Block Burnt Fee counter.
  """
  use GenServer
  use Utils.CompileTimeEnvHelper, enable_consolidation: [:explorer, [__MODULE__, :enable_consolidation]]

  alias Explorer.Chain
  alias Explorer.Chain.Cache.Counters.BoundedCache

  @cache_name :block_burnt_fee_counter

  @spec start_link(term()) :: GenServer.on_start()
  def start_link(_) do
    GenServer.start_link(__MODULE__, :ok, name: __MODULE__)
  end

  @impl true
  def init(_args) do
    BoundedCache.create(@cache_name, __MODULE__)

    {:ok, %{consolidate?: enable_consolidation?()}, {:continue, :ok}}
  end

  @impl true
  def handle_continue(:ok, %{consolidate?: true} = state) do
    {:noreply, state}
  end

  @impl true
  def handle_continue(:ok, state) do
    {:noreply, state}
  end

  @impl true
  def handle_info(:consolidate, state) do
    {:noreply, state}
  end

  @impl true
  def handle_info(:prune_cache, state) do
    BoundedCache.prune(@cache_name, __MODULE__)
    BoundedCache.schedule(__MODULE__)
    {:noreply, state}
  end

  def fetch(block_hash) do
    BoundedCache.fetch(@cache_name, __MODULE__, Base.encode16(block_hash.bytes, case: :lower), fn ->
      Chain.block_to_gas_used_by_1559_transactions(block_hash)
    end)
  end

  def cache_name, do: @cache_name

  defp enable_consolidation?, do: @enable_consolidation
end
