defmodule Explorer.Chain.Cache.Counters.AddressTokensUsdSum do
  @moduledoc """
  Caches Address tokens USD value.
  """
  use GenServer
  use Utils.CompileTimeEnvHelper, enable_consolidation: [:explorer, [__MODULE__, :enable_consolidation]]

  alias Explorer.Chain
  alias Explorer.Chain.Cache.Counters.BoundedCache

  @cache_name :address_tokens_fiat_value

  @spec start_link(term()) :: GenServer.on_start()
  def start_link(_) do
    GenServer.start_link(__MODULE__, :ok, name: __MODULE__)
  end

  @impl true
  def init(_args) do
    BoundedCache.create(@cache_name, __MODULE__)

    {:ok, %{consolidate?: enable_consolidation?(), tasks: %{}}, {:continue, :ok}}
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

  @impl true
  def handle_info({:DOWN, ref, :process, _pid, _reason}, state) do
    {:noreply, %{state | tasks: Map.delete(state.tasks, ref)}}
  end

  @impl true
  def handle_cast({:store, key, value, pid}, state) do
    case Enum.find(state.tasks, fn {_ref, {task_key, task_pid}} -> task_key == key and task_pid == pid end) do
      {ref, _} ->
        BoundedCache.put(@cache_name, __MODULE__, key, value)
        Process.demonitor(ref, [:flush])
        {:noreply, %{state | tasks: Map.delete(state.tasks, ref)}}

      nil ->
        {:noreply, state}
    end
  end

  @impl true
  def handle_call({:fetch, key, balances}, _from, state) do
    case BoundedCache.lookup(@cache_name, __MODULE__, key) do
      {:ok, value} ->
        {:reply, value, state}

      :miss ->
        stale =
          case :ets.lookup(@cache_name, key) do
            [{^key, value, _}] -> value
            [] -> nil
          end

        running? = Enum.any?(state.tasks, fn {_, {task_key, _}} -> task_key == key end)

        if running? or map_size(state.tasks) >= BoundedCache.max_entries(__MODULE__) do
          {:reply, stale, state}
        else
          server = self()

          {:ok, pid} =
            Task.start(fn ->
              value = address_tokens_fiat_sum(balances)
              GenServer.cast(server, {:store, key, value, self()})
            end)

          ref = Process.monitor(pid)
          {:reply, stale, %{state | tasks: Map.put(state.tasks, ref, {key, pid})}}
        end
    end
  end

  def fetch(address_hash_string, token_balances) do
    GenServer.call(__MODULE__, {:fetch, to_string(address_hash_string), token_balances})
  end

  @spec address_tokens_fiat_sum([{Address.CurrentTokenBalance, Explorer.Chain.Token}]) :: Decimal.t()
  defp address_tokens_fiat_sum(token_balances) do
    token_balances
    |> Enum.reduce(Decimal.new(0), fn token_balance, acc ->
      if token_balance.value && token_balance.token.fiat_value && token_balance.token.decimals do
        Decimal.add(acc, Chain.balance_in_fiat(token_balance))
      else
        acc
      end
    end)
  end

  def cache_name, do: @cache_name

  defp enable_consolidation?, do: @enable_consolidation
end
