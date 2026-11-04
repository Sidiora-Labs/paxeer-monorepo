defmodule Explorer.Chain.Cache.Counters.TokenHoldersCount do
  @moduledoc """
  Caches Token holders count.
  """
  use GenServer
  use Utils.CompileTimeEnvHelper, enable_consolidation: [:explorer, [__MODULE__, :enable_consolidation]]

  alias Explorer.Chain.Address.CurrentTokenBalance
  alias Explorer.Chain.Cache.Counters.BoundedCache
  alias Explorer.Chain.{Hash, Token}

  @api_true [api?: true]
  @cache_name :token_holders_count

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

  def fetch(address_hash) do
    BoundedCache.fetch(@cache_name, __MODULE__, to_string(address_hash), fn ->
      new_data = CurrentTokenBalance.count_token_holders_from_token_hash(address_hash)
      put_into_db_cache(address_hash, new_data)
      new_data
    end)
  end

  def cache_name, do: @cache_name

  @doc """
  Fetches the token holders count from the cache or database.
  """
  @spec fetch_count_from_cache(Hash.Address.t()) :: integer()
  def fetch_count_from_cache(address_hash) do
    case BoundedCache.lookup(@cache_name, __MODULE__, to_string(address_hash)) do
      {:ok, count} -> count
      :miss -> fetch_from_db_cache(address_hash)
    end
  end

  defp fetch_from_db_cache(address_hash) do
    token = Token.get_by_contract_address_hash(address_hash, @api_true)
    (token && token.holder_count) || 0
  end

  defp put_into_db_cache(address_hash, count) do
    Token.update_token_holder_count(address_hash, count)
  end

  defp enable_consolidation?, do: @enable_consolidation
end
