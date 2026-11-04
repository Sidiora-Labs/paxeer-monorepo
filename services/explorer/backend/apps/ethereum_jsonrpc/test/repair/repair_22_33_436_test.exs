defmodule EthereumJSONRPC.Repair2233436Test.NodeHTTP do
  @moduledoc false

  def init(request, state) do
    {:ok, body, request} = :cowboy_req.read_body(request)
    payload = Jason.decode!(body)
    path = :cowboy_req.path(request)
    class = classify(payload)
    now = System.monotonic_time(:millisecond)

    Agent.update(state.counter, fn counts ->
      current = Map.update(counts.current, class, 1, &(&1 + 1))
      total = Enum.sum(Map.values(current))

      %{
        counts
        | current: current,
          maximum: max(counts.maximum, total),
          class_maximum: Map.update(counts.class_maximum, class, current[class], &max(&1, current[class])),
          calls: [{now, path, payload} | counts.calls]
      }
    end)

    send(state.owner, {:node_request, path, self(), payload})

    if String.starts_with?(path, "/hold") do
      receive do
        :release -> :ok
      after
        15_000 -> raise "loopback transcript was never released"
      end
    end

    body = payload |> response(state.block) |> Jason.encode!()

    {body, headers} =
      if String.ends_with?(path, "gzip") do
        {:zlib.gzip(body), %{"content-type" => "application/json", "content-encoding" => "gzip"}}
      else
        {body, %{"content-type" => "application/json"}}
      end

    Agent.update(state.counter, fn counts ->
      %{counts | current: Map.update!(counts.current, class, &(&1 - 1))}
    end)

    request = :cowboy_req.reply(200, headers, body, request)
    {:ok, request, state}
  end

  defp response(payload, block) when is_list(payload) do
    payload |> Enum.reverse() |> Enum.map(&response(&1, block))
  end

  defp response(%{"jsonrpc" => "2.0", "id" => id} = request, block) do
    Map.merge(%{"jsonrpc" => "2.0", "id" => id}, result(request, block))
  end

  defp result(%{"method" => "eth_getBlockByHash", "params" => [hash, _hydrated]}, %{"hash" => hash} = block),
    do: %{"result" => block}

  defp result(%{"method" => "eth_getBlockByNumber", "params" => [number, _hydrated]}, %{"number" => number} = block),
    do: %{"result" => block}

  defp result(%{"method" => "eth_getBalance", "params" => [_address, "0x1"]}, _block),
    do: %{"error" => %{"code" => -32000, "message" => "missing trie node"}}

  defp result(%{"method" => "eth_getBalance", "params" => [_address, "latest"]}, _block),
    do: %{"result" => "0x0"}

  defp result(%{"method" => "eth_blockNumber", "params" => []}, block), do: %{"result" => block["number"]}
  defp result(%{"method" => "eth_chainId", "params" => []}, _block), do: %{"result" => "0x7d"}
  defp result(%{"method" => "trace_block", "params" => [_number]}, _block), do: %{"result" => []}

  # Independent classification at the node measures real concurrent HTTP work.
  defp classify(batch) when is_list(batch), do: :batch
  defp classify(%{"method" => "trace_block"}), do: :trace
  defp classify(%{"method" => "eth_getBalance", "params" => [_, "0x1"]}), do: :archive
  defp classify(_request), do: :default
end

defmodule EthereumJSONRPC.Repair2233436Test do
  use ExUnit.Case, async: false

  alias EthereumJSONRPC.HTTP
  alias EthereumJSONRPC.HTTP.Throttle
  alias EthereumJSONRPC.Repair2233436Test.NodeHTTP
  alias Prometheus.Metric.{Counter, Gauge}

  @fixture Path.expand("../support/fixture/paxeer_x/new_head_full.json", __DIR__)
  @listener __MODULE__.Node
  @adapters [HTTP.HTTPoison, HTTP.Tesla]
  @methods ~w(eth_getBlockByHash eth_getBalance eth_chainId eth_blockNumber)
  @error %{code: -32000, message: "missing trie node"}

  setup do
    previous_throttle = Application.get_env(:ethereum_jsonrpc, Throttle)
    previous_http = Application.get_env(:ethereum_jsonrpc, HTTP)
    previous_adapter = Application.get_env(:tesla, :adapter)

    on_exit(fn ->
      :cowboy.stop_listener(@listener)
      restore_env(:ethereum_jsonrpc, Throttle, previous_throttle)
      restore_env(:ethereum_jsonrpc, HTTP, previous_http)
      restore_env(:tesla, :adapter, previous_adapter)
      restart_guard()
    end)

    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)
    Application.put_env(:ethereum_jsonrpc, HTTP, headers: [{"Content-Type", "application/json"}], gzip_enabled?: true)
    configure()

    %{"block" => block} = @fixture |> File.read!() |> Jason.decode!()

    counter =
      start_supervised!({Agent, fn -> %{current: %{}, maximum: 0, class_maximum: %{}, calls: []} end})

    fixture = %{owner: self(), block: block, counter: counter}
    dispatch = :cowboy_router.compile([{:_, [{"/[...]", NodeHTTP, fixture}]}])
    {:ok, _} = :cowboy.start_clear(@listener, [ip: {127, 0, 0, 1}, port: 0], %{env: %{dispatch: dispatch}})
    url = "http://127.0.0.1:#{:ranch.get_port(@listener)}"
    %{url: url, block: block, counter: counter}
  end

  for adapter <- @adapters do
    @adapter adapter
    adapter_name = adapter |> Module.split() |> List.last()

    test "#{adapter_name} attributes mixed reordered batch members and errors by ID", context do
      batch = mixed_batch(context.block)
      before = metrics()

      assert {:ok, responses} = rpc(batch, context.url <> "/plain", @adapter)
      assert Enum.map(responses, & &1.id) == [4, 3, 2, 1, 0]
      assert Enum.find(responses, &(&1.id == 0)).result == context.block
      assert Enum.find(responses, &(&1.id == 1)).error == @error
      assert Enum.find(responses, &(&1.id == 2)).result == "0x0"
      assert Enum.find(responses, &(&1.id == 3)).result == "0x7d"
      assert Enum.find(responses, &(&1.id == 4)).error == @error

      assert_delta(before, %{
        "eth_getBlockByHash" => {1, 0},
        "eth_getBalance" => {3, 2},
        "eth_chainId" => {1, 0},
        "eth_blockNumber" => {0, 0}
      })

      assert_receive {:node_request, "/plain", _, delivered}, 1_000
      assert delivered == batch |> Jason.encode!() |> Jason.decode!()
    end

    test "#{adapter_name} plain and gzip errors have identical accounting and consumer results", context do
      request = balance_request(context.block, 1, "0x1")
      before_plain = metrics()
      assert {:error, @error} = rpc(request, context.url <> "/plain", @adapter)
      plain = metric_delta(before_plain)

      before_gzip = metrics()
      assert {:error, @error} = rpc(request, context.url <> "/gzip", @adapter)
      assert metric_delta(before_gzip) == plain
      assert plain["eth_getBalance"] == {1, 1}

      batch = mixed_batch(context.block)
      before = metrics()
      assert {:ok, responses} = rpc(batch, context.url <> "/gzip", @adapter)
      assert Enum.count(responses, &Map.has_key?(&1, :error)) == 2
      assert metric_delta(before)["eth_getBalance"] == {3, 2}
      assert Enum.find(responses, &(&1.id == 0)).result == context.block
    end

    test "#{adapter_name} whole-transport failure charges every attempted batch member", context do
      # A closed loopback listener produces a real connection refusal in both HTTP clients.
      {:ok, socket} = :gen_tcp.listen(0, [:binary, ip: {127, 0, 0, 1}, active: false])
      {:ok, {_address, port}} = :inet.sockname(socket)
      :ok = :gen_tcp.close(socket)
      before = metrics()

      assert {:error, _reason} = rpc(mixed_batch(context.block), "http://127.0.0.1:#{port}/", @adapter)

      assert_delta(before, %{
        "eth_getBlockByHash" => {1, 1},
        "eth_getBalance" => {3, 3},
        "eth_chainId" => {1, 1},
        "eth_blockNumber" => {0, 0}
      })
    end

    test "#{adapter_name} recorded block travels through the import schema over gzip HTTP", context do
      arguments = [transport: HTTP, transport_options: options(context.url <> "/gzip", @adapter)]

      assert {:ok, %EthereumJSONRPC.Blocks{blocks_params: [block], errors: [], transactions_params: []}} =
               EthereumJSONRPC.fetch_blocks_by_hash([context.block["hash"]], arguments)

      assert block.hash == context.block["hash"]
      assert block.number == EthereumJSONRPC.quantity_to_integer(context.block["number"])
      assert block.state_root == context.block["stateRoot"]
    end

    test "#{adapter_name} admission and HTTP execution consume one absolute deadline", context do
      configure(request_timeout: 5_000, max_inflight: 1)
      url = context.url <> "/hold-deadline"
      holder = Task.async(fn -> rpc(block_request(context.block, 1), url, @adapter) end)
      first = node_request("/hold-deadline", 1)

      set_timeout(1_000)
      started = now()
      waiting = Task.async(fn -> rpc(block_request(context.block, 2), url, @adapter) end)
      await(fn -> Throttle.stats(url).queue_depth == 1 end)
      Process.sleep(400)
      send(first, :release)
      assert {:ok, _block} = Task.await(holder)
      second = node_request("/hold-deadline", 2)

      assert {:error, {:throttle, :deadline_exceeded}} = Task.await(waiting, 2_000)
      elapsed = now() - started
      assert elapsed >= 900
      assert elapsed < 1_300
      await(fn -> Throttle.stats(url).inflight_total == 0 end)
      assert Throttle.stats(url).queue_depth == 0
      assert Agent.get(context.counter, & &1.maximum) == 1
      send(second, :release)
    end
  end

  test "sustained node outage bounds admission, expires waiters and rejects overload", context do
    configure(max_inflight: 1, max_queue: 2, request_timeout: 10_000)
    url = context.url <> "/hold-outage"
    holder = Task.async(fn -> rpc(block_request(context.block, 0), url) end)
    connection = node_request("/hold-outage", 0)
    set_timeout(750)

    for wave <- 1..3 do
      waiting =
        for offset <- 1..2 do
          task = Task.async(fn -> admission_request(block_request(context.block, wave * 100 + offset), url) end)
          await(fn -> Throttle.stats(url).queue_depth == offset end)
          task
        end

      overloaded =
        1..20
        |> Task.async_stream(fn id -> admission_request(block_request(context.block, wave * 1_000 + id), url) end,
          max_concurrency: 20,
          timeout: 1_000
        )
        |> Enum.to_list()

      assert overloaded == List.duplicate({:ok, {:error, {:throttle, :overloaded}}}, 20)
      assert Throttle.stats(url).queue_depth == 2
      assert Task.await_many(waiting) == List.duplicate({:error, {:throttle, :deadline_exceeded}}, 2)
      await(fn -> Throttle.stats(url).queue_depth == 0 end)
      assert Throttle.stats(url).inflight_total == 1
      assert length(Agent.get(context.counter, & &1.calls)) == 1
    end

    assert Agent.get(context.counter, & &1.maximum) == 1
    send(connection, :release)
    assert {:ok, block} = Task.await(holder)
    assert block == context.block
    await(fn -> Throttle.stats(url).inflight_total == 0 end)
  end

  test "coalesced followers share the bounded queue and disappear at their deadlines", context do
    configure(max_inflight: 1, max_queue: 2, coalesce?: true, request_timeout: 5_000)
    url = context.url <> "/hold-coalesce"
    request = block_request(context.block, 0)
    holder = Task.async(fn -> rpc(request, url) end)
    connection = node_request("/hold-coalesce", 0)
    set_timeout(300)

    followers =
      for id <- 1..2 do
        follower = Task.async(fn -> admission_request(%{request | id: id}, url) end)
        await(fn -> Throttle.stats(url).queue_depth == id end)
        follower
      end

    assert {:error, {:throttle, :overloaded}} = admission_request(%{request | id: 3}, url)
    assert gauge(:json_rpc_throttle_queue_depth, :default) == 2
    assert Task.await_many(followers) == List.duplicate({:error, {:throttle, :deadline_exceeded}}, 2)
    await(fn -> gauge(:json_rpc_throttle_queue_depth, :default) == 0 end)
    assert length(Agent.get(context.counter, & &1.calls)) == 1
    send(connection, :release)
    assert {:ok, _block} = Task.await(holder)
  end

  test "queue and inflight gauges sum two busy endpoints and retain them when a third is idle", context do
    configure(max_inflight: 1)

    holders =
      for index <- 1..2 do
        path = "/hold-endpoint-#{index}"
        url = context.url <> path
        task = Task.async(fn -> rpc(block_request(context.block, index), url) end)
        {task, node_request(path, index), url}
      end

    queued =
      for {_task, _connection, url} <- holders do
        task = Task.async(fn -> rpc(block_request(context.block, 3), url) end)
        await(fn -> Throttle.stats(url).queue_depth == 1 end)
        task
      end

    assert gauge(:json_rpc_throttle_inflight, :default) == 2
    assert gauge(:json_rpc_throttle_queue_depth, :default) == 2
    assert {:ok, "0x7d"} = rpc(request(9, "eth_chainId", []), context.url <> "/idle")
    await(fn -> Throttle.stats(context.url <> "/idle").inflight_total == 0 end)
    assert gauge(:json_rpc_throttle_inflight, :default) == 2
    assert gauge(:json_rpc_throttle_queue_depth, :default) == 2

    for {task, connection, url} <- holders do
      send(connection, :release)
      assert {:ok, _block} = Task.await(task)
      path = URI.parse(url).path
      send(node_request(path, 3), :release)
    end

    assert Enum.all?(Task.await_many(queued), &match?({:ok, _}, &1))
    await(fn -> gauge(:json_rpc_throttle_inflight, :default) == 0 end)
    assert gauge(:json_rpc_throttle_queue_depth, :default) == 0

    for class <- [:archive, :trace] do
      assert gauge(:json_rpc_throttle_inflight, class) == 0
      assert gauge(:json_rpc_throttle_queue_depth, class) == 0
    end
  end

  for restart <- [:terminate_and_reopen, :crash], coalesce <- [false, true] do
    @restart restart
    @coalesce coalesce

    test "#{restart} recovers actual outstanding work and fences stale completion with coalescing #{coalesce}",
         context do
      configure(max_inflight: 1, coalesce?: @coalesce)
      url = context.url <> "/hold-restart"
      first = Task.async(fn -> rpc(block_request(context.block, 1), url) end)
      connection_a = node_request("/hold-restart", 1)

      old_guard = Process.whereis(Throttle)

      case @restart do
        :terminate_and_reopen ->
          :ok = Supervisor.terminate_child(EthereumJSONRPC.Supervisor, Throttle)
          assert {:error, {:throttle, :unavailable}} = rpc(block_request(context.block, 9), url)
          {:ok, _} = Supervisor.restart_child(EthereumJSONRPC.Supervisor, Throttle)

        :crash ->
          reference = Process.monitor(old_guard)
          Process.exit(old_guard, :kill)
          assert_receive {:DOWN, ^reference, :process, ^old_guard, :killed}, 1_000
          await(fn -> is_pid(Process.whereis(Throttle)) and Process.whereis(Throttle) != old_guard end)
      end

      assert Throttle.stats(url).inflight_total == 1
      guard = Process.whereis(Throttle)
      assert guard != old_guard
      # Trace captures the real worker's completion, which is later replayed unchanged.
      :erlang.trace(guard, true, [:receive])
      on_exit(fn -> if Process.alive?(guard), do: :erlang.trace(guard, false, [:receive]) end)

      # Different methods prevent identical-read coalescing while exercising both code paths.
      second = Task.async(fn -> rpc(request(2, "eth_blockNumber", []), url) end)
      await(fn -> Throttle.stats(url).queue_depth == 1 end)
      assert length(Agent.get(context.counter, & &1.calls)) == 1
      assert Agent.get(context.counter, & &1.maximum) == 1
      send(connection_a, :release)
      assert {:ok, _block} = Task.await(first)

      assert_receive {:trace, ^guard, :receive, {:"$gen_cast", completion}}, 1_000
      assert elem(completion, 0) in [:release, :complete]
      connection_b = node_request("/hold-restart", 2)
      third = Task.async(fn -> rpc(request(3, "eth_chainId", []), url) end)
      await(fn -> Throttle.stats(url).queue_depth == 1 end)

      GenServer.cast(Throttle, completion)
      assert %{inflight_total: 1, queue_depth: 1} = Throttle.stats(url)
      assert length(Agent.get(context.counter, & &1.calls)) == 2
      assert Agent.get(context.counter, & &1.maximum) == 1
      send(connection_b, :release)
      assert {:ok, number} = Task.await(second)
      assert number == context.block["number"]
      send(node_request("/hold-restart", 3), :release)
      assert {:ok, "0x7d"} = Task.await(third)
      await(fn -> Throttle.stats(url).inflight_total == 0 end)
      assert Throttle.stats(url).queue_depth == 0
      assert Agent.get(context.counter, & &1.maximum) == 1
    end
  end

  test "rate tokens survive guard reopening and retain per-member batch weight", context do
    configure(rps: 2)
    url = context.url <> "/rate"
    started = now()

    assert {:ok, [_, _]} = rpc([request(1, "eth_chainId", []), request(2, "eth_blockNumber", [])], url)
    restart_guard()
    assert {:ok, "0x7d"} = rpc(request(3, "eth_chainId", []), url)
    assert now() - started >= 450
    calls = Agent.get(context.counter, &Enum.reverse(&1.calls))
    assert [{first_at, "/rate", [_, _]}, {second_at, "/rate", %{"id" => 3}}] = calls
    assert second_at - first_at >= 450
  end

  test "a recovered leader still serves identical callers through one real HTTP request", context do
    configure(max_inflight: 1, coalesce?: true)
    url = context.url <> "/hold-recovered-leader"
    payload = block_request(context.block, 1)
    leader = Task.async(fn -> rpc(payload, url) end)
    connection = node_request("/hold-recovered-leader", 1)
    restart_guard()
    follower = Task.async(fn -> rpc(%{payload | id: 2}, url) end)
    await(fn -> Throttle.stats(url).queue_depth == 1 end)
    assert Throttle.stats(url).inflight_total == 1
    assert length(Agent.get(context.counter, & &1.calls)) == 1
    send(connection, :release)
    assert Task.await_many([leader, follower]) == [{:ok, context.block}, {:ok, context.block}]
    await(fn -> Throttle.stats(url).inflight_total == 0 end)
    assert Throttle.stats(url).queue_depth == 0
    assert Agent.get(context.counter, & &1.maximum) == 1
  end

  test "work completed while the guard is stopped does not occupy a recovered slot", context do
    configure(max_inflight: 1)
    url = context.url <> "/hold-completed"
    holder = Task.async(fn -> rpc(block_request(context.block, 1), url) end)
    connection = node_request("/hold-completed", 1)
    :ok = Supervisor.terminate_child(EthereumJSONRPC.Supervisor, Throttle)
    send(connection, :release)
    assert {:ok, block} = Task.await(holder)
    assert block == context.block
    {:ok, _} = Supervisor.restart_child(EthereumJSONRPC.Supervisor, Throttle)
    assert Throttle.stats(url).inflight_total == 0
    next = Task.async(fn -> rpc(block_request(context.block, 2), url) end)
    send(node_request("/hold-completed", 2), :release)
    assert {:ok, ^block} = Task.await(next)
    assert Agent.get(context.counter, & &1.maximum) == 1
  end

  test "global, trace and archive caps remain independent and allow ordinary work to progress", context do
    configure(max_inflight: 3, trace_max_inflight: 1, archive_max_inflight: 1)
    url = context.url <> "/hold-classes"

    payloads = [
      request(1, "trace_block", [context.block["number"]]),
      balance_request(context.block, 2, "0x1"),
      request(3, "eth_chainId", [])
    ]

    holders =
      for payload <- payloads do
        task = Task.async(fn -> rpc(payload, url) end)
        {task, node_request("/hold-classes", payload.id)}
      end

    waiting_payloads = [
      request(4, "trace_block", [context.block["number"]]),
      balance_request(context.block, 5, "0x1"),
      request(6, "eth_chainId", [])
    ]

    queued =
      for {payload, index} <- Enum.with_index(waiting_payloads, 1) do
        task = Task.async(fn -> rpc(payload, url) end)
        await(fn -> Throttle.stats(url).queue_depth == index end)
        task
      end

    assert %{inflight_total: 3, inflight: %{trace: 1, archive: 1, default: 1}} = Throttle.stats(url)
    {default_task, default_connection} = Enum.at(holders, 2)
    send(default_connection, :release)
    assert {:ok, "0x7d"} = Task.await(default_task)
    ordinary = node_request("/hold-classes", 6)
    assert Throttle.stats(url).queue_depth == 2
    send(ordinary, :release)

    for {{task, connection}, id} <- Enum.zip(Enum.take(holders, 2), [4, 5]) do
      send(connection, :release)
      assert Task.await(task) in [{:ok, []}, {:error, @error}]
      send(node_request("/hold-classes", id), :release)
    end

    assert Task.await_many(queued) == [{:ok, []}, {:error, @error}, {:ok, "0x7d"}]
    counts = Agent.get(context.counter, & &1)
    assert counts.maximum == 3
    assert counts.class_maximum == %{trace: 1, archive: 1, default: 1}
  end

  defp mixed_batch(block) do
    [
      block_request(block, 0),
      balance_request(block, 1, "0x1"),
      balance_request(block, 2, "latest"),
      request(3, "eth_chainId", []),
      balance_request(block, 4, "0x1")
    ]
  end

  defp block_request(block, id), do: EthereumJSONRPC.Block.ByHash.request(%{id: id, hash: block["hash"]})

  defp balance_request(block, id, quantity),
    do: EthereumJSONRPC.FetchedBalance.request(%{id: id, hash_data: block["miner"], block_quantity: quantity})

  defp request(id, method, params), do: EthereumJSONRPC.request(%{id: id, method: method, params: params})

  defp rpc(payload, url, adapter \\ HTTP.HTTPoison), do: HTTP.json_rpc(payload, options(url, adapter))

  defp admission_request(payload, url) do
    Throttle.request(url, payload, fn ->
      HTTP.HTTPoison.json_rpc(url, Jason.encode_to_iodata!(payload), [{"Content-Type", "application/json"}],
        timeout: 30_000,
        recv_timeout: 30_000,
        pool: false
      )
    end)
  end

  defp options(url, adapter) do
    [http: adapter, urls: [url], http_options: [timeout: 30_000, recv_timeout: 30_000, pool: false]]
  end

  defp configure(opts \\ []) do
    Application.put_env(
      :ethereum_jsonrpc,
      Throttle,
      Keyword.merge(
        [
          max_inflight: 8,
          trace_max_inflight: 8,
          archive_max_inflight: 8,
          rps: 100_000,
          max_queue: 128,
          request_timeout: 10_000,
          coalesce?: false
        ],
        opts
      )
    )

    restart_guard()
  end

  defp set_timeout(milliseconds) do
    config = Application.fetch_env!(:ethereum_jsonrpc, Throttle)
    Application.put_env(:ethereum_jsonrpc, Throttle, Keyword.put(config, :request_timeout, milliseconds))
  end

  defp restart_guard do
    :ok = Supervisor.terminate_child(EthereumJSONRPC.Supervisor, Throttle)
    {:ok, _pid} = Supervisor.restart_child(EthereumJSONRPC.Supervisor, Throttle)
    :ok
  end

  defp metrics do
    Map.new(@methods, fn method ->
      {method, {counter(:json_rpc_requests_count, method), counter(:json_rpc_requests_errors_count, method)}}
    end)
  end

  defp metric_delta(before) do
    Map.new(metrics(), fn {method, {requests, errors}} ->
      {old_requests, old_errors} = Map.fetch!(before, method)
      {method, {requests - old_requests, errors - old_errors}}
    end)
  end

  defp assert_delta(before, expected), do: assert(metric_delta(before) == expected)

  defp counter(name, method) do
    case Counter.value(name: name, labels: [method]) do
      :undefined -> 0
      value -> value
    end
  end

  defp gauge(name, class), do: Gauge.value(name: name, labels: [class])

  defp node_request(path, id) do
    assert_receive {:node_request, ^path, connection, %{"jsonrpc" => "2.0", "id" => ^id}}, 2_000
    connection
  end

  defp await(condition, attempts \\ 200)
  defp await(_condition, 0), do: flunk("HTTP node or throttle did not reach the required state")

  defp await(condition, attempts) do
    unless condition.() do
      Process.sleep(10)
      await(condition, attempts - 1)
    end
  end

  defp restore_env(app, key, nil), do: Application.delete_env(app, key)
  defp restore_env(app, key, value), do: Application.put_env(app, key, value)
  defp now, do: System.monotonic_time(:millisecond)
end
