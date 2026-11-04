defmodule EthereumJSONRPC.HTTP.Tesla do
  @moduledoc """
  Uses `Tesla.Mint` for `EthereumJSONRPC.HTTP`
  """

  alias EthereumJSONRPC.HTTP
  alias EthereumJSONRPC.HTTP.Helper
  alias Utils.HttpClient.TeslaHelper

  @behaviour HTTP

  @impl HTTP
  def json_rpc(url, json, headers, options) when is_binary(url) and is_list(options) do
    requests = Helper.record_requests(json)

    case Tesla.post(TeslaHelper.client(options), url, json, headers: headers, opts: TeslaHelper.request_opts(options)) do
      {:ok, %Tesla.Env{body: body, status: status_code, headers: headers}} ->
        body = Helper.try_unzip(body, headers)
        Helper.record_response_errors(requests, body)

        {:ok, %{body: body, status_code: status_code}}

      {:error, error} ->
        Helper.record_transport_errors(requests)

        {:error, error}
    end
  end

  def json_rpc(url, _json, _headers, _options) when is_nil(url), do: {:error, "URL is nil"}
end
