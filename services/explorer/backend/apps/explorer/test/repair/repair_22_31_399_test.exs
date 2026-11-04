defmodule Explorer.Repair.Repair2231399Test do
  use ExUnit.Case, async: false

  alias Explorer.Account
  alias Explorer.Account.{Auth0ToKeycloakMigration, Identity, Watchlist, WatchlistAddress}
  alias Explorer.Account.Api.Key
  alias Explorer.Repo
  alias Ueberauth.Strategy.Auth0.OAuth

  @address "0x0000000000000000000000000000000000000678"
  @keycloak_id "b963421b-8e90-4537-8be8-32f96d741859"
  @import_response ~s({"added":1,"failed":0,"overwritten":0,"skipped":0,"results":[{"action":"ADDED","resourceType":"USER","resourceName":"john@blockscout.com","id":"b963421b-8e90-4537-8be8-32f96d741859"}]})

  setup do
    :ok = Ecto.Adapters.SQL.Sandbox.checkout(Repo.Account)
    oauth = Application.get_env(:ueberauth, OAuth, [])
    account = Application.get_env(:explorer, Account, [])
    Application.put_env(:ueberauth, OAuth, Keyword.put(oauth, :auth0_application_id, ""))

    on_exit(fn ->
      Application.put_env(:ueberauth, OAuth, oauth)
      Application.put_env(:explorer, Account, account)
    end)

    :ok
  end

  test "migration-dry-run-writes: gzipped export preview preserves every account row and authentication configuration" do
    {first, _} = account("github|666666", "john@blockscout.com")
    {second, _} = account("google-oauth2|666666", "john@blockscout.com")
    users = [source(first, true), source(second, true)]
    before = snapshot()
    account_config = Application.get_env(:explorer, Account)
    auth_config = Application.get_env(:ueberauth, OAuth)
    token = Process.get(:keycloak_admin_token)

    assert {:ok, %{items: [_], merges: [ids]}} = Auth0ToKeycloakMigration.plan(export_map(users))
    assert Enum.sort(ids) == Enum.sort([first.id, second.id])
    assert Auth0ToKeycloakMigration.run_export(:zlib.gzip(ndjson(users)), dry_run: true) == []
    assert Auth0ToKeycloakMigration.run_export(ndjson(users)) == []

    assert snapshot() == before
    assert Application.get_env(:explorer, Account) == account_config
    assert Application.get_env(:ueberauth, OAuth) == auth_config
    assert Process.get(:keycloak_admin_token) == token
  end

  test "migration-dry-run-writes: applied verified merge transfers real keys and watchlist addresses and updates UID" do
    {first, first_watchlist} = account("github|666666", "john@blockscout.com")
    {second, second_watchlist} = account("google-oauth2|666666", "john@blockscout.com")
    users = [source(first, true), source(second, true)]
    {:ok, plan} = Auth0ToKeycloakMigration.plan(export_map(users))
    [item] = plan.items
    assert [ids] = plan.merges
    assert Enum.sort(ids) == Enum.sort([first.id, second.id])
    assert hd(ids) == item.identity_id

    [{primary, primary_watchlist}, {secondary, secondary_watchlist}] =
      Enum.sort_by([{first, first_watchlist}, {second, second_watchlist}], fn {identity, _} ->
        identity.id != item.identity_id
      end)

    {:ok, key} = Key.create(%{identity_id: secondary.id, name: "private API key"})
    {:ok, address_hash} = Explorer.Chain.string_to_address_hash(@address)

    watch_address =
      %WatchlistAddress{
        watchlist_id: secondary_watchlist.id,
        name: "private wallet",
        address_hash: address_hash,
        address_hash_hash: @address
      }
      |> Repo.account_repo().insert!()

    assert Repo.account_repo().get!(Key, key.value).identity_id == secondary.id
    assert Repo.account_repo().get!(Identity, secondary.id).uid == secondary.uid

    assert {:ok, [{:ok, auth0_id, @keycloak_id}]} =
             Auth0ToKeycloakMigration.apply_plan(plan, imported_ids(plan))

    assert auth0_id == primary.uid
    assert Repo.account_repo().get!(Identity, primary.id).uid == @keycloak_id
    assert Repo.account_repo().get(Identity, secondary.id) == nil
    assert Repo.account_repo().get!(Key, key.value).identity_id == primary.id
    migrated_address = Repo.account_repo().get!(WatchlistAddress, watch_address.id)
    assert migrated_address.watchlist_id == primary_watchlist.id
    assert migrated_address.name == "private wallet"
    assert to_string(migrated_address.address_hash) == @address
  end

  test "migration-dry-run-writes: a failed UID update rolls back merges, associated data and prior UID updates" do
    {first, _} = account("github|666666", "john@blockscout.com")
    {second, _} = account("google-oauth2|666666", "john@blockscout.com")
    {third, _} = account("auth0|unrelated", "other@blockscout.com")
    {:ok, _key} = Key.create(%{identity_id: second.id, name: "private API key"})

    {:ok, plan} =
      Auth0ToKeycloakMigration.plan(export_map([source(first, true), source(second, true), source(third, true)]))

    before = snapshot()
    [merge_ids] = plan.merges
    primary = Enum.find(plan.items, &(&1.identity_id == hd(merge_ids)))
    assert Enum.sort(merge_ids) == Enum.sort([first.id, second.id])

    assert {:error, {:identity_updates_failed, results}} =
             Auth0ToKeycloakMigration.apply_plan(plan, imported_ids(plan))

    assert {:ok, primary.auth0_id, @keycloak_id} in results
    assert {:error, third.uid, :no_keycloak_id} in results
    assert snapshot() == before
  end

  test "migration-unverified-email: false, missing and nonboolean evidence remains false in the Keycloak JSON payload" do
    {identity, _} = account("github|666666", "john@blockscout.com")

    for evidence <- [false, nil, "true", true] do
      user = source(identity, evidence)
      user = if is_nil(evidence), do: Map.delete(user, "email_verified"), else: user
      {:ok, %{items: [item], merges: []}} = Auth0ToKeycloakMigration.plan(export_map([user]))
      assert item.email_verified == (evidence == true)
      {username, body} = Auth0ToKeycloakMigration.build_keycloak_user(item, MapSet.new())
      decoded = Jason.decode!(Jason.encode!(%{ifResourceExists: "SKIP", users: [body]}))
      assert decoded["ifResourceExists"] == "SKIP"

      assert [%{"username" => ^username, "email" => "john@blockscout.com", "emailVerified" => verified}] =
               decoded["users"]

      assert verified == (evidence == true)
    end
  end

  test "migration-unverified-email: verified evidence for a different metadata email cannot verify the migrated email" do
    {identity, _} = account("github|666666", "john@blockscout.com")
    user = source(identity, true) |> Map.put("user_metadata", %{"email" => "victim@blockscout.com"})
    {:ok, %{items: [item]}} = Auth0ToKeycloakMigration.plan(export_map([user]))
    assert item.email == "victim@blockscout.com"
    refute item.email_verified
    {_, body} = Auth0ToKeycloakMigration.build_keycloak_user(item, MapSet.new())
    assert body.emailVerified == false
  end

  test "migration-unverified-email: mixed and unverified email collisions halt preview and apply without account writes" do
    {first, _} = account("github|666666", "john@blockscout.com")
    {second, _} = account("google-oauth2|666666", "JOHN@blockscout.com")
    before = snapshot()
    auth_config = Application.get_env(:ueberauth, OAuth)
    account_config = Application.get_env(:explorer, Account)

    for {left, right} <- [{true, false}, {false, true}, {false, false}, {true, nil}] do
      users = [source(first, left), source(second, right)]

      assert {:error, {:unverified_email_collision, "john@blockscout.com", ids}} =
               Auth0ToKeycloakMigration.plan(export_map(users))

      assert Enum.sort(ids) == Enum.sort([first.id, second.id])
      assert Auth0ToKeycloakMigration.run_export(ndjson(users), dry_run: true) == []
      assert Auth0ToKeycloakMigration.run_export(ndjson(users), dry_run: false) == []
      assert snapshot() == before
    end

    assert Application.get_env(:ueberauth, OAuth) == auth_config
    assert Application.get_env(:explorer, Account) == account_config
  end

  test "migration-unverified-email: existing-user reuse requires verification on both sides for the same email" do
    {identity, _} = account("github|666666", "john@blockscout.com")
    {:ok, %{items: [item]}} = Auth0ToKeycloakMigration.plan(export_map([source(identity, true)]))
    {_, body} = Auth0ToKeycloakMigration.build_keycloak_user(item, MapSet.new())

    target =
      Jason.decode!(
        ~s({"id":"#{@keycloak_id}","username":"john@blockscout.com","email":"JOHN@blockscout.com","emailVerified":true})
      )

    assert Auth0ToKeycloakMigration.verified_common_owner?(body, target)
    refute Auth0ToKeycloakMigration.verified_common_owner?(%{body | emailVerified: false}, target)
    refute Auth0ToKeycloakMigration.verified_common_owner?(body, %{target | "emailVerified" => false})
    refute Auth0ToKeycloakMigration.verified_common_owner?(body, Map.delete(target, "emailVerified"))
    refute Auth0ToKeycloakMigration.verified_common_owner?(body, %{target | "email" => "other@blockscout.com"})
    refute Auth0ToKeycloakMigration.verified_common_owner?(Map.delete(body, :emailVerified), target)
  end

  defp account(uid, email) do
    identity = %Identity{} |> Identity.changeset(%{uid: uid, email: email}) |> Repo.account_repo().insert!()
    watchlist = Repo.account_repo().insert!(%Watchlist{identity_id: identity.id, name: "default"})
    {:ok, _key} = Key.create(%{identity_id: identity.id, name: "account data"})
    {identity, watchlist}
  end

  defp source(identity, verified) do
    [provider, user_id] = String.split(identity.uid, "|", parts: 2)

    %{
      "user_id" => identity.uid,
      "email" => identity.email,
      "email_verified" => verified,
      "name" => "John Snow",
      "nickname" => "johnnny",
      "user_metadata" => %{},
      "identities" => [%{"provider" => provider, "user_id" => user_id, "isSocial" => provider != "auth0"}]
    }
  end

  defp imported_ids(plan) do
    %{"results" => results} = Jason.decode!(@import_response)
    ids_by_username = Map.new(results, &{&1["resourceName"], &1["id"]})

    for item <- plan.items,
        {:ok, id} <- [Map.fetch(ids_by_username, item.username)],
        into: %{},
        do: {item.identity_id, id}
  end

  defp ndjson(users), do: Enum.map_join(users, "\n", &Jason.encode!/1)
  defp export_map(users), do: Map.new(users, &{&1["user_id"], &1})

  defp snapshot do
    tables =
      Repo.account_repo().query!(
        "SELECT tablename FROM pg_tables WHERE schemaname = 'public' AND tablename LIKE 'account_%' ORDER BY tablename"
      ).rows

    Map.new(tables, fn [table] ->
      escaped = String.replace(table, "\"", "\"\"")

      rows =
        Repo.account_repo().query!(~s|SELECT row_to_json(t)::text FROM "#{escaped}" t ORDER BY row_to_json(t)::text|).rows

      {table, rows}
    end)
  end
end
