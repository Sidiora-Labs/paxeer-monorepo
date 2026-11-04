defmodule EthereumJSONRPC.HTTP.Helper do
  @moduledoc """
  Helper functions for `EthereumJSONRPC.HTTP` implementations.
  """

  alias EthereumJSONRPC.Prometheus.Instrumenter

  @doc "Counts every outgoing member and retains its ID for response attribution."
  @spec record_requests(iodata()) :: [map()]
  def record_requests(json) do
    requests = json |> IO.iodata_to_binary() |> Jason.decode!() |> List.wrap()
    Enum.each(requests, &Instrumenter.json_rpc_requests(Map.fetch!(&1, "method")))
    requests
  end

  @doc "Counts JSON-RPC errors by response ID, independently of response order."
  @spec record_response_errors([map()], binary()) :: :ok
  def record_response_errors(requests, body) do
    case Jason.decode(body) do
      {:ok, response} ->
        errors =
          response
          |> List.wrap()
          |> Enum.filter(&response_body_has_error?/1)
          |> MapSet.new(&Map.get(&1, "id"))

        Enum.each(requests, fn request ->
          if Map.has_key?(request, "id") and MapSet.member?(errors, request["id"]) do
            Instrumenter.json_rpc_errors(Map.fetch!(request, "method"))
          end
        end)

      {:error, _} ->
        :ok
    end

    :ok
  end

  @doc "A whole-transport failure counts one error for every outgoing member."
  @spec record_transport_errors([map()]) :: :ok
  def record_transport_errors(requests) do
    Enum.each(requests, &Instrumenter.json_rpc_errors(Map.fetch!(&1, "method")))
  end

  @doc """
  Extracts the JSON-RPC method from a JSON string payload.

  Supports both single objects and batch requests (arrays).

  ## Parameters
  - `json_string`: The JSON string to parse

  ## Returns
  - The method name as a binary, or `{:error, Jason.DecodeError.t()}` if extraction fails
  """
  @spec get_method_from_json_string(binary()) :: binary() | {:error, Jason.DecodeError.t()}
  def get_method_from_json_string(json_string) do
    with {:ok, decoded_json} <- Jason.decode(json_string) do
      if is_map(decoded_json) do
        Map.get(decoded_json, "method")
      else
        decoded_json |> Enum.at(0) |> Map.get("method")
      end
    end
  end

  @spec response_body_has_error?(map() | [map()]) :: boolean()
  def response_body_has_error?(decoded_body) when is_map(decoded_body) do
    Map.has_key?(decoded_body, "error")
  end

  def response_body_has_error?(decoded_body) when is_list(decoded_body) do
    Enum.any?(decoded_body, &response_body_has_error?/1)
  end

  def response_body_has_error?(_decoded_body), do: false

  @doc """
  Conditionally decompresses gzip-encoded HTTP response bodies.

  Checks application configuration and HTTP headers to determine if decompression
  should be attempted.

  ## Parameters
  - `body`: The response body to potentially decompress
  - `headers`: List of HTTP response headers as {key, value} tuples

  ## Returns
  - Decompressed body if gzip-enabled and content is gzipped, otherwise original body
  """
  @spec try_unzip(binary(), [{binary(), binary()}]) :: binary()
  def try_unzip(body, headers) do
    gzip_enabled? = Application.get_env(:ethereum_jsonrpc, EthereumJSONRPC.HTTP)[:gzip_enabled?]

    if gzip_enabled? do
      do_unzip(body, headers)
    else
      body
    end
  end

  defp do_unzip(body, headers) do
    gzipped =
      Enum.any?(
        headers
        |> Enum.map(fn {k, v} ->
          {String.downcase(k), String.downcase(v)}
        end),
        fn kv ->
          case kv do
            {"content-encoding", "gzip"} -> true
            {"content-encoding", "x-gzip"} -> true
            _ -> false
          end
        end
      )

    if gzipped do
      :zlib.gunzip(body)
    else
      body
    end
  end
end
