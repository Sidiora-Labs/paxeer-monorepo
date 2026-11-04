defmodule Explorer.Chain.Cache.Counters.AddressTabsElementsCount do
  @moduledoc """
    Cache for tabs counters on address
  """

  use GenServer

  alias Explorer.Chain.Address.Counters
  alias Explorer.Chain.Cache.Counters.BoundedCache

  @cache_name :addresses_tabs_counters
  @tasks_name :addresses_tabs_counter_tasks

  @typep counter_type ::
           :validations
           | :transactions
           | :token_transfers
           | :token_balances
           | :logs
           | :withdrawals
           | :internal_transactions
           | :beacon_deposits
  @typep response_status :: :limit_value | :stale | :up_to_date

  @spec get_counter(counter_type, String.t()) :: {DateTime.t(), non_neg_integer(), response_status} | nil
  def get_counter(counter_type, address_hash) do
    case :ets.lookup(@cache_name, cache_key(address_hash, counter_type)) do
      [{_, value, _inserted}] -> check_staleness(value)
      [] -> nil
    end
  end

  @spec set_counter(counter_type, String.t(), non_neg_integer()) :: :ok
  def set_counter(counter_type, address_hash, counter) do
    BoundedCache.put(@cache_name, __MODULE__, cache_key(address_hash, counter_type), {DateTime.utc_now(), counter})

    :ok
  end

  @spec set_task(atom, String.t()) :: true
  def set_task(counter_type, address_hash) do
    GenServer.call(__MODULE__, {:set_task, task_cache_key(address_hash, counter_type)})
  end

  @spec drop_task(atom, String.t()) :: true
  def drop_task(counter_type, address_hash) do
    GenServer.call(__MODULE__, {:drop_task, task_cache_key(address_hash, counter_type)})
  end

  @spec get_task(atom, String.t()) :: true | nil
  def get_task(counter_type, address_hash) do
    case :ets.lookup(@tasks_name, task_cache_key(address_hash, counter_type)) do
      [{_, true}] -> true
      [] -> nil
    end
  end

  def save_transactions_counter_progress(address_hash, results) do
    GenServer.cast(__MODULE__, {:set_transactions_state, address_hash, results})
  end

  def start_link(_) do
    GenServer.start_link(__MODULE__, :ok, name: __MODULE__)
  end

  @impl true
  def init(_opts) do
    BoundedCache.create(@cache_name, __MODULE__)

    :ets.new(@tasks_name, [
      :set,
      :named_table,
      :public,
      read_concurrency: true,
      write_concurrency: true
    ])

    {:ok, %{}}
  end

  @impl true
  def handle_call({:set_task, key}, _from, state) do
    {:reply, :ets.insert(@tasks_name, {key, true}), state}
  end

  @impl true
  def handle_call({:drop_task, key}, _from, state) do
    {:reply, :ets.delete(@tasks_name, key), state}
  end

  @impl true
  def handle_cast({:set_transactions_state, address_hash, %{transactions_types: transactions_types} = results}, state) do
    address_hash = lowercased_string(address_hash)

    previous = state[address_hash] || %{}
    types = Enum.uniq(transactions_types ++ (previous[:transactions_types] || []))

    address_state =
      Enum.reduce(transactions_types, previous, fn type, acc -> Map.put(acc, type, results[type]) end)
      |> Map.put(:transactions_types, types)

    counter =
      if previous[:limit_reached] do
        Counters.counters_limit()
      else
        Counters.transactions_types()
        |> Enum.flat_map(fn type -> address_state[type] || [] end)
        |> Enum.uniq()
        |> Enum.count()
        |> min(Counters.counters_limit())
      end

    cond do
      Enum.count(types) == 3 ->
        set_counter(:transactions, address_hash, counter)
        {:noreply, Map.delete(state, address_hash)}

      counter == Counters.counters_limit() ->
        set_counter(:transactions, address_hash, counter)
        {:noreply, Map.put(state, address_hash, %{transactions_types: types, limit_reached: true})}

      true ->
        {:noreply, Map.put(state, address_hash, address_state)}
    end
  end

  @impl true
  def handle_info(:prune_cache, state) do
    BoundedCache.prune(@cache_name, __MODULE__)
    BoundedCache.schedule(__MODULE__)

    active_addresses =
      @tasks_name
      |> :ets.tab2list()
      |> MapSet.new(fn {{:task, address, _type}, true} -> address end)

    {:noreply, Map.filter(state, fn {address, _} -> MapSet.member?(active_addresses, address) end)}
  end

  defp check_staleness(nil), do: nil
  defp check_staleness({datetime, counter}) when counter > 50, do: {datetime, counter, :limit_value}

  defp check_staleness({datetime, counter}) do
    status =
      if up_to_date?(datetime, ttl()) do
        :up_to_date
      else
        :stale
      end

    {datetime, counter, status}
  end

  defp up_to_date?(datetime, ttl) do
    datetime
    |> DateTime.add(ttl, :millisecond)
    |> DateTime.compare(DateTime.utc_now()) != :lt
  end

  defp ttl, do: Application.get_env(:explorer, Explorer.Chain.Cache.Counters.AddressTabsElementsCount)[:ttl]
  defp lowercased_string(str), do: str |> to_string() |> String.downcase()

  defp cache_key(address_hash, counter_type), do: {lowercased_string(address_hash), counter_type}
  defp task_cache_key(address_hash, counter_type), do: {:task, lowercased_string(address_hash), counter_type}
end
