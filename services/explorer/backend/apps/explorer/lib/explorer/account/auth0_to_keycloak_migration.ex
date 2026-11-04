defmodule Explorer.Account.Auth0ToKeycloakMigration do
  @moduledoc """
  Migrates users from Auth0 to Keycloak in bulk using batch APIs.

  Supports merging multiple Auth0 tenants (different Blockscout instances) into
  a single Keycloak realm. When a user with the same email already exists in
  Keycloak (from a previous tenant migration), the new address is appended to
  the existing user's multivalued `address` attribute. Each Blockscout identity
  gets the Keycloak ID of the user matching their email.

  ## Phases
  1. Exports all Auth0 users via async job (~5 API calls instead of N)
  2. Matches DB identities with Auth0 export data (in-memory)
  3. Batch-imports users into Keycloak via partial import (~N/500 API calls)
  4. Updates identity UIDs from Auth0 ID to Keycloak UUID
  5. Deletes non-migrated identities (no user data, cascades via DB foreign keys)

  ## Usage

  ### Development (Mix)

      mix auth0_to_keycloak_migrate --dry-run
      mix auth0_to_keycloak_migrate --batch-size 200

  ### Production (Release)

  Configure `:checkpoint_path` for this module on persistent storage before
  applying. The encrypted checkpoint records source/target mappings and the
  provider transition; authentication reads it after restart. Use `resume/0`
  to continue an interrupted cutover. Run via `rpc` on the serving node so the
  runtime account flag also reflects the durable access state.

      bin/blockscout rpc "Explorer.Account.Auth0ToKeycloakMigration.run(dry_run: true)"
      bin/blockscout rpc "Explorer.Account.Auth0ToKeycloakMigration.run(batch_size: 200)"

  ## Options
  - `:dry_run` - When true, logs what would happen without making changes (default: false)
  - `:batch_size` - Number of users per Keycloak partial import batch (default: 500)

  ## Memory

  The Auth0 export is loaded into memory. For ~800k users expect ~1-2GB RAM usage.
  """
  use Utils.RuntimeEnvHelper,
    keycloak_domain: [:explorer, [Explorer.ThirdPartyIntegrations.Keycloak, :domain]],
    keycloak_realm: [:explorer, [Explorer.ThirdPartyIntegrations.Keycloak, :realm]],
    keycloak_client_id: [:explorer, [Explorer.ThirdPartyIntegrations.Keycloak, :client_id]],
    keycloak_client_secret: [:explorer, [Explorer.ThirdPartyIntegrations.Keycloak, :client_secret]],
    chain_id: [:block_scout_web, :chain_id]

  import Ecto.Query

  alias Explorer.{Account, HttpClient, Repo}

  alias Explorer.Account.{
    Api.Key,
    CustomABI,
    Identity,
    MigrationCheckpoint,
    TagAddress,
    TagTransaction,
    Watchlist,
    WatchlistAddress
  }

  alias Explorer.ThirdPartyIntegrations.{Auth0, Keycloak}
  alias Explorer.ThirdPartyIntegrations.Auth0.Internal, as: Auth0Internal
  alias OAuth2.Client
  alias Ueberauth.Strategy.Auth0.OAuth

  require Logger

  @json_headers [{"content-type", "application/json"}]
  @auth0_json_headers [{"Content-type", "application/json"}]

  # Auth0 export job
  @export_min_poll_interval_ms 3_000
  @export_max_poll_interval_ms 30_000
  @export_max_total_wait_ms 1_800_000

  # Keycloak partial import
  @default_batch_size 500
  @batch_pause_ms 1_000

  @spec run(keyword()) :: [{:ok, String.t(), String.t()} | {:error, String.t(), any()}]
  def run(opts \\ []) do
    result =
      case {Keyword.get(opts, :dry_run, false), MigrationCheckpoint.load()} do
        {false, {:ok, state}} when not is_nil(state) ->
          resume()

        {false, {:error, reason}} ->
          {:error, reason}

        _ ->
          with :ok <- check_auth0_configured(),
               :ok <- check_keycloak_configured() do
            do_run(opts)
          else
            {:error, message} ->
              Logger.error(message)
              []
          end
      end

    case result do
      {:error, reason} -> [{:error, "cutover", reason}]
      results -> results
    end
  end

  defp check_auth0_configured do
    if Auth0.enabled?(), do: :ok, else: {:error, "Auth0 is not configured. Cannot read source users."}
  end

  defp check_keycloak_configured do
    if Keycloak.enabled?(), do: :ok, else: {:error, "Keycloak is not configured. Cannot create target users."}
  end

  @doc "Runs a migration from a downloaded Auth0 NDJSON export; previews by default."
  def run_export(body, opts \\ []) do
    opts = Keyword.put_new(opts, :dry_run, true)

    do_run(opts, fn -> parse_export(body) end)
  end

  defp do_run(opts, source \\ &export_auth0_users/0) do
    if Keyword.get(opts, :dry_run, false) do
      with {:ok, users} <- source.(),
           {:ok, plan} <- plan(users),
           {:ok, results, _} <- preview_plan(plan) do
        results
      else
        {:error, reason} ->
          Logger.error("Migration preview failed: #{inspect(reason)}")
          []
      end
    else
      :global.trans({{__MODULE__, :cutover}, self()}, fn ->
        with {:ok, state} <- MigrationCheckpoint.load() do
          if state do
            continue_cutover(state)
          else
            start_cutover(source, Keyword.get(opts, :batch_size, @default_batch_size))
          end
        else
          {:error, reason} -> {:error, reason}
        end
      end)
    end
  end

  @doc "Resumes the saved cutover without exporting or importing already applied identities again."
  def resume do
    :global.trans({{__MODULE__, :cutover}, self()}, fn ->
      case MigrationCheckpoint.load() do
        {:ok, nil} -> {:error, :checkpoint_missing}
        {:ok, state} -> continue_cutover(state)
        {:error, reason} -> {:error, reason}
      end
    end)
  end

  defp start_cutover(source, batch_size) do
    with {:ok, users} <- source.(),
         {:ok, plan} <- plan(users),
         :ok <- check_keycloak_configured() do
      protected_ids =
        plan.items
        |> MapSet.new(& &1.identity_id)
        |> MapSet.union(MapSet.new(plan.skipped_with_data))

      state = %{
        version: 1,
        phase: :importing,
        provider: :auth0,
        account_access: :disabled,
        plan: plan,
        batch_size: batch_size,
        protected_ids: protected_ids,
        keycloak_map: %{},
        source_uids: source_uids(plan),
        identity_mapping: %{},
        results: [],
        completed_merges: [],
        error: nil,
        revocation: %{pattern: session_pattern(), cursor: "0", pending: nil, count: 0}
      }

      with :ok <- MigrationCheckpoint.save(state), do: continue_cutover(state)
    else
      {:error, reason} ->
        Logger.error("Migration planning failed: #{inspect(reason)}")
        []
    end
  end

  defp continue_cutover(%{phase: :complete} = state) do
    with :keycloak <- MigrationCheckpoint.access(),
         :ok <- check_keycloak_configured() do
      disable_auth0()
      enable_account()
      state.results
    else
      error -> {:error, {:provider_transition, error}}
    end
  end

  defp continue_cutover(state) do
    disable_account()

    case advance_cutover(state) do
      {:ok, next} ->
        continue_cutover(next)

      {:error, reason} ->
        with {:ok, saved} when not is_nil(saved) <- MigrationCheckpoint.load(),
             :ok <- MigrationCheckpoint.save(%{saved | error: reason}) do
          {:error, reason}
        else
          error -> {:error, {reason, error}}
        end
    end
  end

  defp advance_cutover(%{phase: :importing} = state) do
    with :ok <- check_keycloak_configured() do
      keycloak_map = batch_import_to_keycloak(state.plan.items, state.batch_size)
      mapping = identity_mapping(state, keycloak_map)
      save_next(%{state | phase: :applying, keycloak_map: keycloak_map, identity_mapping: mapping, error: nil})
    end
  end

  defp advance_cutover(%{phase: :applying} = state) do
    # A process can stop between the account commit and checkpoint sync. The
    # saved source-to-target mapping distinguishes that commit from a rollback.
    result =
      if applied?(state) do
        {:ok, Enum.map(state.plan.items, &{:ok, &1.auth0_id, Map.fetch!(state.keycloak_map, &1.identity_id)})}
      else
        apply_plan(state.plan, state.keycloak_map)
      end

    with {:ok, results} <- result do
      save_next(%{state | phase: :revoking, results: results, completed_merges: state.plan.merges, error: nil})
    end
  end

  defp advance_cutover(%{phase: :revoking} = state) do
    delete_non_migrated_identities(state.protected_ids)

    # Restart from zero on recovery: Redis cursors are not durable snapshots.
    # A saved unacknowledged DEL is retried before starting the fresh scan.
    with {:ok, revocation} <- delete_pending(state.revocation),
         :ok <- MigrationCheckpoint.save(%{state | revocation: %{revocation | cursor: "0"}}) do
      scan_and_delete(%{state | revocation: %{revocation | cursor: "0"}})
    end
  end

  defp source_uids(plan) do
    ids = Enum.map(plan.items, & &1.identity_id) ++ List.flatten(plan.merges)
    Repo.account_repo().all(from(i in Identity, where: i.id in ^ids, select: {i.id, i.uid})) |> Map.new()
  end

  defp identity_mapping(state, keycloak_map) do
    primary_ids =
      Enum.reduce(state.plan.merges, %{}, fn [primary | _] = group, acc ->
        Enum.reduce(group, acc, &Map.put(&2, &1, primary))
      end)

    Map.new(state.source_uids, fn {id, uid} ->
      primary = Map.get(primary_ids, id, id)
      {id, %{auth0_id: uid, keycloak_id: Map.get(keycloak_map, primary), primary_id: primary}}
    end)
  end

  defp applied?(state) do
    Enum.all?(state.plan.items, fn item ->
      target = Map.get(state.keycloak_map, item.identity_id)
      identity = Repo.account_repo().get(Identity, item.identity_id)
      is_binary(target) and not is_nil(identity) and identity.uid == target
    end) and
      Enum.all?(state.plan.merges, fn [_primary | secondary] ->
        Enum.all?(secondary, &is_nil(Repo.account_repo().get(Identity, &1)))
      end)
  end

  defp save_next(state) do
    with :ok <- MigrationCheckpoint.save(state), do: {:ok, state}
  end

  @doc "Builds a read-only plan. Unverified email collisions require operator resolution."
  def plan(auth0_users) do
    Logger.info("Phase 2: Loading identities and matching with Auth0 data...")
    {items, skipped_with_data} = build_migration_items(load_auth0_identities(), auth0_users)

    with {:ok, planned_items, merges} <- plan_duplicate_emails(items) do
      log_duplicate_addresses(planned_items)
      {:ok, %{items: planned_items, merges: merges, skipped_with_data: skipped_with_data}}
    end
  end

  defp preview_plan(plan) do
    protected_ids =
      plan.items
      |> MapSet.new(& &1.identity_id)
      |> MapSet.union(MapSet.new(plan.skipped_with_data))
      |> MapSet.union(MapSet.new(List.flatten(plan.merges)))

    log_dry_run(plan.items)
    Enum.each(plan.merges, &Logger.info("[DRY RUN] Would merge identities #{inspect(&1)}"))
    log_dry_run_deletion_count(protected_ids)
    {:ok, [], protected_ids}
  end

  @doc "Atomically applies approved merges and imported UIDs; any failure rolls back all account writes."
  def apply_plan(plan, keycloak_map) do
    Repo.account_repo().transaction(fn ->
      Enum.each(plan.merges, fn ids ->
        case merge_group(ids) do
          {:ok, _} -> :ok
          {:error, operation, reason, _} -> Repo.account_repo().rollback({:merge_failed, ids, operation, reason})
          {:error, reason} -> Repo.account_repo().rollback({:merge_failed, ids, reason})
        end
      end)

      results = update_identities(plan.items, keycloak_map)

      if Enum.any?(results, &match?({:error, _, _}, &1)) do
        Repo.account_repo().rollback({:identity_updates_failed, results})
      end

      results
    end)
  end

  defp merge_group(ids) do
    ids
    |> Enum.reduce(Ecto.Multi.new(), fn id, multi ->
      Ecto.Multi.run(multi, {:acquire_identity, id}, fn repo, _ ->
        case repo.one(from(i in Identity, where: i.id == ^id, lock: "FOR UPDATE")) do
          nil -> {:error, :not_found}
          identity -> {:ok, identity}
        end
      end)
    end)
    |> Ecto.Multi.run(:merge, fn _repo, locked ->
      case Account.merge(Enum.map(ids, &Map.fetch!(locked, {:acquire_identity, &1}))) do
        {{:ok, changes}, _} -> {:ok, changes}
        {{:error, operation, reason, _}, _} -> {:error, {operation, reason}}
        {{:error, reason}, _} -> {:error, reason}
      end
    end)
    |> Repo.account_repo().transaction()
  end

  defp export_auth0_users do
    with {:ok, job_id} <- create_export_job(),
         {:ok, location} <- poll_export_job(job_id) do
      download_and_parse_export(location)
    end
  end

  defp create_export_job do
    with token when is_binary(token) <- Auth0.get_m2m_jwt(),
         client = OAuth.client(token: token),
         {:ok, %OAuth2.Response{status_code: 200, body: %{"id" => job_id}}} <-
           Client.post(client, "/api/v2/jobs/users-exports", export_job_body(), @auth0_json_headers) do
      Logger.info("Auth0 export job created: #{job_id}")
      {:ok, job_id}
    else
      nil -> {:error, "Failed to get Auth0 M2M JWT"}
      error -> {:error, "Failed to create Auth0 export job: #{inspect(error)}"}
    end
  end

  defp export_job_body do
    %{
      format: "json",
      fields: [
        %{name: "user_id"},
        %{name: "email"},
        %{name: "email_verified"},
        %{name: "username"},
        %{name: "nickname"},
        %{name: "name"},
        %{name: "given_name"},
        %{name: "family_name"},
        %{name: "picture"},
        %{name: "user_metadata"},
        %{name: "app_metadata"},
        %{name: "identities"}
      ]
    }
  end

  defp poll_export_job(job_id, waited_ms \\ 0)

  defp poll_export_job(_job_id, waited_ms) when waited_ms >= @export_max_total_wait_ms do
    {:error, "Auth0 export job timed out after #{div(@export_max_total_wait_ms, 1000)}s"}
  end

  defp poll_export_job(job_id, waited_ms) do
    with token when is_binary(token) <- Auth0.get_m2m_jwt(),
         client = OAuth.client(token: token),
         {:ok, %OAuth2.Response{status_code: 200, body: body}} <-
           Client.get(client, "/api/v2/jobs/#{job_id}") do
      case body do
        %{"status" => "completed", "location" => location} ->
          Logger.info("Auth0 export job completed")
          {:ok, location}

        %{"status" => "failed"} ->
          {:error, "Auth0 export job failed: #{inspect(body)}"}

        %{"status" => status} = response ->
          sleep_ms = poll_interval_from_estimate(response)

          Logger.info(
            "Auth0 export status: #{status}, " <>
              "estimated time left: #{response["time_left_seconds"] || "unknown"}s, " <>
              "polling again in #{div(sleep_ms, 1000)}s (waited #{div(waited_ms, 1000)}s total)"
          )

          Process.sleep(sleep_ms)
          poll_export_job(job_id, waited_ms + sleep_ms)
      end
    else
      nil -> {:error, "Failed to get Auth0 M2M JWT"}
      error -> {:error, "Failed to poll Auth0 export job: #{inspect(error)}"}
    end
  end

  defp poll_interval_from_estimate(%{"time_left_seconds" => seconds}) when is_number(seconds) and seconds > 0 do
    (seconds * 250)
    |> round()
    |> max(@export_min_poll_interval_ms)
    |> min(@export_max_poll_interval_ms)
  end

  defp poll_interval_from_estimate(_response), do: @export_min_poll_interval_ms

  defp download_and_parse_export(location) do
    Logger.info("Downloading Auth0 export...")

    case HttpClient.get(location, []) do
      {:ok, %{status_code: 200, body: body}} ->
        parse_export(body)

      {:ok, %{status_code: status, body: resp_body}} ->
        {:error, "Failed to download Auth0 export: HTTP #{status}: #{resp_body}"}

      {:error, reason} ->
        {:error, "Failed to download Auth0 export: #{inspect(reason)}"}
    end
  end

  defp parse_export(body) do
    users_map =
      body
      |> safe_gunzip()
      |> String.split("\n", trim: true)
      |> Map.new(fn line ->
        %{"user_id" => user_id} = user = Jason.decode!(line)
        {user_id, user}
      end)

    Logger.info("Parsed #{map_size(users_map)} users from Auth0 export")
    {:ok, users_map}
  end

  # Auth0 export files are gzipped. If content is already decompressed
  # (e.g. HTTP client handled Content-Encoding), pass through as-is.
  defp safe_gunzip(data) do
    :zlib.gunzip(data)
  rescue
    ErlangError -> data
  end

  defp load_auth0_identities do
    has_user_data =
      dynamic(
        [identity: i],
        exists(from(ta in TagAddress, where: ta.identity_id == parent_as(:identity).id, select: 1)) or
          exists(from(tt in TagTransaction, where: tt.identity_id == parent_as(:identity).id, select: 1)) or
          exists(from(ca in CustomABI, where: ca.identity_id == parent_as(:identity).id, select: 1)) or
          exists(from(ak in Key, where: ak.identity_id == parent_as(:identity).id, select: 1)) or
          exists(
            from(wa in WatchlistAddress,
              join: w in Watchlist,
              on: wa.watchlist_id == w.id,
              where: w.identity_id == parent_as(:identity).id,
              select: 1
            )
          ) or i.plan_id != 1
      )

    where_condition =
      dynamic(
        [identity: i],
        ^has_user_data or not is_nil(i.email)
      )

    dynamic_select = dynamic([identity: i], %{identity: i, has_user_data: ^has_user_data})

    q =
      from(i in Identity,
        as: :identity,
        where: ^where_condition,
        select: ^dynamic_select
      )

    Repo.account_repo().all(q)
  end

  defp build_migration_items(identities, auth0_users) do
    {items, skipped, skipped_with_data} =
      Enum.reduce(identities, {[], 0, []}, fn entry, {acc, skip_count, data_acc} ->
        case build_migration_item(entry, auth0_users) do
          {:ok, item} ->
            {[item | acc], skip_count, data_acc}

          {:skip_with_data, id} ->
            {acc, skip_count + 1, [id | data_acc]}

          :skip ->
            {acc, skip_count + 1, data_acc}
        end
      end)

    if skipped > 0,
      do: Logger.warning("Skipped #{skipped} identities (not found in Auth0 export, no data, or no email+address)")

    if skipped_with_data != [] do
      Logger.warning(
        "#{length(skipped_with_data)} identities with user data were NOT found in Auth0 export " <>
          "and will be preserved (not deleted): #{inspect(skipped_with_data)}"
      )
    end

    {items, skipped_with_data}
  end

  defp plan_duplicate_emails(items) do
    groups = Enum.group_by(items, fn item -> if item.email, do: {:email, item.email}, else: {:id, item.identity_id} end)

    Enum.reduce_while(groups, {:ok, [], []}, fn {_key, group}, {:ok, planned, merges} ->
      sorted = Enum.sort_by(group, fn item -> {is_nil(item.address), !item.has_user_data, item.identity_id} end)
      [primary | _] = sorted

      cond do
        length(group) == 1 ->
          {:cont, {:ok, [primary | planned], merges}}

        Enum.all?(group, & &1.email_verified) ->
          {:cont, {:ok, [primary | planned], [Enum.map(sorted, & &1.identity_id) | merges]}}

        true ->
          {:halt, {:error, {:unverified_email_collision, primary.email, Enum.map(sorted, & &1.identity_id)}}}
      end
    end)
  end

  defp log_duplicate_addresses(items) do
    items
    |> Enum.filter(& &1.address)
    |> Enum.group_by(&to_string(&1.address))
    |> Enum.filter(fn {_addr, group} -> length(group) > 1 end)
    |> Enum.each(fn {address, group} ->
      ids = Enum.map(group, & &1.identity_id)

      Logger.warning(
        "Duplicate address within same instance: #{address}, identity IDs: #{inspect(ids)}. " <>
          "Manual resolution required."
      )
    end)
  end

  defp build_migration_item(%{identity: %{id: id, uid: auth0_id}, has_user_data: has_user_data}, auth0_users) do
    with {:ok, user} <- Map.fetch(auth0_users, auth0_id),
         identity = user |> Auth0Internal.create_auth() |> Identity.new_identity(),
         true <- has_user_data || identity.address_hash != nil,
         # Every Auth0 user should have either email or address; skip anomalies.
         keycloak_username when not is_nil(keycloak_username) <-
           identity.email || (identity.address_hash && String.downcase(to_string(identity.address_hash))) do
      {:ok,
       %{
         identity_id: id,
         auth0_id: auth0_id,
         email: normalize_email(identity.email),
         email_verified:
           user["email_verified"] == true and
             normalize_email(user["email"]) == normalize_email(identity.email) and not is_nil(identity.email),
         address: identity.address_hash,
         username: if(identity.email, do: normalize_email(identity.email), else: keycloak_username),
         has_user_data: has_user_data
       }}
    else
      :error ->
        Logger.warning("Auth0 user not found in export for identity #{id} (#{auth0_id})")
        if has_user_data, do: {:skip_with_data, id}, else: :skip

      nil ->
        Logger.warning("Identity #{id} (#{auth0_id}) has no email and no address in Auth0, skipping")
        if has_user_data, do: {:skip_with_data, id}, else: :skip

      false ->
        Logger.debug("No user data and no address in Auth0 for identity #{id} (#{auth0_id}), skipping")
        :skip
    end
  end

  defp normalize_email(nil), do: nil
  defp normalize_email(email), do: email |> String.trim() |> String.downcase()

  # Creates or finds Keycloak users for each migration item.
  # Across tenants, multiple identities with the same email map to one Keycloak user.
  # Returns %{identity_id => keycloak_id}.
  defp batch_import_to_keycloak(items, batch_size) do
    # Pre-check which addresses already exist in Keycloak (from previous tenant migrations).
    # These addresses will be excluded from user bodies to preserve uniqueness.
    taken_addresses = pre_check_addresses(items)
    keycloak_users = Enum.map(items, &build_keycloak_user(&1, taken_addresses))

    Logger.info("#{length(keycloak_users)} Keycloak users to import")

    num_batches = ceil(length(keycloak_users) / batch_size)

    bodies_by_username = Map.new(keycloak_users)

    username_to_keycloak_id =
      keycloak_users
      |> Enum.chunk_every(batch_size)
      |> Enum.with_index(1)
      |> Enum.flat_map(fn {batch, batch_num} ->
        Logger.info("Keycloak import batch #{batch_num}/#{num_batches}")
        results = import_batch(batch)
        unless batch_num == num_batches, do: Process.sleep(@batch_pause_ms)
        results
      end)
      |> Map.new()
      |> resolve_missing_ids(bodies_by_username)

    # Convert username → keycloak_id into identity_id → keycloak_id
    Map.new(items, fn %{identity_id: id, username: username} ->
      {id, Map.get(username_to_keycloak_id, username)}
    end)
  end

  @doc false
  def build_keycloak_user(
        %{username: username, email: email, email_verified: verified, address: address},
        taken_addresses
      ) do
    normalized_address = if address, do: String.downcase(to_string(address))

    # Exclude address if already claimed by another Keycloak user
    use_address =
      if normalized_address && MapSet.member?(taken_addresses, normalized_address) do
        Logger.info(
          "Address #{normalized_address} already exists in Keycloak, " <>
            "not assigning to user #{username} (#{email || "no email"})"
        )

        nil
      else
        normalized_address
      end

    body =
      %{username: username, enabled: true}
      |> then(fn body -> if email, do: Map.merge(body, %{email: email, emailVerified: verified}), else: body end)
      |> then(fn body ->
        if use_address,
          do: Map.put(body, :attributes, %{address: [use_address]}),
          else: body
      end)

    {username, body}
  end

  # Checks which addresses from migration items already exist in Keycloak.
  # Returns a MapSet of lowercased addresses that are already claimed.
  defp pre_check_addresses(items) do
    addresses =
      items
      |> Enum.filter(& &1.address)
      |> Enum.map(&String.downcase(to_string(&1.address)))
      |> Enum.uniq()

    if Enum.empty?(addresses) do
      MapSet.new()
    else
      Logger.info("Pre-checking #{length(addresses)} addresses against Keycloak...")

      taken = Enum.filter(addresses, &address_exists_in_keycloak?/1)

      if taken != [] do
        Logger.info("#{length(taken)} addresses already exist in Keycloak and will be skipped")
      end

      MapSet.new(taken)
    end
  end

  defp address_exists_in_keycloak?(address) do
    match?({:ok, [_ | _]}, Keycloak.find_users_by_address(address))
  end

  defp import_batch(user_entries) do
    users = Enum.map(user_entries, fn {_username, body} -> body end)
    entries_by_username = Map.new(user_entries)

    case keycloak_partial_import(users) do
      {:ok, %{"results" => results}} ->
        Enum.flat_map(results, fn
          %{"resourceName" => username, "id" => keycloak_id, "action" => "ADDED"} ->
            [{username, keycloak_id}]

          %{"resourceName" => username, "id" => keycloak_id, "action" => "SKIPPED"} ->
            [{username, reuse_existing_user(keycloak_id, entries_by_username[username])}]

          %{"resourceName" => username, "action" => "SKIPPED"} ->
            # SKIPPED without ID — will be resolved in resolve_missing_ids
            [{username, nil}]

          other ->
            Logger.warning("Unexpected partial import result: #{inspect(other)}")
            []
        end)

      {:error, reason} ->
        Logger.error("Partial import failed: #{inspect(reason)}, falling back to individual creates")
        individual_create_fallback(user_entries)
    end
  end

  defp individual_create_fallback(user_entries) do
    Enum.map(user_entries, fn {username, body} ->
      {username, create_or_reuse_keycloak_user(body)}
    end)
  end

  defp create_or_reuse_keycloak_user(body) do
    case Keycloak.create_user(body) do
      {:ok, keycloak_id} ->
        keycloak_id

      {:error, "User already exists"} ->
        case lookup_and_append_address(body) do
          {:ok, keycloak_id} -> keycloak_id
          _ -> nil
        end

      error ->
        Logger.error("Failed to create Keycloak user #{body[:username]}: #{inspect(error)}")
        nil
    end
  end

  defp lookup_and_append_address(%{email: email, emailVerified: true} = body) when is_binary(email) do
    case Keycloak.find_users_by_email(email) do
      {:ok, [%{"id" => keycloak_id}]} ->
        case reuse_existing_user(keycloak_id, body) do
          nil -> {:error, :ownership_not_verified}
          id -> {:ok, id}
        end

      _ ->
        {:error, :unresolved_email_collision}
    end
  end

  defp lookup_and_append_address(_body), do: {:error, :ownership_not_verified}

  defp reuse_existing_user(keycloak_id, body) do
    with {:ok, user} <- Keycloak.get_user(keycloak_id),
         true <- verified_common_owner?(body, user),
         :ok <- maybe_append_address(keycloak_id, body) do
      keycloak_id
    else
      _ ->
        Logger.error("Existing Keycloak user #{keycloak_id} requires explicit ownership resolution")
        nil
    end
  end

  @doc false
  def verified_common_owner?(%{email: email, emailVerified: true}, %{"email" => target, "emailVerified" => true})
      when is_binary(email) and is_binary(target),
      do: normalize_email(email) == normalize_email(target)

  def verified_common_owner?(_, _), do: false

  defp resolve_missing_ids(username_to_id, bodies_by_username) do
    Enum.reduce(username_to_id, username_to_id, fn
      {username, nil}, acc ->
        body = Map.fetch!(bodies_by_username, username)

        case lookup_and_append_address(body) do
          {:ok, keycloak_id} -> Map.put(acc, username, keycloak_id)
          _ -> acc
        end

      _, acc ->
        acc
    end)
  end

  # Extracts address from a Keycloak user body and appends it to the existing user.
  defp maybe_append_address(keycloak_id, %{attributes: %{address: [address | _]}}) do
    append_address_to_keycloak_user(keycloak_id, address)
  end

  defp maybe_append_address(_keycloak_id, _body), do: :ok

  # Appends an address to a Keycloak user's multivalued address attribute.
  # Address uniqueness is already guaranteed by pre_check_addresses — addresses
  # claimed by other users were excluded from bodies before import started.
  defp append_address_to_keycloak_user(keycloak_id, address) when is_binary(address) do
    address = String.downcase(to_string(address))

    with {:ok, user} <- Keycloak.get_user(keycloak_id) do
      current_addresses = get_in(user, ["attributes", "address"]) || []

      if address in current_addresses do
        Logger.debug("Address #{address} already on Keycloak user #{keycloak_id}")
        :ok
      else
        new_attributes = Map.put(user["attributes"] || %{}, "address", [address | current_addresses])
        merged = Map.put(user, "attributes", new_attributes)
        Keycloak.update_user(keycloak_id, merged)
      end
    end
  end

  defp append_address_to_keycloak_user(_keycloak_id, _address), do: :ok

  defp update_identities(items, keycloak_map) do
    Enum.map(items, fn %{identity_id: id, auth0_id: auth0_id} ->
      with {:ok, keycloak_id} when is_binary(keycloak_id) <- Map.fetch(keycloak_map, id),
           :ok <- update_identity_uid(id, keycloak_id) do
        {:ok, auth0_id, keycloak_id}
      else
        :error ->
          Logger.error("No Keycloak ID for identity #{id} (#{auth0_id})")
          {:error, auth0_id, :no_keycloak_id}

        {:ok, nil} ->
          Logger.error("No Keycloak ID for identity #{id} (#{auth0_id})")
          {:error, auth0_id, :no_keycloak_id}

        {:error, reason} ->
          Logger.error("Failed to update identity #{id}: #{inspect(reason)}")
          {:error, auth0_id, reason}
      end
    end)
  end

  defp update_identity_uid(id, keycloak_id) do
    case Repo.account_repo().get(Identity, id) do
      nil ->
        {:error, "Identity #{id} not found"}

      identity ->
        identity
        |> Identity.changeset(%{uid: keycloak_id})
        |> Repo.account_repo().update()
        |> case do
          {:ok, _} -> :ok
          {:error, changeset} -> {:error, "Update failed: #{inspect(changeset.errors)}"}
        end
    end
  end

  defp delete_non_migrated_identities(protected_ids) do
    protected_id_list = MapSet.to_list(protected_ids)

    query =
      if Enum.empty?(protected_id_list) do
        from(i in Identity)
      else
        from(i in Identity, where: i.id not in ^protected_id_list)
      end

    {deleted, _} = Repo.account_repo().delete_all(query)
    Logger.info("Deleted #{deleted} non-migrated identities (cascading to their associated data)")
  end

  defp keycloak_partial_import(users) do
    body = %{ifResourceExists: "SKIP", users: users}

    with {:ok, token} <- get_keycloak_admin_token() do
      url = keycloak_url("/admin/realms/#{URI.encode(keycloak_realm())}/partialImport")

      case HttpClient.post(url, Jason.encode!(body), keycloak_auth_headers(token) ++ @json_headers) do
        {:ok, %{status_code: 200, body: resp_body}} ->
          Jason.decode(resp_body)

        {:ok, %{status_code: status, body: resp_body}} ->
          {:error, "HTTP #{status}: #{resp_body}"}

        {:error, reason} ->
          {:error, reason}
      end
    end
  end

  # Caches the admin token in the process dictionary for the duration of the migration.
  defp get_keycloak_admin_token do
    case Process.get(:keycloak_admin_token) do
      {token, expires_at} when is_integer(expires_at) and expires_at > 0 ->
        if System.system_time(:second) < expires_at do
          {:ok, token}
        else
          fetch_keycloak_admin_token()
        end

      _ ->
        fetch_keycloak_admin_token()
    end
  end

  defp fetch_keycloak_admin_token do
    url = keycloak_url("/realms/#{URI.encode(keycloak_realm())}/protocol/openid-connect/token")

    body =
      URI.encode_query(%{
        grant_type: "client_credentials",
        client_id: keycloak_client_id(),
        client_secret: keycloak_client_secret()
      })

    case HttpClient.post(url, body, [{"content-type", "application/x-www-form-urlencoded"}]) do
      {:ok, %{status_code: 200, body: resp_body}} ->
        case Jason.decode(resp_body) do
          {:ok, %{"access_token" => token, "expires_in" => ttl}} ->
            Process.put(:keycloak_admin_token, {token, System.system_time(:second) + ttl - 30})
            {:ok, token}

          _ ->
            {:error, "Invalid Keycloak token response"}
        end

      error ->
        {:error, "Failed to get Keycloak admin token: #{inspect(error)}"}
    end
  end

  defp keycloak_auth_headers(token), do: [{"authorization", "Bearer #{token}"}]

  defp keycloak_url(path) do
    keycloak_domain()
    |> URI.parse()
    |> URI.append_path(path)
    |> URI.to_string()
  end

  defp disable_account do
    Logger.info("Disabling account access for migration")
    update_account_enabled(false)
  end

  defp enable_account do
    update_account_enabled(true)
  end

  defp update_account_enabled(enabled) do
    config = Application.get_env(:explorer, Account, [])
    Application.put_env(:explorer, Account, Keyword.put(config, :enabled, enabled))
  end

  defp disable_auth0 do
    config = Application.get_env(:ueberauth, OAuth, [])
    Application.put_env(:ueberauth, OAuth, Keyword.put(config, :domain, nil))
  end

  defp session_pattern do
    case chain_id() do
      nil -> "*"
      id -> "#{id}_*"
    end
  end

  defp redis_command(command) do
    connection = Application.get_env(:explorer, __MODULE__, [])[:redis] || :redix
    Redix.command(connection, command)
  end

  defp delete_pending(%{pending: nil} = revocation), do: {:ok, revocation}

  defp delete_pending(%{pending: %{keys: keys, next_cursor: next_cursor}} = revocation) do
    case redis_command(["DEL" | keys]) do
      {:ok, deleted} when is_integer(deleted) and deleted >= 0 ->
        {:ok, %{revocation | pending: nil, cursor: next_cursor, count: revocation.count + deleted}}

      error ->
        {:error, {:session_delete, error}}
    end
  end

  defp scan_and_delete(state) do
    revocation = state.revocation

    case redis_command(["SCAN", revocation.cursor, "MATCH", revocation.pattern, "COUNT", 1000]) do
      {:ok, [next_cursor, keys]} when is_binary(next_cursor) and is_list(keys) ->
        pending = if keys == [], do: nil, else: %{keys: keys, next_cursor: next_cursor}
        scanned = %{state | revocation: %{revocation | pending: pending}}

        with :ok <- MigrationCheckpoint.save(scanned),
             {:ok, acknowledged} <- delete_pending(scanned.revocation),
             next = %{scanned | revocation: %{acknowledged | cursor: next_cursor}},
             :ok <- MigrationCheckpoint.save(next) do
          if next_cursor == "0" do
            with :ok <- check_keycloak_configured() do
              save_next(%{next | phase: :complete, provider: :keycloak, account_access: :enabled, error: nil})
            end
          else
            scan_and_delete(next)
          end
        end

      error ->
        {:error, {:session_scan, error}}
    end
  end

  defp log_dry_run_deletion_count(protected_ids) do
    total_count = Repo.account_repo().aggregate(Identity, :count)

    would_delete = total_count - MapSet.size(protected_ids)

    Logger.info(
      "[DRY RUN] Would delete #{would_delete} non-migrated identities " <>
        "(#{total_count} total, #{MapSet.size(protected_ids)} protected)"
    )
  end

  defp log_dry_run(items) do
    Enum.each(items, fn item ->
      identity_label = if Map.has_key?(item, :identity_id), do: "Identity #{item.identity_id}: ", else: ""

      Logger.info(
        "[DRY RUN] #{identity_label}#{item.auth0_id} -> " <>
          "email=#{inspect(item.email)}, address=#{inspect(item.address)}, " <>
          "username=#{item.username}, has_user_data=#{item.has_user_data}"
      )
    end)
  end
end
