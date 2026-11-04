if Application.get_env(:explorer, :chain_type) == :optimism do
  Code.require_file("transaction_batch_rpc_fixture.exs", __DIR__)

  defmodule Indexer.Fetcher.Optimism.TransactionBatchTest do
    use ExUnit.Case, async: false
    use Explorer.DataCase

    alias Indexer.Fetcher.Optimism.TransactionBatch
    alias Indexer.Fetcher.Optimism.TransactionBatchRPCFixture, as: RPCFixture

    describe "get_block_numbers_by_hashes/2" do
      test "processes empty list" do
        assert TransactionBatch.get_block_numbers_by_hashes([], %{}) == %{}
      end

      test "processes list of hashes" do
        [hash_a, hash_b] = RPCFixture.hashes()
        unknown_hash = RPCFixture.unknown_hash()
        hashes = [hash_a, unknown_hash, hash_b]
        json_rpc_named_arguments = RPCFixture.start_rpc(hashes)

        assert %{hash_a => 24_203_005, hash_b => 24_203_006} ==
                 TransactionBatch.get_block_numbers_by_hashes(hashes, json_rpc_named_arguments)

        assert_receive {:block_rpc_exchange, _, responses}
        assert Enum.map(responses, &get_in(&1, ["result", "number"])) == ["0x1714efe", nil, "0x1714efd"]
      end
    end
  end
end
