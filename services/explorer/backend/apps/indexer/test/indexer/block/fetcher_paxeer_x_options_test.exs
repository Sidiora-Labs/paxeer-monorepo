defmodule Indexer.Block.FetcherPaxeerXOptionsTest do
  use ExUnit.Case, async: false

  alias ABI.TypeEncoder
  alias Indexer.Block.Fetcher
  alias Indexer.Transform.PaxeerXLogs

  setup do
    chain_type = Application.fetch_env!(:explorer, :chain_type)
    Application.put_env(:explorer, :chain_type, :paxeer_x)
    on_exit(fn -> Application.put_env(:explorer, :chain_type, chain_type) end)
  end

  test "paxeer_x_import_options_carry_two_keys" do
    paxeer_x_import_options_carry_two_keys()
  end

  def paxeer_x_import_options_carry_two_keys do
    binding_log = %{
      address_hash: "0x0000000000000000000000000000000000001004",
      first_topic: "0x79b5f3e386759c93c56bf67a868c5f49afe6afaf286aa99bbc8e800ca4b7c856",
      second_topic: word(1),
      third_topic: word(2),
      fourth_topic: nil,
      data: encode([3], [{:uint, 64}]),
      transaction_hash: word(10),
      block_hash: word(11),
      block_number: 100,
      index: 0
    }

    market_log = %{
      binding_log
      | address_hash: "0x0000000000000000000000000000000000001015",
        first_topic: "0x39148489da3c16ee8c589a95e2f0c869816ee4f816b4ba1b85b32bc6d94c0241",
        fourth_topic: word(4),
        index: 1
    }

    parsed = PaxeerXLogs.parse([binding_log, market_log])
    assert [%{event_name: "LayerXBound"}] = parsed.lx_account_bindings
    assert [%{event_name: "OrderCancelRequested"}] = parsed.lx_market_events

    options = Fetcher.import_options(%{}, %{paxeer_x_logs: parsed})

    assert Enum.sort(Map.keys(options)) == [:paxeer_x_account_bindings, :paxeer_x_market_events]
    assert options.paxeer_x_account_bindings == %{params: parsed.lx_account_bindings}
    assert options.paxeer_x_market_events == %{params: parsed.lx_market_events}

    assert Fetcher.import_options(%{}, %{paxeer_x_logs: PaxeerXLogs.parse([])}) == %{
             paxeer_x_account_bindings: %{params: []},
             paxeer_x_market_events: %{params: []}
           }
  end

  defp word(value), do: "0x" <> Base.encode16(<<value::256>>, case: :lower)

  defp encode(values, types), do: "0x" <> Base.encode16(TypeEncoder.encode(values, types), case: :lower)
end
