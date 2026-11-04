defmodule BlockScoutWeb.Repair22_33_478Test do
  @moduledoc """
  Task 22.33.478 repair record: audit 22.30.149, acceptance 22998.1.
  account-api-documentation-version is proved by the route inventory, published
  authentication schemas, session/CSRF, address tag, recorded transaction tag,
  watchlist, API key and custom ABI cases below. Examples are read from API.md
  and submitted to the production routers, controllers, repositories and views.
  Chain data comes from the committed transaction importer transaction/receipt
  recordings, decoded by EthereumJSONRPC and stored with Transaction.changeset.
  The session producer is AuthenticateController.put_auth_to_session/2 with the
  committed auth fixture's verified identity and illustrative profile fields.
  Plug.Session's standard ETS store transports the produced session cookies;
  BlockScoutWeb.Router forwards the documented full paths through ApiRouter into
  AccountRouter. No live authentication material or provider requests are used.
  First qualification: the original endpoint fixture reached the deployment's
  Redis session store with accounts disabled. The fixture now explicitly enables
  accounts and uses an in-process session peer, preserving router/CSRF checks.
  Baseline: not run; the explicit bounded gate order requires implementation
  before hf-build and hf-done. No pre-change failures are claimed.
  Continued attempt: the supplied qualification reports five passing cases and
  three failures, identifying a raw-schema module argument, missing published
  account paths and a user-info request without authentication. The cases below
  check the Schema struct, exact published route inventory and produced session.
  Qualification finding: revision 177739c359223461f19bc3e4d98ca3d0533b1e28;
  command: services/explorer/deploy/tools/mix-in-builder.sh test apps/block_scout_web/test/repair/repair_22_33_478_test.exs;
  exit: 2; private evidence: /out/logs/verify-1.log.
  Observed: the recorded creation has no recipient and the tag lookup raises.
  Assumption: creation transactions must retain their recorded nil recipient.
  Continued attempt 3 (decision 2542, gotcha 3037): seven of eight cases passed.
  The controller is now in scope: tags_transaction/2 preserves the sender's
  public tags and omits the recipient lookup when to_address_hash is nil.
  Proving test: "account-api-documentation-version: recorded transaction tag
  examples reach the combined consumer" retains the documented response and
  checks sender public tags for both authenticated and anonymous requests.
  No out-of-path wiring remains for account-api-documentation-version.
  """

  use BlockScoutWeb.ConnCase, async: false

  alias BlockScoutWeb.Account.API.V2.{AuthenticateController, UserView}
  alias BlockScoutWeb.Routers.AccountRouter
  alias BlockScoutWeb.Schemas.API.V2.Account, as: AccountSchemas
  alias Explorer.Chain.Transaction

  @guide Path.expand("../../API.md", __DIR__)
  @recordings Path.expand("../../../explorer/test/support/fixture/vcr_cassettes", __DIR__)

  @session_table :repair_22_33_478_sessions
  @session_key "repair_22_33_478_session"

  defmodule SessionTransport do
    @moduledoc false
    use Plug.Builder

    plug(:session)
    plug(BlockScoutWeb.Router)

    def session(conn, _opts) do
      Plug.CSRFProtection.delete_csrf_token()
      opts = Plug.Session.init(store: :ets, key: "repair_22_33_478_session", table: :repair_22_33_478_sessions)

      conn
      |> Map.put(:secret_key_base, BlockScoutWeb.Endpoint.config(:secret_key_base))
      |> Plug.Conn.put_private(:phoenix_endpoint, BlockScoutWeb.Endpoint)
      |> Plug.Conn.put_private(:plug_skip_csrf_protection, false)
      |> Plug.Session.call(opts)
    end
  end

  @endpoint SessionTransport

  setup %{conn: conn} do
    previous = Application.fetch_env(:explorer, Explorer.Account)
    config = Application.get_env(:explorer, Explorer.Account, [])
    Application.put_env(:explorer, Explorer.Account, Keyword.put(config, :enabled, true))

    on_exit(fn ->
      case previous do
        {:ok, value} -> Application.put_env(:explorer, Explorer.Account, value)
        :error -> Application.delete_env(:explorer, Explorer.Account)
      end
    end)

    :ets.new(@session_table, [:named_table, :public])
    profile = example("user-info")
    auth = build(:auth)

    auth = %{
      auth
      | info: %{
          auth.info
          | name: profile["name"],
            email: profile["email"],
            nickname: profile["nickname"],
            image: profile["avatar"]
        },
        extra: put_in(auth.extra, [Access.key(:raw_info), :user, "user_metadata", "web3_address_hash"], nil)
    }

    signed_in =
      conn
      |> SessionTransport.session([])
      |> fetch_session()
      |> Phoenix.Controller.put_format("json")
      |> Phoenix.Controller.put_view(UserView)
      |> AuthenticateController.put_auth_to_session(auth)

    assert decoded(signed_in, 200) == profile

    user = get_session(signed_in, :current_user)
    assert %{email_verified: true} = user
    conn = Plug.Test.put_req_cookie(conn, @session_key, signed_in.resp_cookies[@session_key].value)
    csrf = conn |> get("/api/account/v2/get_csrf")
    assert decoded(csrf, 200) == %{"message" => "ok"}
    [token] = get_resp_header(csrf, "x-bs-account-csrf")
    assert is_binary(token) and byte_size(token) > 0

    conn =
      csrf
      |> recycle()
      |> put_private(:plug_skip_csrf_protection, false)
      |> put_req_header("accept", "application/json")
      |> put_req_header("content-type", "application/json")
      |> put_req_header("x-csrf-token", token)

    {:ok, conn: conn, user: user}
  end

  test "account-api-documentation-version: every documented verb and route is registered" do
    guide = File.read!(@guide)
    refute guide =~ "/api/account/v1"
    refute guide =~ "Account.Api.V1"

    documented =
      Regex.scan(~r/\| (GET|POST|PUT|DELETE) \| `(\/api\/account\/v2[^`]*)`/, guide)
      |> Enum.map(fn [_, verb, path] -> {verb, String.trim_trailing(path, "/")} end)
      |> MapSet.new()

    registered =
      AccountRouter.__routes__()
      |> Enum.filter(&String.starts_with?(&1.path, "/v2/"))
      |> Enum.map(&{String.upcase(to_string(&1.verb)), "/api/account" <> String.trim_trailing(&1.path, "/")})
      |> MapSet.new()

    # The profile route is documented inline rather than in a resource table.
    assert guide =~ "`GET /api/account/v2/user/info`"
    assert MapSet.put(documented, {"GET", "/api/account/v2/user/info"}) == registered

    published =
      for spec <- [BlockScoutWeb.Specs.Private.spec(), BlockScoutWeb.Specs.Public.spec()],
          {path, item} <- spec.paths,
          String.starts_with?(path, "/account/v2/"),
          {verb, %OpenApiSpex.Operation{}} <- Map.from_struct(item) do
        path = Regex.replace(~r/\{([^}]+)\}/, path, ":\\1")
        {String.upcase(to_string(verb)), "/api" <> String.trim_trailing(path, "/")}
      end

    assert MapSet.new(published) == registered
    assert length(published) == MapSet.size(registered)

    for {verb, path} <- registered do
      path = String.replace(path, ~r/:[a-z_]+/, "1")
      local_path = String.replace_prefix(path, "/api/account", "")
      assert %{plug: controller} = Phoenix.Router.route_info(AccountRouter, verb, local_path, "example.com")
      assert Atom.to_string(controller) =~ "BlockScoutWeb.Account.API.V2."
    end

    assert Phoenix.Router.route_info(AccountRouter, "GET", "/v1/user/info", "example.com") == :error
  end

  test "account-api-documentation-version: published authentication examples satisfy current schemas" do
    spec = BlockScoutWeb.Specs.Private.spec()

    for {name, body} <- [
          {"send-otp", AccountSchemas.send_otp_request_body()},
          {"confirm-otp", AccountSchemas.confirm_otp_request_body()},
          {"wallet-auth", AccountSchemas.authenticate_via_wallet_request_body()}
        ] do
      schema = body.content["application/json"].schema
      assert OpenApiSpex.TestAssertions.assert_raw_schema(example(name), schema, spec)
    end

    user_schema = Map.fetch!(spec.components.schemas, AccountSchemas.User.schema().title)
    assert OpenApiSpex.TestAssertions.assert_raw_schema(example("user-info"), user_schema, spec)

    # Missing required schema fields are rejected before any provider is called.
    for path <- ["send_otp", "confirm_otp", "authenticate_via_wallet"] do
      response =
        build_conn() |> put_req_header("content-type", "application/json") |> post("/api/account/v2/" <> path, "{}")

      assert %{"errors" => errors} = decoded(response, 422)
      assert errors != []
    end
  end

  test "account-api-documentation-version: session profile CSRF and unauthorized contracts", %{conn: conn, user: user} do
    profile = conn |> recycle() |> get("/api/account/v2/user/info")
    assert BlockScoutWeb.Account.AuthController.current_user(profile) == user
    assert decoded(profile, 200) == example("user-info")

    for path <- ["/api/account/v2/user/info", "/api/account/v2/get_csrf"] do
      assert build_conn() |> get(path) |> decoded(401) == %{"message" => "Unauthorized"}
    end

    session = Map.put(user, :email_verified, false)

    unverified =
      build_conn()
      |> SessionTransport.session([])
      |> fetch_session()
      |> put_session(:current_user, session)
      |> send_resp(200, "")
      |> recycle()

    assert unverified |> get("/api/account/v2/user/info") |> decoded(403) == %{
             "message" => "Unverified email",
             "email" => example("user-info")["email"]
           }
  end

  test "account-api-documentation-version: documented address tag request reaches lookup and pagination", %{conn: conn} do
    path = "/api/account/v2/user/tags/address"
    params = example("address-tag")
    assert conn |> get(path) |> decoded(200) == example("empty-page")
    created = conn |> post(path, Jason.encode!(params)) |> decoded(200)
    assert Map.take(created, ["address_hash", "name"]) == params
    assert is_integer(created["id"])
    assert is_map(created["address"])
    assert conn |> get(path) |> decoded(200) == %{"items" => [created], "next_page_params" => nil}

    tags = conn |> get("/api/account/v2/tags/address/" <> params["address_hash"]) |> decoded(200)

    assert tags == %{
             "watchlist_names" => [],
             "common_tags" => [],
             "personal_tags" => [
               %{"label" => params["name"], "display_name" => params["name"], "address_hash" => params["address_hash"]}
             ]
           }

    assert build_conn() |> get("/api/account/v2/tags/address/" <> params["address_hash"]) |> decoded(200) ==
             example("address-tags-response")

    updated = conn |> put(path <> "/#{created["id"]}", Jason.encode!(%{name: "Renamed"})) |> decoded(200)
    assert updated == Map.put(created, "name", "Renamed")
    assert conn |> delete(path <> "/#{created["id"]}") |> decoded(200) == %{"message" => "OK"}
    assert conn |> get(path) |> decoded(200) == example("empty-page")
  end

  test "account-api-documentation-version: recorded transaction tag examples reach the combined consumer", %{conn: conn} do
    path = "/api/account/v2/user/tags/transaction"
    params = example("transaction-tag")
    assert conn |> post(path, Jason.encode!(params)) |> decoded(422) == example("missing-transaction")
    transaction = persist_recorded_transaction()
    assert to_string(transaction.hash) == params["transaction_hash"]
    assert is_nil(transaction.to_address_hash)

    created = conn |> post(path, Jason.encode!(params)) |> decoded(200)
    assert is_integer(created["id"])
    assert created == Map.put(example("transaction-tag-response"), "id", created["id"])

    page = example("transaction-tag-page") |> Map.put("items", [created])
    assert conn |> get(path) |> decoded(200) == page

    assert conn |> get("/api/account/v2/tags/transaction/#{transaction.hash}") |> decoded(200) ==
             example("transaction-tags-response")

    sender = Explorer.Repo.get!(Explorer.Chain.Address, transaction.from_address_hash)
    tag = insert(:address_tag, label: "recorded-sender", display_name: "Recorded sender")
    insert(:address_to_tag, address: sender, tag: tag)

    common_tags = [
      %{
        "label" => tag.label,
        "display_name" => tag.display_name,
        "address_hash" => to_string(transaction.from_address_hash)
      }
    ]

    expected = Map.put(example("transaction-tags-response"), "common_tags", common_tags)
    assert conn |> get("/api/account/v2/tags/transaction/#{transaction.hash}") |> decoded(200) == expected

    assert build_conn() |> get("/api/account/v2/tags/transaction/#{transaction.hash}") |> decoded(200) ==
             Map.put(expected, "personal_transaction_tag", nil)

    updated = conn |> put(path <> "/#{created["id"]}", Jason.encode!(%{name: "Renamed"})) |> decoded(200)
    assert updated == Map.put(created, "name", "Renamed")
    assert conn |> delete(path <> "/#{created["id"]}") |> decoded(200) == %{"message" => "OK"}
    assert conn |> get(path) |> decoded(200) == example("empty-page")
  end

  test "account-api-documentation-version: watchlist example preserves notification wire fields", %{conn: conn} do
    path = "/api/account/v2/user/watchlist"
    params = example("watchlist")
    assert conn |> get(path) |> decoded(200) == example("empty-page")
    created = conn |> post(path, Jason.encode!(params)) |> decoded(200)
    assert Map.take(created, Map.keys(params)) == params

    assert Enum.sort(Map.keys(created)) ==
             Enum.sort([
               "id",
               "name",
               "address_hash",
               "address",
               "address_balance",
               "exchange_rate",
               "notification_settings",
               "notification_methods",
               "tokens_fiat_value",
               "tokens_count",
               "tokens_overflow"
             ])

    assert conn |> post(path, Jason.encode!(params)) |> decoded(422) == example("duplicate-watchlist")
    page = conn |> get(path) |> decoded(200)
    assert page["next_page_params"] == nil
    assert [item] = page["items"]
    assert Map.take(item, Map.keys(params)) == params
    assert item["id"] == created["id"]

    updated_params = Map.put(params, "name", "Renamed")
    updated = conn |> put(path <> "/#{created["id"]}", Jason.encode!(updated_params)) |> decoded(200)
    assert Map.take(updated, Map.keys(params)) == updated_params
    assert conn |> delete(path <> "/#{created["id"]}") |> decoded(200) == %{"message" => "OK"}
    assert conn |> get(path) |> decoded(200) == example("empty-page")
  end

  test "account-api-documentation-version: API key example uses UUIDs bare arrays and the declared limit", %{conn: conn} do
    path = "/api/account/v2/user/api_keys"
    params = example("api-key")
    assert conn |> get(path) |> decoded(200) == []
    created = conn |> post(path, Jason.encode!(params)) |> decoded(200)
    assert {:ok, key} = Ecto.UUID.cast(created["api_key"])
    assert created == Map.put(example("api-key-response"), "api_key", key)
    assert conn |> get(path) |> decoded(200) == [created]
    updated = conn |> put(path <> "/" <> key, Jason.encode!(%{name: "Renamed"})) |> decoded(200)
    assert updated == Map.put(created, "name", "Renamed")

    for name <- ["Second", "Third"] do
      assert conn |> post(path, Jason.encode!(%{name: name})) |> decoded(200) |> Map.fetch!("name") == name
    end

    assert conn |> post(path, Jason.encode!(params)) |> decoded(422) == example("api-key-limit")
    assert conn |> delete(path <> "/" <> key) |> decoded(200) == %{"message" => "OK"}
    assert conn |> get(path) |> decoded(200) |> length() == 2
  end

  test "account-api-documentation-version: custom ABI example validates the indexed contract", %{conn: conn} do
    path = "/api/account/v2/user/custom_abis"
    params = example("custom-abi")

    insert(:contract_address,
      hash: params["contract_address_hash"],
      contract_code: Explorer.Factory.contract_code_info().bytecode
    )

    assert conn |> get(path) |> decoded(200) == []
    created = conn |> post(path, Jason.encode!(params)) |> decoded(200)
    assert Map.take(created, Map.keys(params)) == params
    assert Enum.sort(Map.keys(created)) == Enum.sort(["id", "name", "abi", "contract_address_hash", "contract_address"])
    assert is_integer(created["id"])
    assert is_map(created["contract_address"])
    assert conn |> get(path) |> decoded(200) == [created]
    updated = conn |> put(path <> "/#{created["id"]}", Jason.encode!(%{name: "Renamed"})) |> decoded(200)
    assert updated == Map.put(created, "name", "Renamed")

    for _ <- 2..15 do
      address = insert(:contract_address)
      body = Map.put(params, "contract_address_hash", to_string(address.hash))
      saved = conn |> post(path, Jason.encode!(body)) |> decoded(200)
      assert Map.take(saved, Map.keys(body)) == body
    end

    address = insert(:contract_address)
    over_limit = Map.put(params, "contract_address_hash", to_string(address.hash))
    assert conn |> post(path, Jason.encode!(over_limit)) |> decoded(422) == example("custom-abi-limit")
    assert conn |> get(path) |> decoded(200) |> length() == 15
    assert conn |> delete(path <> "/#{created["id"]}") |> decoded(200) == %{"message" => "OK"}
    assert conn |> get(path) |> decoded(200) |> length() == 14
  end

  defp example(name) do
    pattern = Regex.compile!("<!-- example:" <> name <> " -->\\s*```json\\s*(.*?)\\s*```", "s")
    assert [_, json] = Regex.run(pattern, File.read!(@guide))
    Jason.decode!(json)
  end

  defp decoded(conn, status), do: Phoenix.ConnTest.json_response(conn, status)

  defp persist_recorded_transaction do
    [recording] =
      @recordings
      |> Path.join("transaction_importer_import_saves_the_transaction.json")
      |> File.read!()
      |> Jason.decode!()

    %{"result" => transaction} = Jason.decode!(recording["response"]["body"])
    params = transaction |> EthereumJSONRPC.Transaction.to_elixir() |> EthereumJSONRPC.Transaction.elixir_to_params()

    [recording] =
      @recordings |> Path.join("transaction_importer_import_1_receipt.json") |> File.read!() |> Jason.decode!()

    %{"result" => receipt} = Jason.decode!(recording["response"]["body"])
    assert receipt["transactionHash"] == to_string(params.hash)
    receipt_params = receipt |> EthereumJSONRPC.Receipt.to_elixir() |> EthereumJSONRPC.Receipt.elixir_to_params()
    insert(:address, hash: params.from_address_hash)
    insert(:address, hash: receipt_params.created_contract_address_hash)
    block = insert(:block, hash: params.block_hash, number: params.block_number)

    params =
      params |> Map.merge(receipt_params) |> Map.merge(%{block_consensus: true, block_timestamp: block.timestamp})

    %Transaction{} |> Transaction.changeset(params) |> Explorer.Repo.insert!()
  end
end
