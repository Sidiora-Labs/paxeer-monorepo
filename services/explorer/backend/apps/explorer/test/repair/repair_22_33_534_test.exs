defmodule Explorer.Repair.Repair2233534Test.RedisStore do
  @moduledoc false
  use GenServer

  @consume_script """
  if redis.call('GET', KEYS[1]) == ARGV[1] then
    redis.call('DEL', KEYS[1], KEYS[2])
    return 1
  end
  return 0
  """

  def start_link(_), do: GenServer.start_link(__MODULE__, nil)
  def port(store), do: GenServer.call(store, :port)
  def snapshot(store), do: GenServer.call(store, :snapshot)
  def put(store, key, value), do: GenServer.call(store, {:put, key, value})
  def fail(store, command), do: GenServer.call(store, {:fail, command})
  def barrier(store, key, count), do: GenServer.call(store, {:barrier, key, count})
  def replace_on_consume(store, key, value), do: GenServer.call(store, {:replace_on_consume, key, value})

  def init(_) do
    {:ok, listener} = :gen_tcp.listen(0, [:binary, active: false, packet: :raw, ip: {127, 0, 0, 1}])
    {:ok, {_, port}} = :inet.sockname(listener)
    store = self()
    {:ok, acceptor} = Task.start_link(fn -> accept(listener, store) end)

    {:ok,
     %{
       listener: listener,
       acceptor: acceptor,
       port: port,
       keys: %{},
       commands: [],
       failure: nil,
       barrier: nil,
       replacement: nil
     }}
  end

  def terminate(_, state) do
    :gen_tcp.close(state.listener)
    Process.exit(state.acceptor, :shutdown)
  end

  def handle_call(:port, _, state), do: {:reply, state.port, state}
  def handle_call(:snapshot, _, state), do: {:reply, Map.take(state, [:keys, :commands]), state}
  def handle_call({:put, key, value}, _, state), do: {:reply, :ok, %{state | keys: Map.put(state.keys, key, value)}}
  def handle_call({:fail, command}, _, state), do: {:reply, :ok, %{state | failure: command}}
  def handle_call({:barrier, key, count}, _, state), do: {:reply, :ok, %{state | barrier: {key, count, []}}}
  def handle_call({:replace_on_consume, key, value}, _, state), do: {:reply, :ok, %{state | replacement: {key, value}}}

  def handle_call({:command, [command | _] = args, socket}, _, state) do
    state = %{state | commands: state.commands ++ [args]}

    cond do
      state.failure == command ->
        {:reply, "-ERR recorded #{command} refusal\r\n", %{state | failure: nil}}

      command == "GET" and state.barrier != nil and elem(state.barrier, 0) == Enum.at(args, 1) ->
        {key, count, waiting} = state.barrier
        waiting = [socket | waiting]

        if length(waiting) == count do
          Enum.each(waiting, &:gen_tcp.send(&1, bulk(state.keys[key])))
          {:reply, nil, %{state | barrier: nil}}
        else
          {:reply, nil, %{state | barrier: {key, count, waiting}}}
        end

      true ->
        {response, state} = execute(args, state)
        {:reply, response, state}
    end
  end

  defp execute(["GET", key], state), do: {bulk(state.keys[key]), state}

  defp execute(["SET", key, value, "EX", ttl], state) do
    if String.to_integer(ttl) > 0 do
      {"+OK\r\n", %{state | keys: Map.put(state.keys, key, value)}}
    else
      {"-ERR invalid expire time\r\n", state}
    end
  end

  defp execute(["DEL" | keys], state) do
    count = Enum.count(keys, &Map.has_key?(state.keys, &1))
    {":#{count}\r\n", %{state | keys: Map.drop(state.keys, keys)}}
  end

  defp execute(["EVAL", @consume_script, "2", key, attempts_key, expected], state) do
    state =
      case state.replacement do
        {^key, replacement} -> %{state | keys: Map.put(state.keys, key, replacement), replacement: nil}
        nil -> state
      end

    if state.keys[key] == expected do
      {":1\r\n", %{state | keys: Map.drop(state.keys, [key, attempts_key])}}
    else
      {":0\r\n", state}
    end
  end

  defp execute(["INCR", key], state) do
    value = String.to_integer(Map.get(state.keys, key, "0")) + 1
    {":#{value}\r\n", %{state | keys: Map.put(state.keys, key, Integer.to_string(value))}}
  end

  defp execute(["EXPIRE", key, _], state), do: {if(Map.has_key?(state.keys, key), do: ":1\r\n", else: ":0\r\n"), state}
  defp execute(_, state), do: {"-ERR unsupported fixture command or unrecognised script\r\n", state}

  defp bulk(nil), do: "$-1\r\n"
  defp bulk(value), do: ["$#{byte_size(value)}\r\n", value, "\r\n"]

  defp accept(listener, store) do
    case :gen_tcp.accept(listener) do
      {:ok, socket} ->
        {:ok, _} = Task.start_link(fn -> serve(socket, store) end)
        accept(listener, store)

      {:error, :closed} ->
        :ok
    end
  end

  defp serve(socket, store) do
    case line(socket) do
      {:ok, "*" <> count} ->
        args =
          for _ <- 1..String.to_integer(count) do
            {:ok, "$" <> length} = line(socket)
            size = String.to_integer(length)
            {:ok, <<value::binary-size(size), "\r\n">>} = :gen_tcp.recv(socket, size + 2, 15_000)
            value
          end

        if response = GenServer.call(store, {:command, args, socket}), do: :gen_tcp.send(socket, response)
        serve(socket, store)

      {:error, :closed} ->
        :ok
    end
  end

  defp line(socket, bytes \\ "") do
    case :gen_tcp.recv(socket, 1, 15_000) do
      {:ok, "\n"} -> {:ok, String.trim_trailing(bytes, "\r")}
      {:ok, byte} -> line(socket, bytes <> byte)
      error -> error
    end
  end
end

defmodule Explorer.Repair.Repair2233534Test.Provider do
  @moduledoc false
  import Plug.Conn

  def init(store), do: store

  def call(conn, store) do
    {:ok, body, conn} = read_body(conn)
    conn = fetch_query_params(conn)
    request = {conn.method, conn.request_path, conn.query_params, body, conn.req_headers}

    {status, response} =
      Agent.get_and_update(store, fn state ->
        state = %{state | requests: state.requests ++ [request]}
        {route(request, state), state}
      end)

    response_body = if status == 204, do: "", else: Jason.encode!(response)
    conn |> put_resp_content_type("application/json") |> send_resp(status, response_body)
  end

  defp route({"POST", "/realms/recorded/protocol/openid-connect/token", _, body, headers}, state) do
    if URI.decode_query(body) == %{
         "grant_type" => "client_credentials",
         "client_id" => "recorded-client",
         "client_secret" => "recorded-secret"
       } and
         {"content-type", "application/x-www-form-urlencoded"} in headers do
      state.token_response
    else
      {400, %{"error" => "invalid_client"}}
    end
  end

  defp route({"GET", "/admin/realms/recorded/users", query, _, headers}, state) do
    if authorised?(headers) and query == %{"email" => state.user["email"], "exact" => "true"} do
      {200, [state.user]}
    else
      {401, %{"error" => "unauthorized"}}
    end
  end

  defp route({"GET", "/admin/realms/recorded/users/recorded-user", _, _, headers}, state) do
    if authorised?(headers), do: {200, state.user}, else: {401, %{"error" => "unauthorized"}}
  end

  defp route({"PUT", "/admin/realms/recorded/users/recorded-user", _, body, headers}, state) do
    if authorised?(headers) and Jason.decode!(body) == %{"email" => state.user["email"], "emailVerified" => true},
      do: {204, nil},
      else: {401, %{"error" => "unauthorized"}}
  end

  defp route({"GET", "/config", _, _, _}, state), do: {200, state.config}

  defp route({"GET", "/rpc/0x0000000000000000000000000000000000000125", %{"chain" => "125"}, _, headers}, _) do
    if {"x-recorded-chain", "125"} in headers,
      do: {200, %{"jsonrpc" => "2.0", "id" => 1, "result" => "0x7d"}},
      else: {400, %{"error" => "missing chain header"}}
  end

  defp route(_, _), do: {404, %{"error" => "unrecorded request"}}
  defp authorised?(headers), do: {"authorization", "Bearer recorded-admin-token"} in headers
end

defmodule Explorer.Repair.Repair2233534Test.Client do
  @moduledoc false
  use GenServer
  def start_link(_), do: GenServer.start_link(__MODULE__, nil)
  def init(state), do: {:ok, state}

  def handle_call({function, args}, _, state) do
    {:reply, apply(Explorer.ThirdPartyIntegrations.Keycloak, function, args), state}
  end
end

defmodule Explorer.Repair.Repair2233534Test do
  use ExUnit.Case, async: false

  alias Explorer.Account.Identity
  alias Explorer.Repair.Repair2233534Test.{Client, Provider, RedisStore}
  alias Explorer.ThirdPartyIntegrations.{Keycloak, UniversalProxy}
  alias Explorer.{Helper, Repo, Vault}

  @email "recovery@example.com"
  @address "0x0000000000000000000000000000000000000125"

  setup do
    :ok = Ecto.Adapters.SQL.Sandbox.checkout(Repo.account_repo())
    replace_env(:explorer, :http_client, Explorer.HttpClient.HTTPoison)
    replace_env(:explorer, Explorer.Mailer, adapter: Bamboo.TestAdapter)
    account_config = Application.get_env(:explorer, Explorer.Account, [])

    replace_env(
      :explorer,
      Explorer.Account,
      Keyword.put(account_config, :sendgrid, sender: "noreply@example.com", otp_template: "recorded-otp-template")
    )

    previous_cache = :persistent_term.get(:universal_proxy_config, :missing)
    :persistent_term.erase(:universal_proxy_config)

    on_exit(fn ->
      :persistent_term.erase(:universal_proxy_config)
      if previous_cache != :missing, do: :persistent_term.put(:universal_proxy_config, previous_cache)
    end)

    store = start_supervised!({RedisStore, []})
    previous_redix = Process.whereis(:redix)
    if previous_redix, do: Process.unregister(:redix)

    on_exit(fn ->
      if redix = Process.whereis(:redix), do: GenServer.stop(redix)
      if previous_redix && Process.alive?(previous_redix), do: Process.register(previous_redix, :redix)
    end)

    redix = open_redix(store)

    provider =
      start_supervised!(
        {Agent,
         fn ->
           %{
             requests: [],
             config: nil,
             token_response:
               {200, %{"access_token" => "recorded-admin-token", "expires_in" => 300, "token_type" => "Bearer"}},
             user: %{
               "id" => "recorded-user",
               "username" => @email,
               "email" => @email,
               "emailVerified" => true,
               "enabled" => true,
               "attributes" => %{"address" => [@address]}
             }
           }
         end}
      )

    ref = {__MODULE__, make_ref()}

    start_supervised!(
      {Plug.Cowboy, scheme: :http, plug: {Provider, provider}, options: [port: 0, ref: ref, ip: {127, 0, 0, 1}]}
    )

    url = "http://127.0.0.1:#{:ranch.get_port(ref)}"

    replace_env(:explorer, Keycloak,
      domain: url,
      realm: "recorded",
      client_id: "recorded-client",
      client_secret: "recorded-secret",
      email_webhook_url: nil
    )

    replace_env(:explorer, UniversalProxy, config_url: url <> "/config", config_json: nil)
    %{store: store, redix: redix, provider: provider, url: url}
  end

  test "keycloak-cache-recovery: unreadable persisted token is evicted after reopen and refreshed once", context do
    RedisStore.put(context.store, token_key(), Vault.encrypt!("recorded-admin-token"))
    client = open_client()
    assert {:ok, %{"id" => "recorded-user"}} = request(client, :get_user, ["recorded-user"])

    old_config = [
      vault: Vault,
      ciphers: [default: {Cloak.Ciphers.AES.GCM, tag: "AES.GCM.V1", key: :binary.copy(<<42>>, 32)}]
    ]

    {:ok, unreadable} = Cloak.Vault.encrypt(old_config, "old-admin-token", :default)
    RedisStore.put(context.store, token_key(), unreadable)
    assert {:ok, ^unreadable} = Redix.command(:redix, ["GET", token_key()])
    refute match?({:ok, value} when is_binary(value), Vault.decrypt(unreadable))
    GenServer.stop(client)
    GenServer.stop(context.redix)
    _reopened_redix = open_redix(context.store)
    reopened = open_client()
    assert {:ok, %{"id" => "recorded-user"}} = request(reopened, :get_user, ["recorded-user"])
    assert {:ok, %{"id" => "recorded-user"}} = request(reopened, :get_user, ["recorded-user"])
    assert {:ok, "recorded-admin-token"} = Vault.decrypt(RedisStore.snapshot(context.store).keys[token_key()])
    assert token_requests(context) == 1
    commands = RedisStore.snapshot(context.store).commands
    assert ["DEL", token_key()] in commands
    assert Enum.any?(commands, &match?(["SET", _, _, "EX", "299"], &1))
    assert Enum.count(events(context), &match?({"GET", "/admin/realms/recorded/users/recorded-user", _, _, _}, &1)) == 3
  end

  test "keycloak-cache-recovery: provider and cache refusals and malformed token envelopes are bounded", context do
    for response <- [
          {503, %{"error" => "temporarily_unavailable"}},
          {200, %{"access_token" => "token", "expires_in" => 1}},
          {200, %{"access_token" => "token", "expires_in" => "300"}},
          {200, %{"expires_in" => 300}},
          {200, %{"access_token" => "", "expires_in" => 300}}
        ] do
      RedisStore.put(context.store, token_key(), "unreadable-cached-token")
      Agent.update(context.provider, &%{&1 | requests: [], token_response: response})
      assert :error = Keycloak.get_user("recorded-user")
      assert token_requests(context) == 1
      assert length(events(context)) == 1
      refute Map.has_key?(RedisStore.snapshot(context.store).keys, token_key())
    end

    RedisStore.put(context.store, token_key(), "unreadable-cached-token")
    RedisStore.fail(context.store, "DEL")
    Agent.update(context.provider, &%{&1 | requests: []})
    assert :error = Keycloak.get_user("recorded-user")
    assert events(context) == []
    assert RedisStore.snapshot(context.store).keys[token_key()] == "unreadable-cached-token"
    assert {:ok, 1} = Redix.command(:redix, ["DEL", token_key()])

    Agent.update(
      context.provider,
      &%{&1 | token_response: {200, %{"access_token" => "recorded-admin-token", "expires_in" => 300}}}
    )

    RedisStore.fail(context.store, "SET")
    assert :error = Keycloak.get_user("recorded-user")
    assert token_requests(context) == 1
    refute Enum.any?(events(context), &match?({"GET", _, _, _, _}, &1))
  end

  test "keycloak-otp-consumption: concurrent confirmations consume once and produce one persisted identity", context do
    otp = seed_otp(context)
    RedisStore.put(context.store, attempts_key(), "1")
    RedisStore.barrier(context.store, otp_key(), 12)
    tasks = for _ <- 1..12, do: Task.async(fn -> Keycloak.confirm_otp_and_get_auth(@email, otp, "127.0.0.1") end)
    results = Enum.map(tasks, &Task.await(&1, 15_000))
    assert [{:ok, auth}] = Enum.filter(results, &match?({:ok, _}, &1))
    assert Enum.count(results, &(&1 == {:error, "Verification code has expired."})) == 11
    assert auth.uid == "recorded-user"
    assert auth.info.email == @email
    assert auth.extra.raw_info.address_hash == @address
    assert {:ok, session} = Identity.find_or_create(auth)
    assert session.uid == auth.uid
    assert [identity] = Identity.find_identities([session.uid])
    assert identity.email == @email
    refute Map.has_key?(RedisStore.snapshot(context.store).keys, otp_key())
    refute Map.has_key?(RedisStore.snapshot(context.store).keys, attempts_key())
    assert length(events(context)) == 1
    assert Enum.count(RedisStore.snapshot(context.store).commands, &match?(["EVAL" | _], &1)) == 12
  end

  test "keycloak-otp-consumption: Redis consumption failure authorises neither login nor email linking", context do
    otp = seed_otp(context)

    for operation <- [:login, :link] do
      RedisStore.fail(context.store, "EVAL")

      result =
        case operation do
          :login -> Keycloak.confirm_otp_and_get_auth(@email, otp, "127.0.0.1")
          :link -> Keycloak.link_email(%{uid: "recorded-user", email: nil}, @email, otp, "127.0.0.1")
        end

      assert result == :error
      assert {:ok, ^otp} = Vault.decrypt(RedisStore.snapshot(context.store).keys[otp_key()])
      assert events(context) == []
    end

    assert {:ok, auth} = Keycloak.link_email(%{uid: "recorded-user", email: nil}, @email, otp, "127.0.0.1")
    assert auth.info.email == @email
    assert {:error, "Verification code has expired."} = Keycloak.confirm_otp_and_get_auth(@email, otp, "127.0.0.1")
    assert Enum.map(events(context), &elem(&1, 0)) == ["PUT", "GET"]
  end

  test "keycloak-otp-consumption: replacement code survives a stale comparison and wrong attempts retain their limit",
       context do
    otp = seed_otp(context)
    replacement = Vault.encrypt!("654321")
    RedisStore.replace_on_consume(context.store, otp_key(), replacement)
    assert {:error, "Verification code has expired."} = Keycloak.confirm_otp_and_get_auth(@email, otp, "127.0.0.1")
    assert RedisStore.snapshot(context.store).keys[otp_key()] == replacement
    assert events(context) == []

    for _ <- 1..2 do
      assert {:error, "Wrong verification code."} = Keycloak.confirm_otp_and_get_auth(@email, "000000", "127.0.0.1")
    end

    assert {:error, "Too many wrong verification code attempts. Please request a new code."} =
             Keycloak.confirm_otp_and_get_auth(@email, "000000", "127.0.0.1")

    assert {:error, "Verification code has expired."} = Keycloak.confirm_otp_and_get_auth(@email, "654321", "127.0.0.1")
    assert events(context) == []
  end

  test "proxy-config-shape: decoded values and nested invalid shapes fail before routing or caching", context do
    valid = platform(context.url)

    invalid =
      [
        nil,
        42,
        true,
        "config",
        [],
        %{},
        %{"platforms" => []},
        %{"platforms" => nil},
        %{"platforms" => %{"recorded" => []}}
      ] ++
        Enum.map(
          [
            Map.put(valid, "endpoints", []),
            put_in(valid, ["endpoints", "base"], 9),
            put_in(valid, ["endpoints", "base", "path"], []),
            put_in(valid, ["endpoints", "base", "path"], "https://other.example.com"),
            put_in(valid, ["endpoints", "base", "path"], "//other.example.com"),
            put_in(valid, ["endpoints", "base", "method"], 4),
            put_in(valid, ["endpoints", "base", "params"], %{}),
            put_in(valid, ["endpoints", "base", "params"], [42]),
            put_in(valid, ["endpoints", "base", "params"], [
              %{"location" => "path", "type" => "chain_id_dependent", "mapping" => []}
            ]),
            put_in(valid, ["endpoints", "base", "params"], [
              %{"location" => "path", "name" => "address", "value" => 125}
            ]),
            Map.put(valid, "api_key", %{"location" => "header", "param_name" => []})
          ],
          &%{"platforms" => %{"recorded" => &1}}
        )

    for config <- invalid do
      Agent.update(context.provider, &%{&1 | config: config, requests: []})
      assert {"Invalid config: " <> message, 422} = UniversalProxy.api_request(%{"platform_id" => "recorded"})
      assert byte_size(message) > 0
      assert :persistent_term.get(:universal_proxy_config, :missing) == :missing
      assert [{"GET", "/config", _, _, _}] = events(context)
      Application.put_env(:explorer, UniversalProxy, config_json: Jason.encode!(config))
      assert {"Invalid config: " <> _, 422} = UniversalProxy.api_request(%{"platform_id" => "recorded"})
      assert length(events(context)) == 1
      Application.put_env(:explorer, UniversalProxy, config_url: context.url <> "/config")
    end
  end

  test "proxy-config-shape: malformed URLs fail and a corrected config reaches the real upstream", context do
    for url <- [
          "api.example.com",
          "/relative",
          "//api.example.com",
          "ftp://api.example.com",
          "file:///tmp/rpc",
          "https://",
          "https://api.example.com:invalid",
          "http://api.example.com:70000",
          "http://bad host",
          "https://user:password@api.example.com",
          "https://api.example.com/#fragment"
        ] do
      config = %{"platforms" => %{"recorded" => platform(url)}}
      Application.put_env(:explorer, UniversalProxy, config_json: Jason.encode!(config))
      assert {"Invalid config: " <> _, 422} = UniversalProxy.api_request(%{"platform_id" => "recorded"})
      assert :persistent_term.get(:universal_proxy_config, :missing) == :missing
      assert events(context) == []
    end

    Application.put_env(:explorer, UniversalProxy, config_url: context.url <> "/config")
    config = %{"platforms" => %{"recorded" => platform(context.url)}}
    Agent.update(context.provider, &%{&1 | config: config})
    params = %{"platform_id" => "recorded", "address" => @address, "chain_id" => "125"}
    assert {%{"jsonrpc" => "2.0", "id" => 1, "result" => "0x7d"}, 200} = UniversalProxy.api_request(params)
    assert :persistent_term.get(:universal_proxy_config) == Jason.encode!(config)
    assert {%{"result" => "0x7d"}, 200} = UniversalProxy.api_request(params)
    assert Enum.count(events(context), &match?({"GET", "/config", _, _, _}, &1)) == 1
    assert Enum.count(events(context), &match?({"GET", "/rpc/" <> _, %{"chain" => "125"}, _, _}, &1)) == 2

    for scheme <- ["http", "https", "ws", "wss"] do
      config = %{"platforms" => %{"recorded" => platform(scheme <> "://api.example.com")}}
      Application.put_env(:explorer, UniversalProxy, config_json: Jason.encode!(config))
      parsed = UniversalProxy.parse_proxy_params(params)
      assert Atom.to_string(parsed.protocol) == scheme
      assert parsed.url == scheme <> "://api.example.com/rpc/" <> @address <> "?chain=125"
    end
  end

  defp platform(url) do
    %{
      "base_url" => url,
      "endpoints" => %{
        "base" => %{
          "path" => "/rpc/:address",
          "method" => "GET",
          "params" => [
            %{"location" => "path", "type" => "address"},
            %{"location" => "query", "name" => "chain", "type" => "chain_id"},
            %{"location" => "header", "name" => "x-recorded-chain", "value" => "125"}
          ]
        }
      }
    }
  end

  defp seed_otp(context) do
    RedisStore.put(context.store, token_key(), Vault.encrypt!("recorded-admin-token"))
    assert :ok = Keycloak.send_otp(@email, "127.0.0.1")
    assert_receive {:delivered_email, email}
    otp = email.private.send_grid_template.dynamic_template_data["otp"]
    assert is_binary(otp) and byte_size(otp) == 6
    assert {:ok, ^otp} = Vault.decrypt(RedisStore.snapshot(context.store).keys[otp_key()])
    otp
  end

  defp token_key, do: Helper.redis_key("keycloak:recorded-client:admin_token")
  defp otp_key, do: Helper.redis_key("recorded-client:otp:#{@email}")
  defp attempts_key, do: Helper.redis_key("recorded-client:otp_attempts:#{@email}")
  defp events(context), do: Agent.get(context.provider, & &1.requests)
  defp token_requests(context), do: Enum.count(events(context), &match?({"POST", _, _, _, _}, &1))

  defp open_redix(store) do
    {:ok, redix} = Redix.start_link(host: "127.0.0.1", port: RedisStore.port(store), name: :redix, sync_connect: true)
    Process.unlink(redix)
    redix
  end

  defp open_client do
    {:ok, client} = Client.start_link([])
    on_exit(fn -> if Process.alive?(client), do: GenServer.stop(client) end)
    client
  end

  defp request(client, function, args), do: GenServer.call(client, {function, args}, 15_000)

  defp replace_env(app, key, value) do
    previous = Application.fetch_env(app, key)
    Application.put_env(app, key, value)

    on_exit(fn ->
      case previous do
        {:ok, old} -> Application.put_env(app, key, old)
        :error -> Application.delete_env(app, key)
      end
    end)
  end
end
