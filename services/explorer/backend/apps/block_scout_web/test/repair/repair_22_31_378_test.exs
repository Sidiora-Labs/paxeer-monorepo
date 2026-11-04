defmodule BlockScoutWeb.Repair2231378Test do
  use BlockScoutWeb.ConnCase, async: false

  alias BlockScoutWeb.Routers.ApiRouter
  alias Explorer.Chain.PaxeerX.PriceFeed
  alias Explorer.PaxeerX.CosmosRest

  @fixtures Path.expand("../../../explorer/test/support/fixture/paxeer_x/price_feed", __DIR__)
  @record Path.expand("../../lib/block_scout_web/routers/hosted_route_admission.json", __DIR__)

  setup do
    record = @record |> File.read!() |> Jason.decode!()
    body = fixture("block_txs.json")
    results = fixture("block_results.json")
    raw = body["block"]["data"]["txs"]

    {:ok, submissions} = CosmosRest.decode_block_txs(%{"txs" => body["txs"], "raw" => raw})
    {:ok, [update]} = CosmosRest.decode_price_updates(submissions, results["txs_results"], length(raw))
    header = body["block"]["header"]
    {:ok, timestamp, 0} = DateTime.from_iso8601(header["time"])

    block =
      insert(:block,
        number: String.to_integer(header["height"]),
        hash: Base.decode64!(body["block_id"]["hash"]),
        timestamp: timestamp
      )

    assert {:ok, 1} = PriceFeed.replace_block_updates(block, [update])
    assert record["hosted_backend_prefix"] == "/explorer/backend"
    assert length(record["rest_routes"]) == 7
    assert length(record["page_routes"]) == 3
    %{record: record, update: update, block: block}
  end

  test "recorded submissions traverse the decoder, database, router and feed consumers", %{
    conn: conn,
    record: record,
    update: update,
    block: block
  } do
    for route <- record["rest_routes"] do
      path =
        route["path"]
        |> String.replace("{hash}", update.hash)
        |> String.replace("{market_id}", "0")
        |> String.replace_prefix("/api", "")

      info = Phoenix.Router.route_info(ApiRouter, "GET", path, "localhost")
      assert info.plug == Module.concat([route["controller"]])
      assert Atom.to_string(info.plug_opts) == route["action"]
    end

    detail = json_response(get(conn, "/api/v2/paxeer-x/feed/updates/" <> update.hash), 200)
    assert_recorded_update(detail, update)
    assert detail["block_number"] == block.number
    assert detail["block_hash"] == to_string(block.hash)

    query = URI.encode_query(%{"feeder" => update.feeder_hash, "market_id" => "0"})
    page = json_response(get(conn, "/api/v2/paxeer-x/feed/updates?" <> query), 200)
    assert page["items"] == [detail]
    assert page["next_page_params"] == nil

    empty = json_response(get(conn, "/api/v2/paxeer-x/feed/updates?market_id=999999"), 200)
    assert empty == %{"items" => [], "next_page_params" => nil}

    market = json_response(get(conn, "/api/v2/paxeer-x/feed/markets/0/updates"), 200)
    assert [observation] = market["items"]
    assert observation["update_hash"] == update.hash
    assert observation["index_price"] == "82830900000000000000000"
    assert observation["sequence"] == 63_784

    assert %{"items" => [], "chain_head" => height} =
             json_response(get(conn, "/api/v2/paxeer-x/feed/markets"), 200)

    assert height == block.number

    assert %{"message" => "Not found"} =
             json_response(get(conn, "/api/v2/paxeer-x/feed/updates/0x" <> String.duplicate("ab", 32)), 404)

    assert %{"errors" => [_ | _]} = json_response(get(conn, "/api/v2/paxeer-x/feed/updates/invalid"), 422)
  end

  test "advanced-filter routes preserve list, CSV and method-query responses", %{conn: conn} do
    response = json_response(get(conn, "/api/v2/advanced-filters?transaction_types=COIN_TRANSFER"), 200)
    assert response["items"] == []
    assert response["next_page_params"] == nil

    methods = json_response(get(conn, "/api/v2/advanced-filters/methods?q=%74ransfer"), 200)
    assert methods == [%{"method_id" => "0xa9059cbb", "name" => "transfer"}]

    csv = get(conn, "/api/v2/advanced-filters/csv?transaction_types=COIN_TRANSFER")
    assert csv.status == 200
    assert get_resp_header(csv, "content-type") == ["application/csv; charset=utf-8"]
    assert get_resp_header(csv, "content-disposition") == ["attachment;"]
    assert csv.resp_body =~ "TxHash,Type,MethodId,UtcTimestamp"
    assert csv.resp_body =~ "BlockNumber"
  end

  test "hosted REST admission preserves list producer response", %{record: record, update: update} do
    route = Enum.find(record["rest_routes"], &(&1["action"] == "list"))
    assert_hosted_route(record, update, route)
  end

  test "hosted REST admission preserves list_csv producer response", %{record: record, update: update} do
    route = Enum.find(record["rest_routes"], &(&1["action"] == "list_csv"))
    assert_hosted_route(record, update, route)
  end

  test "hosted REST admission preserves list_methods producer response", %{record: record, update: update} do
    route = Enum.find(record["rest_routes"], &(&1["action"] == "list_methods"))
    assert_hosted_route(record, update, route)
  end

  test "hosted REST admission preserves markets producer response", %{record: record, update: update} do
    route = Enum.find(record["rest_routes"], &(&1["action"] == "markets"))
    assert_hosted_route(record, update, route)
  end

  test "hosted REST admission preserves market_updates producer response", %{record: record, update: update} do
    route = Enum.find(record["rest_routes"], &(&1["action"] == "market_updates"))
    assert_hosted_route(record, update, route)
  end

  test "hosted REST admission preserves updates producer response", %{record: record, update: update} do
    route = Enum.find(record["rest_routes"], &(&1["action"] == "updates"))
    assert_hosted_route(record, update, route)
  end

  test "hosted REST admission preserves update producer response", %{record: record, update: update} do
    route = Enum.find(record["rest_routes"], &(&1["action"] == "update"))
    assert_hosted_route(record, update, route)
  end

  @tag timeout: 60_000
  test "hosted REST admission preserves backend errors", %{record: record} do
    for {path, status} <- [
          {"/api/v2/paxeer-x/feed/updates/invalid", 422},
          {"/api/v2/paxeer-x/feed/markets/invalid/updates", 422},
          {"/api/v2/paxeer-x/feed/updates/0x" <> String.duplicate("ab", 32), 404}
        ] do
      direct = local_request(path)
      hosted = admitted_request(record, record["hosted_backend_prefix"] <> path)
      assert direct.status == status
      assert hosted.status == status
      assert content_type(hosted) == content_type(direct)
      assert Jason.decode!(hosted.body) == Jason.decode!(direct.body)
    end
  end

  test "hosted feed page index reaches the frontend", %{record: record, update: update} do
    assert_hosted_page(record, update, "/explorer/feed")
  end

  test "hosted feed page market reaches the frontend", %{record: record, update: update} do
    assert_hosted_page(record, update, "/explorer/feed/market/[id]")
  end

  test "hosted feed page update reaches the frontend", %{record: record, update: update} do
    assert_hosted_page(record, update, "/explorer/feed/update/[hash]")
  end

  test "admission contract enumerates exactly the seven backend interfaces", %{record: record} do
    assert Enum.sort(Enum.map(record["rest_routes"], &{&1["method"], &1["path"]})) ==
             Enum.sort([
               {"GET", "/api/v2/advanced-filters"},
               {"GET", "/api/v2/advanced-filters/csv"},
               {"GET", "/api/v2/advanced-filters/methods"},
               {"GET", "/api/v2/paxeer-x/feed/markets"},
               {"GET", "/api/v2/paxeer-x/feed/markets/{market_id}/updates"},
               {"GET", "/api/v2/paxeer-x/feed/updates"},
               {"GET", "/api/v2/paxeer-x/feed/updates/{hash}"}
             ])

    assert Enum.sort(Enum.map(record["page_routes"], & &1["pattern"])) == Enum.sort(record["page_patterns"])

    assert Enum.sort(record["page_patterns"]) ==
             Enum.sort(["/explorer/feed", "/explorer/feed/market/[id]", "/explorer/feed/update/[hash]"])

    assert record["finding"] == "hosted-route-admission"
  end

  test "optional hosted probe performs no request without explicit origins", %{record: record, update: update} do
    assert probe_hosted(record, update, nil) == {:skipped, :url_not_supplied}
  end

  defp assert_hosted_route(record, update, route, transport \\ :local) do
    path =
      route["path"]
      |> String.replace("{hash}", update.hash)
      |> String.replace("{market_id}", "0")

    query =
      case route["action"] do
        "list_methods" ->
          "?q=%74ransfer"

        "list" ->
          "?transaction_types=COIN_TRANSFER&from_period=2020-01-01T00%3A00%3A00Z&to_period=2020-01-01T00%3A00%3A01Z"

        "list_csv" ->
          "?transaction_types=COIN_TRANSFER&from_period=2020-01-01T00%3A00%3A00Z&to_period=2020-01-01T00%3A00%3A01Z"

        "updates" ->
          "?market_id=0&block_number=37715714&index=0"

        "market_updates" ->
          "?block_number=37715714&index=0"

        _ ->
          ""
      end

    {direct, hosted} =
      case transport do
        :local ->
          {local_request(path <> query), admitted_request(record, record["hosted_backend_prefix"] <> path <> query)}

        %{backend_origin: backend, gateway_origin: gateway} ->
          {probe_request(backend <> path <> query),
           probe_request(gateway <> record["hosted_backend_prefix"] <> path <> query)}
      end

    assert direct.status == 200, "backend #{path}: #{inspect(direct)}"
    assert hosted.status == 200, "hosted #{path}: #{inspect(hosted)}"
    assert content_type(hosted) == content_type(direct)

    case route["action"] do
      "list_csv" ->
        assert hosted.body == direct.body
        assert hosted.body =~ "TxHash,Type,MethodId,UtcTimestamp"
        assert header(hosted, "content-disposition") == header(direct, "content-disposition")

      "list_methods" ->
        assert Jason.decode!(hosted.body) == [%{"method_id" => "0xa9059cbb", "name" => "transfer"}]
        assert Jason.decode!(hosted.body) == Jason.decode!(direct.body)

      "update" ->
        assert_recorded_update(Jason.decode!(hosted.body), update)
        assert_recorded_update(Jason.decode!(direct.body), update)

      "markets" ->
        direct_markets = Jason.decode!(direct.body)
        hosted_markets = Jason.decode!(hosted.body)
        assert is_integer(hosted_markets["chain_head"])
        assert hosted_markets["chain_head"] >= 37_715_713
        fields = ["market_id", "symbol", "chainflow_symbol"]

        assert Enum.map(hosted_markets["items"], &Map.take(&1, fields)) ==
                 Enum.map(direct_markets["items"], &Map.take(&1, fields))

      _ ->
        assert Jason.decode!(hosted.body) == Jason.decode!(direct.body)
    end
  end

  defp assert_hosted_page(record, update, pattern) do
    assert pattern in record["page_patterns"]
    assert page = Enum.find(record["page_routes"], &(&1["pattern"] == pattern))
    assert page["methods"] == ["GET", "HEAD"]

    {source, component, resource, resource_path} =
      case pattern do
        "/explorer/feed" ->
          {"apps/explorer/pages/feed/index.tsx", "PaxeerXFeed", "paxeer_x_feed_markets",
           "/api/v2/paxeer-x/feed/markets"}

        "/explorer/feed/market/[id]" ->
          {"apps/explorer/pages/feed/market/[id].tsx", "PaxeerXFeedMarket", "paxeer_x_feed_market_updates",
           "/api/v2/paxeer-x/feed/markets/:id/updates"}

        "/explorer/feed/update/[hash]" ->
          {"apps/explorer/pages/feed/update/[hash].tsx", "PaxeerXFeedUpdate", "paxeer_x_feed_update",
           "/api/v2/paxeer-x/feed/updates/:hash"}
      end

    assert page["source"] == source
    assert page["component"] == component
    assert page["resource"] == resource
    assert page["resource_path"] == resource_path
    assert page["recorded_page_source"] =~ String.replace_prefix(pattern, "/explorer", "")
    assert page["recorded_page_source"] =~ "ui/pages/" <> component
    assert page["recorded_page_source"] =~ "export default Page"
    assert page["recorded_page_source"] =~ "getServerSideProps"
    assert record["recorded_resource_source"]["content"] =~ resource <> ":"
    assert record["recorded_resource_source"]["content"] =~ "path: '" <> resource_path <> "'"
    route = Enum.find(record["rest_routes"], &(&1["action"] == page["backend_action"]))
    assert_hosted_route(record, update, route)
  end

  # Explicit opt-in diagnostic, never called by the qualification tests or configured from the environment.
  # With no URL it performs no HTTP requests; local contract assertions still run in every test above.
  def probe_hosted(_record, _update, nil), do: {:skipped, :url_not_supplied}

  def probe_hosted(record, update, %{gateway_origin: _, backend_origin: _, frontend_origin: frontend} = origins) do
    for route <- record["rest_routes"], do: assert_hosted_route(record, update, route, origins)

    for pattern <- record["page_patterns"] do
      path = pattern |> String.replace("[id]", "0") |> String.replace("[hash]", update.hash)
      direct = probe_request(frontend <> path)
      hosted = probe_request(origins.gateway_origin <> path)
      assert direct.status == 200, "frontend #{path}: #{inspect(direct)}"
      assert hosted.status == 200, "hosted page #{path}: #{inspect(hosted)}"
      assert content_type(hosted) == "text/html"
      assert content_type(direct) == "text/html"
      assert hosted.body =~ "__NEXT_DATA__"
      assert direct.body =~ "__NEXT_DATA__"
    end

    :ok
  end

  defp admitted_request(record, hosted_path) do
    prefix = record["hosted_backend_prefix"]
    assert String.starts_with?(hosted_path, prefix <> "/")
    path = String.replace_prefix(hosted_path, prefix, "")
    target = URI.parse(path)

    matches =
      Enum.filter(record["rest_routes"], fn route ->
        template = String.split(route["path"], "/")
        segments = String.split(target.path, "/")

        route["method"] == "GET" and length(template) == length(segments) and
          Enum.all?(Enum.zip(template, segments), fn {expected, actual} ->
            expected == actual or (expected in ["{hash}", "{market_id}"] and actual != "")
          end)
      end)

    assert [route] = matches
    info = Phoenix.Router.route_info(ApiRouter, "GET", String.replace_prefix(target.path, "/api", ""), "localhost")
    assert info.plug == Module.concat([route["controller"]])
    assert Atom.to_string(info.plug_opts) == route["action"]
    response = local_request(path)
    assert response.query_string == (target.query || "")
    response
  end

  defp local_request(path) do
    conn = get(build_conn(), path)
    %{status: conn.status, headers: conn.resp_headers, body: conn.resp_body, query_string: conn.query_string}
  end

  defp assert_recorded_update(item, update) do
    assert item["hash"] == update.hash
    assert item["type"] == 127
    assert item["method"] == "submitPrices"
    assert item["price_update"]["observation_count"] == 18
    assert length(item["price_update"]["observations"]) == 18
    assert first = hd(item["price_update"]["observations"])
    assert first["market_id"] == 0
    assert first["index_price"] == "82830900000000000000000"
    assert first["sequence"] == 63_784
  end

  defp probe_request(url) do
    assert {:ok, _} = Application.ensure_all_started(:inets)
    assert {:ok, _} = Application.ensure_all_started(:ssl)

    options = [
      timeout: 8_000,
      connect_timeout: 3_000,
      autoredirect: false,
      ssl: [
        verify: :verify_peer,
        cacerts: :public_key.cacerts_get(),
        customize_hostname_check: [match_fun: :public_key.pkix_verify_hostname_match_fun(:https)]
      ]
    ]

    assert {:ok, {{_, status, _}, headers, body}} =
             :httpc.request(:get, {String.to_charlist(url), []}, options, body_format: :binary)

    %{status: status, headers: headers, body: body}
  end

  defp header(response, name) do
    Enum.find_value(response.headers, fn {key, value} ->
      if String.downcase(to_string(key)) == name, do: to_string(value)
    end)
  end

  defp content_type(response), do: response |> header("content-type") |> String.split(";") |> hd()
  defp fixture(name), do: @fixtures |> Path.join(name) |> File.read!() |> Jason.decode!()
end
