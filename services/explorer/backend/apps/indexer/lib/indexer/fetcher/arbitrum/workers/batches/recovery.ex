defmodule Indexer.Fetcher.Arbitrum.Workers.Batches.Recovery do
  @moduledoc false

  import Ecto.Query

  alias Explorer.Chain.Arbitrum.{BatchBlock, BatchToDaBlob, BatchTransaction, DaMultiPurposeRecord, L1Batch, Message}
  alias Explorer.Chain.Import.Runner.Arbitrum, as: Runners
  alias Explorer.Repo

  @da_outcome_type 2

  def da_outcome_record(batch) do
    %{
      data_key: :crypto.hash(:sha256, "arbitrum:batch-da-outcome:#{batch.number}"),
      data_type: @da_outcome_type,
      batch_number: batch.number,
      data:
        Map.merge(batch.da_outcome, %{
          transaction_hash: "0x" <> Base.encode16(batch.transaction_hash, case: :lower),
          block_number: batch.l1_block_number,
          attempted_at: DateTime.to_iso8601(DateTime.utc_now())
        })
    }
  end

  def da_outcome(batch_number) do
    Repo.one(
      from(r in DaMultiPurposeRecord,
        where: r.data_type == @da_outcome_type and r.batch_number == ^batch_number,
        select: r.data
      )
    )
  end

  def unresolved_l1_blocks(limit) do
    Repo.all(
      from(b in L1Batch,
        join: t in assoc(b, :commitment_transaction),
        left_join: r in DaMultiPurposeRecord,
        on: r.batch_number == b.number and r.data_type == @da_outcome_type,
        where: is_nil(b.batch_container) or fragment("?->>'retry' = 'true'", r.data),
        order_by: [asc: fragment("COALESCE(?, ?)", r.updated_at, b.updated_at), asc: b.number],
        limit: ^limit,
        select: t.block_number
      )
    )
    |> Enum.uniq()
  end

  def reconcile(prepare) do
    {:ok, result} =
      Repo.transaction(
        fn ->
          lock()
          {batches, lifecycle, blocks, transactions, messages, da, links} = data = prepare.()
          numbers = Enum.map(batches, & &1.number)
          old_blocks = Repo.all(from(b in BatchBlock, where: b.batch_number in ^numbers))
          old_batches = Repo.all(from(b in L1Batch, where: b.number in ^numbers)) |> Map.new(&{&1.number, &1})

          confirmations =
            Map.new(old_blocks, fn block ->
              {{block.batch_number, block.block_number}, block.confirmation_id}
            end)

          blocks =
            Enum.map(blocks, fn block ->
              old = old_batches[block.batch_number]
              batch = Enum.find(batches, &(&1.number == block.batch_number))

              if old && to_string(old.before_acc) == batch.before_acc && to_string(old.after_acc) == batch.after_acc do
                Map.put(block, :confirmation_id, confirmations[{block.batch_number, block.block_number}])
              else
                block
              end
            end)

          clear_associations(numbers)
          hashes = Enum.map(transactions, & &1.transaction_hash)
          Repo.delete_all(from(t in BatchTransaction, where: t.transaction_hash in ^hashes))

          options = %{timeout: :infinity, timestamps: Explorer.Chain.Import.timestamps()}
          import_records(Runners.LifecycleTransactions, lifecycle, options)
          import_records(Runners.L1Batches, batches, options)
          import_records(Runners.BatchBlocks, blocks, options)
          import_records(Runners.BatchTransactions, transactions, options)
          import_records(Runners.DaMultiPurposeRecords, da, options)
          import_records(Runners.BatchToDaBlobs, links, options)

          revoke_uncommitted_messages(Enum.map(old_blocks, & &1.block_number))

          messages =
            Enum.filter(messages, fn message ->
              Repo.exists?(
                from(b in BatchBlock, where: b.block_number == ^message.originating_transaction_block_number)
              )
            end)

          import_records(Runners.Messages, messages, options)
          data
        end,
        timeout: :infinity
      )

    result
  end

  def remove_orphaned_batches(first, last, canonical_numbers) do
    {:ok, _} =
      Repo.transaction(
        fn ->
          lock()

          numbers =
            Repo.all(
              from(b in L1Batch,
                join: t in assoc(b, :commitment_transaction),
                where: t.block_number >= ^first and t.block_number <= ^last and b.number not in ^canonical_numbers,
                select: b.number
              )
            )

          blocks = Repo.all(from(b in BatchBlock, where: b.batch_number in ^numbers, select: b.block_number))
          clear_associations(numbers)

          Repo.delete_all(
            from(r in DaMultiPurposeRecord, where: r.batch_number in ^numbers and r.data_type == @da_outcome_type)
          )

          Repo.update_all(from(r in DaMultiPurposeRecord, where: r.batch_number in ^numbers), set: [batch_number: nil])
          Repo.delete_all(from(b in L1Batch, where: b.number in ^numbers))
          revoke_uncommitted_messages(blocks)
        end,
        timeout: :infinity
      )

    :ok
  end

  defp clear_associations(numbers) do
    Repo.delete_all(from(b in BatchToDaBlob, where: b.batch_number in ^numbers))
    Repo.delete_all(from(b in BatchTransaction, where: b.batch_number in ^numbers))
    Repo.delete_all(from(b in BatchBlock, where: b.batch_number in ^numbers))
  end

  defp revoke_uncommitted_messages(blocks) do
    committed = from(b in BatchBlock, select: b.block_number)

    Repo.update_all(
      from(m in Message,
        where:
          m.direction == :from_l2 and m.status == :sent and m.originating_transaction_block_number in ^blocks and
            m.originating_transaction_block_number not in subquery(committed)
      ),
      set: [status: :initiated, updated_at: DateTime.utc_now()]
    )
  end

  defp import_records(runner, params, options) do
    schema = runner.ecto_schema_module()

    changes =
      Enum.map(params, fn attrs ->
        changeset = schema.changeset(struct(schema), attrs)

        if changeset.valid? do
          changeset.changes
        else
          Repo.rollback({:invalid_batch_record, changeset})
        end
      end)

    {:ok, _} = runner.insert(Repo, changes, options)
  end

  defp lock, do: Repo.query!("SELECT pg_advisory_xact_lock(2233547)")
end
