defmodule BlockScoutWeb.Repair.Repair2233426Test do
  @moduledoc """
  Task 22.33.426 repair record (audit 22.30.146, acceptance 22998.1).

  schema-missing: the missing path, operation, response and nil-schema tests.
  schema-specificity: exact route, cross-document, specificity and ambiguity tests.
  conn-global-cache: concurrent connection fixtures and owner crash tests.
  feature-global-cache: concurrent feature fixtures and mixed fixture tests.
  The verified candidate revision, command, exit and private log are recorded by
  hf-done in its final evidence line; this source is part of that candidate.
  Baseline: not run; the explicit bounded gate order prohibits pre-change verify.
  Wiring: none; production cache names require shared lifetime serialization.
  Gotcha: ExUnit's async flag alone cannot serialize connection and feature cases;
  both case templates must acquire the same lock for the entire test lifetime.
  """

  use ExUnit.Case, async: false

  import Phoenix.ConnTest, except: [json_response: 2]
  import Explorer.Factory

  alias BlockScoutWeb.{CacheFixture, ConnCase, FeatureCase, TestApiSchemaAssertions}
  alias Explorer.Chain.Cache.{Accounts, Transactions}
  alias Explorer.Chain.Transaction
  alias OpenApiSpex.{Components, Reference}

  @endpoint BlockScoutWeb.Endpoint
  @recordings Path.expand("../../../explorer/test/support/fixture/vcr_cassettes", __DIR__)
  @config_path "/v2/config/backend"

  setup tags do
    unless tags[:cache_fixture] do
      :ok = ConnCase.setup_cache_fixture()
      :ok = Ecto.Adapters.SQL.Sandbox.checkout(Explorer.Repo)
      :ok = Ecto.Adapters.SQL.Sandbox.checkout(Explorer.Repo.Account)
    end

    :ok
  end

  test "schema-missing: real recorded transaction response is validated and malformed output fails" do
    transaction = persist_recorded_transaction()
    conn = get(build_conn(), "/api/v2/transactions/#{transaction.hash}")
    json = TestApiSchemaAssertions.json_response(conn, 200)
    assert json["hash"] == to_string(transaction.hash)
    assert json["to"] == nil

    assert_raise ExUnit.AssertionError, fn ->
      TestApiSchemaAssertions.assert_response_schema(conn, 200, Map.put(json, "hash", "invalid"), specs())
    end
  end

  test "schema-missing: missing path fails explicitly" do
    {conn, json, spec} = config_response()

    assert_raise ExUnit.AssertionError, ~r/Missing OpenAPI path/, fn ->
      validate(conn, json, %{spec | paths: Map.delete(spec.paths, @config_path)})
    end
  end

  test "schema-missing: missing operation fails explicitly" do
    {conn, json, spec} = config_response()
    spec = update_in(spec.paths[@config_path].get, fn _ -> nil end)
    assert_raise ExUnit.AssertionError, ~r/Missing OpenAPI GET operation/, fn -> validate(conn, json, spec) end
  end

  test "schema-missing: missing status response fails explicitly" do
    {conn, json, spec} = config_response()
    spec = update_in(spec.paths[@config_path].get.responses, fn _ -> %{} end)
    assert_raise ExUnit.AssertionError, ~r/Missing OpenAPI JSON response schema/, fn -> validate(conn, json, spec) end
  end

  test "schema-missing: nil response schema fails explicitly" do
    {conn, json, spec} = config_response()
    spec = update_in(spec.paths[@config_path].get.responses[200].content["application/json"].schema, fn _ -> nil end)
    assert_raise ExUnit.AssertionError, ~r/Missing OpenAPI JSON response schema/, fn -> validate(conn, json, spec) end
  end

  test "schema-missing: referenced responses resolve their JSON schema" do
    {conn, json, spec} = config_response()
    response = spec.paths[@config_path].get.responses[200]
    components = spec.components || %Components{}
    spec = %{spec | components: %{components | responses: %{"Backend" => response}}}
    spec = put_in(spec.paths[@config_path].get.responses[200], %Reference{"$ref": "#/components/responses/Backend"})
    validate(conn, json, spec)

    spec = put_in(spec.components.responses["Backend"], nil)
    assert_raise ExUnit.AssertionError, ~r/Missing OpenAPI JSON response schema/, fn -> validate(conn, json, spec) end
  end

  test "schema-missing: non-API pages and documented legacy RPC entry points are schema-free" do
    for path <- ["/api", "/api/", "/account", "/apiculture"] do
      conn = build_conn(:get, path) |> Phoenix.Controller.json(%{status: "legacy"})
      assert TestApiSchemaAssertions.json_response(conn, 200) == %{"status" => "legacy"}
    end

    conn = build_conn(:get, "/api/v2/undeclared") |> Phoenix.Controller.json(%{status: "missing"})

    assert_raise ExUnit.AssertionError, ~r/Missing OpenAPI path/, fn ->
      TestApiSchemaAssertions.json_response(conn, 200)
    end
  end

  test "schema-specificity: static transaction stats wins over transaction hash" do
    persist_recorded_transaction()
    conn = get(build_conn(), "/api/v2/transactions/stats")
    json = TestApiSchemaAssertions.json_response(conn, 200)
    assert Map.has_key?(json, "transactions_count_24h")
    refute Map.has_key?(json, "hash")

    [public, _] = specs()
    static = Map.fetch!(public.paths, "/v2/transactions/stats")
    dynamic = Map.fetch!(public.paths, "/v2/transactions/{transaction_hash_param}")
    first = %{public | paths: %{"/v2/transactions/{transaction_hash_param}" => dynamic}}
    second = %{public | paths: %{"/v2/transactions/stats" => static}}

    for documents <- [[first, second], [second, first]] do
      TestApiSchemaAssertions.assert_response_schema(conn, 200, json, documents)
    end
  end

  test "schema-specificity: most specific compatible template wins across documents" do
    {conn, json, spec} = config_response()
    config = spec.paths[@config_path]
    transaction = spec.paths["/v2/transactions/{transaction_hash_param}"]
    broad = %{spec | paths: %{"/v2/{section}/{name}" => transaction}}
    specific = %{spec | paths: %{"/v2/config/{name}" => config}}

    for documents <- [[broad, specific], [specific, broad]] do
      TestApiSchemaAssertions.assert_response_schema(conn, 200, json, documents)
    end
  end

  test "schema-specificity: equally specific compatible templates are rejected" do
    {conn, json, spec} = config_response()
    item = spec.paths[@config_path]
    paths = %{"/v2/config/{name}" => item, "/v2/{section}/backend" => item}

    assert_raise ExUnit.AssertionError, ~r/Ambiguous OpenAPI path/, fn ->
      validate(conn, json, %{spec | paths: paths})
    end

    left = %{spec | paths: %{"/v2/config/{name}" => item}}
    right = %{spec | paths: %{"/v2/config/{other}" => item}}

    assert_raise ExUnit.AssertionError, ~r/Ambiguous OpenAPI path/, fn ->
      TestApiSchemaAssertions.assert_response_schema(conn, 200, json, [left, right])
    end
  end

  @tag cache_fixture: true
  test "conn-global-cache: concurrent connection fixtures retain caches until the owner exits" do
    assert_serialized(ConnCase, ConnCase)
  end

  @tag cache_fixture: true
  test "feature-global-cache: concurrent feature fixtures retain caches until the owner exits" do
    assert_serialized(FeatureCase, FeatureCase)
  end

  @tag cache_fixture: true
  test "feature-global-cache: feature and connection fixtures share lifetime ownership" do
    assert_serialized(ConnCase, FeatureCase)
    assert_serialized(FeatureCase, ConnCase)
  end

  @tag cache_fixture: true
  test "conn-global-cache: a crashed owner releases the lock and the next fixture restarts cleanly" do
    parent = self()

    owner =
      spawn(fn ->
        :ok = ConnCase.setup_cache_fixture()
        cache_recorded_transaction()
        send(parent, {:owner_ready, self(), cache_pids()})

        receive do
          :stay -> :ok
        end
      end)

    monitor = Process.monitor(owner)
    assert_receive {:owner_ready, ^owner, previous}, 10_000
    Process.exit(owner, :kill)
    assert_receive {:DOWN, ^monitor, :process, ^owner, :killed}

    next =
      Task.async(fn ->
        :ok = FeatureCase.setup_cache_fixture()
        assert Transactions.all() == []
        assert Accounts.all() == []
        cache_pids()
      end)

    current = Task.await(next, 15_000)
    assert_restarted(previous, current)
  end

  @tag cache_fixture: true
  test "cache setup reports missing supervised cache children" do
    {:ok, supervisor} = Supervisor.start_link([], strategy: :one_for_one)

    try do
      assert_raise ExUnit.AssertionError, ~r/Cannot terminate cache.*not_found/, fn ->
        CacheFixture.reset!(supervisor)
      end
    after
      Supervisor.stop(supervisor)
    end
  end

  defp assert_serialized(first_case, second_case) do
    parent = self()

    first =
      Task.async(fn ->
        :ok = first_case.setup_cache_fixture()
        transaction = cache_recorded_transaction()
        send(parent, {:ready, self(), cache_pids(), transaction.hash})

        receive do
          :release -> :ok
        end

        assert Transactions.all() == [transaction]
        assert Enum.all?(cache_pids(), &Process.alive?/1)
      end)

    first_pid = first.pid
    assert_receive {:ready, ^first_pid, previous, hash}, 10_000
    refute :global.set_lock({CacheFixture, self()}, [node()], 0)

    second =
      Task.async(fn ->
        send(parent, {:attempting, self()})
        :ok = second_case.setup_cache_fixture()
        assert Transactions.all() == []
        assert Accounts.all() == []
        current = cache_pids()
        transaction = cache_recorded_transaction()
        assert transaction.hash == hash
        assert Transactions.take(1) == [transaction]
        send(parent, {:second_ready, self()})
        current
      end)

    second_pid = second.pid
    assert_receive {:attempting, ^second_pid}
    refute_receive {:second_ready, ^second_pid}, 100
    assert cache_pids() == previous
    send(first_pid, :release)
    Task.await(first, 10_000)
    current = Task.await(second, 15_000)
    assert_restarted(previous, current)
  end

  defp assert_restarted(previous, current) do
    for {old, new} <- Enum.zip(previous, current) do
      refute old == new
      refute Process.alive?(old)
      assert Process.alive?(new)
    end
  end

  defp cache_pids, do: Enum.map([Transactions, Accounts], &Process.whereis(&1.cache_name()))

  defp cache_recorded_transaction do
    transaction = %Transaction{} |> Transaction.changeset(recorded_params()) |> Ecto.Changeset.apply_changes()
    id = Transactions.element_to_id(transaction)
    :ok = Transactions.do_raw_update([{id, transaction}], false)
    assert Transactions.get(id) == transaction
    transaction
  end

  defp persist_recorded_transaction do
    params = recorded_params()

    [receipt_recording] =
      @recordings |> Path.join("transaction_importer_import_1_receipt.json") |> File.read!() |> Jason.decode!()

    %{"result" => receipt} = Jason.decode!(receipt_recording["response"]["body"])
    assert receipt["transactionHash"] == to_string(params.hash)
    receipt_params = receipt |> EthereumJSONRPC.Receipt.to_elixir() |> EthereumJSONRPC.Receipt.elixir_to_params()
    insert(:address, hash: params.from_address_hash)
    insert(:address, hash: receipt_params.created_contract_address_hash)
    block = insert(:block, hash: params.block_hash, number: params.block_number)

    params =
      params |> Map.merge(receipt_params) |> Map.merge(%{block_consensus: true, block_timestamp: block.timestamp})

    %Transaction{} |> Transaction.changeset(params) |> Explorer.Repo.insert!()
  end

  defp recorded_params do
    [recording] =
      @recordings
      |> Path.join("transaction_importer_import_saves_the_transaction.json")
      |> File.read!()
      |> Jason.decode!()

    %{"result" => transaction} = Jason.decode!(recording["response"]["body"])
    transaction |> EthereumJSONRPC.Transaction.to_elixir() |> EthereumJSONRPC.Transaction.elixir_to_params()
  end

  defp specs, do: [BlockScoutWeb.Specs.Public.spec(), BlockScoutWeb.Specs.Private.spec()]

  defp config_response do
    conn = get(build_conn(), "/api" <> @config_path)
    json = TestApiSchemaAssertions.json_response(conn, 200)
    assert Map.has_key?(json, "chain_type")
    {conn, json, BlockScoutWeb.Specs.Public.spec()}
  end

  defp validate(conn, json, spec), do: TestApiSchemaAssertions.assert_response_schema(conn, 200, json, [spec])
end
