defmodule Explorer.ThirdPartyIntegrations.UniversalProxy do
  @moduledoc """
  A single point of entry for proxying third-party HTTP API endpoints
  """
  require Logger
  alias Explorer.{Helper, HttpClient}

  @recv_timeout 60_000

  @type api_request_params :: %{
          url: String.t() | nil,
          body: String.t(),
          headers: [{String.t(), String.t()}],
          method: atom() | nil,
          protocol: atom() | nil,
          platform_config: map() | nil,
          platform_id: String.t() | nil,
          config_error: String.t() | nil
        }

  @type api_key :: %{
          location: String.t(),
          param_name: String.t()
        }

  @type endpoint_param :: %{
          location: String.t(),
          param_name: String.t(),
          param_value: String.t()
        }

  @allowed_methods [:get, :post, :put, :patch, :delete]
  @allowed_protocols [:http, :https, :ws, :wss]
  @reserved_param_types ~w(address chain_id chain_id_dependent)
  @unexpected_error "Unexpected error when calling proxied endpoint"

  @cache_name :universal_proxy_config

  @doc """
  Makes an API request to a third-party service using the provided proxy parameters.

  ## Parameters

    - `proxy_params`: A map containing the parameters for the proxy request. It must include the "platform_id" key to identify the target platform.

  ## Configuration

  The function relies on a configuration map that defines platform-specific settings. The configuration should include:
    - `platforms`: A map where each key is a platform name and the value is a map with platform-specific settings.
      - `base_url`: The base URL for the platform's API.
      - `endpoints`: A map of endpoint configurations, including:
        - `base`: A map with `path` and optional `params`.
      - `api_key`: The API key for the platform, if required.

  ## Behavior

  1. Retrieves the platform-specific configuration based on the `platform_id` key in `proxy_params`.
  2. Constructs the request URL using the `base_url` and `base` endpoint path.
  3. Parses and applies any API key and endpoint parameters.
  4. Sends the HTTP request using `Explorer.HttpClient` with the specified method, URL, headers, and body.

  ## Returns

    - On success: A tuple `{decoded_body, status}` where:
      - `decoded_body`: The JSON-decoded response body.
      - `status`: The HTTP status code of the response.
    - On failure due to invalid configuration: A tuple `{"Invalid config: <error_message>", 422}`.
    - On unexpected errors: A tuple `{"Unexpected error when calling proxied endpoint", 500}`.

  ## Errors

  The function handles the following error scenarios:
    - Missing or invalid platform configuration.
    - Missing `base_url`, `base` endpoint, or `base` endpoint path.
    - Invalid or unsupported HTTP method.
    - Unexpected errors during the HTTP request.

  ## Notes

    - The function uses the `Explorer.HttpClient` with pre-configured adapter.
    - A timeout is applied to the request using the `@recv_timeout` module attribute.
  """
  @spec api_request(map()) ::
          {any(), integer()} | {:ok, pid()} | :ignore | {:error, {:already_started, pid()} | (reason :: term())}
  def api_request(proxy_params) do
    proxy_params
    |> parse_proxy_params()
    |> api_request_inner()
  end

  defp api_request_inner(%{config_error: error}) when is_binary(error), do: {"Invalid config: #{error}", 422}

  defp api_request_inner(%{
         url: url,
         body: body,
         headers: headers,
         method: method,
         protocol: protocol
       })
       when protocol in [:http, :https] and not is_nil(url) and not is_nil(method) do
    with {:invalid_config, false} <- {:invalid_config, is_nil(method)},
         {:ok, %{status_code: status, body: body}} <-
           HttpClient.request(method, url, headers, body, recv_timeout: @recv_timeout) do
      {Helper.decode_json(body), status}
    else
      err ->
        Logger.error(fn -> ["#{@unexpected_error}: ", inspect(err)] end)
        {@unexpected_error, 500}
    end
  end

  defp api_request_inner(%{
         url: url,
         method: method,
         platform_config: platform_config,
         platform_id: platform_id
       }) do
    error_message = construct_error_message(platform_config, platform_id, url, method)
    {"Invalid config: #{error_message}", 422}
  end

  defp construct_error_message(platform_config, platform_id, url, method) do
    cond do
      !platform_config ->
        "Platform '#{platform_id}' not found in config or 'platforms' property doesn't exist at all"

      is_nil(url) ->
        "'base_url' is not defined for platform_id '#{platform_id}' or 'base' endpoint is not defined or 'base' endpoint path is not defined"

      is_nil(method) ->
        "Invalid HTTP request method for platform '#{platform_id}'"

      true ->
        "Unexpected error"
    end
  end

  @doc """
  Parses the proxy parameters and constructs an API request.

  This function takes a map of proxy parameters and uses the configuration
  to determine the platform-specific settings, such as the base URL, API key,
  HTTP method, and endpoint parameters. It returns a map representing the API
  request.

  ## Parameters

    - `proxy_params` (map): A map containing the proxy parameters, including
      the `platform_id` which is used to identify the platform configuration.

  ## Returns

    - A map representing the API request with the following keys:
      - `:url` (String.t | nil): The constructed URL for the API request.
      - `:body` (String.t): The body of the API request (currently an empty string).
      - `:headers` (list): A list of headers for the API request (currently empty).
      - `:method` (atom): The HTTP method for the API request (e.g., `:get`, `:post`).
      - `:platform_config` (map | nil): The configuration for the specified platform.
      - `:platform_id` (String.t | nil): The ID of the platform.

  ## Notes

    - If the platform configuration or required fields are missing, the `:url`
      in the returned map will be `nil`.
    - The function delegates further processing of the API key and endpoint
      parameters to `parse_endpoint_api_key/3` and `parse_endpoint_params/3`.
  """
  @spec parse_proxy_params(map()) :: api_request_params()
  def parse_proxy_params(proxy_params) do
    case config() do
      {:error, error} ->
        %{
          url: nil,
          body: "",
          headers: [],
          method: nil,
          protocol: nil,
          platform_config: nil,
          platform_id: proxy_params["platform_id"],
          config_error: error
        }

      config ->
        parse_proxy_params(config, proxy_params)
    end
  end

  defp parse_proxy_params(config, proxy_params) do
    platform_id = proxy_params["platform_id"]

    platform_config = config["platforms"] && config["platforms"][platform_id]
    endpoint_params = (platform_config && platform_config["endpoints"]["base"]["params"]) || []
    endpoint_api_key = platform_config && platform_config["api_key"]

    method = parse_method(platform_config)
    protocol = parse_protocol(platform_config)

    raw_url =
      with true <- not is_nil(platform_config),
           true <- not is_nil(platform_config["base_url"]),
           endpoints = platform_config["endpoints"],
           true <- not is_nil(endpoints),
           base_endpoint = endpoints["base"],
           true <- not is_nil(base_endpoint),
           true <- not is_nil(base_endpoint["path"]) do
        platform_config["base_url"] <> base_endpoint["path"]
      else
        _ ->
          nil
      end

    %{
      url: raw_url,
      body: "",
      headers: [],
      method: method,
      protocol: protocol,
      platform_config: platform_config,
      platform_id: platform_id
    }
    |> parse_endpoint_api_key(endpoint_api_key, platform_id)
    |> parse_endpoint_params(endpoint_params, proxy_params)
  end

  @spec parse_method(map() | nil) :: atom() | nil
  defp parse_method(nil), do: nil

  defp parse_method(%{"endpoints" => %{"base" => %{"method" => method}}}) when is_binary(method) do
    Enum.find(@allowed_methods, &(Atom.to_string(&1) == String.downcase(method)))
  end

  defp parse_method(_platform_config), do: nil

  defp parse_protocol(%{"base_url" => base_url}) do
    case URI.new(base_url) do
      {:ok, %URI{scheme: scheme}} -> Enum.find(@allowed_protocols, &(Atom.to_string(&1) == scheme))
      _ -> nil
    end
  end

  defp parse_protocol(_platform_config), do: nil

  @spec parse_endpoint_api_key(api_request_params(), api_key() | nil, String.t() | nil) :: api_request_params()
  defp parse_endpoint_api_key(%{url: _url, headers: _headers} = map, nil, _platform_id), do: map

  defp parse_endpoint_api_key(
         %{url: url, headers: headers} = map,
         %{"location" => location, "param_name" => param_name} = endpoint_api_key,
         platform_id
       ) do
    api_key_env_name = "UNIVERSAL_PROXY_" <> String.upcase(platform_id) <> "_API_KEY"
    endpoint_api_key_value = System.get_env(api_key_env_name)

    case location do
      "path" ->
        if endpoint_api_key_value do
          Map.put(map, :url, String.replace(url, ":#{param_name}", endpoint_api_key_value))
        else
          map
        end

      "header" ->
        if endpoint_api_key["prefix"] do
          Map.put(map, :headers, [
            {param_name, "#{endpoint_api_key["prefix"]} #{endpoint_api_key_value}"} | headers
          ])
        else
          Map.put(map, :headers, [{param_name, endpoint_api_key_value} | headers])
        end

      "query" ->
        updated_uri =
          url
          |> URI.parse()
          |> URI.append_query("#{param_name}=#{endpoint_api_key_value}")
          |> URI.to_string()
          |> URI.encode()

        Map.put(map, :url, updated_uri)

      _ ->
        map
    end
  end

  defp parse_endpoint_api_key(map, _endpoint_api_key, _platform_id), do: map

  @spec parse_endpoint_params(api_request_params(), [endpoint_param()], map()) :: api_request_params()
  defp parse_endpoint_params(api_request_map, endpoint_params, proxy_params) do
    endpoint_params
    |> Enum.reduce(api_request_map, fn param, map ->
      case param do
        %{"location" => _location, "name" => _param_name, "value" => value} ->
          parse_param_location(map, param, value)

        %{"location" => _location, "type" => "chain_id_dependent", "mapping" => mapping} ->
          chain_id = proxy_params["chain_id"]

          # credo:disable-for-next-line
          if is_nil(chain_id) or is_nil(mapping[chain_id]) do
            map
          else
            parse_param_location(map, param, mapping[chain_id])
          end

        %{"location" => _location, "type" => param_type} ->
          parse_param_location(map, param, proxy_params[param_type])

        _ ->
          map
      end
    end)
  end

  @spec parse_param_location(api_request_params(), endpoint_param(), String.t()) :: api_request_params()
  defp parse_param_location(
         %{body: body, url: url, headers: headers} = api_request_map,
         %{"location" => location} = params,
         value
       )
       when not is_nil(url) and not is_nil(value) do
    case location do
      "path" ->
        if value do
          Map.put(api_request_map, :url, String.replace(url, ":#{param_name(params)}", value))
        else
          api_request_map
        end

      "query" ->
        query_param_name = param_name(params)

        updated_uri =
          url
          |> URI.parse()
          |> URI.append_query("#{query_param_name}=#{value}")
          |> URI.to_string()
          |> URI.encode()

        Map.put(api_request_map, :url, updated_uri)

      "body" ->
        body_param_name = param_name(params)
        Map.put(api_request_map, :body, body <> "#{body_param_name}=#{value}&")

      "header" ->
        header_name = param_name(params)
        Map.put(api_request_map, :headers, [{header_name, value} | headers])

      _ ->
        api_request_map
    end
  end

  defp parse_param_location(%{body: _body, url: _url, headers: _headers} = api_request_map, _params, _value),
    do: api_request_map

  defp param_name(params) do
    with {:empty_param_type, false} <- {:empty_param_type, params["type"] not in @reserved_param_types},
         {:empty_param_name, false} <- {:empty_param_name, is_nil(params["name"])} do
      params["name"]
    else
      {:empty_param_type, true} -> params["name"]
      {:empty_param_name, true} -> params["type"]
    end
  end

  defp config do
    case config_json() do
      config_string when is_binary(config_string) ->
        safe_parse_config_string(config_string)

      _ ->
        case :persistent_term.get(@cache_name, nil) do
          config_string when not is_nil(config_string) ->
            safe_parse_config_string(config_string)

          nil ->
            fetch_config_from_url()
        end
    end
  end

  defp safe_parse_config_string(config_string, update_cache? \\ false) do
    with {:ok, config} <- Jason.decode(config_string),
         :ok <- validate_config(config) do
      if update_cache?, do: :persistent_term.put(@cache_name, config_string)
      config
    else
      {:error, error} when is_binary(error) -> {:error, error}
      _ -> {:error, "Configuration must be valid JSON"}
    end
  end

  defp validate_config(%{"platforms" => platforms}) when is_map(platforms) do
    Enum.reduce_while(platforms, :ok, fn {id, platform}, :ok ->
      case validate_platform(id, platform) do
        :ok -> {:cont, :ok}
        error -> {:halt, error}
      end
    end)
  end

  defp validate_config(_), do: {:error, "Expected an object with a 'platforms' object"}

  defp validate_platform(id, platform) when is_map(platform) do
    with %{"base_url" => base_url, "endpoints" => %{"base" => %{"path" => path}} = endpoints}
         when is_binary(base_url) and is_binary(path) <- platform,
         true <- valid_endpoint_url?(base_url),
         :ok <- validate_method(id, platform),
         true <- Enum.all?(endpoints, fn {_, endpoint} -> valid_endpoint?(base_url, endpoint) end),
         true <- valid_api_key?(platform["api_key"]) do
      :ok
    else
      {:error, _} = error ->
        error

      false ->
        {:error, "Invalid endpoint URL, parameters or API key for platform '#{id}'"}

      _ ->
        {:error,
         "'base_url' is not defined for platform_id '#{id}' or 'base' endpoint is not defined or 'base' endpoint path is not defined"}
    end
  end

  defp validate_platform(id, _), do: {:error, "Expected an object for platform '#{id}'"}

  defp validate_method(id, platform) do
    if (parse_protocol(platform) in [:http, :https] or Map.has_key?(platform["endpoints"]["base"], "method")) and
         is_nil(parse_method(platform)) do
      {:error, "Invalid HTTP request method for platform '#{id}'"}
    else
      :ok
    end
  end

  defp valid_endpoint?(base_url, %{"path" => path} = endpoint) when is_binary(path) do
    (path == "" or (String.starts_with?(path, "/") and not String.starts_with?(path, "//"))) and
      valid_endpoint_url?(base_url <> path) and valid_params?(Map.get(endpoint, "params", [])) and
      (not Map.has_key?(endpoint, "method") or
         (is_binary(endpoint["method"]) and String.downcase(endpoint["method"]) in ~w(get post put patch delete)))
  end

  defp valid_endpoint?(_, _), do: false

  defp valid_endpoint_url?(url) do
    case URI.new(url) do
      {:ok, %URI{scheme: scheme, host: host, port: port, userinfo: nil, fragment: nil}}
      when scheme in ["http", "https", "ws", "wss"] and is_binary(host) and byte_size(host) > 0 and
             (is_nil(port) or (is_integer(port) and port > 0 and port <= 65_535)) ->
        not Regex.match?(~r/[\s\\]/u, url)

      _ ->
        false
    end
  end

  defp valid_params?(params) when is_list(params), do: Enum.all?(params, &valid_param?/1)
  defp valid_params?(_), do: false

  defp valid_param?(%{"location" => location} = param) when location in ~w(path query body header) do
    name = param["name"] || param["type"]

    is_binary(name) and name != "" and
      cond do
        Map.has_key?(param, "value") -> is_binary(param["value"])
        param["type"] == "chain_id_dependent" -> valid_mapping?(param["mapping"])
        true -> param["type"] in @reserved_param_types
      end
  end

  defp valid_param?(_), do: false

  defp valid_mapping?(mapping) when is_map(mapping), do: Enum.all?(mapping, fn {_, value} -> is_binary(value) end)
  defp valid_mapping?(_), do: false

  defp valid_api_key?(nil), do: true

  defp valid_api_key?(%{"location" => location, "param_name" => name} = key)
       when location in ~w(path query header) and is_binary(name) and byte_size(name) > 0 do
    not Map.has_key?(key, "prefix") or is_binary(key["prefix"])
  end

  defp valid_api_key?(_), do: false

  defp fetch_config_from_url do
    case HttpClient.get(config_url()) do
      {:ok, %{status_code: 200, body: config_string}} ->
        safe_parse_config_string(config_string, true)

      _ ->
        %{}
    end
  end

  defp config_json do
    Application.get_env(:explorer, __MODULE__)[:config_json]
  end

  defp config_url do
    Application.get_env(:explorer, __MODULE__)[:config_url]
  end

  @doc """
  Retrieves a list of all WebSocket proxies that have been configured for use within the application.

  Returns:
    - A list of tuples {platform_id, url}.

  ## Examples

      iex> Explorer.ThirdPartyIntegrations.UniversalProxy.websocket_proxies()
      [
        {"ws_1", "wss://proxy1.example.com"},
        {"ws_2", "wss://proxy2.example.com"}
      ]
  """
  @spec websocket_proxies() :: [{String.t(), String.t()}]
  def websocket_proxies do
    if Mix.env() == :test do
      []
    else
      config =
        case config() do
          {:error, _} -> %{}
          valid_config -> valid_config
        end

      if is_nil(config["platforms"]) do
        []
      else
        config["platforms"]
        |> Map.keys()
        |> Enum.reduce([], fn platform_id, acc ->
          proxy_params = parse_proxy_params(%{"platform_id" => platform_id})

          # credo:disable-for-next-line Credo.Check.Refactor.Nesting
          if valid_websocket_url?(proxy_params.url) do
            [{platform_id, proxy_params.url} | acc]
          else
            acc
          end
        end)
        |> Enum.reverse()
      end
    end
  end

  defp valid_websocket_url?(nil), do: false

  defp valid_websocket_url?(url) do
    case URI.parse(url) do
      %URI{scheme: scheme, host: host} when scheme in ["ws", "wss"] and not is_nil(host) -> true
      _ -> false
    end
  end
end
