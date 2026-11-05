Code.require_file("../indexer/fetcher/token_balance/historical_test.exs", __DIR__)

defmodule Indexer.Repair2233566Test do
  use Explorer.DataCase, async: false

  alias Explorer.Chain.Address.TokenBalance
  alias Explorer.Utility.MissingBalanceOfToken
  alias Indexer.Fetcher.TokenBalance.{Historical, RecordedNode}

  @moduletag :capture_log

  setup do
    address = insert(:address)
    token = insert(:token)
    node = RecordedNode.start()

    %{address: address, token: token, node: node}
  end

  test "053: numeric revert code and recorded revert data survive the HTTP consumer", context do
    %{address: address, token: token, node: node} = context
    record = RecordedNode.transcript("revert")
    [response] = record["response"]
    error = response["error"]
    assert error["code"] === 3
    assert "0x08c379a0" <> _ = error["data"]

    arguments = RecordedNode.expect_response(node, "revert", address.hash, token.contract_address_hash, 1)
    [request] = record["request"]

    request = %{
      jsonrpc: request["jsonrpc"],
      id: request["id"],
      method: request["method"],
      params: [
        %{
          to: to_string(token.contract_address_hash),
          data: "0x70a08231" <> String.pad_leading(String.slice(to_string(address.hash), 2..-1//1), 64, "0")
        },
        "0x1"
      ]
    }

    assert {:ok, [%{id: 0, error: decoded_error} = decoded]} = EthereumJSONRPC.json_rpc([request], arguments)
    assert decoded_error == %{code: 3, message: error["message"], data: error["data"]}
    assert {0, {:error, ^decoded_error}} = EthereumJSONRPC.Encoder.decode_result(decoded, [], true)

    assert {0, {:error, message}} = EthereumJSONRPC.Encoder.decode_result(decoded, [])
    assert message == "(3) #{error["message"]} (#{error["data"]})"
    assert EthereumJSONRPC.contract_failure?({:error, message})
    assert_receive {:recorded_eth_call, "revert", _}
  end

  test "053: revert imports missing balanceOf and removes only eligible placeholders", context do
    %{address: address, token: token, node: node} = context
    old = placeholder(address, token, 0)
    at_revert = placeholder(address, token, 1)
    later = placeholder(address, token, 2)
    unrelated = insert(:token_balance, block_number: 0, value: nil, value_fetched_at: nil)

    fetched =
      insert(:token_balance,
        address: insert(:address),
        token_contract_address_hash: token.contract_address_hash,
        block_number: 0,
        value: 7,
        value_fetched_at: DateTime.utc_now()
      )

    later = Repo.get!(TokenBalance, later.id)
    fetched = Repo.get!(TokenBalance, fetched.id)
    unrelated = Repo.get!(TokenBalance, unrelated.id)

    RecordedNode.expect_response(node, "revert", address.hash, token.contract_address_hash, 1)
    assert Historical.run([entry(address, token, 1, 0)], nil) == :ok
    assert_receive {:recorded_eth_call, "revert", _}

    assert %{
             token_contract_address_hash: hash,
             block_number: 1,
             currently_implemented: nil
           } = Repo.one!(MissingBalanceOfToken)

    assert hash == token.contract_address_hash
    refute Repo.get(TokenBalance, old.id)
    refute Repo.get(TokenBalance, at_revert.id)
    assert Repo.get!(TokenBalance, later.id) == later
    assert Repo.get!(TokenBalance, fetched.id) == fetched
    assert Repo.get!(TokenBalance, unrelated.id) == unrelated

    assert Historical.run([entry(address, token, 1, 0)], nil) == :ok
    refute_receive {:recorded_eth_call, _, _}
    assert Repo.one!(MissingBalanceOfToken).block_number == 1

    window = Application.fetch_env!(:explorer, MissingBalanceOfToken)[:window_size]
    recovery_block = 1 + window + 1
    placeholder(address, token, recovery_block)
    RecordedNode.expect_response(node, "success", address.hash, token.contract_address_hash, recovery_block)

    assert Historical.run([entry(address, token, recovery_block, 0)], nil) == :ok
    assert_receive {:recorded_eth_call, "success", _}
    assert Repo.one!(MissingBalanceOfToken).currently_implemented

    recovered = Repo.get_by!(TokenBalance, address_hash: address.hash, block_number: recovery_block)
    assert recovered.value == Decimal.new(1_000_000_000_000_000_000_000_000)
    refute is_nil(recovered.value_fetched_at)
  end

  test "053: numeric node failures schedule retries and import a later successful balance", context do
    %{address: address, token: token, node: node} = context
    balance = placeholder(address, token, 1)
    RecordedNode.expect_response(node, "retry", address.hash, token.contract_address_hash, 1)
    before_fetch = DateTime.utc_now()

    assert Historical.run([entry(address, token, 1, 2)], nil) == :ok
    assert_receive {:recorded_eth_call, "retry", _}
    failed = Repo.get!(TokenBalance, balance.id)
    assert failed.retries_count == 3
    assert is_nil(failed.value)
    assert is_nil(failed.value_fetched_at)
    assert DateTime.compare(failed.refetch_after, before_fetch) == :gt
    assert Repo.all(MissingBalanceOfToken) == []
    assert Historical.init([], &[&1 | &2], nil) == []

    failed
    |> Ecto.Changeset.change(refetch_after: DateTime.add(before_fetch, -1, :second))
    |> Repo.update!()

    retry = entry(address, token, 1, 3)
    assert Historical.init([], &[&1 | &2], nil) == [retry]
    RecordedNode.expect_response(node, "success", address.hash, token.contract_address_hash, 1)
    assert Historical.run([retry], nil) == :ok
    assert_receive {:recorded_eth_call, "success", _}

    recovered = Repo.get!(TokenBalance, balance.id)
    assert recovered.value == Decimal.new(1_000_000_000_000_000_000_000_000)
    refute is_nil(recovered.value_fetched_at)
    assert Repo.all(MissingBalanceOfToken) == []
    assert Historical.init([], &[&1 | &2], nil) == []
  end

  defp placeholder(address, token, block_number) do
    insert(:token_balance,
      address: address,
      token_contract_address_hash: token.contract_address_hash,
      block_number: block_number,
      value: nil,
      value_fetched_at: nil,
      refetch_after: nil,
      retries_count: nil
    )
  end

  defp entry(address, token, block_number, retries_count) do
    {address.hash.bytes, token.contract_address_hash.bytes, block_number, "ERC-20", nil, retries_count}
  end
end
