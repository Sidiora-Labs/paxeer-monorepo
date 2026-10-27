defmodule BlockScoutWeb.API.V2.PaxeerX.TransactionSignatureController do
  @moduledoc false

  use BlockScoutWeb, :controller
  use OpenApiSpex.ControllerSpecs

  alias BlockScoutWeb.API.V2.PaxeerX.TransactionSignatureView
  alias BlockScoutWeb.Schemas.API.V2.ErrorResponses.NotFoundResponse
  alias Explorer.Chain
  alias Explorer.Chain.PaxeerX.SequencerReceipt
  alias Explorer.HttpClient

  action_fallback(BlockScoutWeb.API.V2.FallbackController)

  @router_timeout 10_000

  tags(["paxeer-x"])

  operation :signature,
    summary: "Retrieve the verified sequencer signature of a transaction",
    parameters: [transaction_hash_param() | base_params()],
    responses: [
      ok: {"The verified sequencer receipt and signature.", "application/json", Schemas.PaxeerX.TransactionSignature},
      not_found: NotFoundResponse.response(),
      unprocessable_entity: {"Invalid parameter(s).", "application/json", message_response_schema()}
    ]

  def signature(conn, %{"transaction_hash_param" => transaction_hash_string}) do
    with {:format, {:ok, transaction_hash}} <- {:format, Chain.string_to_full_hash(transaction_hash_string)},
         {:ok, signature} <- verified_signature(transaction_hash) do
      response = TransactionSignatureView.render("signature.json", signature)

      response =
        if signature.rung == :committed,
          do: Map.put(response, "committed_height", signature.committed_height),
          else: response

      conn
      |> put_status(200)
      |> json(response)
    end
  end

  defp verified_signature(transaction_hash) do
    router_url = Application.get_env(:explorer, SequencerReceipt, [])[:router_url]

    with url when is_binary(url) <- router_url,
         {:ok, answer} <- sequencer_receipt(url, transaction_hash),
         {:ok, receipt_keys} <- SequencerReceipt.receipt_keys(url),
         verification <- sequencer_verification(url, transaction_hash, answer, receipt_keys),
         {:ok, rung, key} <- verified_key(verification),
         "0x" <> encoded <- answer["receipt"],
         {:ok, bytes} <- Base.decode16(encoded, case: :mixed),
         {:ok, receipt} <- SequencerReceipt.decode(bytes) do
      {:ok,
       %{
         rung: rung,
         committed_height: committed_height(verification),
         receipt: bytes,
         receipt_key: key,
         signature: receipt.signature
       }}
    else
      _unverified -> {:error, :not_found}
    end
  end

  defp verified_key({:receipted, key}), do: {:ok, :receipted, key}
  defp verified_key({:committed, key, _height}), do: {:ok, :committed, key}
  defp verified_key(_unverified), do: :error

  defp committed_height({:committed, _key, height}), do: height
  defp committed_height({:receipted, _key}), do: nil

  defp sequencer_verification(router_url, transaction_hash, answer, receipt_keys) do
    case committed_options(router_url, transaction_hash) do
      {:ok, opts} -> SequencerReceipt.verify(transaction_hash, answer, receipt_keys, opts)
      :disabled -> SequencerReceipt.verify(transaction_hash, answer, receipt_keys)
    end
  end

  defp committed_options(router_url, transaction_hash) do
    with {:ok, %{"blockNumber" => block_number} = eth_receipt} <-
           router_call(router_url, "eth_getTransactionReceipt", [to_string(transaction_hash)]),
         {:ok, height} <- quantity(block_number),
         true <- height > 0,
         {:ok, commit} <- SequencerReceipt.commit(height, router_url),
         {:ok, validators} <- SequencerReceipt.validators(height, router_url),
         {:ok, %{"from_seq" => from_seq, "to_seq" => to_seq}} <-
           router_call(router_url, "px_getSequenceBatch", [height]),
         true <- is_integer(from_seq) and is_integer(to_seq),
         %{"header" => %{"chain_id" => chain_id}} <- Map.get(commit, "signed_header", commit),
         true <- is_binary(chain_id) and chain_id != "" do
      {:ok,
       [
         eth_receipt: eth_receipt,
         batch: %{from_seq: from_seq, to_seq: to_seq},
         chain_id: chain_id,
         commit: commit,
         validators: validators
       ]}
    else
      _unavailable -> :disabled
    end
  end

  defp quantity("0x" <> digits) when digits != "" do
    case Integer.parse(digits, 16) do
      {height, ""} when height >= 0 -> {:ok, height}
      _invalid -> :error
    end
  end

  defp quantity(_value), do: :error

  defp sequencer_receipt(router_url, transaction_hash) do
    router_call(router_url, "pax_sequencerReceipt", [to_string(transaction_hash)])
  end

  defp router_call(router_url, method, params) do
    body = Jason.encode!(%{jsonrpc: "2.0", id: 1, method: method, params: params})

    case HttpClient.post(router_url, body, [{"Content-Type", "application/json"}],
           recv_timeout: @router_timeout,
           timeout: @router_timeout
         ) do
      {:ok, %{status_code: 200, body: response}} -> rpc_result(response)
      _unavailable -> {:error, :not_found}
    end
  end

  defp rpc_result(response) do
    case Jason.decode(response) do
      {:ok, %{"result" => %{} = answer}} -> {:ok, answer}
      _unavailable -> {:error, :not_found}
    end
  end
end
