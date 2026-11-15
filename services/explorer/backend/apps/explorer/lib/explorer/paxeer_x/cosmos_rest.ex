defmodule Explorer.PaxeerX.CosmosRest do
  @moduledoc """
  Reads the native price-feed submissions of one block from a node.

  The entries come from the consensus route `GET /block?height={height}`. Each
  transaction is decoded from its protobuf wire bytes; other entries keep their
  position without producing a submission. Execution results come from
  `GET /block_results?height={height}` only when the block carries a submission,
  and join by the entry's position in the block.

  Every request starts at the URL that last answered for the same URL list and goes
  round the list at most `@rounds` times, so one dead node costs one short connect
  timeout until another node answers. A node that no longer serves the height answers
  `height_unavailable`, which is returned only when every attempt answered so; a node
  whose head is below the height answers `height_ahead`. A definitive failure wins
  over transient failures, followed by `height_ahead` so the caller retries later.
  Otherwise the last failure other than `height_unavailable` is returned.
  """

  import Bitwise

  alias Explorer.Chain.PaxeerX.Identity

  @submit_prices_type "/paxprotocol.paxchain.pricefeed.MsgSubmitPrices"
  @rounds 2
  @retry_delay_ms 200
  @max_body_bytes 33_554_432
  @http_options [recv_timeout: 30_000, timeout: 2_000, follow_redirect: false]
  @max_int64 9_223_372_036_854_775_807
  @max_uint64 18_446_744_073_709_551_615

  @type error ::
          :no_urls
          | {:height_unavailable, String.t()}
          | {:height_ahead, String.t()}
          | {:unreachable, String.t()}
          | {:http_status, non_neg_integer(), String.t()}
          | {:node_error, String.t(), String.t()}
          | {:too_large, String.t()}
          | {:malformed, String.t()}

  @type observation :: %{
          optional(:accepted) => boolean(),
          market_id: non_neg_integer(),
          sequence: non_neg_integer(),
          index_price: non_neg_integer(),
          mark_price: non_neg_integer(),
          source_timestamp_ms: non_neg_integer()
        }

  @type price_update :: %{
          hash: String.t(),
          index: non_neg_integer(),
          feeder_bech32: String.t(),
          feeder_hash: String.t(),
          status: 0 | 1,
          gas_used: non_neg_integer(),
          gas_wanted: non_neg_integer(),
          observations: [observation()]
        }

  @doc """
  The price-feed submissions of the block at `height`, in CometBFT transaction order.
  """
  @spec block_price_updates([String.t()], [String.t()], non_neg_integer()) ::
          {:ok, [price_update()]} | {:error, error()}
  def block_price_updates(_rest_urls, rpc_urls, height) when is_integer(height) and height >= 0 do
    with {:ok, block_txs} <- fetch_block_txs(rpc_urls, height),
         {:ok, submissions} <- decode_block_txs(block_txs) do
      case submissions do
        [] ->
          {:ok, []}

        _ ->
          with {:ok, results} <- fetch_block_results(rpc_urls, height) do
            decode_price_updates(submissions, results, length(block_txs["raw"]))
          end
      end
    end
  end

  @doc """
  The consensus block's base64 entries in their original order.
  """
  @spec fetch_block_txs([String.t()], non_neg_integer()) :: {:ok, map()} | {:error, error()}
  def fetch_block_txs(urls, height) do
    case get_json(urls, "/block?height=#{height}", fn _, _ -> false end) do
      {:ok, %{"result" => %{} = result}} -> block_entries(result)
      {:ok, %{} = result} -> block_entries(result)
      error -> error
    end
  end

  defp block_entries(%{"block" => %{"data" => %{} = data}}) do
    case Map.get(data, "txs") do
      nil -> {:ok, %{"raw" => []}}
      entries when is_list(entries) -> {:ok, %{"raw" => entries}}
      _ -> {:error, {:malformed, "unexpected block entries response"}}
    end
  end

  defp block_entries(_), do: {:error, {:malformed, "unexpected block entries response"}}

  @doc """
  The CometBFT execution results of the block's transactions, in transaction order.
  """
  @spec fetch_block_results([String.t()], non_neg_integer()) :: {:ok, [map()]} | {:error, error()}
  def fetch_block_results(urls, height) do
    case get_json(urls, "/block_results?height=#{height}", fn _, _ -> false end) do
      {:ok, %{"result" => %{} = result}} -> results(result)
      {:ok, %{} = result} -> results(result)
      error -> error
    end
  end

  defp results(%{"txs_results" => nil}), do: {:ok, []}
  defp results(%{"txs_results" => results}) when is_list(results), do: {:ok, results}
  defp results(_), do: {:error, {:malformed, "unexpected block results response"}}

  @doc """
  The price-feed submissions among raw block entries, each with its index, hash and
  declared gas limit, before execution results are joined in. Retained payloads
  with decoded transactions are accepted for acceptance replay.
  """
  @spec decode_block_txs(map()) :: {:ok, [map()]} | {:error, error()}
  def decode_block_txs(%{"txs" => txs, "raw" => raw}) do
    txs
    |> Enum.with_index()
    |> Enum.reduce_while({:ok, []}, fn {tx, index}, {:ok, acc} ->
      case decode_tx(tx, Enum.at(raw, index), index) do
        {:ok, nil} -> {:cont, {:ok, acc}}
        {:ok, submission} -> {:cont, {:ok, [submission | acc]}}
        {:error, _} = error -> {:halt, error}
      end
    end)
    |> case do
      {:ok, submissions} -> {:ok, Enum.reverse(submissions)}
      error -> error
    end
  end

  def decode_block_txs(%{"raw" => raw}) when is_list(raw) do
    raw
    |> Enum.with_index()
    |> Enum.reduce_while({:ok, []}, fn {entry, index}, {:ok, acc} ->
      case decode_entry(entry, index) do
        {:ok, nil} -> {:cont, {:ok, acc}}
        {:ok, submission} -> {:cont, {:ok, [submission | acc]}}
        {:error, _} = error -> {:halt, error}
      end
    end)
    |> case do
      {:ok, submissions} -> {:ok, Enum.reverse(submissions)}
      error -> error
    end
  end

  def decode_block_txs(_), do: {:error, {:malformed, "unexpected block entries"}}

  defp decode_entry(raw, index) when is_binary(raw) do
    case Base.decode64(raw) do
      {:ok, bytes} ->
        case wire_transaction(bytes) do
          {:ok, tx} ->
            decode_tx(tx, raw, index)

          :error ->
            if :binary.match(bytes, @submit_prices_type) == :nomatch,
              do: {:ok, nil},
              else: {:error, {:malformed, "invalid submission wire bytes"}}
        end

      :error ->
        {:error, {:malformed, "raw transaction is not base64"}}
    end
  end

  defp decode_entry(_, _), do: {:error, {:malformed, "missing raw transaction"}}

  defp wire_transaction(bytes) do
    with {:ok, tx} <- wire_fields(bytes, %{1 => 2, 2 => 2, 3 => 2}),
         {:ok, body} <- wire_fields(wire_value(tx, 1, 2, ""), %{1 => 2}),
         {:ok, messages} <- wire_messages(wire_values(body, 1, 2)),
         {:ok, auth} <- wire_fields(wire_value(tx, 2, 2, ""), %{1 => 2, 2 => 2}),
         {:ok, fee} <- wire_fields(wire_value(auth, 2, 2, ""), %{2 => 0}) do
      {:ok,
       %{"body" => %{"messages" => messages}, "auth_info" => %{"fee" => %{"gas_limit" => wire_value(fee, 2, 0, 0)}}}}
    end
  end

  defp wire_messages(messages) do
    Enum.reduce_while(messages, {:ok, []}, fn bytes, {:ok, acc} ->
      with {:ok, fields} <- wire_fields(bytes, %{1 => 2, 2 => 2}),
           {:ok, message} <- wire_message(wire_value(fields, 1, 2, ""), wire_value(fields, 2, 2, "")) do
        {:cont, {:ok, [message | acc]}}
      else
        :error -> {:halt, :error}
      end
    end)
    |> case do
      {:ok, messages} -> {:ok, Enum.reverse(messages)}
      :error -> :error
    end
  end

  defp wire_message(@submit_prices_type, bytes) do
    with {:ok, fields} <- wire_fields(bytes, %{1 => 2, 2 => 2}),
         [_ | _] = observations <- wire_values(fields, 2, 2),
         {:ok, observations} <- wire_observations(observations) do
      {:ok, %{"@type" => @submit_prices_type, "feeder" => wire_value(fields, 1, 2, ""), "observations" => observations}}
    else
      _ -> :error
    end
  end

  defp wire_message(type, _), do: {:ok, %{"@type" => type}}

  defp wire_observations(observations) do
    Enum.reduce_while(observations, {:ok, []}, fn bytes, {:ok, acc} ->
      case wire_fields(bytes, %{1 => 0, 2 => 0, 3 => 2, 4 => 2, 5 => 0}) do
        {:ok, fields} ->
          observation = %{
            "market_id" => wire_value(fields, 1, 0, 0),
            "sequence" => wire_value(fields, 2, 0, 0),
            "index_price" => wire_value(fields, 3, 2, ""),
            "mark_price" => wire_value(fields, 4, 2, ""),
            "source_timestamp_ms" => wire_value(fields, 5, 0, 0)
          }

          {:cont, {:ok, [observation | acc]}}

        :error ->
          {:halt, :error}
      end
    end)
    |> case do
      {:ok, observations} -> {:ok, Enum.reverse(observations)}
      :error -> :error
    end
  end

  defp wire_value(fields, number, type, default), do: fields |> wire_values(number, type) |> List.last(default)

  defp wire_values(fields, number, type) do
    for {^number, ^type, value} <- fields, do: value
  end

  defp wire_fields(bytes, schema) do
    with {:ok, fields} <- parse_wire_fields(bytes, []),
         true <- Enum.all?(fields, fn {number, type, _} -> Map.get(schema, number, type) == type end) do
      {:ok, fields}
    else
      _ -> :error
    end
  end

  defp parse_wire_fields(<<>>, acc), do: {:ok, Enum.reverse(acc)}

  defp parse_wire_fields(bytes, acc) do
    case wire_varint(bytes, 0, 0) do
      {:ok, key, rest} when key >>> 3 > 0 and key >>> 3 <= 536_870_911 ->
        case wire_field(key &&& 7, rest) do
          {:ok, value, tail} -> parse_wire_fields(tail, [{key >>> 3, key &&& 7, value} | acc])
          :error -> :error
        end

      _ ->
        :error
    end
  end

  defp wire_field(0, bytes), do: wire_varint(bytes, 0, 0)
  defp wire_field(1, <<value::binary-size(8), rest::binary>>), do: {:ok, value, rest}
  defp wire_field(5, <<value::binary-size(4), rest::binary>>), do: {:ok, value, rest}

  defp wire_field(2, bytes) do
    with {:ok, size, rest} <- wire_varint(bytes, 0, 0),
         <<value::binary-size(size), tail::binary>> <- rest do
      {:ok, value, tail}
    else
      _ -> :error
    end
  end

  defp wire_field(_, _), do: :error

  defp wire_varint(<<byte, rest::binary>>, value, shift) when shift < 63 or (shift == 63 and byte <= 1) do
    value = value ||| (byte &&& 127) <<< shift
    if byte < 128, do: {:ok, value, rest}, else: wire_varint(rest, value, shift + 7)
  end

  defp wire_varint(_, _, _), do: :error

  defp decode_tx(%{"body" => %{"messages" => messages}} = tx, raw, index) when is_list(messages) do
    case Enum.filter(messages, &match?(%{"@type" => @submit_prices_type}, &1)) do
      [] ->
        {:ok, nil}

      [%{"feeder" => feeder} | _] = submissions ->
        with {:ok, hash} <- tx_hash(raw),
             {:ok, feeder_bytes} <- feeder(feeder),
             true <- Enum.all?(submissions, &(&1["feeder"] == feeder)) || {:error, {:malformed, "mixed feeders"}},
             {:ok, gas_wanted} <- integer(get_in(tx, ["auth_info", "fee", "gas_limit"]) || "0", @max_int64),
             {:ok, observations} <- observations(Enum.flat_map(submissions, &List.wrap(&1["observations"]))) do
          {:ok,
           %{
             hash: hash,
             index: index,
             feeder_bech32: feeder,
             feeder_hash: "0x" <> Base.encode16(feeder_bytes, case: :lower),
             gas_wanted: gas_wanted,
             observations: observations
           }}
        end

      _ ->
        {:error, {:malformed, "submission without a feeder"}}
    end
  end

  defp decode_tx(_tx, _raw, _index), do: {:error, {:malformed, "transaction without a body"}}

  defp tx_hash(raw) when is_binary(raw) do
    case Base.decode64(raw) do
      {:ok, bytes} -> {:ok, "0x" <> Base.encode16(:crypto.hash(:sha256, bytes), case: :lower)}
      :error -> {:error, {:malformed, "raw transaction is not base64"}}
    end
  end

  defp tx_hash(_raw), do: {:error, {:malformed, "missing raw transaction"}}

  defp feeder(feeder) when is_binary(feeder) do
    case Identity.pax_address_bytes(feeder) do
      {:ok, bytes} -> {:ok, bytes}
      :error -> {:error, {:malformed, "feeder is not a pax account"}}
    end
  end

  defp feeder(_feeder), do: {:error, {:malformed, "feeder is not a pax account"}}

  defp observations(observations) do
    observations
    |> Enum.reduce_while({:ok, %{}}, fn observation, {:ok, acc} ->
      case observation(observation) do
        {:ok, decoded} -> {:cont, {:ok, Map.put(acc, decoded.market_id, decoded)}}
        error -> {:halt, error}
      end
    end)
    |> case do
      {:ok, by_market} -> {:ok, by_market |> Map.values() |> Enum.sort_by(& &1.market_id)}
      error -> error
    end
  end

  defp observation(%{} = observation) do
    with {:ok, market_id} <- integer(observation["market_id"], @max_uint64),
         {:ok, sequence} <- integer(observation["sequence"], @max_uint64),
         {:ok, index_price} <- price(observation["index_price"]),
         {:ok, mark_price} <- price(observation["mark_price"]),
         {:ok, source_timestamp_ms} <- integer(observation["source_timestamp_ms"], @max_uint64) do
      {:ok,
       %{
         market_id: market_id,
         sequence: sequence,
         index_price: index_price,
         mark_price: mark_price,
         source_timestamp_ms: source_timestamp_ms
       }}
    end
  end

  defp observation(_observation), do: {:error, {:malformed, "observation is not an object"}}

  defp price(value) when is_binary(value) and byte_size(value) <= 78, do: integer(value, nil)
  defp price(_value), do: {:error, {:malformed, "price is not an 18-decimal integer"}}

  defp integer(value, max) when is_binary(value) do
    if value =~ ~r/\A(0|[1-9][0-9]*)\z/ and (is_nil(max) or String.to_integer(value) <= max) do
      {:ok, String.to_integer(value)}
    else
      {:error, {:malformed, "not an unsigned integer in range: #{inspect(value)}"}}
    end
  end

  defp integer(value, max) when is_integer(value) and value >= 0 and (is_nil(max) or value <= max), do: {:ok, value}
  defp integer(value, _max), do: {:error, {:malformed, "not an unsigned integer in range: #{inspect(value)}"}}

  @doc """
  Joins the execution result at each submission's index: status 1 for code 0, else 0,
  and the gas used.
  """
  @spec decode_price_updates([map()], [map()], non_neg_integer()) :: {:ok, [price_update()]} | {:error, error()}
  def decode_price_updates(submissions, results, transaction_count) do
    if length(results) == transaction_count do
      submissions
      |> Enum.reduce_while({:ok, []}, fn submission, {:ok, acc} ->
        result = Enum.at(results, submission.index)

        with %{} <- result,
             {:ok, code} <- integer(Map.get(result, "code", 0), nil),
             {:ok, gas_used} <- integer(Map.get(result, "gas_used", "0"), @max_int64) do
          refused_attributes =
            for %{"type" => "pricefeed_observation_refused", "attributes" => attributes} <- result["events"] || [],
                %{"key" => key, "value" => value} <- attributes,
                into: MapSet.new(),
                do: {key, value}

          observations =
            Enum.map(submission.observations, fn observation ->
              market_id = Integer.to_string(observation.market_id)

              refused? =
                MapSet.member?(refused_attributes, {"market_id", market_id}) or
                  MapSet.member?(refused_attributes, {Base.encode64("market_id"), Base.encode64(market_id)})

              Map.put(observation, :accepted, code == 0 and not refused?)
            end)

          update =
            Map.merge(submission, %{
              status: if(code == 0, do: 1, else: 0),
              gas_used: gas_used,
              observations: observations
            })

          {:cont, {:ok, [update | acc]}}
        else
          {:error, _} = error -> {:halt, error}
          _ -> {:halt, {:error, {:malformed, "execution result is not an object"}}}
        end
      end)
      |> case do
        {:ok, updates} -> {:ok, Enum.reverse(updates)}
        error -> error
      end
    else
      {:error, {:malformed, "execution results do not match the block transactions"}}
    end
  end

  defp get_json([], _path, _accept?), do: {:error, :no_urls}

  defp get_json(urls, path, accept?) do
    count = length(urls)
    preferred = preferred_url(urls)
    start = :atomics.get(preferred, 1)

    0..(@rounds * count - 1)
    |> Enum.reduce_while([], fn attempt, errors ->
      if attempt >= count, do: Process.sleep(@retry_delay_ms)
      position = rem(start + attempt, count)

      case get_once(Enum.at(urls, position), path, accept?) do
        {:ok, _} = ok ->
          :atomics.put(preferred, 1, position)
          {:halt, ok}

        {:error, reason} ->
          {:cont, [reason | errors]}
      end
    end)
    |> case do
      {:ok, _} = ok -> ok
      errors -> {:error, pick_error(Enum.reverse(errors))}
    end
  end

  defp pick_error(errors) do
    definitive =
      Enum.find(errors, fn
        {kind, _} when kind in [:unreachable, :height_unavailable, :height_ahead] -> false
        {:http_status, status, _} when status in 500..599 -> false
        _ -> true
      end)

    cond do
      definitive -> definitive
      Enum.all?(errors, &match?({:height_unavailable, _}, &1)) -> List.last(errors)
      ahead = Enum.find(errors, &match?({:height_ahead, _}, &1)) -> ahead
      true -> errors |> Enum.reject(&match?({:height_unavailable, _}, &1)) |> List.last()
    end
  end

  # ponytail: one atomic per distinct URL list, created once; the lists come from config.
  defp preferred_url(urls) do
    key = {__MODULE__, :preferred, urls}

    case :persistent_term.get(key, nil) do
      nil ->
        ref = :atomics.new(1, [])
        :persistent_term.put(key, ref)
        ref

      ref ->
        ref
    end
  end

  defp get_once(url, path, accept?) do
    case HTTPoison.get(url <> path, [{"accept", "application/json"}], @http_options) do
      {:ok, %HTTPoison.Response{body: body}} when byte_size(body) > @max_body_bytes ->
        {:error, {:too_large, url}}

      {:ok, %HTTPoison.Response{status_code: status, body: body}} ->
        case {status, Jason.decode(body)} do
          {200, {:ok, %{"code" => _, "message" => message} = decoded}}
          when is_binary(message) and not is_map_key(decoded, "result") ->
            case node_error(status, body, url) do
              {:error, {:height_unavailable, _}} = error -> error
              _ -> {:error, {:node_error, url, message}}
            end

          {200, {:ok, %{"error" => _}}} ->
            node_error(status, body, url)

          {200, {:ok, %{} = decoded}} ->
            {:ok, decoded}

          {_, {:ok, %{} = decoded}} ->
            if accept?.(status, decoded), do: {:ok, :empty}, else: node_error(status, body, url)

          {200, _} ->
            {:error, {:malformed, "response from #{url} is not a JSON object"}}

          _ ->
            node_error(status, body, url)
        end

      {:error, %HTTPoison.Error{}} ->
        {:error, {:unreachable, url}}
    end
  end

  # CometBFT answers with one of several pruned-height messages below its retained
  # history. The Cosmos REST route wraps those texts in both successful and failed
  # response envelopes. A height above the node's head has a separate retryable error.
  defp node_error(status, body, url) do
    cond do
      body =~ ~r/height \d+ is not available(?:, lowest height is \d+)?/ or
          String.contains?(body, "height is not available") ->
        {:error, {:height_unavailable, url}}

      body =~ ~r/must be less than or equal to|greater than the current height/ ->
        {:error, {:height_ahead, url}}

      true ->
        {:error, {:http_status, status, url}}
    end
  end
end
