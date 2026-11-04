defmodule Mix.Tasks.Auth0ToKeycloakMigrate.Preview do
  @moduledoc false

  import Ecto.Query

  alias Explorer.Account.{Api.Key, CustomABI, Identity, TagAddress, TagTransaction, Watchlist, WatchlistAddress}
  alias Explorer.{HttpClient, Repo}
  alias Explorer.ThirdPartyIntegrations.Auth0
  alias Explorer.ThirdPartyIntegrations.Auth0.Internal, as: Auth0Internal
  alias OAuth2.Client
  alias Ueberauth.Strategy.Auth0.OAuth

  require Logger

  def run(opts) do
    identities = load_identities()
    users = source_users(opts[:auth0_export])
    plan = plan(identities, users)

    Enum.each(plan.merges, fn %{email: email, primary: primary, duplicates: duplicates} ->
      Logger.info("[DRY RUN] Would merge duplicate email #{email}: #{inspect(duplicates)} into identity #{primary}")
    end)

    Enum.each(plan.items, fn item ->
      Logger.info(
        "[DRY RUN] Identity #{item.identity_id}: #{item.auth0_id} -> " <>
          "email=#{inspect(item.email)}, address=#{inspect(item.address)}, " <>
          "username=#{item.username}, has_user_data=#{item.has_user_data}"
      )
    end)

    total = Repo.account_repo().aggregate(Identity, :count)
    deleted = total - MapSet.size(plan.protected_ids)

    Logger.info("[DRY RUN] Would delete #{deleted} identities, including proposed duplicate merges (#{total} total)")
    []
  end

  def plan(identities, users) do
    {items, preserved} =
      Enum.reduce(identities, {[], []}, fn %{identity: identity, has_user_data: has_data}, {items, preserved} ->
        with {:ok, user} <- Map.fetch(users, identity.uid),
             source = user |> Auth0Internal.create_auth() |> Identity.new_identity(),
             true <- has_data || source.address_hash != nil,
             username when not is_nil(username) <-
               source.email || (source.address_hash && String.downcase(to_string(source.address_hash))) do
          item = %{
            identity_id: identity.id,
            auth0_id: identity.uid,
            email: source.email,
            address: source.address_hash,
            username: username,
            has_user_data: has_data
          }

          {[item | items], preserved}
        else
          _ -> {items, if(has_data, do: [identity.id | preserved], else: preserved)}
        end
      end)

    {with_email, without_email} = Enum.split_with(items, &(&1.email != nil))

    {deduplicated, merges} =
      with_email
      |> Enum.group_by(& &1.email)
      |> Enum.sort_by(&elem(&1, 0))
      |> Enum.reduce({without_email, []}, fn {email, group}, {items, merges} ->
        [primary | duplicates] =
          Enum.sort_by(group, &{is_nil(&1.address), !&1.has_user_data})

        merges =
          if duplicates == [] do
            merges
          else
            [%{email: email, primary: primary.identity_id, duplicates: Enum.map(duplicates, & &1.identity_id)} | merges]
          end

        {[primary | items], merges}
      end)

    %{
      items: Enum.sort_by(deduplicated, & &1.identity_id),
      merges: Enum.reverse(merges),
      protected_ids: MapSet.new(preserved ++ Enum.map(deduplicated, & &1.identity_id))
    }
  end

  defp source_users(nil) do
    unless Auth0.enabled?(), do: Mix.raise("Auth0 is not configured. Cannot read source users.")

    fields =
      ~w(user_id email email_verified username nickname name given_name family_name picture user_metadata app_metadata identities)

    body = %{format: "json", fields: Enum.map(fields, &%{name: &1})}

    case Client.post(auth0_client(), "/api/v2/jobs/users-exports", body, [{"content-type", "application/json"}]) do
      {:ok, %OAuth2.Response{status_code: 200, body: %{"id" => job_id}}} ->
        location = poll_export(job_id, 0)

        case HttpClient.get(location, []) do
          {:ok, %{status_code: 200, body: data}} -> parse_export(data)
          error -> Mix.raise("Cannot download Auth0 export: #{inspect(error)}")
        end

      error ->
        Mix.raise("Cannot create Auth0 export: #{inspect(error)}")
    end
  end

  defp source_users(path), do: path |> File.read!() |> parse_export()

  defp auth0_client do
    case Auth0.get_m2m_jwt() do
      token when is_binary(token) -> OAuth.client(token: token)
      _ -> Mix.raise("Cannot obtain Auth0 management token for preview")
    end
  end

  defp poll_export(_job_id, waited_ms) when waited_ms >= 1_800_000 do
    Mix.raise("Auth0 export timed out after 1800s")
  end

  defp poll_export(job_id, waited_ms) do
    case Client.get(auth0_client(), "/api/v2/jobs/#{job_id}") do
      {:ok, %OAuth2.Response{status_code: 200, body: %{"status" => "completed", "location" => location}}} ->
        location

      {:ok, %OAuth2.Response{status_code: 200, body: %{"status" => "failed"} = response}} ->
        Mix.raise("Auth0 export failed: #{inspect(response)}")

      {:ok, %OAuth2.Response{status_code: 200, body: %{"status" => _status} = response}} ->
        delay =
          case response["time_left_seconds"] do
            seconds when is_number(seconds) and seconds > 0 ->
              seconds |> Kernel.*(250) |> round() |> max(3_000) |> min(30_000)

            _ ->
              3_000
          end

        Process.sleep(delay)
        poll_export(job_id, waited_ms + delay)

      error ->
        Mix.raise("Cannot poll Auth0 export: #{inspect(error)}")
    end
  end

  defp parse_export(data) do
    text = if String.starts_with?(data, <<31, 139>>), do: :zlib.gunzip(data), else: data

    text
    |> String.split("\n", trim: true)
    |> Map.new(fn line ->
      %{"user_id" => uid} = user = Jason.decode!(line)
      {uid, user}
    end)
  end

  defp load_identities do
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

    where_condition = dynamic([identity: i], ^has_user_data or not is_nil(i.email))
    selection = dynamic([identity: i], %{identity: i, has_user_data: ^has_user_data})

    Repo.account_repo().all(from(i in Identity, as: :identity, where: ^where_condition, select: ^selection))
  end
end
