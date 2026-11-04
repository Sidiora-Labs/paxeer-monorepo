defmodule Explorer.Repair.Repair2233470Test do
  @moduledoc """
  Repair record 22.33.470: erc404-zero-balance-collection is covered by both
  transfer cases below, through the production schemas, import runners and NFT
  collection/instance consumers. No out-of-path wiring is required.

  Block heights and hashes come from the committed consecutive_blocks recording;
  ERC-404 balance shapes and token ID 333 follow the committed
  CurrentTokenBalancesTest import fixtures. Transfer participants use the regular
  chain factories. The scenarios replay imported ownership changes in PostgreSQL.

  Baseline verification was not run: the task's ordered gates require complete
  implementation before hf-build and allow verification only through hf-done.
  """

  use Explorer.DataCase

  alias EthereumJSONRPC.Block, as: RPCBlock
  alias Explorer.Chain.{Import, TokenTransfer}
  alias Explorer.Chain.Address.CurrentTokenBalance
  alias Explorer.Chain.Import.Runner.{TokenInstances, TokenTransfers}
  alias Explorer.Chain.Import.Runner.Address.CurrentTokenBalances
  alias Explorer.Chain.Token.Instance

  @blocks Path.expand("../../../ethereum_jsonrpc/test/support/fixture/paxeer_x/consecutive_blocks.json", __DIR__)
  @options [token_type: ["ERC-404"]]

  setup do
    [before_transfer, after_transfer] =
      @blocks
      |> File.read!()
      |> Jason.decode!()
      |> Map.values()
      |> Enum.map(&RPCBlock.to_elixir/1)
      |> Enum.sort_by(& &1["number"])

    assert after_transfer["number"] == before_transfer["number"] + 1

    block = insert(:block, number: after_transfer["number"], hash: after_transfer["hash"])
    transaction = :transaction |> insert() |> with_block(block)
    token = insert(:token, type: "ERC-404", holder_count: 0)
    sender = insert(:address)
    recipient = insert(:address)

    {:ok, multi} =
      Import.all_single_multi([TokenInstances], %{
        token_instances: %{
          params: Enum.map([333, 555], &%{token_contract_address_hash: token.contract_address_hash, token_id: &1})
        }
      })

    assert {:ok, %{token_instances: instances}} = Repo.transaction(multi)
    assert length(instances) == 2

    %{
      token: token,
      sender: sender,
      recipient: recipient,
      block: block,
      transaction: transaction,
      previous_height: before_transfer["number"]
    }
  end

  test "erc404-zero-balance-collection: transferring the last NFT removes the former owner's collection", context do
    import_balances(context, context.previous_height, [{context.sender, 333, 1}, {context.sender, nil, 25}])
    assert_collection(context.sender, context.token, [333])
    assert Instance.nft_collections(context.recipient.hash, @options) == []

    transfer(context)

    retained =
      Repo.get_by!(CurrentTokenBalance,
        address_hash: context.sender.hash,
        token_contract_address_hash: context.token.contract_address_hash,
        token_id: Decimal.new(333)
      )

    assert Decimal.equal?(retained.value, 0)
    assert Decimal.equal?(retained.old_value, 1)
    assert retained.block_number == context.block.number
    assert Instance.erc_404_collections_by_address_hash(context.sender.hash, @options) == []
    assert Instance.nft_collections(context.sender.hash, @options) == []
    assert Instance.nft_list(context.sender.hash, @options) == []
    assert_collection(context.recipient, context.token, [333])
  end

  test "erc404-zero-balance-collection: a retained NFT excludes transferred IDs from counts and previews", context do
    import_balances(context, context.previous_height, [
      {context.sender, 333, 1},
      {context.sender, 555, 1},
      {context.sender, nil, 25}
    ])

    assert_collection(context.sender, context.token, [333, 555])
    transfer(context)

    assert_collection(context.sender, context.token, [555])
    assert_collection(context.recipient, context.token, [333])
  end

  defp transfer(context) do
    params = %{
      block_number: context.block.number,
      block_hash: context.block.hash,
      transaction_hash: context.transaction.hash,
      log_index: 0,
      from_address_hash: context.sender.hash,
      to_address_hash: context.recipient.hash,
      token_contract_address_hash: context.token.contract_address_hash,
      token_type: "ERC-404",
      token_ids: [333],
      amount: 1,
      block_consensus: true
    }

    {:ok, multi} = Import.all_single_multi([TokenTransfers], %{token_transfers: %{params: [params]}})
    assert {:ok, %{token_transfers: [%TokenTransfer{} = imported]}} = Repo.transaction(multi)
    assert imported.from_address_hash == context.sender.hash
    assert imported.to_address_hash == context.recipient.hash
    assert imported.token_ids == [Decimal.new(333)]

    import_balances(context, context.block.number, [{context.sender, 333, 0}, {context.recipient, 333, 1}])
  end

  defp import_balances(context, height, balances) do
    params =
      Enum.map(balances, fn {address, token_id, value} ->
        %{
          address_hash: address.hash,
          token_contract_address_hash: context.token.contract_address_hash,
          token_type: "ERC-404",
          token_id: token_id,
          block_number: height,
          value: value,
          value_fetched_at: ~U[2026-09-01 00:00:00.000000Z]
        }
      end)

    {:ok, multi} =
      Import.all_single_multi([CurrentTokenBalances], %{address_current_token_balances: %{params: params}})

    assert {:ok, %{address_current_token_balances: imported}} = Repo.transaction(multi)
    assert length(imported) == length(params)
  end

  defp assert_collection(address, token, ids) do
    expected_ids = Enum.map(ids, &Decimal.new/1) |> Enum.sort(Decimal)
    [collection] = Instance.nft_collections(address.hash, @options)
    assert collection.token_contract_address_hash == token.contract_address_hash
    assert collection.token.type == "ERC-404"
    assert collection.distinct_token_instances_count == length(ids)
    assert Enum.sort(collection.token_ids, Decimal) == expected_ids

    assert Enum.sort_by(collection.preloaded_token_instances, & &1.token_id, Decimal) |> Enum.map(& &1.token_id) ==
             expected_ids

    assert Enum.all?(collection.preloaded_token_instances, fn instance ->
             instance.current_token_balance.address_hash == address.hash and
               Decimal.compare(instance.current_token_balance.value, 0) == :gt
           end)

    assert Instance.nft_list(address.hash, @options) |> Enum.map(& &1.token_id) |> Enum.sort(Decimal) == expected_ids
  end
end
