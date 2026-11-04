defmodule Explorer.Chain.PaxeerX.ReceiptEvidence do
  @moduledoc false
  alias Explorer.Chain.PaxeerX.{NativeSettlement, ReceiptInclusion, SequencerReceipt, SettlementProof}
  alias Explorer.HttpClient

  @batch_domain "PAXEER/v1/sequence-batch\0"

  def options(router, tx_hash, answer) do
    config = Application.get_env(:explorer, SequencerReceipt, [])

    with {:ok, %{"blockNumber" => number} = eth} <- rpc(router, "eth_getTransactionReceipt", [to_string(tx_hash)]),
         {:ok, height} <- quantity(number),
         true <- height > 0,
         {:ok, commit} <- SequencerReceipt.commit(height, router),
         {:ok, validators} <- SequencerReceipt.validators(height, router),
         {:ok, batch} <- rpc(router, "px_getSequenceBatch", [Integer.to_string(height)]) do
      case ReceiptInclusion.authenticate(commit, validators, height) do
        :ok ->
          opts = [
            on_invalid_inclusion: :receipted,
            eth_receipt: eth,
            batch: batch,
            chain_id: config[:chain_id],
            commit: commit,
            validators: validators
          ]

          case settlement(config[:node_rpc_url], router, height, commit, answer) do
            {:ok, settlement, ids} -> {:ok, opts ++ [settlement: settlement, batch_activity_ids: ids]}
            _ -> :disabled
          end

        _ ->
          {:error, :r8}
      end
    else
      _ -> :disabled
    end
  end

  defp settlement(node, router, height, commit, answer) when is_binary(node) do
    decimal = Integer.to_string(height)

    with {:ok, %{"block" => %{"header" => header, "data" => %{"txs" => encoded}}}} <-
           rpc(node, "block", %{"height" => decimal}),
         signed <- Map.get(commit, "signed_header", commit),
         true <- header == signed["header"],
         true <- is_list(encoded) and length(encoded) in 2..100_001,
         {:ok, transactions} <- decode_transactions(encoded),
         [batch | native] <- transactions,
         {:ok, ids, raw_transactions} <- activities(native),
         {:ok, index} <- quantity(answer["index"]),
         true <- index > 0 and index < length(transactions),
         {:ok, %{"height" => ^decimal, "txs_results" => results}} <- rpc(node, "block_results", %{"height" => decimal}),
         true <- is_list(results) and length(results) == length(transactions),
         {:ok, result_bytes} <- results_bytes(results),
         {:ok, successor} <- SequencerReceipt.commit(height + 1, router),
         {:ok, validators} <- SequencerReceipt.validators(height + 1, router),
         successor <- Map.get(successor, "signed_header", successor),
         {:ok, fields} <- batch_fields(batch) do
      tx_hashes = Enum.map(transactions, &:crypto.hash(:sha256, &1))
      batch_proof = tx_hashes |> SettlementProof.make(0) |> Map.put("data", Base.encode64(batch))
      tx_proof = tx_hashes |> SettlementProof.make(index) |> Map.put("data", Enum.at(encoded, index))
      result_proof = SettlementProof.make(result_bytes, index)["proof"]

      {:ok,
       %{
         "raw_transaction" => hex(Enum.at(raw_transactions, index - 1)),
         "receipt" => answer,
         "sequence_batch" => fields,
         "sequence_batch_proof" => batch_proof,
         "transaction_proof" => tx_proof,
         "exec_tx_result" => Enum.at(results, index),
         "exec_tx_result_bytes" => hex(Enum.at(result_bytes, index)),
         "exec_tx_result_proof" => result_proof,
         "execution_header" => header,
         "execution_commit" => signed["commit"],
         "header" => successor["header"],
         "commit" => successor["commit"],
         "validators" => validators["validators"]
       }, ids}
    else
      _ -> :error
    end
  end

  defp settlement(_, _, _, _, _), do: :error

  defp decode_transactions(encoded) do
    collect(encoded, fn
      text when is_binary(text) and byte_size(text) <= 1_398_104 -> Base.decode64(text)
      _ -> :error
    end)
  end

  defp activities(transactions) do
    case collect(transactions, fn tx ->
           case NativeSettlement.transaction(tx) do
             {:ok, raw, hash} -> {:ok, {hash, raw}}
             _ -> :error
           end
         end) do
      {:ok, pairs} ->
        {ids, raw} = Enum.unzip(pairs)
        {:ok, ids, raw}

      _ ->
        :error
    end
  end

  defp results_bytes(results), do: collect(results, &NativeSettlement.result_bytes/1)

  defp collect(items, decode) do
    Enum.reduce_while(items, {:ok, []}, fn item, {:ok, acc} ->
      case decode.(item) do
        {:ok, value} -> {:cont, {:ok, [value | acc]}}
        _ -> {:halt, :error}
      end
    end)
    |> case do
      {:ok, values} -> {:ok, Enum.reverse(values)}
      _ -> :error
    end
  end

  defp batch_fields(
         <<@batch_domain, height::64, first::64, last::64, previous::binary-size(32), resulting::binary-size(32),
           root::binary-size(32), key::binary-size(32), signature::binary-size(64)>>
       ) do
    {:ok,
     %{
       "Height" => height,
       "FromSeq" => first,
       "ToSeq" => last,
       "PrevSeqHash" => Base.encode64(previous),
       "LastSeqHash" => Base.encode64(resulting),
       "TxRoot" => Base.encode64(root),
       "KeyId" => Base.encode64(key),
       "Sig" => Base.encode64(signature)
     }}
  end

  defp batch_fields(_), do: :error

  defp rpc(url, method, params) do
    body = Jason.encode!(%{jsonrpc: "2.0", id: 1, method: method, params: params})

    case HttpClient.post(url, body, [{"Content-Type", "application/json"}], recv_timeout: 10_000, timeout: 10_000) do
      {:ok, %{status_code: 200, body: body}} ->
        case Jason.decode(body) do
          {:ok, %{"result" => %{} = result}} -> {:ok, result}
          _ -> :error
        end

      _ ->
        :error
    end
  end

  defp quantity("0x" <> digits) do
    case Integer.parse(digits, 16) do
      {n, ""} when n >= 0 -> {:ok, n}
      _ -> :error
    end
  end

  defp quantity(_), do: :error
  defp hex(bytes), do: "0x" <> Base.encode16(bytes, case: :lower)
end
