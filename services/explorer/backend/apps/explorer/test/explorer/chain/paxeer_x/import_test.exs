defmodule Explorer.Chain.PaxeerX.ImportTest do
  use Explorer.DataCase

  alias Explorer.Chain.Import
  alias Explorer.Chain.PaxeerX.{AccountBinding, MarketEvent}

  setup do
    chain_type = Application.get_env(:explorer, :chain_type)
    Application.put_env(:explorer, :chain_type, :paxeer_x)

    on_exit(fn -> Application.put_env(:explorer, :chain_type, chain_type) end)

    block = insert(:block)
    transaction = :transaction |> insert() |> with_block(block)

    %{block: block, transaction: transaction}
  end

  describe "Explorer.Chain.Import.all/1 with the paxeer_x runners" do
    test "writes account bindings and market events and reads them back", %{block: block, transaction: transaction} do
      assert {:ok, imported} = Import.all(import_data(block, transaction))

      assert [%AccountBinding{}] = imported[:insert_paxeer_x_account_bindings]
      assert [%MarketEvent{}] = imported[:insert_paxeer_x_market_events]

      binding = Repo.one!(AccountBinding)
      assert binding.transaction_hash == transaction.hash
      assert binding.block_hash == block.hash
      assert binding.block_number == block.number
      assert binding.log_index == 0
      assert binding.bound == true
      assert binding.event_name == "LayerXBound"
      assert to_string(binding.evm_address_hash) == "0x0000000000000000000000000000000000000011"
      assert binding.layerx_did == "did:layerx:" <> String.duplicate("0", 62) <> "22"
      assert binding.nonce == 7
      assert binding.parameters["nonce"] == 7
      assert binding.block_consensus == true

      market = Repo.one!(MarketEvent)
      assert market.domain == :launchpad
      assert market.kind == "swap"
      assert market.event_name == "Swap"
      assert to_string(market.address_hash) == "0x0000000000000000000000000000000000000052"
      assert to_string(market.asset_address_hash) == "0x0000000000000000000000000000000000000051"
      assert market.parameters["isBuy"] == true
    end

    test "re-importing the same logs keeps one row per log", %{block: block, transaction: transaction} do
      data = import_data(block, transaction)

      assert {:ok, _} = Import.all(data)
      assert {:ok, _} = Import.all(data)

      assert Repo.aggregate(AccountBinding, :count) == 1
      assert Repo.aggregate(MarketEvent, :count) == 1
    end

    test "a market event whose domain the schema does not declare is rejected before any insert", %{
      block: block,
      transaction: transaction
    } do
      data =
        block
        |> import_data(transaction)
        |> put_in([:paxeer_x_market_events, :params, Access.at(0), :domain], :retired)

      assert {:error, [changeset]} = Import.all(data)
      refute changeset.valid?
      assert {"is invalid", _} = changeset.errors[:domain]
      assert Repo.aggregate(MarketEvent, :count) == 0
      assert Repo.aggregate(AccountBinding, :count) == 0
    end
  end

  test "import_writes_no_retired_tables", %{block: block, transaction: transaction} do
    assert Repo.aggregate("lx_custody_events", :count, :log_index) == 0
    assert Repo.aggregate("lx_deposit_roots", :count, :log_index) == 0
    assert Repo.aggregate("lx_guarantor_events", :count, :log_index) == 0

    assert {:ok, imported} = Import.all(import_data(block, transaction))
    assert [%AccountBinding{}] = imported[:insert_paxeer_x_account_bindings]
    assert [%MarketEvent{}] = imported[:insert_paxeer_x_market_events]
    assert Repo.aggregate(AccountBinding, :count) == 1
    assert Repo.aggregate(MarketEvent, :count) == 1

    assert Repo.aggregate("lx_custody_events", :count, :log_index) == 0
    assert Repo.aggregate("lx_deposit_roots", :count, :log_index) == 0
    assert Repo.aggregate("lx_guarantor_events", :count, :log_index) == 0
  end

  defp import_data(block, transaction) do
    key = %{
      transaction_hash: transaction.hash,
      block_hash: block.hash,
      block_number: block.number
    }

    %{
      paxeer_x_account_bindings: %{
        params: [
          Map.merge(key, %{
            log_index: 0,
            event_name: "LayerXBound",
            parameters: %{"evm" => address(0x11), "didPublicKey" => bytes32(0x22), "nonce" => 7},
            bound: true,
            evm_address_hash: address(0x11),
            layerx_did: "did:layerx:" <> String.duplicate("0", 62) <> "22",
            nonce: 7
          })
        ]
      },
      paxeer_x_market_events: %{
        params: [
          Map.merge(key, %{
            log_index: 4,
            event_name: "Swap",
            parameters: %{
              "token" => address(0x51),
              "trader" => address(0x52),
              "recipient" => address(0x53),
              "isBuy" => true,
              "amountIn" => 100,
              "amountOut" => 98,
              "feeAmount" => 2,
              "price" => 1_020_000
            },
            domain: :launchpad,
            kind: "swap",
            address_hash: address(0x52),
            asset_address_hash: address(0x51)
          })
        ]
      }
    }
  end

  defp bytes32(value), do: "0x" <> (value |> Integer.to_string(16) |> String.downcase() |> String.pad_leading(64, "0"))

  defp address(value), do: "0x" <> (value |> Integer.to_string(16) |> String.downcase() |> String.pad_leading(40, "0"))
end
