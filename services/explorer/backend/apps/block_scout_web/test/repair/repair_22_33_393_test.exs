defmodule BlockScoutWeb.Repair22_33_393Test do
  @moduledoc """
  Repair record 22.33.393 for audit 22.30.142.
  Finding 1: "finding 1: profile denies missing, expired and unverified sessions".
  Finding 2: "finding 2: API keys deny reads and mutations before entering the model".
  Finding 3: "finding 3: custom ABIs deny reads and mutations before entering the model".
  Finding 4: "finding 4: address tags deny reads and mutations before entering the model".
  Finding 5: "finding 5: transaction tags deny reads and mutations before entering the model".
  Finding 6: "finding 6: watchlist addresses deny reads and mutations before entering the model".
  Finding 7: "finding 7: watchlist denies reads before entering the model".
  Verified sessions additionally exercise real account persistence and ownership.
  The real callback produces sessions, transported through Plug.Session with its
  standard in-memory ETS store. Expiry evicts that store entry before the next
  request; the old cookie is retained. Reads traverse AccountRouter; denied
  mutations traverse the production controller pipeline, independently of router
  CSRF rejection. Authenticated mutations also traverse AccountRouter with a real
  CSRF token. Chain request values come from the committed transaction recording
  and SimpleStorage contract fixture. No component is replaced or mocked.
  Baseline verification was not run: the explicit bounded gate order requires
  implementation before hf-build and hf-done; no baseline failures are claimed.
  No proposal requires an out-of-scope wiring change. Spec and gotcha ledgers are
  outside this task's writable scope; this module carries the repair record.
  """

  use BlockScoutWeb.ConnCase, async: false

  alias BlockScoutWeb.Account.{ApiKeyController, AuthController, CustomABIController}
  alias BlockScoutWeb.Account.{TagAddressController, TagTransactionController}
  alias BlockScoutWeb.Account.{WatchlistAddressController, WatchlistController}
  alias BlockScoutWeb.Routers.AccountRouter
  alias Explorer.Account.{CustomABI, Identity, TagAddress, TagTransaction, Watchlist, WatchlistAddress}
  alias Explorer.Account.Api.Key
  alias Explorer.Repo

  @session_table :repair_22_33_393_sessions
  @session_key "repair_account_session"
  @models [Key, CustomABI, Identity, TagAddress, TagTransaction, Watchlist, WatchlistAddress]
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
    session = login(build(:auth))
    [recording] = @recording |> File.read!() |> Jason.decode!()
    %{"method" => "eth_getTransactionByHash", "params" => [hash]} = Jason.decode!(recording["request"]["body"])
    %{"result" => transaction} = Jason.decode!(recording["response"]["body"])
    assert transaction["hash"] == hash

    {:ok, session: session, recorded_transaction: transaction}
  end

  test "finding 1: profile denies missing, expired and unverified sessions", context do
    assert_denied(context, AuthController, [{:get, "/auth/profile", :profile, %{}}])
  end

  test "finding 2: API keys deny reads and mutations before entering the model", context do
    key = "f16340f1-0d56-40bc-a54a-b4fb32661da0"

    assert_denied(context, ApiKeyController, [
      {:get, "/api_key", :index, %{}},
      {:get, "/api_key/new", :new, %{}},
      {:get, "/api_key/#{key}/edit", :edit, %{"id" => key}},
      {:post, "/api_key", :create, %{"key" => %{"name" => "Recorded API key"}}},
      {:post, "/api_key", :create, %{}},
      {:put, "/api_key/#{key}", :update, %{"id" => key, "key" => %{"value" => key, "name" => "Renamed"}}},
      {:patch, "/api_key/#{key}", :update, %{"id" => key, "key" => %{"value" => key, "name" => "Renamed"}}},
      {:delete, "/api_key/#{key}", :delete, %{"id" => key}}
    ])
  end

  test "finding 3: custom ABIs deny reads and mutations before entering the model", context do
    abi = custom_abi_params(context.recorded_transaction["to"])

    assert_denied(context, CustomABIController, [
      {:get, "/custom_abi", :index, %{}},
      {:get, "/custom_abi/new", :new, %{}},
      {:get, "/custom_abi/1/edit", :edit, %{"id" => "1"}},
      {:post, "/custom_abi", :create, %{"custom_abi" => abi}},
      {:post, "/custom_abi", :create, %{}},
      {:put, "/custom_abi/1", :update, %{"id" => "1", "custom_abi" => abi}},
      {:patch, "/custom_abi/1", :update, %{"id" => "1", "custom_abi" => abi}},
      {:put, "/custom_abi/1", :update, %{"id" => "1"}},
      {:delete, "/custom_abi/1", :delete, %{"id" => "1"}}
    ])
  end

  test "finding 4: address tags deny reads and mutations before entering the model", context do
    params = %{"name" => "Recorded sender", "address_hash" => context.recorded_transaction["from"]}

    assert_denied(context, TagAddressController, [
      {:get, "/tag_address", :index, %{}},
      {:get, "/tag_address/new", :new, %{}},
      {:post, "/tag_address", :create, %{"tag_address" => params}},
      {:post, "/tag_address", :create, %{}},
      {:delete, "/tag_address/1", :delete, %{"id" => "1"}}
    ])
  end

  test "finding 5: transaction tags deny reads and mutations before entering the model", context do
    params = %{"name" => "Recorded transaction", "transaction_hash" => context.recorded_transaction["hash"]}

    assert_denied(context, TagTransactionController, [
      {:get, "/tag_transaction", :index, %{}},
      {:get, "/tag_transaction/new", :new, %{}},
      {:post, "/tag_transaction", :create, %{"tag_transaction" => params}},
      {:post, "/tag_transaction", :create, %{}},
      {:delete, "/tag_transaction/1", :delete, %{"id" => "1"}}
    ])
  end

  test "finding 6: watchlist addresses deny reads and mutations before entering the model", context do
    params = watchlist_params(context.recorded_transaction["from"])

    assert_denied(context, WatchlistAddressController, [
      {:get, "/watchlist_address/new", :new, %{}},
      {:get, "/watchlist_address/1/edit", :edit, %{"id" => "1"}},
      {:post, "/watchlist_address", :create, %{"watchlist_address" => params}},
      {:put, "/watchlist_address/1", :update, %{"id" => "1", "watchlist_address" => params}},
      {:patch, "/watchlist_address/1", :update, %{"id" => "1", "watchlist_address" => params}},
      {:delete, "/watchlist_address/1", :delete, %{"id" => "1"}}
    ])
  end

  test "finding 7: watchlist denies reads before entering the model", context do
    assert_denied(context, WatchlistController, [{:get, "/watchlist", :show, %{}}])
  end

  test "verified callback sessions assign the identity and scope API key mutations", %{session: session} do
    profile = route(session, :get, "/auth/profile")
    assert profile.assigns.current_user == session.user
    assert html_response(profile, 200) =~ session.user.email

    other = login(build(:auth))

    assert redirected_to(route(other, :post, "/api_key", %{"key" => %{"name" => "Other identity key"}})) ==
             api_key_path(build_conn(), :index)

    [other_key] = Key.get_api_keys_by_identity_id(other.user.id)

    created = route(session, :post, "/api_key", %{"key" => %{"name" => "Owner key"}})
    assert redirected_to(created) == api_key_path(created, :index)
    assert created.assigns.current_user == session.user
    [key] = Key.get_api_keys_by_identity_id(session.user.id)
    assert key.identity_id == session.user.id

    listing = route(session, :get, "/api_key") |> html_response(200)
    assert listing =~ "Owner key"
    refute listing =~ "Other identity key"

    assert html_response(route(session, :get, "/api_key/new"), 200) =~ "form"
    assert html_response(route(session, :get, "/api_key/#{key.value}/edit"), 200) =~ "Owner key"

    for target <- [key.value, other_key.value] do
      updated = route(session, :put, "/api_key/#{target}", %{"key" => %{"value" => target, "name" => "Renamed key"}})
      assert redirected_to(updated) == api_key_path(updated, :index)
    end

    assert Repo.account_repo().get!(Key, key.value).name == "Renamed key"
    assert Repo.account_repo().get!(Key, other_key.value).name == "Other identity key"

    for target <- [other_key.value, key.value] do
      deleted = route(session, :delete, "/api_key/#{target}")
      assert redirected_to(deleted) == api_key_path(deleted, :index)
    end

    assert Repo.account_repo().get(Key, key.value) == nil
    assert Repo.account_repo().get!(Key, other_key.value).identity_id == other.user.id
  end

  test "verified sessions persist and read custom ABIs and recorded chain tags", context do
    session = context.session
    contract = insert(:smart_contract)
    params = custom_abi_params(to_string(contract.address_hash)) |> Map.put("abi", Jason.encode!(contract.abi))

    created = route(session, :post, "/custom_abi", %{"custom_abi" => params})
    assert redirected_to(created) == custom_abi_path(created, :index)
    [abi] = CustomABI.get_custom_abis_by_identity_id(session.user.id)
    assert abi.identity_id == session.user.id
    assert to_string(abi.address_hash) == params["address_hash"]
    assert html_response(route(session, :get, "/custom_abi"), 200) =~ params["name"]
    assert html_response(route(session, :get, "/custom_abi/new"), 200) =~ "form"
    assert html_response(route(session, :get, "/custom_abi/#{abi.id}/edit"), 200) =~ params["name"]

    updated = route(session, :patch, "/custom_abi/#{abi.id}", %{"custom_abi" => Map.put(params, "name", "Updated ABI")})
    assert redirected_to(updated) == custom_abi_path(updated, :index)
    assert Repo.account_repo().get!(CustomABI, abi.id).name == "Updated ABI"

    transaction = persist_transaction(context.recorded_transaction)

    for {path, key, tag_params, model} <- [
          {"/tag_address", "tag_address",
           %{"name" => "Recorded sender", "address_hash" => to_string(transaction.from_address_hash)}, TagAddress},
          {"/tag_transaction", "tag_transaction",
           %{"name" => "Recorded transaction", "transaction_hash" => to_string(transaction.hash)}, TagTransaction}
        ] do
      response = route(session, :post, path, %{key => tag_params})
      assert response.status == 302
      tag = Repo.account_repo().get_by!(model, identity_id: session.user.id)
      assert tag.name == tag_params["name"]
      assert html_response(route(session, :get, path), 200) =~ tag.name
      assert html_response(route(session, :get, path <> "/new"), 200) =~ "form"
      assert route(session, :delete, path <> "/#{tag.id}").status == 302
      assert Repo.account_repo().get(model, tag.id) == nil
    end

    assert route(session, :delete, "/custom_abi/#{abi.id}").status == 302
    assert Repo.account_repo().get(CustomABI, abi.id) == nil
  end

  test "verified watchlist sessions use their assigned watchlist through create read update delete", context do
    session = context.session
    params = watchlist_params(context.recorded_transaction["from"])
    created = route(session, :post, "/watchlist_address", %{"watchlist_address" => params})
    assert redirected_to(created) == watchlist_path(created, :show)
    address = Repo.account_repo().get_by!(WatchlistAddress, watchlist_id: session.user.watchlist_id)
    assert to_string(address.address_hash) == params["address_hash"]
    assert html_response(route(session, :get, "/watchlist_address/new"), 200) =~ "form"
    assert html_response(route(session, :get, "/watchlist_address/#{address.id}/edit"), 200) =~ params["name"]
    assert html_response(route(session, :get, "/watchlist"), 200) =~ params["name"]

    updated =
      route(session, :put, "/watchlist_address/#{address.id}", %{
        "watchlist_address" => Map.put(params, "name", "Updated watch")
      })

    assert redirected_to(updated) == watchlist_path(updated, :show)
    assert Repo.account_repo().get!(WatchlistAddress, address.id).name == "Updated watch"
    assert route(session, :delete, "/watchlist_address/#{address.id}").status == 302
    assert Repo.account_repo().get(WatchlistAddress, address.id) == nil
  end

  test "the account call observer detects real schema execution" do
    calls = account_calls(fn -> Key.changeset() end)
    assert Enum.any?(calls, fn {module, function, _args} -> module == Key and function == :changeset end)
  end

  defp assert_denied(context, controller, requests) do
    unverified_auth = put_in(build(:auth).extra.raw_info.user["email_verified"], false)
    unverified = login(unverified_auth)
    assert unverified.user.email_verified == false
    expired = context.session
    :ok = Plug.Session.ETS.delete(build_conn(), expired.cookie, @session_table)
    assert {nil, %{}} == Plug.Session.ETS.get(build_conn(), expired.cookie, @session_table)

    for session <- [nil, expired, unverified] do
      calls =
        account_calls(fn ->
          for {method, path, action, params} <- requests do
            response =
              if method == :get do
                route(session, method, path, params)
              else
                conn =
                  session_conn(session, method, path, params) |> fetch_session() |> Phoenix.Controller.fetch_flash([])

                controller.call(conn, controller.init(action))
              end

            assert response.halted
            assert redirected_to(response) == Explorer.Repo.ConfigHelper.network_path()
            assert html_response(response, 302) =~ "redirected"
            refute Map.has_key?(response.assigns, :current_user)
            assert response.private.phoenix_controller == controller
            assert response.private.phoenix_action == action
          end
        end)

      assert calls == []
    end
  end

  defp login(auth) do
    Plug.CSRFProtection.delete_csrf_token()

    conn =
      session_conn(nil, :get, "/auth/auth0/callback", %{})
      |> fetch_session()
      |> Phoenix.Controller.fetch_flash([])
      |> Plug.CSRFProtection.call(Plug.CSRFProtection.init([]))

    csrf = Plug.CSRFProtection.get_csrf_token()
    conn = conn |> assign(:ueberauth_auth, auth) |> AuthController.callback(%{})

    assert conn.status == 302
    assert Plug.CSRFProtection.valid_state_and_csrf_token?(get_session(conn, "_csrf_token"), csrf)
    user = get_session(conn, :current_user)
    assert is_integer(user.id)
    %{cookie: conn.resp_cookies[@session_key].value, csrf: csrf, user: user}
  end

  defp session_conn(session, method, path, params) do
    conn =
      Plug.Test.conn(method, path, params)
      |> Map.put(:secret_key_base, BlockScoutWeb.Endpoint.config(:secret_key_base))
      |> put_private(:phoenix_endpoint, BlockScoutWeb.Endpoint)

    conn = if session, do: Plug.Test.put_req_cookie(conn, @session_key, session.cookie), else: conn
    opts = Plug.Session.init(store: :ets, key: @session_key, table: @session_table)
    Plug.Session.call(conn, opts)
  end

  defp route(session, method, path, params \\ %{}) do
    Plug.CSRFProtection.delete_csrf_token()
    conn = session_conn(session, method, path, params)
    conn = if session, do: put_req_header(conn, "x-csrf-token", session.csrf), else: conn
    AccountRouter.call(conn, AccountRouter.init([]))
  end

  defp account_calls(fun) do
    for model <- @models do
      Code.ensure_loaded!(model)
      :erlang.trace_pattern({model, :_, :_}, true, [:local])
    end

    collector = Task.async(fn -> collect_calls([]) end)
    :erlang.trace(self(), true, [:call, {:tracer, collector.pid}])

    try do
      fun.()
      ref = :erlang.trace_delivered(self())
      assert_receive {:trace_delivered, _, ^ref}
    after
      :erlang.trace(self(), false, [:call])
      for model <- @models, do: :erlang.trace_pattern({model, :_, :_}, false, [:local])
      send(collector.pid, :collect)
    end

    Task.await(collector)
  end

  defp collect_calls(calls) do
    receive do
      {:trace, _pid, :call, mfa} -> collect_calls([mfa | calls])
      :collect -> Enum.reverse(calls)
    end
  end

  defp custom_abi_params(address) do
    %{
      "name" => "Recorded contract ABI",
      "address_hash" => address,
      "abi" =>
        ~s([{"type":"function","name":"name","inputs":[],"outputs":[{"type":"string"}],"stateMutability":"view"}])
    }
  end

  defp watchlist_params(address) do
    %{
      "address_hash" => address,
      "name" => "Recorded sender watch",
      "watch_coin_input" => "true",
      "watch_coin_output" => "true",
      "watch_erc_20_input" => "true",
      "watch_erc_20_output" => "true",
      "watch_erc_721_input" => "true",
      "watch_erc_721_output" => "true",
      "notify_email" => "true"
    }
  end

  defp persist_transaction(recorded) do
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

    %Explorer.Chain.Transaction{} |> Explorer.Chain.Transaction.changeset(params) |> Repo.insert!()
  end
end
