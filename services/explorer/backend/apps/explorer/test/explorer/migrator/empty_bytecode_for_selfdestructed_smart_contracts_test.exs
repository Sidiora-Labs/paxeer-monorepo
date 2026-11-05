Code.require_file("selfdestruct_chain_data.exs", __DIR__)

defmodule Explorer.Migrator.EmptyBytecodeForSelfdestructedSmartContractsTest do
  use Explorer.DataCase, async: false

  import Explorer.Migrator.SelfdestructChainData

  alias Explorer.Chain.{Address, Data}
  alias Explorer.Migrator.EmptyBytecodeForSelfdestructedSmartContracts
  alias Explorer.Repo

  describe "migration_name/0" do
    test "returns the correct migration name" do
      assert EmptyBytecodeForSelfdestructedSmartContracts.migration_name() ==
               "empty_bytecode_for_selfdestructed_smart_contracts"
    end
  end

  describe "update_batch/1" do
    test "returns {:ok, []} when block_numbers is empty" do
      assert {:ok, []} = EmptyBytecodeForSelfdestructedSmartContracts.update_batch([])
    end

    test "returns {:ok, []} when no selfdestruct transactions exist in blocks" do
      configure_cancun_block(:never)
      block = insert(:block, number: 100)
      transaction = insert(:transaction) |> with_block(block)

      insert(:internal_transaction,
        transaction: transaction,
        block_hash: transaction.block_hash,
        block_number: transaction.block_number,
        index: 0,
        transaction_index: transaction.index,
        type: :call
      )

      assert {:ok, []} = EmptyBytecodeForSelfdestructedSmartContracts.update_batch([block.number])
    end

    test "empties historically deleted pre-Cancun code confirmed at a committed height" do
      recording = import_recording("shanghai_existing")
      assert recording["before"]["response"]["result"] == recording["cached_code"]
      assert recording["at_destruction"]["response"]["result"] == "0x"

      assert {:ok, 1} =
               EmptyBytecodeForSelfdestructedSmartContracts.update_batch([quantity(recording["destruction_block"])])

      assert_committed_code(recording)
    end

    test "preserves a live post-Cancun contract despite its SELFDESTRUCT trace" do
      recording = import_recording("cancun_existing")
      assert recording["at_destruction"]["response"]["result"] == recording["cached_code"]
      assert_committed_code(recording)
    end

    test "empties deleted code when CREATE and SELFDESTRUCT occur in the same transaction" do
      recording = import_recording("cancun_same_transaction_create")
      assert recording["current"]["response"]["result"] == "0x"
      assert_committed_code(recording)
    end

    test "empties deleted code when CREATE2 and SELFDESTRUCT occur in the same transaction" do
      recording = import_recording("cancun_same_transaction_create2")
      assert recording["current"]["response"]["result"] == "0x"
      assert_committed_code(recording)
    end

    test "handles multiple selfdestruct transactions in same block" do
      history = import_recording("geth_history")
      first = contract_recording(history, "first")
      second = contract_recording(history, "second")
      assert first["destruction_block"] == second["destruction_block"]
      contract_code_1 = contract_data(first["cached_code"])
      contract_code_2 = contract_data(second["cached_code"])
      contract_address_1 = Repo.get!(Address, first["stored_address_hash"])
      contract_address_2 = Repo.get!(Address, second["stored_address_hash"])
      assert contract_address_1.contract_code == contract_code_1
      assert contract_address_2.contract_code == contract_code_2
      assert contract_address_1.hash != contract_address_2.hash

      assert {:ok, 2} =
               EmptyBytecodeForSelfdestructedSmartContracts.update_batch([quantity(first["destruction_block"])])

      updated_address_1 = Repo.get(Address, contract_address_1.hash)
      updated_address_2 = Repo.get(Address, contract_address_2.hash)
      assert updated_address_1.contract_code == %Data{bytes: <<>>}
      assert updated_address_2.contract_code == %Data{bytes: <<>>}
    end

    test "does NOT update address that already has empty contract_code" do
      configure_cancun_block(:never)
      block = insert(:block, number: 500)
      empty_contract_code = %Data{bytes: <<>>}

      # Create a contract with already empty bytecode
      contract_address = insert(:address, contract_code: empty_contract_code)
      recipient_address = insert(:address)

      # Create a transaction with selfdestruct
      transaction = insert(:transaction) |> with_block(block)

      insert(:internal_transaction,
        transaction: transaction,
        block_hash: transaction.block_hash,
        block_number: transaction.block_number,
        index: 0,
        transaction_index: transaction.index,
        type: :selfdestruct,
        from_address: contract_address,
        to_address: recipient_address,
        gas: nil
      )

      assert {:ok, 0} = EmptyBytecodeForSelfdestructedSmartContracts.update_batch([block.number])

      # Verify contract_code remains empty (no update occurred)
      updated_address = Repo.get(Address, contract_address.hash)
      assert updated_address.contract_code == empty_contract_code
    end

    test "does NOT update address with nil contract_code" do
      configure_cancun_block(:never)
      block = insert(:block, number: 550)

      # Create a regular address (not a contract, contract_code is nil)
      address = insert(:address, contract_code: nil)
      recipient_address = insert(:address)

      # Create a transaction with selfdestruct
      transaction = insert(:transaction) |> with_block(block)

      insert(:internal_transaction,
        transaction: transaction,
        block_hash: transaction.block_hash,
        block_number: transaction.block_number,
        index: 0,
        transaction_index: transaction.index,
        type: :selfdestruct,
        from_address: address,
        to_address: recipient_address,
        gas: nil
      )

      assert {:ok, 0} = EmptyBytecodeForSelfdestructedSmartContracts.update_batch([block.number])

      # Verify contract_code remains nil
      updated_address = Repo.get(Address, address.hash)
      assert updated_address.contract_code == nil
    end

    test "handles multiple blocks in one batch" do
      history = import_recording("geth_history")
      first = contract_recording(history, "third")
      third = contract_recording(history, "repeated")
      assert first["destruction_block"] != third["destruction_block"]

      contract_address_1 = Repo.get!(Address, first["stored_address_hash"])
      contract_address_2 = Repo.get!(Address, third["stored_address_hash"])
      assert contract_address_1.contract_code == contract_data(first["cached_code"])
      assert contract_address_2.contract_code == contract_data(third["cached_code"])

      assert {:ok, 2} =
               EmptyBytecodeForSelfdestructedSmartContracts.update_batch([
                 quantity(first["destruction_block"]),
                 quantity(third["destruction_block"])
               ])

      updated_address_1 = Repo.get(Address, contract_address_1.hash)
      updated_address_2 = Repo.get(Address, contract_address_2.hash)
      assert updated_address_1.contract_code == %Data{bytes: <<>>}
      assert updated_address_2.contract_code == %Data{bytes: <<>>}
    end

    test "handles mixed fork history: deleted pre-Cancun code and live post-Cancun code" do
      history = import_recording("geth_history")
      historical = contract_recording(history, "third")
      current = contract_recording(history, "survivor")
      block_numbers = Enum.map([historical, current], &quantity(&1["destruction_block"]))

      assert {:ok, 1} = EmptyBytecodeForSelfdestructedSmartContracts.update_batch(block_numbers)
      assert Repo.get!(Address, historical["stored_address_hash"]).contract_code == contract_data("0x")
      assert Repo.get!(Address, current["stored_address_hash"]).contract_code == contract_data(current["cached_code"])
    end

    test "handles same contract selfdestructed multiple times in different transactions" do
      history = import_recording("geth_history")
      recording = contract_recording(history, "repeated")
      contract_address = Repo.get!(Address, recording["stored_address_hash"])
      assert contract_address.contract_code == contract_data(recording["cached_code"])

      destructions =
        Repo.all(
          from(it in Explorer.Chain.InternalTransaction,
            where: it.type == :selfdestruct and it.from_address_hash == ^contract_address.hash,
            select: it.transaction_hash
          )
        )

      assert length(Enum.uniq(destructions)) == 2

      assert {:ok, 1} =
               EmptyBytecodeForSelfdestructedSmartContracts.update_batch([
                 quantity(recording["first_destruction_block"]),
                 quantity(recording["destruction_block"])
               ])

      updated_address = Repo.get(Address, contract_address.hash)
      assert updated_address.contract_code == %Data{bytes: <<>>}
    end
  end

  describe "last_unprocessed_identifiers/1" do
    test "returns empty list when min_block_number is negative" do
      state = %{"min_block_number" => -1}

      assert {[], %{"min_block_number" => -1}} =
               EmptyBytecodeForSelfdestructedSmartContracts.last_unprocessed_identifiers(state)
    end

    test "returns block numbers in descending order" do
      state = %{"min_block_number" => 100}

      {block_numbers, new_state} =
        EmptyBytecodeForSelfdestructedSmartContracts.last_unprocessed_identifiers(state)

      # Should return blocks in descending order
      assert length(block_numbers) > 0
      assert block_numbers == Enum.sort(block_numbers, :desc)
      assert hd(block_numbers) == 100
      assert new_state["min_block_number"] < 100
    end

    test "handles blocks down to zero" do
      # Set a small number to test boundary
      state = %{"min_block_number" => 5}

      {block_numbers, new_state} =
        EmptyBytecodeForSelfdestructedSmartContracts.last_unprocessed_identifiers(state)

      assert length(block_numbers) > 0
      assert Enum.min(block_numbers) >= 0
      assert new_state["min_block_number"] <= 0
    end
  end

  describe "update_cache/0" do
    test "returns :ok" do
      assert :ok = EmptyBytecodeForSelfdestructedSmartContracts.update_cache()
    end
  end
end
