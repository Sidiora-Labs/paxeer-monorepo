defmodule Indexer.Transform.PaxeerXLogsRetiredTest do
  use ExUnit.Case, async: true

  alias ABI.TypeEncoder
  alias Indexer.Transform.PaxeerXLogs

  test "retired_precompile_logs_yield_no_rows" do
    retired_precompile_logs_yield_no_rows()
  end

  def retired_precompile_logs_yield_no_rows do
    custody_log = %{
      address_hash: "0x0000000000000000000000000000000000001013",
      first_topic: "0x7edb71c9100c656847896d0b5b194f69f7da287eb57964a81e7f807a6a944028",
      second_topic: word(1),
      third_topic: word(2),
      fourth_topic: word(3),
      data: encode([<<4::256>>, 100, 1], [{:bytes, 32}, {:uint, 256}, {:uint, 64}]),
      transaction_hash: word(10),
      block_hash: word(11),
      block_number: 100,
      index: 0
    }

    anchor_log = %{
      address_hash: "0x0000000000000000000000000000000000001014",
      first_topic: "0xf732efc9df2e7589898899f85ca5e6cb25fa1c619461d96e81e82be9ccf14416",
      second_topic: word(1),
      third_topic: word(2),
      fourth_topic: nil,
      data: encode([<<3::256>>, <<4::256>>, 9], [{:bytes, 32}, {:bytes, 32}, {:uint, 8}]),
      transaction_hash: word(10),
      block_hash: word(11),
      block_number: 100,
      index: 1
    }

    expected = %{lx_account_bindings: [], lx_market_events: []}

    assert PaxeerXLogs.parse([custody_log, anchor_log]) == expected
    assert PaxeerXLogs.parse([custody_log]) == expected
    assert PaxeerXLogs.parse([anchor_log]) == expected
  end

  defp word(value), do: "0x" <> Base.encode16(<<value::256>>, case: :lower)

  defp encode(values, types), do: "0x" <> Base.encode16(TypeEncoder.encode(values, types), case: :lower)
end
