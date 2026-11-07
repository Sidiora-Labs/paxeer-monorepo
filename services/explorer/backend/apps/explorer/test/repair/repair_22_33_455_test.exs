defmodule Explorer.Repair2233455Test do
  @moduledoc """
  Task 22.33.455 repair record (audit 22.30.148).

  account-abstraction-invalid-json: account abstraction controller contains malformed upstream bodies.
  bens-test-route-divergence: BENS preloader uses the production route in both protocol modes.
  tag-metadata-decode: metadata preloader reports and omits invalid nested annotations;
    metadata search validates the same nested annotation boundary.
  export-mixed-error-reduction: mixed HTTP failure and task exit recover in either order after restart.
  export-invalid-success-body: malformed successful export is durable and replays after restart.

  Baseline verification was not run: the task's gate order requires the whole implementation
  before its single verification. No baseline failure or baseline pass is claimed.
  No out-of-path wiring change is required; this in-scope module carries the repair record.
  """

  use Explorer.DataCase

  import ExUnit.CaptureLog, only: [capture_log: 1, with_log: 1]

  alias Explorer.Chain.{Address, Hash, Wei}
  alias Explorer.Chain.Cache.ChainId
  alias Explorer.Chain.MultichainSearchDb.{BalancesExportQueue, MainExportQueue}
  alias Explorer.MicroserviceInterfaces.{AccountAbstraction, BENS, Metadata, MultichainSearch}
  alias Indexer.Fetcher.MultichainSearchDb.BalancesExportQueue, as: BalancesExporter
  alias Indexer.Fetcher.MultichainSearchDb.MainExportQueue, as: MainExporter

  # Addresses and transaction from the committed account-abstraction controller transcript.
  @address "0xc9f2b9af320d92a7c9cd67bbbF0f3055f81b6d31"
  @other_address "0x92613ef2df071255d6ccd554651e7e445e939a32"
  @token "0x2cc0c7981d846b9f2a16276556f6e8cb52bfB633"
  @transaction "0xe0f96d979a2610a89a642744124e7caa087fe4771092286b763ea0d963a73fca"

  defmodule Listener do
    @behaviour Plug
    import Plug.Conn

    def init(options), do: options

    def call(conn, {owner, responses}) do
      {:ok, body, conn} = read_body(conn)
      conn = fetch_query_params(conn)
      payload = if body == "", do: nil, else: Jason.decode!(body)

      send(
        owner,
        {:wire, conn.method, conn.request_path, conn.query_params, get_req_header(conn, "content-type"), payload}
      )

      responder = Agent.get(responses, & &1)
      {status, response} = responder.(conn, payload)
      conn |> put_resp_header("content-type", "application/json") |> resp(status, response)
    end
  end

  setup_all do
    # Load the HTTP transport before the request deadline starts. On a cold VM,
    # lazy beam loading otherwise competes with the first bounded request.
    for app <- [:tesla, :mint, :plug, :plug_cowboy, :cowboy, :cowlib, :ranch] do
      {:ok, _} = Application.ensure_all_started(app)
      :ok = :code.ensure_modules_loaded(Application.spec(app, :modules))
    end

    :ok
  end

  setup do
    keys = [
      {:explorer, AccountAbstraction},
      {:explorer, BENS},
      {:explorer, Metadata},
      {:explorer, MultichainSearch},
      {:tesla, :adapter},
      {:block_scout_web, :chain_id},
      {:indexer, MainExporter.Supervisor},
      {:indexer, BalancesExporter.Supervisor}
    ]

    saved = Enum.map(keys, fn {app, key} -> {app, key, Application.fetch_env(app, key)} end)
    old_chain_id = ConCache.get(ChainId.cache_name(), :id)

    on_exit(fn ->
      Enum.each(saved, fn
        {app, key, {:ok, value}} -> Application.put_env(app, key, value)
        {app, key, :error} -> Application.delete_env(app, key)
      end)

      ChainId.set_id(old_chain_id)
    end)

    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)
    Application.put_env(:block_scout_web, :chain_id, 125)
    ChainId.set_id(125)

    responses = start_supervised!({Agent, fn -> fn _, _ -> {200, ~s({"status":"ok"})} end end})
    ref = make_ref()
    {:ok, _pid} = Plug.Cowboy.http(Listener, {self(), responses}, port: 0, ref: ref, ip: {127, 0, 0, 1})
    on_exit(fn -> Plug.Cowboy.shutdown(ref) end)
    port = :ranch.get_port(ref)
    url = "http://127.0.0.1:#{port}"

    for module <- [AccountAbstraction, BENS, Metadata] do
      Application.put_env(:explorer, module,
        service_url: url,
        enabled: true,
        requests_timeout: 1_000,
        proxy_requests_timeout: 1_000
      )
    end

    Application.put_env(:explorer, MultichainSearch,
      service_url: url,
      api_key: "recorded-test-key",
      addresses_chunk_size: 1
    )

    {:ok, responses: responses}
  end

  test "account abstraction controller contains malformed upstream bodies", %{responses: responses} do
    for {upstream_status, expected_status} <- [{200, 502}, {404, 404}, {502, 502}] do
      respond(responses, fn _, _ -> {upstream_status, "<html>proxy unavailable</html>"} end)

      log =
        capture_log(fn ->
          conn = Plug.Test.conn(:get, "/api/v2/proxy/account-abstraction/operations")
          response = BlockScoutWeb.API.V2.Proxy.AccountAbstractionController.operations(conn, %{})
          assert response.status == expected_status

          assert Jason.decode!(response.resp_body) == %{
                   "error" => "Invalid response from Account Abstraction microservice"
                 }
        end)

      assert log =~ "Invalid JSON from Account Abstraction microservice"
      assert_receive {:wire, "GET", "/api/v1/userOps", %{}, _, nil}
    end

    for {status, body} <- [
          {200, %{"items" => [], "next_page_params" => nil}},
          {404, %{"message" => "Not found"}},
          {503, %{"error" => "Unavailable"}}
        ] do
      respond(responses, fn _, _ -> {status, Jason.encode!(body)} end)
      conn = Plug.Test.conn(:get, "/api/v2/proxy/account-abstraction/operations")
      response = BlockScoutWeb.API.V2.Proxy.AccountAbstractionController.operations(conn, %{})
      assert response.status == status
      assert Jason.decode!(response.resp_body) == body
      assert_receive {:wire, "GET", "/api/v1/userOps", %{}, _, nil}
    end
  end

  test "BENS preloader uses the production route in both protocol modes", %{responses: responses} do
    address = recorded_address(@address)
    hash = to_string(address.hash)
    checksum = Address.checksum(address.hash)
    respond(responses, fn _, _ -> {200, Jason.encode!(%{"names" => %{checksum => "recorded.eth"}})} end)

    for {protocols, path, expected_body} <- [
          {[], "/api/v1/125/addresses:batch-resolve-names", %{"addresses" => [hash]}},
          {["ens", "base-ens"], "/api/v1/addresses:batch-resolve",
           %{"addresses" => [hash], "protocols" => "ens,base-ens"}}
        ] do
      config = Application.fetch_env!(:explorer, BENS)
      Application.put_env(:explorer, BENS, Keyword.put(config, :protocols, protocols))
      assert [%Address{ens_domain_name: "recorded.eth", hash: address_hash}] = BENS.maybe_preload_ens([address])
      assert address_hash == address.hash
      assert_receive {:wire, "POST", ^path, %{}, ["application/json"], ^expected_body}
    end
  end

  test "metadata preloader reports and omits invalid nested annotations", %{responses: responses} do
    address = recorded_address(@address)
    hash = to_string(address.hash)
    checksum = Address.checksum(address.hash)
    good = %{"slug" => "recorded-protocol", "tagType" => "protocol", "meta" => ~s({"styles":"danger_high"})}

    for invalid_meta <- ["{broken", "[]", "null", "42", %{}, nil] do
      bad = %{"slug" => "invalid", "meta" => invalid_meta}
      body = Jason.encode!(%{"addresses" => %{checksum => %{"tags" => [good, bad]}}})
      respond(responses, fn _, _ -> {200, body} end)

      log =
        capture_log(fn ->
          assert [%Address{hash: recovered_hash, metadata: %{"tags" => [tag]}}] =
                   Metadata.maybe_preload_metadata([address])

          assert recovered_hash == address.hash
          assert tag == Map.put(good, "meta", %{"styles" => "danger_high"})
        end)

      assert log =~ "Invalid tag meta from Metadata microservice"
      assert_receive {:wire, "GET", "/api/v1/metadata", params, _, nil}
      assert params == %{"addresses" => hash, "chain_id" => "125", "tags_limit" => "5"}
    end
  end

  test "metadata search validates the same nested annotation boundary", %{responses: responses} do
    good = %{"slug" => "recorded-protocol", "meta" => ~s({"styles":"danger_high"})}

    body =
      Jason.encode!(%{
        "items" => [
          %{"tag" => good, "addresses" => [@address]},
          %{"tag" => %{"slug" => "broken", "meta" => "{"}, "addresses" => [@address]}
        ],
        "next_page_params" => nil
      })

    respond(responses, fn _, _ -> {200, body} end)

    {result, log} = with_log(fn -> Metadata.search_tags_by_name("recorded", nil) end)
    assert {:ok, %{items: [%{metadata: tag, hash: hash, addresses_index: 0}], next_page_params: nil}} = result, log
    assert tag == Map.put(good, "meta", %{"styles" => "danger_high"})
    assert to_string(hash) == String.downcase(@address)
    assert log =~ "Invalid tag meta from Metadata microservice"

    assert_receive {:wire, "GET", path, params, _, nil}
    assert path == "/api/v1/tags%3Asearch"
    assert params == %{"name" => "recorded", "chain_id" => "125", "tag_types" => "protocol,name"}
  end

  test "mixed HTTP failure and task exit recover in either order after restart", %{responses: responses} do
    params = recorded_export()
    chunks = MultichainSearch.extract_batch_import_params_into_chunks(params)
    assert length(chunks) == 2

    for kill_index <- [0, 1] do
      kill_hash = chunks |> Enum.at(kill_index) |> Map.fetch!(:addresses) |> hd() |> Map.fetch!(:hash)
      owner = self()

      respond(responses, fn conn, payload ->
        if hd(payload["addresses"])["hash"] == kill_hash do
          kill_export_task(conn, owner)
        end

        {503, ~s({"code":0,"message":"Unavailable"})}
      end)

      result =
        Task.async(fn ->
          Process.flag(:trap_exit, true)
          MainExporter.run(params, nil)
        end)

      assert {:retry, retry} = Task.await(result, 10_000)
      assert_retry_payload(retry, chunks)
      assert_receive {:export_task_killed, killed}
      refute Process.alive?(killed)
      assert_export_request()
      assert_export_request()
      assert_durable_queues()
      recover_queues(responses, chunks)
    end
  end

  test "malformed successful export is durable and replays after restart", %{responses: responses} do
    params = recorded_export()
    chunks = MultichainSearch.extract_batch_import_params_into_chunks(params)
    respond(responses, fn _, _ -> {200, "<html>invalid successful response</html>"} end)

    log =
      capture_log(fn ->
        assert {:retry, retry} = MainExporter.run(params, nil)
        assert_retry_payload(retry, chunks)
      end)

    assert log =~ "invalid_json"
    assert log =~ "/api/v1/import:batch"
    refute log =~ "recorded-test-key"
    assert_export_request()
    assert_export_request()
    assert_durable_queues()

    before_retry = Repo.all(MainExportQueue)
    assert {:retry, retry} = MainExporter.run(before_retry, nil)
    assert length(retry.addresses) == 2
    assert Repo.aggregate(MainExportQueue, :count) == 3
    assert Enum.all?(Repo.all(MainExportQueue), &(&1.retries_number == 1))
    assert_export_request()
    assert_export_request()
    recover_queues(responses, chunks)
  end

  defp respond(agent, responder), do: Agent.update(agent, fn _ -> responder end)

  defp recorded_address(text) do
    {:ok, hash} = Hash.Address.cast(text)
    insert(:address, hash: hash, fetched_coin_balance: %Wei{value: Decimal.new(42)})
  end

  defp recorded_export do
    first = recorded_address(@address)
    second = recorded_address(@other_address)
    {:ok, token_hash} = Hash.Address.cast(@token)

    %{
      addresses: [first, second],
      transactions: [%{hash: @transaction}],
      block_hashes: [],
      block_ranges: [%{min_block_number: "9208931", max_block_number: "9208931"}],
      address_current_token_balances: [
        %{
          address_hash: first.hash,
          token_contract_address_hash: token_hash,
          token_id: nil,
          value: Decimal.new(123)
        }
      ]
    }
  end

  defp assert_retry_payload(retry, chunks) do
    keys = [:addresses, :block_ranges, :hashes, :address_coin_balances, :address_token_balances]
    assert Enum.sort(Map.keys(retry)) == Enum.sort(keys)

    for key <- keys do
      assert Enum.sort(retry[key]) == Enum.sort(Enum.flat_map(chunks, & &1[key]))
    end
  end

  defp assert_durable_queues do
    rows = Repo.all(MainExportQueue)
    expected_hashes = [String.downcase(@address), String.downcase(@other_address), @transaction]
    assert Enum.sort(Enum.map(rows, &("0x" <> Base.encode16(&1.hash, case: :lower)))) == Enum.sort(expected_hashes)
    assert Enum.sort(Enum.map(rows, & &1.hash_type)) == [:address, :address, :transaction]
    balances = Repo.all(BalancesExportQueue)
    assert length(balances) == 3
    assert Enum.sort(Enum.map(balances, &Decimal.to_integer(&1.value.value))) == [42, 42, 123]
    assert Enum.count(balances, &(&1.token_contract_address_hash_or_native == "native")) == 2
  end

  defp recover_queues(responses, chunks) do
    # The actual BufferedTask init callbacks read committed queue rows on each reopen.
    start_supervised!({Task.Supervisor, name: Indexer.TaskSupervisor})

    for {fetcher, schema} <- [{MainExporter, MainExportQueue}, {BalancesExporter, BalancesExportQueue}] do
      supervisor = Module.concat(fetcher, Supervisor)
      Application.put_env(:indexer, supervisor, disabled?: false)
      persisted = Repo.all(schema)
      count = length(persisted)
      pid = start_supervised!({supervisor, [[poll: false, flush_interval: :infinity, max_concurrency: 0]]})
      eventually(fn -> GenServer.call(fetcher, :state).bound_queue.size == count end)
      worker = Process.whereis(fetcher)
      assert :ok = stop_supervised(supervisor)
      refute Process.alive?(pid)
      refute Process.alive?(worker)
      assert Repo.all(schema) == persisted

      respond(responses, fn _, _ -> {200, ~s({"status":"ok"})} end)
      reopened = start_supervised!({supervisor, [[poll: false, flush_interval: :infinity, max_concurrency: 1]]})
      assert reopened != pid
      eventually(fn -> Repo.aggregate(schema, :count) == 0 end)
      assert :ok = stop_supervised(supervisor)
    end

    assert :ok = stop_supervised(Indexer.TaskSupervisor)

    # Main and balances queues replay the same producer data, with balances sent
    # once by each queue. Compare all fields that survive queue serialization.
    messages = drain_exports([])
    assert messages != []
    all_addresses = Enum.flat_map(messages, & &1["addresses"])
    assert Enum.sort(all_addresses) == Enum.sort(Enum.flat_map(chunks, & &1.addresses) |> json_roundtrip())
    assert Enum.flat_map(messages, & &1["hashes"]) == json_roundtrip(Enum.flat_map(chunks, & &1.hashes))

    assert Enum.uniq(Enum.flat_map(messages, & &1["block_ranges"])) ==
             json_roundtrip(Enum.flat_map(chunks, & &1.block_ranges))

    replay_balances = Enum.flat_map(messages, & &1["address_coin_balances"])
    assert length(replay_balances) == 4
    assert Enum.all?(replay_balances, &(&1["value"] == "42"))

    assert Enum.sort(Enum.map(replay_balances, & &1["address_hash"])) ==
             Enum.sort(
               List.duplicate(String.downcase(@address), 2) ++ List.duplicate(String.downcase(@other_address), 2)
             )

    assert Enum.flat_map(messages, & &1["address_token_balances"]) ==
             json_roundtrip(Enum.flat_map(chunks, & &1.address_token_balances))

    assert Repo.aggregate(MainExportQueue, :count) == 0
    assert Repo.aggregate(BalancesExportQueue, :count) == 0
  end

  defp assert_export_request do
    assert_receive {:wire, "POST", "/api/v1/import:batch", %{}, ["application/json"], payload}, 5_000
    assert payload["chain_id"] == "125"
    assert payload["api_key"] == "recorded-test-key"
    payload
  end

  defp drain_exports(acc) do
    receive do
      {:wire, "POST", "/api/v1/import:batch", %{}, ["application/json"], payload} ->
        assert payload["chain_id"] == "125"
        assert payload["api_key"] == "recorded-test-key"
        drain_exports([payload | acc])
    after
      0 -> Enum.reverse(acc)
    end
  end

  defp json_roundtrip(value), do: value |> Jason.encode!() |> Jason.decode!()

  defp kill_export_task(conn, owner) do
    # Mint owns a real loopback socket. Locate its exporting task and interrupt it
    # after the upstream has received the complete payload, without replacing HTTP.
    peer_port = Plug.Conn.get_peer_data(conn).port

    client =
      Enum.find_value(Port.list(), fn port ->
        if Port.info(port, :name) == {:name, ~c"tcp_inet"} do
          case :inet.sockname(port) do
            {:ok, {{127, 0, 0, 1}, ^peer_port}} ->
              {:connected, pid} = Port.info(port, :connected)
              pid

            _ ->
              nil
          end
        end
      end)

    true = is_pid(client)
    {:dictionary, dictionary} = Process.info(client, :dictionary)
    candidates = [client | Keyword.get(dictionary, :"$callers", [])]

    task =
      Enum.find(candidates, fn pid ->
        case Process.info(pid, :current_stacktrace) do
          {:current_stacktrace, frames} ->
            Enum.any?(frames, fn {module, function, _, _} ->
              module == MultichainSearch and function == :http_post_request
            end)

          _ ->
            false
        end
      end)

    true = is_pid(task)
    send(owner, {:export_task_killed, task})
    Process.exit(task, :kill)
  end

  defp eventually(check, attempts \\ 250)
  defp eventually(check, 0), do: assert(check.())

  defp eventually(check, attempts) do
    if check.() do
      :ok
    else
      Process.sleep(20)
      eventually(check, attempts - 1)
    end
  end
end
