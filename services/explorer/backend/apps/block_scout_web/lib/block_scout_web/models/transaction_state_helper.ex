defmodule BlockScoutWeb.Models.TransactionStateHelper do
  @moduledoc """
    Transaction state changes related functions
  """

  import Explorer.Chain.Address.Reputation, only: [reputation_association: 0]
  import Explorer.PagingOptions, only: [default_paging_options: 0]
  import Explorer.Chain.SmartContract, only: [burn_address_hash_string: 0]
  import Explorer.Chain.SmartContract.Proxy.Models.Implementation, only: [proxy_implementations_association: 0]

  alias Explorer.{Chain, PagingOptions, Repo}

  alias Explorer.Chain.{
    Address.CoinBalance,
    BlockNumberHelper,
    DenormalizationHelper,
    InternalTransaction,
    Transaction,
    Wei
  }

  alias Explorer.Chain.Cache.StateChanges
  alias Explorer.Chain.Transaction.StateChange
  alias Indexer.Fetcher.OnDemand.CoinBalance, as: CoinBalanceOnDemand
  alias Indexer.Fetcher.OnDemand.TokenBalance, as: TokenBalanceOnDemand

  {:ok, burn_address_hash} = Chain.string_to_address_hash(burn_address_hash_string())
  @burn_address_hash burn_address_hash

  @doc """
  This function takes transaction, fetches all the transactions before this one from the same block
  together with internal transactions and token transfers and calculates native coin and token
  balances before and after this transaction.
  """
  @spec state_changes(Transaction.t(), [Chain.paging_options() | Chain.api?()]) :: [StateChange.t()]
  def state_changes(transaction, options \\ [])

  def state_changes(%Transaction{} = transaction, options) do
    paging_options = Keyword.get(options, :paging_options, default_paging_options())
    {offset} = paging_options.key || {0}

    offset
    |> Kernel.==(0)
    |> if do
      get_and_cache_state_changes(transaction, options)
    else
      case StateChanges.get(transaction.hash) do
        %StateChanges{state_changes: state_changes} -> state_changes
        _ -> get_and_cache_state_changes(transaction, options)
      end
    end
    |> Enum.drop(offset)
  end

  def state_changes(_transaction, _options), do: []

  defp get_and_cache_state_changes(transaction, options) do
    {state_changes, history_available?} = do_state_changes(transaction, options)

    if history_available? do
      StateChanges.update(%StateChanges{
        transaction_hash: transaction.hash,
        state_changes: state_changes
      })
    else
      ConCache.delete(StateChanges.cache_name(), transaction.hash)
    end

    state_changes
  end

  defp do_state_changes(%Transaction{} = transaction, options) do
    block_transactions =
      transaction.block_hash
      |> Chain.block_to_transactions(
        paging_options: %PagingOptions{key: nil, page_size: nil},
        api?: Keyword.get(options, :api?, false)
      )
      |> Enum.filter(&(&1.index <= transaction.index))
      |> Repo.preload([:token_transfers, :internal_transactions])

    transaction =
      block_transactions
      |> Enum.find(&(&1.hash == transaction.hash))
      |> Repo.preload(
        token_transfers: [
          token: reputation_association(),
          from_address: [:scam_badge, :names, :smart_contract, proxy_implementations_association()],
          to_address: [:scam_badge, :names, :smart_contract, proxy_implementations_association()]
        ],
        internal_transactions: [
          from_address: [:scam_badge, :names, :smart_contract, proxy_implementations_association()],
          to_address: [:scam_badge, :names, :smart_contract, proxy_implementations_association()]
        ],
        block: [miner: [:names, :smart_contract, proxy_implementations_association()]],
        from_address: [:scam_badge, :names, :smart_contract, proxy_implementations_association()],
        to_address: [:scam_badge, :names, :smart_contract, proxy_implementations_association()]
      )

    previous_block_number = BlockNumberHelper.previous_block_number(transaction.block_number)

    coin_balances_before_block = transaction_to_coin_balances(transaction, previous_block_number, options)

    native_coin_entries = native_coin_entries(transaction, block_transactions, coin_balances_before_block)

    token_balances_before_block =
      transaction.token_transfers
      |> Enum.reduce(%{}, &token_transfers_to_balances_reducer(&1, &2, previous_block_number, options))

    tokens_entries = token_entries(transaction, block_transactions, token_balances_before_block)

    history_available? =
      Enum.all?(coin_balances_before_block, fn {_, {_, balance}} -> balance != :pending end) and
        Enum.all?(token_balances_before_block, fn {_, tokens} ->
          Enum.all?(tokens, fn {_, balances} -> Enum.all?(balances, fn {_, value} -> value != :pending end) end)
        end)

    {native_coin_entries ++ tokens_entries, history_available?}
  end

  defp native_coin_entries(transaction, block_transactions, balances_before_block) do
    balances_before =
      balances_before_block
      |> Map.reject(fn {_, {_, balance}} -> balance == :pending end)
      |> then(&StateChange.coin_balances_before(transaction, block_transactions, &1))

    # Zero is the origin for transaction deltas, never a substitute for missing history.
    deltas = Map.new(balances_before_block, fn {hash, {address, _}} -> {hash, {address, Wei.zero()}} end)

    transaction
    |> StateChange.native_coin_entries(deltas)
    |> Enum.map(fn entry ->
      case Map.get(balances_before, entry.address && entry.address.hash) do
        {_, balance} -> with_balances(entry, balance)
        nil -> with_balances(entry, :pending)
      end
    end)
  end

  defp token_entries(transaction, block_transactions, balances_before_block) do
    balances_before =
      balances_before_block
      |> map_token_balances(&Map.reject(&1, fn {_, value} -> value == :pending end))
      |> StateChange.token_balances_before(transaction, block_transactions)

    deltas =
      map_token_balances(balances_before_block, fn balances ->
        Map.new(balances, fn
          {:transfers, transfers} -> {:transfers, transfers}
          {id, _} -> {id, Decimal.new(0)}
        end)
      end)

    transaction.token_transfers
    |> StateChange.token_entries(deltas)
    |> Enum.map(fn entry ->
      [{_, transfer} | _] = entry.coin_or_token_transfers
      balance = balances_before[entry.address][transfer.token_contract_address_hash][entry.token_id]
      with_balances(entry, balance || :pending)
    end)
  end

  defp map_token_balances(balances, fun) do
    Map.new(balances, fn {address, tokens} ->
      {address, Map.new(tokens, fn {token, values} -> {token, fun.(values)} end)}
    end)
  end

  defp with_balances(entry, :pending) do
    # Both existing views render negative balances as unavailable (JSON null).
    unavailable = if entry.coin_or_token_transfers == :coin, do: %Wei{value: Decimal.new(-1)}, else: Decimal.new(-1)
    %{entry | balance_before: unavailable, balance_after: unavailable}
  end

  defp with_balances(%StateChange{coin_or_token_transfers: :coin} = entry, balance) do
    %{entry | balance_before: balance, balance_after: Wei.sum(balance, entry.balance_diff)}
  end

  defp with_balances(entry, balance) do
    %{entry | balance_before: balance, balance_after: Decimal.add(balance, entry.balance_diff)}
  end

  defp transaction_to_coin_balances(transaction, previous_block_number, options) do
    Enum.reduce(
      transaction.internal_transactions,
      %{
        transaction.from_address_hash =>
          {transaction.from_address, coin_balance(transaction.from_address_hash, previous_block_number, options)},
        transaction.to_address_hash =>
          {transaction.to_address, coin_balance(transaction.to_address_hash, previous_block_number, options)},
        transaction.block.miner_hash =>
          {transaction.block.miner, coin_balance(transaction.block.miner_hash, previous_block_number, options)}
      },
      &internal_transaction_to_coin_balances(&1, previous_block_number, options, &2)
    )
  end

  defp internal_transaction_to_coin_balances(
         %InternalTransaction{call_type: call_type, call_type_enum: call_type_enum},
         _,
         _,
         acc
       )
       when :delegatecall in [call_type, call_type_enum],
       do: acc

  defp internal_transaction_to_coin_balances(internal_transaction, previous_block_number, options, acc) do
    if not is_nil(internal_transaction.value) and Decimal.positive?(Wei.to(internal_transaction.value, :wei)) do
      acc
      |> Map.put_new_lazy(internal_transaction.from_address_hash, fn ->
        {internal_transaction.from_address,
         coin_balance(internal_transaction.from_address_hash, previous_block_number, options)}
      end)
      |> Map.put_new_lazy(internal_transaction.to_address_hash, fn ->
        {internal_transaction.to_address,
         coin_balance(internal_transaction.to_address_hash, previous_block_number, options)}
      end)
    else
      acc
    end
  end

  defp coin_balance(address_hash, _block_number, _options) when is_nil(address_hash) do
    Wei.zero()
  end

  defp coin_balance(address_hash, block_number, options) do
    case CoinBalance.get_coin_balance(address_hash, block_number, options) do
      %{value: val} when not is_nil(val) ->
        val

      _ ->
        CoinBalanceOnDemand.trigger_historic_fetch(options[:ip], address_hash, block_number)
        :pending
    end
  end

  defp token_balances(address_hash, token_transfer, block_number, options) do
    token = token_transfer.token

    token_ids =
      if token.type == "ERC-1155" do
        token_transfer.token_ids
      else
        [nil]
      end

    Enum.into(token_ids, %{transfers: []}, &{&1, token_balance(address_hash, block_number, token, &1, options)})
  end

  defp token_balance(@burn_address_hash, _block_number, _token, _token_id, _options) do
    Decimal.new(0)
  end

  defp token_balance(address_hash, block_number, token, token_id, options) do
    case Chain.get_token_balance(address_hash, token.contract_address_hash, block_number, token_id, options) do
      %{value: val} when not is_nil(val) ->
        val

      _ ->
        TokenBalanceOnDemand.trigger_historic_fetch(
          options[:ip],
          address_hash,
          token.contract_address_hash,
          token.type,
          token_id,
          block_number
        )

        :pending
    end
  end

  defp token_transfers_to_balances_reducer(transfer, balances, prev_block, options) do
    token_type =
      if DenormalizationHelper.tt_denormalization_finished?() do
        transfer.token_type
      else
        transfer.token && transfer.token.type
      end

    # Skip ERC-7984 (confidential) transfers - we can't track encrypted balances
    if token_type == "ERC-7984" do
      balances
    else
      from = transfer.from_address
      to = transfer.to_address
      token_hash = transfer.token_contract_address_hash

      balances
      |> case do
        # from address already in the map
        %{^from => %{^token_hash => _}} = balances ->
          balances

        # we need to add from address into the map
        balances ->
          put_in(
            balances,
            Enum.map([from, token_hash], &Access.key(&1, %{})),
            token_balances(from.hash, transfer, prev_block, options)
          )
      end
      |> case do
        # to address already in the map
        %{^to => %{^token_hash => _}} = balances ->
          balances

        # we need to add to address into the map
        balances ->
          put_in(
            balances,
            Enum.map([to, token_hash], &Access.key(&1, %{})),
            token_balances(to.hash, transfer, prev_block, options)
          )
      end
    end
  end
end
