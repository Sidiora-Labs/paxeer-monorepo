Code.require_file("../indexer/fetcher/optimism/transaction_batch_rpc_fixture.exs", __DIR__)

defmodule Indexer.Repair2233564Test do
  use ExUnit.Case, async: false
  use Explorer.DataCase

  alias Indexer.Fetcher.Optimism.TransactionBatch
  alias Indexer.Fetcher.Optimism.TransactionBatchRPCFixture, as: RPCFixture

  test "033: recorded hexadecimal heights and null pass through the HTTP adapter into the hash map" do
    [hash_a, hash_b] = RPCFixture.hashes()
    unknown_hash = RPCFixture.unknown_hash()
    arguments = RPCFixture.start_rpc([hash_a, unknown_hash, hash_b])

    numbers = TransactionBatch.get_block_numbers_by_hashes([hash_a, unknown_hash, hash_b], arguments)

    assert numbers == %{hash_a => 24_203_005, hash_b => 24_203_006}
    refute Map.has_key?(numbers, unknown_hash)
    assert_receive {:block_rpc_exchange, requests, responses}
    assert Enum.map(requests, & &1["id"]) == [0, 1, 2]
    assert Enum.map(responses, & &1["id"]) == [2, 1, 0]
    assert Enum.map(responses, &get_in(&1, ["result", "number"])) == ["0x1714efe", nil, "0x1714efd"]
    assert Enum.at(responses, 1) == %{"jsonrpc" => "2.0", "id" => 1, "result" => nil}
  end

  test "033: an unknown hash alone produces an empty map after an HTTP null response" do
    unknown_hash = RPCFixture.unknown_hash()
    arguments = RPCFixture.start_rpc([unknown_hash])

    assert TransactionBatch.get_block_numbers_by_hashes([unknown_hash], arguments) == %{}
    assert_receive {:block_rpc_exchange, [_], [%{"jsonrpc" => "2.0", "id" => 0, "result" => nil}]}
  end

  test "033: a persisted block is retained while HTTP resolves only the missing hashes" do
    [hash_a, hash_b] = RPCFixture.hashes()
    insert(:block, hash: hash_a, number: 24_203_005)
    unknown_hash = RPCFixture.unknown_hash()
    arguments = RPCFixture.start_rpc([hash_b, unknown_hash])

    assert TransactionBatch.get_block_numbers_by_hashes([hash_a, hash_b, unknown_hash], arguments) ==
             %{hash_a => 24_203_005, hash_b => 24_203_006}

    assert_receive {:block_rpc_exchange, requests, responses}
    assert length(requests) == 2
    assert Enum.map(responses, &get_in(&1, ["result", "number"])) == [nil, "0x1714efe"]
    assert Repo.get!(Explorer.Chain.Block, hash_a).number == 24_203_005
    refute Repo.get(Explorer.Chain.Block, hash_b)
  end

  test "033: an empty lookup needs no transport" do
    assert TransactionBatch.get_block_numbers_by_hashes([], %{}) == %{}
  end
end
