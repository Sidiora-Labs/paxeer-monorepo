defmodule Explorer.Chain.PaxeerX.ReceiptInclusion do
  @moduledoc """
  Binds a signed receipt to its batch, native transaction and deterministic execution result.

  `SequencerReceipt` configuration supplies the independent chain identity and validator
  checkpoints. Contiguous signed `validator_history` authenticates transitions from a
  checkpoint; the linked successor header commits execution results at H+1. Complete
  gateway batch bytes/proofs and native settlement proofs are required for promotion.
  """

  import Bitwise

  alias Explorer.Chain.PaxeerX.{CometCommit, NativeSettlement, SequencerReceipt, SettlementProof}

  @batch_domain "PAXEER/v1/sequence-batch\0"
  @sequence_domain "PAXEER/v1/sequence\0"
  @max_activities 100_000

  @spec verify(map(), map(), pos_integer(), keyword()) :: :ok | {:error, :r7 | :r8}
  def verify(receipt, key, height, opts) do
    config = Application.get_env(:explorer, SequencerReceipt, [])

    with {:ok, header, chain_id} <- trusted_header(config, opts, height),
         {:ok, ^height} <- CometCommit.verify(chain_id, opts[:commit], opts[:validators]),
         {:ok, settlement, successor} <- linked_settlement(config, opts, header, chain_id, height) do
      if included?(receipt, key, height, chain_id, header, opts) and
           settlement_matches?(settlement, successor, header, receipt, opts),
         do: :ok,
         else: {:error, :r7}
    else
      _ -> {:error, :r8}
    end
  end

  def authenticate(commit, validators, height) do
    config = Application.get_env(:explorer, SequencerReceipt, [])
    opts = [commit: commit, validators: validators, chain_id: config[:chain_id]]

    with {:ok, _header, chain_id} <- trusted_header(config, opts, height),
         {:ok, ^height} <- CometCommit.verify(chain_id, commit, validators),
         do: :ok
  end

  defp trusted_header(config, opts, height) do
    signed = Map.get(opts[:commit] || %{}, "signed_header", opts[:commit])

    with chain_id when is_binary(chain_id) and chain_id != "" <- config[:chain_id],
         true <- opts[:chain_id] == chain_id,
         checkpoints when is_map(checkpoints) <- config[:validator_checkpoints],
         checkpoint_height when is_integer(checkpoint_height) <-
           checkpoints |> Map.keys() |> Enum.filter(&(is_integer(&1) and &1 <= height)) |> Enum.max(fn -> nil end),
         %{block_hash: <<_::256>> = block_hash, validators_hash: <<_::256>> = validator_hash} <-
           checkpoints[checkpoint_height],
         history when is_list(history) <- Keyword.get(config, :validator_history, []),
         {:ok, header} <-
           authenticate_history(
             chain_id,
             checkpoint_height,
             height,
             block_hash,
             validator_hash,
             history,
             signed,
             opts[:validators]
           ) do
      {:ok, header, chain_id}
    else
      _ -> :error
    end
  end

  defp authenticate_history(chain_id, start, height, block_hash, validator_hash, history, signed, validators)
       when height - start <= 10_000 do
    Enum.reduce_while(start..height, {:ok, nil}, fn current, {:ok, previous} ->
      entry =
        if current == height,
          do: %{"signed_header" => signed, "validators" => validators},
          else:
            Enum.find(history, fn
              %{"signed_header" => %{"header" => %{"height" => h}}} -> h == Integer.to_string(current)
              _ -> false
            end)

      case authenticate_entry(entry, chain_id, current, previous, block_hash, validator_hash) do
        {:ok, authenticated} -> {:cont, {:ok, authenticated}}
        _ -> {:halt, :error}
      end
    end)
    |> case do
      {:ok, %{"header" => header}} -> {:ok, header}
      _ -> :error
    end
  end

  defp authenticate_history(_, _, _, _, _, _, _, _), do: :error

  defp authenticate_entry(
         %{"signed_header" => %{"header" => header, "commit" => commit} = signed, "validators" => validators},
         chain_id,
         height,
         previous,
         block_hash,
         validator_hash
       ) do
    with {:ok, ^height} <- CometCommit.verify(chain_id, signed, validators),
         true <- trusted_link?(header, commit, previous, block_hash, validator_hash) do
      {:ok, signed}
    else
      _ -> :error
    end
  end

  defp authenticate_entry(_, _, _, _, _, _), do: :error

  defp trusted_link?(header, commit, nil, block_hash, validator_hash) do
    consensus_hash(header["validators_hash"]) == {:ok, validator_hash} and
      consensus_hash(commit["block_id"]["hash"]) == {:ok, block_hash}
  end

  defp trusted_link?(header, _commit, previous, _block_hash, _validator_hash) do
    header["last_block_id"] == previous["commit"]["block_id"] and
      consensus_hash(header["validators_hash"]) == consensus_hash(previous["header"]["next_validators_hash"])
  end

  defp linked_settlement(_config, opts, execution, chain_id, height) do
    with %{
           "execution_header" => ^execution,
           "execution_commit" => execution_commit,
           "header" => successor,
           "commit" => successor_commit,
           "validators" => validators
         } = settlement <- opts[:settlement],
         signed <- Map.get(opts[:commit], "signed_header", opts[:commit]),
         true <- execution_commit == signed["commit"],
         %{"block_id" => block_id} <- execution_commit,
         true <- successor["last_block_id"] == block_id,
         true <- successor["height"] == Integer.to_string(height + 1),
         true <- successor["validators_hash"] == execution["next_validators_hash"],
         {:ok, successor_height} <-
           CometCommit.verify(
             chain_id,
             %{"header" => successor, "commit" => successor_commit},
             %{
               "block_height" => Integer.to_string(height + 1),
               "validators" => validators,
               "count" => Integer.to_string(length(validators)),
               "total" => Integer.to_string(length(validators))
             }
           ),
         true <- successor_height == height + 1 do
      {:ok, settlement, successor}
    else
      _ -> :error
    end
  end

  defp settlement_matches?(s, successor, execution, receipt, opts) do
    with %{"root_hash" => batch_root, "data" => batch_data, "proof" => batch_proof} <- s["sequence_batch_proof"],
         %{"root_hash" => tx_root, "data" => tx_data, "proof" => tx_proof} <- s["transaction_proof"],
         {:ok, data_hash} <- consensus_hash(execution["data_hash"]),
         {:ok, ^data_hash} <- consensus_hash(batch_root),
         {:ok, ^data_hash} <- consensus_hash(tx_root),
         {:ok, batch_bytes} <- base64(batch_data),
         {:ok, ^batch_bytes} <- hex(opts[:batch]["tx"]),
         true <- batch_fields_match?(batch_bytes, s["sequence_batch"]),
         true <- SettlementProof.transaction(batch_bytes, batch_proof, data_hash),
         true <- batch_proof["index"] == "0",
         {:ok, transaction} <- base64(tx_data),
         {:ok, raw, id} <- NativeSettlement.transaction(transaction),
         true <- id == receipt.activity_id,
         {:ok, ^raw} <- hex(s["raw_transaction"]),
         true <- SettlementProof.transaction(transaction, tx_proof, data_hash),
         true <- tx_proof["index"] == Integer.to_string(receipt_index(receipt)),
         true <- tx_proof["total"] == batch_proof["total"],
         {:ok, result_bytes} <- hex(s["exec_tx_result_bytes"]),
         {:ok, ^result_bytes} <- NativeSettlement.result_bytes(s["exec_tx_result"]),
         {:ok, results_root} <- consensus_hash(successor["last_results_hash"]),
         %{"index" => result_index, "total" => result_total} = result_proof <- s["exec_tx_result_proof"],
         true <- result_index == tx_proof["index"] and result_total == tx_proof["total"],
         true <- SettlementProof.verify(result_bytes, result_proof, results_root),
         true <- NativeSettlement.result_matches?(result_bytes, receipt, opts[:receipt_view]) do
      true
    else
      _ -> false
    end
  end

  defp receipt_index(%{effects: [%{body: <<_::binary-size(24), index::32, _::binary>>} | _]}), do: index

  defp batch_fields_match?(
         <<@batch_domain, height::64, first::64, last::64, previous::binary-size(32), resulting::binary-size(32),
           root::binary-size(32), key::binary-size(32), signature::binary-size(64)>>,
         fields
       )
       when is_map(fields) do
    fields == %{
      "Height" => height,
      "FromSeq" => first,
      "ToSeq" => last,
      "PrevSeqHash" => Base.encode64(previous),
      "LastSeqHash" => Base.encode64(resulting),
      "TxRoot" => Base.encode64(root),
      "KeyId" => Base.encode64(key),
      "Sig" => Base.encode64(signature)
    }
  end

  defp batch_fields_match?(_, _), do: false
  defp base64(text) when is_binary(text), do: Base.decode64(text)
  defp base64(_), do: :error

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
