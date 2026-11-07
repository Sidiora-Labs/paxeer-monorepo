defmodule BlockScoutWeb.Repair22_33_461Test do
  @moduledoc """
  Task 22.33.461 / audit 22.30.148 repair record.
  Finding blueprint-account-api-version is proven by the cases prefixed with
  that finding's ID below: published routes and origin, profile and session
  errors, recorded-chain tags, watchlist example, API keys and custom ABIs.
  The base already contains the regenerated v2 blueprint; this task binds it
  to production routing, controller responses and account persistence.
  Chain hashes come from the committed transaction-importer RPC recording,
  decoded by EthereumJSONRPC and persisted through the production changeset.
  Requests use the production account router, controllers and views, with
  Plug.Session.ETS as the in-memory session peer and real CSRF protection.
  Initial qualification exposed the unavailable endpoint Redis session peer;
  this test supplies a local session store without replacing account code.
  Wiring request (domain 14, fm-web-explorer-bridge): keep
  services/explorer/backend/apps/block_scout_web/API blueprint.md generated
  from current account/v2 controller contracts, a configurable explorer origin
  and explicitly retired v1 examples; that publication path is outside scope.
  Baseline verification was not run: the explicit gate order requires complete
  implementation before hf-build and hf-done, and forbids a separate verify.
  Before this continuation, attempt 1 ran all five original cases: each failed
  on missing OpenAPI account v2 declarations (including /account/v2/user/info).
  This continuation retains those cases and adds exact method/path coverage.
  The runner's private gate ledger records
  the verified revision, exact command, exit status and evidence location.
  """

  use BlockScoutWeb.ConnCase, async: false

  alias BlockScoutWeb.Routers.AccountRouter
  alias Explorer.Account.{CustomABI, Identity, TagAddress, TagTransaction, WatchlistAddress}
  alias Explorer.Account.Api.Key
  alias Explorer.Repo

  @session_table :repair_22_33_461_sessions
  @session_key "repair_account_session"

  @blueprint Path.expand("../../API blueprint.md", __DIR__)
  @recording Path.expand(
               "../../../explorer/test/support/fixture/vcr_cassettes/transaction_importer_binds_internal_transactions.json",
               __DIR__
             )

  setup do
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
    {:ok, user} = Identity.find_or_create(build(:auth))
    conn = session_conn(user)
    [recording] = @recording |> File.read!() |> Jason.decode!()
    %{"method" => "eth_getTransactionByHash", "params" => [hash]} = Jason.decode!(recording["request"]["body"])
    %{"result" => recorded} = Jason.decode!(recording["response"]["body"])
    assert recorded["hash"] == hash

    params = recorded |> EthereumJSONRPC.Transaction.to_elixir() |> EthereumJSONRPC.Transaction.elixir_to_params()
    insert(:address, hash: params.from_address_hash)
    insert(:address, hash: params.to_address_hash)
    block = insert(:block, hash: params.block_hash, number: params.block_number)

    params =
      Map.merge(params, %{
        block_consensus: true,
        block_timestamp: block.timestamp,
        status: :ok,
        gas_used: 21_000,
        cumulative_gas_used: 21_000
      })

    transaction = %Explorer.Chain.Transaction{} |> Explorer.Chain.Transaction.changeset(params) |> Repo.insert!()
    {:ok, conn: conn, user: user, transaction: transaction, blueprint: File.read!(@blueprint)}
  end

  test "blueprint-account-api-version: published routes and configurable origin", %{blueprint: blueprint, conn: conn} do
    assert blueprint =~ "# Explorer Account API v2"
    assert blueprint =~ "EXPLORER_ORIGIN"
    assert blueprint =~ "$EXPLORER_ORIGIN/api/account/v2/get_csrf"
    assert blueprint =~ "The former account/v1 examples and their upstream POA host are retired."
    refute blueprint =~ "blockscout.com/poa/core"
    refute blueprint =~ "/api/account/v1"

    [_, host] = Regex.run(~r/^HOST: (\S+)$/m, blueprint)
    assert %URI{scheme: scheme, host: hostname} = URI.parse(host)
    assert scheme in ["http", "https"]
    assert is_binary(hostname)

    groups = [
      {"Authentication", "/api/account/v2"},
      {"Private tags", "/api/account/v2/user/tags"},
      {"Watchlist", ""},
      {"API keys", ""},
      {"Custom ABIs", ""}
    ]

    for {group, prefix} <- groups do
      rows = Regex.scan(~r/^\| (GET|POST|PUT|DELETE) \| (.+?) \|/m, section(blueprint, group))
      assert rows != []

      for [_, method, paths] <- rows, [_, path] <- Regex.scan(~r/`([^`]+)`/, paths) do
        path = if String.starts_with?(path, "/api/"), do: path, else: prefix <> path
        path = Regex.replace(~r/\{[^}]+\}/, path, "1")
        local_path = String.replace_prefix(path, "/api/account", "")
        route = Phoenix.Router.route_info(AccountRouter, method, local_path, hostname)
        assert %{plug: controller} = route
        assert Atom.to_string(controller) =~ "BlockScoutWeb.Account.API.V2."
      end
    end

    response = conn |> account_request(:get, "/api/account/v2/user/info")
    assert response.private.phoenix_controller == BlockScoutWeb.Account.API.V2.UserController
    assert json_response(response, 200)["email"]
  end

  test "blueprint-account-api-version: OpenAPI methods equal registered v2 routes and use configured origin" do
    endpoint = Application.fetch_env!(:block_scout_web, BlockScoutWeb.Endpoint)
    on_exit(fn -> Application.put_env(:block_scout_web, BlockScoutWeb.Endpoint, endpoint) end)

    url = [scheme: "https", host: "explorer.example", http: [port: 8443], path: "/scan/"]
    Application.put_env(:block_scout_web, BlockScoutWeb.Endpoint, Keyword.put(endpoint, :url, url))
    public = BlockScoutWeb.Specs.Public.spec()
    private = BlockScoutWeb.Specs.Private.spec()

    for spec <- [public, private] do
      assert [%OpenApiSpex.Server{url: "https://explorer.example:8443/scan/api"}] = spec.servers
    end

    registered =
      for route <- AccountRouter.__routes__(), String.starts_with?(route.path, "/v2/") do
        path = Regex.replace(~r/:([^\/]+)/, "/account" <> route.path, "{\\1}")
        {route.verb, path}
      end
      |> MapSet.new()

    documented =
      for spec <- [public, private],
          {path, item} <- spec.paths,
          String.starts_with?(path, "/account/"),
          method <- [:get, :post, :put, :delete, :patch, :head, :options, :trace],
          operation = Map.get(item, method),
          not is_nil(operation) do
        assert %OpenApiSpex.Operation{operationId: id, responses: responses} = operation
        assert is_binary(id) and id != ""

        assert %OpenApiSpex.Response{content: %{"application/json" => %OpenApiSpex.MediaType{schema: schema}}} =
                 responses[200] || responses["200"]

        assert match?(%OpenApiSpex.Schema{}, schema) or match?(%OpenApiSpex.Reference{}, schema)
        {method, path}
      end

    assert MapSet.new(documented) == registered
    assert length(documented) == MapSet.size(registered)
    assert Map.has_key?(public.paths, "/account/v2/tags/address/{address_hash}")
    assert Map.has_key?(public.paths, "/account/v2/tags/transaction/{transaction_hash}")
    assert Map.has_key?(private.paths, "/account/v2/user/info")
    refute Map.has_key?(public.paths, "/account/v2/user/info")
    refute Map.has_key?(private.paths, "/account/v2/tags/address/{address_hash}")
  end

  test "blueprint-account-api-version: profile schema and session errors", %{
    conn: conn,
    user: user,
    blueprint: blueprint
  } do
    profile_example = section(blueprint, "User information") |> json_example("+ Response 200 (application/json)")
    profile = conn |> account_request(:get, "/api/account/v2/user/info") |> json_response(200)
    assert Enum.sort(Map.keys(profile)) == Enum.sort(Map.keys(profile_example))

    assert profile == %{
             "name" => user.name,
             "email" => user.email,
             "avatar" => user.avatar,
             "nickname" => user.nickname,
             "address_hash" => user.address_hash
           }

    csrf_response = account_request(conn, :get, "/api/account/v2/get_csrf")
    assert json_response(csrf_response, 200) == %{"message" => "ok"}
    [csrf] = get_resp_header(csrf_response, "x-bs-account-csrf")
    assert Plug.CSRFProtection.valid_state_and_csrf_token?(get_session(csrf_response, "_csrf_token"), csrf)
    assert blueprint =~ "x-bs-account-csrf"
    assert blueprint =~ "x-csrf-token"

    unauthorized = build_conn() |> account_request(:get, "/api/account/v2/user/info") |> json_response(401)
    assert unauthorized == section(blueprint, "User information") |> json_example("+ Response 401 (application/json)")

    unverified = %{user | email_verified: false}
    forbidden = session_conn(unverified)

    assert forbidden |> account_request(:get, "/api/account/v2/user/info") |> json_response(403) ==
             %{"message" => "Unverified email", "email" => user.email}

    config = Application.get_env(:explorer, Explorer.Account)
    Application.put_env(:explorer, Explorer.Account, Keyword.put(config, :enabled, false))

    assert conn |> account_request(:get, "/api/account/v2/user/info") |> json_response(404) ==
             %{"message" => "Account functionality is disabled"}

    assert blueprint =~ "Account functionality is disabled"
    assert blueprint =~ "Unverified email"
  end

  test "blueprint-account-api-version: recorded-chain private tags and public lookup shapes", context do
    conn = csrf_conn(context.conn)
    address = to_string(context.transaction.from_address_hash)
    transaction_hash = to_string(context.transaction.hash)
    group = section(context.blueprint, "Private tags")
    params = json_example(group, "+ Body") |> Map.put("address_hash", address)
    assert_request_schema("/account/v2/user/tags/address", :post, params)
    address_tag = conn |> account_request(:post, "/api/account/v2/user/tags/address/", params) |> json_response(200)
    assert_documented_keys(address_tag, group, ["id", "address_hash", "address", "name"])
    assert address_tag["address_hash"] == address
    assert address_tag["name"] == params["name"]
    assert Repo.account_repo().get!(TagAddress, address_tag["id"]).identity_id == context.user.id

    transaction_tag =
      conn
      |> account_request(:post, "/api/account/v2/user/tags/transaction/", %{
        "transaction_hash" => transaction_hash,
        "name" => "Recorded transfer"
      })
      |> json_response(200)

    assert_documented_keys(transaction_tag, group, ["id", "transaction_hash", "name"])
    assert transaction_tag["transaction_hash"] == transaction_hash
    assert Repo.account_repo().get!(TagTransaction, transaction_tag["id"]).identity_id == context.user.id

    for {kind, tag} <- [{"address", address_tag}, {"transaction", transaction_tag}] do
      listing = conn |> account_request(:get, "/api/account/v2/user/tags/#{kind}/") |> json_response(200)
      assert listing == %{"items" => [tag], "next_page_params" => nil}
      assert group =~ "`items` and `next_page_params`"
    end

    lookup = conn |> account_request(:get, "/api/account/v2/tags/address/#{address}") |> json_response(200)
    assert_documented_keys(lookup, group, ["personal_tags", "watchlist_names", "common_tags"])
    assert Enum.map(lookup["personal_tags"], & &1["label"]) == [params["name"]]
    lookup = conn |> account_request(:get, "/api/account/v2/tags/transaction/#{transaction_hash}") |> json_response(200)
    assert lookup["personal_transaction_tag"] == %{"label" => "Recorded transfer"}

    for {kind, tag} <- [{"address", address_tag}, {"transaction", transaction_tag}] do
      assert conn |> account_request(:delete, "/api/account/v2/user/tags/#{kind}/#{tag["id"]}") |> json_response(200) ==
               %{
                 "message" => "OK"
               }

      assert conn |> account_request(:get, "/api/account/v2/user/tags/#{kind}/") |> json_response(200) ==
               %{"items" => [], "next_page_params" => nil}
    end
  end

  test "blueprint-account-api-version: published watchlist request round trips recorded address", context do
    conn = csrf_conn(context.conn)
    group = section(context.blueprint, "Watchlist")
    params = json_example(group, "+ Request (application/json)")
    params = Map.put(params, "address_hash", to_string(context.transaction.from_address_hash))
    assert_request_schema("/account/v2/user/watchlist", :post, params)
    created = conn |> account_request(:post, "/api/account/v2/user/watchlist", params) |> json_response(200)

    assert_documented_keys(created, group, [
      "id",
      "address",
      "address_hash",
      "name",
      "address_balance",
      "exchange_rate",
      "notification_settings",
      "notification_methods",
      "tokens_fiat_value",
      "tokens_count",
      "tokens_overflow"
    ])

    assert Map.take(created, Map.keys(params)) == params
    stored = Repo.account_repo().get!(WatchlistAddress, created["id"])
    assert stored.watchlist_id == context.user.watchlist_id
    assert stored.watch_erc_1155_input == params["notification_settings"]["ERC-721"]["incoming"]
    assert stored.watch_erc_1155_output == params["notification_settings"]["ERC-721"]["outcoming"]

    listing = conn |> account_request(:get, "/api/account/v2/user/watchlist") |> json_response(200)
    assert %{"items" => [item], "next_page_params" => nil} = listing
    assert Map.take(item, ["id" | Map.keys(params)]) == Map.take(created, ["id" | Map.keys(params)])
    assert item["tokens_count"] == 0
    assert item["tokens_overflow"] == false

    assert conn |> account_request(:delete, "/api/account/v2/user/watchlist/#{created["id"]}") |> json_response(200) ==
             %{
               "message" => "OK"
             }

    assert Repo.account_repo().get(WatchlistAddress, created["id"]) == nil
  end

  test "blueprint-account-api-version: API key and custom ABI schemas persist and delete", context do
    conn = csrf_conn(context.conn)
    keys_group = section(context.blueprint, "API keys")
    abis_group = section(context.blueprint, "Custom ABIs")

    assert conn |> account_request(:get, "/api/account/v2/user/api_keys") |> json_response(200) ==
             json_example(keys_group, "+ Response 200 (application/json)")

    assert conn |> account_request(:get, "/api/account/v2/user/custom_abis") |> json_response(200) ==
             json_example(abis_group, "+ Response 200 (application/json)")

    assert_request_schema("/account/v2/user/api_keys", :post, %{"name" => "Recorded reader"})

    key =
      conn
      |> account_request(:post, "/api/account/v2/user/api_keys", %{"name" => "Recorded reader"})
      |> json_response(200)

    assert_documented_keys(key, keys_group, ["api_key", "name"])
    assert {:ok, _} = Ecto.UUID.cast(key["api_key"])
    assert key["name"] == "Recorded reader"
    assert Repo.account_repo().get!(Key, key["api_key"]).identity_id == context.user.id
    assert conn |> account_request(:get, "/api/account/v2/user/api_keys") |> json_response(200) == [key]

    params = build(:custom_abi)
    assert_request_schema("/account/v2/user/custom_abis", :post, params)
    abi = conn |> account_request(:post, "/api/account/v2/user/custom_abis", params) |> json_response(200)
    assert_documented_keys(abi, abis_group, ["id", "contract_address_hash", "contract_address", "name", "abi"])
    assert Map.take(abi, Map.keys(params)) == params
    assert Repo.account_repo().get!(CustomABI, abi["id"]).identity_id == context.user.id
    assert conn |> account_request(:get, "/api/account/v2/user/custom_abis") |> json_response(200) == [abi]

    for {path, id} <- [{"api_keys", key["api_key"]}, {"custom_abis", abi["id"]}] do
      assert conn |> account_request(:delete, "/api/account/v2/user/#{path}/#{id}") |> json_response(200) == %{
               "message" => "OK"
             }

      assert conn |> account_request(:get, "/api/account/v2/user/#{path}") |> json_response(200) == []
    end
  end

  defp assert_request_schema(path, method, params) do
    spec = BlockScoutWeb.Specs.Private.spec()
    operation = spec.paths |> Map.fetch!(path) |> Map.fetch!(method)
    schema = operation.requestBody.content["application/json"].schema
    OpenApiSpex.TestAssertions.assert_raw_schema(params, schema, spec)
  end

  defp session_conn(user) do
    build_conn()
    |> Map.put(:secret_key_base, BlockScoutWeb.Endpoint.config(:secret_key_base))
    |> Plug.Session.call(session_options())
    |> fetch_session()
    |> put_session(:current_user, user)
    |> send_resp(200, "")
  end

  defp session_options do
    Plug.Session.init(store: :ets, key: @session_key, table: @session_table)
  end

  defp account_request(previous, method, path, params \\ %{}) do
    Plug.CSRFProtection.delete_csrf_token()

    conn =
      Plug.Test.conn(method, path, Jason.encode!(params))
      |> Plug.Test.recycle_cookies(previous)
      |> put_req_header("accept", "application/json")
      |> put_req_header("content-type", "application/json")
      |> Map.put(:secret_key_base, BlockScoutWeb.Endpoint.config(:secret_key_base))
      |> put_private(:phoenix_endpoint, BlockScoutWeb.Endpoint)
      |> Plug.Session.call(session_options())

    conn =
      case get_req_header(previous, "x-csrf-token") do
        [csrf] -> put_req_header(conn, "x-csrf-token", csrf)
        [] -> conn
      end

    # Match the path/script_name split produced by ApiRouter's account forward.
    ["api", "account" | account_path] = conn.path_info
    conn = %{conn | path_info: account_path, script_name: ["api", "account"]}
    AccountRouter.call(conn, AccountRouter.init([]))
  end

  defp csrf_conn(conn) do
    response = account_request(conn, :get, "/api/account/v2/get_csrf")
    assert json_response(response, 200) == %{"message" => "ok"}
    [csrf] = get_resp_header(response, "x-bs-account-csrf")
    response |> recycle() |> put_req_header("x-csrf-token", csrf)
  end

  defp section(blueprint, group) do
    [_, rest] = String.split(blueprint, "# Group #{group}\n", parts: 2)
    rest |> String.split("\n# Group ", parts: 2) |> hd()
  end

  defp json_example(section, marker) do
    [_, rest] = String.split(section, marker, parts: 2)
    [_, json] = Regex.run(~r/\n\n((?:[ ]{8,}[^\n]*\n?)+)/, rest)
    Jason.decode!(json)
  end

  defp assert_documented_keys(response, documentation, keys) do
    assert Enum.sort(Map.keys(response)) == Enum.sort(keys)
    for key <- keys, do: assert(documentation =~ "`#{key}`")
  end
end
