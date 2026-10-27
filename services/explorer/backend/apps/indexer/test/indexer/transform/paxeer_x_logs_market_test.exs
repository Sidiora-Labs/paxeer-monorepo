defmodule Indexer.Transform.PaxeerXLogsMarketTest do
  use ExUnit.Case, async: true

  alias ABI.TypeEncoder
  alias Indexer.Transform.PaxeerXLogs

  test "market_logs_still_parse" do
    market_logs_still_parse()
  end

  def market_logs_still_parse do
    amount = 18_446_744_073_709_551_617

    exchange_log =
      log(0)
      |> Map.merge(%{
        address_hash: "0x0000000000000000000000000000000000001015",
        first_topic: "0x456ba29aa60d5cac1a6dc1c0f3df30b1f18963fd90dafe8c5f4a6de80440118a",
        second_topic: word(1),
        third_topic: word(2),
        fourth_topic: word(3),
        data: encode([<<4::256>>, amount, <<5::256>>, 6], [{:bytes, 32}, {:uint, 256}, {:bytes, 32}, {:uint, 64}])
      })

    assert %{lx_market_events: [exchange_row], lx_account_bindings: []} = PaxeerXLogs.parse([exchange_log])

    assert exchange_row == %{
             transaction_hash: exchange_log.transaction_hash,
             block_hash: exchange_log.block_hash,
             block_number: exchange_log.block_number,
             log_index: exchange_log.index,
             event_name: "MarginDeposited",
             domain: :exchange,
             kind: "margin_deposited",
             address_hash: address(3),
             account: word(2),
             asset_id: word(4),
             asset_address_hash: nil,
             amount: amount,
             parameters: %{
               "intentId" => word(1),
               "account" => word(2),
               "owner" => address(3),
               "assetId" => word(4),
               "amount" => amount,
               "depositId" => word(5),
               "nonce" => 6
             }
           }

    launchpad_log =
      log(1)
      |> Map.merge(%{
        address_hash: "0x0000000000000000000000000000000000001017",
        first_topic: "0xfe3464cd748424446c37877c28ce5b700222c5bc9f90d908afcc4e5cb22707ff",
        second_topic: word(7),
        third_topic: word(8),
        fourth_topic: nil,
        data: encode([900], [{:uint, 256}])
      })

    assert %{lx_market_events: [launchpad_row], lx_account_bindings: []} = PaxeerXLogs.parse([launchpad_log])

    assert launchpad_row == %{
             transaction_hash: launchpad_log.transaction_hash,
             block_hash: launchpad_log.block_hash,
             block_number: launchpad_log.block_number,
             log_index: launchpad_log.index,
             event_name: "FeesClaimed",
             domain: :launchpad,
             kind: "fees_claimed",
             address_hash: address(8),
             account: nil,
             asset_id: nil,
             asset_address_hash: address(7),
             amount: 900,
             parameters: %{"token" => address(7), "recipient" => address(8), "amount" => 900}
           }
  end

  test "binding_log_still_parses" do
    binding_log_still_parses()
  end

  def binding_log_still_parses do
    binding_log =
      log(2)
      |> Map.merge(%{
        address_hash: "0x0000000000000000000000000000000000001004",
        first_topic: "0x79b5f3e386759c93c56bf67a868c5f49afe6afaf286aa99bbc8e800ca4b7c856",
        second_topic: word(9),
        third_topic: word(10),
        fourth_topic: nil,
        data: encode([11], [{:uint, 64}])
      })

    assert %{lx_account_bindings: [binding_row], lx_market_events: []} = PaxeerXLogs.parse([binding_log])

    assert binding_row == %{
             transaction_hash: binding_log.transaction_hash,
             block_hash: binding_log.block_hash,
             block_number: binding_log.block_number,
             log_index: binding_log.index,
             event_name: "LayerXBound",
             bound: true,
             evm_address_hash: address(9),
             layerx_did: "did:layerx:" <> Base.encode16(<<10::256>>, case: :lower),
             nonce: 11,
             parameters: %{"evm" => address(9), "didPublicKey" => word(10), "nonce" => 11}
           }
  end

  defp log(index) do
    %{
      transaction_hash: word(100),
      block_hash: word(101),
      block_number: 4_000_000,
      index: index
    }
  end

  defp word(value), do: "0x" <> Base.encode16(<<value::256>>, case: :lower)

  defp address(value), do: "0x" <> Base.encode16(<<value::160>>, case: :lower)

  defp encode(values, types), do: "0x" <> Base.encode16(TypeEncoder.encode(values, types), case: :lower)
end
