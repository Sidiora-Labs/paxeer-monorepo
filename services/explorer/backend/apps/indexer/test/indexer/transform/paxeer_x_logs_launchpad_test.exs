defmodule Indexer.Transform.PaxeerXLogsLaunchpadTest do
  use ExUnit.Case, async: true

  import ExUnit.CaptureLog

  alias ABI.TypeEncoder
  alias Indexer.Transform.PaxeerXLogs

  @launchpad_precompile "0x0000000000000000000000000000000000001017"

  test "launchpad_v612_logs_parse" do
    launchpad_v612_logs_parse()
  end

  def launchpad_v612_logs_parse do
    amount = Integer.pow(2, 255) + 7

    fixtures = [
      %{
        event_name: "ConfigUpdated",
        kind: "config_updated",
        signature: "ConfigUpdated(bytes32,uint256,uint256,uint256,uint256)",
        topic: "0x2903b3642d2105b7fbd127326de888ab50e8d975bf819398337fb58407c1b10e",
        indexed: [word(81)],
        data: encode([12, 24, 1_700_000_001, 4_000_001], [{:uint, 256}, {:uint, 256}, {:uint, 256}, {:uint, 256}]),
        address_hash: nil,
        asset_address_hash: nil,
        amount: nil,
        parameters: %{
          "key" => word(81),
          "oldValue" => 12,
          "newValue" => 24,
          "timestamp" => 1_700_000_001,
          "blockNumber" => 4_000_001
        }
      },
      %{
        event_name: "FeeDistributed",
        kind: "fee_distributed",
        signature: "FeeDistributed(bytes32,uint256,uint8,uint256,address,uint256,uint256)",
        topic: "0xf7ab826cedf9d005102eddf95e65f43597aef36a55ff09ee4f452204c83f4afc",
        indexed: [word(82)],
        data:
          encode([901, 2, amount, <<11::160>>, 1_700_000_002, 4_000_002], [
            {:uint, 256},
            {:uint, 8},
            {:uint, 256},
            :address,
            {:uint, 256},
            {:uint, 256}
          ]),
        address_hash: address(11),
        asset_address_hash: nil,
        amount: amount,
        parameters: %{
          "poolId" => word(82),
          "nftId" => 901,
          "strategy" => 2,
          "amount" => amount,
          "recipient" => address(11),
          "timestamp" => 1_700_000_002,
          "blockNumber" => 4_000_002
        }
      },
      %{
        event_name: "FeeFlow",
        kind: "fee_flow",
        signature: "FeeFlow(uint8,address,address,uint256,uint256,uint256,uint256,uint256,uint256)",
        topic: "0x626e3fd5d9323ca4b7ffa3ca51c21c02766b638d15eb8896a36a4e0e97ae6de4",
        indexed: [word(3), word(12), word(13)],
        data:
          encode([amount, 21, 22, 23, 1_700_000_003, 4_000_003], [
            {:uint, 256},
            {:uint, 256},
            {:uint, 256},
            {:uint, 256},
            {:uint, 256},
            {:uint, 256}
          ]),
        address_hash: address(13),
        asset_address_hash: nil,
        amount: amount,
        parameters: %{
          "kind" => 3,
          "pool" => address(12),
          "party" => address(13),
          "amount" => amount,
          "protocolCut" => 21,
          "poolCut" => 22,
          "epoch" => 23,
          "timestamp" => 1_700_000_003,
          "blockNumber" => 4_000_003
        }
      },
      %{
        event_name: "FeeStrategyChanged",
        kind: "fee_strategy_changed",
        signature: "FeeStrategyChanged(bytes32,uint256,uint8,uint8,uint256,uint256)",
        topic: "0x337fd9f9304e9da4c04886a5b8bf56e74032b459758212a860bc61d3544353dd",
        indexed: [word(84)],
        data:
          encode([902, 1, 2, 1_700_000_004, 4_000_004], [
            {:uint, 256},
            {:uint, 8},
            {:uint, 8},
            {:uint, 256},
            {:uint, 256}
          ]),
        address_hash: nil,
        asset_address_hash: nil,
        amount: nil,
        parameters: %{
          "poolId" => word(84),
          "nftId" => 902,
          "oldStrategy" => 1,
          "newStrategy" => 2,
          "timestamp" => 1_700_000_004,
          "blockNumber" => 4_000_004
        }
      },
      %{
        event_name: "MarketCreated",
        kind: "market_created",
        signature: "MarketCreated(bytes32,address,address,address,address,uint256,uint256)",
        topic: "0xb0a9c7605389f1127c2299d4a3ed85f0b2fc19894ab5411d76b453bb29f9f47b",
        indexed: [word(85), word(14), word(15)],
        data:
          encode([<<16::160>>, <<17::160>>, 1_700_000_005, 4_000_005], [:address, :address, {:uint, 256}, {:uint, 256}]),
        address_hash: address(15),
        asset_address_hash: address(14),
        amount: nil,
        parameters: %{
          "poolId" => word(85),
          "token" => address(14),
          "creator" => address(15),
          "pool" => address(16),
          "optical" => address(17),
          "timestamp" => 1_700_000_005,
          "blockNumber" => 4_000_005
        }
      },
      %{
        event_name: "NftMint",
        kind: "nft_mint",
        signature: "NftMint(uint256,address,address,uint8,uint256,uint256)",
        topic: "0xd2917d37031c0dfe5016665c7a83360cbf2ff32063096b2caf61ba66c06832e5",
        indexed: [word(903), word(18), word(19)],
        data: encode([3, 1_700_000_006, 4_000_006], [{:uint, 8}, {:uint, 256}, {:uint, 256}]),
        address_hash: address(18),
        asset_address_hash: nil,
        amount: nil,
        parameters: %{
          "tokenId" => 903,
          "creator" => address(18),
          "pool" => address(19),
          "strategy" => 3,
          "timestamp" => 1_700_000_006,
          "blockNumber" => 4_000_006
        }
      },
      %{
        event_name: "NftTransfer",
        kind: "nft_transfer",
        signature: "NftTransfer(address,address,address,uint256,uint256,uint256)",
        topic: "0x021f03e8a1315704c74c03e8c022c12bc80c4a2bd4fec8bd36b217322c340ad3",
        indexed: [word(20), word(21), word(22)],
        data: encode([904, 1_700_000_007, 4_000_007], [{:uint, 256}, {:uint, 256}, {:uint, 256}]),
        address_hash: address(21),
        asset_address_hash: nil,
        amount: nil,
        parameters: %{
          "nft" => address(20),
          "from" => address(21),
          "to" => address(22),
          "tokenId" => 904,
          "timestamp" => 1_700_000_007,
          "blockNumber" => 4_000_007
        }
      },
      %{
        event_name: "PoolRegistered",
        kind: "pool_registered",
        signature: "PoolRegistered(address,address,address,address,uint256,uint256,uint256)",
        topic: "0xda5568642b67660133b1de538c867c58f3037a5722b0e231a959ee8ca11b2ed7",
        indexed: [word(23), word(24), word(25)],
        data:
          encode([<<26::160>>, 905, 1_700_000_008, 4_000_008], [:address, {:uint, 256}, {:uint, 256}, {:uint, 256}]),
        address_hash: address(25),
        asset_address_hash: address(24),
        amount: nil,
        parameters: %{
          "pool" => address(23),
          "token" => address(24),
          "creator" => address(25),
          "optical" => address(26),
          "nftId" => 905,
          "timestamp" => 1_700_000_008,
          "blockNumber" => 4_000_008
        }
      },
      %{
        event_name: "PoolStateUpdated",
        kind: "pool_state_updated",
        signature: "PoolStateUpdated(bytes32,uint256,uint256,uint256,uint256,uint256,uint256)",
        topic: "0x8155a4b38a877d212b271811786986f8f7d78074c82fce4526c58defcc6d7ebd",
        indexed: [word(89)],
        data:
          encode([amount, 32, 33, 34, 1_700_000_009, 4_000_009], [
            {:uint, 256},
            {:uint, 256},
            {:uint, 256},
            {:uint, 256},
            {:uint, 256},
            {:uint, 256}
          ]),
        address_hash: nil,
        asset_address_hash: nil,
        amount: nil,
        parameters: %{
          "poolId" => word(89),
          "virtualReserve" => amount,
          "realReserve" => 32,
          "tokenReserve" => 33,
          "price" => 34,
          "timestamp" => 1_700_000_009,
          "blockNumber" => 4_000_009
        }
      },
      %{
        event_name: "RouterTrade",
        kind: "router_trade",
        signature: "RouterTrade(uint8,address,address,address,address,uint256,uint256,uint256,uint256,uint256)",
        topic: "0x17c1899a3fd431f2b2200b90e738d270bb856a4222040c5650bfa208acb3b650",
        indexed: [word(4), word(27), word(28)],
        data:
          encode([<<29::160>>, <<30::160>>, amount, 41, 42, 1_700_000_010, 4_000_010], [
            :address,
            :address,
            {:uint, 256},
            {:uint, 256},
            {:uint, 256},
            {:uint, 256},
            {:uint, 256}
          ]),
        address_hash: address(28),
        asset_address_hash: nil,
        amount: nil,
        parameters: %{
          "kind" => 4,
          "pool" => address(27),
          "sender" => address(28),
          "tokenIn" => address(29),
          "tokenOut" => address(30),
          "amountIn" => amount,
          "amountOut" => 41,
          "intermediateUsdl" => 42,
          "timestamp" => 1_700_000_010,
          "blockNumber" => 4_000_010
        }
      },
      %{
        event_name: "Swap",
        kind: "swap",
        signature: "Swap(bytes32,address,bool,uint256,uint256,uint256,uint256,uint256,uint256)",
        topic: "0xa234ab85728295548823d8a26077ecc77aa5815f9ed8f4c5e6fd390ac851d4cc",
        indexed: [word(91), word(31)],
        data:
          encode([true, amount, 51, 52, 53, 1_700_000_011, 4_000_011], [
            :bool,
            {:uint, 256},
            {:uint, 256},
            {:uint, 256},
            {:uint, 256},
            {:uint, 256},
            {:uint, 256}
          ]),
        address_hash: address(31),
        asset_address_hash: nil,
        amount: nil,
        parameters: %{
          "poolId" => word(91),
          "sender" => address(31),
          "isBuy" => true,
          "amountIn" => amount,
          "amountOut" => 51,
          "fee" => 52,
          "price" => 53,
          "timestamp" => 1_700_000_011,
          "blockNumber" => 4_000_011
        }
      },
      %{
        event_name: "TokenDeployed",
        kind: "token_deployed",
        signature: "TokenDeployed(address,address,address,bytes32,string,string,uint8,uint256,uint256,uint256)",
        topic: "0xad187874b793634c55d977f35d4828526eb48170a9531c71d88f8fd10cf9c237",
        indexed: [word(32), word(33), word(34)],
        data:
          encode(
            [
              <<92::256>>,
              "Paxeer Launch Token with a name longer than one ABI word",
              "PXT",
              18,
              amount,
              1_700_000_012,
              4_000_012
            ],
            [{:bytes, 32}, :string, :string, {:uint, 8}, {:uint, 256}, {:uint, 256}, {:uint, 256}]
          ),
        address_hash: address(34),
        asset_address_hash: address(32),
        amount: nil,
        parameters: %{
          "token" => address(32),
          "pool" => address(33),
          "creator" => address(34),
          "salt" => word(92),
          "name" => "Paxeer Launch Token with a name longer than one ABI word",
          "symbol" => "PXT",
          "decimals" => 18,
          "totalSupply" => amount,
          "timestamp" => 1_700_000_012,
          "blockNumber" => 4_000_012
        }
      }
    ]

    definitions = Enum.filter(PaxeerXLogs.definitions(), &(&1.address == @launchpad_precompile))
    assert length(definitions) == 22
    keys = Enum.map(definitions, &{&1.address, &1.topic})
    assert Enum.uniq(keys) == keys

    logs =
      fixtures
      |> Enum.with_index()
      |> Enum.map(fn {fixture, index} ->
        [second, third, fourth] = Enum.take(fixture.indexed ++ [nil, nil, nil], 3)

        log(index)
        |> Map.merge(%{
          address_hash: @launchpad_precompile,
          first_topic: fixture.topic,
          second_topic: second,
          third_topic: third,
          fourth_topic: fourth,
          data: fixture.data
        })
      end)

    swap = Enum.find(logs, &(&1.first_topic == "0xa234ab85728295548823d8a26077ecc77aa5815f9ed8f4c5e6fd390ac851d4cc"))
    assert swap.fourth_topic == nil

    expected_rows =
      Enum.zip_with(fixtures, logs, fn fixture, event_log ->
        assert [definition] = Enum.filter(definitions, &(&1.topic == fixture.topic))
        assert definition.table == :lx_market_events
        assert definition.domain == :launchpad
        assert definition.name == fixture.event_name
        assert definition.signature == fixture.signature
        assert definition.topic == "0x" <> Base.encode16(ExKeccak.hash_256(fixture.signature), case: :lower)
        assert definition.kind == fixture.kind
        assert definition.kind == Macro.underscore(fixture.event_name)

        %{
          transaction_hash: event_log.transaction_hash,
          block_hash: event_log.block_hash,
          block_number: event_log.block_number,
          log_index: event_log.index,
          event_name: fixture.event_name,
          domain: :launchpad,
          kind: fixture.kind,
          address_hash: fixture.address_hash,
          account: nil,
          asset_id: nil,
          asset_address_hash: fixture.asset_address_hash,
          amount: fixture.amount,
          parameters: fixture.parameters
        }
      end)

    assert capture_log(fn ->
             result = PaxeerXLogs.parse(logs)
             rows = Map.fetch!(result, :lx_market_events)

             assert Map.delete(result, :lx_market_events) == %{
                      lx_account_bindings: [],
                      undecoded_log_count: 0
                    }

             assert length(rows) == length(expected_rows)

             Enum.zip(rows, expected_rows)
             |> Enum.with_index()
             |> Enum.each(fn {{got, want}, i} -> assert {i, got} == {i, want} end)
           end) == ""
  end

  test "launchpad_v610_logs_still_parse" do
    launchpad_v610_logs_still_parse()
  end

  def launchpad_v610_logs_still_parse do
    amount = 18_446_744_073_709_551_617

    fees_claimed_log =
      log(12)
      |> Map.merge(%{
        address_hash: @launchpad_precompile,
        first_topic: "0xfe3464cd748424446c37877c28ce5b700222c5bc9f90d908afcc4e5cb22707ff",
        second_topic: word(7),
        third_topic: word(8),
        fourth_topic: nil,
        data: encode([amount], [{:uint, 256}])
      })

    swap_log =
      log(13)
      |> Map.merge(%{
        address_hash: @launchpad_precompile,
        first_topic: "0xf3369c7e0aa652773c7246b5481ca4b1ee0b408d90467d2ce93b165b9938fde5",
        second_topic: word(9),
        third_topic: word(10),
        fourth_topic: word(11),
        data: encode([false, amount, 202, 203, 204], [:bool, {:uint, 256}, {:uint, 256}, {:uint, 256}, {:uint, 256}])
      })

    definitions = Enum.filter(PaxeerXLogs.definitions(), &(&1.address == @launchpad_precompile))

    legacy_signatures = [
      "MarketCreated(address,address,string,string,string,uint8)",
      "Swap(address,address,address,bool,uint256,uint256,uint256,uint256)",
      "FeeRecorded(address,uint256,uint256,uint256)",
      "FeesClaimed(address,address,uint256)",
      "FeesBurned(address,uint256)",
      "FeeStrategyChanged(address,uint8,uint8)",
      "AirdropExecuted(address,uint256,uint256)",
      "AirdropClaimed(address,address,uint256,uint256)",
      "LpRewardsExecuted(address,uint256)",
      "PauseToggled(address,bool)"
    ]

    for signature <- legacy_signatures do
      assert [definition] = Enum.filter(definitions, &(&1.signature == signature))
      assert definition.table == :lx_market_events
      assert definition.domain == :launchpad
      assert definition.kind == Macro.underscore(definition.name)
    end

    assert capture_log(fn ->
             assert PaxeerXLogs.parse([fees_claimed_log, swap_log]) == %{
                      lx_account_bindings: [],
                      undecoded_log_count: 0,
                      lx_market_events: [
                        %{
                          transaction_hash: fees_claimed_log.transaction_hash,
                          block_hash: fees_claimed_log.block_hash,
                          block_number: fees_claimed_log.block_number,
                          log_index: fees_claimed_log.index,
                          event_name: "FeesClaimed",
                          domain: :launchpad,
                          kind: "fees_claimed",
                          address_hash: address(8),
                          account: nil,
                          asset_id: nil,
                          asset_address_hash: address(7),
                          amount: amount,
                          parameters: %{"token" => address(7), "recipient" => address(8), "amount" => amount}
                        },
                        %{
                          transaction_hash: swap_log.transaction_hash,
                          block_hash: swap_log.block_hash,
                          block_number: swap_log.block_number,
                          log_index: swap_log.index,
                          event_name: "Swap",
                          domain: :launchpad,
                          kind: "swap",
                          address_hash: address(10),
                          account: nil,
                          asset_id: nil,
                          asset_address_hash: address(9),
                          amount: nil,
                          parameters: %{
                            "token" => address(9),
                            "trader" => address(10),
                            "recipient" => address(11),
                            "isBuy" => false,
                            "amountIn" => amount,
                            "amountOut" => 202,
                            "feeAmount" => 203,
                            "price" => 204
                          }
                        }
                      ]
                    }
           end) == ""
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
