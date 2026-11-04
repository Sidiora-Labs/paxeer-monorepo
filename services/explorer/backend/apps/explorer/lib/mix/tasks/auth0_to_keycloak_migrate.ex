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
    * `--batch-size` - Number of users per batch (default: 50)

  ## Prerequisites

  Both Auth0 and Keycloak must be configured via environment variables.
  Account access is automatically disabled during migration and re-enabled after.
  """

  use Mix.Task

  alias Explorer.Account.Auth0ToKeycloakMigration
  alias Mix.Tasks.Auth0ToKeycloakMigrate.Preview
  alias Mix.Task, as: MixTask

  @shortdoc "Migrate users from Auth0 to Keycloak"

  @impl MixTask
  def run(args) do
    {opts, _, _} =
      OptionParser.parse(args,
        switches: [dry_run: :boolean, batch_size: :integer, auth0_export: :string],
        aliases: [n: :dry_run, b: :batch_size]
      )

    if opts[:auth0_export] && opts[:dry_run] != true do
      Mix.raise("--auth0-export requires --dry-run")
    end

    MixTask.run("app.start", args)

    if opts[:dry_run] do
      Preview.run(opts)
    else
      Auth0ToKeycloakMigration.run(opts)
    end
  end
end
