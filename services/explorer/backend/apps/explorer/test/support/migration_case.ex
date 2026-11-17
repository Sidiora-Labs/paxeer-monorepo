defmodule Explorer.MigrationCase do
  @moduledoc false

  import Ecto.Query

  alias Explorer.Chain.Address
  alias Explorer.Chain.Cache.Counters.LastFetchedCounter
  alias Explorer.Chain.InternalTransaction.DeleteQueue
  alias Explorer.Repo

  def start_owner! do
    owner = Explorer.TestHelper.start_migration_owner!()
    addresses = Repo.all(from(address in Address, select: address.hash))
    queue = Repo.all(from(entry in DeleteQueue, select: entry.block_number))
    counters = Repo.all(LastFetchedCounter)
    counter_types = Enum.map(counters, & &1.counter_type)
    counter_rows = Enum.map(counters, &(&1 |> Map.from_struct() |> Map.delete(:__meta__)))

    ExUnit.Callbacks.on_exit(fn ->
      Repo.delete_all(from(entry in DeleteQueue, where: entry.block_number not in ^queue))
      Repo.delete_all(from(address in Address, where: address.hash not in ^addresses))
      Repo.delete_all(from(counter in LastFetchedCounter, where: counter.counter_type not in ^counter_types))
      Repo.insert_all(LastFetchedCounter, counter_rows, on_conflict: :replace_all, conflict_target: :counter_type)
    end)

    owner
  end
end
