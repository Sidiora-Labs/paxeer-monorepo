defmodule Explorer.Token.MetadataHTTPClient do
  @moduledoc false

  alias Explorer.MetadataURIValidator

  @ipv6_reserved Enum.map(~w(::/128 ::1/128 fc00::/7 fe80::/10 ff00::/8), &InetCidr.parse_cidr!/1)

  def get(uri, headers, validate? \\ true) do
    with {:ok, target, addresses} <- connection_target(uri),
         :ok <- if(validate?, do: validate_addresses(target, addresses), else: :ok),
         {:ok, connection} <- connect(target, hd(addresses)) do
      try do
        client =
          Tesla.client(
            [{Tesla.Middleware.Timeout, timeout: 30_000}],
            {Tesla.Adapter.Mint,
             conn: connection, original: "#{target.host}:#{target.port}", timeout: 30_000, mode: :passive}
          )

        case Tesla.get(client, uri, headers: headers) do
          {:ok, %{body: body, status: status, headers: response_headers}} ->
            {:ok, %{body: body, status_code: status, headers: response_headers}}

          error ->
            error
        end
      after
        Mint.HTTP.close(connection)
      end
    end
  end

  defp connection_target(uri) do
    with {:not_printable, true} <- {:not_printable, String.printable?(uri)},
         {:empty_host, %URI{host: host} = target} when host not in [nil, ""] <- {:empty_host, URI.parse(uri)},
         {:disallowed_protocol, true} <- {:disallowed_protocol, target.scheme in ["http", "https"]} do
      case :inet.parse_address(to_charlist(host)) do
        {:ok, address} ->
          {:ok, target, [address]}

        {:error, _} ->
          addresses =
            Enum.flat_map([:inet, :inet6], fn family ->
              case :inet.getaddrs(to_charlist(host), family) do
                {:ok, addresses} -> addresses
                {:error, _} -> []
              end
            end)

          if addresses == [], do: {:error, :nxdomain}, else: {:ok, target, Enum.uniq(addresses)}
      end
    else
      {reason, _} -> {:error, reason}
    end
  end

  defp validate_addresses(target, addresses) do
    Enum.reduce_while(addresses, :ok, fn address, :ok ->
      address = unmapped(address)
      uri = URI.to_string(%{target | host: to_string(:inet.ntoa(address)), userinfo: nil})

      result =
        if tuple_size(address) == 8 and Enum.any?(@ipv6_reserved, &InetCidr.contains?(&1, address)) do
          {:error, :blacklist}
        else
          MetadataURIValidator.validate_uri(uri)
        end

      case result do
        :ok -> {:cont, :ok}
        error -> {:halt, error}
      end
    end)
  end

  defp unmapped({0, 0, 0, 0, 0, 65_535, high, low}),
    do: {div(high, 256), rem(high, 256), div(low, 256), rem(low, 256)}

  defp unmapped(address), do: address

  defp connect(target, address) do
    scheme = if target.scheme == "https", do: :https, else: :http

    Mint.HTTP.connect(scheme, address, target.port,
      hostname: target.host,
      mode: :passive,
      protocols: [:http1],
      transport_opts: [timeout: 30_000]
    )
  end
end
