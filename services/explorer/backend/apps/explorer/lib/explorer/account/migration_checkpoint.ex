defmodule Explorer.Account.MigrationCheckpoint do
  @moduledoc """
  Durable, encrypted Auth0 cutover state. `:checkpoint_path` must point at storage
  retained across releases and shared by the account-serving nodes. A configured
  but missing, unreadable or incomplete checkpoint closes authentication access.
  """

  alias Explorer.Account.Auth0ToKeycloakMigration
  alias Explorer.Vault

  def configured?, do: not is_nil(path())

  def load do
    case path() do
      nil -> {:ok, nil}
      path -> read(path)
    end
  end

  def save(state) do
    case path() do
      nil -> {:error, :checkpoint_not_configured}
      path -> table(path, :read_write, fn table -> persist(table, path, state) end)
    end
  end

  def access do
    case load() do
      {:ok, %{phase: :complete, provider: :keycloak, account_access: :enabled}} -> :keycloak
      {:ok, nil} -> if configured?(), do: :closed, else: :legacy
      _ -> :closed
    end
  end

  defp path do
    Application.get_env(:explorer, Auth0ToKeycloakMigration, [])[:checkpoint_path]
  end

  defp read(path) do
    case File.stat(path) do
      {:ok, _} -> table(path, :read, &decode/1)
      {:error, :enoent} -> {:ok, nil}
      {:error, reason} -> {:error, {:checkpoint_read, reason}}
    end
  end

  defp table(path, access, fun) do
    :global.trans({__MODULE__, self()}, fn ->
      case :dets.open_file(__MODULE__, file: String.to_charlist(path), access: access, repair: false) do
        {:ok, table} ->
          try do
            result = fun.(table)

            case :dets.close(table) do
              :ok -> result
              {:error, reason} -> {:error, {:checkpoint_close, reason}}
            end
          after
            if :dets.info(table) != :undefined, do: :dets.close(table)
          end

        {:error, reason} ->
          {:error, {:checkpoint_open, reason}}
      end
    end)
  end

  defp persist(table, path, state) do
    with :ok <- File.chmod(path, 0o600),
         {:ok, encrypted} <- Vault.encrypt(:erlang.term_to_binary(state)),
         :ok <- :dets.insert(table, {:migration, encrypted}),
         :ok <- :dets.sync(table) do
      :ok
    else
      {:error, reason} -> {:error, {:checkpoint_write, reason}}
    end
  end

  defp decode(table) do
    case :dets.lookup(table, :migration) do
      [{:migration, encrypted}] ->
        with {:ok, binary} <- Vault.decrypt(encrypted),
             %{version: 1} = state <- :erlang.binary_to_term(binary, [:safe]) do
          {:ok, state}
        else
          _ -> {:error, :invalid_checkpoint}
        end

      _ ->
        {:error, :invalid_checkpoint}
    end
  rescue
    ArgumentError -> {:error, :invalid_checkpoint}
  end
end
