defmodule Explorer.Chain.PaxeerX.GovernedReceiptFixtures do
  alias Explorer.Chain.PaxeerX.{ReceiptKeysConfig, SequencerReceipt}

  def restore_on_exit do
    original = Application.get_env(:explorer, SequencerReceipt)

    ExUnit.Callbacks.on_exit(fn ->
      if original == nil,
        do: Application.delete_env(:explorer, SequencerReceipt),
        else: Application.put_env(:explorer, SequencerReceipt, original)
    end)
  end

  def configure_receipted do
    corpus = read("receipt_vectors.json")
    [key] = ReceiptKeysConfig.parse([hd(corpus["sequencer"]["cases"])["pinnedKey"]])
    configure_keys([key], Integer.to_string(corpus["sequencer"]["chainId"]))
  end

  def configure_committed do
    recording = read("committed_vectors.json") |> Map.fetch!("cases") |> hd()
    [key] = ReceiptKeysConfig.parse([recording["pinnedKey"]])
    signed = recording["commit"]["signed_header"]
    height = String.to_integer(signed["header"]["height"])

    checkpoint = %{
      block_hash: Base.decode16!(signed["commit"]["block_id"]["hash"], case: :mixed),
      validators_hash: Base.decode16!(signed["header"]["validators_hash"], case: :mixed)
    }

    configure_keys([key], recording["chain_id"], %{height => checkpoint})
  end

  def configure_keys(keys, chain_id \\ "125", checkpoints \\ %{}) do
    Application.put_env(:explorer, SequencerReceipt,
      chain_id: chain_id,
      receipt_keys: keys,
      validator_checkpoints: checkpoints
    )
  end

  def committed_opts(recording) do
    [
      prev_seq_hash: recording["prevSeqHash"],
      eth_receipt: recording["eth_receipt"],
      batch: recording["batch"],
      batch_activity_ids: [transaction_hash(recording)],
      chain_id: recording["chain_id"],
      commit: recording["commit"],
      validators: recording["validators"],
      receipt_inclusion: recording["receiptInclusion"]
    ]
  end

  def transaction_hash(%{"rawTransaction" => "0x" <> raw}) do
    raw |> Base.decode16!(case: :mixed) |> ExKeccak.hash_256()
  end

  def read(name), do: __DIR__ |> Path.join(name) |> File.read!() |> Jason.decode!()
end
