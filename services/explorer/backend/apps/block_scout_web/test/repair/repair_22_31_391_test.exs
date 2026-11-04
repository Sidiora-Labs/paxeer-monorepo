defmodule BlockScoutWeb.Repair.Repair2231391Test do
  @moduledoc """
  Repair record 22.31.391 for audit 22.30.142:
  57: "57: address detail and owned lists round-trip the restricted key".
  58: "58: verified contract and audit reports round-trip the restricted key".
  59: "59: token detail, holders and NFT routes round-trip the restricted key".
  60: "60: transaction detail and owned lists authorize both addresses".
  61: "61: token-holder CSV exports the authorized recorded balance".
  Wiring request: domain 14 owns services/explorer/backend/apps/explorer/lib/explorer/access_helper.ex;
  normalize :key alongside legacy "key" there for other typed callers.
  Baseline verification was not run: the bounded gate order requires full
  implementation before hf-build and hf-done; no baseline failures are claimed.
  """

  use BlockScoutWeb.ConnCase, async: false

  alias BlockScoutWeb.API.V2.RestrictedAccessHelper
  alias EthereumJSONRPC.Transaction, as: JSONRPCTransaction
  alias Explorer.Chain.{Address, Transaction}
  alias Explorer.Chain.SmartContract.AuditReport
  alias Explorer.Repo

  @key "repair+key&22=31?391"
  @recordings Path.expand("../../../explorer/test/support/fixture/vcr_cassettes", __DIR__)
  @kitty_contract "0x06012c8cf97bead5deae237070f9587f8e7a266d"
  @kitty_owner "0x7b9ea9ac69b8fde875554321472c732eeff06ca0"

  setup do
    for key <- [:addresses_blacklist, :addresses_blacklist_key] do
      previous = Application.fetch_env(:explorer, key)

      on_exit(fn ->
        case previous do
          {:ok, value} -> Application.put_env(:explorer, key, value)
          :error -> Application.delete_env(:explorer, key)
        end
      end)
    end

    Application.put_env(:explorer, :addresses_blacklist_key, @key)
    :ok
  end

  test "57: address detail and owned lists round-trip the restricted key", %{conn: conn} do
    transaction = recorded_transaction()
    address_hash = transaction.from_address_hash
    restrict([address_hash])
    base = "/api/v2/addresses/#{address_hash}"

    detail = authorized_json(conn, base)
    assert detail["hash"] == Address.checksum(address_hash)

    transactions = authorized_json(conn, base <> "/transactions")
    assert [%{"hash" => hash}] = transactions["items"]
    assert hash == to_string(transaction.hash)

    assert [] == authorized_json(conn, base <> "/token-balances")

    for suffix <- [
          "/tokens",
          "/token-transfers",
          "/internal-transactions",
          "/logs",
          "/blocks-validated",
          "/coin-balance-history",
          "/withdrawals",
          "/nft",
          "/nft/collections"
        ] do
      assert %{"items" => [], "next_page_params" => nil} = authorized_json(conn, base <> suffix)
    end
  end

  test "58: verified contract and audit reports round-trip the restricted key", %{conn: conn} do
    contract = insert(:smart_contract, partially_verified: false)
    insert(:proxy_implementation, proxy_address_hash: contract.address_hash, address_hashes: [], names: [])
    restrict([contract.address_hash])

    report =
      Repo.insert!(%AuditReport{
        address_hash: contract.address_hash,
        is_approved: true,
        submitter_name: "Recorded project",
        submitter_email: "project@example.com",
        is_project_owner: true,
        project_name: contract.name,
        project_url: "https://example.com",
        audit_company_name: "Recorded audit",
        audit_report_url: "https://example.com/audit.pdf",
        audit_publish_date: ~D[2020-01-01]
      })

    base = "/api/v2/smart-contracts/#{contract.address_hash}"
    detail = authorized_json(conn, base)
    assert detail["name"] == contract.name
    assert detail["source_code"] == contract.contract_source_code
    assert detail["is_verified"] == true

    reports = authorized_json(conn, base <> "/audit-reports")
    assert [%{"audit_report_url" => url, "audit_company_name" => company}] = reports["items"]
    assert url == report.audit_report_url
    assert company == report.audit_company_name
  end

  test "59: token detail, holders and NFT routes round-trip the restricted key", %{conn: conn} do
    {token, owner, instance} = recorded_nft()
    restrict([token.contract_address_hash])
    base = "/api/v2/tokens/#{token.contract_address_hash}"

    detail = authorized_json(conn, base)
    assert detail["address_hash"] == Address.checksum(token.contract_address_hash)
    assert detail["type"] == "ERC-721"

    holders = authorized_json(conn, base <> "/holders")
    assert [%{"address" => %{"hash" => holder}, "value" => "1"}] = holders["items"]
    assert holder == Address.checksum(owner.hash)

    instances = authorized_json(conn, base <> "/instances")
    assert [%{"id" => "100500"}] = instances["items"]

    detail = authorized_json(conn, base <> "/instances/100500")
    assert detail["id"] == "100500"
    assert detail["metadata"] == instance.metadata
    assert detail["owner"]["hash"] == Address.checksum(owner.hash)

    for suffix <- ["/transfers", "/instances/100500/transfers", "/instances/100500/holders"] do
      response = authorized_json(conn, base <> suffix)
      assert %{"items" => [], "next_page_params" => nil} = response
    end

    # The holder filter performs a second access check after the token check.
    restrict([owner.hash])
    balances = authorized_json(conn, "/api/v2/addresses/#{owner.hash}/token-balances")
    assert [%{"value" => "1", "token" => %{"address_hash" => token_hash}}] = balances
    assert token_hash == Address.checksum(token.contract_address_hash)

    instances = authorized_json(conn, base <> "/instances", %{"holder_address_hash" => to_string(owner.hash)})
    assert [%{"id" => "100500"}] = instances["items"]
  end

  test "60: transaction detail and owned lists authorize both addresses", %{conn: conn} do
    transaction = recorded_transaction()
    base = "/api/v2/transactions/#{transaction.hash}"

    for restricted_hash <- [transaction.from_address_hash, transaction.to_address_hash] do
      restrict([restricted_hash])
      detail = authorized_json(conn, base)
      assert detail["hash"] == to_string(transaction.hash)
      assert detail["from"]["hash"] == Address.checksum(transaction.from_address_hash)
      assert detail["to"]["hash"] == Address.checksum(transaction.to_address_hash)
      assert detail["value"] == to_string(transaction.value.value)

      for suffix <- ["/token-transfers", "/internal-transactions", "/logs"] do
        assert %{"items" => [], "next_page_params" => nil} = authorized_json(conn, base <> suffix)
      end
    end
  end

  test "61: token-holder CSV exports the authorized recorded balance", %{conn: conn} do
    {token, owner, _instance} = recorded_nft()
    restrict([token.contract_address_hash])
    path = "/api/v2/tokens/#{token.contract_address_hash}/holders/csv"
    periods = %{"from_period" => "2020-01-01", "to_period" => "2020-01-02"}
    assert_forbidden_keys(conn, path, periods)

    response = get(conn, path, Map.put(periods, "key", @key))
    assert_cast_key(response, @key)
    assert response.status == 200
    assert get_resp_header(response, "content-type") == ["application/csv; charset=utf-8"]
    assert response.resp_body == "HolderAddress,Balance\r\n#{Address.checksum(owner.hash)},1\r\n"
  end

  test "legacy string keys retain the shared restriction policy" do
    transaction = recorded_transaction()
    hash = to_string(transaction.from_address_hash)
    restrict([transaction.from_address_hash])

    assert {:ok, false} = Explorer.AccessHelper.restricted_access?(hash, %{"key" => @key})
    assert {:ok, false} = RestrictedAccessHelper.restricted_access?(hash, %{"key" => @key})
    assert {:restricted_access, true} = RestrictedAccessHelper.restricted_access?(hash, %{"key" => "wrong"})
    assert {:restricted_access, true} = RestrictedAccessHelper.restricted_access?(hash, nil)
    assert {:restricted_access, true} = RestrictedAccessHelper.restricted_access?(hash, %{})
    assert {:ok, false} = RestrictedAccessHelper.restricted_access?(hash, %{key: @key})
    assert {:restricted_access, true} = RestrictedAccessHelper.restricted_access?(hash, %{"key" => @key, key: "wrong"})
    assert {:ok, false} = RestrictedAccessHelper.restricted_access?("", nil)
  end

  defp restrict(hashes) do
    Application.put_env(:explorer, :addresses_blacklist, Enum.map_join(hashes, ",", &to_string/1))
  end

  defp authorized_json(conn, path, params \\ %{}) do
    assert_forbidden_keys(conn, path, params)
    response = get(conn, path, Map.put(params, "key", @key))
    assert_cast_key(response, @key)
    json_response(response, 200)
  end

  defp assert_forbidden_keys(conn, path, params) do
    for key <- [nil, "wrong"] do
      query = if is_nil(key), do: params, else: Map.put(params, "key", key)
      response = get(conn, path, query)
      assert Map.get(response.params, :key) == key
      assert %{"message" => "Restricted access"} = json_response(response, 403)
    end
  end

  defp assert_cast_key(response, key) do
    assert response.params[:key] == key
    refute Map.has_key?(response.params, "key")
  end

  defp recorded_transaction do
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

    params =
      Map.merge(params, %{
        block_consensus: true,
        block_timestamp: block.timestamp,
        status: :ok,
        gas_used: 21_000,
        cumulative_gas_used: 21_000
      })

    %Transaction{} |> Transaction.changeset(params) |> Repo.insert!()
  end

  defp recorded_nft do
    # CryptoKitties metadata recorded in Indexer.Fetcher.TokenInstance.HelperTest.
    token =
      insert(:token,
        contract_address: insert(:address, hash: @kitty_contract),
        type: "ERC-721",
        name: "CryptoKitties",
        symbol: "CK",
        decimals: 0
      )

    owner = insert(:address, hash: @kitty_owner)

    instance =
      insert(:token_instance,
        token_contract_address_hash: token.contract_address_hash,
        token_id: 100_500,
        owner_address_hash: owner.hash,
        metadata: %{"id" => 100_500, "name" => "KittyBlue_2_Lemonade", "generation" => 20}
      )

    insert(:address_current_token_balance,
      address: owner,
      token_contract_address_hash: token.contract_address_hash,
      token_type: "ERC-721",
      value: 1
    )

    {token, owner, instance}
  end
end
