defmodule Explorer.Chain.Cache.Counters.AddressesCountTest do
  use Explorer.DataCase

  alias Explorer.Chain.Address
  alias Explorer.Chain.Cache.Counters.AddressesCount
  alias Explorer.Chain.Cache.Counters.LastFetchedCounter

  setup do
    Repo.delete_all(Address)
    LastFetchedCounter.delete("addresses_count")
    :ok
  end

  test "populates the cache with the number of all addresses" do
    insert(:address, fetched_coin_balance: 0)
    insert(:address, fetched_coin_balance: 1)
    insert(:address, fetched_coin_balance: 2)

    start_supervised!(AddressesCount)
    AddressesCount.consolidate()

    assert AddressesCount.fetch() == 3
  end
end
