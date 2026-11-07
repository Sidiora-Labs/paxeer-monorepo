defmodule Explorer.Repair.Repair2233497Test do
  use Explorer.DataCase, async: false

  alias Explorer.Chain.PaxeerX.{Capabilities, PriceFeed, PriceFeedScannedBlock, PriceUpdate}
  alias Explorer.Chain.Cache.Counters.LastFetchedCounter
  alias Explorer.PaxeerX.CosmosRest

  @recording Path.expand("../../lib/explorer/chain/paxeer_x/fixtures/capability_discovery.json", __DIR__)
  @success_counter "paxeer_x_price_feed_last_successful_scan"
  @height 37_715_713

  defmodule RecordedNode do
    @behaviour Plug

    def init(options), do: options

    def call(%{method: "POST"} = conn, {recording, state, observer}) do
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      request = Jason.decode!(body)
      settings = Agent.get(state, & &1)
      send(observer, {:rpc_request, request})

      if is_list(request) do
        responses =
          request
          |> Enum.reject(&(hd(&1["params"])["to"] in settings.omit))
          |> Enum.map(fn request ->
            entry = Enum.find(recording["calls"], &(&1["params"] == request["params"]))
            %{"method" => "eth_call"} = request
            response = Map.get(settings.overrides, hd(request["params"])["to"], entry["response"])
            Map.merge(response, %{"id" => request["id"], "jsonrpc" => "2.0"})
          end)
          |> Enum.reverse()

        reply(conn, 200, responses)
      else
        %{"method" => "eth_blockNumber", "params" => []} = request
        response = Map.merge(recording["head"]["response"], %{"jsonrpc" => "2.0", "id" => request["id"]})
        reply(conn, settings.head_status, response)
      end
    end

    def call(conn, {_recording, _state, observer}) do
      send(observer, {:chain_request, conn.request_path, conn.query_string})

      case conn.request_path do
        "/cosmos/tx/v1beta1/txs/block/37715713" -> reply(conn, 200, fixture("block_txs.json"))
        "/cosmos/tx/v1beta1/txs/block/37715714" -> reply(conn, 400, fixture("block_txs_empty.json"))
        "/block_results" -> reply(conn, 200, fixture("block_results.json"))
      end
    end

    defp fixture(name) do
      Path.expand("../support/fixture/paxeer_x/price_feed/#{name}", __DIR__)
      |> File.read!()
      |> Jason.decode!()
    end

    defp reply(conn, status, response) do
      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.send_resp(status, Jason.encode!(response))
    end
  end

  defmodule API do
    @behaviour Plug

    def init(options), do: options

    def call(%{request_path: "/api/v2/paxeer-x/capabilities"} = conn, _) do
      BlockScoutWeb.API.V2.PaxeerX.CapabilitiesController.capabilities(conn, %{})
    end
  end

  setup do
    keys = [
      {:explorer, :json_rpc_named_arguments},
      {:explorer, Capabilities},
      {:indexer, Indexer.Supervisor},
      {:indexer, Indexer.Fetcher.PaxeerX.PriceUpdates.Supervisor}
    ]

    previous = Enum.map(keys, fn {app, key} -> {app, key, Application.fetch_env(app, key)} end)

    on_exit(fn ->
      for {app, key, value} <- previous do
        case value do
          {:ok, config} -> Application.put_env(app, key, config)
          :error -> Application.delete_env(app, key)
        end
      end
    end)

    Application.put_env(:indexer, Indexer.Supervisor, enabled: false)
    Application.put_env(:indexer, Indexer.Fetcher.PaxeerX.PriceUpdates.Supervisor, disabled?: true)
    Application.put_env(:explorer, Capabilities, refresh_interval_seconds: 3600)
    {:ok, _} = Application.ensure_all_started(:plug_cowboy)

    recording = @recording |> File.read!() |> Jason.decode!()
    state = start_supervised!({Agent, fn -> %{overrides: %{}, omit: [], head_status: 200} end})
    {_id, _pid, url} = serve(RecordedNode, {recording, state, self()})

    Application.put_env(:explorer, :json_rpc_named_arguments,
      transport: EthereumJSONRPC.HTTP,
      transport_options: [
        http: EthereumJSONRPC.HTTP.HTTPoison,
        urls: [url],
        http_options: [timeout: 2000, recv_timeout: 2000]
      ]
    )

    %{recording: recording, state: state, node_url: url}
  end

  test "capability-transient-errors: per-call failures retain the whole snapshot through the API", ctx do
    start_capabilities()
    assert {:ok, snapshot} = Capabilities.refresh()
    assert Enum.all?(Capabilities.surfaces(), &Map.fetch!(snapshot, &1))
    {_id, _pid, api} = serve(API, [])
    published = capabilities(api)

    for surface <- Capabilities.surfaces(), error <- ctx.recording["transient_errors"] do
      address = Capabilities.address(surface)
      Agent.update(ctx.state, &%{&1 | overrides: %{address => %{"error" => error}}})
      assert {:error, _reason} = Capabilities.refresh()
      assert Capabilities.all() == snapshot
      assert Capabilities.live?(surface)
      assert capabilities(api) == published
    end

    Agent.update(ctx.state, &%{&1 | overrides: %{}, head_status: 503})
    assert {:error, _reason} = Capabilities.refresh()
    assert Capabilities.all() == snapshot
    assert capabilities(api) == published

    Agent.update(ctx.state, &%{&1 | head_status: 200})
    assert {:ok, recovered} = Capabilities.refresh()
    assert Enum.all?(Capabilities.surfaces(), &Map.fetch!(recovered, &1))
    assert %DateTime{} = recovered.checked_at
    assert_received {:rpc_request, %{"method" => "eth_blockNumber", "params" => []}}
    assert_received {:rpc_request, requests} when is_list(requests)
    assert length(requests) == 4
  end

  test "capability-transient-errors: only explicit unsupported calls publish absence", ctx do
    start_capabilities()
    assert {:ok, _snapshot} = Capabilities.refresh()
    {_id, _pid, api} = serve(API, [])

    for surface <- Capabilities.surfaces(), error <- ctx.recording["unsupported_errors"] do
      address = Capabilities.address(surface)
      Agent.update(ctx.state, &%{&1 | overrides: %{address => %{"error" => error}}})
      assert {:ok, snapshot} = Capabilities.refresh()
      refute Map.fetch!(snapshot, surface)
      refute Map.fetch!(capabilities(api), api_name(surface))

      for other <- Capabilities.surfaces() -- [surface] do
        assert Map.fetch!(snapshot, other)
      end
    end

    Agent.update(ctx.state, &%{&1 | overrides: %{}})
    assert {:ok, snapshot} = Capabilities.refresh()
    assert Enum.all?(Capabilities.surfaces(), &Map.fetch!(snapshot, &1))
  end

  test "capability-transient-errors: partial discovery cannot replace a successful snapshot", ctx do
    start_capabilities()
    assert {:ok, snapshot} = Capabilities.refresh()
    exchange = Capabilities.address(:exchange)
    bridge = Capabilities.address(:bridge)
    unsupported = hd(ctx.recording["unsupported_errors"])
    transient = hd(ctx.recording["transient_errors"])

    Agent.update(ctx.state, fn settings ->
      %{settings | overrides: %{exchange => %{"error" => unsupported}, bridge => %{"error" => transient}}}
    end)

    assert {:error, {:bridge, _}} = Capabilities.refresh()
    assert Capabilities.all() == snapshot

    for surface <- Capabilities.surfaces() do
      Agent.update(ctx.state, &%{&1 | overrides: %{}, omit: [Capabilities.address(surface)]})
      assert {:error, _} = Capabilities.refresh()
      assert Capabilities.all() == snapshot
    end
  end

  test "feed-compaction-health: API-only restart retains recent success after every marker is compacted", ctx do
    block = insert(:block, number: @height, consensus: true)
    assert {:ok, [update]} = CosmosRest.block_price_updates([ctx.node_url], [ctx.node_url], @height)
    assert {:ok, 1} = PriceFeed.replace_block_updates(block, [update])
    assert [%PriceUpdate{observation_count: 18}] = PriceFeed.block_updates(@height)
    assert PriceFeed.scanned_recently?(1000)

    empty = insert(:block, number: @height + 1, consensus: true)
    assert {:ok, []} = CosmosRest.block_price_updates([ctx.node_url], [ctx.node_url], empty.number)
    assert {:ok, 0} = PriceFeed.replace_block_updates(empty, [])
    assert is_nil(LastFetchedCounter.get(@success_counter, nullable: true))
    {id, old_pid, api} = serve(API, [])
    assert capabilities(api)["price_feed"] == true

    assert PriceFeed.compact_scanned_blocks(@height, 1) == @height
    assert PriceFeed.compact_scanned_blocks(@height, 1) == @height + 1
    assert Repo.aggregate(PriceFeedScannedBlock, :count) == 0
    assert Decimal.to_integer(LastFetchedCounter.get(@success_counter)) == empty.number
    assert capabilities(api)["price_feed"] == true

    assert :ok = stop_supervised(id)
    refute Process.alive?(old_pid)
    assert Application.get_env(:indexer, Indexer.Supervisor)[:enabled] == false
    {_id, reopened, api} = serve(API, [])
    assert reopened != old_pid
    assert capabilities(api)["price_feed"] == true
    assert PriceFeed.scanned_through(@height) == empty.number
    assert {:ok, []} = PriceFeed.stream_unscanned_block_numbers([], &[&1 | &2], @height)

    insert(:block, number: empty.number + 1000, consensus: true)
    assert capabilities(api)["price_feed"] == true
    insert(:block, number: empty.number + 1001, consensus: true)
    refute Map.has_key?(capabilities(api), "price_feed")
    refute PriceFeed.scanned_recently?(1000)

    assert_received {:chain_request, "/cosmos/tx/v1beta1/txs/block/37715713", _query}
    assert_received {:chain_request, "/block_results", "height=37715713"}
  end

  test "feed-compaction-health: an empty successful scan survives compaction without stored submissions", ctx do
    block = insert(:block, number: @height + 1, consensus: true)
    assert {:ok, []} = CosmosRest.block_price_updates([ctx.node_url], [ctx.node_url], block.number)
    assert {:ok, 0} = PriceFeed.replace_block_updates(block, [])
    assert PriceFeed.compact_scanned_blocks(block.number, 10) == block.number
    assert Repo.aggregate(PriceFeedScannedBlock, :count) == 0
    assert Repo.aggregate(PriceUpdate, :count) == 0
    {id, pid, api} = serve(API, [])
    assert capabilities(api)["price_feed"] == true
    assert :ok = stop_supervised(id)
    refute Process.alive?(pid)
    {_id, _pid, reopened_api} = serve(API, [])
    assert capabilities(reopened_api)["price_feed"] == true
  end

  test "feed-compaction-health: failed and unavailable watermark heights cannot manufacture recent success" do
    insert(:block, number: @height, consensus: true)
    assert :ok = PriceFeed.mark_unscannable(@height, "failed", "timeout")
    assert :ok = PriceFeed.mark_unscannable(@height + 1, "unavailable", "pruned")
    assert PriceFeed.compact_scanned_blocks(@height, 10) == @height + 1
    assert is_nil(LastFetchedCounter.get(@success_counter, nullable: true))
    refute PriceFeed.scanned_recently?(1000)
    {_id, _pid, api} = serve(API, [])
    refute Map.has_key?(capabilities(api), "price_feed")

    old = insert(:block, number: @height - 1001, consensus: true)
    assert {:ok, 0} = PriceFeed.replace_block_updates(old, [])
    assert PriceFeed.compact_scanned_blocks(old.number, 1) == old.number
    assert Decimal.to_integer(LastFetchedCounter.get(@success_counter)) == old.number
    assert PriceFeed.compact_scanned_blocks(@height, 10) == @height + 1
    assert Decimal.to_integer(LastFetchedCounter.get(@success_counter)) == old.number
    refute PriceFeed.scanned_recently?(1000)
    refute Map.has_key?(capabilities(api), "price_feed")
  end

  defp start_capabilities do
    pid = start_supervised!(Capabilities)
    assert_receive {:rpc_request, requests} when is_list(requests), 2000
    pid
  end

  defp serve(plug, options) do
    ref = {plug, make_ref()}
    child = Plug.Cowboy.child_spec(scheme: :http, plug: {plug, options}, options: [port: 0, ref: ref])
    id = {plug, ref}
    pid = start_supervised!(%{child | id: id})
    {id, pid, "http://127.0.0.1:#{:ranch.get_port(ref)}"}
  end

  defp capabilities(url) do
    {:ok, %HTTPoison.Response{status_code: 200, body: body}} =
      HTTPoison.get(url <> "/api/v2/paxeer-x/capabilities")

    Jason.decode!(body)
  end

  defp api_name(:unified_account), do: "addr"
  defp api_name(surface), do: Atom.to_string(surface)
end
