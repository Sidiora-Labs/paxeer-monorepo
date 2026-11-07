defmodule Explorer.Chain.Cache.Counters.AddressTransactionsGasUsageSum do
  @moduledoc """
  Caches Address transactions gas usage count.
  """
  use GenServer
  use Utils.CompileTimeEnvHelper, enable_consolidation: [:explorer, [__MODULE__, :enable_consolidation]]

  alias Ecto.Changeset
  alias Explorer.Chain.Address.Counters
  alias Explorer.Chain.Cache.Counters.BoundedCache
  alias Explorer.Repo

  @cache_name :address_transactions_gas_usage_counter

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

  def fetch(address) do
    BoundedCache.fetch(@cache_name, __MODULE__, to_string(address.hash), fn ->
      value = Counters.address_to_gas_usage_count(address)
      put_into_db(address, value)
      value
    end)
  end

  def cache_name, do: @cache_name

  defp put_into_db(_address, value) when is_nil(value), do: :ignore

  defp put_into_db(address, value) do
    address
    |> Changeset.change(%{gas_used: Decimal.to_integer(value)})
    |> Repo.update()
  end

  defp enable_consolidation?, do: @enable_consolidation
end
