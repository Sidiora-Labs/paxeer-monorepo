defmodule Indexer.Fetcher.MultichainSearchDb.ExportQueueContract do
  import ExUnit.Assertions
  import ExUnit.Callbacks

  alias Explorer.MicroserviceInterfaces.MultichainSearch
  alias Explorer.Repo
  alias Plug.Conn

  def preserve_config(supervisor) do
    preserve_env([
      {:explorer, MultichainSearch},
      {:tesla, :adapter},
      {:indexer, supervisor}
    ])
  end

  def preserve_env(keys) do
    Enum.each(keys, fn {app, key} ->
      saved = Application.fetch_env(app, key)

      on_exit(fn ->
        case saved do
          {:ok, value} -> Application.put_env(app, key, value)
          :error -> Application.delete_env(app, key)
        end

        assert Application.fetch_env(app, key) == saved
      end)
    end)
  end

  def empty_batch(chain_id, api_key) do
    %{
      "api_key" => api_key,
      "chain_id" => chain_id,
      "addresses" => [],
      "block_ranges" => [],
      "hashes" => [],
      "address_coin_balances" => [],
      "address_token_balances" => []
    }
  end

  def expect_import(bypass, expected, status \\ 200) do
    owner = self()

    Bypass.expect_once(bypass, "POST", "/api/v1/import:batch", fn conn ->
      assert Conn.get_req_header(conn, "content-type") == ["application/json"]
      {:ok, body, conn} = Conn.read_body(conn)
      decoded = Jason.decode!(body)
      assert canonical_batch(decoded) == canonical_batch(expected)
      send(owner, {:import_received, status, decoded})

      response = if status == 200, do: %{"status" => "ok"}, else: %{"code" => 0, "message" => "Unavailable"}
      Conn.resp(conn, status, Jason.encode!(response))
    end)
  end

  # Lists in import batches are sets of independent records; compare every field,
  # including all empty collections, without imposing a database scan order.
  defp canonical_batch(batch) do
    Map.new(batch, fn
      {key, records} when is_list(records) -> {key, Enum.sort(records)}
      entry -> entry
    end)
  end

  def replay_after_restart(fetcher, schema, bypass, expected) do
    rows = Repo.all(schema)
    assert rows != []
    expect_import(bypass, expected, 503)
    assert {:retry, _} = fetcher.run(rows, nil)
    assert_receive {:import_received, 503, _}, 5_000
    persisted = Repo.all(schema)
    assert length(persisted) == length(rows)

    supervisor = Module.concat(fetcher, Supervisor)
    # Hold dispatch while the real init callback recovers the durable queue.
    pid = start_supervised!({supervisor, [[poll: false, flush_interval: :infinity, max_concurrency: 0]]})
    eventually(fn -> GenServer.call(fetcher, :state).bound_queue.size == length(rows) end)
    worker = Process.whereis(fetcher)
    assert is_pid(worker)
    assert :ok = stop_supervised(supervisor)
    refute Process.alive?(pid)
    refute Process.alive?(worker)
    assert Repo.all(schema) == persisted

    expect_import(bypass, expected)
    reopened = start_supervised!({supervisor, [[poll: false, flush_interval: :infinity, max_concurrency: 1]]})
    assert reopened != pid
    assert_receive {:import_received, 200, received}, 5_000
    assert canonical_batch(received) == canonical_batch(expected)
    eventually(fn -> Repo.aggregate(schema, :count) == 0 end)
    assert :ok = stop_supervised(supervisor)
  end

  def eventually(check, attempts \\ 100)
  def eventually(check, 0), do: assert(check.())

  def eventually(check, attempts) do
    if check.() do
      :ok
    else
      Process.sleep(20)
      eventually(check, attempts - 1)
    end
  end
end
