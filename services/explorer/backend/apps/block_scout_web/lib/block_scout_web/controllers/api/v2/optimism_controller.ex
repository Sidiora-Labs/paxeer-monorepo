defmodule BlockScoutWeb.API.V2.OptimismController do
  use BlockScoutWeb, :controller
  use OpenApiSpex.ControllerSpecs

  require Logger

  import BlockScoutWeb.Chain,
    only: [
      next_page_params: 3,
      paging_options: 1,
      split_list_by_page: 1
    ]

  alias BlockScoutWeb.API.V2.{ApiView, OptimismView}
  alias BlockScoutWeb.Schemas.API.V2.ErrorResponses.NotFoundResponse
  alias Explorer.{Chain, PagingOptions}
  alias Explorer.Chain.Cache.ChainId
  alias Explorer.Chain.{Data, Hash, Token, Transaction}

  alias Explorer.Chain.Optimism.{
    Deposit,
    DisputeGame,
    FrameSequence,
    FrameSequenceBlob,
    InteropMessage,
    OutputRoot,
    TransactionBatch,
    Withdrawal
  }

  alias Indexer.Fetcher.Optimism.Interop.MessageQueue, as: InteropMessageQueue

  action_fallback(BlockScoutWeb.API.V2.FallbackController)

  plug(OpenApiSpex.Plug.CastAndValidate, json_render_error_v2: true)

  @api_true [api?: true]

  operation :batches,
    summary: "List batches.",
    description: "Retrieves a paginated list of batches.",
    parameters:
      base_params() ++
        define_paging_params([
          "id",
          "items_count"
        ]),
    responses: [
      ok:
        {"List of batches.", "application/json",
         paginated_response(
           items: Schemas.Optimism.Batch,
           next_page_params_example: %{
             "id" => 394_591,
             "items_count" => 50
           },
           title_prefix: "Batches"
         )},
      unprocessable_entity: JsonErrorResponse.response()
    ]

  @doc """
  Function to handle GET requests to `/api/v2/optimism/batches` endpoint.
  """
  @spec batches(Plug.Conn.t(), map()) :: Plug.Conn.t()
  def batches(conn, params) do
    {batches, next_page} =
      params
      |> paging_options()
      |> Keyword.put(:api?, true)
      |> Keyword.put(:only_view_ready?, true)
      |> FrameSequence.list()
      |> split_list_by_page()

    next_page_params = next_page_params(next_page, batches, params)

    items =
      batches
      |> Enum.map(fn fs ->
        Task.async(fn ->
          l2_block_number_from = TransactionBatch.edge_l2_block_number(fs.id, :min, @api_true)
          l2_block_number_to = TransactionBatch.edge_l2_block_number(fs.id, :max, @api_true)

          l2_block_range =
            if not is_nil(l2_block_number_from) and not is_nil(l2_block_number_to) do
              l2_block_number_from..l2_block_number_to
            end

          # credo:disable-for-lines:2 Credo.Check.Refactor.Nesting
          transactions_count =
            case l2_block_range do
              nil -> 0
              range -> Transaction.transaction_count_for_block_range(range)
            end

          {batch_data_container, _} = FrameSequenceBlob.list(fs.id, api?: true)

          fs
          |> Map.put(:l2_block_range, l2_block_range)
          |> Map.put(:transactions_count, transactions_count)
          |> Map.put(:batch_data_container, batch_data_container)
        end)
      end)
      |> Task.yield_many(:infinity)
      |> Enum.map(fn {_task, {:ok, item}} -> item end)
      |> Enum.reject(&is_nil(&1.l2_block_range))

    conn
    |> put_status(200)
    |> render(:optimism_batches, %{
      batches: items,
      next_page_params: next_page_params
    })
  end

  operation :batches_count,
    summary: "Number of batches in the list.",
    description: "Retrieves a size of the batch list.",
    parameters: base_params(),
    responses: [
      ok: {"Number of items in the batch list.", "application/json", %Schema{type: :integer, nullable: false}},
      unprocessable_entity: JsonErrorResponse.response()
    ]

  @doc """
  Function to handle GET requests to `/api/v2/optimism/batches/count` endpoint.
  """
  @spec batches_count(Plug.Conn.t(), map()) :: Plug.Conn.t()
  def batches_count(conn, _params) do
    items_count(conn, FrameSequence)
  end

  operation :batch_by_celestia_blob,
    summary: "Batch by celestia blob.",
    description: "Retrieves batch detailed info by the given celestia blob metadata (height and commitment).",
    parameters:
      base_params() ++
        [
          %OpenApiSpex.Parameter{
            name: :height,
            in: :path,
            schema: Schemas.General.IntegerString,
            required: true,
            description: "Celestia blob height in the path."
          },
          %OpenApiSpex.Parameter{
            name: :commitment,
            in: :path,
            schema: Schemas.General.HexString,
            required: true,
            description: "Celestia blob commitment in the path."
          }
        ],
    responses: [
      ok: {"Batch detailed info.", "application/json", Schemas.Optimism.Batch.Detailed},
      unprocessable_entity: JsonErrorResponse.response(),
      not_found: NotFoundResponse.response()
    ]

  @doc """
  Function to handle GET requests to `/api/v2/optimism/batches/da/celestia/:height/:commitment` endpoint.
  """
  @spec batch_by_celestia_blob(Plug.Conn.t(), map()) :: Plug.Conn.t()
  def batch_by_celestia_blob(conn, %{height: height, commitment: commitment}) do
    {height, ""} = Integer.parse(height)

    commitment =
      if String.starts_with?(String.downcase(commitment), "0x") do
        commitment
      else
        "0x" <> commitment
      end

    batch = FrameSequence.batch_by_celestia_blob(commitment, height, api?: true)

    if is_nil(batch) do
      {:error, :not_found}
    else
      conn
      |> put_status(200)
      |> render(:optimism_batch, %{batch: batch})
    end
  end

  operation :batch_by_number,
    summary: "Batch by its number.",
    description: "Retrieves batch detailed info by the given number.",
    parameters: [
      %OpenApiSpex.Parameter{
        name: :number,
        in: :path,
        schema: Schemas.General.IntegerString,
        required: true,
        description: "Batch number in the path."
      }
      | base_params()
    ],
    responses: [
      ok: {"Batch detailed info.", "application/json", Schemas.Optimism.Batch.Detailed},
      unprocessable_entity: JsonErrorResponse.response(),
      not_found: NotFoundResponse.response()
    ]

  @doc """
  Function to handle GET requests to `/api/v2/optimism/batches/:number` endpoint.
  """
  @spec batch_by_number(Plug.Conn.t(), map()) :: Plug.Conn.t()
  def batch_by_number(conn, %{number: number}) do
    {number, ""} = Integer.parse(number)

    batch = FrameSequence.batch_by_number(number, api?: true)

    if is_nil(batch) do
      {:error, :not_found}
    else
      conn
      |> put_status(200)
      |> render(:optimism_batch, %{batch: batch})
    end
  end

  operation :output_roots,
    summary: "List output roots.",
    description: "Retrieves a paginated list of output roots.",
    parameters:
      base_params() ++
        define_paging_params([
          "index",
          "items_count"
        ]),
    responses: [
      ok:
        {"List of output roots.", "application/json",
         paginated_response(
           items: Schemas.Optimism.OutputRoot,
           next_page_params_example: %{
             "index" => 8829,
             "items_count" => 50
           },
           title_prefix: "OutputRoots"
         )},
      unprocessable_entity: JsonErrorResponse.response()
    ]

  @doc """
  Function to handle GET requests to `/api/v2/optimism/output-roots` endpoint.
  """
  @spec output_roots(Plug.Conn.t(), map()) :: Plug.Conn.t()
  def output_roots(conn, params) do
    {roots, next_page} =
      params
      |> paging_options()
      |> Keyword.put(:api?, true)
      |> OutputRoot.list()
      |> split_list_by_page()

    next_page_params = next_page_params(next_page, roots, params)

    conn
    |> put_status(200)
    |> render(:optimism_output_roots, %{
      roots: roots,
      next_page_params: next_page_params
    })
  end

  operation :output_roots_count,
    summary: "Number of output roots in the list.",
    description: "Retrieves a size of the output roots list.",
    parameters: base_params(),
    responses: [
      ok: {"Number of items in the output roots list.", "application/json", %Schema{type: :integer, nullable: false}},
      unprocessable_entity: JsonErrorResponse.response()
    ]

  @doc """
  Function to handle GET requests to `/api/v2/optimism/output-roots/count` endpoint.
  """
  @spec output_roots_count(Plug.Conn.t(), map()) :: Plug.Conn.t()
  def output_roots_count(conn, _params) do
    items_count(conn, OutputRoot)
  end

  operation :games,
    summary: "List games.",
    description: "Retrieves a paginated list of games.",
    parameters:
      base_params() ++
        define_paging_params([
          "index",
          "items_count"
        ]),
    responses: [
      ok:
        {"List of games.", "application/json",
         paginated_response(
           items: Schemas.Optimism.Game,
           next_page_params_example: %{
             "index" => 12967,
             "items_count" => 50
           },
           title_prefix: "Games"
         )},
      unprocessable_entity: JsonErrorResponse.response()
    ]

  @doc """
  Function to handle GET requests to `/api/v2/optimism/games` endpoint.
  """
  @spec games(Plug.Conn.t(), map()) :: Plug.Conn.t()
  def games(conn, params) do
    {games, next_page} =
      params
      |> paging_options()
      |> Keyword.put(:api?, true)
      |> DisputeGame.list()
      |> split_list_by_page()

    next_page_params = next_page_params(next_page, games, params)

    conn
    |> put_status(200)
    |> render(:optimism_games, %{
      games: games,
      next_page_params: next_page_params
    })
  end

  operation :games_count,
    summary: "Number of games in the list.",
    description: "Retrieves a size of the games list.",
    parameters: base_params(),
    responses: [
      ok: {"Number of items in the games list.", "application/json", %Schema{type: :integer, nullable: false}},
      unprocessable_entity: JsonErrorResponse.response()
    ]

  @doc """
  Function to handle GET requests to `/api/v2/optimism/games/count` endpoint.
  """
  @spec games_count(Plug.Conn.t(), map()) :: Plug.Conn.t()
  def games_count(conn, _params) do
    count = DisputeGame.get_last_known_index() + 1

    conn
    |> put_status(200)
    |> render(:optimism_items_count, %{count: count})
  end

  operation :deposits,
    summary: "List deposits.",
    description: "Retrieves a paginated list of deposits.",
    parameters:
      base_params() ++
        define_paging_params([
          "items_count",
          "l1_block_number",
          "transaction_hash"
        ]),
    responses: [
      ok:
        {"List of deposits.", "application/json",
         paginated_response(
           items: Schemas.Optimism.Deposit,
           next_page_params_example: %{
             "items_count" => 50,
             "l1_block_number" => 23_937_283,
             "transaction_hash" => "0x5dc155c382d95353c5876e735d675d284e3b29b1379e5859dc35cfd4a1dd5188"
           },
           title_prefix: "Deposits"
         )},
      unprocessable_entity: JsonErrorResponse.response()
    ]

  @doc """
  Function to handle GET requests to `/api/v2/optimism/deposits` endpoint.
  """
  @spec deposits(Plug.Conn.t(), map()) :: Plug.Conn.t()
  def deposits(conn, params) do
    {deposits, next_page} =
      params
      |> paging_options()
      |> Keyword.put(:api?, true)
      |> Deposit.list()
      |> split_list_by_page()

    next_page_params = next_page_params(next_page, deposits, params)

    conn
    |> put_status(200)
    |> render(:optimism_deposits, %{
      deposits: deposits,
      next_page_params: next_page_params
    })
  end

  operation :main_page_deposits,
    summary: "List deposits on the main page.",
    description: "Retrieves a list of deposits for the main page.",
    parameters: base_params(),
    responses: [
      ok:
        {"List of deposits on the main page.", "application/json",
         %Schema{type: :array, items: Schemas.Optimism.Deposit.MainPage, nullable: false}},
      unprocessable_entity: JsonErrorResponse.response()
    ]

  @doc """
  Function to handle GET requests to `/api/v2/main-page/optimism-deposits` endpoint.
  """
  @spec main_page_deposits(Plug.Conn.t(), map()) :: Plug.Conn.t()
  def main_page_deposits(conn, _params) do
    recent_deposits =
      Deposit.list(
        paging_options: %PagingOptions{page_size: 6},
        api?: true
      )

    conn
    |> put_status(200)
    |> put_view(OptimismView)
    |> render(:optimism_deposits, %{deposits: recent_deposits})
  end

  operation :deposits_count,
    summary: "Number of deposits in the list.",
    description: "Retrieves a size of the deposits list.",
    parameters: base_params(),
    responses: [
      ok: {"Number of items in the deposits list.", "application/json", %Schema{type: :integer, nullable: false}},
      unprocessable_entity: JsonErrorResponse.response()
    ]

  @doc """
  Function to handle GET requests to `/api/v2/optimism/deposits/count` endpoint.
  """
  @spec deposits_count(Plug.Conn.t(), map()) :: Plug.Conn.t()
  def deposits_count(conn, _params) do
    count = Deposit.count(@api_true)

    conn
    |> put_status(200)
    |> render(:optimism_items_count, %{count: count})
  end

  operation :interop_message, false

  @doc """
  Function to handle GET requests to `/api/v2/optimism/interop/messages/:unique_id` endpoint.
  """
  @spec interop_message(Plug.Conn.t(), map()) :: Plug.Conn.t()
  def interop_message(conn, params) do
    unique_id = Map.get(params, "unique_id", "")

    with true <- String.length(unique_id) == 16,
         {init_chain_id_string, nonce_string} = String.split_at(unique_id, 8),
         {init_chain_id, ""} <- Integer.parse(init_chain_id_string, 16),
         {nonce, ""} <- Integer.parse(nonce_string, 16),
         msg = InteropMessage.get_message(init_chain_id, nonce),
         false <- is_nil(msg) do
      current_chain_id = ChainId.get_id()

      relay_chain_id = msg.relay_chain_id

      direction =
        case current_chain_id do
          ^init_chain_id -> :out
          ^relay_chain_id -> :in
          _ -> nil
        end

      transfer_token =
        if not is_nil(msg.transfer_token_address_hash) do
          case Token.get_by_contract_address_hash(msg.transfer_token_address_hash, @api_true) do
            nil -> %{contract_address_hash: msg.transfer_token_address_hash, symbol: nil, decimals: nil}
            t -> %{contract_address_hash: t.contract_address_hash, symbol: t.symbol, decimals: t.decimals}
          end
        end

      message =
        msg
        |> InteropMessage.extend_with_status()
        |> Map.put(:init_chain, interop_chain_id_to_instance_info(msg.init_chain_id))
        |> Map.put(:relay_chain, interop_chain_id_to_instance_info(msg.relay_chain_id))
        |> Map.put(:direction, direction)
        |> Map.put(:transfer_token, transfer_token)

      conn
      |> put_status(200)
      |> render(:optimism_interop_message, %{message: message})
    else
      _ ->
        conn
        |> put_view(ApiView)
        |> put_status(:not_found)
        |> render(:message, %{message: "Invalid message id or the message with such id is not found"})
    end
  end

  # Calls `InteropMessage.interop_chain_id_to_instance_info` function and depending on the result
  # returns a map with the instance info.
  #
  # ## Parameters
  # - `chain_id`: ID of the chain the instance info is needed for.
  #
  # ## Returns
  # - A map with the instance info.
  # - If the info cannot be retrieved, anyway returns the map with a single `chain_id` item.
  @spec interop_chain_id_to_instance_info(non_neg_integer()) :: map()
  defp interop_chain_id_to_instance_info(chain_id) do
    case InteropMessage.interop_chain_id_to_instance_info(chain_id) do
      nil -> %{chain_id: chain_id}
      chain -> chain
    end
  end

  operation :interop_messages, false

  @doc """
    Function to handle GET requests to `/api/v2/optimism/interop/messages` endpoint.
  """
  @spec interop_messages(Plug.Conn.t(), map()) :: Plug.Conn.t()
  def interop_messages(conn, params) do
    current_chain_id = ChainId.get_id()

    {messages, next_page} =
      params
      |> interop_extract_message_filters()
      |> Keyword.merge(paging_options(params))
      |> Keyword.merge(@api_true)
      |> Keyword.merge(current_chain_id: current_chain_id)
      |> InteropMessage.list()
      |> split_list_by_page()

    next_page_params = next_page_params(next_page, messages, Map.take(params, ["items_count"]))

    messages_extended =
      messages
      |> Enum.map(fn message ->
        message_extended =
          cond do
            message.init_chain_id != current_chain_id and not is_nil(current_chain_id) ->
              Map.put(message, :init_chain, InteropMessage.interop_chain_id_to_instance_info(message.init_chain_id))

            message.relay_chain_id != current_chain_id and not is_nil(current_chain_id) ->
              Map.put(message, :relay_chain, InteropMessage.interop_chain_id_to_instance_info(message.relay_chain_id))

            true ->
              message
          end

        InteropMessage.extend_with_status(message_extended)
      end)

    conn
    |> put_status(200)
    |> render(:optimism_interop_messages, %{
      messages: messages_extended,
      next_page_params: next_page_params
    })
  end

  operation :interop_messages_count, false

  @doc """
  Function to handle GET requests to `/api/v2/optimism/interop/messages/count` endpoint.
  """
  @spec interop_messages_count(Plug.Conn.t(), map()) :: Plug.Conn.t()
  def interop_messages_count(conn, _params) do
    conn
    |> put_status(200)
    |> render(:optimism_items_count, %{count: InteropMessage.count(@api_true)})
  end

  operation :withdrawals,
    summary: "List withdrawals.",
    description: "Retrieves a paginated list of withdrawals.",
    parameters:
      base_params() ++
        define_paging_params([
          "items_count",
          "nonce"
        ]),
    responses: [
      ok:
        {"List of withdrawals.", "application/json",
         paginated_response(
           items: Schemas.Optimism.Withdrawal,
           next_page_params_example: %{
             "items_count" => 50,
             "nonce" => "1766847064778384329583297500742918515827483896875618958121606201292650102"
           },
           title_prefix: "Withdrawals"
         )},
      unprocessable_entity: JsonErrorResponse.response()
    ]

  @doc """
  Function to handle GET requests to `/api/v2/optimism/withdrawals` endpoint.
  """
  @spec withdrawals(Plug.Conn.t(), map()) :: Plug.Conn.t()
  def withdrawals(conn, params) do
    {withdrawals, next_page} =
      params
      |> paging_options()
      |> Keyword.put(:api?, true)
      |> Withdrawal.list()
      |> split_list_by_page()

    next_page_params = next_page_params(next_page, withdrawals, params)

    conn
    |> put_status(200)
    |> render(:optimism_withdrawals, %{
      withdrawals: withdrawals,
      next_page_params: next_page_params
    })
  end

  operation :withdrawals_count,
    summary: "Number of withdrawals in the list.",
    description: "Retrieves a size of the withdrawals list.",
    parameters: base_params(),
    responses: [
      ok: {"Number of items in the withdrawals list.", "application/json", %Schema{type: :integer, nullable: false}},
      unprocessable_entity: JsonErrorResponse.response()
    ]

  @doc """
  Function to handle GET requests to `/api/v2/optimism/withdrawals/count` endpoint.
  """
  @spec withdrawals_count(Plug.Conn.t(), map()) :: Plug.Conn.t()
  def withdrawals_count(conn, _params) do
    items_count(conn, Withdrawal)
  end

  operation :interop_public_key, false

  @doc """
  Function to handle GET requests to `/api/v2/optimism/interop/public-key` endpoint.
  """
  @spec interop_public_key(Plug.Conn.t(), map()) :: Plug.Conn.t()
  def interop_public_key(conn, _params) do
    env = Application.get_all_env(:indexer)[InteropMessageQueue]

    with key when is_binary(key) and byte_size(key) > 0 <- env[:private_key],
         {:ok, private_key} <- key |> String.trim_leading("0x") |> Base.decode16(case: :mixed),
         true <- byte_size(private_key) == 32,
         {:ok, public_key} <- ExSecp256k1.create_public_key(private_key) do
      conn
      |> put_status(200)
      |> render(:optimism_interop_public_key, %{public_key: %Data{bytes: public_key}})
    else
      _ ->
        Logger.error("Interop: cannot derive a public key from the private key. Private key is invalid or undefined.")

        conn
        |> put_view(ApiView)
        |> put_status(:not_found)
        |> render(:message, %{message: "private key is invalid or undefined"})
    end
  end

  operation :interop_import, false

  @doc """
  Function to handle POST request to `/api/v2/import/optimism/interop/` endpoint.
  Accepts `init` part of the interop message from the source instance or
  `relay` part of the interop message from the target instance.
  """
  @spec interop_import(Plug.Conn.t(), map()) :: Plug.Conn.t()
  def interop_import(conn, params) do
    with {:ok, data_to_verify, import_params, remote_chain_id, local_chain_id, missed_part_fn} <-
           interop_validate_payload(params),
         {:ok, signature} <- interop_decode_hex(params["signature"], 64),
         true <- local_chain_id == ChainId.get_id() do
      interop_import_internal(remote_chain_id, data_to_verify, signature, import_params, missed_part_fn, conn)
    else
      _ -> interop_render_http_error(conn, :bad_request, "Invalid interop request")
    end
  end

  @doc """
  Returns the version 1 signing preimage for an interop request.

  The domain is `blockscout:optimism:interop:v1` followed by a NUL byte and
  an init (0) or relay (1) byte. Integers occupy 32 unsigned big-endian bytes;
  addresses and hashes are decoded bytes. Timestamp is signed 64-bit seconds.
  Init includes a 4-byte payload length;
  relay ends with a single result byte. JSON requests must set protocol_version=1.
  """
  @spec interop_signing_data(map()) :: {:ok, binary()} | :error
  def interop_signing_data(params) do
    case interop_validate_payload(params) do
      {:ok, data, _, _, _, _} -> {:ok, data}
      _ -> :error
    end
  end

  @interop_domain "blockscout:optimism:interop:v1\0"
  @interop_common ~w(protocol_version nonce init_chain_id relay_chain_id)
  @interop_init ~w(sender_address_hash target_address_hash init_transaction_hash timestamp payload)
  @interop_relay ~w(relay_transaction_hash failed)

  defp interop_validate_payload(%{"protocol_version" => 1} = params) do
    with true <- interop_integer?(params["nonce"], 9_223_372_036_854_775_807),
         true <- interop_integer?(params["init_chain_id"], 2_147_483_647) and params["init_chain_id"] > 0,
         true <- interop_integer?(params["relay_chain_id"], 2_147_483_647) and params["relay_chain_id"] > 0,
         true <- params["init_chain_id"] != params["relay_chain_id"] do
      common = <<params["nonce"]::256, params["init_chain_id"]::256, params["relay_chain_id"]::256>>
      interop_validate_part(params, common)
    else
      _ -> :error
    end
  end

  defp interop_validate_payload(_), do: :error

  defp interop_validate_part(%{"init_transaction_hash" => _} = params, common) do
    with true <- interop_keys?(params, @interop_common ++ @interop_init),
         {:ok, sender} <- interop_decode_hex(params["sender_address_hash"], 20),
         {:ok, target} <- interop_decode_hex(params["target_address_hash"], 20),
         {:ok, transaction} <- interop_decode_hex(params["init_transaction_hash"], 32),
         true <- interop_integer?(params["timestamp"], 253_402_300_799),
         {:ok, timestamp} <- DateTime.from_unix(params["timestamp"]),
         {:ok, payload} <- interop_decode_hex(params["payload"], :payload),
         true <- interop_transfer_payload?(payload) do
      [token, from, to, amount] = InteropMessage.decode_payload(payload)

      data =
        <<@interop_domain::binary, 0, common::binary, sender::binary, target::binary, transaction::binary,
          params["timestamp"]::signed-64, byte_size(payload)::32, payload::binary>>

      import_params = %{
        sender_address_hash: params["sender_address_hash"],
        target_address_hash: params["target_address_hash"],
        nonce: params["nonce"],
        init_chain_id: params["init_chain_id"],
        relay_chain_id: params["relay_chain_id"],
        init_transaction_hash: params["init_transaction_hash"],
        timestamp: timestamp,
        payload: %Data{bytes: payload},
        transfer_token_address_hash: token,
        transfer_from_address_hash: from,
        transfer_to_address_hash: to,
        transfer_amount: amount
      }

      {:ok, data, import_params, params["init_chain_id"], params["relay_chain_id"], &InteropMessage.get_relay_part/2}
    else
      _ -> :error
    end
  end

  defp interop_validate_part(params, common) do
    with true <- interop_keys?(params, @interop_common ++ @interop_relay),
         {:ok, transaction} <- interop_decode_hex(params["relay_transaction_hash"], 32),
         failed when is_boolean(failed) <- params["failed"] do
      result = if failed, do: 1, else: 0
      data = <<@interop_domain::binary, 1, common::binary, transaction::binary, result>>

      import_params = %{
        nonce: params["nonce"],
        init_chain_id: params["init_chain_id"],
        relay_chain_id: params["relay_chain_id"],
        relay_transaction_hash: params["relay_transaction_hash"],
        failed: failed
      }

      {:ok, data, import_params, params["relay_chain_id"], params["init_chain_id"], &InteropMessage.get_init_part/2}
    else
      _ -> :error
    end
  end

  defp interop_keys?(params, fields), do: Enum.sort(Map.keys(Map.delete(params, "signature"))) == Enum.sort(fields)
  defp interop_integer?(number, maximum), do: is_integer(number) and number >= 0 and number <= maximum

  defp interop_decode_hex("0x" <> hex, size) do
    valid_size =
      if size == :payload,
        do: rem(byte_size(hex), 2) == 0 and byte_size(hex) <= 2_097_152,
        else: byte_size(hex) == size * 2

    if valid_size, do: Base.decode16(hex, case: :mixed), else: :error
  end

  defp interop_decode_hex(_, _), do: :error

  # Known transfer selectors must contain the complete ABI words before decoding.
  defp interop_transfer_payload?(<<0x7C, 0xFD, 0x6D, 0xBC, rest::binary>>), do: byte_size(rest) == 128
  defp interop_transfer_payload?(<<0x4F, 0x0E, 0xDC, 0xC9, rest::binary>>), do: byte_size(rest) == 96
  defp interop_transfer_payload?(_), do: true

  # Implements import logic for the interop message's data sent to `/api/v2/import/optimism/interop/` endpoint.
  # Used by the public `interop_import` function. It requests a public key from the remote Blockscout instance,
  # then after verifying the data with this key, imports the data to database and renders missed part of the message
  # for the remote side (the request was sent from). In case of any error, responds with the corresponding HTTP code
  # and renders the corresponding error message.
  #
  # ## Parameters
  # - `remote_chain_id`: Chain ID of the remote instance which public key should be retrieved for data verifying.
  # - `data_to_verify`: Signed binary data which needs to be verified with the public key.
  # - `signature`: The validated 64-byte compact signature.
  # - `params`: Validated import parameters decoded from the JSON request.
  # - `missed_part_fn`: Reference to the function that should return missed part of the message data (from db) for the remote side.
  # - `conn`: The connection struct.
  #
  # ## Returns
  # - The connection struct with the rendered response.
  @spec interop_import_internal(non_neg_integer(), binary(), binary(), map(), function(), Plug.Conn.t()) ::
          Plug.Conn.t()
  defp interop_import_internal(remote_chain_id, data_to_verify, signature, params, missed_part_fn, conn) do
    instance_api_url = InteropMessage.interop_chain_id_to_instance_api_url(remote_chain_id)

    with :ok <- interop_verify_signature(instance_api_url, data_to_verify, signature),
         # the data is verified, so now we can import that to the database
         {:ok, _} <-
           Chain.import(%{optimism_interop_messages: %{params: [params]}, timeout: :infinity}) do
      conn
      |> put_status(200)
      |> render(:optimism_interop_response, missed_part_fn.(params.init_chain_id, params.nonce))
    else
      :empty_public_key ->
        interop_render_http_error(conn, 500, "Unable to get public key")

      :wrong_signature ->
        interop_render_http_error(conn, :unauthorized, "Wrong signature")

      _ ->
        interop_render_http_error(conn, 500, "Cannot import the data")
    end
  end

  # Gets filter parameters for message list and prepares them for `Explorer.Chain.Optimism.InteropMessage.list/1` function.
  #
  # ## Parameters
  # - `params`: A map with filter parameters defined in HTTP request.
  #
  # ## Returns
  # - A list with prepared filter parameters.
  @spec interop_extract_message_filters(map()) :: list()
  defp interop_extract_message_filters(params) do
    [
      nonce: interop_prepare_nonce_filter(params["interop_message_nonce"]),
      age: interop_prepare_age_filter(params["interop_message_age_from"], params["interop_message_age_to"]),
      statuses: interop_prepare_statuses_filter(params["interop_message_statuses"]),
      init_transaction_hash: interop_prepare_transaction_hash_filter(params["interop_message_init_transaction_hash"]),
      relay_transaction_hash: interop_prepare_transaction_hash_filter(params["interop_message_relay_transaction_hash"]),
      senders:
        interop_prepare_include_exclude_address_hashes_filter(
          params["interop_message_sender_address_hashes_to_include"],
          params["interop_message_sender_address_hashes_to_exclude"]
        ),
      targets:
        interop_prepare_include_exclude_address_hashes_filter(
          params["interop_message_target_address_hashes_to_include"],
          params["interop_message_target_address_hashes_to_exclude"]
        ),
      direction: interop_prepare_direction_filter(params["interop_message_direction"])
    ]
  end

  # Handles the `interop_message_nonce` parameter from HTTP request for the interop message list.
  # Converts the string with nonce to the integer.
  #
  # ## Parameters
  # - `nonce`: The nonce string.
  #
  # ## Returns
  # - The nonce integer in case the nonce string is correct.
  # - `nil` in case of invalid string.
  @spec interop_prepare_nonce_filter(String.t()) :: non_neg_integer() | nil
  defp interop_prepare_nonce_filter(nonce) when is_binary(nonce) do
    nonce
    |> String.trim()
    |> Integer.parse()
    |> case do
      {int_nonce, ""} -> int_nonce
      _ -> nil
    end
  end

  defp interop_prepare_nonce_filter(_), do: nil

  # Handles `interop_message_age_from` and `interop_message_age_to` parameters from HTTP request for the interop message list.
  # Converts the ISO 8601 strings to the corresponding `DateTime` typed values.
  #
  # ## Parameters
  # - `from`: The string with start datetime of the range.
  # - `to`: The string with end datetime of the range.
  #
  # ## Returns
  # - A list `[from: DateTime.t() | nil, to: DateTime.t() | nil]` with the converted values.
  #   The `from` or `to` component can be `nil` if the corresponding input has invalid datetime format.
  @spec interop_prepare_age_filter(String.t(), String.t()) :: list()
  defp interop_prepare_age_filter(from, to), do: [from: parse_date(from), to: parse_date(to)]

  # Converts ISO 8601 string to the corresponding `DateTime` typed value.
  #
  # ## Parameters
  # - `string_date`: The string with datetime in ISO 8601.
  #
  # ## Returns
  # - The converted datetime value.
  # - `nil` in case of invalid input.
  @spec parse_date(String.t()) :: DateTime.t() | nil
  defp parse_date(string_date) do
    case string_date && DateTime.from_iso8601(string_date) do
      {:ok, date, _utc_offset} -> date
      _ -> nil
    end
  end

  @allowed_interop_message_statuses ~w(SENT RELAYED FAILED)

  # Handles the `interop_message_statuses` parameter from HTTP request for the interop message list.
  # Converts the string with statuses to the statuses list.
  #
  # ## Parameters
  # - `statuses`: The string with comma-separated statuses, e.g.: `Sent,Relayed,Failed`.
  #
  # ## Returns
  # - The corresponding list with uppercased items, e.g.: ["SENT","RELAYED","FAILED"]
  # - An empty list if the input string is invalid.
  @spec interop_prepare_statuses_filter(String.t()) :: list()
  defp interop_prepare_statuses_filter(statuses) when is_binary(statuses) do
    statuses
    |> String.upcase()
    |> String.split(",")
    |> Enum.map(&String.trim(&1))
    |> Enum.filter(&(&1 in @allowed_interop_message_statuses))
  end

  defp interop_prepare_statuses_filter(_), do: []

  # Handles the `interop_message_init_transaction_hash` and `interop_message_relay_transaction_hash` parameters
  # from HTTP request for the interop message list. Converts the string with transaction hash to `Explorer.Chain.Hash.t()`.
  #
  # ## Parameters
  # - `transaction_hash`: The transaction hash string containing 64 symbols after `0x` prefix.
  #
  # ## Returns
  # - The transaction hash in case the input string is correct.
  # - `nil` in case of invalid string.
  @spec interop_prepare_transaction_hash_filter(String.t()) :: Hash.t() | nil
  defp interop_prepare_transaction_hash_filter(transaction_hash) when is_binary(transaction_hash) do
    transaction_hash
    |> String.trim()
    |> Chain.string_to_full_hash()
    |> case do
      {:ok, hash} -> hash
      _ -> nil
    end
  end

  defp interop_prepare_transaction_hash_filter(_), do: nil

  # Handles `interop_message_sender_address_hashes_to_include`, `interop_message_sender_address_hashes_to_exclude`,
  # `interop_message_target_address_hashes_to_include`, and `interop_message_target_address_hashes_to_exclude` parameters
  # from HTTP request for the interop message list. Converts the strings containing comma-separated addresses
  # to the input lists of `Hash.Address.t()` for `Explorer.Chain.Optimism.InteropMessage.list/1` function.
  # Each address must have 40 hexadecimal digits after the `0x` base prefix.
  #
  # ## Parameters
  # - `include`: accepts `interop_message_sender_address_hashes_to_include` or `interop_message_target_address_hashes_to_include` parameter value.
  # - `exclude`: accepts `interop_message_sender_address_hashes_to_exclude` or `interop_message_target_address_hashes_to_exclude` parameter value.
  #
  # ## Returns
  # - A list `[include: [Hash.Address.t()], exclude: [Hash.Address.t()]]` with the converted addresses.
  @spec interop_prepare_include_exclude_address_hashes_filter(String.t(), String.t()) :: [
          include: [Hash.Address.t()],
          exclude: [Hash.Address.t()]
        ]
  defp interop_prepare_include_exclude_address_hashes_filter(include, exclude) do
    [
      include: interop_prepare_address_hashes_filter(include),
      exclude: interop_prepare_address_hashes_filter(exclude)
    ]
  end

  # Converts the string containing comma-separated addresses to the list of `Hash.Address.t()`.
  # Each address must have 40 hexadecimal digits after the `0x` base prefix.
  #
  # ## Parameters
  # - `address_hashes`: The string with comma-separated addresses.
  #
  # ## Returns
  # - The corresponding list `[Hash.Address.t()]`. The list is empty if the input string is empty or all addresses
  #   are incorrect. Incorrect addresses are not included into the list.
  @spec interop_prepare_address_hashes_filter(String.t()) :: [Hash.Address.t()]
  defp interop_prepare_address_hashes_filter(address_hashes) when is_binary(address_hashes) do
    address_hashes
    |> String.split(",")
    |> Enum.map(&interop_prepare_address_hash_filter(&1))
    |> Enum.reject(&is_nil(&1))
  end

  defp interop_prepare_address_hashes_filter(_), do: nil

  # Converts an address hash string with `0x` prefix to `Hash.Address.t()`.
  # The address must have 40 hexadecimal digits after the `0x` base prefix.
  #
  # ## Parameters
  # - `address_hash`: The input string with address.
  #
  # ## Returns
  # - `Hash.Address.t()` in case of correct address.
  # - `nil` if the address is invalid.
  @spec interop_prepare_address_hash_filter(String.t()) :: Hash.Address.t() | nil
  defp interop_prepare_address_hash_filter(address_hash) do
    address_hash
    |> String.trim()
    |> Chain.string_to_address_hash_or_nil()
  end

  # Handles the `interop_message_direction` parameter from HTTP request for the interop message list.
  # Converts the string with the direction to the corresponding atom.
  #
  # ## Parameters
  # - `direction`: The direction string. Can be one of: "in", "out".
  #
  # ## Returns
  # - `:in` for the incoming direction.
  # - `:out` for the outgoing direction.
  # - `nil` for all directions.
  @spec interop_prepare_direction_filter(String.t() | nil) :: :in | :out | nil
  defp interop_prepare_direction_filter(direction) do
    case direction && String.downcase(direction) do
      "in" -> :in
      "out" -> :out
      _ -> nil
    end
  end

  # Renders HTTP error code and message.
  #
  # ## Parameters
  # - `conn`: The connection struct.
  # - `error_code`: The error code (e.g. 500 or :unauthorized).
  # - `error_message`: The error description.
  #
  # ## Returns
  # - The connection struct with the rendered response.
  @spec interop_render_http_error(Plug.Conn.t(), atom() | non_neg_integer(), String.t()) :: Plug.Conn.t()
  defp interop_render_http_error(conn, error_code, error_message) do
    Logger.error("Interop: #{error_message}")

    conn
    |> put_view(ApiView)
    |> put_status(error_code)
    |> render(:message, %{message: error_message})
  end

  @interop_public_key_lifetime :timer.minutes(5)

  defp interop_verify_signature(nil, _, _), do: :empty_public_key

  defp interop_verify_signature(instance_api_url, data, signature) do
    cache = InteropMessage.interop_instance_api_url_to_public_key_cache()
    digest = ExKeccak.hash_256(data)

    # Serialize refresh and verification so an older in-flight fetch cannot restore a retired key.
    ConCache.isolated(cache, instance_api_url, fn ->
      now = System.monotonic_time(:millisecond)

      case ConCache.get(cache, instance_api_url) do
        %{public_key: public_key, key_version: version, expires_at: expires_at}
        when is_binary(public_key) and is_binary(version) and is_integer(expires_at) and expires_at > now ->
          if ExSecp256k1.verify(digest, signature, public_key) == :ok do
            :ok
          else
            # A signature failure permits one authenticated refresh, never signature bypass.
            interop_verify_remote_key(instance_api_url, digest, signature, :wrong_signature)
          end

        _ ->
          # Also discards pre-upgrade, unbounded binary cache entries.
          interop_verify_remote_key(instance_api_url, digest, signature, :empty_public_key)
      end
    end)
  end

  defp interop_verify_remote_key(instance_api_url, digest, signature, fetch_error) do
    case interop_fetch_public_key(instance_api_url) do
      nil -> fetch_error
      key -> if ExSecp256k1.verify(digest, signature, key) == :ok, do: :ok, else: :wrong_signature
    end
  end

  # The configured instance authenticates its current key through certificate- and
  # hostname-verified HTTPS. Its fingerprint is the key version; replacements retire
  # the previous version. Expiry is checked on reads even with ConCache's timer off.
  @spec interop_fetch_public_key(String.t()) :: binary() | nil
  defp interop_fetch_public_key(instance_api_url) do
    cache = InteropMessage.interop_instance_api_url_to_public_key_cache()
    ConCache.dirty_delete(cache, instance_api_url)
    env = Application.get_all_env(:indexer)[InteropMessageQueue]
    url = instance_api_url <> "/api/v2/optimism/interop/public-key"
    timeout = :timer.seconds(env[:connect_timeout])
    recv_timeout = :timer.seconds(env[:recv_timeout])
    client = Tesla.client([{Tesla.Middleware.Timeout, timeout: recv_timeout}], Tesla.Adapter.Mint)

    with %URI{scheme: "https", host: host, userinfo: nil} when is_binary(host) and host != "" <- URI.parse(url),
         {:ok, %{body: response_body, status: 200}} <-
           Tesla.get(client, url,
             opts: [adapter: [timeout: recv_timeout, transport_opts: [timeout: timeout, verify: :verify_peer]]]
           ),
         {:ok, %{"public_key" => "0x" <> key}} <- Jason.decode(response_body),
         {:ok, key_binary} <- Base.decode16(key, case: :mixed),
         true <- byte_size(key_binary) in [33, 65] do
      ConCache.dirty_put(cache, instance_api_url, %{
        public_key: key_binary,
        key_version: :crypto.hash(:sha256, key_binary),
        expires_at: System.monotonic_time(:millisecond) + @interop_public_key_lifetime
      })

      key_binary
    else
      _ ->
        Logger.error("Interop: unable to get an authenticated public key")
        nil
    end
  end

  defp items_count(conn, module) do
    count = Chain.get_table_rows_total_count(module, api?: true)

    conn
    |> put_status(200)
    |> render(:optimism_items_count, %{count: count})
  end
end
