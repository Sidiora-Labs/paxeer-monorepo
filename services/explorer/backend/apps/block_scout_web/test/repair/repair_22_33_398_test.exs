defmodule BlockScoutWeb.Repair22_33_398Test do
  @moduledoc """
  Audit 22.30.142 findings 14–21. The in-scope repair_22_33_398.kvx maps
  each finding to its proving case and records out-of-scope wiring requests.
  SQL temporary tables reproduce the production MUD store and Optimism migration
  schemas in the disposable test database. Controllers, ABI codecs, signatures,
  HTTP transport, repositories, importer and views are production components.
  The interop transaction hash and payload come from the existing committed
  Optimism MessageQueueTest and OptimismControllerTest recordings.
  """

  use BlockScoutWeb.ConnCase, async: false

  alias BlockScoutWeb.API.V2.{
    ConfigController,
    CsvExportController,
    FallbackController,
    MudController,
    MudView,
    OptimismController,
    OptimismView
  }

  alias Ecto.Adapters.SQL
  alias Ecto.Adapters.SQL.Sandbox
  alias Explorer.Chain.{Address, Data, Hash, Mud, Token}
  alias Explorer.Chain.Cache.ChainId
  alias Explorer.Chain.Optimism.InteropMessage
  alias Explorer.Repo
  alias Indexer.Fetcher.Optimism.Interop.MessageQueue

  @key "repair+22&33=398"
  @private_key <<51, 169, 186, 160, 251, 109, 12, 35, 225, 13, 110, 62, 216, 253, 27, 181, 187, 222, 222, 75, 79, 84,
                 185, 24, 245, 213, 28, 21, 76, 179, 162, 16>>
  @transaction "0xf463ce43ac52251f60f437a0346b7168874e6eb13a9689d7efde4ce907826897"
  @payload "0x30787849217424792273712777707576696a6c5d"
  @sender "0x06012c8cf97bead5deae237070f9587f8e7a266d"
  @target "0x7b9ea9ac69b8fde875554321472c732eeff06ca0"
  @fetchers [
    {Indexer.Fetcher.InternalTransaction.Supervisor, "internal_transactions_fetcher_enabled"},
    {Indexer.Fetcher.PendingTransaction.Supervisor, "pending_transactions_fetcher_enabled"},
    {Indexer.Fetcher.TokenInstance.Retry.Supervisor, "token_instance_retry_fetcher_enabled"},
    {Indexer.Fetcher.TokenInstance.Sanitize.Supervisor, "token_instance_sanitize_fetcher_enabled"},
    {Indexer.Fetcher.BlockReward.Supervisor, "block_reward_fetcher_enabled"}
  ]

  defmodule Peer do
    @moduledoc false
    def init(owner), do: owner

    def call(conn, owner) do
      conn =
        Plug.Parsers.call(conn, Plug.Parsers.init(parsers: [:json], json_decoder: Jason, pass: ["application/json"]))

      conn = conn |> Phoenix.Controller.put_view(OptimismView) |> Phoenix.Controller.put_format("json")

      case {conn.method, conn.request_path} do
        {"GET", "/api/v2/optimism/interop/public-key"} ->
          send(owner, :public_key_requested)
          OptimismController.interop_public_key(conn, %{})

        {"POST", "/api/v2/import/optimism/interop/"} ->
          :ok = Sandbox.allow(Repo, owner, self())
          OptimismController.interop_import(conn, conn.body_params)
      end
    end
  end

  setup do
    settings =
      [
        {:indexer, Indexer.Supervisor},
        {:indexer, MessageQueue},
        {:explorer, :addresses_blacklist},
        {:explorer, :addresses_blacklist_key},
        {:explorer, :chain_type},
        {:block_scout_web, :sensitive_endpoints_api_key}
      ] ++ Enum.map(@fetchers, fn {module, _} -> {:indexer, module} end)

    for {app, key} <- settings do
      previous = Application.fetch_env(app, key)

      on_exit(fn ->
        case previous do
          {:ok, value} -> Application.put_env(app, key, value)
          :error -> Application.delete_env(app, key)
        end
      end)
    end

    Application.put_env(:explorer, :addresses_blacklist, "")
    Application.put_env(:explorer, :addresses_blacklist_key, @key)
    Application.put_env(:block_scout_web, :sensitive_endpoints_api_key, @key)
    :ok
  end

  test "finding 14: API-only mode reports the effective disabled supervisor", %{conn: conn} do
    Application.put_env(:indexer, Indexer.Supervisor, enabled: false)
    for {module, _} <- @fetchers, do: Application.put_env(:indexer, module, disabled?: false)

    response = conn |> ConfigController.indexer(%{}) |> Phoenix.ConnTest.json_response(200)
    assert response["indexer_enabled"] == false
    for {_, field} <- @fetchers, do: assert(response[field] == false)
  end

  test "finding 14: each effective fetcher flag is combined with its parent", %{conn: conn} do
    Application.put_env(:indexer, Indexer.Supervisor, enabled: true)

    for {disabled, _} <- @fetchers do
      for {module, _} <- @fetchers, do: Application.put_env(:indexer, module, disabled?: module == disabled)
      response = conn |> ConfigController.indexer(%{}) |> Phoenix.ConnTest.json_response(200)
      assert response["indexer_enabled"] == true
      for {module, field} <- @fetchers, do: assert(response[field] == (module != disabled))
    end
  end

  test "finding 15: every shared CSV action denies restrictions before streaming and accepts the key" do
    address = insert(:address, hash: @sender)
    Application.put_env(:explorer, :addresses_blacklist, String.upcase(@sender))
    # Celo's exporter reads the epoch range even when no epochs have been indexed.
    SQL.query!(Repo, "CREATE TEMP TABLE celo_epochs (number bigint, end_block_number bigint)", [])

    actions = [
      :transactions_csv,
      :internal_transactions_csv,
      :token_transfers_csv,
      :logs_csv,
      :celo_election_rewards_csv
    ]

    params = %{address_hash_param: Address.checksum(address.hash), from_period: "2023-01-01", to_period: "2023-01-02"}

    for action <- actions do
      for access <- [%{}, %{key: "wrong"}] do
        conn = build_conn()

        assert {:restricted_access, true} =
                 error = apply(CsvExportController, action, [conn, Map.merge(params, access)])

        assert conn.state == :unset
        denied = FallbackController.call(json_conn(), error)
        assert Phoenix.ConnTest.json_response(denied, 403) == %{"message" => "Restricted access"}
        assert get_resp_header(denied, "content-disposition") == []
        assert denied.resp_cookies == %{}
      end

      authorized = apply(CsvExportController, action, [fetch_cookies(build_conn()), Map.put(params, :key, @key)])
      assert authorized.status == 200
      assert authorized.state == :chunked
      assert get_resp_header(authorized, "content-type") == ["application/csv; charset=utf-8"]
      assert get_resp_header(authorized, "content-disposition") == ["attachment;"]
      assert authorized.resp_body != ""
    end
  end

  test "finding 16: source lookup for an indexed EOA renders the existing client error", %{conn: conn} do
    address = insert(:address, hash: @target, contract_code: nil)
    response = conn |> put_req_header("x-api-key", @key) |> get("/api/v2/import/smart-contracts/#{address.hash}")
    assert Phoenix.ConnTest.json_response(response, 404) == %{"message" => "Address is not a smart-contract"}
  end

  test "finding 17: header-authenticated JSON token metadata persists without logging the key", %{conn: conn} do
    token = insert(:token, contract_address: insert(:address, hash: @sender))

    body = %{
      "icon_url" => "https://example.com/token.svg",
      "token_address" => to_string(token.contract_address_hash),
      "token_symbol" => "REC",
      "token_name" => "Recorded token"
    }

    log =
      ExUnit.CaptureLog.capture_log(fn ->
        response =
          conn
          |> put_req_header("x-api-key", @key)
          |> put_req_header("content-type", "application/json")
          |> post("/api/v2/import/token-info", Jason.encode!(body))

        assert Phoenix.ConnTest.json_response(response, 200) == %{"message" => "Success"}
        updated = Repo.get!(Token, token.contract_address_hash)
        assert updated.name == "Recorded token"
        assert updated.symbol == "REC"
        assert updated.icon_url == body["icon_url"]
        assert updated.is_verified_via_admin_panel
      end)

    refute log =~ @key

    body_only = post(conn, "/api/v2/import/token-info", Map.put(body, "api_key", @key))
    assert Phoenix.ConnTest.json_response(body_only, 401) == %{"message" => "Wrong API key"}

    legacy = %{
      "iconUrl" => "https://example.com/legacy.svg",
      "tokenAddress" => to_string(token.contract_address_hash),
      "tokenSymbol" => "OLD",
      "tokenName" => "Legacy fields"
    }

    response = conn |> put_req_header("x-api-key", @key) |> post("/api/v2/import/token-info", legacy)
    assert Phoenix.ConnTest.json_response(response, 200) == %{"message" => "Success"}
    assert Repo.get!(Token, token.contract_address_hash).symbol == "OLD"
  end

  test "finding 17: DELETE uses the same single header and rejects body and duplicate keys", %{conn: conn} do
    params = %{"token_address_hash" => @target}

    for headers <- [[], [{"x-api-key", "wrong"}], [{"x-api-key", @key}, {"x-api-key", @key}]] do
      request =
        Enum.reduce(headers, conn, fn {key, value}, acc -> %{acc | req_headers: [{key, value} | acc.req_headers]} end)

      response = delete(request, "/api/v2/import/token-info", Map.put(params, "api_key", @key))
      assert Phoenix.ConnTest.json_response(response, 401) == %{"message" => "Wrong API key"}
    end

    response = conn |> put_req_header("x-api-key", @key) |> delete("/api/v2/import/token-info", params)
    assert Phoenix.ConnTest.json_response(response, 404) == %{"message" => "Not found"}
  end

  test "finding 18: 32-byte filters reach MUD records and malformed filters are client errors" do
    {world, table} = mud_fixture(95, <<255::256>>)
    valid = "0x" <> Base.encode16(<<255::256>>, case: :lower)
    params = %{world: to_string(world), table_id: to_string(table), filter_key0: valid, filter_key1: "1"}
    assert Phoenix.ConnTest.json_response(mud_count(params), 200) == 1
    listed = Phoenix.ConnTest.json_response(mud_records(params), 200)
    assert [%{"decoded" => %{"key0" => ^valid, "key1" => "1"}}] = listed["items"]

    assert Phoenix.ConnTest.json_response(mud_count(%{params | filter_key0: "0x" <> String.duplicate("00", 32)}), 200) ==
             0

    for invalid <- ["0x" <> String.duplicate("ff", 33), "0xgg", "0x1", "0x", "12tail", "true"] do
      response = mud_count(%{params | filter_key0: invalid})
      assert mud_records(%{params | filter_key0: invalid}).status == 422
      assert response.status == 422
      assert Phoenix.ConnTest.json_response(response, 422)["message"] != nil
    end

    specs = MudController.open_api_operation(:world_table_records)

    for param <- Enum.filter(specs.parameters, &(&1.name in [:filter_key0, :filter_key1])) do
      assert param.schema.maxLength == 78
      assert param.schema.pattern =~ "1,32"
    end
  end

  test "finding 18: numeric filters respect the FieldSchema width and consume the whole number" do
    {world, table} = mud_fixture(0, <<255::256>>)
    params = %{world: to_string(world), table_id: to_string(table), filter_key0: "255"}
    assert Phoenix.ConnTest.json_response(mud_count(params), 200) == 1

    for invalid <- ["256", "-1", "255tail", "0x0100"] do
      assert mud_count(%{params | filter_key0: invalid}).status == 422
    end
  end

  test "finding 18: signed numeric bounds cannot wrap the key field" do
    {world, table} = mud_fixture(32, <<-128::256>>)
    params = %{world: to_string(world), table_id: to_string(table), filter_key0: "-128"}
    assert Phoenix.ConnTest.json_response(mud_count(params), 200) == 1

    for invalid <- ["-129", "128", "0xff"] do
      assert mud_count(%{params | filter_key0: invalid}).status == 422
    end
  end

  test "finding 19: missing and malformed optional keys return 404 and the recorded key derives a public key" do
    Application.delete_env(:indexer, MessageQueue)
    missing = OptimismController.interop_public_key(json_conn(OptimismView), %{})
    assert Phoenix.ConnTest.json_response(missing, 404) == %{"message" => "private key is invalid or undefined"}

    for key <- [nil, "", 12, [], "0xzz", "0x01", "0x" <> String.duplicate("00", 32)] do
      Application.put_env(:indexer, MessageQueue, private_key: key)
      response = OptimismController.interop_public_key(json_conn(OptimismView), %{})
      assert Phoenix.ConnTest.json_response(response, 404) == %{"message" => "private key is invalid or undefined"}
    end

    Application.put_env(:indexer, MessageQueue, private_key: "0x" <> Base.encode16(@private_key, case: :lower))
    {:ok, key} = ExSecp256k1.create_public_key(@private_key)
    response = OptimismController.interop_public_key(json_conn(OptimismView), %{})
    assert Phoenix.ConnTest.json_response(response, 200) == %{"public_key" => hex(key)}
  end

  test "findings 20 and 21: canonical relay crosses HTTP, binds fields and persists via the importer" do
    peer = interop_fixture()
    params = relay_params()
    signed = sign_request(params)
    assert {200, %{"init_transaction_hash" => nil}} = post_interop(peer, signed)
    assert_receive :public_key_requested
    stored = Repo.get_by!(InteropMessage, init_chain_id: 23, nonce: 1)
    assert to_string(stored.relay_transaction_hash) == @transaction
    assert stored.failed == false
    assert stored.relay_chain_id == 45

    {:ok, original} = OptimismController.interop_signing_data(params)
    altered = Map.merge(signed, %{"nonce" => 12, "init_chain_id" => 3})
    {:ok, changed} = OptimismController.interop_signing_data(altered)
    refute original == changed
    ChainId.set_id(3)
    assert {401, %{"message" => "Wrong signature"}} = post_interop(peer, altered)
    assert Repo.get_by(InteropMessage, init_chain_id: 3, nonce: 12) == nil

    ChainId.set_id(23)

    for altered <- [Map.put(signed, "failed", true), Map.put(signed, "relay_transaction_hash", hex(<<1::256>>))] do
      assert {401, %{"message" => "Wrong signature"}} = post_interop(peer, altered)
    end

    ChainId.set_id(999)
    assert {400, %{"message" => "Invalid interop request"}} = post_interop(peer, signed)
    assert Repo.get_by!(InteropMessage, init_chain_id: 23, nonce: 1).failed == false
  end

  test "finding 20: malformed complete requests are rejected before any remote key lookup" do
    peer = interop_fixture()
    relay = Map.put(relay_params(), "signature", hex(<<0::512>>))
    init = Map.put(init_params(), "signature", hex(<<0::512>>))

    invalid = [
      %{},
      Map.delete(relay, "nonce"),
      Map.delete(relay, "protocol_version"),
      Map.put(relay, "protocol_version", 2),
      Map.put(relay, "signature", "0xgg"),
      Map.put(relay, "signature", "0x" <> String.duplicate("zz", 64)),
      Map.put(relay, "signature", hex(<<1>>)),
      Map.put(relay, "nonce", "1"),
      Map.put(relay, "nonce", -1),
      Map.put(relay, "nonce", 9_223_372_036_854_775_808),
      Map.put(relay, "init_chain_id", 1.5),
      Map.put(relay, "relay_chain_id", "45"),
      Map.put(relay, "failed", "false"),
      Map.put(relay, "relay_transaction_hash", "0xgg"),
      Map.put(relay, "payload", "0x"),
      Map.put(init, "timestamp", "1"),
      Map.put(init, "timestamp", 253_402_300_800),
      Map.put(init, "timestamp", -62_167_219_201),
      Map.put(init, "sender_address_hash", "bad"),
      Map.put(init, "payload", "0x1"),
      Map.put(init, "payload", "0x4f0edcc9"),
      Map.put(init, "target_address_hash", 1),
      Map.put(init, "init_transaction_hash", nil)
    ]

    for params <- invalid do
      ChainId.set_id(if Map.has_key?(params, "init_transaction_hash"), do: 45, else: 23)
      assert {400, %{"message" => "Invalid interop request"}} = post_interop(peer, params)
    end

    refute_received :public_key_requested
    assert Repo.aggregate(InteropMessage, :count) == 0
  end

  test "findings 20 and 21: canonical init validates the date and stores the decoded recorded payload" do
    peer = interop_fixture()
    ChainId.set_id(45)
    params = init_params()
    assert {200, %{"relay_transaction_hash" => nil, "failed" => nil}} = post_interop(peer, sign_request(params))
    assert_receive :public_key_requested
    stored = Repo.get_by!(InteropMessage, init_chain_id: 23, nonce: 1)
    assert DateTime.to_unix(stored.timestamp) == params["timestamp"]
    assert to_string(stored.payload) == @payload
    assert to_string(stored.sender_address_hash) == @sender
    assert to_string(stored.target_address_hash) == @target
    assert to_string(stored.init_transaction_hash) == @transaction
    assert stored.relay_chain_id == 45

    altered = params |> sign_request() |> Map.put("timestamp", params["timestamp"] + 1)
    assert {401, %{"message" => "Wrong signature"}} = post_interop(peer, altered)
    assert DateTime.to_unix(Repo.get_by!(InteropMessage, init_chain_id: 23, nonce: 1).timestamp) == params["timestamp"]
  end

  defp json_conn(view \\ BlockScoutWeb.API.V2.ApiView) do
    build_conn() |> Phoenix.Controller.put_view(view) |> Phoenix.Controller.put_format("json")
  end

  defp mud_count(params) do
    conn = json_conn(MudView)

    case MudController.world_table_records_count(conn, params) do
      %Plug.Conn{} = result -> result
      error -> FallbackController.call(conn, error)
    end
  end

  defp mud_records(params) do
    conn = json_conn(MudView) |> fetch_query_params()

    case MudController.world_table_records(conn, params) do
      %Plug.Conn{} = result -> result
      error -> FallbackController.call(conn, error)
    end
  end

  defp mud_fixture(type, key) do
    :ok = Sandbox.checkout(Repo.Mud)

    SQL.query!(
      Repo.Mud,
      """
      CREATE TEMP TABLE records (
        address bytea NOT NULL, table_id bytea NOT NULL, key_bytes bytea, key0 bytea, key1 bytea,
        static_data bytea, encoded_lengths bytea, dynamic_data bytea, is_deleted boolean NOT NULL,
        block_number numeric NOT NULL, log_index numeric NOT NULL
      )
      """,
      []
    )

    {:ok, world} = Hash.Address.cast(@sender)
    {:ok, table} = Mud.Table.table_full_name_to_table_id("tb.repair.Keys")
    {:ok, store} = Hash.Full.cast("0x746273746f72650000000000000000005461626c657300000000000000000000")
    names = ABI.TypeEncoder.encode([["key0", "key1"]], [{:array, :string}])
    values = ABI.TypeEncoder.encode([[]], [{:array, :string}])
    key_schema = <<0::16, 2, 0, type, 31, 0::208>>

    registration = %Mud{
      address: world,
      table_id: store,
      key0: table,
      key_bytes: %Data{bytes: table.bytes},
      static_data: %Data{bytes: <<0::256, key_schema::binary, 0::256>>},
      encoded_lengths: %Data{bytes: <<0::120, byte_size(values)::40, byte_size(names)::40, 0::56>>},
      dynamic_data: %Data{bytes: names <> values},
      is_deleted: false,
      block_number: Decimal.new(1),
      log_index: Decimal.new(0)
    }

    Repo.Mud.insert!(registration)
    {:ok, key0} = Hash.Full.cast(key)
    {:ok, key1} = Hash.Full.cast(<<1::256>>)

    Repo.Mud.insert!(%Mud{
      address: world,
      table_id: table,
      key0: key0,
      key1: key1,
      key_bytes: %Data{bytes: key <> <<1::256>>},
      static_data: %Data{bytes: <<>>},
      is_deleted: false,
      block_number: Decimal.new(1),
      log_index: Decimal.new(1)
    })

    assert {:ok, %{key_names: ["key0", "key1"]}} = Mud.world_table_schema(world, table)
    {world, table}
  end

  defp relay_params do
    %{
      "protocol_version" => 1,
      "nonce" => 1,
      "init_chain_id" => 23,
      "relay_chain_id" => 45,
      "relay_transaction_hash" => @transaction,
      "failed" => false
    }
  end

  defp init_params do
    relay_params()
    |> Map.drop(["relay_transaction_hash", "failed"])
    |> Map.merge(%{
      "sender_address_hash" => @sender,
      "target_address_hash" => @target,
      "init_transaction_hash" => @transaction,
      "timestamp" => 1_700_000_000,
      "payload" => @payload
    })
  end

  defp sign_request(params) do
    {:ok, data} = OptimismController.interop_signing_data(params)
    {:ok, {signature, _}} = data |> ExKeccak.hash_256() |> ExSecp256k1.sign_compact(@private_key)
    {_, signed} = MessageQueue.set_post_data_signature(23, %{signature: nil}, signature)
    Map.put(params, "signature", signed.signature)
  end

  defp interop_fixture do
    SQL.query!(
      Repo,
      """
      CREATE TEMP TABLE op_interop_messages (
        sender_address_hash bytea, target_address_hash bytea, nonce bigint NOT NULL, init_chain_id integer NOT NULL,
        init_transaction_hash bytea, block_number bigint, timestamp timestamp(6), relay_chain_id integer NOT NULL,
        relay_transaction_hash bytea, payload bytea, failed boolean, transfer_token_address_hash bytea,
        transfer_from_address_hash bytea, transfer_to_address_hash bytea, transfer_amount numeric,
        sent_to_multichain boolean NOT NULL DEFAULT false, inserted_at timestamp(6) NOT NULL,
        updated_at timestamp(6) NOT NULL, PRIMARY KEY (nonce, init_chain_id)
      )
      """,
      []
    )

    Application.put_env(:explorer, :chain_type, :optimism)

    for cache <- [
          InteropMessage.interop_instance_api_url_to_public_key_cache(),
          InteropMessage.interop_chain_id_to_instance_info_cache()
        ] do
      if Process.whereis(cache) == nil do
        start_supervised!(Supervisor.child_spec({ConCache, name: cache, ttl_check_interval: false}, id: cache))
      end
    end

    previous_id = ConCache.get(ChainId.cache_name(), :id)
    ChainId.set_id(23)
    on_exit(fn -> ChainId.set_id(previous_id) end)
    ref = make_ref()
    start_supervised!({Plug.Cowboy, scheme: :http, plug: {Peer, self()}, options: [port: 0, ref: ref]})
    peer = "http://127.0.0.1:#{:ranch.get_port(ref)}"

    Application.put_env(:indexer, MessageQueue,
      private_key: hex(@private_key),
      connect_timeout: 2,
      recv_timeout: 2,
      chainscout_fallback_map: %{"23" => peer, "3" => peer, "45" => peer}
    )

    peer
  end

  defp post_interop(peer, params) do
    client = Tesla.client([], Tesla.Adapter.Mint)

    {:ok, response} =
      Tesla.post(client, peer <> "/api/v2/import/optimism/interop/", Jason.encode!(params),
        headers: [{"content-type", "application/json"}]
      )

    {response.status, Jason.decode!(response.body)}
  end

  defp hex(bytes), do: "0x" <> Base.encode16(bytes, case: :lower)
end
