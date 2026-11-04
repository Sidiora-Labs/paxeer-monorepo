defmodule Indexer.Transform.PaxeerXLogsTest do
  use ExUnit.Case, async: true

  alias ABI.TypeEncoder
  alias Explorer.Chain.PaxeerX.{AccountBinding, MarketEvent}
  alias Indexer.Transform.PaxeerXLogs

  @addr_precompile "0x0000000000000000000000000000000000001004"
  @custody_precompile "0x0000000000000000000000000000000000001013"
  @anchor_precompile "0x0000000000000000000000000000000000001014"
  @exchange_precompile "0x0000000000000000000000000000000000001015"
  @bridge_precompile "0x0000000000000000000000000000000000001016"
  @launchpad_precompile "0x0000000000000000000000000000000000001017"

  @erc20_transfer_topic "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"

  @schemas %{lx_account_bindings: AccountBinding, lx_market_events: MarketEvent}
  @empty %{lx_account_bindings: [], lx_market_events: [], undecoded_log_count: 0}
  @row_tables [:lx_account_bindings, :lx_market_events]
  @retired_keys [:lx_custody_events, :lx_deposit_roots, :lx_anchors, :lx_guarantor_events, :lx_receipts]

  test "a v6.12.0 perps collateral deposit at 0x1015 decodes into an exchange market event" do
    account = <<0x77::160>>
    amount = 1_000_000
    new_balance = 2_000_000

    deposit = %{
      address_hash: @exchange_precompile,
      block_hash: bytes32(0x78),
      block_number: 4_000_000,
      transaction_hash: bytes32(0x79),
      index: 0,
      first_topic:
        "0x" <> Base.encode16(ExKeccak.hash_256("CollateralDeposited(address,uint256,uint256)"), case: :lower),
      second_topic: topic(:address, account),
      third_topic: nil,
      fourth_topic: nil,
      data: "0x" <> Base.encode16(TypeEncoder.encode([amount, new_balance], [{:uint, 256}, {:uint, 256}]), case: :lower)
    }

    assert %{lx_market_events: [row], undecoded_log_count: 0} = PaxeerXLogs.parse([deposit])
    assert row.domain == :exchange
    assert row.kind == "collateral_deposited"
    assert row.address_hash == address(0x77)
    assert row.amount == amount
    assert row.event_name == "CollateralDeposited"
    assert row.parameters == %{"account" => address(0x77), "amount" => amount, "newBalance" => new_balance}
    assert row.account == nil
    assert_schema_row(:lx_market_events, row)

    legacy =
      log(
        definition!(@exchange_precompile, "MarginDeposited(bytes32,bytes32,address,bytes32,uint256,bytes32,uint64)"),
        1,
        %{
          "intentId" => <<0x31::256>>,
          "account" => <<0x32::256>>,
          "owner" => account,
          "assetId" => <<0x34::256>>,
          "amount" => amount,
          "depositId" => <<0x35::256>>,
          "nonce" => 3
        }
      )

    assert %{lx_market_events: [^row, legacy_row], undecoded_log_count: 0} = PaxeerXLogs.parse([deposit, legacy])
    assert legacy_row.domain == :exchange
    assert legacy_row.kind == "margin_deposited"
    assert legacy_row.address_hash == address(0x77)
    assert legacy_row.account == bytes32(0x32)
    assert legacy_row.amount == amount
    assert_schema_row(:lx_market_events, legacy_row)
  end

  describe "definitions/0" do
    test "every topic0 constant is keccak256 of the event signature from the precompile ABI" do
      for definition <- PaxeerXLogs.definitions() do
        assert definition.signature == canonical_signature(definition),
               "#{definition.name} signature does not match its argument list"

        assert definition.topic == "0x" <> Base.encode16(ExKeccak.hash_256(definition.signature), case: :lower),
               "#{definition.name} topic0 is not keccak256(#{definition.signature})"
      end
    end

    test "every event is attributed to the precompile that emits it" do
      by_table = Enum.group_by(PaxeerXLogs.definitions(), & &1.table, & &1.address)

      assert by_table |> Map.fetch!(:lx_account_bindings) |> Enum.uniq() == [@addr_precompile]
      for table <- @retired_keys, do: refute(Map.has_key?(by_table, table))
      refute Enum.any?(PaxeerXLogs.definitions(), &(&1.address in [@custody_precompile, @anchor_precompile]))

      assert by_table |> Map.fetch!(:lx_market_events) |> Enum.uniq() |> Enum.sort() == [
               @exchange_precompile,
               @bridge_precompile,
               @launchpad_precompile
             ]
    end

    test "no two events share an emitter and a topic0" do
      keys = Enum.map(PaxeerXLogs.definitions(), &{&1.address, &1.topic})

      assert keys == Enum.uniq(keys)
    end

    test "no event targets lx_receipts: a kernel receipt is not an EVM event" do
      refute Enum.any?(PaxeerXLogs.definitions(), &(&1.table == :lx_receipts))
    end

    test "the retired custody kinds and directions cannot produce rows" do
      assert_retired_abi(
        "layerxcustody",
        @custody_precompile,
        ~w(CustodyDeposit ClaimQueued ClaimFinalised CustodyRelease EmergencyExitExecuted DepositRootRegistered)
      )
    end

    test "the retired submitted and final anchor statuses cannot produce rows" do
      assert_retired_abi("layerxanchor", @anchor_precompile, ~w(CheckpointSubmitted CheckpointFinalized))
    end

    test "the ten retired guarantor event kinds cannot produce rows" do
      assert_retired_abi(
        "layerxanchor",
        @anchor_precompile,
        ~w(AvailabilityAttested BondIncreased ChallengeOpened ChallengeResolved GuarantorActivated GuarantorRegistered
           GuarantorSlashed SequencerAuthorized UnbondBegun UnbondCompleted)
      )
    end

    test "retired deposit root registration cannot produce a table" do
      assert_retired_abi("layerxcustody", @custody_precompile, ~w(DepositRootRegistered))
    end

    test "every committed anchor and custody event is explicitly rejected" do
      for {name, address} <- [{"layerxcustody", @custody_precompile}, {"layerxanchor", @anchor_precompile}] do
        events = retired_definitions(name, address)
        assert length(events) == if(name == "layerxcustody", do: 6, else: 12)
        fixtures = Enum.map(Enum.with_index(events), &fixture/1)
        assert_retired_logs(Enum.map(fixtures, & &1.log))
      end
    end

    test "the market definitions cover the domains the schema declares" do
      market = Enum.filter(PaxeerXLogs.definitions(), &(&1.table == :lx_market_events))

      assert market |> Enum.map(& &1.domain) |> Enum.uniq() |> Enum.sort() == Enum.sort(MarketEvent.domains())

      for definition <- market do
        assert definition.kind == Macro.underscore(definition.name),
               "#{definition.name} has a kind that is not the underscore spelling of its event name"
      end
    end
  end

  describe "parse/1" do
    test "decodes one ABI encoded log per precompile event into a row its schema accepts" do
      fixtures = Enum.map(Enum.with_index(PaxeerXLogs.definitions()), &fixture/1)

      result = fixtures |> Enum.map(& &1.log) |> PaxeerXLogs.parse()

      assert Map.keys(result) |> Enum.sort() == Enum.sort(Map.keys(@empty))

      rows = Enum.flat_map(@row_tables, &result[&1])
      assert length(rows) == length(fixtures)
      assert result.undecoded_log_count == 0

      for %{definition: definition, log: log, arguments: arguments} <- fixtures do
        row =
          result
          |> Map.fetch!(definition.table)
          |> Enum.find(&(&1.log_index == log.index))

        assert row, "#{definition.name} produced no #{definition.table} row"
        assert row.event_name == definition.name
        assert row.transaction_hash == log.transaction_hash
        assert row.block_hash == log.block_hash
        assert row.block_number == log.block_number
        assert row.parameters == arguments

        assert_schema_row(definition.table, row)
      end
    end

    test "a binding log fills the promoted account binding columns" do
      definition = definition!(@addr_precompile, "LayerXBound(address,bytes32,uint64)")

      log =
        log(definition, 12, %{
          "evm" => <<0x11::160>>,
          "didPublicKey" => <<0x22::256>>,
          "nonce" => 7
        })

      assert %{lx_account_bindings: [row]} = PaxeerXLogs.parse([log])

      assert row.evm_address_hash == "0x0000000000000000000000000000000000000011"
      assert row.layerx_did == "did:layerx:0000000000000000000000000000000000000000000000000000000000000022"
      assert row.nonce == 7
      assert row.bound == true
      assert row.event_name == "LayerXBound"
      assert row.log_index == 12
      assert row.parameters["didPublicKey"] == "0x0000000000000000000000000000000000000000000000000000000000000022"

      assert_schema_row(:lx_account_bindings, row)
    end

    test "an unbinding log is the same row with bound false" do
      definition = definition!(@addr_precompile, "LayerXUnbound(address,bytes32,uint64)")

      log =
        log(definition, 13, %{
          "evm" => <<0x11::160>>,
          "didPublicKey" => <<0x22::256>>,
          "nonce" => 8
        })

      assert %{lx_account_bindings: [row]} = PaxeerXLogs.parse([log])

      assert row.bound == false
      assert row.event_name == "LayerXUnbound"

      assert_schema_row(:lx_account_bindings, row)
    end

    test "retired: a custody deposit log is a deposit of the payer, not of the precompile is rejected" do
      definition =
        retired_definition!(@custody_precompile, "CustodyDeposit(bytes32,bytes32,address,bytes32,uint256,uint64)")

      log =
        log(definition, 3, %{
          "depositId" => <<0xAB::256>>,
          "assetId" => <<0xCD::256>>,
          "payer" => <<0xEF::160>>,
          "beneficiary" => <<0x99::256>>,
          "amount" => 1_500_000_000_000_000_000,
          "nonce" => 42
        })

      assert_retired_logs([log])
    end

    test "retired: a claim queued log keeps the claim, the nullifier and the checkpoint apart is rejected" do
      definition =
        retired_definition!(@custody_precompile, "ClaimQueued(bytes32,bytes32,bytes32,bytes32,address,uint256,uint64)")

      log =
        log(definition, 4, %{
          "claimId" => <<0x01::256>>,
          "nullifier" => <<0x02::256>>,
          "checkpointHash" => <<0x03::256>>,
          "assetId" => <<0x04::256>>,
          "recipient" => <<0x05::160>>,
          "amount" => 250,
          "availableAt" => 1_700_000_000
        })

      assert_retired_logs([log])
    end

    test "retired: a claim finalisation is a balance delta that carries only its linkage is rejected" do
      definition = retired_definition!(@custody_precompile, "ClaimFinalised(bytes32,bytes32)")

      log = log(definition, 2, %{"claimId" => <<0x01::256>>, "nullifier" => <<0x02::256>>})

      assert_retired_logs([log])
    end

    test "retired: an emergency exit is a withdrawal of its recipient crediting its kernel account is rejected" do
      definition =
        retired_definition!(
          @custody_precompile,
          "EmergencyExitExecuted(bytes32,bytes32,bytes32,bytes32,bytes32,address,uint256)"
        )

      log =
        log(definition, 14, %{
          "claimId" => <<0x01::256>>,
          "nullifier" => <<0x02::256>>,
          "checkpointHash" => <<0x03::256>>,
          "account" => <<0x04::256>>,
          "assetId" => <<0x05::256>>,
          "recipient" => <<0x06::160>>,
          "amount" => 9
        })

      assert_retired_logs([log])
    end

    test "retired: a checkpoint submission fills the promoted anchor columns with the submitted status is rejected" do
      definition = retired_definition!(@anchor_precompile, "CheckpointSubmitted(uint64,bytes32,bytes32,bytes32,uint8)")

      log =
        log(definition, 0, %{
          "batchNumber" => 91_234,
          "checkpointId" => <<0x7A::256>>,
          "stateRoot" => <<0x7B::256>>,
          "receiptRoot" => <<0x7C::256>>,
          "signers" => 9
        })

      assert_retired_logs([log])
    end

    test "retired: a checkpoint finalization is the same checkpoint with the final status is rejected" do
      definition = retired_definition!(@anchor_precompile, "CheckpointFinalized(uint64,bytes32,bytes32,bytes32)")

      log =
        log(definition, 1, %{
          "batchNumber" => 91_234,
          "checkpointId" => <<0x7A::256>>,
          "stateRoot" => <<0x7B::256>>,
          "receiptRoot" => <<0x7C::256>>
        })

      assert_retired_logs([log])
    end

    test "retired: a guarantor registration fills the signer, the operator and the bond is rejected" do
      definition = retired_definition!(@anchor_precompile, "GuarantorRegistered(bytes32,address,address,uint256,uint8)")

      log =
        log(definition, 20, %{
          "guarantorId" => <<0x1A::256>>,
          "signer" => <<0x21::160>>,
          "operator" => <<0x22::160>>,
          "bond" => 5_000_000_000_000_000_000,
          "status" => 1
        })

      assert_retired_logs([log])
    end

    test "retired: a guarantor activation carries its guarantor and nothing else is rejected" do
      definition = retired_definition!(@anchor_precompile, "GuarantorActivated(bytes32)")

      log = log(definition, 21, %{"guarantorId" => <<0x1A::256>>})

      assert_retired_logs([log])
    end

    test "retired: a slash records the slashed value, the batch and the reporter's reward is rejected" do
      definition =
        retired_definition!(@anchor_precompile, "GuarantorSlashed(bytes32,uint8,uint64,uint256,address,uint256)")

      log =
        log(definition, 22, %{
          "guarantorId" => <<0x1A::256>>,
          "reason" => 2,
          "batchNumber" => 91_234,
          "amount" => 4_000_000_000_000_000_000,
          "reporter" => <<0x31::160>>,
          "reporterReward" => 400_000_000_000_000_000
        })

      assert_retired_logs([log])
    end

    test "retired: a bond increase keeps the moved value apart from the resulting bond is rejected" do
      definition = retired_definition!(@anchor_precompile, "BondIncreased(bytes32,uint256,uint256)")

      log =
        log(definition, 23, %{
          "guarantorId" => <<0x1A::256>>,
          "amount" => 1_000_000_000_000_000_000,
          "bond" => 6_000_000_000_000_000_000
        })

      assert_retired_logs([log])
    end

    test "retired: a begun unbonding carries the value and the moment it becomes claimable is rejected" do
      definition = retired_definition!(@anchor_precompile, "UnbondBegun(bytes32,uint256,uint64)")

      log =
        log(definition, 24, %{
          "guarantorId" => <<0x1A::256>>,
          "amount" => 2_000_000_000_000_000_000,
          "completionTime" => 1_700_000_000
        })

      assert_retired_logs([log])
    end

    test "retired: a completed unbonding is the same guarantor without a completion time is rejected" do
      definition = retired_definition!(@anchor_precompile, "UnbondCompleted(bytes32,uint256)")

      log = log(definition, 25, %{"guarantorId" => <<0x1A::256>>, "amount" => 2_000_000_000_000_000_000})

      assert_retired_logs([log])
    end

    test "retired: an opened challenge keeps its kind, its evidence and its challenger is rejected" do
      definition = retired_definition!(@anchor_precompile, "ChallengeOpened(uint64,uint64,uint8,bytes32,address)")

      log =
        log(definition, 26, %{
          "challengeId" => 77,
          "batchNumber" => 91_234,
          "kind" => 3,
          "evidenceHash" => <<0x4A::256>>,
          "challenger" => <<0x4B::160>>
        })

      assert_retired_logs([log])
    end

    test "retired: a resolved challenge records whether it was upheld is rejected" do
      definition = retired_definition!(@anchor_precompile, "ChallengeResolved(uint64,uint64,bool)")

      log = log(definition, 27, %{"challengeId" => 77, "batchNumber" => 91_234, "upheld" => true})

      assert_retired_logs([log])
    end

    test "retired: an availability attestation keeps the classes it covers and the ones it reports available is rejected" do
      definition = retired_definition!(@anchor_precompile, "AvailabilityAttested(uint64,bytes32,uint8,uint8)")

      log =
        log(definition, 28, %{
          "batchNumber" => 91_234,
          "guarantorId" => <<0x5E::256>>,
          "classMask" => 3,
          "availabilityMask" => 1
        })

      assert_retired_logs([log])
    end

    test "retired: a sequencer authorisation carries its public key and the batch range it covers is rejected" do
      definition = retired_definition!(@anchor_precompile, "SequencerAuthorized(bytes32,bytes32,uint64,uint64)")

      log =
        log(definition, 29, %{
          "sequencerId" => <<0x6A::256>>,
          "publicKey" => <<0x6B::256>>,
          "firstBatchNumber" => 91_000,
          "lastBatchNumber" => 92_000
        })

      assert_retired_logs([log])
    end

    test "retired: a deposit root registration is a deposit root, not a custody movement is rejected" do
      definition = retired_definition!(@custody_precompile, "DepositRootRegistered(bytes32,bytes32,bytes32,uint16)")

      log =
        log(definition, 30, %{
          "checkpointId" => <<0x7A::256>>,
          "depositRoot" => <<0x7B::256>>,
          "commitment" => <<0x7C::256>>,
          "version" => 1
        })

      assert_retired_logs([log])
    end

    test "an exchange log is a market event of the exchange domain" do
      definition =
        definition!(@exchange_precompile, "OrderPlaced(bytes32,bytes32,address,uint8,uint256,uint256,uint8,uint64)")

      log =
        log(definition, 5, %{
          "intentId" => <<0x21::256>>,
          "marketId" => <<0x22::256>>,
          "owner" => <<0x23::160>>,
          "side" => 1,
          "price" => 42_000_000,
          "quantity" => 3,
          "timeInForce" => 2,
          "nonce" => 11
        })

      assert %{lx_market_events: [row]} = PaxeerXLogs.parse([log])

      assert row.domain == :exchange
      assert row.kind == "order_placed"
      assert row.address_hash == address(0x23)
      assert row.amount == nil
      assert row.asset_address_hash == nil
      assert row.parameters["marketId"] == bytes32(0x22)
      assert row.parameters["price"] == 42_000_000
      assert row.parameters["nonce"] == 11

      assert_schema_row(:lx_market_events, row)
    end

    test "a margin deposit keeps both the EVM owner and the kernel account" do
      definition =
        definition!(@exchange_precompile, "MarginDeposited(bytes32,bytes32,address,bytes32,uint256,bytes32,uint64)")

      log =
        log(definition, 15, %{
          "intentId" => <<0x31::256>>,
          "account" => <<0x32::256>>,
          "owner" => <<0x33::160>>,
          "assetId" => <<0x34::256>>,
          "amount" => 4_000,
          "depositId" => <<0x35::256>>,
          "nonce" => 3
        })

      assert %{lx_market_events: [row]} = PaxeerXLogs.parse([log])

      assert row.kind == "margin_deposited"
      assert row.address_hash == address(0x33)
      assert row.account == bytes32(0x32)
      assert row.asset_id == bytes32(0x34)
      assert row.amount == 4_000

      assert_schema_row(:lx_market_events, row)
    end

    test "a bridge log decodes its dynamic denom and keeps the source chain" do
      definition = definition!(@bridge_precompile, "BridgeIn(uint64,bytes32,address,uint64,address,uint256,string)")

      log =
        log(definition, 6, %{
          "chain" => 125,
          "txHash" => <<0x31::256>>,
          "recipient" => <<0x32::160>>,
          "logIndex" => 4,
          "asset" => <<0x33::160>>,
          "amount" => 10_000,
          "denom" => "ulxbtc"
        })

      assert %{lx_market_events: [row]} = PaxeerXLogs.parse([log])

      assert row.domain == :bridge
      assert row.kind == "bridge_in"
      assert row.address_hash == address(0x32)
      assert row.asset_address_hash == address(0x33)
      assert row.amount == 10_000
      assert row.parameters["chain"] == 125
      assert row.parameters["denom"] == "ulxbtc"
      assert row.parameters["logIndex"] == 4

      assert_schema_row(:lx_market_events, row)
    end

    test "a bridge out log reads its nonce from the third topic" do
      definition = definition!(@bridge_precompile, "BridgeOut(uint64,address,uint256,address,uint64)")

      log =
        log(definition, 7, %{
          "chain" => 1,
          "asset" => <<0x41::160>>,
          "amount" => 77,
          "recipient" => <<0x42::160>>,
          "nonce" => 512
        })

      assert %{lx_market_events: [row]} = PaxeerXLogs.parse([log])

      assert row.parameters["nonce"] == 512
      assert row.asset_address_hash == address(0x41)
      assert row.address_hash == address(0x42)
      assert row.amount == 77

      assert_schema_row(:lx_market_events, row)
    end

    test "a launchpad swap prefers the trader over the recipient as the acting address" do
      definition =
        definition!(@launchpad_precompile, "Swap(address,address,address,bool,uint256,uint256,uint256,uint256)")

      log =
        log(definition, 8, %{
          "token" => <<0x51::160>>,
          "trader" => <<0x52::160>>,
          "recipient" => <<0x53::160>>,
          "isBuy" => true,
          "amountIn" => 100,
          "amountOut" => 98,
          "feeAmount" => 2,
          "price" => 1_020_000
        })

      assert %{lx_market_events: [row]} = PaxeerXLogs.parse([log])

      assert row.domain == :launchpad
      assert row.kind == "swap"
      assert row.address_hash == address(0x52)
      assert row.asset_address_hash == address(0x51)
      assert row.amount == nil
      assert row.parameters["isBuy"] == true
      assert row.parameters["amountOut"] == 98

      assert_schema_row(:lx_market_events, row)
    end

    test "a launchpad market creation decodes its three dynamic strings" do
      definition = definition!(@launchpad_precompile, "MarketCreated(address,address,string,string,string,uint8)")

      log =
        log(definition, 9, %{
          "token" => <<0x61::160>>,
          "creator" => <<0x62::160>>,
          "denom" => "factory/pax1/demo",
          "name" => "Demo Market",
          "symbol" => "DEMO",
          "feeStrategy" => 2
        })

      assert %{lx_market_events: [row]} = PaxeerXLogs.parse([log])

      assert row.kind == "market_created"
      assert row.address_hash == address(0x62)
      assert row.parameters["denom"] == "factory/pax1/demo"
      assert row.parameters["name"] == "Demo Market"
      assert row.parameters["symbol"] == "DEMO"
      assert row.parameters["feeStrategy"] == 2

      assert_schema_row(:lx_market_events, row)
    end

    test "a topic0 written in upper case still matches" do
      definition = definition!(@launchpad_precompile, "FeesBurned(address,uint256)")

      log =
        definition
        |> log(10, %{"token" => <<0x71::160>>, "amount" => 5})
        |> Map.update!(:first_topic, &String.upcase/1)

      assert %{lx_market_events: [row]} = PaxeerXLogs.parse([log])

      assert row.kind == "fees_burned"
      assert row.amount == 5

      assert_schema_row(:lx_market_events, row)
    end

    test "retired result keys are absent: a kernel receipt is not an EVM event" do
      fixtures = Enum.map(Enum.with_index(PaxeerXLogs.definitions()), &fixture/1)

      result = fixtures |> Enum.map(& &1.log) |> PaxeerXLogs.parse()
      assert Enum.sort(Map.keys(result)) == Enum.sort(Map.keys(@empty))
      for key <- @retired_keys, do: refute(Map.has_key?(result, key))
    end

    test "an empty log list yields empty lists" do
      assert PaxeerXLogs.parse([]) == @empty
    end

    test "a log that is not a precompile event yields nothing" do
      erc20_transfer = %{
        address_hash: "0xf2eec76e45b328df99a34fa696320a262cb92154",
        block_hash: "0x79594150677f083756a37eee7b97ed99ab071f502104332cb3835bac345711ca",
        block_number: 3_530_917,
        data: "0x000000000000000000000000000000000000000000000000ebec21ee1da40000",
        first_topic: @erc20_transfer_topic,
        second_topic: "0x000000000000000000000000556813d9cc20acfe8388af029a679d34a63388db",
        third_topic: "0x00000000000000000000000092148dd870fa1b7c4700f2bd7f44238821c26f73",
        fourth_topic: nil,
        index: 8,
        transaction_hash: "0x43dfd761974e8c3351d285ab65bee311454eb45b149a015fe7804a33252f19e5"
      }

      assert PaxeerXLogs.parse([erc20_transfer]) == @empty
    end

    test "a precompile topic0 emitted by an ordinary contract yields nothing" do
      definition =
        retired_definition!(@custody_precompile, "CustodyDeposit(bytes32,bytes32,address,bytes32,uint256,uint64)")

      log =
        definition
        |> log(3, %{
          "depositId" => <<0xAB::256>>,
          "assetId" => <<0xCD::256>>,
          "payer" => <<0xEF::160>>,
          "beneficiary" => <<0x99::256>>,
          "amount" => 1,
          "nonce" => 1
        })
        |> Map.put(:address_hash, "0xf2eec76e45b328df99a34fa696320a262cb92154")

      assert PaxeerXLogs.parse([log]) == @empty
    end

    test "an unknown topic0 at a retained precompile address yields no row and one undecoded log" do
      log =
        definition!(@addr_precompile, "LayerXBound(address,bytes32,uint64)")
        |> log(3, %{"evm" => <<0x11::160>>, "didPublicKey" => <<0x22::256>>, "nonce" => 7})
        |> Map.put(:first_topic, @erc20_transfer_topic)

      assert PaxeerXLogs.parse([log]) == %{@empty | undecoded_log_count: 1}
    end

    test "the committed ABI set leaves no precompile log undecoded" do
      fixtures = Enum.map(Enum.with_index(PaxeerXLogs.definitions()), &fixture/1)

      result = fixtures |> Enum.map(& &1.log) |> PaxeerXLogs.parse()

      assert result.undecoded_log_count == 0
      assert length(Enum.flat_map(@row_tables, &result[&1])) == length(PaxeerXLogs.definitions())
    end

    test "a precompile log whose topic0 no definition matches is counted rather than dropped" do
      unknown = %{
        address_hash: @addr_precompile,
        block_hash: "0x79594150677f083756a37eee7b97ed99ab071f502104332cb3835bac345711ca",
        block_number: 42,
        data: "0x" <> Base.encode16(TypeEncoder.encode([5], [{:uint, 256}]), case: :lower),
        first_topic: "0x" <> Base.encode16(ExKeccak.hash_256("GuarantorRetired(bytes32,uint256)"), case: :lower),
        second_topic: bytes32(1),
        third_topic: nil,
        fourth_topic: nil,
        index: 0,
        transaction_hash: "0x43dfd761974e8c3351d285ab65bee311454eb45b149a015fe7804a33252f19e5"
      }

      decoded =
        log(definition!(@addr_precompile, "LayerXBound(address,bytes32,uint64)"), 1, %{
          "evm" => <<0x11::160>>,
          "didPublicKey" => <<0x22::256>>,
          "nonce" => 7
        })

      result = PaxeerXLogs.parse([unknown, decoded])

      assert result.undecoded_log_count == 1
      assert [%{event_name: "LayerXBound", nonce: 7}] = result.lx_account_bindings
      assert result.lx_market_events == []
      for key <- @retired_keys, do: refute(Map.has_key?(result, key))
    end

    test "a log missing an indexed topic yields no row and one undecoded log" do
      log =
        definition!(@addr_precompile, "LayerXBound(address,bytes32,uint64)")
        |> log(2, %{"evm" => <<0x11::160>>, "didPublicKey" => <<0x22::256>>, "nonce" => 7})
        |> Map.put(:third_topic, nil)

      assert PaxeerXLogs.parse([log]) == %{@empty | undecoded_log_count: 1}
    end

    test "a log carrying no data for its unindexed arguments yields no row and one undecoded log" do
      log =
        definition!(@addr_precompile, "LayerXBound(address,bytes32,uint64)")
        |> log(4, %{"evm" => <<0x11::160>>, "didPublicKey" => <<0x22::256>>, "nonce" => 7})
        |> Map.put(:data, "0x")

      assert PaxeerXLogs.parse([log]) == %{@empty | undecoded_log_count: 1}
    end

    test "current perps OrderPlaced uses its own topic and address account layout" do
      definition = definition!(@exchange_precompile, "OrderPlaced(uint256,address,uint256,uint8,bool,uint256,uint256)")

      values = %{
        "orderId" => 91,
        "user" => <<0x23::160>>,
        "marketId" => 22,
        "orderType" => 1,
        "isLong" => true,
        "triggerPrice" => 42_000_000,
        "sizeUsd" => 3_000_000
      }

      log = log(definition, 31, values)
      assert %{lx_market_events: [row], lx_account_bindings: [], undecoded_log_count: 0} = PaxeerXLogs.parse([log])
      assert row.domain == :exchange
      assert row.kind == "order_placed"
      assert row.address_hash == address(0x23)
      assert row.account == nil
      assert row.parameters == Map.put(values, "user", address(0x23))
      refute Map.has_key?(row.parameters, "intentId")
      refute Map.has_key?(row.parameters, "nonce")
      assert_schema_row(:lx_market_events, row)
    end
  end

  defp assert_schema_row(table, row) do
    schema = Map.fetch!(@schemas, table)
    columns = MapSet.new(schema.__schema__(:fields))
    unknown = row |> Map.keys() |> MapSet.new() |> MapSet.difference(columns) |> MapSet.to_list()

    assert unknown == [], "#{table} row carries keys that are not columns of #{inspect(schema)}: #{inspect(unknown)}"
    assert is_binary(row.event_name)
    assert is_map(row.parameters) and map_size(row.parameters) > 0

    changeset = schema.changeset(struct(schema), row)

    assert changeset.valid?,
           "#{table} row #{inspect(row)} is rejected by #{inspect(schema)}: #{inspect(changeset.errors)}"

    changeset
  end

  defp definition!(address, signature) do
    [definition] = Enum.filter(PaxeerXLogs.definitions(), &(&1.address == address and &1.signature == signature))
    definition
  end

  defp assert_retired_logs(logs) do
    assert logs != []

    for log <- logs do
      assert log.address_hash in [@custody_precompile, @anchor_precompile]
      refute Enum.any?(PaxeerXLogs.definitions(), &(&1.address == log.address_hash))
    end

    result = PaxeerXLogs.parse(logs)
    assert result == @empty
    for key <- @retired_keys, do: refute(Map.has_key?(result, key))
  end

  defp assert_retired_abi(name, address, names) do
    definitions = retired_definitions(name, address) |> Enum.filter(&(&1.name in names))
    assert Enum.sort(Enum.map(definitions, & &1.name)) == Enum.sort(names)

    for {definition, index} <- Enum.with_index(definitions) do
      assert_retired_logs([fixture({definition, index}).log])
    end
  end

  defp retired_definition!(address, signature) do
    name = if address == @custody_precompile, do: "layerxcustody", else: "layerxanchor"
    [definition] = Enum.filter(retired_definitions(name, address), &(&1.signature == signature))
    definition
  end

  defp retired_definitions(name, address) do
    root = System.get_env("PAXEER_PRECOMPILES_DIR") || Path.expand("../../../../../../../../precompiles", __DIR__)

    root
    |> Path.join("#{name}/abi.json")
    |> File.read!()
    |> Jason.decode!()
    |> Enum.filter(&(&1["type"] == "event"))
    |> Enum.map(fn event ->
      signature = event["name"] <> "(" <> Enum.map_join(event["inputs"], ",", & &1["type"]) <> ")"

      %{
        address: address,
        name: event["name"],
        signature: signature,
        topic: "0x" <> Base.encode16(ExKeccak.hash_256(signature), case: :lower),
        arguments: Enum.map(event["inputs"], &{&1["name"], from_abi_type(&1["type"]), &1["indexed"]})
      }
    end)
  end

  defp from_abi_type("address"), do: :address
  defp from_abi_type("bool"), do: :bool
  defp from_abi_type("bytes32"), do: {:bytes, 32}
  defp from_abi_type("uint" <> bits), do: {:uint, String.to_integer(bits)}

  defp bytes32(value), do: "0x" <> (value |> Integer.to_string(16) |> String.downcase() |> String.pad_leading(64, "0"))

  defp address(value), do: "0x" <> (value |> Integer.to_string(16) |> String.downcase() |> String.pad_leading(40, "0"))

  defp canonical_signature(%{name: name, arguments: arguments}) do
    name <> "(" <> Enum.map_join(arguments, ",", fn {_name, type, _indexed?} -> abi_type(type) end) <> ")"
  end

  defp abi_type(:address), do: "address"
  defp abi_type(:bool), do: "bool"
  defp abi_type(:string), do: "string"
  defp abi_type({:bytes, size}), do: "bytes#{size}"
  defp abi_type({:uint, bits}), do: "uint#{bits}"
  defp abi_type({:int, bits}), do: "int#{bits}"

  defp fixture({definition, index}) do
    values =
      definition.arguments
      |> Enum.with_index()
      |> Map.new(fn {{name, type, _indexed?}, position} ->
        {name, sample(type, index * 31 + position + 1)}
      end)

    %{
      definition: definition,
      log: log(definition, index, values),
      arguments: Map.new(values, fn {name, value} -> {name, expected(type_of(definition, name), value)} end)
    }
  end

  defp type_of(definition, name) do
    Enum.find_value(definition.arguments, fn {argument, type, _indexed?} -> argument == name && type end)
  end

  defp sample(:address, seed), do: <<seed::160>>
  defp sample({:bytes, 32}, seed), do: <<seed::256>>
  defp sample({:uint, bits}, seed) when bits <= 16, do: rem(seed, 200) + 1
  defp sample({:uint, _bits}, seed), do: seed * 7 + 3
  defp sample({:int, 256}, seed), do: -(seed * 7 + 3)
  defp sample(:bool, seed), do: rem(seed, 2) == 0
  defp sample(:string, seed), do: "denom-#{seed}"

  defp expected(:address, value), do: "0x" <> Base.encode16(value, case: :lower)
  defp expected({:bytes, 32}, value), do: "0x" <> Base.encode16(value, case: :lower)
  defp expected(_type, value), do: value

  defp log(definition, index, values) do
    {indexed, unindexed} = Enum.split_with(definition.arguments, fn {_name, _type, indexed?} -> indexed? end)

    [second_topic, third_topic, fourth_topic] =
      indexed
      |> Enum.map(fn {name, type, _indexed?} -> topic(type, Map.fetch!(values, name)) end)
      |> Kernel.++([nil, nil, nil])
      |> Enum.take(3)

    %{
      address_hash: definition.address,
      block_hash: "0x79594150677f083756a37eee7b97ed99ab071f502104332cb3835bac345711ca",
      block_number: 4_000_000 + index,
      data: data(unindexed, values),
      first_topic: definition.topic,
      second_topic: second_topic,
      third_topic: third_topic,
      fourth_topic: fourth_topic,
      index: index,
      transaction_hash: "0x8425a9b81a9bd1c64861110c1a453b84719cb0361d6fa0db68abf7611b9a890e"
    }
  end

  defp data([], _values), do: "0x"

  defp data(unindexed, values) do
    types = Enum.map(unindexed, fn {_name, type, _indexed?} -> type end)
    arguments = Enum.map(unindexed, fn {name, _type, _indexed?} -> Map.fetch!(values, name) end)

    "0x" <> Base.encode16(TypeEncoder.encode(arguments, types), case: :lower)
  end

  defp topic(:address, value), do: "0x" <> String.duplicate("0", 24) <> Base.encode16(value, case: :lower)
  defp topic({:bytes, 32}, value), do: "0x" <> Base.encode16(value, case: :lower)
  defp topic(:bool, value), do: topic({:uint, 256}, if(value, do: 1, else: 0))

  defp topic({:uint, _bits}, value) do
    "0x" <> (value |> Integer.to_string(16) |> String.downcase() |> String.pad_leading(64, "0"))
  end
end
