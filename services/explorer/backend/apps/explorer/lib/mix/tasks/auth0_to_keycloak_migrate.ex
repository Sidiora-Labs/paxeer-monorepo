defmodule Mix.Tasks.Auth0ToKeycloakMigrate do
  @moduledoc """
  Migrates users from Auth0 to Keycloak.

  Reads all account identities with Auth0 UIDs, fetches their Auth0 user data,
  creates corresponding Keycloak users, and updates the identity UIDs.

  ## Usage

      mix auth0_to_keycloak_migrate [--dry-run] [--batch-size N]

  ## Options

    * `--dry-run` - Preview migration without making changes
    * `--auth0-export` - Read an existing Auth0 JSON-lines export for an offline dry-run
    * `--no-start` - Use applications already started by the caller
    * `--batch-size` - Number of users per batch, from 1 to 1000 (default: 500)

  ## Prerequisites

  Both Auth0 and Keycloak must be configured via environment variables.
  Account access is automatically disabled during migration and re-enabled after.
  """

  use Mix.Task

  alias Explorer.Account.Auth0ToKeycloakMigration
  alias Explorer.ThirdPartyIntegrations.{Auth0, Keycloak}
  alias Mix.Tasks.Auth0ToKeycloakMigrate.Preview
  alias Mix.Task, as: MixTask

  @shortdoc "Migrate users from Auth0 to Keycloak"
  @default_batch_size 500
  @max_batch_size 1000

  @impl MixTask
  def run(args) do
    {opts, positional, invalid} =
      OptionParser.parse(args,
        strict: [dry_run: :boolean, batch_size: :integer, auth0_export: :string, start: :boolean],
        aliases: [n: :dry_run, b: :batch_size]
      )

    if invalid != [] do
      Mix.raise("Invalid migration options: #{inspect(invalid)}")
    end

    if positional != [] do
      Mix.raise("Unexpected migration arguments: #{inspect(positional)}")
    end

    batch_size = Keyword.get(opts, :batch_size, @default_batch_size)

    unless batch_size in 1..@max_batch_size do
      Mix.raise("--batch-size must be an integer from 1 to #{@max_batch_size}")
    end

    if opts[:auth0_export] && opts[:dry_run] != true do
      Mix.raise("--auth0-export requires --dry-run")
    end

    if Keyword.get(opts, :start, true), do: MixTask.run("app.start", [])

    if opts[:dry_run] do
      Preview.run(opts)
    else
      unless Auth0.enabled?(), do: Mix.raise("Auth0 is not configured. Cannot read source users.")
      unless Keycloak.enabled?(), do: Mix.raise("Keycloak is not configured. Cannot create target users.")

      opts
      |> Keyword.put(:batch_size, batch_size)
      |> Keyword.delete(:start)
      |> Auth0ToKeycloakMigration.run()
      |> migration_result!()
    end
  end

  defp migration_result!({:ok, results}) when is_list(results), do: successful_results!(results)
  defp migration_result!({:error, reason}), do: Mix.raise("Migration failed: #{inspect(reason)}")

  defp migration_result!([]) do
    Mix.raise("Migration did not report an explicit success status. Review migration errors before retrying.")
  end

  defp migration_result!([_ | _] = results), do: successful_results!(results)
  defp migration_result!(_), do: Mix.raise("Migration returned an invalid result")

  defp successful_results!(results) do
    if Enum.all?(results, &match?({:ok, source, target} when is_binary(source) and is_binary(target), &1)) do
      results
    else
      Mix.raise("Migration reported failed or invalid identity results. Review migration errors before retrying.")
    end
  end
end
