defmodule EthereumJSONRPC.PaxeerXNewHeadFieldsTest do
  use ExUnit.Case, async: true

  alias EthereumJSONRPC.{Blocks, PaxeerX}

  @fixture Path.expand("../support/fixture/paxeer_x/new_head_full.json", __DIR__)

  test "new_head_with_other_receipts_root_is_rejected" do
    %{"new_head" => new_head, "block" => block} =
      @fixture
      |> File.read!()
      |> Jason.decode!()

    number = EthereumJSONRPC.quantity_to_integer(block["number"])

    %Blocks{blocks_params: [block_params], errors: []} =
      Blocks.from_responses([%{id: 0, result: block}], %{0 => %{number: number}})

    assert PaxeerX.new_head_matches_block?(new_head, block_params) == true
    assert PaxeerX.receipts_root_verifiable?() == false

    for field <- ~w(receiptsRoot parentHash) do
      "0x" <> hex = Map.fetch!(new_head, field)
      <<first, rest::binary>> = Base.decode16!(hex, case: :mixed)
      changed = "0x" <> Base.encode16(<<Bitwise.bxor(first, 1), rest::binary>>, case: :lower)
      changed_head = Map.replace!(new_head, field, changed)

      assert PaxeerX.new_head_matches_block?(changed_head, block_params) == false, field
    end
  end
end
