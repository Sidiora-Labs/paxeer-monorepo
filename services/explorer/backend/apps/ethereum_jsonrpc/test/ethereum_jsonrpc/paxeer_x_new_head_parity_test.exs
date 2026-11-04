Code.require_file("paxeer_x_capture.exs", __DIR__)

defmodule EthereumJSONRPC.PaxeerXNewHeadParityTest do
  @moduledoc """
  `new_head_full.json` pairs the captured block at height 24_203_006 from
  `consecutive_blocks.json` with its newHeads payload reconstructed from the
  parity encoder's header fields. `new_head.json` retains the pre-parity payload.

  The transport regression separately replays a committed block emitted by the
  node formatter repaired in 22.31.460; its source revision and both wire payloads
  are stored in captures/new_heads.json. Historical public-node payloads remain intact.
  """

  use ExUnit.Case, async: false

  alias EthereumJSONRPC.{Blocks, PaxeerX}

  @fixture Path.expand("../support/fixture/paxeer_x/new_head_full.json", __DIR__)

  test "repaired node newHeads and fetched body agree through the production transports" do
    EthereumJSONRPC.PaxeerXCapture.assert_recorded_head_parity()
  end

  test "full_new_head_matches_block_body" do
    %{"new_head" => new_head, "block" => block} =
      @fixture
      |> File.read!()
      |> Jason.decode!()

    for field <- ~w(number hash logsBloom gasUsed transactionsRoot receiptsRoot parentHash) do
      assert Map.fetch!(new_head, field) == Map.fetch!(block, field), field
    end

    assert new_head == Map.drop(block, ~w(size totalDifficulty transactions uncles))

    number = EthereumJSONRPC.quantity_to_integer(block["number"])

    %Blocks{blocks_params: [block_params], errors: []} =
      Blocks.from_responses([%{id: 0, result: block}], %{0 => %{number: number}})

    assert PaxeerX.new_head_matches_block?(new_head, block_params) == true
  end
end
