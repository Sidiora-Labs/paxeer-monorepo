defmodule BlockScoutWeb.Repair22_33_401Test do
  @moduledoc """
  Repair record 22.33.401 for audit 22.30.142:
  28: "finding 28: holder HTML and JSON reject unauthorized reads",
  "finding 28: authorized holder pagination preserves the restriction key".
  29: "finding 29: metadata HTML rejects unauthorized reads" and
  "authorized instance pages preserve recorded data and async request keys".
  30: "finding 30: transfer HTML and JSON reject unauthorized reads",
  "finding 30: authorized transfer pagination preserves the restriction key".
  The committed ERC-1155 batch log from Indexer.Transform.TokenTransfersTest
  is decoded by the production transformer and persisted through the real schemas.
  Pagination extends that fixture with fifty holders or log indices in the same
  block; metadata is a local instance fixture. Requests traverse WebRouter,
  production controllers, restriction policy, repository queries and views.
  Wiring request: domain 14 owns services/explorer/backend/apps/block_scout_web/lib/
  block_scout_web/templates/tokens/instance/overview/_tabs.html.eex; preserve the
  authorized restriction key in instance tab links (outside this task's scope).
  Baseline verification was not run: the explicit bounded gate order requires
  complete implementation before hf-build and hf-done; no baseline failures are claimed.
  First qualification stopped during fixture insertion: a collated transaction
  needs its block number, index and gas fields, now supplied by with_block/3.
  Gate evidence records the verified candidate revision and private log path.
  """

  use BlockScoutWeb.ConnCase, async: false

  alias Explorer.Chain.{Address, TokenTransfer}
  alias Explorer.Repo
  alias Indexer.Transform.TokenTransfers

  @endpoint BlockScoutWeb.Routers.WebRouter
  @key "repair+key&22=33?401"
  @recorded_log %{
    address_hash: "0x58Ab73CB79c8275628E0213742a85B163fE0A9Fb",
    block_number: 8_683_457,
    data:
      "0x000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000800000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000001388",
    first_topic: "0x4a39dc06d4c0dbc64b70af90fd698a233a518aa5d07e595d983b8c0526c8f7fb",
    second_topic: "0x0000000000000000000000006c943470780461b00783ad530a53913bd2c104d3",
    third_topic: "0x0000000000000000000000006c943470780461b00783ad530a53913bd2c104d3",
    fourth_topic: "0x0000000000000000000000006c943470780461b00783ad530a53913bd2c104d3",
    index: 2,
    transaction_hash: "0x6d2dd62c178e55a13b65601f227c4ffdd8aa4e3bcb1f24731363b4f7619e92c8",
    block_hash: "0x79594150677f083756a37eee7b97ed99ab071f502104332cb3835bac345711ca"
  }

  setup %{conn: conn} do
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
    fixture = recorded_instance()
    Application.put_env(:explorer, :addresses_blacklist, String.upcase(to_string(fixture.token.contract_address_hash)))

    conn =
      conn
      |> Plug.Test.init_test_session(%{})
      |> put_private(:phoenix_endpoint, BlockScoutWeb.Endpoint)
      |> Map.put(:secret_key_base, BlockScoutWeb.Endpoint.config(:secret_key_base))

    {:ok, conn: conn, fixture: fixture}
  end

  test "finding 28: holder HTML and JSON reject unauthorized reads", %{conn: conn, fixture: fixture} do
    assert_unauthorized(conn, fixture, "token-holders", [%{}, %{"type" => "JSON"}])
  end

  test "finding 29: metadata HTML rejects unauthorized reads", %{conn: conn, fixture: fixture} do
    assert_unauthorized(conn, fixture, "metadata", [%{}])
  end

  test "finding 30: transfer HTML and JSON reject unauthorized reads", %{conn: conn, fixture: fixture} do
    assert_unauthorized(conn, fixture, "token-transfers", [%{}, %{"type" => "JSON"}])
  end

  test "authorized instance pages preserve recorded data and async request keys", %{conn: conn, fixture: fixture} do
    for prefix <- ["/token", "/tokens"], tab <- ["token-holders", "metadata", "token-transfers"] do
      response = get(conn, instance_path(fixture, tab, prefix), %{"key" => @key})
      body = html_response(response, 200)
      assert response.assigns.token.contract_address_hash == fixture.token.contract_address_hash
      assert response.assigns.token_instance.instance.metadata == fixture.instance.metadata
      assert response.assigns.token_instance.token_id == fixture.instance.token_id
      assert response.assigns.total_token_transfers == 1

      if tab == "metadata" do
        assert body =~ "Recorded ERC-1155 instance"
      else
        assert [listing] =
                 body |> Floki.parse_document!() |> Floki.attribute("[data-async-listing]", "data-async-listing")

        assert URI.decode_query(URI.parse(listing).query)["key"] == @key

        assert %{"items" => [item], "next_page_path" => nil} =
                 conn |> get(listing, %{"type" => "JSON"}) |> json_response(200)

        assert item =~
                 if(tab == "token-holders",
                   do: Address.checksum(fixture.owner.hash),
                   else: @recorded_log.transaction_hash
                 )
      end
    end
  end

  test "finding 28: authorized holder pagination preserves the restriction key", %{conn: conn, fixture: fixture} do
    for value <- 1..50 do
      insert(:address_current_token_balance,
        token_contract_address_hash: fixture.token.contract_address_hash,
        token_type: "ERC-1155",
        token_id: fixture.instance.token_id,
        value: 5000 + value
      )
    end

    for prefix <- ["/token", "/tokens"] do
      path = instance_path(fixture, "token-holders", prefix)
      first = conn |> get(path, %{"type" => "JSON", "key" => @key}) |> json_response(200)
      assert length(first["items"]) == 50
      next_path = assert_page_key(first)

      assert %{"items" => [item], "next_page_path" => nil} =
               conn |> get(next_path, %{"type" => "JSON"}) |> json_response(200)

      assert item =~ Address.checksum(fixture.owner.hash)
      refute item in first["items"]
      assert_denied_page(conn, next_path)
    end
  end

  test "finding 30: authorized transfer pagination preserves the restriction key", %{conn: conn, fixture: fixture} do
    for index <- 3..52 do
      persist_log(%{@recorded_log | index: index})
    end

    for prefix <- ["/token", "/tokens"] do
      path = instance_path(fixture, "token-transfers", prefix)
      first = conn |> get(path, %{"type" => "JSON", "key" => @key}) |> json_response(200)
      assert length(first["items"]) == 50
      next_path = assert_page_key(first)

      assert %{"items" => [item], "next_page_path" => nil} =
               conn |> get(next_path, %{"type" => "JSON"}) |> json_response(200)

      assert item =~ ~s(data-identifier-hash="#{@recorded_log.transaction_hash}-2")
      refute item in first["items"]
      assert_denied_page(conn, next_path)
    end
  end

  test "unrestricted instance routes preserve HTML and JSON access", %{conn: conn, fixture: fixture} do
    Application.put_env(:explorer, :addresses_blacklist, "")

    for tab <- ["token-holders", "metadata", "token-transfers"] do
      assert conn |> get(instance_path(fixture, tab)) |> html_response(200)
    end

    for tab <- ["token-holders", "token-transfers"] do
      assert %{"items" => [_], "next_page_path" => nil} =
               conn |> get(instance_path(fixture, tab), %{"type" => "JSON"}) |> json_response(200)
    end
  end

  test "invalid addresses and fungible tokens retain not-found behavior", %{conn: conn, fixture: fixture} do
    fungible = insert(:token, type: "ERC-20")

    for tab <- ["token-holders", "metadata", "token-transfers"],
        hash <- ["invalid", to_string(fungible.contract_address_hash)] do
      path = "/token/#{hash}/instance/#{fixture.instance.token_id}/#{tab}"
      assert conn |> get(path, %{"key" => @key}) |> html_response(404)
    end
  end

  def record_query(_event, _measurements, metadata, owner) do
    if self() == owner && metadata.repo == Repo &&
         Enum.any?(["tokens", "token_instances", "address_current_token_balances", "token_transfers"], fn table ->
           String.contains?(metadata.query, ~s(FROM "#{table}"))
         end) do
      send(owner, :instance_data_query)
    end
  end

  defp assert_unauthorized(conn, fixture, tab, variants) do
    id = {__MODULE__, self()}
    prefix = Repo.config()[:telemetry_prefix] || [:explorer, :repo]
    :ok = :telemetry.attach(id, prefix ++ [:query], &__MODULE__.record_query/4, self())
    on_exit(fn -> :telemetry.detach(id) end)

    for prefix <- ["/token", "/tokens"], variant <- variants, key <- [nil, "wrong"] do
      params = if is_nil(key), do: variant, else: Map.put(variant, "key", key)
      response = get(conn, instance_path(fixture, tab, prefix), params)
      assert html_response(response, 404)
      refute Map.has_key?(response.assigns, :token_instance)
      refute response.resp_body =~ "Recorded ERC-1155 instance"
      refute_received :instance_data_query
    end
  end

  defp assert_page_key(%{"next_page_path" => path}) do
    assert is_binary(path)
    query = URI.decode_query(URI.parse(path).query)
    assert query["key"] == @key
    refute Map.has_key?(query, "type")
    path
  end

  defp assert_denied_page(conn, path) do
    uri = URI.parse(path)
    params = uri.query |> URI.decode_query() |> Map.delete("key") |> Map.put("type", "JSON")
    assert conn |> get(uri.path, params) |> html_response(404)
    assert conn |> get(uri.path, Map.put(params, "key", "wrong")) |> html_response(404)
  end

  defp instance_path(fixture, tab, prefix \\ "/token") do
    "#{prefix}/#{Address.checksum(fixture.token.contract_address_hash)}/instance/#{fixture.instance.token_id}/#{tab}"
  end

  defp recorded_instance do
    %{tokens: [token_params], token_transfers: [transfer]} = TokenTransfers.parse([@recorded_log], true)

    token =
      insert(:token,
        contract_address: insert(:address, hash: token_params.contract_address_hash),
        type: token_params.type,
        decimals: 0
      )

    owner = insert(:address, hash: transfer.to_address_hash)
    block = insert(:block, hash: @recorded_log.block_hash, number: @recorded_log.block_number, consensus: true)

    insert(:transaction, hash: @recorded_log.transaction_hash, from_address: owner, to_address: owner)
    |> with_block(block, status: :ok)

    persisted = persist_log(@recorded_log)
    token_id = hd(persisted.token_ids)

    instance =
      insert(:token_instance,
        token_contract_address_hash: token.contract_address_hash,
        token_id: token_id,
        owner_address_hash: owner.hash,
        metadata: %{"name" => "Recorded ERC-1155 instance", "description" => "Batch transfer at block 8683457"}
      )

    insert(:address_current_token_balance,
      address: owner,
      token_contract_address_hash: token.contract_address_hash,
      token_type: "ERC-1155",
      token_id: token_id,
      block_number: block.number,
      value: hd(transfer.amounts)
    )

    %{token: token, owner: owner, instance: instance}
  end

  defp persist_log(log) do
    %{token_transfers: [params]} = TokenTransfers.parse([log], true)
    %TokenTransfer{} |> TokenTransfer.changeset(Map.put(params, :block_consensus, true)) |> Repo.insert!()
  end
end
