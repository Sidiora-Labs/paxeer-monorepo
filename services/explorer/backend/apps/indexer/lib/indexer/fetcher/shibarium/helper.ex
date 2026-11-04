defmodule Indexer.Fetcher.Shibarium.Helper do
  @moduledoc """
  Common functions for Indexer.Fetcher.Shibarium.* modules.
  """

  import Ecto.Query
  import Explorer.Helper, only: [hash_to_binary: 1]

  alias Explorer.Chain.Cache.Counters.Shibarium.DepositsAndWithdrawalsCount
  alias Explorer.Chain.Shibarium.{Bridge, Reader}
  alias Explorer.Repo

  @empty_hash "0x0000000000000000000000000000000000000000000000000000000000000000"

  @block_check_interval_range_size 100
  @initial_block_check_interval 2_000

  @doc """
  Uses the configured polling period until two non-genesis committed heights
  exist, then estimates half the average block time using Indexer.Helper's RPC readers.
  """
  @spec get_block_check_interval(non_neg_integer(), list()) ::
          {:ok, non_neg_integer(), non_neg_integer()} | {:error, any()}
  def get_block_check_interval(latest_block, _json_rpc_named_arguments) when latest_block < 2 do
    interval =
      Application.get_env(:indexer, Indexer.Block.Realtime.Fetcher)[:polling_period] || @initial_block_check_interval

    {:ok, interval, latest_block}
  end

  def get_block_check_interval(latest_block, json_rpc_named_arguments) do
    first_block = max(latest_block - @block_check_interval_range_size, 1)

    with {:ok, first_timestamp} <-
           Indexer.Helper.get_block_timestamp_by_number_or_tag(first_block, json_rpc_named_arguments),
         {:ok, latest_timestamp} <-
           Indexer.Helper.get_block_timestamp_by_number_or_tag(latest_block, json_rpc_named_arguments) do
      interval = ceil((latest_timestamp - first_timestamp) / (latest_block - first_block) * 1000 / 2)
      {:ok, interval, latest_block}
    else
      {:error, error} ->
        {:error, "Failed to calculate block check interval due to #{inspect(error)}"}
    end
  end

  @doc """
  Calculates Shibarium Bridge operation hash as hash_256(user_address, amount_or_id, erc1155_ids, erc1155_amounts, operation_id).
  """
  @spec calc_operation_hash(binary(), non_neg_integer() | nil, list(), list(), non_neg_integer()) :: binary()
  def calc_operation_hash(user, amount_or_id, erc1155_ids, erc1155_amounts, operation_id) do
    user_binary = hash_to_binary(user)

    amount_or_id =
      if is_nil(amount_or_id) and not Enum.empty?(erc1155_ids) do
        0
      else
        amount_or_id
      end

    operation_encoded =
      ABI.encode("(address,uint256,uint256[],uint256[],uint256)", [
        {
          user_binary,
          amount_or_id,
          erc1155_ids,
          erc1155_amounts,
          operation_id
        }
      ])

    "0x" <>
      (operation_encoded
       |> ExKeccak.hash_256()
       |> Base.encode16(case: :lower))
  end

  @doc """
  Prepares a list of Shibarium Bridge operations to import them into database.
  Tries to bind the given operations to the existing ones in DB first.
  If they don't exist, prepares the insertion list and returns it.
  """
  @spec prepare_insert_items(list(), module()) :: list()
  def prepare_insert_items(operations, calling_module) do
    operations
    |> Enum.reduce([], fn op, acc ->
      if not imported_operation?(op, calling_module) and bind_existing_operation_in_db(op, calling_module) == 0 do
        [op | acc]
      else
        acc
      end
    end)
    |> Enum.reverse()
    |> Enum.reduce(%{}, fn item, acc ->
      Map.put(acc, {item.operation_hash, item.l1_transaction_hash, item.l2_transaction_hash}, item)
    end)
    |> Map.values()
  end

  @doc """
  Recalculate the cached count of complete rows for deposits and withdrawals.
  """
  @spec recalculate_cached_count() :: no_return()
  def recalculate_cached_count do
    DepositsAndWithdrawalsCount.deposits_count_save(Reader.deposits_count())
    DepositsAndWithdrawalsCount.withdrawals_count_save(Reader.withdrawals_count())
  end

  defp imported_operation?(op, calling_module) do
    {hash_field, block_field} =
      case calling_module do
        Indexer.Fetcher.Shibarium.L1 -> {:l1_transaction_hash, :l1_block_number}
        Indexer.Fetcher.Shibarium.L2 -> {:l2_transaction_hash, :l2_block_number}
      end

    transaction_hash = Map.fetch!(op, hash_field)
    block_number = Map.fetch!(op, block_field)

    Repo.exists?(
      from(sb in Bridge,
        where:
          sb.operation_hash == ^op.operation_hash and sb.operation_type == ^op.operation_type and
            field(sb, ^hash_field) == ^transaction_hash and field(sb, ^block_field) == ^block_number
      )
    )
  end

  # credo:disable-for-next-line Credo.Check.Refactor.CyclomaticComplexity
  defp bind_existing_operation_in_db(op, calling_module) do
    {query, set} = make_query_for_bind(op, calling_module)

    updated_count =
      try do
        {updated_count, _} =
          Repo.update_all(
            from(b in Bridge,
              join: s in subquery(query),
              on:
                b.operation_hash == s.operation_hash and b.l1_transaction_hash == s.l1_transaction_hash and
                  b.l2_transaction_hash == s.l2_transaction_hash
            ),
            set: set
          )

        updated_count
      rescue
        error in Postgrex.Error ->
          # if this is unique violation, we just ignore such an operation as it was inserted before
          if error.postgres.code != :unique_violation do
            reraise error, __STACKTRACE__
          end
      end

    # increment the cached count of complete rows
    case !is_nil(updated_count) && updated_count > 0 && op.operation_type do
      :deposit -> DepositsAndWithdrawalsCount.deposits_count_save(updated_count, true)
      :withdrawal -> DepositsAndWithdrawalsCount.withdrawals_count_save(updated_count, true)
      false -> nil
    end

    updated_count
  end

  defp make_query_for_bind(op, calling_module) when calling_module == Indexer.Fetcher.Shibarium.L1 do
    query =
      from(sb in Bridge,
        where:
          sb.operation_hash == ^op.operation_hash and sb.operation_type == ^op.operation_type and
            sb.l2_transaction_hash != ^@empty_hash and sb.l1_transaction_hash == ^@empty_hash,
        order_by: [asc: sb.l2_block_number],
        limit: 1
      )

    set =
      [l1_transaction_hash: op.l1_transaction_hash, l1_block_number: op.l1_block_number] ++
        if(op.operation_type == :deposit, do: [timestamp: op.timestamp], else: [])

    {query, set}
  end

  defp make_query_for_bind(op, calling_module) when calling_module == Indexer.Fetcher.Shibarium.L2 do
    query =
      from(sb in Bridge,
        where:
          sb.operation_hash == ^op.operation_hash and sb.operation_type == ^op.operation_type and
            sb.l1_transaction_hash != ^@empty_hash and sb.l2_transaction_hash == ^@empty_hash,
        order_by: [asc: sb.l1_block_number],
        limit: 1
      )

    set =
      [l2_transaction_hash: op.l2_transaction_hash, l2_block_number: op.l2_block_number] ++
        if(op.operation_type == :withdrawal, do: [timestamp: op.timestamp], else: [])

    {query, set}
  end
end
