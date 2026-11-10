Code.require_file("../../../support/fixture/paxeer_x/governed_receipt_fixtures.exs", __DIR__)

defmodule Explorer.Chain.PaxeerX.StatusCommittedTest do
  use ExUnit.Case, async: false

  alias Explorer.Chain.PaxeerX.Finality.Heights
  alias Explorer.Chain.PaxeerX.SequencerReceipt
  alias Explorer.Chain.PaxeerX.GovernedReceiptFixtures
  alias Explorer.Chain.PaxeerX.Status

  setup do
    GovernedReceiptFixtures.restore_on_exit()
    GovernedReceiptFixtures.configure_committed()
    :ok
  end

  @vectors Path.expand("../../../support/fixture/paxeer_x/committed_vectors.json", __DIR__)

  setup do
    %{"cases" => cases} = @vectors |> File.read!() |> Jason.decode!()
    test_case = Enum.find(cases, &(&1["id"] == "committed-ok"))
    "0x" <> raw_transaction = test_case["rawTransaction"]
    tx_hash = raw_transaction |> Base.decode16!(case: :mixed) |> ExKeccak.hash_256()
    {:ok, [key]} = SequencerReceipt.decode_receipt_keys([test_case["pinnedKey"]])

    {:ok, test_case: test_case, tx_hash: tx_hash, key: key}
  end

  test "committed_verification_promotes_unindexed_hash", context do
    committed_verification_promotes_unindexed_hash(context)
  end

  test "receipted_verification_stays_receipted", context do
    receipted_verification_stays_receipted(context)
  end

  defp committed_verification_promotes_unindexed_hash(%{test_case: test_case, tx_hash: tx_hash, key: key}) do
    verification =
      SequencerReceipt.verify(
        tx_hash,
        test_case["receiptJson"],
        [key],
        GovernedReceiptFixtures.committed_opts(test_case)
      )

    assert verification == {:committed, key, 10}

    for {head, expected_height} <- [{nil, 10}, {0, 10}, {9, 10}, {10, 10}, {11, 11}] do
      heights = %Heights{committed_height: head}

      assert Status.of(nil, heights, verification) == %Status{
               rung: :committed,
               reason: :sequencer_receipt,
               block_number: nil,
               committed_height: expected_height
             }

      assert Status.rung(nil, heights, verification) == :committed
    end

    assert Status.of(10, %Heights{committed_height: 9}, verification) == %Status{
             rung: :committed,
             reason: :sequencer_receipt,
             block_number: 10,
             committed_height: 10
           }

    assert Status.of(9, %Heights{committed_height: 9}, verification) == %Status{
             rung: :committed,
             reason: :in_committed_block,
             block_number: 9,
             committed_height: 10
           }
  end

  defp receipted_verification_stays_receipted(%{test_case: test_case, tx_hash: tx_hash, key: key}) do
    verification =
      SequencerReceipt.verify(tx_hash, test_case["receiptJson"], [key], prev_seq_hash: test_case["prevSeqHash"])

    assert verification == {:receipted, key}

    for head <- [nil, 0, 9, 10, 11] do
      heights = %Heights{committed_height: head}

      assert Status.of(nil, heights, verification) == %Status{
               rung: :receipted,
               reason: :sequencer_receipt,
               block_number: nil,
               committed_height: head
             }

      assert Status.rung(nil, heights, verification) == :receipted
    end
  end
end
