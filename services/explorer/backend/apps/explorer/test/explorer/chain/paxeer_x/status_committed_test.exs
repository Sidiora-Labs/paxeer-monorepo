defmodule Explorer.Chain.PaxeerX.StatusCommittedTest do
  use ExUnit.Case, async: true

  alias Explorer.Chain.PaxeerX.Finality.Heights
  alias Explorer.Chain.PaxeerX.SequencerReceipt
  alias Explorer.Chain.PaxeerX.Status

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
    %{"from_seq" => from_seq, "to_seq" => to_seq} = test_case["batch"]

    verification =
      SequencerReceipt.verify(tx_hash, test_case["receiptJson"], [key],
        prev_seq_hash: test_case["prevSeqHash"],
        eth_receipt: test_case["eth_receipt"],
        batch: %{from_seq: from_seq, to_seq: to_seq},
        chain_id: test_case["chain_id"],
        commit: test_case["commit"],
        validators: test_case["validators"]
      )

    assert verification == {:committed, key, 1_000}

    for {head, expected_height} <- [{nil, 1_000}, {0, 1_000}, {999, 1_000}, {1_000, 1_000}, {1_001, 1_001}] do
      heights = %Heights{committed_height: head}

      assert Status.of(nil, heights, verification) == %Status{
               rung: :committed,
               reason: :sequencer_receipt,
               block_number: nil,
               committed_height: expected_height
             }

      assert Status.rung(nil, heights, verification) == :committed
    end

    assert Status.of(1_000, %Heights{committed_height: 999}, verification) == %Status{
             rung: :committed,
             reason: :sequencer_receipt,
             block_number: 1_000,
             committed_height: 1_000
           }

    assert Status.of(999, %Heights{committed_height: 999}, verification) == %Status{
             rung: :committed,
             reason: :in_committed_block,
             block_number: 999,
             committed_height: 1_000
           }
  end

  defp receipted_verification_stays_receipted(%{test_case: test_case, tx_hash: tx_hash, key: key}) do
    verification =
      SequencerReceipt.verify(tx_hash, test_case["receiptJson"], [key], prev_seq_hash: test_case["prevSeqHash"])

    assert verification == {:receipted, key}

    for head <- [nil, 0, 999, 1_000, 1_001] do
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
