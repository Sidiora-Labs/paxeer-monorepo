defmodule Explorer.Repair2233454Test do
  @moduledoc """
  Task 22.33.454 repair record:
  created-contract-metadata: "created-contract-metadata enriches a creation through both HTTP interfaces".
  scam-badge-batch-recovery: the batch, concurrent writer and bounded retry cases below.
  token-pagination-order: "token-pagination-order traverses every boundary with a complete cursor".
  Wiring request (domain 14): apps/block_scout_web/lib/block_scout_web/chain.ex must emit and
  parse the contract address alongside name, type and insertion time using Address.Token.paging_key/1.
  The existing three-field caller remains a legacy interface until that separately scoped change.
  Baseline verification was not run: the gate order prohibits verification before implementation.
  The bounded gate ledger records the verified revision; no pre-change failure is claimed.
  """

  use Explorer.DataCase

  alias Explorer.Chain.{Address, InternalTransaction}
  alias Explorer.Chain.Address.{ScamBadgeToAddress, Token}
  alias Explorer.Chain.Hash.Address, as: AddressHash
  alias Explorer.MicroserviceInterfaces.{BENS, Metadata}
  alias Explorer.PagingOptions

  @creator "0x1111111111111111111111111111111111111111"
  @created "0x2222222222222222222222222222222222222222"
  @timestamp ~U[2026-01-01 00:00:00.000000Z]

  setup do
    keys = [
      {:explorer, :http_client},
      {:tesla, :adapter},
      {:explorer, BENS},
      {:explorer, Metadata},
      {:ethereum_jsonrpc, EthereumJSONRPC.Geth}
    ]

    original = Enum.map(keys, fn {app, key} -> {app, key, Application.fetch_env(app, key)} end)
    Application.put_env(:explorer, :http_client, Explorer.HttpClient.Tesla)
    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)
    Application.put_env(:ethereum_jsonrpc, EthereumJSONRPC.Geth, tracer: "call_tracer", debug_trace_timeout: "5s")

    on_exit(fn ->
      Enum.each(original, fn
        {app, key, {:ok, value}} -> Application.put_env(app, key, value)
        {app, key, :error} -> Application.delete_env(app, key)
      end)
    end)

    :ok
  end

  test "created-contract-metadata enriches a creation through both HTTP interfaces" do
    creator = insert(:address, hash: @creator)
    created = insert(:address, hash: @created)
    server = Bypass.open()
    url = "http://localhost:#{server.port}"

    transcript =
      __DIR__
      |> Path.join("../../../ethereum_jsonrpc/test/support/fixture/paxeer_x/call_tracer_block.json")
      |> File.read!()
      |> Jason.decode!()

    Bypass.expect_once(server, "POST", "/", fn conn ->
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      assert [%{"method" => "debug_traceBlockByNumber", "params" => ["0x2A", options]}] = Jason.decode!(body)
      assert options == %{"tracer" => "callTracer", "timeout" => "5s"}
      Plug.Conn.resp(conn, 200, Jason.encode!([transcript]))
    end)

    assert {:ok, [creation, _call]} =
             EthereumJSONRPC.Geth.fetch_block_internal_transactions([42],
               transport: EthereumJSONRPC.HTTP,
               transport_options: [http: EthereumJSONRPC.HTTP.Tesla, http_options: [], urls: [url]],
               variant: EthereumJSONRPC.PaxeerX
             )

    changeset = InternalTransaction.blockless_changeset(%InternalTransaction{}, creation)

    assert changeset.valid?
    internal = Ecto.Changeset.apply_changes(changeset)
    assert internal.to_address_hash == nil
    checksum = Address.checksum(created.hash)
    creator_checksum = Address.checksum(creator.hash)
    Application.put_env(:explorer, BENS, enabled: true, service_url: url, protocols: [])
    Application.put_env(:explorer, Metadata, enabled: true, service_url: url)
    chain_id = Application.get_env(:block_scout_web, :chain_id)

    Bypass.expect(server, "POST", "/api/v1/#{chain_id}/addresses:batch_resolve_names", fn conn ->
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      assert %{"addresses" => addresses} = Jason.decode!(body)
      assert Enum.sort(addresses) == Enum.sort([@creator, @created])

      Plug.Conn.resp(conn, 200, Jason.encode!(%{names: %{checksum => "created.pax"}}))
    end)

    tag = %{"name" => "Created contract", "meta" => ~s({"source":"chain"})}

    Bypass.expect(server, "GET", "/api/v1/metadata", fn conn ->
      conn = Plug.Conn.fetch_query_params(conn)
      assert Enum.sort(String.split(conn.query_params["addresses"], ",")) == Enum.sort([@creator, @created])
      assert conn.query_params["chain_id"] == to_string(chain_id)
      assert conn.query_params["tags_limit"] == "5"

      Plug.Conn.resp(conn, 200, Jason.encode!(%{addresses: %{checksum => %{tags: [tag]}}}))
    end)

    for association <- [created, nil, internal.created_contract_address] do
      item = %{internal | created_contract_address: association, from_address: creator}
      [enriched] = [item, item] |> BENS.maybe_preload_ens() |> Metadata.maybe_preload_metadata() |> Enum.uniq()
      assert enriched.created_contract_address.ens_domain_name == "created.pax"

      assert enriched.created_contract_address.metadata == %{
               "tags" => [%{"name" => "Created contract", "meta" => %{"source" => "chain"}}]
             }

      assert enriched.to_address_hash == nil
      assert enriched.from_address.hash == creator.hash
      assert enriched.from_address.metadata == nil
      refute checksum == creator_checksum
    end
  end

  test "scam-badge-batch-recovery creates multiple missing addresses and survives writer restart" do
    existing = insert(:address)
    hashes = [address_hash(1), address_hash(2), existing.hash]
    writer = Task.async(fn -> ScamBadgeToAddress.add(Enum.map(hashes, &to_string/1)) end)
    monitor = Process.monitor(writer.pid)
    assert {3, rows} = Task.await(writer)
    assert_receive {:DOWN, ^monitor, :process, _, :normal}
    assert Enum.sort(Enum.map(rows, & &1.address_hash)) == Enum.sort(hashes)
    assert Enum.all?(hashes, &Repo.get(Address, &1))

    reader = Task.async(fn -> ScamBadgeToAddress.get([]) end)
    assert Enum.sort(Enum.map(Task.await(reader), & &1.address_hash)) == Enum.sort(hashes)
    assert {0, []} = ScamBadgeToAddress.add(Enum.map(hashes, &to_string/1))
    assert {0, []} = ScamBadgeToAddress.add([])
  end

  test "scam-badge-batch-recovery retries when a concurrent creator wins every address" do
    hashes = [address_hash(3), address_hash(4)]
    observe_queries(:race, hashes)

    assert {2, rows} = ScamBadgeToAddress.add(Enum.map(hashes, &to_string/1))
    assert Enum.sort(Enum.map(rows, & &1.address_hash)) == hashes
    assert_receive {:concurrent_addresses, {2, created}}
    assert length(created) == 2
    assert_receive {:address_insert, {:ok, %{num_rows: 0, rows: []}}}
    assert_receive {:badge_insert, {:error, %Postgrex.Error{postgres: %{code: :foreign_key_violation}}}}
    assert_receive {:badge_insert, {:ok, %{num_rows: 2}}}
    assert Enum.sort(Enum.map(ScamBadgeToAddress.get([]), & &1.address_hash)) == hashes
  end

  test "scam-badge-batch-recovery bounds a persistent foreign key failure to one retry" do
    hashes = [address_hash(5), address_hash(6)]
    observe_queries(:remove, hashes)

    assert_raise Postgrex.Error, fn -> ScamBadgeToAddress.add(Enum.map(hashes, &to_string/1)) end

    for _ <- 1..2 do
      assert_receive {:badge_insert, {:error, %Postgrex.Error{postgres: %{code: :foreign_key_violation}}}}
    end

    assert_receive {:address_insert, {:ok, %{num_rows: 2}}}
    refute_receive {:badge_insert, _}
    refute_receive {:address_insert, _}
    assert ScamBadgeToAddress.get([]) == []
  end

  test "token-pagination-order traverses every boundary with a complete cursor" do
    owner = insert(:address)
    older = DateTime.add(@timestamp, -1, :second)

    records = [
      {11, "zeta", "ERC-721", @timestamp},
      {12, "alpha", "ERC-20", @timestamp},
      {13, "Alpha", "ERC-20", @timestamp},
      {14, "ALPHA", "ERC-20", older},
      {15, "Beta", "ERC-20", @timestamp},
      {16, "beta", "ERC-20", @timestamp},
      {17, nil, "ERC-20", @timestamp},
      {18, nil, "ERC-20", @timestamp},
      {19, nil, "ERC-20", older},
      {20, "alpha", "ERC-1155", @timestamp},
      {21, nil, "ERC-1155", @timestamp}
    ]

    for {id, name, type, timestamp} <- Enum.reverse(records) do
      contract = insert(:address, hash: address_hash(id))
      token = insert(:token, contract_address: contract, name: name, type: type, inserted_at: timestamp)

      insert(:address_current_token_balance,
        address: owner,
        token_contract_address_hash: token.contract_address_hash,
        token_type: type,
        value: 10,
        block_number: 100
      )
    end

    expected = Enum.map(records, fn {id, _, _, _} -> address_hash(id) end)

    for page_size <- 1..5 do
      tokens = collect_pages(owner.hash, page_size, nil, length(records) + 1)
      assert Enum.map(tokens, & &1.contract_address_hash) == expected
      assert length(Enum.uniq_by(tokens, & &1.contract_address_hash)) == length(records)
      assert Enum.all?(tokens, &Decimal.equal?(&1.balance, 10))
    end

    all = owner.hash |> Token.list_address_tokens_with_balance() |> Repo.all()
    assert Enum.map(all, & &1.contract_address_hash) == expected

    for {token, index} <- Enum.with_index(all) do
      key = Token.paging_key(token)
      assert key == {token.name, token.type, token.inserted_at, token.contract_address_hash}

      rest =
        owner.hash
        |> Token.list_address_tokens_with_balance(paging_options: %PagingOptions{page_size: 50, key: key})
        |> Repo.all()

      assert Enum.map(rest, & &1.contract_address_hash) == Enum.drop(expected, index + 1)
    end
  end

  defp collect_pages(_owner, _size, _key, 0), do: flunk("pagination did not terminate")

  defp collect_pages(owner, size, key, remaining) do
    page =
      owner
      |> Token.list_address_tokens_with_balance(paging_options: %PagingOptions{page_size: size, key: key})
      |> Repo.all()

    case page do
      [] -> []
      _ -> page ++ collect_pages(owner, size, Token.paging_key(List.last(page)), remaining - 1)
    end
  end

  defp address_hash(id) do
    {:ok, hash} = AddressHash.cast("0x2233454" <> String.pad_leading(Integer.to_string(id, 16), 33, "0"))
    hash
  end

  defp observe_queries(action, hashes) do
    handler = {__MODULE__, make_ref()}
    prefix = Repo.config()[:telemetry_prefix] || [:explorer, :repo]
    config = %{owner: self(), action: action, hashes: hashes, handler: handler}
    :ok = :telemetry.attach(handler, prefix ++ [:query], &__MODULE__.observe_query/4, config)
    on_exit(fn -> :telemetry.detach(handler) end)
  end

  def observe_query(_event, _measurements, metadata, %{owner: owner} = config) do
    if self() == owner and String.starts_with?(metadata.query, "INSERT") do
      case metadata.source do
        "scam_address_badge_mappings" ->
          send(owner, {:badge_insert, metadata.result})

          if config.action == :race and match?({:error, _}, metadata.result) do
            result =
              Task.async(fn ->
                Address.create_multiple(address_params(config.hashes))
              end)
              |> Task.await()

            send(owner, {:concurrent_addresses, result})
          end

        "addresses" ->
          send(owner, {:address_insert, metadata.result})

          if config.action == :remove do
            Repo.delete_all(from(address in Address, where: address.hash in ^config.hashes))
          end

        _ ->
          :ok
      end
    end
  end

  defp address_params(hashes) do
    Enum.map(hashes, &%{hash: &1, inserted_at: @timestamp, updated_at: @timestamp})
  end
end
