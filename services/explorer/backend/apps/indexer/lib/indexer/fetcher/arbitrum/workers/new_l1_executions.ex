defmodule Indexer.Fetcher.Arbitrum.Workers.NewL1Executions do
  @moduledoc """
    Coordinates the discovery and processing of new and historical L2-to-L1 message executions for an Arbitrum rollup.

    This module is responsible for identifying and importing executions of messages
    that were initiated from Arbitrum's Layer 2 (L2) and are to be relayed to
    Layer 1 (L1). It handles both new executions that are currently occurring on L1
    and historical executions that occurred in the past but have not yet been
    processed.

    Discovery of these message executions involves parsing logs for
    `OutBoxTransactionExecuted` events emitted by the Arbitrum outbox contract. As
    the logs do not provide comprehensive data for constructing the related
    lifecycle transactions, the module executes batched RPC calls to
    `eth_getBlockByNumber`, using the responses to obtain transaction timestamps,
    thereby enriching the lifecycle transaction data.
  """

  import Ecto.Query
  import EthereumJSONRPC, only: [quantity_to_integer: 1]
  alias EthereumJSONRPC.Arbitrum.Constants.Events, as: ArbitrumEvents

  import Indexer.Fetcher.Arbitrum.Utils.Logging, only: [log_info: 1, log_debug: 1]

  alias EthereumJSONRPC.Block.ByNumber, as: BlockByNumber

  import Explorer.Helper, only: [decode_data: 2]

  alias Indexer.Fetcher.Arbitrum.Utils.Db.Messages, as: DbMessages
  alias Indexer.Fetcher.Arbitrum.Utils.Db.ParentChainTransactions, as: DbParentChainTransactions
  alias Indexer.Fetcher.Arbitrum.Utils.Helper, as: ArbitrumHelper
  alias Indexer.Fetcher.Arbitrum.Utils.Rpc
  alias Indexer.Fetcher.Arbitrum.Workers.Confirmations.Tasks, as: ConfirmationsTasks
  alias Indexer.Helper, as: IndexerHelper

  alias Explorer.Chain.Arbitrum
  alias Explorer.Repo
  alias Explorer.Chain.Import.Runner.Arbitrum.{L1Executions, LifecycleTransactions, Messages}

  require Logger

  @type new_executions_data_map :: %{
          :start_block => non_neg_integer()
        }

  @type historical_executions_data_map :: %{
          :end_block => non_neg_integer()
        }

  @typep executions_related_state :: %{
           :config => %{
             :l1_outbox_address => binary(),
             :l1_rollup_init_block => non_neg_integer(),
             :l1_rpc => %{
               :json_rpc_named_arguments => EthereumJSONRPC.json_rpc_named_arguments(),
               :logs_block_range => non_neg_integer(),
               optional(any()) => any()
             },
             :rollup_first_block => non_neg_integer(),
             optional(any()) => any()
           },
           :task_data => %{
             :new_executions => new_executions_data_map(),
             :historical_executions => historical_executions_data_map(),
             :historical_confirmations => ConfirmationsTasks.historical_confirmations_data_map(),
             optional(any()) => any()
           },
           optional(any()) => any()
         }

  @doc """
    Discovers and processes new executions of L2-to-L1 messages within the current L1 block range.

    This function fetches logs for `OutBoxTransactionExecuted` events within the
    specified L1 block range to identify new execution transactions for L2-to-L1
    messages, updating their status and linking them with corresponding lifecycle
    transactions in the database. Additionally, the function checks unexecuted
    L2-to-L1 messages to match them with any newly recorded executions and updates
    their status to `:relayed`.

    ## Parameters
    - A map containing:
      - `config`: Configuration settings including the Arbitrum outbox contract
                  address, JSON RPC arguments, and the block range for fetching
                  logs.
      - `data`: Contains the starting block number for new execution discovery.

    ## Returns
    - `{:ok, state}`: On successful discovery and processing, where `state` includes
      an updated `start_block` for the next iteration if new blocks were processed,
      or remains unchanged if no new blocks were found on L1.
  """
  @spec discover_new_l1_messages_executions(executions_related_state()) :: {:ok, executions_related_state()}
  def discover_new_l1_messages_executions(
        %{
          config: %{
            l1_rpc: l1_rpc_config,
            l1_outbox_address: outbox_address
          },
          task_data: %{new_executions: %{start_block: start_block}}
        } = state
      ) do
    # Requesting the "latest" block instead of "safe" allows to catch executions
    # without latency.
    {:ok, latest_block} =
      IndexerHelper.get_block_number_by_tag(
        "latest",
        l1_rpc_config.json_rpc_named_arguments,
        Rpc.get_resend_attempts()
      )

    window = Map.get(l1_rpc_config, :execution_reorg_depth, l1_rpc_config.logs_block_range)
    first_block = max(state.config.l1_rollup_init_block, min(start_block, latest_block + 1) - window)
    end_block = min(start_block + l1_rpc_config.logs_block_range - 1, latest_block)

    if first_block <= end_block do
      with {:ok, identities} <- reconcile_executions(outbox_address, first_block, end_block, start_block, l1_rpc_config) do
        {:ok,
         ArbitrumHelper.update_fetcher_task_data(state, :new_executions, %{
           start_block: end_block + 1,
           observed_blocks: identities
         })}
      end
    else
      {:ok, state}
    end
  end

  defp reconcile_executions(outbox, first, last, cursor, rpc) do
    with {:ok, blocks} <- execution_blocks(first, last, rpc),
         logs <- get_logs_for_new_executions(first, last, outbox, rpc.json_rpc_named_arguments),
         true <- canonical_execution_logs?(logs, blocks),
         {:ok, ^blocks} <- execution_blocks(first, last, rpc) do
      upper = max(last, min(cursor - 1, last + Map.get(rpc, :execution_reorg_depth, rpc.logs_block_range)))

      case Repo.transaction(fn ->
             # Serializes executions and revocations across new/historical worker runs.
             Repo.query!("SELECT pg_advisory_xact_lock(2233546)")
             {transactions, executions} = get_executions_from_logs(logs, rpc, blocks)
             revoke_executions(first, upper, Enum.map(executions, & &1.message_id))
             now = DateTime.utc_now()
             options = %{timeout: :infinity, timestamps: %{inserted_at: now, updated_at: now}}
             {:ok, _} = import_execution_records(LifecycleTransactions, transactions, options)
             {:ok, _} = import_execution_records(L1Executions, executions, options)
             {:ok, _} = import_execution_records(Messages, get_relayed_messages(), options)
           end) do
        {:ok, _} -> {:ok, Map.new(blocks, fn {number, block} -> {number, block["hash"]} end)}
        {:error, reason} -> {:error, reason}
      end
    else
      {:error, reason} -> {:error, reason}
      _ -> {:error, :inconsistent_execution_blocks}
    end
  end

  defp import_execution_records(runner, params, options) do
    schema = runner.ecto_schema_module()

    changes =
      Enum.map(params, fn attrs ->
        changeset = schema.changeset(struct(schema), attrs)

        if changeset.valid? do
          changeset.changes
        else
          Repo.rollback({:invalid_execution_record, changeset})
        end
      end)

    runner.insert(Repo, changes, options)
  end

  defp execution_blocks(first, last, rpc) do
    first..last
    |> Enum.chunk_every(rpc.chunk_size)
    |> Enum.reduce_while({:ok, %{}}, fn numbers, {:ok, acc} ->
      requests = Enum.map(numbers, &BlockByNumber.request(%{id: &1, number: &1}, false))

      with {:ok, responses} <- EthereumJSONRPC.json_rpc(requests, rpc.json_rpc_named_arguments),
           true <- Enum.sort(Enum.map(responses, & &1.id)) == numbers,
           true <- Enum.all?(responses, &valid_execution_block?/1) do
        blocks = Map.new(responses, fn %{id: number, result: block} -> {number, block} end)
        {:cont, {:ok, Map.merge(acc, blocks)}}
      else
        {:error, reason} -> {:halt, {:error, reason}}
        _ -> {:halt, {:error, :incomplete_execution_blocks}}
      end
    end)
    |> case do
      {:ok, blocks} ->
        if Enum.all?(first..last, fn number ->
             number == first or blocks[number]["parentHash"] == blocks[number - 1]["hash"]
           end) do
          {:ok, blocks}
        else
          {:error, :inconsistent_execution_blocks}
        end

      error ->
        error
    end
  end

  defp valid_execution_block?(%{id: number, result: %{"number" => raw, "hash" => hash, "timestamp" => timestamp}}) do
    quantity_to_integer(raw) == number and is_binary(hash) and is_binary(timestamp)
  end

  defp valid_execution_block?(_), do: false

  defp canonical_execution_logs?(logs, blocks) do
    Enum.all?(logs, fn log ->
      block = blocks[quantity_to_integer(log["blockNumber"])]
      not is_nil(block) and log["removed"] != true and log["blockHash"] == block["hash"]
    end)
  end

  defp revoke_executions(first, last, incoming_ids) do
    previous =
      Repo.all(
        from(e in Arbitrum.L1Execution,
          join: tx in Arbitrum.LifecycleTransaction,
          on: tx.id == e.execution_id,
          where: (tx.block_number >= ^first and tx.block_number <= ^last) or e.message_id in ^incoming_ids,
          order_by: e.message_id,
          select: {e.message_id, tx.hash},
          lock: fragment("FOR UPDATE OF ?", e)
        )
      )

    Enum.each(previous, fn {id, hash} ->
      Repo.update_all(
        from(m in Arbitrum.Message,
          where: m.direction == :from_l2 and m.message_id == ^id and m.completion_transaction_hash == ^hash
        ),
        set: [status: :confirmed, completion_transaction_hash: nil, updated_at: DateTime.utc_now()]
      )
    end)

    ids = Enum.map(previous, &elem(&1, 0))
    Repo.delete_all(from(e in Arbitrum.L1Execution, where: e.message_id in ^ids))
  end

  @doc """
    Discovers and processes historical executions of L2-to-L1 messages within a calculated L1 block range.

    This function fetches logs for `OutBoxTransactionExecuted` events within the
    calculated L1 block range. It then processes these logs to identify execution
    transactions for L2-to-L1 messages, updating their status and linking them with
    corresponding lifecycle transactions in the database. Additionally, the
    function goes through unexecuted L2-to-L1 messages, matches them with the
    executions recorded in the database up to this moment, and updates the messages'
    status to `:relayed`.

    ## Parameters
    - A map containing:
      - `config`: Configuration settings including the Arbitrum outbox contract
        address, the initialization block for the rollup, and JSON RPC arguments.
      - `data`: Contains the ending block number for the historical execution
                discovery.

    ## Returns
    - `{:ok, state}`: On successful discovery and processing, where `state` includes
      an updated `end_block` for the next iteration. If executions were found,
      `end_block` is set to the block before the last processed block. If the
      historical discovery process has reached the lowest L1 block that needs to
      be checked, `end_block` is set to one block before that lowest block.
  """
  @spec discover_historical_l1_messages_executions(executions_related_state()) :: {:ok, executions_related_state()}
  def discover_historical_l1_messages_executions(
        %{
          config: %{
            l1_rpc: l1_rpc_config,
            l1_outbox_address: outbox_address
          },
          task_data: %{historical_executions: %{end_block: end_block}}
        } = state
      ) do
    # This is used to optimize historical discovery processes by avoiding scanning
    # blocks before the first possible confirmation. Since cross-chain message
    # executions on the parent chain cannot occur before their corresponding L2
    # blocks are confirmed, this provides a safe lower bound for message execution
    # discovery.
    {lowest_l1_block, new_state} = ConfirmationsTasks.get_lowest_l1_block_for_confirmations(state)

    data_for_next_iteration =
      if end_block >= lowest_l1_block do
        start_block = max(lowest_l1_block, end_block - l1_rpc_config.logs_block_range + 1)

        log_info("Block range for historical l2-to-l1 messages executions discovery: #{start_block}..#{end_block}")

        discover(outbox_address, start_block, end_block, l1_rpc_config)

        # The next iteration will consider the block range which ends by the block
        # before the last processed block
        %{end_block: start_block - 1}
      else
        # The historical discovery process has reached the lowest L1 block that
        # needs to be checked for executions
        %{end_block: lowest_l1_block - 1}
      end

    {:ok, ArbitrumHelper.update_fetcher_task_data(new_state, :historical_executions, data_for_next_iteration)}
  end

  @doc """
    Determines whether the historical executions discovery process has completed.

    This function checks if the end block of historical executions discovery has
    reached below the lowest L1 block that needs to be checked for executions.
    When this happens, it means we have searched back far enough in history and can
    stop the historical discovery process.

    ## Parameters
    - A map containing:
      - `task_data`: Contains historical executions data with an end block
      - Other configuration needed to determine the lowest L1 block

    ## Returns
    - `true` if the end block is less than the lowest L1 block that needs checking
    - `false` otherwise
  """
  @spec historical_executions_discovery_completed?(executions_related_state()) :: boolean()
  def historical_executions_discovery_completed?(
        %{
          task_data: %{historical_executions: %{end_block: end_block}}
        } = state
      ) do
    {lowest_l1_block, _} = ConfirmationsTasks.get_lowest_l1_block_for_confirmations(state)

    end_block < lowest_l1_block
  end

  # Discovers and imports execution transactions for L2-to-L1 messages within a specified L1 block range.
  #
  # This function fetches logs for `OutBoxTransactionExecuted` events within the
  # specified L1 block range to discover new execution transactions. It processes
  # these logs to extract execution details and associated lifecycle transactions,
  # which are then imported into the database. For lifecycle timestamps not
  # available in the logs, RPC calls to `eth_getBlockByNumber` are made to fetch
  # the necessary data. Furthermore, the function checks unexecuted L2-to-L1
  # messages to match them with any recorded executions, updating their status to
  # `:relayed` and establishing links with the corresponding lifecycle
  # transactions. These updated messages are also imported into the database.
  #
  # ## Parameters
  # - `outbox_address`: The address of the Arbitrum outbox contract to filter the
  #                     logs.
  # - `start_block`: The starting block number for log retrieval.
  # - `end_block`: The ending block number for log retrieval.
  # - `l1_rpc_config`: Configuration parameters including JSON RPC arguments and
  #                    settings for processing the logs.
  #
  # ## Returns
  # - N/A
  defp discover(outbox_address, start_block, end_block, l1_rpc_config) do
    {:ok, _} =
      reconcile_executions(outbox_address, start_block, end_block, end_block + 1, l1_rpc_config)
  end

  # Retrieves logs representing `OutBoxTransactionExecuted` events between the specified blocks.
  defp get_logs_for_new_executions(start_block, end_block, outbox_address, json_rpc_named_arguments)
       when start_block <= end_block do
    {:ok, logs} =
      IndexerHelper.get_logs(
        start_block,
        end_block,
        outbox_address,
        [ArbitrumEvents.outbox_transaction_executed()],
        json_rpc_named_arguments
      )

    unless Enum.empty?(logs) do
      log_debug("Found #{length(logs)} OutBoxTransactionExecuted logs")
    end

    logs
  end

  # Extracts and processes execution details from logs for L2-to-L1 message transactions.
  #
  # This function parses logs representing `OutBoxTransactionExecuted` events to
  # extract basic execution details. It then requests block timestamps and
  # associates them with the extracted executions, forming lifecycle transactions
  # enriched with timestamps and finalization statuses. Subsequently, unique
  # identifiers for the lifecycle transactions are determined, and the connection
  # between execution records and lifecycle transactions is established.
  #
  # ## Parameters
  # - `logs`: A collection of log entries to be processed.
  # - `l1_rpc_config`: Configuration parameters including JSON RPC arguments,
  #   chunk size for RPC calls, and a flag indicating whether to track the
  #   finalization of transactions.
  #
  # ## Returns
  # - A tuple containing:
  #   - A list of lifecycle transactions with updated timestamps, finalization
  #     statuses, and unique identifiers.
  #   - A list of detailed execution information for L2-to-L1 messages.
  # Both lists are prepared for database importation.
  @spec get_executions_from_logs([%{String.t() => any()}], map(), map()) ::
          {[Arbitrum.LifecycleTransaction.to_import()], [Arbitrum.L1Execution.to_import()]}
  defp get_executions_from_logs(logs, l1_rpc_config, blocks)

  defp get_executions_from_logs([], _, _), do: {[], []}

  defp get_executions_from_logs(logs, %{track_finalization: track_finalization?}, blocks) do
    {basics_executions, basic_lifecycle_transactions, _blocks_requests} = parse_logs_for_new_executions(logs)

    blocks_to_ts =
      Map.new(blocks, fn {number, block} ->
        {number, EthereumJSONRPC.timestamp_to_datetime(block["timestamp"])}
      end)

    lifecycle_transactions =
      basic_lifecycle_transactions
      |> ArbitrumHelper.extend_lifecycle_transactions_with_ts_and_status(blocks_to_ts, track_finalization?)
      |> DbParentChainTransactions.get_indices_for_l1_transactions()

    executions =
      basics_executions
      |> Enum.reduce([], fn execution, updated_executions ->
        updated =
          execution
          |> Map.put(:execution_id, lifecycle_transactions[execution.execution_transaction_hash].id)
          |> Map.drop([:execution_transaction_hash])

        [updated | updated_executions]
      end)

    {Map.values(lifecycle_transactions), executions}
  end

  # Parses logs to extract new execution transactions for L2-to-L1 messages.
  #
  # This function processes log entries to identify `OutBoxTransactionExecuted`
  # events, extracting the message ID, transaction hash, and block number for
  # each. It accumulates this data into execution details, lifecycle
  # transaction descriptions, and RPC requests for block information. These
  # are then used in  subsequent steps to finalize the execution status of the
  # messages.
  #
  # ## Parameters
  # - `logs`: A collection of log entries to be processed.
  #
  # ## Returns
  # - A tuple containing:
  #   - `executions`: A list of details for execution transactions related to
  #     L2-to-L1 messages.
  #   - `lifecycle_transactions`: A map of lifecycle transaction details, keyed by L1
  #     transaction hash.
  #   - `blocks_requests`: A list of RPC requests for fetching block data where
  #     the executions occurred.
  defp parse_logs_for_new_executions(logs) do
    {executions, lifecycle_transactions, blocks_requests} =
      logs
      |> Enum.reduce({[], %{}, %{}}, fn event, {executions, lifecycle_transactions, blocks_requests} ->
        msg_id = outbox_transaction_executed_event_parse(event)

        l1_transaction_hash_raw = event["transactionHash"]
        l1_transaction_hash = Rpc.string_hash_to_bytes_hash(l1_transaction_hash_raw)
        l1_blk_num = quantity_to_integer(event["blockNumber"])

        updated_executions = [
          %{
            message_id: msg_id,
            execution_transaction_hash: l1_transaction_hash
          }
          | executions
        ]

        updated_lifecycle_transactions =
          Map.put(
            lifecycle_transactions,
            l1_transaction_hash,
            %{hash: l1_transaction_hash, block_number: l1_blk_num}
          )

        updated_blocks_requests =
          Map.put(
            blocks_requests,
            l1_blk_num,
            BlockByNumber.request(%{id: 0, number: l1_blk_num}, false, true)
          )

        log_debug("Execution for L2 message ##{msg_id} found in #{l1_transaction_hash_raw}")

        {updated_executions, updated_lifecycle_transactions, updated_blocks_requests}
      end)

    {executions, lifecycle_transactions, Map.values(blocks_requests)}
  end

  # Parses `OutBoxTransactionExecuted` event data to extract the transaction index parameter
  defp outbox_transaction_executed_event_parse(event) do
    [transaction_index] = decode_data(event["data"], ArbitrumEvents.outbox_transaction_executed_unindexed_params())

    transaction_index
  end

  # Retrieves unexecuted messages from L2 to L1, marking them as completed if their
  # corresponding execution transactions are identified.
  #
  # This function fetches messages confirmed on L1 and matches these messages with
  # their corresponding execution transactions. For matched pairs, it updates the
  # message status to `:relayed` and links them with the execution transactions.
  #
  # ## Returns
  # - A list of messages marked as completed, ready for database import.
  @spec get_relayed_messages() :: [Arbitrum.Message.to_import()]
  defp get_relayed_messages do
    # Assuming that both catchup block fetcher and historical messages catchup fetcher
    # will check all discovered historical messages to be marked as executed it is not
    # needed to handle :initiated and :sent of historical messages here, only for
    # new messages discovered and changed their status from `:sent` to `:confirmed`
    confirmed_messages = DbMessages.confirmed_l2_to_l1_messages()

    if Enum.empty?(confirmed_messages) do
      []
    else
      log_debug("Identified #{length(confirmed_messages)} l2-to-l1 messages already confirmed but not completed")

      messages_map =
        confirmed_messages
        |> Enum.reduce(%{}, fn msg, acc ->
          Map.put(acc, msg.message_id, msg)
        end)

      messages_map
      |> Map.keys()
      |> DbMessages.l1_executions()
      |> Enum.map(fn execution ->
        messages_map
        |> Map.get(execution.message_id)
        |> Map.put(:completion_transaction_hash, execution.execution_transaction.hash.bytes)
        |> Map.put(:status, :relayed)
      end)
    end
  end
end
