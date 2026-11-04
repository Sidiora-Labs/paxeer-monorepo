defmodule Explorer.Chain.Cache.TransactionActionUniswapPools do
  @moduledoc """
  Caches Uniswap pools for Indexer.Transform.TransactionActions.

  Retains at most `:max_cache_size` pools (10,000 by default), evicting the
  oldest inserted pool first. Updating an entry preserves its position.
  Evicted entries return `nil`, so the importer fetches their metadata again.
  The GenServer owns the table and serializes writes to enforce the bound
  even when multiple importers discover pools concurrently.
  """
  use GenServer

  @cache_name :transaction_actions_uniswap_pools_cache
  @default_max_cache_size 10_000

  @spec start_link(term()) :: GenServer.on_start()
  def start_link(_) do
    GenServer.start_link(__MODULE__, :ok, name: __MODULE__)
  end

  @impl true
  def init(_args) do
    capacity = Application.get_env(:explorer, __MODULE__, [])[:max_cache_size] || @default_max_cache_size

    if not (is_integer(capacity) and capacity > 0) do
      raise ArgumentError, "Uniswap pool cache max_cache_size must be a positive integer"
    end

    :ets.new(@cache_name, [:set, :named_table, :protected, read_concurrency: true])
    {:ok, %{capacity: capacity, insertion_order: :queue.new()}}
  end

  def create_cache_table do
    GenServer.call(__MODULE__, :cache_table)
  end

  def fetch_from_cache(pool_address) do
    case :ets.whereis(@cache_name) do
      :undefined -> nil
      table -> lookup(table, pool_address)
    end
  end

  def put_to_cache(address, value) do
    GenServer.call(__MODULE__, {:put, address, value})
  end

  @impl true
  def handle_call(:cache_table, _from, state) do
    {:reply, :ets.whereis(@cache_name), state}
  end

  def handle_call({:put, address, value}, _from, state) do
    order =
      if :ets.member(@cache_name, address) do
        state.insertion_order
      else
        order =
          if :ets.info(@cache_name, :size) == state.capacity do
            {{:value, oldest}, rest} = :queue.out(state.insertion_order)
            :ets.delete(@cache_name, oldest)
            rest
          else
            state.insertion_order
          end

        :queue.in(address, order)
      end

    result = :ets.insert(@cache_name, {address, value})
    {:reply, result, %{state | insertion_order: order}}
  end

  defp lookup(table, pool_address) do
    case :ets.lookup(table, pool_address) do
      [{_, value}] -> value
      [] -> nil
    end
  rescue
    # The owner may restart between resolving the table and reading it.
    ArgumentError -> nil
  end
end
