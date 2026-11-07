defmodule Explorer.Chain.PaxeerX.ReceiptTest do
  use Explorer.DataCase

  alias Ecto.Multi
  alias Explorer.Chain.PaxeerX.Receipt
  alias Explorer.Chain.PaxeerX.Receipt.KernelCursor

  @cursor_identity String.duplicate("0a", 32)

  describe "historical archive rows" do
    test "inserts a receipt row keyed by transaction hash and log index" do
      %{block: block, transaction: transaction} = block_with_transaction()

      assert {:ok, %{insert_paxeer_x_receipts: [inserted]}} = run_changes([attributes(block, transaction)])

      assert inserted.transaction_hash == transaction.hash
      assert inserted.log_index == 0
      assert inserted.status == :batch_included
      assert inserted.block_consensus
    end

    test "leaves the stored row untouched when the same log is imported again" do
      %{block: block, transaction: transaction} = block_with_transaction()

      attrs = attributes(block, transaction)

      assert {:ok, %{insert_paxeer_x_receipts: [_]}} = run_changes([attrs])

      assert {:ok, %{insert_paxeer_x_receipts: []}} =
               run_changes([Map.put(attrs, :status, :settlement_anchored)])

      assert [stored] = Repo.all(Receipt)
      assert stored.status == :batch_included
    end

    test "handles an empty changes list" do
      assert {:ok, %{insert_paxeer_x_receipts: []}} = run_changes([])
    end
  end

  describe "only_consensus_query/0" do
    test "drops the rows of a block that lost consensus" do
      %{block: kept_block, transaction: kept_transaction} = block_with_transaction()
      %{block: reorged_block, transaction: reorged_transaction} = block_with_transaction()

      assert {:ok, _} =
               run_changes([
                 attributes(kept_block, kept_transaction),
                 attributes(reorged_block, reorged_transaction)
               ])

      Repo.update!(Ecto.Changeset.change(reorged_block, consensus: false))

      assert [kept] = Repo.all(Receipt.only_consensus_query())
      assert kept.transaction_hash == kept_transaction.hash

      assert {1, nil} =
               [reorged_block.hash]
               |> Receipt.lose_consensus_query()
               |> Repo.update_all(set: [block_consensus: false])
    end
  end

  describe "KernelCursor.changeset/2" do
    test "admits a listed refusal code and any relay status code" do
      for code <- ["native_verification_failed", "cursor_changed", "relay_status_404", "relay_status_503"] do
        changeset = KernelCursor.changeset(%KernelCursor{}, cursor_attributes(%{refusal_code: code, refused_batch: 3}))

        assert changeset.valid?
      end
    end

    test "refuses an unlisted refusal code and a status code outside 100..599" do
      for code <- ["accepted", "relay_status_600", "relay_status_42", "RELAY_STATUS_404"] do
        changeset = KernelCursor.changeset(%KernelCursor{}, cursor_attributes(%{refusal_code: code, refused_batch: 3}))

        refute changeset.valid?
        assert {"is invalid", _} = changeset.errors[:refusal_code]
      end
    end
  end

  defp cursor_attributes(attributes) do
    Map.merge(%{source: "https://relay.invalid", source_identity: @cursor_identity, last_batch: 2}, attributes)
  end

  test "live import refuses receipt archive writes" do
    previous = Application.get_env(:explorer, :chain_type)
    Application.put_env(:explorer, :chain_type, :paxeer_x)
    on_exit(fn -> Application.put_env(:explorer, :chain_type, previous) end)
    alias Explorer.Chain.Import.Runner.PaxeerX.{AccountBindings, MarketEvents}
    assert Explorer.Chain.Import.Stage.ChainTypeSpecific.runners() == [AccountBindings, MarketEvents]

    {[multi], remaining} =
      Explorer.Chain.Import.Stage.ChainTypeSpecific.multis(
        %{Explorer.Chain.Import.Runner.PaxeerX.Receipts => [%{}]},
        %{}
      )

    assert Ecto.Multi.to_list(multi) == []
    assert remaining == %{Explorer.Chain.Import.Runner.PaxeerX.Receipts => [%{}]}
    refute function_exported?(Receipt, :changeset, 2)
    %{block: block, transaction: transaction} = block_with_transaction()
    assert {:error, _} = Explorer.Chain.Import.all(%{paxeer_x_receipts: %{params: [attributes(block, transaction)]}})
    assert Repo.aggregate(Receipt, :count) == 0
  end

  defp block_with_transaction do
    block = insert(:block)
    transaction = :transaction |> insert() |> with_block(block)

    %{block: block, transaction: transaction}
  end

  defp attributes(block, transaction) do
    %{
      transaction_hash: transaction.hash,
      log_index: 0,
      block_hash: block.hash,
      block_number: block.number,
      block_consensus: true,
      receipt_id: block_hash(),
      account: block_hash(),
      payload_hash: block_hash(),
      status: :batch_included
    }
  end

  defp run_changes(changes) when is_list(changes) do
    rows =
      Enum.map(changes, fn attrs ->
        Ecto.Changeset.cast(%Receipt{}, attrs, Map.keys(attrs))
        |> Ecto.Changeset.apply_changes()
        |> Map.from_struct()
        |> Map.take(Map.keys(attrs))
        |> Map.merge(%{inserted_at: DateTime.utc_now(), updated_at: DateTime.utc_now()})
      end)

    Multi.new()
    |> Multi.run(:insert_paxeer_x_receipts, fn repo, _ ->
      {_count, inserted} =
        repo.insert_all(Receipt, rows,
          on_conflict: :nothing,
          returning: true
        )

      {:ok, inserted}
    end)
    |> Repo.transaction()
  end
end
