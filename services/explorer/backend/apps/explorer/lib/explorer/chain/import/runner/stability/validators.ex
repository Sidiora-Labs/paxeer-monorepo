defmodule Explorer.Chain.Import.Runner.Stability.Validators do
  @moduledoc """
  Bulk updates `t:Explorer.Chain.Stability.Validator.t/0` blocks_validated counters.
  """

  require Ecto.Query

  alias Ecto.{Multi, Repo}
  alias Explorer.Chain.{Block, Import, Stability.Validator}
  alias Explorer.Prometheus.Instrumenter

  import Ecto.Query, only: [from: 2]

  @behaviour Import.Runner

  # milliseconds
  @timeout 60_000

  @type imported :: [Validator.t()]

  @impl Import.Runner
  def ecto_schema_module, do: Validator

  @impl Import.Runner
  def option_key, do: :stability_validators

  @impl Import.Runner
  def imported_table_row do
    %{
      value_type: "[#{ecto_schema_module()}.t()]",
      value_description: "List of `t:#{ecto_schema_module()}.t/0`s with updated counters"
    }
  end

  @impl Import.Runner
  def run(multi, changes_list, %{timestamps: timestamps} = options) do
    insert_options =
      options
      |> Map.get(option_key(), %{})
      |> Map.take(~w(timeout)a)
      |> Map.put_new(:timeout, @timeout)
      |> Map.put(:timestamps, timestamps)

    Multi.run(multi, :stability_validators, fn repo, _ ->
      Instrumenter.block_import_stage_runner(
        fn -> update_counters(repo, changes_list, insert_options) end,
        :block_referencing,
        :stability_validators,
        :stability_validators
      )
    end)
  end

  @impl Import.Runner
  def timeout, do: @timeout

  @spec update_counters(Repo.t(), [map()], %{
          required(:timeout) => timeout,
          required(:timestamps) => Import.timestamps()
        }) ::
          {:ok, [Validator.t()]}
  defp update_counters(repo, changes_list, %{timeout: timeout, timestamps: timestamps}) when is_list(changes_list) do
    # Lock in address order before reading blocks so concurrent recovery runs
    # cannot overwrite a newer count with a snapshot taken before the lock.
    validators =
      from(v in Validator, order_by: [asc: v.address_hash], lock: "FOR UPDATE")
      |> repo.all(timeout: timeout)

    address_hashes = Enum.map(validators, & &1.address_hash)

    counts =
      from(b in Block,
        where: b.consensus == true and b.miner_hash in ^address_hashes,
        group_by: b.miner_hash,
        select: {b.miner_hash, count(b.hash)}
      )
      |> repo.all(timeout: timeout)
      |> Map.new()

    # Blocks retain contributions even when validator metadata has not arrived.
    # Reconcile every validator, including on an empty range, to retry those
    # contributions and remove counts for blocks that have lost consensus.
    Enum.reduce_while(validators, {:ok, []}, fn validator, {:ok, updated} ->
      query = from(v in Validator, where: v.address_hash == ^validator.address_hash, select: v)

      case repo.update_all(
             query,
             [set: [blocks_validated: Map.get(counts, validator.address_hash, 0), updated_at: timestamps.updated_at]],
             timeout: timeout
           ) do
        {1, [persisted]} -> {:cont, {:ok, [persisted | updated]}}
        result -> {:halt, {:error, {:validator_counter_update_failed, validator.address_hash, result}}}
      end
    end)
  end
end
