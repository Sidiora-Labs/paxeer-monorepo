defmodule BlockScoutWeb.Repair2233391Test do
  @moduledoc """
  Repair record 22.33.391 for state-changes-invents-zero-balances:
  * missing native history stays unavailable and pagination recovers fetched balances
  * an unfetched native placeholder differs from a fetched zero
  * earlier block transactions cannot turn pending history into a confirmed balance
  * recorded ERC-20 transfers recover missing token history on a later page
  * recorded ERC-1155 self-transfers distinguish pending history from confirmed zero
  No out-of-path wiring is required; both existing views already render negative
  balance sentinels as unavailable. The hf-done evidence records the verified revision.
  Baseline verification was not run: the bounded gate order requires implementation
  before hf-build and hf-done, and forbids a separate unchanged-tree verify run.
  """

  use BlockScoutWeb.ConnCase, async: false

  import Ecto.Query, only: [from: 2]

  alias EthereumJSONRPC.Transaction, as: JSONRPCTransaction
  alias Explorer.Chain.{Address, TokenTransfer, Transaction, Wei}
  alias Explorer.Chain.Address.CoinBalance
  alias Explorer.Chain.Cache.StateChanges
  alias Explorer.Chain.Transaction.StateChange
  alias Explorer.Repo
  alias Indexer.Transform.TokenTransfers

  @recordings Path.expand("../../../explorer/test/support/fixture/vcr_cassettes", __DIR__)
  @funds Decimal.new("100000000000000000000")

  # Recorded logs from Indexer.Transform.TokenTransfersTest, parsed by the real producer.
  @erc20_log %{
    address_hash: "0xf2eec76e45b328df99a34fa696320a262cb92154",
    block_number: 3_530_917,
    block_hash: "0x79594150677f083756a37eee7b97ed99ab071f502104332cb3835bac345711ca",
    data: "0x000000000000000000000000000000000000000000000000ebec21ee1da40000",
    first_topic: "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef",
    fourth_topic: nil,
    index: 8,
    second_topic: "0x000000000000000000000000556813d9cc20acfe8388af029a679d34a63388db",
    third_topic: "0x00000000000000000000000092148dd870fa1b7c4700f2bd7f44238821c26f73",
    transaction_hash: "0x43dfd761974e8c3351d285ab65bee311454eb45b149a015fe7804a33252f19e5"
  }
  @erc1155_log %{
    address_hash: "0x58Ab73CB79c8275628E0213742a85B163fE0A9Fb",
    block_number: 8_683_457,
    data:
      "0x1000000000000c520000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001",
    first_topic: "0xc3d58168c5ae7397731d063d5bbf3d657854427343f4c083240f7aacaa2d0f62",
    second_topic: "0x0000000000000000000000009c978f4cfa1fe13406bcc05baf26a35716f881dd",
    third_topic: "0x0000000000000000000000009c978f4cfa1fe13406bcc05baf26a35716f881dd",
    fourth_topic: "0x0000000000000000000000009c978f4cfa1fe13406bcc05baf26a35716f881dd",
    index: 2,
    transaction_hash: "0x6d2dd62c178e55a13b65601f227c4ffdd8aa4e3bcb1f24731363b4f7619e92c8",
    block_hash: "0x79594150677f083756a37eee7b97ed99ab071f502104332cb3835bac345711ca"
  }

  test "missing native history stays unavailable and pagination recovers fetched balances", %{conn: conn} do
    transaction = recorded_transaction()
    pending = state_changes(conn, transaction)
    assert length(pending["items"]) == 3
    Enum.each(pending["items"], &assert_unavailable/1)
    assert StateChanges.get(transaction.hash) == nil

    fund_native_participants(transaction)

    recovered_page = state_changes(conn, transaction, %{"items_count" => "1", "state_changes" => "null"})
    assert length(recovered_page["items"]) == 2

    assert Enum.map(recovered_page["items"], & &1["address"]["hash"]) ==
             Enum.map(Enum.drop(pending["items"], 1), & &1["address"]["hash"])

    recovered = state_changes(conn, transaction)
    assert recovered_page["items"] == Enum.drop(recovered["items"], 1)
    assert %StateChanges{state_changes: cached} = StateChanges.get(transaction.hash)
    assert length(cached) == 3

    sender = item(recovered, transaction.from_address_hash, "coin")
    receiver = item(recovered, transaction.to_address_hash, "coin")
    assert sender["balance_before"] == Decimal.to_string(@funds)
    assert sender["balance_after"] == Decimal.to_string(Decimal.sub(@funds, StateChange.from_loss(transaction).value))
    assert receiver["balance_before"] == Decimal.to_string(@funds)
    assert receiver["balance_after"] == Decimal.to_string(Decimal.add(@funds, transaction.value.value))
    assert receiver["change"] == to_string(transaction.value.value)
  end

  test "an unfetched native placeholder differs from a fetched zero", %{conn: conn} do
    transaction = recorded_transaction()
    fund_native_participants(transaction, [transaction.to_address_hash])

    placeholder =
      insert(:address_coin_balance,
        address: transaction.to_address,
        block_number: transaction.block_number - 1,
        value: nil,
        value_fetched_at: nil
      )

    response = state_changes(conn, transaction)
    assert_unavailable(item(response, transaction.to_address_hash, "coin"))
    assert item(response, transaction.from_address_hash, "coin")["balance_before"] == Decimal.to_string(@funds)
    assert StateChanges.get(transaction.hash) == nil

    assert {1, nil} =
             Repo.update_all(
               from(balance in CoinBalance,
                 where: balance.address_hash == ^placeholder.address_hash,
                 where: balance.block_number == ^placeholder.block_number
               ),
               set: [value: Wei.zero(), value_fetched_at: DateTime.utc_now()]
             )

    receiver = conn |> state_changes(transaction) |> item(transaction.to_address_hash, "coin")
    assert receiver["balance_before"] == "0"
    assert receiver["balance_after"] == to_string(transaction.value.value)
  end

  test "earlier block transactions cannot turn pending history into a confirmed balance", %{conn: conn} do
    transaction = recorded_transaction(1)
    fund_native_participants(transaction, [transaction.to_address_hash])

    earlier =
      insert(:transaction,
        block: transaction.block,
        block_hash: transaction.block_hash,
        block_number: transaction.block_number,
        block_consensus: true,
        index: 0,
        from_address: transaction.from_address,
        to_address: transaction.to_address,
        value: %Wei{value: Decimal.new(7)},
        gas_used: 21_000,
        cumulative_gas_used: 21_000,
        gas_price: transaction.gas_price,
        status: :ok
      )

    response = state_changes(conn, transaction)
    assert_unavailable(item(response, transaction.to_address_hash, "coin"))
    sender = item(response, transaction.from_address_hash, "coin")
    assert sender["balance_before"] == Decimal.to_string(Decimal.sub(@funds, StateChange.from_loss(earlier).value))

    insert(:address_coin_balance,
      address: transaction.to_address,
      block_number: transaction.block_number - 1,
      value: Wei.zero()
    )

    receiver = conn |> state_changes(transaction) |> item(transaction.to_address_hash, "coin")
    assert receiver["balance_before"] == "7"
    assert receiver["balance_after"] == Decimal.to_string(Decimal.add(transaction.value.value, 7))
  end

  test "recorded ERC-20 transfers recover missing token history on a later page", %{conn: conn} do
    {transaction, transfer} = recorded_token_transfer(@erc20_log)
    fund_native_participants(transaction)

    response = state_changes(conn, transaction)
    assert length(response["items"]) == 5
    tokens = Enum.filter(response["items"], &(&1["type"] == "token"))
    assert length(tokens) == 2
    Enum.each(tokens, &assert_unavailable/1)
    assert StateChanges.get(transaction.hash) == nil

    from_balance = put_token_balance(transaction, transfer, transfer.from_address_hash, nil, @funds)
    to_balance = put_token_balance(transaction, transfer, transfer.to_address_hash, nil, nil)
    response = state_changes(conn, transaction)
    sender = item(response, transfer.from_address_hash, "token")
    assert sender["balance_before"] == to_string(from_balance.value)
    assert sender["balance_after"] == Decimal.to_string(Decimal.sub(@funds, transfer.amount))
    assert_unavailable(item(response, transfer.to_address_hash, "token"))
    assert StateChanges.get(transaction.hash) == nil

    to_balance
    |> Ecto.Changeset.change(value: Decimal.new(0), value_fetched_at: DateTime.utc_now())
    |> Repo.update!()

    page = state_changes(conn, transaction, %{"items_count" => "3", "state_changes" => "null"})
    assert length(page["items"]) == 2
    receiver = item(page, transfer.to_address_hash, "token")
    assert receiver["balance_before"] == "0"
    assert receiver["balance_after"] == to_string(transfer.amount)
    assert receiver["change"] == to_string(transfer.amount)
    assert %StateChanges{state_changes: cached} = StateChanges.get(transaction.hash)
    assert length(cached) == 5
  end

  test "recorded ERC-1155 self-transfers distinguish pending history from confirmed zero", %{conn: conn} do
    {transaction, transfer} = recorded_token_transfer(@erc1155_log)
    fund_native_participants(transaction)
    [token_id] = transfer.token_ids

    response = state_changes(conn, transaction)
    token = item(response, transfer.to_address_hash, "token")
    assert_unavailable(token)
    assert token["token_id"] == to_string(token_id)
    assert token["change"] == "0"
    assert StateChanges.get(transaction.hash) == nil

    put_token_balance(transaction, transfer, transfer.to_address_hash, token_id, Decimal.new(0))
    page = state_changes(conn, transaction, %{"items_count" => "3", "state_changes" => "null"})
    assert [token] = page["items"]
    assert token["balance_before"] == "0"
    assert token["balance_after"] == "0"
    assert token["change"] == "0"
    assert %StateChanges{} = StateChanges.get(transaction.hash)
  end

  defp recorded_transaction(index \\ 0) do
    [recording] =
      @recordings
      |> Path.join("transaction_importer_binds_internal_transactions.json")
      |> File.read!()
      |> Jason.decode!()

    %{"method" => "eth_getTransactionByHash", "params" => [hash]} = Jason.decode!(recording["request"]["body"])
    %{"result" => recorded} = Jason.decode!(recording["response"]["body"])
    assert recorded["hash"] == hash
    params = recorded |> JSONRPCTransaction.to_elixir() |> JSONRPCTransaction.elixir_to_params()
    insert(:address, hash: params.from_address_hash)
    insert(:address, hash: params.to_address_hash)
    block = insert(:block, hash: params.block_hash, number: params.block_number)
    insert(:block, number: params.block_number - 1)

    params =
      Map.merge(params, %{
        index: index,
        block_consensus: true,
        block_timestamp: block.timestamp,
        status: :ok,
        gas_used: 21_000,
        cumulative_gas_used: 21_000
      })

    %Transaction{}
    |> Transaction.changeset(params)
    |> Repo.insert!()
    |> Repo.preload([:from_address, :to_address, :block])
  end

  defp recorded_token_transfer(log) do
    %{token_transfers: [params], tokens: [token_params]} = TokenTransfers.parse([log])

    token =
      insert(:token,
        contract_address: insert(:address, hash: token_params.contract_address_hash),
        type: token_params.type
      )

    from = insert(:address, hash: params.from_address_hash)

    to =
      if params.to_address_hash == params.from_address_hash,
        do: from,
        else: insert(:address, hash: params.to_address_hash)

    block = insert(:block, hash: log.block_hash, number: log.block_number)
    insert(:block, number: log.block_number - 1)

    transaction =
      insert(:transaction,
        hash: log.transaction_hash,
        block: block,
        block_number: block.number,
        block_consensus: true,
        block_timestamp: block.timestamp,
        from_address: from,
        to_address: token.contract_address,
        index: 0,
        gas_used: 21_000,
        cumulative_gas_used: 21_000,
        gas_price: %Wei{value: Decimal.new(1)},
        value: %Wei{value: Decimal.new(1)},
        status: :ok
      )

    transfer =
      %TokenTransfer{transaction_hash: transaction.hash}
      |> TokenTransfer.changeset(Map.put(params, :block_consensus, true))
      |> Repo.insert!()

    assert transfer.from_address_hash == from.hash
    assert transfer.to_address_hash == to.hash
    {transaction, transfer}
  end

  defp fund_native_participants(transaction, except \\ []) do
    for hash <- [transaction.from_address_hash, transaction.to_address_hash, transaction.block.miner_hash],
        hash not in except do
      insert(:address_coin_balance,
        address: Repo.get!(Address, hash),
        block_number: transaction.block_number - 1,
        value: %Wei{value: @funds}
      )
    end
  end

  defp put_token_balance(transaction, transfer, address_hash, token_id, value) do
    insert(:token_balance,
      address: Repo.get!(Address, address_hash),
      token_contract_address_hash: transfer.token_contract_address_hash,
      token_type: transfer.token_type,
      token_id: token_id,
      block_number: transaction.block_number - 1,
      value: value,
      value_fetched_at: if(is_nil(value), do: nil, else: DateTime.utc_now())
    )
  end

  defp state_changes(conn, transaction, params \\ %{}) do
    conn
    |> get("/api/v2/transactions/#{transaction.hash}/state-changes", params)
    |> json_response(200)
  end

  defp item(response, hash, type) do
    found = Enum.find(response["items"], &(&1["type"] == type and &1["address"]["hash"] == Address.checksum(hash)))
    assert found
    found
  end

  defp assert_unavailable(entry) do
    assert Map.has_key?(entry, "balance_before")
    assert Map.has_key?(entry, "balance_after")
    assert entry["balance_before"] == nil
    assert entry["balance_after"] == nil
    assert Map.has_key?(entry, "change")
    refute is_nil(entry["change"])
  end
end
