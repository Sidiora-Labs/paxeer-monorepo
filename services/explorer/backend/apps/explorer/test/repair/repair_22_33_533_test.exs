defmodule Explorer.Repair.Repair2233533Test.Provider do
  @moduledoc false
  import Plug.Conn

  def init(store), do: store

  def call(conn, store) do
    {:ok, raw, conn} = read_body(conn)
    body = if raw == "", do: %{}, else: Jason.decode!(raw)
    conn = fetch_query_params(conn)
    request = {conn.method, URI.decode(conn.request_path), conn.query_params, body, conn.req_headers}

    {status, response} =
      Agent.get_and_update(store, fn state ->
        state = %{state | events: state.events ++ [request]}
        {method, path, _, _, _} = request
        key = {method, path}
        {mode, failures} = Map.pop(state.failures, key)
        state = %{state | failures: failures}

        case mode do
          :refuse ->
            {{503, %{"message" => "Recorded provider refusal"}}, state}

          :ignore_patch ->
            {{200, state.users[Path.basename(path)]}, state}

          :apply_then_refuse ->
            {_, updated} = route(request, state)
            {{503, %{"message" => "Recorded lost response after acceptance"}}, updated}

          _ ->
            route(request, state)
        end
      end)

    conn |> put_resp_content_type("application/json") |> send_resp(status, Jason.encode!(response))
  end

  defp route({"GET", "/.well-known/jwks.json", _, _, _}, state), do: {{200, state.jwks}, state}

  defp route({"POST", "/oauth/token", _, body, _}, state) do
    if body["otp"] == "123456" and body["client_id"] == "recorded-client" do
      {{200, %{"access_token" => "recorded-access", "token_type" => "Bearer", "id_token" => state.token}}, state}
    else
      {{403, %{"error" => "invalid_grant"}}, state}
    end
  end

  defp route({method, "/api/v2/users" <> rest, query, body, headers}, state) do
    if {"authorization", "Bearer recorded-management-token"} in headers do
      users_request(method, rest, query, body, state)
    else
      {{401, %{"message" => "Missing management bearer"}}, state}
    end
  end

  defp users_request("GET", "", %{"q" => query}, _, state) do
    users = Enum.map(state.order, &state.users[&1]) |> Enum.reject(&is_nil/1)

    matches =
      cond do
        String.starts_with?(query, "user_metadata.explorer.user_id:") ->
          [_, uid] = Regex.run(~r/user_id:"([^"]+)"/, query)
          Enum.filter(users, &(get_in(&1, ["user_metadata", "explorer", "user_id"]) == uid))

        String.contains?(query, "AND NOT user_id:") ->
          [_, email, excluded] = Regex.run(~r/email:"([^"]+)" AND NOT user_id:"([^"]+)"/, query)
          Enum.filter(users, &(&1["email"] == email and &1["user_id"] != excluded))

        true ->
          []
      end

    {{200, matches}, state}
  end

  defp users_request("GET", "/" <> id, _, _, state) do
    if MapSet.member?(state.hidden, id) or is_nil(state.users[id]) do
      {{404, %{"statusCode" => 404, "message" => "The user does not exist."}}, state}
    else
      {{200, state.users[id]}, state}
    end
  end

  defp users_request("PATCH", "/" <> id, _, body, state) do
    if Enum.any?(Map.keys(body), &(&1 not in ["user_metadata", "app_metadata", "email_verified"])) do
      {{400, %{"message" => "Payload validation error: additional properties"}}, state}
    else
      user = merge_patch(Map.fetch!(state.users, id), body)
      {{200, user}, %{state | users: Map.put(state.users, id, user), hidden: MapSet.delete(state.hidden, id)}}
    end
  end

  defp users_request("POST", "/" <> rest, _, body, state) do
    [id, "identities"] = String.split(rest, "/")
    identity = Map.take(body, ["user_id", "provider"])
    user = Map.fetch!(state.users, id)

    if identity in user["identities"] do
      {{409, %{"message" => "Identity already linked"}}, state}
    else
      user = Map.update!(user, "identities", &(&1 ++ [identity]))
      secondary_id = identity["provider"] <> "|" <> identity["user_id"]
      users = state.users |> Map.put(id, user) |> Map.delete(secondary_id)

      token =
        if state.token do
          {:ok, claims} = Joken.peek_claims(state.token)
          {:ok, token, _claims} = Joken.encode_and_sign(Map.put(claims, "sub", id), state.signer)
          token
        end

      {{201, user["identities"]}, %{state | users: users, token: token}}
    end
  end

  defp merge_patch(original, patch) do
    Enum.reduce(patch, original, fn
      {key, nil}, acc ->
        Map.delete(acc, key)

      {key, value}, acc when is_map(value) ->
        Map.put(acc, key, merge_patch(Map.get(acc, key, %{}), value))

      {key, value}, acc ->
        Map.put(acc, key, value)
    end)
  end
end

defmodule Explorer.Repair.Repair2233533Test.Client do
  @moduledoc false
  use GenServer
  def start_link(_), do: GenServer.start_link(__MODULE__, nil)
  def init(state), do: {:ok, state}
  def handle_call({module, function, args}, _from, state), do: {:reply, apply(module, function, args), state}
end

defmodule Explorer.Repair.Repair2233533Test do
  use ExUnit.Case, async: false

  alias Explorer.Account
  alias Explorer.Account.{Identity, TagAddress}
  alias Explorer.Repo
  alias Explorer.ThirdPartyIntegrations.Auth0
  alias Explorer.ThirdPartyIntegrations.Auth0.{Internal, Legacy, Migrated}
  alias Explorer.Repair.Repair2233533Test.{Client, Provider}
  alias Ueberauth.Strategy.Auth0.OAuth

  @certs Path.expand("../../lib/mix/tasks/auth0_to_keycloak_migrate/fixtures/status", __DIR__)
  @email "recovery@example.com"
  @address "0x0000000000000000000000000000000000000125"

  setup do
    :ok = Ecto.Adapters.SQL.Sandbox.checkout(Repo.account_repo())
    replace_env(:oauth2, :adapter, Tesla.Adapter.Httpc)
    replace_env(:oauth2, :middleware, [])
    replace_env(:explorer, Account, Keyword.put(Application.get_env(:explorer, Account), :enabled, true))
    replace_env(:ueberauth, OAuth, [])

    key = JOSE.JWK.generate_key({:rsa, 2048})
    {_, public_key} = key |> JOSE.JWK.to_public() |> JOSE.JWK.to_map()
    public_key = Map.merge(public_key, %{"kid" => "recorded-rsa", "use" => "sig", "alg" => "RS256"})

    {_, private_key} = JOSE.JWK.to_map(key)
    signer = Joken.Signer.create("RS256", private_key, %{"kid" => "recorded-rsa"})

    store =
      start_supervised!(
        {Agent,
         fn ->
           %{
             users: %{},
             order: [],
             events: [],
             failures: %{},
             hidden: MapSet.new(),
             token: nil,
             jwks: %{"keys" => [public_key]},
             signer: signer
           }
         end}
      )

    ref = {__MODULE__, make_ref()}

    start_supervised!(
      {Plug.Cowboy,
       scheme: :https,
       plug: {Provider, store},
       options: [
         ref: ref,
         ip: {127, 0, 0, 1},
         port: 0,
         certfile: Path.join(@certs, "localhost-cert.pem"),
         keyfile: Path.join(@certs, "localhost-key.pem")
       ]}
    )

    config = [
      domain: "localhost:#{:ranch.get_port(ref)}",
      auth0_application_id: "explorer",
      client_id: "recorded-client",
      client_secret: "recorded-secret",
      request_opts: [ssl: [verify: :verify_peer, cacertfile: Path.join(@certs, "ca-cert.pem")]]
    ]

    Application.put_env(:ueberauth, OAuth, config)
    start_token_cache()
    context = %{store: store, key: key, issuer: "https://#{config[:domain]}/"}
    put_token(context, "email|fresh")
    context
  end

  test "auth0-token-validation: verified OTP claims reach the identity schema", context do
    user = user("email|fresh", %{"explorer" => %{"user_id" => "email|fresh"}})
    put_users(context, [user])
    assert {:ok, auth} = login()
    assert auth.uid == "email|fresh"
    assert {:ok, session} = Identity.find_or_create(auth)
    assert session.uid == auth.uid
    assert [identity] = Identity.find_identities([session.uid])
    assert identity.email == @email
    assert Enum.any?(events(context), &match?({"GET", "/.well-known/jwks.json", _, _, _}, &1))
  end

  test "auth0-token-validation: wrong issuer audience expiry signature and missing claims fail before account writes",
       context do
    valid = claims(context, "email|fresh")
    put_users(context, [user("email|fresh")])
    {_, private_key} = JOSE.JWK.to_map(context.key)
    signer = Joken.Signer.create("RS256", private_key, %{"kid" => "recorded-rsa"})
    {:ok, signed, _claims} = Joken.encode_and_sign(valid, signer)
    [header, payload, signature] = String.split(signed, ".")
    <<first, rest::binary>> = Base.url_decode64!(signature, padding: false)

    tampered =
      Enum.join([header, payload, Base.url_encode64(<<Bitwise.bxor(first, 1), rest::binary>>, padding: false)], ".")

    invalid_claims = [
      Map.put(valid, "iss", "https://untrusted.example/"),
      Map.put(valid, "aud", "another-client"),
      Map.put(valid, "exp", System.system_time(:second) - 1),
      Map.delete(valid, "exp"),
      Map.put(valid, "exp", "9999999999"),
      Map.put(valid, "iat", System.system_time(:second) + 300),
      Map.put(valid, "nbf", System.system_time(:second) + 300),
      Map.put(valid, "sub", ""),
      Map.put(valid, "aud", ["recorded-client", "other"]),
      Map.put(valid, "azp", "another-client")
    ]

    tokens =
      Enum.map(invalid_claims, fn c ->
        {:ok, token, _claims} = Joken.encode_and_sign(c, signer)
        token
      end)

    {:ok, confused, _claims} =
      Joken.encode_and_sign(valid, Joken.Signer.create("HS256", "recorded-secret", %{"kid" => "recorded-rsa"}))

    for token <- tokens ++ [tampered, confused, "not.a.jwt"] do
      Agent.update(context.store, &%{&1 | token: token, events: []})
      assert :error = login()
      refute Enum.any?(events(context), fn {_, path, _, _, _} -> String.starts_with?(path, "/api/v2/users") end)
      assert Identity.find_identities(["email|fresh"]) == []
    end
  end

  test "auth0-link-errors: remote link and verification refusals are propagated", context do
    legacy_config()
    source = user("email|new")
    primary = user("auth0|primary")
    put_users(context, [source, primary])
    put_token(context, source["user_id"])
    refuse(context, "POST", "/api/v2/users/auth0|primary/identities")
    assert :error = login()
    assert get_user(context, primary["user_id"])["email_verified"] == false

    refuse(context, "PATCH", "/api/v2/users/auth0|primary")
    assert :error = login()
    assert length(get_user(context, primary["user_id"])["identities"]) == 1
    assert {:ok, auth} = login()
    assert auth.uid == primary["user_id"]
    assert get_user(context, primary["user_id"])["email_verified"] == true
    assert count_requests(context, "POST", "/api/v2/users/auth0|primary/identities") == 2
  end

  test "auth0-link-errors: a durable intent resumes after local merge and client restart", context do
    legacy_config()
    source = user("email|new")
    primary = user("auth0|primary")
    secondary = user("auth0|secondary")
    put_users(context, [source, primary, secondary])
    put_token(context, source["user_id"])
    primary_identity = insert_identity(primary["user_id"])
    secondary_identity = insert_identity(secondary["user_id"])
    {:ok, hash} = Explorer.Chain.string_to_address_hash(@address)

    tag =
      Repo.account_repo().insert!(%TagAddress{
        identity_id: secondary_identity.id,
        name: "recorded chain account",
        address_hash: hash,
        address_hash_hash: @address
      })

    refuse(context, "POST", "/api/v2/users/auth0|primary/identities")

    client = open_client()
    assert :error = request(client, Auth0, :confirm_otp_and_get_auth, [@email, "123456", "127.0.0.1"])
    assert Repo.account_repo().get(Identity, secondary_identity.id) == nil
    assert Repo.account_repo().get!(TagAddress, tag.id).identity_id == primary_identity.id

    assert get_in(get_user(context, primary["user_id"]), ["app_metadata", "explorer_email_link"]) ==
             %{"identity_id" => "new", "provider" => "email", "user_ids" => ["auth0|primary", "auth0|secondary"]}

    GenServer.stop(client)

    Agent.update(context.store, &%{&1 | order: Enum.reverse(&1.order)})
    reopened = open_client()
    assert {:ok, auth} = request(reopened, Auth0, :confirm_otp_and_get_auth, [@email, "123456", "127.0.0.1"])
    assert auth.uid == primary["user_id"]
    assert {:ok, session} = Identity.find_or_create(auth)
    assert session.id == primary_identity.id
    assert Repo.account_repo().get!(TagAddress, tag.id).identity_id == session.id
    assert get_in(get_user(context, primary["user_id"]), ["app_metadata", "explorer_email_link"]) == nil
    GenServer.stop(reopened)
  end

  test "auth0-link-errors: an accepted link with a lost response is not posted twice after reopening", context do
    legacy_config()
    source = user("email|new")
    primary = user("auth0|primary")
    secondary = user("auth0|secondary")
    put_users(context, [source, primary, secondary])
    insert_identity(primary["user_id"])
    insert_identity(secondary["user_id"])
    refuse(context, "POST", "/api/v2/users/auth0|primary/identities", :apply_then_refuse)
    client = open_client()
    assert :error = request(client, Legacy, :process_email_user, [source])
    GenServer.stop(client)
    reopened = open_client()
    assert {:ok, recovered} = request(reopened, Legacy, :process_email_user, [source])
    assert recovered["user_id"] == primary["user_id"]
    assert recovered["email_verified"] == true
    assert count_requests(context, "POST", "/api/v2/users/auth0|primary/identities") == 1
    GenServer.stop(reopened)
  end

  test "auth0-link-envelope: destination readback prevents source loss and retry preserves other namespaces", context do
    {source, destination, metadata} = migrated_users(context)
    refuse(context, "PATCH", "/api/v2/users/email|destination", :ignore_patch)

    assert {:error, "Email link metadata readback failed"} =
             Internal.link_email("canonical-wallet", destination["user_id"], @email)

    assert get_in(get_user(context, source["user_id"]), ["user_metadata", "explorer"]) == metadata
    assert count_requests(context, "PATCH", "/api/v2/users/auth0|wallet") == 0
    assert {:ok, linked} = Internal.link_email("canonical-wallet", destination["user_id"], @email)
    assert Migrated.create_auth(linked).uid == "canonical-wallet"
    assert get_in(linked, ["user_metadata", "other"]) == %{"retained" => true}
    assert get_user(context, source["user_id"])["user_metadata"] == %{"other" => %{"retained" => true}}
    assert {:ok, ^linked} = Migrated.get_user_by_id_from_session("canonical-wallet")

    patches = for {"PATCH", "/api/v2/users/email|destination", _, body, _} <- events(context), do: body
    assert patches == List.duplicate(%{"user_metadata" => %{"explorer" => metadata}}, 2)
  end

  test "auth0-link-envelope: interrupted source cleanup recovers through OTP transport after restart", context do
    {source, destination, metadata} = migrated_users(context)
    put_token(context, destination["user_id"])
    refuse(context, "PATCH", "/api/v2/users/auth0|wallet")
    client = open_client()
    args = [%{uid: "canonical-wallet", email: nil}, @email, "123456", "127.0.0.1"]
    assert :error = request(client, Auth0, :link_email, args)
    assert get_in(get_user(context, destination["user_id"]), ["user_metadata", "explorer"]) == metadata
    assert get_in(get_user(context, source["user_id"]), ["user_metadata", "explorer"]) == metadata
    GenServer.stop(client)

    reopened = open_client()
    assert {:ok, auth} = request(reopened, Auth0, :link_email, args)
    assert auth.uid == "canonical-wallet"
    assert {:ok, session} = Identity.find_or_create(auth)
    assert session.uid == "canonical-wallet"
    assert {:ok, linked} = Migrated.get_user_by_id_from_session(session.uid)
    assert linked["user_id"] == destination["user_id"]
    assert {:ok, ^linked} = Internal.link_email(session.uid, destination["user_id"], @email)
    GenServer.stop(reopened)
  end

  test "auth0-new-user-id: verified fresh-user fallback persists the canonical session across reopen", context do
    put_users(context, [user("email|fresh", %{"other" => %{"retained" => true}})])
    Agent.update(context.store, &%{&1 | hidden: MapSet.new(["email|fresh"])})
    client = open_client()
    assert {:ok, auth} = request(client, Auth0, :confirm_otp_and_get_auth, [@email, "123456", "127.0.0.1"])
    assert auth.uid == "email|fresh"
    assert {:ok, session} = Identity.find_or_create(auth)
    assert session.uid == "email|fresh"

    assert get_in(get_user(context, auth.uid), ["user_metadata", "explorer"]) ==
             %{"user_id" => "email|fresh", "name" => "Recorded user", "nickname" => "recorded"}

    refute Map.has_key?(get_in(get_user(context, auth.uid), ["user_metadata", "explorer"]), "sub")
    GenServer.stop(client)

    reopened = open_client()
    assert {:ok, user} = request(reopened, Migrated, :get_user_by_id_from_session, [session.uid])
    assert user["user_id"] == auth.uid

    assert {:new, recovered_session} =
             request(reopened, Internal, :update_session_with_address_hash, [Map.delete(session, :address_hash)])

    assert recovered_session.uid == session.uid
    assert recovered_session.id == session.id
    assert {:ok, recovered_auth} = request(reopened, Auth0, :confirm_otp_and_get_auth, [@email, "123456", "127.0.0.1"])
    assert {:ok, recovered} = Identity.find_or_create(recovered_auth)
    assert recovered.id == session.id
    GenServer.stop(reopened)
  end

  defp user(id, metadata \\ %{}) do
    %{
      "user_id" => id,
      "email" => @email,
      "email_verified" => false,
      "name" => "Recorded user",
      "nickname" => "recorded",
      "identities" => [],
      "user_metadata" => metadata,
      "app_metadata" => %{}
    }
  end

  defp migrated_users(context) do
    metadata = %{"user_id" => "canonical-wallet", "web3_address_hash" => @address, "name" => "Recorded wallet"}
    source = user("auth0|wallet", %{"explorer" => metadata, "other" => %{"retained" => true}})
    destination = user("email|destination", %{"other" => %{"retained" => true}})
    put_users(context, [source, destination])
    {source, destination, metadata}
  end

  defp claims(context, subject) do
    %{
      "sub" => subject,
      "iss" => context.issuer,
      "aud" => "recorded-client",
      "iat" => System.system_time(:second),
      "exp" => System.system_time(:second) + 300,
      "name" => "Recorded user",
      "nickname" => "recorded"
    }
  end

  defp put_token(context, subject) do
    {_, key} = JOSE.JWK.to_map(context.key)

    {:ok, token, _claims} =
      Joken.encode_and_sign(claims(context, subject), Joken.Signer.create("RS256", key, %{"kid" => "recorded-rsa"}))

    Agent.update(context.store, &%{&1 | token: token})
  end

  defp put_users(context, users),
    do:
      Agent.update(
        context.store,
        &%{&1 | users: Map.new(users, fn u -> {u["user_id"], u} end), order: Enum.map(users, fn u -> u["user_id"] end)}
      )

  defp get_user(context, id), do: Agent.get(context.store, & &1.users[id])
  defp events(context), do: Agent.get(context.store, & &1.events)

  defp refuse(context, method, path, mode \\ :refuse),
    do: Agent.update(context.store, &%{&1 | failures: Map.put(&1.failures, {method, path}, mode)})

  defp count_requests(context, method, path),
    do: Enum.count(events(context), fn {m, p, _, _, _} -> m == method and p == path end)

  defp login, do: Auth0.confirm_otp_and_get_auth(@email, "123456", "127.0.0.1")

  defp insert_identity(uid) do
    {:ok, session} = uid |> user() |> Legacy.create_auth() |> Identity.find_or_create()
    Repo.account_repo().get!(Identity, session.id)
  end

  defp legacy_config do
    config = Application.get_env(:ueberauth, OAuth)
    Application.put_env(:ueberauth, OAuth, Keyword.put(config, :auth0_application_id, ""))
  end

  defp open_client do
    {:ok, client} = Client.start_link([])
    Ecto.Adapters.SQL.Sandbox.allow(Repo.account_repo(), self(), client)
    on_exit(fn -> if Process.alive?(client), do: GenServer.stop(client) end)
    client
  end

  defp request(client, module, function, args), do: GenServer.call(client, {module, function, args}, 15_000)

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

  defp start_token_cache do
    {:ok, listener} = :gen_tcp.listen(0, [:binary, active: false, packet: :line, ip: {127, 0, 0, 1}])
    {:ok, {_, port}} = :inet.sockname(listener)

    start_supervised!(
      {Task,
       fn ->
         {:ok, socket} = :gen_tcp.accept(listener)
         serve_token_cache(socket)
       end}
    )

    previous = Process.whereis(:redix)
    if previous, do: Process.unregister(:redix)
    {:ok, redix} = Redix.start_link(host: "127.0.0.1", port: port, name: :redix, sync_connect: true)
    Process.unlink(redix)

    on_exit(fn ->
      if Process.alive?(redix), do: GenServer.stop(redix)
      if previous && Process.alive?(previous), do: Process.register(previous, :redix)
      :gen_tcp.close(listener)
    end)
  end

  defp serve_token_cache(socket) do
    case :gen_tcp.recv(socket, 0) do
      {:ok, "*2\r\n"} ->
        {:ok, "$3\r\n"} = :gen_tcp.recv(socket, 0)
        {:ok, "GET\r\n"} = :gen_tcp.recv(socket, 0)
        {:ok, "$" <> _} = :gen_tcp.recv(socket, 0)
        {:ok, _key} = :gen_tcp.recv(socket, 0)
        token = "recorded-management-token"
        :ok = :gen_tcp.send(socket, "$#{byte_size(token)}\r\n#{token}\r\n")
        serve_token_cache(socket)

      {:error, :closed} ->
        :ok
    end
  end
end
