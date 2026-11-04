defmodule Explorer.Repair.Repair2233469Test do
  use Explorer.DataCase

  import Phoenix.ConnTest

  alias EthereumJSONRPC.Block, as: RPCBlock
  alias Explorer.Chain.{Import, TokenTransfer}
  alias Explorer.Chain.Import.Runner.TokenTransfers

  @endpoint BlockScoutWeb.Endpoint
  @blocks Path.expand("../../../ethereum_jsonrpc/test/support/fixture/paxeer_x/consecutive_blocks.json", __DIR__)

  setup_all do
    {:ok, _} = Application.ensure_all_started(:block_scout_web)
    :ok
  end

  setup do
    recorded =
      @blocks
      |> File.read!()
      |> Jason.decode!()
      |> Map.values()
      |> Enum.map(&RPCBlock.to_elixir/1)
      |> Enum.max_by(& &1["number"])

    block =
      insert(:block,
        number: recorded["number"],
        hash: recorded["hash"],
        timestamp: recorded["timestamp"]
      )

    %{block: block}
  end

  test "filter-zero-log-batch-page: a first-log batch survives three successive API pages", %{block: block} do
    token = insert(:token, type: "ERC-1155", decimals: 0)
    transaction = transaction(block)
    ids = Enum.to_list(1..123)
    transfer(transaction, token, 0, ids, Enum.map(ids, fn _ -> 3 end))

    params = %{"transaction_types" => "ERC-1155"}
    first = request(params)
    assert length(first["items"]) == 50
    assert first["next_page_params"]["token_transfer_index"] == 0
    assert first["next_page_params"]["token_transfer_batch_index"] == 74

    second = request(Map.merge(params, first["next_page_params"]))
    assert length(second["items"]) == 50
    assert second["next_page_params"]["token_transfer_index"] == 0
    assert second["next_page_params"]["token_transfer_batch_index"] == 24

    third = request(Map.merge(params, second["next_page_params"]))
    assert length(third["items"]) == 23
    assert third["next_page_params"] == nil

    items = first["items"] ++ second["items"] ++ third["items"]
    assert Enum.map(items, & &1["total"]["token_id"]) == Enum.map(ids, &to_string/1)
    assert Enum.map(items, & &1["token_transfer_batch_index"]) == Enum.to_list(123..1//-1)
    assert Enum.all?(items, &(&1["hash"] == to_string(transaction.hash) and &1["token_transfer_index"] == 0))
    assert_unique(items)
  end

  test "filter-or-duplicates: both mixed OR directions page transactions exactly once", %{block: block} do
    sender = insert(:address)
    recipient = insert(:address)
    included = insert_list(2, :address)
    excluded = insert(:address)

    transactions =
      for _ <- 1..53 do
        transaction(block, from_address: sender, to_address: recipient)
      end

    for direction <- [:from, :to] do
      params = mixed_or_params(direction, included, excluded, "COIN_TRANSFER")
      first = request(params)
      assert length(first["items"]) == 50
      assert is_map(first["next_page_params"])
      second = request(Map.merge(params, first["next_page_params"]))
      assert length(second["items"]) == 3
      assert second["next_page_params"] == nil

      items = first["items"] ++ second["items"]
      assert Enum.map(items, & &1["hash"]) == Enum.map(Enum.reverse(transactions), &to_string(&1.hash))
      assert_unique(items)
    end
  end

  test "filter-or-duplicates: both mixed OR directions preserve distinct ERC-1155 batch elements", %{block: block} do
    token = insert(:token, type: "ERC-1155", decimals: 0)
    transaction = transaction(block)
    included = insert_list(2, :address)
    excluded = insert(:address)
    ids = List.duplicate(333, 57)
    transfer(transaction, token, 1, ids, List.duplicate(3, 57))

    for direction <- [:from, :to] do
      params = mixed_or_params(direction, included, excluded, "ERC-1155")
      first = request(params)
      assert length(first["items"]) == 50
      second = request(Map.merge(params, first["next_page_params"]))
      assert length(second["items"]) == 7
      assert second["next_page_params"] == nil

      items = first["items"] ++ second["items"]
      assert Enum.map(items, & &1["total"]["token_id"]) == Enum.map(ids, &to_string/1)
      assert Enum.map(items, & &1["token_transfer_batch_index"]) == Enum.to_list(57..1//-1)
      assert Enum.all?(items, &(&1["hash"] == to_string(transaction.hash) and &1["token_transfer_index"] == 1))
      assert_unique(items)
    end
  end

  test "filter-or-duplicates: each disjunct independently admits its matching transaction and transfer", %{block: block} do
    token = insert(:token, type: "ERC-1155", decimals: 0)
    included = insert_list(2, :address)
    excluded = insert(:address)

    for direction <- [:from, :to] do
      {include_field, exclude_field} =
        if direction == :from, do: {:from_address, :to_address}, else: {:to_address, :from_address}

      included_only = transaction(block, [{include_field, hd(included)}, {exclude_field, excluded}])
      excluded_by_both = transaction(block, [{exclude_field, excluded}])
      exclusion_only = transaction(block)
      both = transaction(block, [{include_field, List.last(included)}])

      for tx <- [included_only, excluded_by_both, exclusion_only, both] do
        transfer(tx, token, tx.index, [333], [3])
      end

      for type <- ["COIN_TRANSFER", "ERC-1155"] do
        params =
          mixed_or_params(direction, included, excluded, type)
          |> Map.put("amount_from", "2.0")
          |> Map.put("amount_to", "10")

        response = request(params)
        hashes = MapSet.new(response["items"], & &1["hash"])
        assert MapSet.member?(hashes, to_string(included_only.hash))
        assert MapSet.member?(hashes, to_string(exclusion_only.hash))
        assert MapSet.member?(hashes, to_string(both.hash))
        refute MapSet.member?(hashes, to_string(excluded_by_both.hash))
        assert_unique(response["items"])
      end
    end
  end

  test "filter-decimal-term-order: mixed scales apply both bounds before and after unnesting", %{block: block} do
    token = insert(:token, type: "ERC-1155", decimals: 2)
    scalar_token = insert(:token, type: "ERC-20", decimals: 2)
    tx = transaction(block)

    for {amount, index} <- Enum.with_index([100, 200, 300, 1000, 1200]) do
      transfer(tx, scalar_token, index, nil, amount)
    end

    transfer(tx, token, 5, [11, 22, 33, 44, 55], [100, 200, 300, 1000, 1200])

    for {lower, upper} <- [{"2.0", "10"}, {"2", "10.0"}, {"2.00", "1e1"}, {"2e0", "10.00"}] do
      scalar = request(%{"transaction_types" => "ERC-20", "amount_from" => lower, "amount_to" => upper})
      assert Enum.map(scalar["items"], & &1["total"]["value"]) == ["1000", "300", "200"]
      assert Enum.map(scalar["items"], & &1["token_transfer_index"]) == [3, 2, 1]

      batch = request(%{"transaction_types" => "ERC-1155", "amount_from" => lower, "amount_to" => upper})
      assert Enum.map(batch["items"], & &1["total"]["token_id"]) == ["22", "33", "44"]
      assert Enum.map(batch["items"], & &1["total"]["value"]) == ["200", "300", "1000"]
      assert Enum.map(batch["items"], & &1["token_transfer_batch_index"]) == [4, 3, 2]
    end

    for type <- ["ERC-20", "ERC-1155"],
        {lower, upper} <- [{"3.0", "3"}, {"3", "3.00"}, {"10.0", "2"}, {"10", "2.0"}] do
      response = request(%{"transaction_types" => type, "amount_from" => lower, "amount_to" => upper})
      assert response["items"] == []
      assert response["next_page_params"] == nil
    end
  end

  test "filter-empty-inclusion: fully overlapping address lists match no API rows", %{block: block} do
    token = insert(:token, type: "ERC-1155", decimals: 0)
    tx = transaction(block)
    transfer(tx, token, 0, [333, 555], [3, 5])
    other = transaction(block)
    transfer(other, token, 1, [777], [7])

    assert length(request(%{})["items"]) == 5

    for {direction, hash} <- [{"from", tx.from_address_hash}, {"to", tx.to_address_hash}] do
      overlapping = %{
        "#{direction}_address_hashes_to_include" => to_string(hash),
        "#{direction}_address_hashes_to_exclude" => to_string(hash)
      }

      response = request(overlapping)
      assert response["items"] == []
      assert response["next_page_params"] == nil

      opposite = if direction == "from", do: "to", else: "from"

      for suffix <- ["include", "exclude"] do
        params =
          overlapping
          |> Map.put("#{opposite}_address_hashes_to_#{suffix}", to_string(other.to_address_hash))
          |> Map.put("address_relation", "and")

        assert request(params)["items"] == []
      end
    end

    both_empty = %{
      "from_address_hashes_to_include" => to_string(tx.from_address_hash),
      "from_address_hashes_to_exclude" => to_string(tx.from_address_hash),
      "to_address_hashes_to_include" => to_string(tx.to_address_hash),
      "to_address_hashes_to_exclude" => to_string(tx.to_address_hash)
    }

    for relation <- ["or", "and"] do
      assert request(Map.put(both_empty, "address_relation", relation))["items"] == []
    end
  end

  test "filter-empty-inclusion: overlapping token lists remain empty alongside unrelated transfers and native rows", %{
    block: block
  } do
    token = insert(:token, type: "ERC-1155", decimals: 0)
    unrelated_token = insert(:token, type: "ERC-20", decimals: 0)
    tx = transaction(block)
    transfer(tx, token, 0, [333, 555], [3, 5])
    transfer(tx, unrelated_token, 1, nil, 7)

    assert length(request(%{})["items"]) == 4

    for include <- [to_string(token.contract_address_hash), "native,#{token.contract_address_hash}"] do
      response =
        request(%{
          "token_contract_address_hashes_to_include" => include,
          "token_contract_address_hashes_to_exclude" => include
        })

      assert response["items"] == []
      assert response["next_page_params"] == nil
    end

    native_only =
      request(%{
        "token_contract_address_hashes_to_include" => "native,#{token.contract_address_hash}",
        "token_contract_address_hashes_to_exclude" => to_string(token.contract_address_hash)
      })

    assert [%{"hash" => hash, "token" => nil}] = native_only["items"]
    assert hash == to_string(tx.hash)
  end

  test "filter-empty-inclusion: an empty address disjunct retains the other side of an OR", %{block: block} do
    token = insert(:token, type: "ERC-1155", decimals: 0)
    tx = transaction(block)
    transfer(tx, token, 0, [333], [3])

    for {direction, hash, opposite, other_hash} <- [
          {"from", tx.from_address_hash, "to", tx.to_address_hash},
          {"to", tx.to_address_hash, "from", tx.from_address_hash}
        ],
        suffix <- ["include", "exclude"] do
      other = if suffix == "include", do: other_hash, else: insert(:address).hash

      response =
        request(%{
          "#{direction}_address_hashes_to_include" => to_string(hash),
          "#{direction}_address_hashes_to_exclude" => to_string(hash),
          "#{opposite}_address_hashes_to_#{suffix}" => to_string(other),
          "address_relation" => "or"
        })

      assert length(response["items"]) == 2
      assert Enum.all?(response["items"], &(&1["hash"] == to_string(tx.hash)))
      assert_unique(response["items"])
    end
  end

  defp transaction(block, attrs \\ []) do
    :transaction
    |> insert(Keyword.merge([input: "0x", value: 3_000_000_000_000_000_000], attrs))
    |> with_block(block, status: :ok)
  end

  defp transfer(tx, token, index, ids, amounts) do
    params = %{
      block_number: tx.block_number,
      block_hash: tx.block_hash,
      transaction_hash: tx.hash,
      log_index: index,
      from_address_hash: tx.from_address_hash,
      to_address_hash: tx.to_address_hash,
      token_contract_address_hash: token.contract_address_hash,
      token_type: token.type,
      token_ids: ids,
      block_consensus: true
    }

    params = if is_list(amounts), do: Map.put(params, :amounts, amounts), else: Map.put(params, :amount, amounts)
    {:ok, multi} = Import.all_single_multi([TokenTransfers], %{token_transfers: %{params: [params]}})
    assert {:ok, %{token_transfers: [%TokenTransfer{} = imported]}} = Repo.transaction(multi)
    assert imported.log_index == index
    assert imported.transaction_hash == tx.hash
    imported
  end

  defp mixed_or_params(direction, included, excluded, type) do
    {include, exclude} = if direction == :from, do: {"from", "to"}, else: {"to", "from"}

    %{
      "transaction_types" => type,
      "#{include}_address_hashes_to_include" => Enum.map_join(included, ",", &to_string(&1.hash)),
      "#{exclude}_address_hashes_to_exclude" => to_string(excluded.hash),
      "address_relation" => "or"
    }
  end

  defp request(params) do
    build_conn() |> get("/api/v2/advanced-filters", params) |> json_response(200)
  end

  defp assert_unique(items) do
    identities =
      Enum.map(items, fn item ->
        {item["hash"], item["internal_transaction_index"], item["token_transfer_index"],
         item["token_transfer_batch_index"]}
      end)

    assert length(identities) == MapSet.size(MapSet.new(identities))
  end
end
