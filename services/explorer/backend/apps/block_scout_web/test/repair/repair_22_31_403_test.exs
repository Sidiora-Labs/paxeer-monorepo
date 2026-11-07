defmodule BlockScoutWeb.Repair.Repair2231403Test do
  @moduledoc """
  Task 22.31.403 repair record, audit 22.30.152, finding health-shape:
  the healthy, unhealthy, readiness, standalone and rollup cases below exercise
  production Phoenix endpoints over loopback HTTP and consume their published
  OpenAPI schemas without discarding fields. The block height and timestamp are
  recorded in the client's eth_blockscout_com/_api_health.json transcript.

  Wiring request (explorer client domain 14): update
  services/explorer/services/libs/blockscout-client/swaggers/overrides/002_health.yaml,
  regenerate the health models/status variants and add Phoenix-to-Rust HTTP
  coverage for healthy, unhealthy, readiness and standalone-media responses.
  These client paths and the task's spec record are outside this task's scope;
  this ExUnit gate cannot certify the Rust decoder or close that remaining work.

  No baseline verification ran: the explicit bounded gate order requires the
  entire implementation before hf-build and hf-done and forbids another run.
  The runner's evidence record supplies the verified candidate revision.
  """

  use BlockScoutWeb.ConnCase

  alias BlockScoutWeb.Schemas.API.Health
  alias Explorer.Chain.Cache.Counters.LastFetchedCounter
  alias Explorer.Chain.Health.{Helper, Monitor}
  alias Explorer.Repo

  @height 23_116_935
  @timestamp ~U[2025-08-11 09:32:59Z]

  setup do
    media = Application.fetch_env(:nft_media_handler, :standalone_media_worker?)
    chain = Application.fetch_env(:explorer, :chain_type)
    monitor_running = Process.whereis(Monitor) != nil
    if monitor_running, do: Supervisor.terminate_child(Explorer.Supervisor, Monitor)

    Application.put_env(:nft_media_handler, :standalone_media_worker?, false)
    Application.put_env(:explorer, :chain_type, :paxeer_x)
    Repo.delete_all(LastFetchedCounter)
    :ets.delete_all_objects(BlockScoutWeb.RateLimit.Hammer.ETS)

    on_exit(fn ->
      restore_env(:nft_media_handler, :standalone_media_worker?, media)
      restore_env(:explorer, :chain_type, chain)
      if monitor_running, do: Supervisor.restart_child(Explorer.Supervisor, Monitor)
    end)

    urls =
      for endpoint <- [BlockScoutWeb.Endpoint, BlockScoutWeb.HealthRouter] do
        ref = make_ref()

        start_supervised!(
          {Plug.Cowboy, scheme: :http, plug: endpoint, options: [ref: ref, ip: {127, 0, 0, 1}, port: 0]}
        )

        "http://127.0.0.1:#{:ranch.get_port(ref)}"
      end

    %{urls: urls, spec: BlockScoutWeb.Specs.Public.spec()}
  end

  test "health-shape: healthy progress survives Phoenix HTTP and schema consumption", %{urls: urls, spec: spec} do
    persist_progress(@height)

    for url <- urls do
      body = health_response(url, 200, spec)
      assert body["healthy"] == true
      assert body["error"] == nil

      assert body["metadata"]["blocks"] == %{
               "healthy" => true,
               "latest_block" => progress()
             }

      assert {:error, _} = OpenApiSpex.cast_value(body, Health.Standalone.schema(), spec)
    end
  end

  test "health-shape: unhealthy HTTP 500 retains node progress and both typed errors", %{urls: urls, spec: spec} do
    persist_progress(@height + 21)

    for url <- urls do
      body = health_response(url, 500, spec)
      blocks = body["metadata"]["blocks"]
      assert body["healthy"] == false
      assert blocks["healthy"] == false
      assert blocks["latest_block"] == put_in(progress(), ["node", "number"], to_string(@height + 21))
      assert %{"code" => 5001, "message" => message} = body["error"]
      assert message =~ "There are no new blocks in the DB for the last "
      assert message =~ "Check the healthiness of the JSON RPC archive node or the DB."
      assert blocks["error"] == body["error"]
      assert {:error, _} = OpenApiSpex.cast_value(body, response_schema(spec, "/health", 200), spec)
    end
  end

  test "health-shape: missing blocks retain empty progress and the no-blocks error", %{urls: urls, spec: spec} do
    for url <- urls do
      body = health_response(url, 500, spec)
      error = %{"code" => 5002, "message" => "There are no blocks in the DB."}
      assert body["healthy"] == false
      assert body["error"] == error
      assert body["metadata"]["blocks"]["error"] == error

      assert body["metadata"]["blocks"]["latest_block"] == %{
               "db" => %{"number" => "", "timestamp" => ""},
               "cache" => %{"number" => "", "timestamp" => ""},
               "node" => %{"number" => ""}
             }
    end
  end

  test "health-shape: readiness and liveness consume plain text rather than health JSON", %{urls: urls, spec: spec} do
    block = insert(:block, number: @height, timestamp: @timestamp, consensus: true) |> Repo.reload!()
    assert Helper.last_db_block() == {block.number, block.timestamp}

    for url <- urls, action <- ["readiness", "liveness"] do
      response = HTTPoison.get!(url <> "/api/health/" <> action)
      assert response.status_code == 200
      assert content_type(response) == "text/plain; charset=utf-8"
      assert response.body == "OK"
      assert {:ok, "OK"} = OpenApiSpex.cast_value(response.body, response_schema(spec, "/health/" <> action, 200), spec)
    end
  end

  test "health-shape: standalone media has an explicit disjoint success variant", %{urls: urls, spec: spec} do
    Application.put_env(:nft_media_handler, :standalone_media_worker?, true)

    for url <- urls do
      body = health_response(url, 200, spec)
      assert body == %{"healthy" => true, "data" => %{}}
      assert {:ok, _} = OpenApiSpex.cast_value(body, Health.Standalone.schema(), spec)
      assert {:error, _} = OpenApiSpex.cast_value(body, Health.Healthy.schema(), spec)
      response = HTTPoison.get!(url <> "/api/health/readiness")
      assert response.status_code == 200
      assert response.body == "OK"
    end
  end

  test "health-shape: rollup batch progress and errors also survive consumption", %{urls: urls, spec: spec} do
    persist_progress(@height)
    Application.put_env(:explorer, :chain_type, :optimism)

    for url <- urls do
      body = health_response(url, 200, spec)
      assert body["metadata"]["blocks"]["latest_block"] == progress()

      assert body["metadata"]["batches"] == %{
               "healthy" => false,
               "latest_batch" => %{"db" => %{"number" => "", "timestamp" => "", "average_batch_time" => ""}},
               "error" => %{"code" => 5002, "message" => "There are no batches in the DB."}
             }
    end
  end

  test "health-shape: malformed progress or errors cannot silently disappear", %{urls: [url | _], spec: spec} do
    persist_progress(@height + 21)
    body = health_response(url, 500, spec)
    schema = response_schema(spec, "/health", 500)

    for invalid <- [
          update_in(body, ["metadata", "blocks", "latest_block"], &Map.delete(&1, "node")),
          put_in(body, ["metadata", "blocks", "latest_block", "node", "number"], 23_116_956),
          put_in(body, ["metadata", "blocks", "error", "code"], "invalid"),
          update_in(body, ["metadata", "blocks"], &Map.delete(&1, "error")),
          put_in(body, ["error", "message"], nil),
          put_in(body, ["metadata", "latest_block"], progress())
        ] do
      assert {:error, _} = OpenApiSpex.cast_value(invalid, schema, spec)
    end
  end

  defp health_response(url, status, spec) do
    response = HTTPoison.get!(url <> "/api/health")
    assert response.status_code == status
    assert content_type(response) == "application/json; charset=utf-8"
    body = Jason.decode!(response.body)
    schema = response_schema(spec, "/health", status)
    OpenApiSpex.TestAssertions.assert_raw_schema(body, schema, spec)
    assert {:ok, consumed} = OpenApiSpex.cast_value(body, schema, spec)
    assert consumed |> Jason.encode!() |> Jason.decode!() == body
    body
  end

  defp response_schema(spec, path, status) do
    response = spec.paths |> Map.fetch!(path) |> Map.fetch!(:get) |> Map.fetch!(:responses) |> Map.fetch!(status)
    response.content |> Map.values() |> hd() |> Map.fetch!(:schema)
  end

  defp content_type(response) do
    {_, value} = Enum.find(response.headers, fn {key, _} -> String.downcase(key) == "content-type" end)
    value
  end

  defp persist_progress(node) do
    for {key, value} <- [
          {"health_latest_block_number_from_db", @height},
          {"health_latest_block_timestamp_from_db", DateTime.to_unix(@timestamp)},
          {"health_latest_block_number_from_cache", @height},
          {"health_latest_block_timestamp_from_cache", DateTime.to_unix(@timestamp)},
          {"health_latest_block_number_from_node", node}
        ] do
      assert {:ok, _} = LastFetchedCounter.upsert(%{counter_type: key, value: value})
    end
  end

  defp progress do
    %{
      "db" => %{"number" => to_string(@height), "timestamp" => to_string(@timestamp)},
      "cache" => %{"number" => to_string(@height), "timestamp" => to_string(@timestamp)},
      "node" => %{"number" => to_string(@height)}
    }
  end

  defp restore_env(app, key, {:ok, value}), do: Application.put_env(app, key, value)
  defp restore_env(app, key, :error), do: Application.delete_env(app, key)
end
