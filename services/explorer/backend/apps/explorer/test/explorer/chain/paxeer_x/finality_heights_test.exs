defmodule Explorer.Chain.PaxeerX.FinalityHeightsTest do
  use Explorer.DataCase

  alias Explorer.Chain.PaxeerX.Finality
  alias Explorer.Chain.PaxeerX.Finality.Heights

  test "finality_anchor_struct_is_gone" do
    finality_anchor_struct_is_gone()
  end

  test "heights_reads_max_consensus_block" do
    heights_reads_max_consensus_block()
  end

  defp finality_anchor_struct_is_gone do
    refute Code.ensure_loaded?(Explorer.Chain.PaxeerX.Finality.Anchor)
  end

  defp heights_reads_max_consensus_block do
    insert(:block, number: 7, consensus: true)
    insert(:block, number: 9, consensus: true)
    insert(:block, number: 11, consensus: false)

    assert Finality.heights() == %Heights{committed_height: 9}
  end
end
