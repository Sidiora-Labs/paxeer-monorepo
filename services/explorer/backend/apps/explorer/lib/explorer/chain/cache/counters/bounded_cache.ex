defmodule Explorer.Chain.Cache.Counters.BoundedCache do
  @moduledoc false

  alias Explorer.Chain.Cache.Counters.Helper

  def create(table, module) do
    Helper.create_cache_table(table)
    schedule(module)
  end

  def max_entries(module) do
    limit = config(module)[:max_entries] || 10_000
    if not (is_integer(limit) and limit > 0), do: raise(ArgumentError, "max_entries must be positive")
    limit
  end

  def schedule(module) do
    interval =
      case ttl(module) do
        :infinity -> 60_000
        period -> max(1, min(period, 60_000))
      end

    Process.send_after(self(), :prune_cache, interval)
  end

  def lookup(table, module, key) do
    case :ets.lookup(table, key) do
      [{^key, value, inserted}] ->
        if expired?(inserted, ttl(module)), do: :miss, else: {:ok, value}

      [] ->
        :miss
    end
  end

  def fetch(table, module, key, producer) do
    :global.trans(
      {{__MODULE__, table, key}, self()},
      fn ->
        case lookup(table, module, key) do
          {:ok, value} ->
            value

          :miss ->
            value = producer.()
            put(table, module, key, value)
            value
        end
      end,
      [node()]
    )
  end

  def put(table, module, key, value) do
    locked(table, fn ->
      prune_entries(table, module)
      :ets.delete(table, key)
      evict(table, max_entries(module) - 1)
      :ets.insert(table, {key, value, System.monotonic_time(:millisecond)})
    end)
  end

  def prune(table, module) do
    locked(table, fn ->
      prune_entries(table, module)
      evict(table, max_entries(module))
    end)
  end

  defp locked(table, fun), do: :global.trans({{__MODULE__, table}, self()}, fun, [node()])

  defp prune_entries(table, module) do
    case ttl(module) do
      :infinity ->
        :ok

      period ->
        cutoff = System.monotonic_time(:millisecond) - period
        :ets.select_delete(table, [{{:_, :_, :"$1"}, [{:<, :"$1", cutoff}], [true]}])
    end
  end

  defp evict(table, limit) do
    excess = :ets.info(table, :size) - limit

    if excess > 0 do
      table
      |> :ets.tab2list()
      |> Enum.sort_by(fn {_key, _value, inserted} -> inserted end)
      |> Enum.take(excess)
      |> Enum.each(fn {key, _, _} -> :ets.delete(table, key) end)
    end
  end

  defp expired?(_, :infinity), do: false
  defp expired?(inserted, period), do: System.monotonic_time(:millisecond) - inserted > period
  defp ttl(module), do: config(module)[:cache_period] || config(module)[:ttl] || :infinity
  defp config(module), do: Application.get_env(:explorer, module, [])
end
