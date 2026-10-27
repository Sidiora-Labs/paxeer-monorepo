defmodule BlockScoutWeb.API.V2.PaxeerX.TransactionSignatureController do
  @moduledoc false

  use BlockScoutWeb, :controller
  use OpenApiSpex.ControllerSpecs

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
      conn
      |> put_status(200)
      |> render(:signature, signature)
    end
  end

  defp verified_signature(transaction_hash) do
    router_url = Application.get_env(:explorer, SequencerReceipt, [])[:router_url]

    with url when is_binary(url) <- router_url,
         {:ok, answer} <- sequencer_receipt(url, transaction_hash),
         {:ok, receipt_keys} <- SequencerReceipt.receipt_keys(url),
         {:ok, rung, key} <- verified_key(SequencerReceipt.verify(transaction_hash, answer, receipt_keys, [])),
         "0x" <> encoded <- answer["receipt"],
         {:ok, bytes} <- Base.decode16(encoded, case: :mixed),
         {:ok, receipt} <- SequencerReceipt.decode(bytes) do
      {:ok, %{rung: rung, receipt: bytes, receipt_key: key, signature: receipt.signature}}
    else
      _unverified -> {:error, :not_found}
    end
  end

  defp verified_key({:receipted, key}), do: {:ok, :receipted, key}
  defp verified_key({:committed, key, _height}), do: {:ok, :committed, key}
  defp verified_key(_unverified), do: :error

  defp sequencer_receipt(router_url, transaction_hash) do
    body =
      Jason.encode!(%{jsonrpc: "2.0", id: 1, method: "pax_sequencerReceipt", params: [to_string(transaction_hash)]})

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
