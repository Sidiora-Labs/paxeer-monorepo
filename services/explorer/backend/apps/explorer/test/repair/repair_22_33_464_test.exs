defmodule Explorer.Repair2233464Test do
  use ExUnit.Case, async: false

  alias EthereumJSONRPC.NFT
  alias Explorer.Chain.Token.Instance
  alias Explorer.Token.MetadataRetriever

  @contract "0x7e01cc81fcfdf6a71323900288a69e234c464f63"
  @cid "Qmd9pvThEwgjTBbEkNmmGFbcpJKw17fnRBAT4Td4rcog22"
  @chain_result "0x00000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000033697066732f516d6439707654684577676a544262456b4e6d6d47466263704a4b773137666e524241543454643472636f67323200000000000000000000000000"
  @metadata %{"name" => "Recorded NFT", "description" => "Token metadata", "attributes" => []}

  defmodule Transcript do
    @behaviour Plug

    def init(options), do: options

    def call(conn, {observer, routes, chain_result}) do
      send(observer, {:http_request, conn.request_path, conn.host, conn.req_headers})

      if conn.request_path == "/rpc" do
        {:ok, body, conn} = Plug.Conn.read_body(conn)
        requests = Jason.decode!(body)
        send(observer, {:chain_requests, requests})

        responses =
          Enum.map(requests, fn %{"id" => id} ->
            %{jsonrpc: "2.0", id: id, result: chain_result}
          end)

        conn
        |> Plug.Conn.put_resp_content_type("application/json")
        |> Plug.Conn.send_resp(200, Jason.encode!(responses))
      else
        {status, headers, body} = Map.fetch!(routes, conn.request_path)

        conn = Enum.reduce(headers, conn, fn {name, value}, acc -> Plug.Conn.put_resp_header(acc, name, value) end)
        Plug.Conn.send_resp(conn, status, body)
      end
    end
  end

  setup do
    entries = [
      {:explorer, :http_client},
      {:explorer, :json_rpc_named_arguments},
      {:tesla, :adapter},
      {:indexer, Indexer.Fetcher.TokenInstance.Helper},
      {:indexer, :ipfs}
    ]

    original = Enum.map(entries, fn {app, key} -> {app, key, Application.fetch_env(app, key)} end)
    cached_policy = :persistent_term.get(:parsed_cidr_list, :absent)

    Application.put_env(:explorer, :http_client, Explorer.HttpClient.Tesla)
    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)

    Application.put_env(:indexer, Indexer.Fetcher.TokenInstance.Helper,
      host_filtering_enabled?: true,
      cidr_blacklist: [],
      allowed_uri_protocols: ["http", "https"],
      base_uri_retry?: false
    )

    :persistent_term.erase(:parsed_cidr_list)
    {:ok, _} = Application.ensure_all_started(:plug_cowboy)

    on_exit(fn ->
      Enum.each(original, fn
        {app, key, {:ok, value}} -> Application.put_env(app, key, value)
        {app, key, :error} -> Application.delete_env(app, key)
      end)

      case cached_policy do
        :absent -> :persistent_term.erase(:parsed_cidr_list)
        value -> :persistent_term.put(:parsed_cidr_list, value)
      end
    end)

    :ok
  end

  test "metadata_redirect_filter_bypass is blocked from recorded chain URI through HTTP into the instance schema" do
    private = replay(%{"/private" => json_response(@metadata)})

    for status <- [301, 302, 303, 307, 308] do
      gateway = replay(%{("/ipfs/" <> @cid) => {status, [{"location", private <> "/private"}], ""}})
      configure_gateway(gateway)
      decoded = chain_uri(gateway)

      assert {:error, reason} = MetadataRetriever.fetch_json(decoded, 0, nil, false)
      assert reason == "blacklist"

      changeset =
        Instance.changeset(%Instance{}, %{
          token_id: 0,
          token_contract_address_hash: @contract,
          error: reason
        })

      assert changeset.valid?
      instance = Ecto.Changeset.apply_changes(changeset)
      assert instance.error == "blacklist"
      assert instance.metadata == nil
      assert instance.metadata_url == nil
      assert_receive {:http_request, "/ipfs/" <> @cid, "localhost", headers}
      assert {"x-gateway-key", "recorded-gateway-key"} in headers
      refute_receive {:http_request, "/private", _, _}
    end
  end

  test "resolved IPv4 and IPv6 connection addresses and configured CIDRs are filtered before connecting" do
    for host <- [
          "127.0.0.1",
          "169.254.169.254",
          "10.0.0.1",
          "[::1]",
          "[fc00::1]",
          "[fe80::1]",
          "[::ffff:127.0.0.1]",
          "[::ffff:169.254.169.254]"
        ] do
      assert {:error, "blacklist"} = MetadataRetriever.fetch_json({:ok, ["http://#{host}/metadata"]})
    end

    lookup = :inet_db.res_option(:lookup)
    host = ~c"metadata-2233464.invalid"
    address = {127, 23, 46, 4}
    :ok = :inet_db.add_host(address, [host])
    :ok = :inet_db.set_lookup([:file])

    try do
      assert {:ok, [^address]} = :inet.getaddrs(host, :inet)

      assert {:error, "blacklist"} =
               MetadataRetriever.fetch_json({:ok, ["http://metadata-2233464.invalid/metadata"]})
    after
      :inet_db.del_host(address)
      :inet_db.set_lookup(lookup)
    end

    config = Application.fetch_env!(:indexer, Indexer.Fetcher.TokenInstance.Helper)

    Application.put_env(
      :indexer,
      Indexer.Fetcher.TokenInstance.Helper,
      Keyword.put(config, :cidr_blacklist, ["8.8.8.0/24"])
    )

    :persistent_term.erase(:parsed_cidr_list)
    assert {:error, "blacklist"} = MetadataRetriever.fetch_json({:ok, ["http://8.8.8.8/metadata"]})
  end

  test "trusted gateway response uses the pinned HTTP adapter and preserves chain metadata" do
    gateway = replay(%{("/ipfs/" <> @cid) => json_response(@metadata)})
    configure_gateway(gateway)

    assert {:ok, %{metadata: @metadata}} = MetadataRetriever.fetch_json(chain_uri(gateway))
    assert_receive {:http_request, "/ipfs/" <> @cid, "localhost", headers}
    assert {"host", "localhost:#{URI.parse(gateway).port}"} in headers
    assert {"x-gateway-key", "recorded-gateway-key"} in headers
  end

  test "redirected gateway targets cannot retain the initial gateway exemption" do
    for location <- [
          "/private",
          "//127.0.0.1/private",
          "http://169.254.169.254/latest/meta-data/",
          "http://[::ffff:127.0.0.1]/private",
          "http://metadata-2233464.invalid/private"
        ] do
      gateway =
        replay(%{("/ipfs/" <> @cid) => {302, [{"location", location}], ""}, "/private" => json_response(@metadata)})

      configure_gateway(gateway)

      if String.contains?(location, "metadata-2233464.invalid") do
        lookup = :inet_db.res_option(:lookup)
        address = {127, 23, 46, 4}
        :ok = :inet_db.add_host(address, [~c"metadata-2233464.invalid"])
        :ok = :inet_db.set_lookup([:file])

        try do
          assert {:error, "blacklist"} = MetadataRetriever.fetch_json({:ok, ["ipfs/" <> @cid]})
        after
          :inet_db.del_host(address)
          :inet_db.set_lookup(lookup)
        end
      else
        assert {:error, "blacklist"} = MetadataRetriever.fetch_json({:ok, ["ipfs/" <> @cid]})
      end

      assert_receive {:http_request, "/ipfs/" <> @cid, _, _}
      refute_receive {:http_request, "/private", _, _}
    end
  end

  test "redirects reject missing locations and disallowed protocols" do
    for {headers, expected} <- [
          {[], ":invalid_redirect"},
          {[{"location", ""}], ":invalid_redirect"},
          {[{"location", "file:///etc/passwd"}], "empty_host"},
          {[{"location", "ftp://127.0.0.1/metadata"}], "disallowed_protocol"}
        ] do
      gateway = replay(%{("/ipfs/" <> @cid) => {302, headers, ""}})
      configure_gateway(gateway)
      assert {:error, ^expected} = MetadataRetriever.fetch_json({:ok, ["ipfs/" <> @cid]})
      assert_receive {:http_request, "/ipfs/" <> @cid, _, _}
    end
  end

  test "filtering disabled still follows relative redirects and enforces the five hop limit" do
    set_filtering(false)

    routes = %{
      "/start" => {301, [{"location", "/second"}], ""},
      "/second" => {308, [{"location", "metadata?token=1302"}], ""},
      "/metadata" => json_response(@metadata),
      "/loop" => {302, [{"location", "/loop"}], ""}
    }

    server = replay(routes)
    original = server <> "/start"
    assert {:ok_store_uri, %{metadata: @metadata}, ^original} = MetadataRetriever.fetch_json({:ok, [original]})

    for path <- ["/start", "/second", "/metadata"] do
      assert_receive {:http_request, ^path, _, _}
    end

    assert {:error, ":too_many_redirects"} = MetadataRetriever.fetch_json({:ok, [server <> "/loop"]})
    for _ <- 1..6, do: assert_receive({:http_request, "/loop", _, _})
    refute_receive {:http_request, "/loop", _, _}
  end

  test "cross origin redirects remove gateway credentials through the actual HTTP adapter" do
    set_filtering(false)
    destination = replay(%{"/metadata" => json_response(@metadata)})
    gateway = replay(%{("/ipfs/" <> @cid) => {307, [{"location", destination <> "/metadata"}], ""}})
    configure_gateway(gateway)

    assert {:ok, %{metadata: @metadata}} = MetadataRetriever.fetch_json({:ok, ["ipfs/" <> @cid]})
    assert_receive {:http_request, "/ipfs/" <> @cid, _, gateway_headers}
    assert {"x-gateway-key", "recorded-gateway-key"} in gateway_headers
    assert_receive {:http_request, "/metadata", _, destination_headers}
    refute List.keymember?(destination_headers, "x-gateway-key", 0)
  end

  defp replay(routes) do
    ref = {:repair_2233464, make_ref()}

    start_supervised!(
      Supervisor.child_spec(
        {Plug.Cowboy,
         scheme: :http,
         plug: {Transcript, {self(), routes, @chain_result}},
         options: [port: 0, ref: ref, ip: {127, 0, 0, 1}]},
        id: ref
      )
    )

    "http://localhost:#{:ranch.get_port(ref)}"
  end

  defp json_response(metadata), do: {200, [{"content-type", "application/json"}], Jason.encode!(metadata)}

  defp configure_gateway(gateway) do
    Application.put_env(:indexer, :ipfs,
      gateway_url: gateway <> "/ipfs",
      public_gateway_url: "https://ipfs.io/ipfs",
      gateway_url_param_location: :header,
      gateway_url_param_key: "x-gateway-key",
      gateway_url_param_value: "recorded-gateway-key"
    )
  end

  defp set_filtering(value) do
    config = Application.fetch_env!(:indexer, Indexer.Fetcher.TokenInstance.Helper)

    Application.put_env(
      :indexer,
      Indexer.Fetcher.TokenInstance.Helper,
      Keyword.put(config, :host_filtering_enabled?, value)
    )
  end

  defp chain_uri(gateway) do
    arguments = [
      transport: EthereumJSONRPC.HTTP,
      transport_options: [
        http: EthereumJSONRPC.HTTP.Tesla,
        http_options: [recv_timeout: 1_000],
        urls: [gateway <> "/rpc"]
      ]
    ]

    assert [{decoded, false}] = NFT.batch_metadata_url_request([{@contract, 0, "ERC-721"}], arguments)
    assert decoded == {:ok, ["ipfs/" <> @cid]}
    assert_receive {:http_request, "/rpc", _, _}

    assert_receive {:chain_requests,
                    [
                      %{
                        "jsonrpc" => "2.0",
                        "method" => "eth_call",
                        "params" => [%{"to" => @contract, "data" => data}, "latest"]
                      }
                    ]}

    assert data == "0xc87b56dd" <> String.duplicate("0", 64)
    decoded
  end
end
