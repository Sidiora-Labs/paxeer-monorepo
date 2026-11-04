defmodule Explorer.Migrator.EmptyBytecodeForSelfdestructedSmartContracts do
  @moduledoc """
  Repairs bytecode for canonical SELFDESTRUCT history using the configured Cancun
  activation and committed chain state. Before Cancun every successful destruction
  can delete code; after Cancun only transaction-local creations can be deleted.

  Code is reconciled at the newest imported canonical block, so historical
  destructions cannot erase a later deployment at the same address.
  """

  use Explorer.Migrator.FillingMigration

  import Ecto.Query

  alias Explorer.Chain.{Address, Block, Import, InternalTransaction, Transaction}
  alias Explorer.Chain.Import.Runner.InternalTransactions
  alias Explorer.Chain.Cache.BlockNumber
  alias Explorer.ChainSpec.GenesisData
  alias Explorer.Migrator.{FillingMigration, MigrationStatus}
  alias Explorer.Repo

  @migration_name "empty_bytecode_for_selfdestructed_smart_contracts"

  @impl FillingMigration
  def migration_name, do: @migration_name

  @impl FillingMigration
  def last_unprocessed_identifiers(%{"min_block_number" => min_block_number} = state)
      when not is_nil(min_block_number) and min_block_number < 0 do
    {[], state}
  end

  def last_unprocessed_identifiers(%{"min_block_number" => min_block_number} = state)
      when not is_nil(min_block_number) do
    limit = batch_size() * concurrency()
    from_block_number = min_block_number
    to_block_number = max(from_block_number - limit + 1, 0)

    block_numbers = Enum.to_list(from_block_number..to_block_number//-1)

    {block_numbers, %{state | "min_block_number" => to_block_number - 1}}
  end

  def last_unprocessed_identifiers(state) do
    query =
      from(
        migration_status in MigrationStatus,
        where: migration_status.migration_name == ^@migration_name,
        select: migration_status.meta
      )

    meta = Repo.one(query, timeout: :infinity)

    state
    |> Map.put("min_block_number", (meta && meta["min_block_number"]) || BlockNumber.get_max())
    |> last_unprocessed_identifiers()
  end

  @impl FillingMigration
  def unprocessed_data_query, do: nil

  @impl FillingMigration
  def update_batch([]), do: {:ok, []}

  def update_batch(block_numbers) do
    activation = cancun_activation()

    result =
      Repo.transaction(
        fn ->
          destructions =
            Repo.all(
              from(it in InternalTransaction,
                join: transaction in Transaction,
                on: transaction.hash == it.transaction_hash,
                join: block in Block,
                on: block.hash == transaction.block_hash and block.number == it.block_number,
                where: block.consensus == true and it.block_number in ^block_numbers,
                where: it.type == :selfdestruct and is_nil(it.error),
                select: %{
                  transaction_hash: it.transaction_hash,
                  from_address_hash: it.from_address_hash,
                  block_number: block.number,
                  timestamp: block.timestamp
                }
              ),
              timeout: :infinity
            )

          case destructions do
            [] ->
              []

            _ ->
              transaction_hashes = Enum.map(destructions, & &1.transaction_hash)

              created_in_same_tx =
                Repo.all(
                  from(it in InternalTransaction,
                    join: transaction in Transaction,
                    on: transaction.hash == it.transaction_hash,
                    join: block in Block,
                    on: block.hash == transaction.block_hash and block.number == it.block_number,
                    where: block.consensus == true and it.transaction_hash in ^transaction_hashes,
                    where: it.type in [:create, :create2] and is_nil(it.error),
                    select: {it.transaction_hash, it.created_contract_address_hash}
                  ),
                  timeout: :infinity
                )
                |> MapSet.new()

              candidates =
                destructions
                |> Enum.filter(fn destruction ->
                  before_cancun?(activation, destruction) or
                    MapSet.member?(created_in_same_tx, {destruction.transaction_hash, destruction.from_address_hash})
                end)
                |> Enum.map(& &1.from_address_hash)
                |> Enum.uniq()

              addresses =
                Repo.all(
                  from(address in Address,
                    where: address.hash in ^candidates,
                    where: not is_nil(address.contract_code),
                    where: fragment("octet_length(?) > 0", address.contract_code),
                    select: address.hash
                  ),
                  timeout: :infinity
                )

              case InternalTransactions.reconcile_selfdestructed_contracts_bytecode(
                     Repo,
                     addresses,
                     Import.timestamps()
                   ) do
                {:ok, count} -> count
                {:error, reason} -> Repo.rollback(reason)
              end
          end
        end,
        timeout: :infinity
      )

    case result do
      {:ok, _} -> result
      {:error, reason} -> raise "Cannot reconcile SELFDESTRUCT bytecode: #{inspect(reason)}"
    end
  end

  defp cancun_activation do
    case Keyword.fetch(Application.get_env(:explorer, __MODULE__, []), :cancun_block) do
      {:ok, :never} -> :never
      {:ok, height} when is_integer(height) and height >= 0 -> {:block, height}
      {:ok, value} -> raise "Invalid SELFDESTRUCT Cancun activation block: #{inspect(value)}"
      :error -> chain_spec_activation()
    end
  end

  defp chain_spec_activation do
    case Application.get_env(:explorer, GenesisData, [])[:chain_spec_path] do
      nil ->
        raise "Missing SELFDESTRUCT fork schedule: configure cancun_block or a chain spec"

      path ->
        spec = path |> read_chain_spec!() |> Jason.decode!()

        case spec do
          %{"config" => config} ->
            activation_timestamp(Map.get(config, "cancunTime"))

          %{"app_state" => %{"evm" => %{"params" => %{"chain_config" => config}}}} ->
            activation_timestamp(Map.fetch!(config, "cancun_time"))

          _ ->
            raise "Chain spec does not contain an EVM fork configuration"
        end
    end
  end

  defp read_chain_spec!(path) do
    if Utils.ConfigHelper.valid_url?(path) do
      case Explorer.HttpClient.get(path, [], timeout: 60_000, recv_timeout: 60_000) do
        {:ok, %{status_code: 200, body: body}} -> body
        error -> raise "Cannot read SELFDESTRUCT fork configuration: #{inspect(error)}"
      end
    else
      File.read!(path)
    end
  end

  defp activation_timestamp(nil), do: :never
  defp activation_timestamp(-1), do: :never
  defp activation_timestamp(value) when is_integer(value) and value >= 0, do: {:timestamp, value}
  defp activation_timestamp("-1"), do: :never
  defp activation_timestamp("0x" <> value), do: value |> String.to_integer(16) |> activation_timestamp()
  defp activation_timestamp(value) when is_binary(value), do: value |> String.to_integer() |> activation_timestamp()

  defp before_cancun?(:never, _destruction), do: true
  defp before_cancun?({:block, activation}, destruction), do: destruction.block_number < activation
  defp before_cancun?({:timestamp, activation}, destruction), do: DateTime.to_unix(destruction.timestamp) < activation

  @impl FillingMigration
  def update_cache, do: :ok
end
