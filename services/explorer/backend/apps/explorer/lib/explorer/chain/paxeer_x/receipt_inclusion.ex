defmodule Explorer.Chain.PaxeerX.ReceiptInclusion do
  @moduledoc """
  Binds a signed sequencer receipt to transaction zero of a trusted Comet block.

  The `SequencerReceipt` application configuration must pin `:chain_id` and
  `:trusted_validator_hashes` (a map from landing height to the 32-byte validator
  set hash). RPC answers cannot establish either trust anchor. `:batch` is the
  complete `px_getSequenceBatch` result, including its transaction and proof;
  `:batch_activity_ids` contains the ordered 32-byte activity IDs of the block.
  Their concatenation is authenticated by the batch's txRoot before use.
  """

  import Bitwise

  alias Explorer.Chain.PaxeerX.{CometCommit, SequencerReceipt}

  @batch_domain "PAXEER/v1/sequence-batch\0"
  @sequence_domain "PAXEER/v1/sequence\0"
  @max_activities 100_000

  @spec verify(map(), map(), pos_integer(), keyword()) :: :ok | {:error, :r7 | :r8}
  def verify(receipt, key, height, opts) do
    config = Application.get_env(:explorer, SequencerReceipt, [])

    with {:ok, header, chain_id} <- trusted_header(config, opts, height),
         {:ok, ^height} <- CometCommit.verify(chain_id, opts[:commit], opts[:validators]) do
      if included?(receipt, key, height, chain_id, header, opts), do: :ok, else: {:error, :r7}
    else
      _ -> {:error, :r8}
    end
  end

  defp trusted_header(config, opts, height) do
    signed_header =
      case opts[:commit] do
        %{"signed_header" => signed_header} -> signed_header
        signed_header -> signed_header
      end

    with chain_id when is_binary(chain_id) and chain_id != "" <- config[:chain_id],
         true <- opts[:chain_id] == chain_id,
         pins when is_map(pins) <- config[:trusted_validator_hashes],
         <<_::256>> = trusted_hash <- Map.get(pins, height),
         %{"header" => %{"chain_id" => ^chain_id, "validators_hash" => validator_hash} = header} <- signed_header,
         {:ok, ^trusted_hash} <- consensus_hash(validator_hash) do
      {:ok, header, chain_id}
    else
      _ -> :error
    end
  end

  defp included?(receipt, key, height, chain_id, header, opts) do
    with %{"height" => batch_height, "tx" => tx_hex, "proof" => proof} <- opts[:batch],
         true <- batch_height == Integer.to_string(height),
         {:ok, tx} <- hex(tx_hex),
         <<@batch_domain, ^height::64, from_seq::64, to_seq::64, previous::binary-size(32), last::binary-size(32),
           tx_root::binary-size(32), key_id::binary-size(32), signature::binary-size(64)>> <- tx,
         true <- from_seq > 0 and from_seq <= receipt.global_sequence and receipt.global_sequence <= to_seq,
         true <- key_id == key.key_id and key_id == receipt.context_hash,
         true <- key.first_height <= height and height <= key.last_height,
         true <- batch_signature?(tx, chain_id, signature, key.public_key),
         {:ok, ids} <- activity_ids(opts[:batch_activity_ids]),
         true <- length(ids) == to_seq - from_seq + 1,
         true <- index_matches?(receipt, from_seq),
         true <- :crypto.hash(:sha256, IO.iodata_to_binary(ids)) == tx_root,
         true <- sequence_matches?(ids, from_seq, previous, last, receipt),
         {:ok, <<_::256>> = data_hash} <- consensus_hash(header["data_hash"]),
         true <- proof_matches?(tx, proof, length(ids) + 1, data_hash),
         true <- block_matches?(opts[:eth_receipt], opts[:commit]) do
      true
    else
      _ -> false
    end
  end

  defp index_matches?(%{effects: [%{body: <<_gas::binary-size(24), index::32, _::binary>>} | _]} = receipt, from_seq),
    do: index == receipt.global_sequence - from_seq + 1

  defp index_matches?(_receipt, _from_seq), do: false

  defp batch_signature?(tx, chain_id, signature, public_key) do
    <<@batch_domain, fields::binary-size(120), _::binary>> = tx
    digest = :crypto.hash(:sha256, @batch_domain <> chain_id <> fields)
    :crypto.verify(:eddsa, :none, digest, signature, [public_key, :ed25519])
  end

  defp activity_ids(ids) when is_list(ids) and ids != [] and length(ids) <= @max_activities do
    Enum.reduce_while(ids, {:ok, []}, fn
      <<_::256>> = id, {:ok, acc} -> {:cont, {:ok, [id | acc]}}
      _, _ -> {:halt, :error}
    end)
    |> case do
      {:ok, reversed} -> {:ok, Enum.reverse(reversed)}
      :error -> :error
    end
  end

  defp activity_ids(_ids), do: :error

  defp sequence_matches?(ids, from_seq, previous, last, receipt) do
    {root, matched} =
      ids
      |> Enum.with_index(from_seq)
      |> Enum.reduce({previous, false}, fn {id, sequence}, {root, matched} ->
        next = :crypto.hash(:sha256, @sequence_domain <> root <> <<sequence::64>> <> id)

        matches =
          sequence == receipt.global_sequence and id == receipt.activity_id and
            root == receipt.previous_state_root and next == receipt.resulting_state_root

        {next, matched or matches}
      end)

    root == last and matched
  end

  defp proof_matches?(tx, %{"total" => total, "index" => 0, "leafHash" => leaf_hex, "aunts" => aunts}, total, root)
       when is_integer(total) and total > 1 and is_list(aunts) and length(aunts) <= 17 do
    leaf = :crypto.hash(:sha256, <<0>> <> :crypto.hash(:sha256, tx))

    with {:ok, ^leaf} <- hex(leaf_hex),
         {:ok, ^root} <- proof_root(leaf, total, Enum.reverse(aunts)) do
      true
    else
      _ -> false
    end
  end

  defp proof_matches?(_tx, _proof, _total, _root), do: false

  defp proof_root(leaf, 1, []), do: {:ok, leaf}

  defp proof_root(leaf, total, [aunt | rest]) when total > 1 do
    split = split_point(total, 1)

    with {:ok, <<_::256>> = right} <- hex(aunt),
         {:ok, left} <- proof_root(leaf, split, rest) do
      {:ok, :crypto.hash(:sha256, <<1>> <> left <> right)}
    end
  end

  defp proof_root(_leaf, _total, _aunts), do: :error

  defp split_point(total, split) when split <<< 1 < total, do: split_point(total, split <<< 1)
  defp split_point(_total, split), do: split

  defp block_matches?(%{"blockHash" => block_hash}, %{"signed_header" => signed_header}),
    do: block_matches?(%{"blockHash" => block_hash}, signed_header)

  defp block_matches?(%{"blockHash" => block_hash}, %{"commit" => %{"block_id" => %{"hash" => hash}}}) do
    with {:ok, <<_::256>> = block_hash} <- hex(block_hash),
         {:ok, ^block_hash} <- consensus_hash(hash) do
      true
    else
      _ -> false
    end
  end

  defp block_matches?(_receipt, _commit), do: false

  defp consensus_hash(value) when is_binary(value), do: Base.decode16(value, case: :mixed)
  defp consensus_hash(_value), do: :error

  defp hex("0x" <> digits), do: Base.decode16(digits, case: :mixed)
  defp hex(_value), do: :error
end
