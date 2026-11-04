defmodule Indexer.Fetcher.OnDemand.TokenBalance do
  @moduledoc """
  Ensures that we have a reasonably up to date address tokens balance.

  """

  use Indexer.Fetcher

  import Ecto.Query, only: [from: 2]

  alias Explorer.{Chain, Repo}
  alias Explorer.Chain.Address.{CurrentTokenBalance, TokenBalance}
  alias Explorer.Chain.Cache.BlockNumber
  alias Explorer.Chain.Cache.Counters.AverageBlockTime
  alias Explorer.Chain.Events.Publisher
  alias Explorer.Chain.Hash
  alias Explorer.Token.BalanceReader
  alias Explorer.Utility.RateLimiter
  alias Indexer.BufferedTask
  alias Timex.Duration

  require Logger

  @behaviour BufferedTask

  @max_batch_size 500
  @max_concurrency 4
  @defaults [
    flush_interval: :timer.seconds(3),
    max_concurrency: @max_concurrency,
    max_batch_size: @max_batch_size,
    task_supervisor: Indexer.Fetcher.OnDemand.TokenBalance.TaskSupervisor,
    metadata: [fetcher: :token_balance_on_demand]
  ]

  @spec trigger_fetch(String.t() | nil, Hash.Address.t()) :: :ok
  def trigger_fetch(caller \\ nil, address_hash) do
    if not __MODULE__.Supervisor.disabled?() and RateLimiter.check_rate(caller, :on_demand) == :allow do
      BufferedTask.buffer(__MODULE__, [{:fetch, address_hash}], false)
    end
  end

  @spec trigger_historic_fetch(
          String.t() | nil,
          Hash.t(),
          Hash.t(),
          String.t(),
          Decimal.t() | nil,
          non_neg_integer()
        ) :: :ok
  def trigger_historic_fetch(caller \\ nil, address_hash, contract_address_hash, token_type, token_id, block_number) do
    if not __MODULE__.Supervisor.disabled?() and RateLimiter.check_rate(caller, :on_demand) == :allow do
      BufferedTask.buffer(
        __MODULE__,
        [{:historic_fetch, {address_hash, contract_address_hash, token_type, token_id, block_number}}],
        false
      )
    end
  end

  @doc false
  def child_spec([init_options, gen_server_options]) do
    merged_init_opts =
      @defaults
      |> Keyword.merge(init_options)
      |> Keyword.put(:state, %{})

    Supervisor.child_spec({BufferedTask, [{__MODULE__, merged_init_opts}, gen_server_options]}, id: __MODULE__)
  end

  @impl BufferedTask
  def init(initial, _, _) do
    initial
  end

  @impl BufferedTask
  def run(entries, _) do
    entries_by_type = Enum.group_by(entries, &elem(&1, 0), &elem(&1, 1))

    latest_block_number = latest_block_number()

    fetch_data = prepare_fetch_requests(entries_by_type[:fetch], latest_block_number)

    {historic_fetch_ft_requests, historic_fetch_nft_requests} =
      prepare_historic_fetch_requests(entries_by_type[:historic_fetch])

    all_responses =
      BalanceReader.get_balances_of_all(
        fetch_data.ft_balances ++ historic_fetch_ft_requests,
        fetch_data.nft_balances ++ historic_fetch_nft_requests
      )

    {fetch_ft_responses, other_responses} = Enum.split(all_responses, Enum.count(fetch_data.ft_balances))
    {historic_fetch_ft_responses, nft_responses} = Enum.split(other_responses, Enum.count(historic_fetch_ft_requests))
    {fetch_nft_responses, historic_fetch_nft_responses} = Enum.split(nft_responses, Enum.count(fetch_data.nft_balances))

    (fetch_ft_responses ++ fetch_nft_responses)
    |> Enum.zip(fetch_data.ft_balances ++ fetch_data.nft_balances)
    |> process_fetch_responses(fetch_data.tokens, fetch_data.balances_map, latest_block_number)

    (historic_fetch_ft_responses ++ historic_fetch_nft_responses)
    |> Enum.zip(historic_fetch_ft_requests ++ historic_fetch_nft_requests)
    |> process_historic_fetch_responses()

    :ok
  end

  defp prepare_fetch_requests(nil, _latest_block_number),
    do: %{nft_balances: [], ft_balances: [], tokens: %{}, balances_map: %{}}

  defp prepare_fetch_requests(address_hashes, latest_block_number) do
    initial_acc = %{nft_balances: [], ft_balances: [], tokens: %{}, balances_map: %{}}

    case stale_balance_window(latest_block_number) do
      {:error, _} ->
        initial_acc

      stale_balance_window ->
        address_hashes
        |> Enum.uniq()
        |> Chain.fetch_last_token_balances_include_unfetched()
        |> delete_invalid_balances()
        |> Enum.filter(fn current_token_balance -> current_token_balance.block_number < stale_balance_window end)
        |> prepare_ctb_params(initial_acc, latest_block_number)
    end
  end

  defp prepare_ctb_params(current_token_balances, initial_acc, block_number) do
    Enum.reduce(current_token_balances, initial_acc, fn %{token_id: token_id} = stale_current_token_balance, acc ->
      prepared_ctb = %{
        token_contract_address_hash: to_string(stale_current_token_balance.token_contract_address_hash),
        address_hash: to_string(stale_current_token_balance.address_hash),
        block_number: block_number,
        token_id: token_id && Decimal.to_integer(token_id),
        token_type: stale_current_token_balance.token_type
      }

      updated_tokens =
        Map.put_new(
          acc[:tokens],
          stale_current_token_balance.token_contract_address_hash.bytes,
          stale_current_token_balance.token
        )

      result =
        if nft_balance?(prepared_ctb) do
          Map.put(acc, :nft_balances, [prepared_ctb | acc[:nft_balances]])
        else
          Map.put(acc, :ft_balances, [prepared_ctb | acc[:ft_balances]])
        end

      updated_balances_map =
        Map.put(
          acc[:balances_map],
          ctb_to_key(stale_current_token_balance),
          stale_current_token_balance.value
        )

      result
      |> Map.put(:tokens, updated_tokens)
      |> Map.put(:balances_map, updated_balances_map)
    end)
  end

  defp prepare_historic_fetch_requests(nil), do: {[], []}

  defp prepare_historic_fetch_requests(params) do
    Enum.reduce(params, {[], []}, fn {address_hash, contract_address_hash, token_type, token_id, block_number},
                                     {regular_acc, erc_1155_acc} ->
      request = %{
        token_contract_address_hash: to_string(contract_address_hash),
        address_hash: to_string(address_hash),
        block_number: block_number,
        token_type: token_type,
        token_id: token_id && Decimal.to_integer(token_id)
      }

      if nft_balance?(request) do
        {regular_acc, [request | erc_1155_acc]}
      else
        {[request | regular_acc], erc_1155_acc}
      end
    end)
  end

  defp nft_balance?(%{token_type: "ERC-1155"}), do: true
  defp nft_balance?(%{token_type: "ERC-404", token_id: token_id}), do: not is_nil(token_id)
  defp nft_balance?(_), do: false

  defp process_fetch_responses(responses, tokens, balances_map, block_number) do
    filtered_current_token_balances_update_params =
      responses
      |> Enum.map(&prepare_updated_balance(&1, block_number))
      |> Enum.reject(&is_nil/1)

    if not Enum.empty?(filtered_current_token_balances_update_params) do
      {:ok,
       %{
         address_current_token_balances: imported_ctbs
       }} =
        Chain.import(%{
          address_current_token_balances: %{
            params: filtered_current_token_balances_update_params
          },
          broadcast: false
        })

      imported_ctbs
      |> filter_imported_ctbs(balances_map)
      |> Enum.group_by(& &1.address_hash)
      |> Enum.each(fn {address_hash, ctbs} ->
        Publisher.broadcast(
          %{
            address_current_token_balances: %{
              address_hash: to_string(address_hash),
              address_current_token_balances:
                Enum.map(ctbs, fn %CurrentTokenBalance{} = ctb ->
                  %CurrentTokenBalance{ctb | token: tokens[ctb.token_contract_address_hash.bytes]}
                end)
            }
          },
          :on_demand
        )
      end)
    end
  end

  defp process_historic_fetch_responses([]), do: :ok

  defp process_historic_fetch_responses(responses) do
    import_params =
      Enum.reduce(responses, [], fn
        {{:ok, balance}, request}, acc ->
          params = %{
            address_hash: request.address_hash,
            token_contract_address_hash: request.token_contract_address_hash,
            token_type: request.token_type,
            token_id: request.token_id,
            block_number: request.block_number,
            value: Decimal.new(balance),
            value_fetched_at: DateTime.utc_now()
          }

          [params | acc]

        {{:error, error}, request}, acc ->
          Logger.error("Error while fetching token balances: #{inspect(error)}, request: #{inspect(request)}")
          acc
      end)

    {id_balances, other_balances} =
      Enum.split_with(import_params, &(&1.token_type == "ERC-404" and nft_balance?(&1)))

    {:ok, multi} =
      Chain.Import.all_single_multi([Chain.Import.Runner.Address.TokenBalances], %{
        address_token_balances: %{params: other_balances}
      })

    {:ok, %{historic_id_balances: fetched_id_balances} = imported} =
      multi
      |> Ecto.Multi.run(:historic_id_balances, fn _repo, _changes ->
        # Transfer imports deliberately leave ERC-404 id balances unfetched.
        # These values came from balanceOf(address,uint256), so persist them as fetched schema values.
        fetched_id_balances =
          id_balances
          |> Enum.sort_by(&{&1.token_contract_address_hash, &1.token_id, &1.address_hash, &1.block_number})
          |> Enum.map(&import_historic_id_balance/1)

        {:ok, fetched_id_balances}
      end)
      |> Repo.transaction()

    imported
    |> Map.delete(:historic_id_balances)
    |> Map.update(:address_token_balances, fetched_id_balances, &(&1 ++ fetched_id_balances))
    |> Publisher.broadcast(:on_demand)
  end

  defp import_historic_id_balance(params) do
    on_conflict =
      from(balance in TokenBalance,
        update: [
          set: [
            value: fragment("EXCLUDED.value"),
            value_fetched_at: fragment("EXCLUDED.value_fetched_at"),
            refetch_after: nil,
            retries_count: nil,
            updated_at: fragment("GREATEST(EXCLUDED.updated_at, ?)", balance.updated_at)
          ]
        ],
        where: is_nil(balance.value_fetched_at) or fragment("? < EXCLUDED.value_fetched_at", balance.value_fetched_at)
      )

    %TokenBalance{}
    |> TokenBalance.changeset(params)
    |> Repo.insert!(
      on_conflict: on_conflict,
      conflict_target:
        {:unsafe_fragment, "(address_hash, token_contract_address_hash, COALESCE(token_id, -1), block_number)"},
      returning: true
    )
  end

  defp delete_invalid_balances(current_token_balances) do
    {invalid_balances, valid_balances} = Enum.split_with(current_token_balances, &is_nil(&1.token_type))
    Enum.each(invalid_balances, &Repo.delete/1)
    valid_balances
  end

  defp filter_imported_ctbs(imported_ctbs, balances_map) do
    Enum.filter(imported_ctbs, fn ctb ->
      if balance = balances_map[ctb_to_key(ctb)] do
        Decimal.compare(balance, ctb.value) != :eq
      else
        Logger.error("Imported unknown balance")
        true
      end
    end)
  end

  defp ctb_to_key(ctb) do
    {ctb.address_hash.bytes, ctb.token_contract_address_hash.bytes, ctb.token_type,
     ctb.token_id && Decimal.to_integer(ctb.token_id)}
  end

  defp prepare_updated_balance({{:ok, updated_balance}, stale_current_token_balance}, block_number) do
    %{}
    |> Map.put(:address_hash, stale_current_token_balance.address_hash)
    |> Map.put(:token_contract_address_hash, stale_current_token_balance.token_contract_address_hash)
    |> Map.put(:token_type, stale_current_token_balance.token_type)
    |> Map.put(:token_id, stale_current_token_balance.token_id)
    |> Map.put(:block_number, block_number)
    |> Map.put(:value, Decimal.new(updated_balance))
    |> Map.put(:value_fetched_at, DateTime.utc_now())
  end

  defp prepare_updated_balance({{:error, error}, ctb}, block_number) do
    error_message =
      if ctb.token_id do
        "Error on updating current token #{to_string(ctb.token_contract_address_hash)} balance for address #{to_string(ctb.address_hash)} and token id #{to_string(ctb.token_id)} at block number #{block_number}: "
      else
        "Error on updating current token #{to_string(ctb.token_contract_address_hash)} balance for address #{to_string(ctb.address_hash)} at block number #{block_number}: "
      end

    Logger.warning(fn ->
      [
        error_message,
        inspect(error)
      ]
    end)

    nil
  end

  defp latest_block_number do
    BlockNumber.get_max()
  end

  defp stale_balance_window(block_number) do
    case AverageBlockTime.average_block_time() do
      {:error, :disabled} ->
        fallback_threshold_in_blocks = Application.get_env(:indexer, __MODULE__)[:fallback_threshold_in_blocks]
        block_number - fallback_threshold_in_blocks

      duration ->
        average_block_time =
          duration
          |> Duration.to_milliseconds()
          |> round()

        if average_block_time == 0 do
          {:error, :empty_database}
        else
          threshold = Application.get_env(:indexer, __MODULE__)[:threshold]
          block_number - div(threshold, average_block_time)
        end
    end
  end
end
