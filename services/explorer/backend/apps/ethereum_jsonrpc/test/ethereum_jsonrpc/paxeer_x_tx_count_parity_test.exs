defmodule EthereumJSONRPC.PaxeerXTxCountParityTest do
  @moduledoc """
  The mixed block fixture uses the reconstructed Cosmos-originated transaction
  from cosmos_originated_transaction.json and an EVM transfer at the same height.
  The block and count answers describe that reconstructed two-transaction block.
  """

  use ExUnit.Case, async: true

  alias EthereumJSONRPC.Blocks

  test "block_tx_count_matches_count_method" do
    fixture =
      __DIR__
      |> Path.join("../support/fixture/paxeer_x/tx_count_parity.json")
      |> File.read!()
      |> Jason.decode!()

    %{
      "block_request" => block_request,
      "block_response" => %{"id" => block_id, "result" => block},
      "count_request" => count_request,
      "count_response" => %{"id" => count_id, "result" => count_quantity}
    } = fixture

    assert block_request == %{
             "jsonrpc" => "2.0",
             "id" => block_id,
             "method" => "eth_getBlockByNumber",
             "params" => [block["number"], true]
           }

    assert count_request == %{
             "jsonrpc" => "2.0",
             "id" => count_id,
             "method" => "eth_getBlockTransactionCountByNumber",
             "params" => [block["number"]]
           }

    number = EthereumJSONRPC.quantity_to_integer(block["number"])
    count = EthereumJSONRPC.quantity_to_integer(count_quantity)

    assert %Blocks{blocks_params: [block_params], transactions_params: transactions, errors: []} =
             Blocks.from_responses(
               [%{id: block_id, result: block}],
               %{block_id => %{number: number}}
             )

    assert block_params.number == number
    assert block_params.hash == block["hash"]
    assert count == 2
    assert length(transactions) == count
    assert length(block["transactions"]) == count
    assert Enum.map(transactions, & &1.hash) == Enum.map(block["transactions"], & &1["hash"])
    assert Enum.map(transactions, & &1.index) == [0, 1]

    for transaction <- transactions do
      assert transaction.block_number == number
      assert transaction.block_hash == block_params.hash
    end

    [cosmos_wire, evm_wire] = block["transactions"]
    [cosmos, evm] = transactions

    assert is_nil(cosmos_wire["value"])
    assert is_nil(cosmos_wire["gasPrice"])
    assert cosmos.value == 0
    assert cosmos.gas_price == 0
    assert cosmos.input == cosmos_wire["input"]
    assert evm.value == EthereumJSONRPC.quantity_to_integer(evm_wire["value"])
    assert evm.gas_price == EthereumJSONRPC.quantity_to_integer(evm_wire["gasPrice"])
  end
end
