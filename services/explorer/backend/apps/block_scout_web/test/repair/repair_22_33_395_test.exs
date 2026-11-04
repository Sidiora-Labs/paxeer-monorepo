defmodule BlockScoutWeb.Repair22_33_395Test do
  @moduledoc """
  Repair record 22.33.395 for audit 22.30.142, finding 34.
  Proving cases: "finding 34: restricted HTML and JSON do not resolve or visualize contracts",
  "finding 34: authorized HTML carries the key through the real JSON visualization request",
  and "finding 34: unrestricted contracts retain the production visualization transport".
  The real WebRouter, controller, restriction helper, persisted SimpleStorage
  chain fixture and Sol2uml HTTP adapter are exercised against a loopback peer.
  Query telemetry proves denial happens before contract resolution; the peer
  records every request and returns a committed SVG response shape.
  Baseline verification was not run: the explicit bounded gate order requires
  complete implementation before hf-build and hf-done and forbids manual verify.
  No baseline failure is claimed. Task/spec/gotcha records are outside writable
  scope; this module carries the repair record and hf-done records revision evidence.
  No out-of-path wiring is required for finding 34.
  """

  use BlockScoutWeb.ConnCase, async: false

  alias Explorer.Repo
  alias Explorer.Visualize.Sol2uml

  @endpoint BlockScoutWeb.Routers.WebRouter
  @key "repair+key&22=33?395"
  @svg ~s(<svg xmlns="http://www.w3.org/2000/svg"><text>SimpleStorage</text></svg>)

  defmodule VisualizerPeer do
    import Plug.Conn

    def init(state), do: state

    def call(conn, {owner, svg}) do
      {:ok, body, conn} = read_body(conn)

      send(
        owner,
        {:visualization, conn.method, conn.request_path, get_req_header(conn, "content-type"), Jason.decode!(body)}
      )

      conn
      |> put_resp_content_type("application/json")
      |> send_resp(200, Jason.encode!(%{"svg" => svg}))
    end
  end

  setup %{conn: conn} do
    for key <- [:addresses_blacklist, :addresses_blacklist_key, :http_client, Sol2uml] do
      previous = Application.fetch_env(:explorer, key)

      on_exit(fn ->
        case previous do
          {:ok, value} -> Application.put_env(:explorer, key, value)
          :error -> Application.delete_env(:explorer, key)
        end
      end)
    end

    ref = make_ref()

    start_supervised!(
      {Plug.Cowboy, scheme: :http, plug: {VisualizerPeer, {self(), @svg}}, options: [port: 0, ref: ref]}
    )

    Application.put_env(:explorer, Sol2uml, enabled: true, service_url: "http://127.0.0.1:#{:ranch.get_port(ref)}")
    Application.put_env(:explorer, :http_client, HTTPoison)
    Application.put_env(:explorer, :addresses_blacklist, "")
    Application.put_env(:explorer, :addresses_blacklist_key, @key)

    contract =
      insert(:smart_contract,
        language: :solidity,
        partially_verified: false,
        verified_via_sourcify: false,
        verified_via_eth_bytecode_db: false,
        verified_via_verifier_alliance: false
      )

    insert(:proxy_implementation, proxy_address_hash: contract.address_hash, address_hashes: [], names: [])

    additional_source =
      insert(:smart_contract_additional_source,
        address_hash: contract.address_hash,
        file_name: "SimpleStorageCopy.sol",
        contract_source_code: contract.contract_source_code
      )

    sources = %{
      "main.sol" => contract.contract_source_code,
      additional_source.file_name => additional_source.contract_source_code
    }

    conn =
      conn
      |> Plug.Test.init_test_session(%{})
      |> put_private(:phoenix_endpoint, BlockScoutWeb.Endpoint)
      |> Map.put(:secret_key_base, BlockScoutWeb.Endpoint.config(:secret_key_base))

    id = {__MODULE__, self()}
    prefix = Repo.config()[:telemetry_prefix] || [:explorer, :repo]
    :ok = :telemetry.attach(id, prefix ++ [:query], &__MODULE__.record_query/4, self())
    on_exit(fn -> :telemetry.detach(id) end)

    {:ok, conn: conn, contract: contract, sources: sources}
  end

  test "finding 34: restricted HTML and JSON do not resolve or visualize contracts", %{conn: conn, contract: contract} do
    hash = to_string(contract.address_hash)
    Application.put_env(:explorer, :addresses_blacklist, String.upcase(hash))

    for type <- [nil, "JSON"], key <- [nil, "wrong"] do
      params = %{"address" => hash}
      params = if type, do: Map.put(params, "type", type), else: params
      params = if key, do: Map.put(params, "key", key), else: params
      response = get(conn, "/visualize/sol2uml", params)

      assert html_response(response, 404)
      assert response.halted
      refute response.resp_body =~ contract.contract_source_code
      refute response.resp_body =~ "data-async-contract-svg"
      refute_received {:contract_query, _}
      refute_received {:visualization, _, _, _, _}
    end
  end

  test "finding 34: authorized HTML carries the key through the real JSON visualization request", %{
    conn: conn,
    contract: contract,
    sources: sources
  } do
    Application.put_env(:explorer, :addresses_blacklist, to_string(contract.address_hash))

    response = get(conn, "/visualize/sol2uml", %{"address" => to_string(contract.address_hash), "key" => @key})
    html = html_response(response, 200)
    assert_received {:contract_query, "addresses"}
    refute_received {:visualization, _, _, _, _}

    [path] = html |> Floki.parse_document!() |> Floki.attribute("[data-async-contract-svg]", "data-async-contract-svg")

    assert URI.decode_query(URI.parse(path).query) == %{
             "address" => to_string(contract.address_hash),
             "key" => @key,
             "type" => "JSON"
           }

    assert_visualization(get(conn, path), contract, sources)
  end

  test "finding 34: unrestricted contracts retain the production visualization transport", %{
    conn: conn,
    contract: contract,
    sources: sources
  } do
    response = get(conn, "/visualize/sol2uml", %{"address" => to_string(contract.address_hash), "type" => "JSON"})
    assert_visualization(response, contract, sources)
    assert_received {:contract_query, "addresses"}
  end

  test "disabled and malformed visualization requests do not query contracts or send sources", %{conn: conn} do
    for params <- [%{}, %{"address" => "invalid"}, %{"address" => "invalid", "type" => "JSON"}] do
      response = get(conn, "/visualize/sol2uml", params)

      if params["type"] == "JSON" do
        assert Phoenix.ConnTest.json_response(response, 200) == %{"error" => "contract not found or unverified"}
      else
        assert html_response(response, 404)
      end
    end

    Application.put_env(:explorer, Sol2uml, enabled: false)

    for type <- [nil, "JSON"] do
      response = get(conn, "/visualize/sol2uml", %{"address" => "invalid", "type" => type})
      assert html_response(response, 404)
    end

    refute_received {:contract_query, _}
    refute_received {:visualization, _, _, _, _}
  end

  def record_query(_event, _measurements, metadata, owner) do
    if self() == owner do
      for table <- ["addresses", "smart_contracts", "smart_contracts_additional_sources"] do
        if String.contains?(metadata.query, ~s(FROM "#{table}")), do: send(owner, {:contract_query, table})
      end
    end
  end

  defp assert_visualization(response, contract, sources) do
    assert Phoenix.ConnTest.json_response(response, 200) == %{
             "address" => to_string(contract.address_hash),
             "contract_svg" => @svg,
             "error" => nil
           }

    assert_received {:visualization, "POST", "/api/v1/solidity:visualize-contracts", ["application/json"],
                     %{"sources" => ^sources}}

    refute_received {:visualization, _, _, _, _}
  end
end
