defmodule EthereumJSONRPC.DecodeError do
  @moduledoc """
  An error has occurred decoding the response to an `EthereumJSONRPC.json_rpc` request.
  """

  @enforce_keys [:request, :response]
  defexception [:request, :response]

  defmodule Request do
    @moduledoc """
    Ethereum JSONRPC request whose `EthereumJSONRPC.DecodeError.Response` had a decode error.
    """

    @enforce_keys [:url, :body, :headers]
    defstruct [:url, :body, :headers]
  end

  defmodule Response do
    @moduledoc """
    Ethereum JSONRPC response that had a decode error.
    """

    @enforce_keys [:status_code, :body]
    defstruct [:status_code, :body]
  end

  @impl Exception
  def exception(named_arguments) do
    request_fields = Keyword.fetch!(named_arguments, :request)
    request = struct!(EthereumJSONRPC.DecodeError.Request, request_fields)

    response_fields = Keyword.fetch!(named_arguments, :response)
    response = struct!(EthereumJSONRPC.DecodeError.Response, response_fields)

    %EthereumJSONRPC.DecodeError{request: request, response: response}
  end

  @request_body_limit 700
  @response_body_limit 2_048
  @safe_headers ~w(accept accept-encoding content-type content-length)

  @impl Exception
  def message(%__MODULE__{request: request, response: response} = error) do
    secrets = secret_values(request.url, request.headers)

    headers =
      Enum.map(request.headers, fn {key, value} ->
        value =
          if String.downcase(to_string(key)) in @safe_headers, do: bounded(value, 100, secrets), else: "[redacted]"

        {bounded(to_string(key), 80, secrets), value}
      end)
      |> Enum.take(30)

    url = if Map.get(error, :hide_url, false), do: "rpc_url", else: redact_url(request.url)

    """
    Failed to decode Ethereum JSONRPC response:

      request:
        url: #{url}
        body: #{inspect(bounded(request.body, @request_body_limit, secrets))}
        headers: #{inspect(headers, limit: 30, printable_limit: 1_024)}

      response:
        status code: #{response.status_code}
        body: #{inspect(bounded(response.body, @response_body_limit, secrets), printable_limit: @response_body_limit)}
    """
  end

  @doc false
  def redact_url(url) when is_binary(url) do
    case URI.parse(url) do
      %URI{scheme: scheme, host: host} = uri when scheme in ["http", "https"] and is_binary(host) ->
        %URI{uri | userinfo: nil, path: "/[redacted]", query: nil, fragment: nil}
        |> URI.to_string()
        |> String.slice(0, 256)

      _ ->
        "[invalid endpoint]"
    end
  rescue
    ArgumentError -> "[invalid endpoint]"
  end

  def redact_url(_), do: "[invalid endpoint]"

  defp secret_values(url, headers) do
    uri = URI.parse(url)

    header_values =
      headers
      |> Enum.reject(fn {key, _} -> String.downcase(to_string(key)) in @safe_headers end)
      |> Enum.flat_map(fn {_key, value} -> [to_string(value) | String.split(to_string(value))] end)

    url_values =
      [uri.userinfo, uri.path, uri.query, uri.fragment]
      |> Enum.reject(&is_nil/1)
      |> Enum.flat_map(fn value -> [value, URI.decode(value) | String.split(value, ["/", ":", "=", "&"])] end)

    (header_values ++ url_values)
    |> Enum.reject(&(&1 == ""))
    |> Enum.uniq()
    |> Enum.sort_by(&byte_size/1, :desc)
  end

  defp bounded(data, limit, secrets) do
    binary = IO.iodata_to_binary(data)
    redacted = Enum.reduce(secrets, binary, &String.replace(&2, &1, "[redacted]"))

    if byte_size(redacted) > limit do
      binary_part(redacted, 0, limit) <> "[truncated]"
    else
      redacted
    end
  end
end
