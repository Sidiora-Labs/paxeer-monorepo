defmodule BlockScoutWeb.API.V2.PaxeerX.TransactionStatusController do
  @moduledoc """
  Publishes the rung a transaction has reached on the Paxeer X Network settlement ladder.

  An indexed transaction is placed from the block holding it and the committed head. A hash
  the explorer has not indexed is answered from the sequencer: its `pax_sequencerReceipt`
  answer is verified (R1-R6) against the pinned receipt keys and answers `receipted`.
  Landing evidence that passes R7-R8 promotes it to `committed`; every other hash is not found.
  """

  use BlockScoutWeb, :controller
  use OpenApiSpex.ControllerSpecs

  alias BlockScoutWeb.Schemas.API.V2.ErrorResponses.NotFoundResponse
  alias Explorer.Chain
  alias Explorer.Chain.Hash
  alias Explorer.Chain.PaxeerX.{Finality, SequencerReceipt, Status}
  alias Explorer.HttpClient

  action_fallback(BlockScoutWeb.API.V2.FallbackController)

  @api_true [api?: true]
  @router_timeout 10_000

  tags(["paxeer-x"])

  operation :status,
    summary: "Retrieve the settlement status of a transaction on Paxeer X Network",
    description:
      "Retrieves the rung the transaction has reached on the settlement ladder: `committed` once a block " <>
        "the committed head covers holds it, `receipted` once its sequencer receipt verifies against the " <>
        "pinned receipt keys, `submitted` otherwise.",
    parameters: [transaction_hash_param() | base_params()],
    responses: [
      ok: {"The settlement status of the transaction.", "application/json", Schemas.PaxeerX.TransactionStatus},
      not_found: NotFoundResponse.response(),
      unprocessable_entity: {"Invalid parameter(s).", "application/json", message_response_schema()}
    ]

  @doc """
  Handles GET requests to `/api/v2/transactions/:transaction_hash_param/status`.

  Answers with the rung the transaction has reached, the block holding it and the committed
  height that rung was measured against.
  """
  @spec status(Plug.Conn.t(), map()) :: Plug.Conn.t() | {atom(), any()}
  def status(conn, %{"transaction_hash_param" => transaction_hash_string} = _params) do
    with {:format, {:ok, transaction_hash}} <- {:format, Chain.string_to_full_hash(transaction_hash_string)},
         {:ok, %Status{} = status} <- status_of(transaction_hash) do
      conn
      |> put_status(200)
      |> render(:status, %{status: status})
    end
  end

  defp status_of(%Hash{} = transaction_hash) do
    case Chain.hash_to_transaction(transaction_hash, @api_true) do
      {:ok, %{block_number: block_number, block_consensus: true}} when is_integer(block_number) ->
        {:ok, Status.of(block_number, Finality.heights(@api_true))}

      {:ok, %{block_number: block_number}} ->
        status = Status.of(nil, Finality.heights(@api_true), sequencer_verification(transaction_hash))
        {:ok, %Status{status | block_number: block_number}}

      {:error, :not_found} ->
        case sequencer_verification(transaction_hash) do
          {:receipted, _key} = verification -> {:ok, Status.of(nil, Finality.heights(@api_true), verification)}
          {:committed, _key, _height} = verification -> {:ok, Status.of(nil, Finality.heights(@api_true), verification)}
          _unverified -> {:error, :not_found}
        end
    end
  end

  defp sequencer_verification(%Hash{} = transaction_hash) do
    router_url = Application.get_env(:explorer, SequencerReceipt, [])[:router_url]

    with url when is_binary(url) <- router_url,
         {:ok, %{} = answer} <- sequencer_receipt(url, transaction_hash),
         {:ok, receipt_keys} <- SequencerReceipt.receipt_keys(url) do
      case committed_options(url, transaction_hash) do
        {:ok, opts} -> SequencerReceipt.verify(transaction_hash, answer, receipt_keys, opts)
        _unavailable -> SequencerReceipt.verify(transaction_hash, answer, receipt_keys)
      end
    else
      _unavailable -> nil
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

  defp sequencer_receipt(router_url, %Hash{} = transaction_hash) do
    router_call(router_url, "pax_sequencerReceipt", [to_string(transaction_hash)])
  end

  defp router_call(router_url, method, params) do
    body =
      Jason.encode!(%{
        jsonrpc: "2.0",
        id: 1,
        method: method,
        params: params
      })

    case HttpClient.post(router_url, body, [{"Content-Type", "application/json"}],
           recv_timeout: @router_timeout,
           timeout: @router_timeout
         ) do
      {:ok, %{status_code: 200, body: response}} -> rpc_result(response)
      {:ok, %{status_code: status}} -> {:error, {:http_status, status}}
      {:error, reason} -> {:error, reason}
    end
  end

  defp rpc_result(response) do
    case Jason.decode(response) do
      {:ok, %{"result" => %{} = result}} -> {:ok, result}
      {:ok, %{"result" => nil}} -> {:error, :not_found}
      {:ok, %{"error" => error}} -> {:error, {:rpc_error, error}}
      _ -> {:error, :malformed_response}
    end
  end
end
