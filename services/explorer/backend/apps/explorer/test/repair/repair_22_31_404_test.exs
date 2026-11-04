defmodule Explorer.Repair.Repair2231404Test do
  use ExUnit.Case, async: false

  import Ecto.Query
  import ExUnit.CaptureLog

  alias Explorer.Account
  alias Explorer.Account.{Api.Key, CustomABI, Identity, TagAddress, TagTransaction, Watchlist, WatchlistAddress}
  alias Explorer.Repo
  alias Mix.Tasks.Auth0ToKeycloakMigrate.Preview
  alias Ueberauth.Strategy.Auth0.OAuth

  @export Path.expand("../../lib/mix/tasks/auth0_to_keycloak_migrate/fixtures/duplicate_users.jsonl", __DIR__)
  @schemas [Identity, Key, TagAddress, TagTransaction, CustomABI, Watchlist, WatchlistAddress]

  setup do
    :ok = Ecto.Adapters.SQL.Sandbox.checkout(Repo.account_repo())

    previous = Application.fetch_env(:ueberauth, OAuth)
    Application.put_env(:ueberauth, OAuth, auth0_application_id: "", domain: nil)

    on_exit(fn ->
      case previous do
        {:ok, config} -> Application.put_env(:ueberauth, OAuth, config)
        :error -> Application.delete_env(:ueberauth, OAuth)
      end
    end)

    users = @export |> File.read!() |> String.split("\n", trim: true) |> Enum.map(&Jason.decode!/1)
    [email, wallet] = Enum.map(users, &insert_identity(&1["user_id"], &1["email"]))
    seed_associations(email, 1)
    seed_associations(wallet, 2)

    missing = insert_identity("auth0|not-in-export", "missing@example.com")
    seed_associations(missing, 3)
    empty = insert_identity("auth0|empty", "empty@example.com")

    %{email: email, wallet: wallet, missing: missing, empty: empty, users: Map.new(users, &{&1["user_id"], &1})}
  end

  test "migration-preview CLI leaves duplicate identities and all associations byte-for-byte unchanged", context do
    before = snapshot()
    config = Application.get_env(:explorer, Account)
    owner = self()
    handler = {__MODULE__, make_ref()}
    event = Repo.account_repo().config()[:telemetry_prefix] ++ [:query]

    :ok =
      :telemetry.attach(
        handler,
        event,
        fn _event, _measurements, metadata, _config ->
          send(owner, {:preview_query, metadata.query})
        end,
        nil
      )

    try do
      log =
        capture_log(fn ->
          assert run_cli(["--dry-run", "--auth0-export", @export]) == []
        end)

      assert log =~ "Would merge duplicate email preview@example.com"
      assert log =~ "into identity #{context.wallet.id}"
      assert log =~ "Would delete 2 identities"
      assert snapshot() == before
      assert Application.get_env(:explorer, Account) == config
      assert Application.get_env(:ueberauth, OAuth)[:domain] == nil

      queries = drain_queries([])
      assert Enum.any?(queries, &String.starts_with?(&1, "SELECT"))
      refute Enum.any?(queries, &Regex.match?(~r/^\s*(INSERT|UPDATE|DELETE|MERGE|TRUNCATE)\b/i, &1))
    after
      :telemetry.detach(handler)
    end
  end

  test "migration-preview CLI accepts the compressed Auth0 export without writes" do
    path = Path.join(System.tmp_dir!(), "repair-22-31-404-#{System.unique_integer([:positive])}.json.gz")
    File.write!(path, :zlib.gzip(File.read!(@export)))
    on_exit(fn -> File.rm!(path) end)
    before = snapshot()

    assert capture_log(fn -> assert run_cli(["-n", "--auth0-export", path]) == [] end) =~
             "Would merge duplicate email"

    assert snapshot() == before
  end

  test "migration-preview planning selects the wallet primary and protects missing users with data", context do
    before = snapshot()

    entries = [
      %{identity: context.email, has_user_data: true},
      %{identity: context.wallet, has_user_data: true},
      %{identity: context.missing, has_user_data: true},
      %{identity: context.empty, has_user_data: false}
    ]

    plan = Preview.plan(entries, context.users)

    assert [%{identity_id: primary, auth0_id: "auth0|preview-wallet"}] = plan.items
    assert primary == context.wallet.id

    assert plan.merges == [
             %{email: "preview@example.com", primary: context.wallet.id, duplicates: [context.email.id]}
           ]

    assert plan.protected_ids == MapSet.new([context.wallet.id, context.missing.id])
    assert snapshot() == before
  end

  test "explicit execution merges duplicate identities and moves their real associations", context do
    primary = context.wallet
    duplicate = context.email
    original_keys = Repo.account_repo().aggregate(Key, :count)
    primary_watchlist = Repo.account_repo().one!(from(w in Watchlist, where: w.identity_id == ^primary.id))
    duplicate_watchlist = Repo.account_repo().one!(from(w in Watchlist, where: w.identity_id == ^duplicate.id))

    assert {{:ok, _operations}, ^primary} = Account.merge([primary, duplicate])
    assert Repo.account_repo().get(Identity, duplicate.id) == nil
    assert Repo.account_repo().get!(Identity, primary.id).uid == primary.uid
    assert Repo.account_repo().get!(Identity, context.missing.id).uid == context.missing.uid
    assert Repo.account_repo().aggregate(Key, :count) == original_keys

    for schema <- [Key, CustomABI, TagAddress, TagTransaction] do
      assert Repo.account_repo().aggregate(from(row in schema, where: row.identity_id == ^duplicate.id), :count) == 0
      assert Repo.account_repo().aggregate(from(row in schema, where: row.identity_id == ^primary.id), :count) == 2
    end

    assert Repo.account_repo().aggregate(
             from(row in WatchlistAddress, where: row.watchlist_id == ^primary_watchlist.id),
             :count
           ) == 2

    assert Repo.account_repo().aggregate(
             from(row in WatchlistAddress, where: row.watchlist_id == ^duplicate_watchlist.id),
             :count
           ) == 0
  end

  test "explicit CLI execution still enters the configured migration runner" do
    before = snapshot()

    assert capture_log(fn -> assert run_cli(["--no-dry-run"]) == [] end) =~
             "Auth0 is not configured. Cannot read source users."

    assert snapshot() == before
  end

  test "an export cannot accidentally select execution" do
    before = snapshot()

    assert_raise Mix.Error, "--auth0-export requires --dry-run", fn ->
      run_cli(["--auth0-export", @export])
    end

    assert snapshot() == before
  end

  test "an unreadable export fails visibly without changing identities or associations" do
    before = snapshot()

    assert_raise File.Error, fn ->
      run_cli(["--dry-run", "--auth0-export", @export <> ".missing"])
    end

    assert snapshot() == before
  end

  defp run_cli(args) do
    Mix.Task.rerun("auth0_to_keycloak_migrate", ["--no-start" | args])
  end

  defp insert_identity(uid, email) do
    %Identity{}
    |> Identity.changeset(%{uid: uid, email: email})
    |> Repo.account_repo().insert!()
  end

  defp seed_associations(identity, index) do
    {:ok, address} =
      Explorer.Chain.string_to_address_hash("0x" <> String.pad_leading(Integer.to_string(index, 16), 40, "0"))

    {:ok, transaction} =
      Explorer.Chain.string_to_full_hash("0x" <> String.pad_leading(Integer.to_string(index, 16), 64, "0"))

    repo = Repo.account_repo()
    repo.insert!(%Key{identity_id: identity.id, name: "migration key #{index}", value: Ecto.UUID.generate()})

    repo.insert!(%TagAddress{
      identity_id: identity.id,
      name: "migration address #{index}",
      address_hash: address,
      address_hash_hash: to_string(address)
    })

    repo.insert!(%TagTransaction{
      identity_id: identity.id,
      name: "migration transaction #{index}",
      transaction_hash: transaction,
      transaction_hash_hash: to_string(transaction)
    })

    repo.insert!(%CustomABI{
      identity_id: identity.id,
      name: "migration ABI #{index}",
      address_hash: address,
      address_hash_hash: to_string(address),
      abi: [
        %{
          "type" => "function",
          "name" => "balanceOf",
          "inputs" => [%{"name" => "owner", "type" => "address"}],
          "outputs" => [%{"type" => "uint256"}],
          "stateMutability" => "view"
        }
      ]
    })

    watchlist = repo.insert!(%Watchlist{identity_id: identity.id, name: "migration watchlist #{index}"})

    repo.insert!(%WatchlistAddress{
      watchlist_id: watchlist.id,
      name: "migration watch #{index}",
      address_hash: address,
      address_hash_hash: to_string(address)
    })
  end

  defp snapshot do
    Map.new(@schemas, fn schema ->
      table = schema.__schema__(:source)
      keys = Enum.map_join(schema.__schema__(:primary_key), ", ", &Atom.to_string/1)
      result = Ecto.Adapters.SQL.query!(Repo.account_repo(), "SELECT * FROM #{table} ORDER BY #{keys}", [])
      {schema, {result.columns, result.rows}}
    end)
  end

  defp drain_queries(queries) do
    receive do
      {:preview_query, query} -> drain_queries([query | queries])
    after
      0 -> queries
    end
  end
end
