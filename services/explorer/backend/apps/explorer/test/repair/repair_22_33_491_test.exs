defmodule Explorer.Repair.Repair2233491Test do
  use ExUnit.Case, async: false

  import Ecto.Query
  import Explorer.Factory

  alias Ecto.Adapters.SQL.Sandbox
  alias Explorer.Account

  alias Explorer.Account.{
    Authentication,
    Auth0ToKeycloakMigration,
    Identity,
    MigrationCheckpoint,
    TagAddress,
    TagTransaction,
    Watchlist
  }

  alias Explorer.Account.Api.Key
  alias Explorer.Repo
  alias Explorer.ThirdPartyIntegrations.{Auth0, Keycloak}
  alias Ueberauth.Auth
  alias Ueberauth.Auth.{Extra, Info}
  alias Ueberauth.Strategy.Auth0.OAuth

  @address "0x000000000000000000000000000000000000aBcD"
  @target "b963421b-8e90-4537-8be8-32f96d741859"
  @other_target "393c8e6a-166b-4d5f-9acf-2014b3dd64ed"
  @recording Path.expand("../../../ethereum_jsonrpc/test/support/fixture/geth/trace/struct_logger.json", __DIR__)

  # These peers implement recorded HTTP/RESP exchanges; migration, encryption,
  # schemas, Ecto.Multi, Redis client and authentication are production modules.
  defmodule KeycloakRecording do
    @behaviour Plug
    import Ecto.Query
    def init(options), do: options

    def call(conn, {observer, deletion}) do
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      send(observer, {:keycloak_request, conn.request_path, body})

      response =
        case conn.request_path do
          "/realms/repair/protocol/openid-connect/token" ->
            %{"access_token" => "recorded-admin-token", "expires_in" => 300}

          "/admin/realms/repair/partialImport" ->
            %{"ifResourceExists" => "SKIP", "users" => users} = Jason.decode!(body)

            Enum.each(users, fn user ->
              true = user["enabled"]
              true = user["emailVerified"]
            end)

            if deletion do
              Explorer.Repo.account_repo().delete_all(from(i in Explorer.Account.Identity, where: i.id == ^deletion))
            end

            %{
              "results" =>
                Enum.map(users, fn user ->
                  %{
                    "action" => "ADDED",
                    "resourceType" => "USER",
                    "resourceName" => user["username"],
                    "id" =>
                      if(user["username"] == "john@blockscout.com",
                        do: "b963421b-8e90-4537-8be8-32f96d741859",
                        else: "393c8e6a-166b-4d5f-9acf-2014b3dd64ed"
                      )
                  }
                end)
            }
        end

      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.send_resp(200, Jason.encode!(response))
    end
  end

  defmodule RedisNamespace do
    use GenServer

    def start_link(options), do: GenServer.start_link(__MODULE__, options)
    def port(server), do: GenServer.call(server, :port)
    def snapshot(server), do: GenServer.call(server, :snapshot)

    @impl true
    def init(options) do
      {:ok, listener} = :gen_tcp.listen(0, [:binary, active: false, reuseaddr: true, ip: {127, 0, 0, 1}])
      {:ok, {_ip, port}} = :inet.sockname(listener)
      owner = self()
      acceptor = spawn_link(fn -> accept(listener, owner) end)
      keys = ["125_session:first", "125_session:second", "999_session:other"]

      {:ok,
       %{
         listener: listener,
         acceptor: acceptor,
         port: port,
         keys: MapSet.new(keys),
         scan_calls: 0,
         delete_calls: 0,
         commands: [],
         fail_scan_at: Keyword.get(options, :fail_scan_at),
         fail_delete_at: Keyword.get(options, :fail_delete_at)
       }}
    end

    @impl true
    def handle_call(:port, _from, state), do: {:reply, state.port, state}
    def handle_call(:snapshot, _from, state), do: {:reply, state, state}

    def handle_call({:command, command}, _from, state) do
      state = %{state | commands: state.commands ++ [command]}
      {reply, next} = execute(command, state)
      {:reply, reply, next}
    end

    @impl true
    def terminate(_reason, state) do
      :gen_tcp.close(state.listener)
      Process.exit(state.acceptor, :shutdown)
    end

    defp execute(["CLIENT" | _], state), do: {"+OK\r\n", state}
    defp execute(["PING"], state), do: {"+PONG\r\n", state}

    defp execute(["SCAN", cursor, "MATCH", "125_*", "COUNT", "1000"], state) do
      next = %{state | scan_calls: state.scan_calls + 1}

      if next.scan_calls == state.fail_scan_at do
        {"-ERR recorded SCAN unavailable\r\n", next}
      else
        # A stable two-page namespace exercises acknowledgment across cursors.
        {cursor, candidates} =
          if cursor == "0",
            do: {"7", ["125_session:first"]},
            else: {"0", ["125_session:second"]}

        keys = Enum.filter(candidates, &MapSet.member?(state.keys, &1))
        {array([bulk(cursor), array(Enum.map(keys, &bulk/1))]), next}
      end
    end

    defp execute(["DEL" | keys], state) do
      next = %{state | delete_calls: state.delete_calls + 1}

      if next.delete_calls == state.fail_delete_at do
        {"-ERR recorded DEL unavailable\r\n", next}
      else
        count = Enum.count(keys, &MapSet.member?(state.keys, &1))
        {":#{count}\r\n", %{next | keys: Enum.reduce(keys, state.keys, &MapSet.delete(&2, &1))}}
      end
    end

    defp array(items), do: ["*#{length(items)}\r\n", items]
    defp bulk(value), do: ["$#{byte_size(value)}\r\n", value, "\r\n"]

    defp accept(listener, owner) do
      case :gen_tcp.accept(listener) do
        {:ok, socket} ->
          handler = spawn_link(fn -> serve(socket, owner, "") end)
          :ok = :gen_tcp.controlling_process(socket, handler)
          send(handler, :ready)
          accept(listener, owner)

        {:error, :closed} ->
          :ok
      end
    end

    defp serve(socket, owner, buffer) do
      receive do
        :ready -> exchange(socket, owner, buffer)
      end
    end

    defp exchange(socket, owner, buffer) do
      case parse(buffer) do
        {:ok, command, rest} ->
          reply = GenServer.call(owner, {:command, command})
          :ok = :gen_tcp.send(socket, reply)
          exchange(socket, owner, rest)

        :more ->
          case :gen_tcp.recv(socket, 0) do
            {:ok, bytes} -> exchange(socket, owner, buffer <> bytes)
            {:error, :closed} -> :ok
          end
      end
    end

    defp parse("*" <> rest) do
      case String.split(rest, "\r\n", parts: 2) do
        [count, data] -> parse_items(data, String.to_integer(count), [])
        _ -> :more
      end
    end

    defp parse(_), do: :more
    defp parse_items(data, 0, acc), do: {:ok, Enum.reverse(acc), data}

    defp parse_items("$" <> rest, count, acc) do
      case String.split(rest, "\r\n", parts: 2) do
        [length, data] ->
          size = String.to_integer(length)

          case data do
            <<item::binary-size(size), "\r\n", tail::binary>> -> parse_items(tail, count - 1, [item | acc])
            _ -> :more
          end

        _ ->
          :more
      end
    end

    defp parse_items(_, _, _), do: :more
  end

  setup do
    :ok = Sandbox.checkout(Repo.Account, sandbox: false)
    :ok = Sandbox.mode(Repo.Account, {:shared, self()})
    :ok = Sandbox.checkout(Repo)
    existing_ids = Repo.Account.all(from(i in Identity, select: i.id))

    configs = [
      {:explorer, Auth0ToKeycloakMigration},
      {:explorer, Account},
      {:explorer, Keycloak},
      {:ueberauth, OAuth},
      {:tesla, :adapter},
      {:block_scout_web, :chain_id}
    ]

    saved = Enum.map(configs, fn {app, key} -> {app, key, Application.fetch_env(app, key)} end)
    Application.delete_env(:explorer, Auth0ToKeycloakMigration)
    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)
    Application.put_env(:block_scout_web, :chain_id, "125")
    Application.put_env(:ueberauth, OAuth, domain: "recorded-auth0.example", auth0_application_id: "")
    Application.put_env(:explorer, Account, enabled: true, private_tags_limit: 20)
    path = Path.join(System.tmp_dir!(), "repair-2233491-#{System.unique_integer([:positive])}.dets")
    {:ok, _} = Application.ensure_all_started(:plug_cowboy)

    on_exit(fn ->
      Sandbox.unboxed_run(Repo.Account, fn ->
        Repo.Account.query!("ALTER TABLE account_watchlists DROP CONSTRAINT IF EXISTS repair_2233491_watchlist")
        Repo.Account.delete_all(from(i in Identity, where: i.id not in ^existing_ids))
      end)

      Enum.each(saved, fn
        {app, key, {:ok, value}} -> Application.put_env(app, key, value)
        {app, key, :error} -> Application.delete_env(app, key)
      end)

      File.rm(path)
    end)

    {:ok, path: path}
  end

  test "migration-error-shape: real Multi failure records its group and rollback survives reopening", %{path: path} do
    first = account("github|666666", "john@blockscout.com")
    second = account("google-oauth2|666666", "john@blockscout.com")
    third = account("github|777777", "other@blockscout.com")
    fourth = account("google-oauth2|777777", "other@blockscout.com")
    users = Enum.map([first, second, third, fourth], &source/1)
    {:ok, plan} = Auth0ToKeycloakMigration.plan(Map.new(users, &{&1["user_id"], &1}))
    [earlier_group, failing_group] = plan.merges
    deleted_id = List.last(failing_group)
    redis = configure_cutover(path, [], deleted_id)

    assert {:error, {:merge_failed, ^failing_group, {:acquire_identity, ^deleted_id}, :not_found}} =
             in_new_process(fn -> Auth0ToKeycloakMigration.run_export(ndjson(users), dry_run: false) end)

    assert {:ok, checkpoint} = MigrationCheckpoint.load()
    assert checkpoint.phase == :applying
    assert checkpoint.account_access == :disabled
    assert checkpoint.completed_merges == []
    assert checkpoint.error == {:merge_failed, failing_group, {:acquire_identity, deleted_id}, :not_found}
    assert map_size(checkpoint.keycloak_map) == 2
    assert map_size(checkpoint.identity_mapping) == 4

    for identity <- [first, second, third, fourth] do
      assert checkpoint.identity_mapping[identity.id].auth0_id == identity.uid
      assert checkpoint.identity_mapping[identity.id].keycloak_id in [@target, @other_target]
    end

    assert checkpoint.plan.merges == plan.merges

    for id <- earlier_group,
        do: assert(Repo.Account.get!(Identity, id).uid in Enum.map([first, second, third, fourth], & &1.uid))

    assert RedisNamespace.snapshot(redis).commands == []
    assert Authentication.responsible_module() == {:enabled, false}

    # Repair the missing input, then reopen the real migration in a new process.
    deleted = Enum.find([first, second, third, fourth], &(&1.id == deleted_id))

    restored =
      Repo.Account.insert!(%Identity{id: deleted.id, uid: deleted.uid, uid_hash: deleted.uid, email: deleted.email})

    Repo.Account.insert!(%Watchlist{identity_id: restored.id, name: "default"})
    # Reproduce a process stopping after the real account COMMIT but before
    # the next checkpoint: the last durable phase still says :applying.
    assert {:ok, committed} =
             in_new_process(fn ->
               Auth0ToKeycloakMigration.apply_plan(checkpoint.plan, checkpoint.keycloak_map)
             end)

    assert length(committed) == 2
    assert {:ok, %{phase: :applying}} = MigrationCheckpoint.load()
    assert results = in_new_process(&Auth0ToKeycloakMigration.resume/0)
    assert results == committed
    assert length(results) == 2
    assert Enum.all?(results, &match?({:ok, _, _}, &1))
    assert {:ok, %{phase: :complete, completed_merges: completed}} = MigrationCheckpoint.load()
    assert completed == plan.merges

    for [primary | secondary] <- plan.merges do
      assert Repo.Account.get!(Identity, primary).uid in [@target, @other_target]
      for id <- secondary, do: assert(Repo.Account.get(Identity, id) == nil)
    end

    assert_import_requests(1)
  end

  test "migration-provider-restart: durable provider and source mapping survive lost runtime configuration", %{
    path: path
  } do
    identity = account("github|666666", "john@blockscout.com")
    redis = configure_cutover(path)

    assert [{:ok, source_id, @target}] =
             in_new_process(fn ->
               Auth0ToKeycloakMigration.run_export(ndjson([source(identity)]), dry_run: false)
             end)

    assert source_id == identity.uid
    assert {:ok, checkpoint} = MigrationCheckpoint.load()
    assert checkpoint.phase == :complete
    assert checkpoint.provider == :keycloak
    assert checkpoint.account_access == :enabled
    assert checkpoint.keycloak_map[identity.id] == @target
    assert hd(checkpoint.plan.items).auth0_id == identity.uid
    assert Repo.Account.get!(Identity, identity.id).uid == @target
    assert Bitwise.band(File.stat!(path).mode, 0o777) == 0o600
    refute File.read!(path) =~ identity.uid
    assert RedisNamespace.snapshot(redis).keys == MapSet.new(["999_session:other"])

    # Reconstitute the original deployment settings, with no migration process alive.
    Application.put_env(:ueberauth, OAuth, domain: "recorded-auth0.example", auth0_application_id: "")
    assert Auth0.enabled?()
    assert in_new_process(&Authentication.responsible_module/0) == {:ok, Keycloak}
    assert [{:ok, ^source_id, @target}] = in_new_process(&Auth0ToKeycloakMigration.resume/0)
    assert Authentication.account_access() == :ok
    assert_access_closed(auth(identity.uid))
    assert Authentication.authenticate_via_dynamic("old-provider-token") == {:enabled, false}
    assert_import_requests(1)
  end

  test "migration-revocation-errors: DEL rejection retains keys and pending acknowledgment across restart", %{
    path: path
  } do
    identity = account("github|666666", "john@blockscout.com")
    redis = configure_cutover(path, fail_delete_at: 1)

    assert {:error, {:session_delete, {:error, %Redix.Error{message: "ERR recorded DEL unavailable"}}}} =
             in_new_process(fn -> Auth0ToKeycloakMigration.run_export(ndjson([source(identity)]), dry_run: false) end)

    assert {:ok, checkpoint} = MigrationCheckpoint.load()
    assert checkpoint.phase == :revoking
    assert checkpoint.revocation.pending == %{keys: ["125_session:first"], next_cursor: "7"}
    assert checkpoint.revocation.count == 0
    assert MapSet.member?(RedisNamespace.snapshot(redis).keys, "125_session:first")
    assert Authentication.responsible_module() == {:enabled, false}
    assert Account.enabled?() == false
    assert_access_closed(auth("keycloak-id"))

    assert [{:ok, _, @target}] = in_new_process(&Auth0ToKeycloakMigration.resume/0)
    snapshot = RedisNamespace.snapshot(redis)
    assert snapshot.keys == MapSet.new(["999_session:other"])
    assert Enum.count(snapshot.commands, &(&1 == ["DEL", "125_session:first"])) == 2
    assert {:ok, %{phase: :complete, revocation: %{pending: nil, cursor: "0", count: 2}}} = MigrationCheckpoint.load()
    assert Authentication.responsible_module() == {:ok, Keycloak}
    assert_import_requests(1)
  end

  test "migration-revocation-errors: SCAN rejection preserves acknowledged progress and rescans the whole namespace", %{
    path: path
  } do
    identity = account("github|666666", "john@blockscout.com")
    redis = configure_cutover(path, fail_scan_at: 2)

    assert {:error, {:session_scan, {:error, %Redix.Error{message: "ERR recorded SCAN unavailable"}}}} =
             in_new_process(fn -> Auth0ToKeycloakMigration.run_export(ndjson([source(identity)]), dry_run: false) end)

    assert {:ok, checkpoint} = MigrationCheckpoint.load()
    assert checkpoint.phase == :revoking
    assert checkpoint.revocation == %{pattern: "125_*", cursor: "7", pending: nil, count: 1}
    assert RedisNamespace.snapshot(redis).keys == MapSet.new(["125_session:second", "999_session:other"])
    Application.put_env(:explorer, Account, enabled: true, private_tags_limit: 20)
    assert in_new_process(&Authentication.responsible_module/0) == {:enabled, false}
    assert [{:ok, _, @target}] = in_new_process(&Auth0ToKeycloakMigration.resume/0)
    assert RedisNamespace.snapshot(redis).keys == MapSet.new(["999_session:other"])
    assert {:ok, %{phase: :complete, revocation: %{count: 2}}} = MigrationCheckpoint.load()
    assert_import_requests(1)
  end

  test "migration-provider-restart: missing or damaged durable state never falls back to Auth0", %{path: path} do
    Application.put_env(:explorer, Auth0ToKeycloakMigration, checkpoint_path: path)
    assert Authentication.responsible_module() == {:enabled, false}
    assert Auth0ToKeycloakMigration.resume() == {:error, :checkpoint_missing}
    File.write!(path, "interrupted or invalid checkpoint")
    assert {:error, _} = MigrationCheckpoint.load()
    assert Authentication.responsible_module() == {:enabled, false}
    assert_access_closed(auth("legacy-blocked"))
  end

  test "migration-provider-restart: checkpoint storage failure prevents imports and identity writes", %{path: path} do
    identity = account("github|666666", "john@blockscout.com")
    redis = configure_cutover(Path.join(path, "missing-parent.dets"))

    assert {:error, {:checkpoint_open, _}} =
             in_new_process(fn ->
               Auth0ToKeycloakMigration.run_export(ndjson([source(identity)]), dry_run: false)
             end)

    assert Repo.Account.get!(Identity, identity.id).uid == identity.uid
    assert Account.enabled?()
    assert RedisNamespace.snapshot(redis).commands == []
    assert_import_requests(0)
  end

  test "identity-watchlist-atomicity: rejected initial watchlist rolls identity back before a fresh login" do
    auth = auth("github|atomic")
    reject_watchlists()
    assert_raise Ecto.ConstraintError, fn -> Identity.find_or_create(auth) end
    assert Identity.find_identity(auth) == nil
    allow_watchlists()
    assert {:ok, session} = in_new_process(fn -> Identity.find_or_create(auth) end)
    assert is_integer(session.watchlist_id)
    assert Repo.Account.get!(Watchlist, session.watchlist_id).identity_id == session.id
    assert {:ok, repeated} = in_new_process(fn -> Identity.find_or_create(auth) end)
    assert repeated.watchlist_id == session.watchlist_id
  end

  test "identity-watchlist-atomicity: legacy identity recovers its missing watchlist after login process restart" do
    auth = auth("github|legacy")
    identity = Repo.Account.insert!(Identity.new_identity(auth))
    assert Repo.Account.all(from(w in Watchlist, where: w.identity_id == ^identity.id)) == []
    reject_watchlists()
    assert_raise Ecto.ConstraintError, fn -> Identity.find_or_create(auth) end
    assert Repo.Account.get!(Identity, identity.id).uid == auth.uid
    allow_watchlists()
    assert {:ok, session} = in_new_process(fn -> Identity.find_or_create(auth) end)
    assert session.id == identity.id
    assert {:ok, reopened} = in_new_process(fn -> Identity.find_or_create(auth) end)
    assert reopened.watchlist_id == session.watchlist_id
    assert Repo.Account.aggregate(from(w in Watchlist, where: w.identity_id == ^identity.id), :count) == 1
  end

  test "tag-address-encrypted-lookup: a saved private tag is found after independently randomized encryption" do
    {:ok, session} = Identity.find_or_create(auth("github|address"))
    {:ok, hash} = Explorer.Chain.string_to_address_hash(@address)
    insert(:address, hash: hash)
    assert {:ok, tag} = TagAddress.create(%{identity_id: session.id, address_hash: @address, name: "cold wallet"})
    assert {:ok, first_ciphertext} = Explorer.Encrypted.AddressHash.dump(hash)
    assert {:ok, second_ciphertext} = Explorer.Encrypted.AddressHash.dump(hash)
    refute first_ciphertext == second_ciphertext
    assert {:ok, ^hash} = Explorer.Encrypted.AddressHash.load(second_ciphertext)
    assert TagAddress.get_tag_address_by_address_hash_and_identity_id(@address, session.id).id == tag.id
    assert TagAddress.get_tag_address_by_address_hash_and_identity_id(hash, session.id).name == "cold wallet"

    assert Repo.Account.one(
             TagAddress.tag_address_by_address_hash_and_identity_id_query(String.upcase(@address), session.id)
           ).id == tag.id

    assert TagAddress.get_tag_address_by_address_hash_and_identity_id(hash, session.id + 1) == nil
  end

  test "tag-transaction-error-shape: real identity deletion Multi returns the documented validation changeset" do
    {:ok, session} = Identity.find_or_create(auth("github|transaction"))
    %{"tx" => recording} = @recording |> File.read!() |> Jason.decode!()
    transaction = insert(:transaction, hash: recording["hash"])
    attrs = %{identity_id: session.id, transaction_hash: to_string(transaction.hash), name: "recorded transfer"}
    assert {:ok, saved} = TagTransaction.create(attrs)
    assert saved.identity_id == session.id

    assert {:ok, %{delete_identities: {1, nil}}} =
             Ecto.Multi.new() |> Identity.delete([session.id]) |> Repo.Account.transaction()

    assert {:error, %Ecto.Changeset{} = changeset} = TagTransaction.create(attrs)
    assert {"User not found", details} = Keyword.fetch!(changeset.errors, :identity_id)
    assert details[:constraint] == :foreign
    assert details[:constraint_name] == "account_tag_transactions_identity_id_fkey"
    assert Repo.Account.get(Identity, session.id) == nil
    assert Repo.Account.get(TagTransaction, saved.id) == nil
  end

  defp assert_access_closed(auth) do
    assert {:error, %Ecto.Changeset{} = changeset} = Identity.find_or_create(auth)
    assert {"Account access is disabled", []} = Keyword.fetch!(changeset.errors, :uid)
  end

  defp configure_cutover(path, redis_options \\ [], deletion \\ nil) do
    redis = start_supervised!({RedisNamespace, redis_options})
    connection = start_supervised!({Redix, host: "127.0.0.1", port: RedisNamespace.port(redis), sync_connect: true})
    Application.put_env(:explorer, Auth0ToKeycloakMigration, checkpoint_path: path, redis: connection)
    ref = {:repair_2233491, make_ref()}

    start_supervised!(
      Supervisor.child_spec(
        {Plug.Cowboy,
         scheme: :http, plug: {KeycloakRecording, {self(), deletion}}, options: [port: 0, ref: ref, ip: {127, 0, 0, 1}]},
        id: ref
      )
    )

    Application.put_env(:explorer, Keycloak,
      domain: "http://127.0.0.1:#{:ranch.get_port(ref)}",
      realm: "repair",
      client_id: "recorded-client",
      client_secret: "recorded-secret"
    )

    redis
  end

  defp in_new_process(fun) do
    task = Task.async(fun)
    pid = task.pid
    monitor = Process.monitor(pid)
    result = Task.await(task, 15_000)
    assert_receive {:DOWN, ^monitor, :process, ^pid, :normal}
    refute Process.alive?(pid)
    result
  end

  defp assert_import_requests(expected) do
    requests = drain_requests([])
    imports = Enum.filter(requests, fn {path, _} -> String.ends_with?(path, "/partialImport") end)
    assert length(imports) == expected

    Enum.each(imports, fn {_, body} ->
      assert %{"ifResourceExists" => "SKIP", "users" => [_ | _]} = Jason.decode!(body)
    end)
  end

  defp drain_requests(acc) do
    receive do
      {:keycloak_request, path, body} -> drain_requests([{path, body} | acc])
    after
      0 -> Enum.reverse(acc)
    end
  end

  defp account(uid, email) do
    identity = %Identity{} |> Identity.changeset(%{uid: uid, email: email}) |> Repo.Account.insert!()
    Repo.Account.insert!(%Watchlist{identity_id: identity.id, name: "default"})
    {:ok, _} = Key.create(%{identity_id: identity.id, name: "account data"})
    identity
  end

  defp source(identity) do
    [provider, id] = String.split(identity.uid, "|", parts: 2)

    %{
      "user_id" => identity.uid,
      "email" => identity.email,
      "email_verified" => true,
      "name" => "John Snow",
      "nickname" => "johnnny",
      "user_metadata" => %{},
      "identities" => [%{"provider" => provider, "user_id" => id, "isSocial" => true}]
    }
  end

  defp auth(uid) do
    %Auth{
      uid: uid,
      provider: :auth0,
      info: %Info{email: "john@blockscout.com", name: "John Snow", nickname: "johnnny"},
      extra: %Extra{raw_info: %{user: %{"email" => "john@blockscout.com", "email_verified" => true}}}
    }
  end

  defp ndjson(users), do: Enum.map_join(users, "\n", &Jason.encode!/1)

  defp reject_watchlists do
    Repo.Account.query!(
      "ALTER TABLE account_watchlists ADD CONSTRAINT repair_2233491_watchlist CHECK (name <> 'default') NOT VALID"
    )
  end

  defp allow_watchlists do
    Repo.Account.query!("ALTER TABLE account_watchlists DROP CONSTRAINT repair_2233491_watchlist")
  end
end
