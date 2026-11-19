defmodule Explorer.Migrator.DeleteZeroValueInternalTransactionsTest do
  use Explorer.DataCase, async: false

  alias Explorer.Chain.InternalTransaction
  alias Explorer.Migrator.{DeleteZeroValueInternalTransactions, MigrationStatus}
  alias Explorer.Repo
  alias Explorer.Utility.{AddressIdToAddressHash, InternalTransactionsAddressPlaceholder}

  setup do
    settings =
      Enum.map(
        [Explorer.Migrator.ShrinkInternalTransactions, DeleteZeroValueInternalTransactions],
        &{&1, Application.fetch_env(:explorer, &1)}
      )

    Application.put_env(:explorer, Explorer.Migrator.ShrinkInternalTransactions, enabled: false)

    Application.put_env(:explorer, DeleteZeroValueInternalTransactions,
      batch_size: 1,
      storage_period: :timer.hours(24 * 30),
      check_interval: :timer.minutes(1),
      dependency_check_interval: 10
    )

    on_exit(fn ->
      Enum.each(settings, fn
        {module, {:ok, config}} -> Application.put_env(:explorer, module, config)
        {module, :error} -> Application.delete_env(:explorer, module)
      end)
    end)

    :ok
  end

  test "Deletes zero value calls" do
    address_1 = insert(:address)
    address_2 = insert(:address)
    address_3 = insert(:address)

    block = insert(:block, number: 1, timestamp: Timex.shift(Timex.now(), days: -40))

    Enum.map(1..3, fn i ->
      transaction =
        :transaction
        |> insert()
        |> with_block(block)

      insert(:internal_transaction,
        index: 10,
        transaction: transaction,
        block_hash: transaction.block_hash,
        block_number: transaction.block_number,
        from_address: address_1,
        to_address: address_2,
        transaction_index: transaction.index,
        type: :call,
        value: 0
      )
    end)

    Enum.map(1..4, fn i ->
      transaction =
        :transaction
        |> insert()
        |> with_block(block)

      insert(:internal_transaction,
        index: 10,
        transaction: transaction,
        block_hash: transaction.block_hash,
        block_number: transaction.block_number,
        from_address: address_2,
        to_address: address_3,
        transaction_index: transaction.index,
        type: :call,
        value: 0
      )
    end)

    Enum.map(1..5, fn i ->
      transaction =
        :transaction
        |> insert()
        |> with_block(block)

      insert(:internal_transaction,
        index: 10,
        transaction: transaction,
        block_hash: transaction.block_hash,
        block_number: transaction.block_number,
        from_address: address_3,
        to_address: address_1,
        transaction_index: transaction.index,
        type: :call,
        value: 0
      )
    end)

    recent_block = insert(:block, number: 2, timestamp: Timex.now())

    Enum.map(1..4, fn i ->
      transaction =
        :transaction
        |> insert()
        |> with_block(recent_block)

      insert(:internal_transaction,
        index: 10,
        transaction: transaction,
        block_hash: transaction.block_hash,
        block_number: transaction.block_number,
        transaction_index: transaction.index,
        type: :call,
        value: 1
      )
    end)

    Enum.map(1..5, fn i ->
      transaction =
        :transaction
        |> insert()
        |> with_block(recent_block)

      insert(:internal_transaction_create,
        index: 10,
        transaction: transaction,
        block_hash: transaction.block_hash,
        block_number: transaction.block_number,
        transaction_index: transaction.index,
        value: 0
      )
    end)

    Enum.map(1..6, fn i ->
      transaction =
        :transaction
        |> insert()
        |> with_block(recent_block)

      insert(:internal_transaction,
        index: 10,
        transaction: transaction,
        block_hash: transaction.block_hash,
        block_number: transaction.block_number,
        transaction_index: transaction.index,
        type: :call,
        value: 0
      )
    end)

    assert MigrationStatus.get_status("delete_zero_value_internal_transactions") == nil

    start_supervised!(DeleteZeroValueInternalTransactions)
    wait_for_migration(15)
    stop_supervised!(DeleteZeroValueInternalTransactions)

    all_internal_transactions = Repo.all(InternalTransaction)

    assert Enum.count(all_internal_transactions) == 15

    non_zero_value_calls =
      Enum.filter(all_internal_transactions, &(&1.type == :call and not Decimal.eq?(&1.value.value, 0)))

    non_calls = Enum.filter(all_internal_transactions, &(&1.type != :call))

    recent_zero_value_calls =
      Enum.filter(all_internal_transactions, &(&1.type == :call and Decimal.eq?(&1.value.value, 0)))

    assert Enum.count(non_zero_value_calls) == 4
    assert Enum.all?(non_zero_value_calls, &(not Decimal.eq?(&1.value.value, 0)))

    assert Enum.count(non_calls) == 5

    assert Enum.count(recent_zero_value_calls) == 6

    id_to_hashes = Repo.all(AddressIdToAddressHash)

    assert length(id_to_hashes) == 3

    assert %{address_id: address_1_id} = Enum.find(id_to_hashes, &(&1.address_hash == address_1.hash))
    assert %{address_id: address_2_id} = Enum.find(id_to_hashes, &(&1.address_hash == address_2.hash))
    assert %{address_id: address_3_id} = Enum.find(id_to_hashes, &(&1.address_hash == address_3.hash))

    placeholders = Repo.all(InternalTransactionsAddressPlaceholder)

    assert length(placeholders) == 3

    assert Enum.any?(placeholders, fn p ->
             p.address_id == address_1_id and p.block_number == block.number and p.count_tos == 5 and p.count_froms == 3
           end)

    assert Enum.any?(placeholders, fn p ->
             p.address_id == address_2_id and p.block_number == block.number and p.count_tos == 3 and p.count_froms == 4
           end)

    assert Enum.any?(placeholders, fn p ->
             p.address_id == address_3_id and p.block_number == block.number and p.count_tos == 4 and p.count_froms == 5
           end)
  end

  test "Deletes every eligible block across batches with equal timestamps" do
    timestamp = Timex.shift(Timex.now(), days: -40)
    from_address = insert(:address)
    to_address = insert(:address)

    Enum.each([3, 0, 2, 1], fn number ->
      block = insert(:block, number: number, timestamp: timestamp)
      transaction = :transaction |> insert() |> with_block(block)

      insert(:internal_transaction,
        index: 10,
        transaction: transaction,
        block_hash: block.hash,
        block_number: number,
        transaction_index: transaction.index,
        from_address: from_address,
        to_address: to_address,
        type: :call,
        value: 0
      )
    end)

    assert DeleteZeroValueInternalTransactions.border_number() == 3

    start_supervised!(DeleteZeroValueInternalTransactions)
    wait_for_migration(0)
    stop_supervised!(DeleteZeroValueInternalTransactions)

    assert Repo.aggregate(InternalTransaction, :count) == 0

    placeholders = Repo.all(InternalTransactionsAddressPlaceholder)
    assert length(placeholders) == 8

    for number <- 0..3 do
      assert Enum.count(placeholders, &(&1.block_number == number and &1.count_froms == 1 and &1.count_tos == 0)) == 1
      assert Enum.count(placeholders, &(&1.block_number == number and &1.count_froms == 0 and &1.count_tos == 1)) == 1
    end
  end

  describe "ShrinkInternalTransactions migration dependency handling" do
    test "Waits for ShrinkInternalTransactions migration to complete before starting" do
      address_1 = insert(:address)
      address_2 = insert(:address)

      block = insert(:block, number: 1, timestamp: Timex.shift(Timex.now(), days: -40))

      transaction =
        :transaction
        |> insert()
        |> with_block(block)

      insert(:internal_transaction,
        index: 10,
        transaction: transaction,
        block_hash: transaction.block_hash,
        block_number: transaction.block_number,
        transaction_index: transaction.index,
        from_address: address_1,
        to_address: address_2,
        type: :call,
        value: 0
      )

      assert MigrationStatus.get_status("delete_zero_value_internal_transactions") == nil
      assert MigrationStatus.get_status("shrink_internal_transactions") == nil

      # Configure ShrinkInternalTransactions as enabled
      Application.put_env(:explorer, Explorer.Migrator.ShrinkInternalTransactions, enabled: true)

      # Start the migration without ShrinkInternalTransactions being completed
      migrator = start_supervised!(DeleteZeroValueInternalTransactions)
      assert :sys.get_state(migrator) == %{}

      # Migration should not be initialized yet
      migration_status = MigrationStatus.fetch("delete_zero_value_internal_transactions")
      assert is_nil(migration_status)

      # Internal transactions should still exist (not deleted)
      all_internal_transactions = Repo.all(InternalTransaction)
      assert Enum.count(all_internal_transactions) == 1

      # Now mark ShrinkInternalTransactions as completed
      MigrationStatus.set_status("shrink_internal_transactions", "completed")

      # Wait for DeleteZeroValueInternalTransactions to detect completion and start
      wait_for_migration(0)
      stop_supervised!(DeleteZeroValueInternalTransactions)

      # Now the internal transaction should be deleted
      remaining_internal_transactions = Repo.all(InternalTransaction)
      assert Enum.count(remaining_internal_transactions) == 0
    end

    test "Starts immediately when ShrinkInternalTransactions is disabled/not configured" do
      address_1 = insert(:address)
      address_2 = insert(:address)

      block = insert(:block, number: 1, timestamp: Timex.shift(Timex.now(), days: -40))

      transaction =
        :transaction
        |> insert()
        |> with_block(block)

      insert(:internal_transaction,
        index: 10,
        transaction: transaction,
        block_hash: transaction.block_hash,
        block_number: transaction.block_number,
        transaction_index: transaction.index,
        from_address: address_1,
        to_address: address_2,
        type: :call,
        value: 0
      )

      assert MigrationStatus.get_status("delete_zero_value_internal_transactions") == nil
      assert MigrationStatus.get_status("shrink_internal_transactions") == nil

      # Configure ShrinkInternalTransactions as disabled
      Application.put_env(:explorer, Explorer.Migrator.ShrinkInternalTransactions, enabled: false)

      # Start the migration
      start_supervised!(DeleteZeroValueInternalTransactions)

      # Wait for DeleteZeroValueInternalTransactions to start and complete
      wait_for_migration(0)
      stop_supervised!(DeleteZeroValueInternalTransactions)

      # Internal transaction should be deleted
      remaining_internal_transactions = Repo.all(InternalTransaction)
      assert Enum.count(remaining_internal_transactions) == 0
    end
  end

  defp wait_for_migration(expected_count) do
    wait_for_migration(expected_count, System.monotonic_time(:millisecond) + 5_000)
  end

  defp wait_for_migration(expected_count, deadline) do
    status = MigrationStatus.get_status("delete_zero_value_internal_transactions")
    count = Repo.aggregate(InternalTransaction, :count)

    cond do
      status == "completed" and count == expected_count ->
        :ok

      System.monotonic_time(:millisecond) < deadline ->
        Process.sleep(10)
        wait_for_migration(expected_count, deadline)

      true ->
        assert status == "completed"
        assert count == expected_count
    end
  end
end
