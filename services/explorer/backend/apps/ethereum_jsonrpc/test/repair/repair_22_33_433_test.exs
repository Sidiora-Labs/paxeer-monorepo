defmodule EthereumJSONRPC.Repair2233433Test do
  @moduledoc """
  Task record 22.33.433. Finding proofs are the tests prefixed with:
  decode-error-secrets, ipc-timeout-mismatch, ipc-dead-socket, ipc-error-shape,
  finalized-log-tag, cosmos-type-storage-bound, sync-error-becomes-synced,
  and receipt-hex-id-crash below. HTTP and IPC peers replay committed PaxeerX
  block/receipt fixtures and the node's false/progress and RPC error envelopes.

  Baseline was not run: the explicit ordered instructions prohibit verification
  before implementation and require verification only through hf-done. No
  pre-change failure is claimed; the runner records the qualified revision.

  Wiring request (domain 14): services/explorer/backend/config/runtime.exs should
  validate PAXEER_X_COSMOS_TRANSACTION_TYPE against PaxeerX's 127..2147483647
  bound when parsing; this task validates the resulting configuration at app boot
  and on every marker import without editing that out-of-scope reader.
  Wiring request (domain 7): services/chain/rpc FilterAPI integration should replay
  the finalized range transcript against its Go producer; this gate uses the
  recorded FilterLog corpus through the real explorer HTTP transport/import path.
  """

  use ExUnit.Case, async: false
  import ExUnit.CaptureLog
  @behaviour :cowboy_handler

  alias EthereumJSONRPC.{DecodeError, HTTP, IPC, Logs, PaxeerX, Receipts}
  alias EthereumJSONRPC.Block.ByTag

  @receipt_fixture Path.expand("../support/fixture/paxeer_x/cosmos_originated_transaction.json", __DIR__)
  @block_fixture Path.expand("../support/fixture/paxeer_x/new_head_full.json", __DIR__)
  @rpc_error ~s({"jsonrpc":"2.0","id":0,"error":{"code":-32602,"message":"Invalid params","data":{"tag":"bogus"}}})
  @sync_error ~s({"jsonrpc":"2.0","id":0,"error":{"code":-32000,"message":"consensus status unavailable"}})
  @sync_false ~s({"jsonrpc":"2.0","id":0,"result":false})
  @sync_progress ~s({"jsonrpc":"2.0","id":0,"result":{"startingBlock":"0x0","currentBlock":"0x1714efd","highestBlock":"0x1714efe"}})

  setup_all do
    {:ok, _} = Application.ensure_all_started(:cowboy)
    {:ok, _} = Application.ensure_all_started(:httpoison)
    :ok
  end

  setup do
    previous = Application.get_env(:ethereum_jsonrpc, PaxeerX)
    previous_http = Application.get_env(:ethereum_jsonrpc, HTTP)
    Application.put_env(:ethereum_jsonrpc, PaxeerX, cosmos_transaction_type: 127)

    on_exit(fn ->
      restore(PaxeerX, previous)
      restore(HTTP, previous_http)
    end)

    {:ok, receipt: @receipt_fixture |> File.read!() |> Jason.decode!() |> Map.fetch!("receipt")}
  end

  test "decode-error-secrets redacts credentials and bounds exception diagnostics" do
    headers = [
      {"Authorization", "Bearer authorization-secret"},
      {"pRoXy-AuThOrIzAtIoN", "Basic proxy-secret"},
      {"Cookie", "session=cookie-secret"},
      {"X-Api-Key", "api-secret"},
      {"Content-Type", "application/json"}
    ]

    message =
      DecodeError.exception(
        request: [
          url: "https://user-secret:password-secret@example.org/v2/path-secret?token=query-secret#fragment-secret",
          headers: headers,
          body: "eth_getLogs " <> String.duplicate("r", 2_000)
        ],
        response: [status_code: 503, body: "upstream unavailable " <> String.duplicate("z", 20_000)]
      )
      |> Exception.message()

    assert message =~ "status code: 503"
    assert message =~ "upstream unavailable"
    assert message =~ "eth_getLogs"
    assert message =~ "application/json"
    assert message =~ "https://example.org/[redacted]"
    assert message =~ "[truncated]"
    assert byte_size(message) < 5_000

    for secret <-
          ~w(authorization-secret proxy-secret cookie-secret api-secret user-secret password-secret path-secret query-secret fragment-secret) do
      refute message =~ secret
    end

    assert DecodeError.redact_url("not a URL with a password") == "[invalid endpoint]"
  end

  test "decode-error-secrets redacts the actual HTTP decode logger" do
    Application.put_env(:ethereum_jsonrpc, HTTP,
      headers: [{"Authorization", "Bearer logger-secret"}, {"X-Api-Key", "logger-api-secret"}],
      gzip_enabled?: false
    )

    args = http_endpoint("upstream logger-secret logger-api-secret " <> String.duplicate("x", 30_000), 503)
    options = args[:transport_options]
    url = hd(options[:urls]) <> "/logger-path-secret?key=logger-query-secret#logger-fragment-secret"
    args = Keyword.put(args, :transport_options, Keyword.put(options, :urls, [url]))

    log =
      capture_log(fn ->
        assert {:error, {:bad_response, ^url}} = EthereumJSONRPC.json_rpc(request(), args)
      end)

    assert log =~ "Failed to decode Ethereum JSONRPC response"
    assert log =~ "status code: 503"
    assert log =~ "upstream"
    assert byte_size(log) < 6_000

    for secret <- ~w(logger-secret logger-api-secret logger-path-secret logger-query-secret logger-fragment-secret) do
      refute log =~ secret
    end
  end

  test "ipc-error-shape matches HTTP success errors data and batch responses" do
    %{"block" => block} = @block_fixture |> File.read!() |> Jason.decode!()
    success = Jason.encode!(%{jsonrpc: "2.0", id: 0, result: block})
    null = ~s({"jsonrpc":"2.0","id":0,"result":null})

    for response <- [success, null, @rpc_error, @sync_error] do
      node = ipc_node(response)
      worker = ipc_worker(node)
      http_result = EthereumJSONRPC.json_rpc(request(), http_endpoint(response))
      ipc_result = IPC.request(worker, request())
      assert ipc_result == http_result
      if response == @rpc_error, do: assert(ByTag.number_from_result(ipc_result) == {:error, :invalid_tag})
    end

    response = "[" <> success <> "," <> String.replace(@rpc_error, ~s("id":0), ~s("id":1)) <> "]"
    payload = [request(), %{request() | id: 1}]
    node = ipc_node(response)
    worker = ipc_worker(node)
    assert {:ok, ipc_result} = IPC.request(worker, payload)
    assert {:ok, http_result} = EthereumJSONRPC.json_rpc(payload, http_endpoint(response))
    assert ipc_result == http_result
    assert [%{id: 0, result: ^block}, %{id: 1, error: %{code: -32602, data: %{"tag" => "bogus"}}}] = ipc_result
  end

  test "ipc-timeout-mismatch bounds socket fragments and discards late replies" do
    node = ipc_node(:hold)
    pool = ipc_pool(node)
    started = System.monotonic_time(:millisecond)
    task = Task.async(fn -> IPC.json_rpc(request(), pool: pool, timeout: 180) end)
    assert_receive {:ipc_request, connection, _}, 1_000
    send(connection, {:fragments, ["{", "\"jsonrpc\"", ":\"2.0\"", ",\"id\":0", ",\"result\":false}", "\n"], 60})
    assert Task.await(task, 1_000) == {:error, :timeout}
    assert System.monotonic_time(:millisecond) - started < 800

    Agent.update(node.store, &%{&1 | response: @sync_progress})
    assert {:ok, %{"currentBlock" => "0x1714efd"}} = IPC.json_rpc(request(), pool: pool, timeout: 1_000)
    assert_receive {:ipc_request, next_connection, _}, 1_000
    refute next_connection == connection
  end

  test "ipc-timeout-mismatch shares the deadline with pool acquisition and queued GenServer calls" do
    node = ipc_node(@sync_false)
    pool = ipc_pool(node)
    held = :poolboy.checkout(pool)
    started = System.monotonic_time(:millisecond)
    assert IPC.json_rpc(request(), pool: pool, timeout: 80) == {:error, :timeout}
    assert System.monotonic_time(:millisecond) - started < 600
    :poolboy.checkin(pool, held)
    assert {:ok, false} = IPC.json_rpc(request(), pool: pool, timeout: 1_000)
    assert_receive {:ipc_request, _, _}, 1_000

    :ok = :sys.suspend(held)
    task = Task.async(fn -> IPC.json_rpc(request(), pool: pool, timeout: 80) end)
    assert Task.await(task, 1_000) == {:error, :timeout}
    :ok = :sys.resume(held)
    # The queued expired request must never reach the node.
    assert {:ok, false} = IPC.json_rpc(request(), pool: pool, timeout: 1_000)
    assert_receive {:ipc_request, _, _}, 1_000
    refute_receive {:ipc_request, _, _}, 100
  end

  @tag timeout: 10_000
  test "ipc-timeout-mismatch permits a configured valid response beyond five seconds" do
    node = ipc_node(:hold)
    pool = ipc_pool(node)
    task = Task.async(fn -> IPC.json_rpc(request(), pool: pool, timeout: 7_000) end)
    assert_receive {:ipc_request, connection, _}, 1_000
    send(connection, {:fragments, [@sync_false <> "\n"], 5_150})
    assert Task.await(task, 8_000) == {:ok, false}
  end

  test "ipc-dead-socket recovers after node close restart and worker reopen" do
    node = ipc_node(@sync_false)
    worker = ipc_worker(node)
    assert {:ok, false} = IPC.request(worker, request())
    close_ipc_node(node)
    assert IPC.request(worker, request()) == {:error, :closed}
    assert :sys.get_state(worker)[:socket] == nil
    reopened = reopen_ipc_node(node)
    assert {:ok, false} = IPC.request(worker, request())
    assert Process.alive?(worker)
    GenServer.stop(worker)
    replacement = ipc_worker(reopened)
    assert {:ok, false} = IPC.request(replacement, request())
    assert :sys.get_state(replacement)[:path] == node.path
    refute replacement == worker
  end

  test "finalized-log-tag reaches HTTP in either range bound and imports FilterLogs", %{receipt: receipt} do
    logs = receipt["logs"]
    response = Jason.encode!([%{jsonrpc: "2.0", id: 0, result: logs}])
    args = http_endpoint(response)

    for {from, to} <- [{"finalized", "latest"}, {"earliest", "finalized"}, {"finalized", "finalized"}] do
      payload = Logs.request(0, %{from_block: from, to_block: to})
      assert {:ok, responses} = EthereumJSONRPC.json_rpc([payload], args)
      assert {:ok, [log]} = Logs.from_responses(responses)
      assert log.block_number == EthereumJSONRPC.quantity_to_integer(receipt["blockNumber"])
      assert log.transaction_hash == receipt["transactionHash"]
      assert_receive {:http_request, [%{"method" => "eth_getLogs", "params" => [filter]}]}, 1_000
      assert filter == %{"fromBlock" => from, "toBlock" => to}
    end

    for invalid <- ["bogus", "FINALIZED", "0xfinalized"] do
      assert_raise FunctionClauseError, fn -> Logs.request(0, %{from_block: invalid, to_block: "latest"}) end
      assert_raise FunctionClauseError, fn -> Logs.request(0, %{from_block: "earliest", to_block: invalid}) end
    end
  end

  test "cosmos-type-storage-bound validates boot configuration and receipt import", %{receipt: receipt} do
    Application.delete_env(:ethereum_jsonrpc, PaxeerX)
    assert PaxeerX.stored_cosmos_transaction_type() == 127

    for type <- [127, 128, 2_147_483_647] do
      Application.put_env(:ethereum_jsonrpc, PaxeerX, cosmos_transaction_type: type)
      args = receipt_endpoint(receipt)

      assert {:ok, %{receipts: [%{type: ^type}], logs: [_]}} =
               Receipts.fetch_by_block_numbers([receipt["blockNumber"]], args)
    end

    for invalid <- [-1, 0, 1, 2, 3, 4, 126, 2_147_483_648, 0xFFFF_FFFF, "127", nil] do
      Application.put_env(:ethereum_jsonrpc, PaxeerX, cosmos_transaction_type: invalid)

      assert_raise ArgumentError, ~r/cosmos_transaction_type must be an integer in 127\.\.2147483647/, fn ->
        EthereumJSONRPC.Application.start(:normal, [])
      end

      assert_raise ArgumentError, fn ->
        receipt |> EthereumJSONRPC.Receipt.to_elixir() |> EthereumJSONRPC.Receipt.elixir_to_params()
      end
    end

    Application.put_env(:ethereum_jsonrpc, PaxeerX, cosmos_transaction_type: 127)
    for type <- 0..4, do: assert(PaxeerX.normalize_transaction_type(type) == type)
  end

  test "sync-error-becomes-synced accepts only false as synced and preserves RPC and HTTP errors" do
    assert PaxeerX.fetch_syncing_status(http_endpoint(@sync_false)) == {:ok, :synced}
    assert PaxeerX.fetch_syncing_status(http_endpoint(@sync_progress)) == {:ok, {:syncing, 24_203_005, 24_203_006}}

    assert PaxeerX.fetch_syncing_status(http_endpoint(@sync_error)) ==
             {:error, %{code: -32000, message: "consensus status unavailable"}}

    assert PaxeerX.fetch_syncing_status(http_endpoint(@rpc_error)) ==
             {:error, %{code: -32602, message: "Invalid params", data: %{"tag" => "bogus"}}}

    args = http_endpoint("gateway unavailable", 502)
    assert {:error, {:bad_gateway, _}} = PaxeerX.fetch_syncing_status(args)

    assert {:error, {:unexpected_syncing_result, nil}} =
             PaxeerX.fetch_syncing_status(http_endpoint(~s({"jsonrpc":"2.0","id":0,"result":null})))

    listener_args = http_endpoint(@sync_false)
    url = hd(listener_args[:transport_options][:urls])
    # A real refused loopback connection is an operational transport failure.
    assert_receive {:unused_listener, listener, ^url}, 1_000
    :ok = :cowboy.stop_listener(listener)
    assert {:error, :econnrefused} = PaxeerX.fetch_syncing_status(listener_args)
  end

  test "receipt-hex-id-crash imports nonempty receipts and logs for either height encoding", %{receipt: receipt} do
    args = receipt_endpoint(receipt)
    height = EthereumJSONRPC.quantity_to_integer(receipt["blockNumber"])
    assert {:ok, integer_result} = Receipts.fetch_by_block_numbers([height], args)
    assert_receive {:http_request, [%{"id" => ^height, "params" => [quantity]}]}, 1_000
    assert quantity == receipt["blockNumber"]
    assert {:ok, hex_result} = Receipts.fetch_by_block_numbers([receipt["blockNumber"]], args)
    assert hex_result == integer_result
    assert_receive {:http_request, [%{"id" => ^height, "params" => [^quantity]}]}, 1_000
    assert [%{transaction_hash: hash, type: 127, status: :ok, gas_used: 120_000}] = hex_result.receipts
    assert hash == receipt["transactionHash"]
    assert [%{block_number: ^height, transaction_hash: ^hash}] = hex_result.logs
  end

  @impl :cowboy_handler
  def init(req, {owner, response, status} = state) do
    {:ok, body, req} = :cowboy_req.read_body(req)
    send(owner, {:http_request, Jason.decode!(body)})
    req = :cowboy_req.reply(status, %{"content-type" => "application/json"}, response, req)
    {:ok, req, state}
  end

  defp request, do: EthereumJSONRPC.request(%{id: 0, method: "eth_syncing", params: []})

  defp receipt_endpoint(receipt) do
    height = EthereumJSONRPC.quantity_to_integer(receipt["blockNumber"])
    http_endpoint(Jason.encode!([%{jsonrpc: "2.0", id: height, result: [receipt]}]))
  end

  defp http_endpoint(response, status \\ 200) do
    listener = make_ref()
    dispatch = :cowboy_router.compile([{:_, [{"/[...]", __MODULE__, {self(), response, status}}]}])

    {:ok, _} =
      :cowboy.start_clear(listener, %{socket_opts: [ip: {127, 0, 0, 1}, port: 0]}, %{env: %{dispatch: dispatch}})

    on_exit(fn -> :cowboy.stop_listener(listener) end)
    url = "http://127.0.0.1:#{:ranch.get_port(listener)}"
    send(self(), {:unused_listener, listener, url})

    [
      transport: HTTP,
      transport_options: [http: HTTP.HTTPoison, http_options: [recv_timeout: 1_000, timeout: 1_000], urls: [url]]
    ]
  end

  defp ipc_node(response) do
    path = Path.join(System.tmp_dir!(), "repair433-#{System.unique_integer([:positive])}.sock")
    store = start_supervised!({Agent, fn -> %{response: response, sockets: []} end}, id: make_ref())
    node = open_ipc_node(%{path: path, store: store, owner: self()})

    on_exit(fn ->
      close_ipc_node(node)
      File.rm(path)
    end)

    node
  end

  defp open_ipc_node(node) do
    File.rm(node.path)
    {:ok, listener} = :gen_tcp.listen(0, [:binary, active: false, ifaddr: {:local, node.path}])
    acceptor = spawn(fn -> accept_ipc(listener, node) end)
    Map.merge(node, %{listener: listener, acceptor: acceptor})
  end

  defp close_ipc_node(node) do
    :gen_tcp.close(node.listener)
    Process.exit(node.acceptor, :kill)
    if Process.alive?(node.store), do: Enum.each(Agent.get(node.store, & &1.sockets), &:gen_tcp.close/1)
    :ok
  end

  defp reopen_ipc_node(node) do
    reopened = open_ipc_node(node)
    on_exit(fn -> close_ipc_node(reopened) end)
    reopened
  end

  defp accept_ipc(listener, node) do
    case :gen_tcp.accept(listener) do
      {:ok, socket} ->
        connection =
          spawn(fn ->
            receive do
              :ready -> serve_ipc(socket, node)
            end
          end)

        :ok = :gen_tcp.controlling_process(socket, connection)
        Agent.update(node.store, &%{&1 | sockets: [socket | &1.sockets]})
        send(connection, :ready)
        accept_ipc(listener, node)

      {:error, :closed} ->
        :ok
    end
  end

  defp serve_ipc(socket, node) do
    case read_ipc(socket, <<>>) do
      {:ok, request} ->
        send(node.owner, {:ipc_request, self(), request})

        case Agent.get(node.store, & &1.response) do
          :hold ->
            receive do
              {:fragments, fragments, interval} ->
                Enum.each(fragments, fn fragment ->
                  Process.sleep(interval)
                  :gen_tcp.send(socket, fragment)
                end)
            after
              2_000 -> :gen_tcp.close(socket)
            end

          response ->
            :gen_tcp.send(socket, response <> "\n")
        end

        serve_ipc(socket, node)

      {:error, _} ->
        :gen_tcp.close(socket)
    end
  end

  defp read_ipc(socket, buffer) do
    with {:ok, bytes} <- :gen_tcp.recv(socket, 0, 2_000) do
      buffer = buffer <> bytes

      case Jason.decode(buffer) do
        {:ok, request} -> {:ok, request}
        {:error, _} -> read_ipc(socket, buffer)
      end
    end
  end

  defp ipc_worker(node), do: start_supervised!({IPC, path: node.path}, id: make_ref(), restart: :temporary)

  defp ipc_pool(node) do
    {:ok, pool} = :poolboy.start_link([worker_module: IPC, size: 1, max_overflow: 0], path: node.path)
    on_exit(fn -> if Process.alive?(pool), do: GenServer.stop(pool) end)
    pool
  end

  defp restore(key, nil), do: Application.delete_env(:ethereum_jsonrpc, key)
  defp restore(key, value), do: Application.put_env(:ethereum_jsonrpc, key, value)
end
