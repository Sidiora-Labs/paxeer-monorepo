Code.require_file("../../../support/fixture/paxeer_x/governed_receipt_fixtures.exs", __DIR__)

defmodule Explorer.Chain.PaxeerX.CommittedVectorsTest do
  use ExUnit.Case, async: false

  alias Explorer.Chain.PaxeerX.SequencerReceipt
  alias Explorer.Chain.PaxeerX.GovernedReceiptFixtures

  setup do
    GovernedReceiptFixtures.restore_on_exit()
    GovernedReceiptFixtures.configure_committed()
    :ok
  end

  @vectors Path.expand("../../../support/fixture/paxeer_x/committed_vectors.json", __DIR__)

  test "committed_vectors_drive_verify" do
    %{"cases" => cases} = @vectors |> File.read!() |> Jason.decode!()

    assert Enum.map(cases, & &1["id"]) == [
             "committed-ok",
             "r7-height-below-receipt",
             "r8-commit-height-mismatch"
           ]

    {:ok, [key]} = SequencerReceipt.decode_receipt_keys([hd(cases)["pinnedKey"]])

    assert key.key_id == :crypto.hash(:sha256, key.public_key)
    assert {key.first_height, key.last_height} == {10, 10}

    assert Enum.map(cases, &committed_vectors_drive_verify/1) == [
             {:committed, key, 10},
             {:error, :r7},
             {:error, :r8}
           ]
  end

  defp committed_vectors_drive_verify(test_case) do
    "0x" <> raw_transaction = test_case["rawTransaction"]
    tx_hash = raw_transaction |> Base.decode16!(case: :mixed) |> ExKeccak.hash_256()
    {:ok, keys} = SequencerReceipt.decode_receipt_keys([test_case["pinnedKey"]])

    SequencerReceipt.verify(
      tx_hash,
      test_case["receiptJson"],
      keys,
      GovernedReceiptFixtures.committed_opts(test_case)
    )
  end
end
