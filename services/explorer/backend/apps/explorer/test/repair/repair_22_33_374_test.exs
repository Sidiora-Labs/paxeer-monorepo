defmodule Explorer.Repair.Repair2233374Test do
  @moduledoc """
  Task 22.33.374, audit record 22.30.140.

  proxy-call-error-cache is covered by the provider-failure, RPC-reconnection,
  and revert-classification tests through the production resolvers, HTTP
  transport, implementation schema, PostgreSQL, and ABI consumer. The chain
  addresses, bytecode and storage come from the committed zkatana genesis.
  The loopback node serves those snapshots with JSON-RPC error transcripts.

  contract-verification-uid is covered by simultaneous schema submissions,
  duplicate worker admission, and current/legacy polling validation. A duplicate
  attempt raises before the existing worker can publish or change its status.

  Baseline verification was not run: the ordered gate instructions prohibit
  verification before implementation and require hf-build before hf-done.
  No pre-change test failures are claimed.

  Wiring request: domain 14, services/explorer/backend/apps/explorer/lib/explorer/smart_contract/solidity/publisher_worker.ex: match insert_status/3 success before publishing and propagate other admission errors; duplicate UIDs already fail closed at the schema boundary.
  Wiring request: domain 14, services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex: keep GUIDs opaque and handle queue admission failure before reporting an accepted submission.

  Gotcha: verification UIDs have a 64-character database limit; random IDs must
  fit that existing column without an out-of-scope migration.
  """

  use Explorer.DataCase, async: false

  alias Explorer.Chain
  alias Explorer.Chain.SmartContract.{Proxy, VerificationStatus}
  alias Explorer.Chain.SmartContract.Proxy.Models.Implementation
  alias Explorer.SmartContract.Solidity.PublisherWorker

  @zero_word "0x" <> String.duplicate("0", 64)
  @revert %{"code" => 3, "message" => "execution reverted", "data" => "0x"}
  @implementation_slot "0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc"
  @storage_slots [
    @implementation_slot,
    "0xc5f16f0fcc639fa48a6947836d9850f504798523bf8c9a3a87d5876cf622bcf7",
    "0xa3f0ad74e5423aebfd80d3ef4346578335a9a72aeaee59ff6cb3582b35133d50",
    "0x7050c9e0f4ca769c69bd3a8ef740bc37934f8e2c036e5a723fd8ee048ed3f8c3"
  ]
  @implementation_abi [
    %{
      "type" => "function",
      "name" => "get",
      "inputs" => [],
      "outputs" => [%{"name" => "", "type" => "uint256"}],
      "stateMutability" => "view"
    }
  ]

  setup do
    proxy_config = Application.fetch_env!(:explorer, :proxy)
    replace_env(:explorer, :proxy, Keyword.put(proxy_config, :caching_implementation_data_enabled, false))
    :ok
  end

  test "proxy-call-error-cache provider failures preserve implementation rows and ABI" do
    records = recorded_genesis()
    contract = seed_contract(records["PolygonZkEVMDeployer"], [])
    implementation = seed_contract(records["PolygonZkEVMBridge implementation"], @implementation_abi)
    node = rpc_node(records["PolygonZkEVMDeployer"])

    cached =
      Implementation.save_implementation_data(
        %{
          proxy_address_hash: contract.address_hash,
          address_hashes: [implementation.address_hash],
          proxy_type: :basic_implementation
        },
        []
      )

    assert %Implementation{} = cached

    for error <- [
          %{"code" => -32601, "message" => "Method not found"},
          %{"code" => -32000, "message" => "header not found"},
          %{"code" => -32005, "message" => "request limit exceeded"},
          %{"code" => -32603, "message" => "Internal error"},
          %{"code" => 3, "message" => "provider unavailable"}
        ] do
      Agent.update(node.state, &Map.put(&1, :call_error, error))
      assert Proxy.fetch_implementation_address_hash(contract.address_hash, []) == :error
      assert refresh(contract) == cached
      assert Implementation.get_proxy_implementations(contract.address_hash) == cached
      assert Proxy.get_implementation_abi_from_proxy(contract, []) == @implementation_abi
    end

    assert Agent.get(node.state, & &1.requests) > 0
  end

  test "proxy-call-error-cache actual RPC connection loss and recovery retain cached data" do
    records = recorded_genesis()
    snapshot = records["PolygonZkEVMGlobalExitRootL2 proxy"]
    contract = seed_contract(snapshot, [])
    implementation = seed_contract(records["PolygonZkEVMGlobalExitRootL2 implementation"], @implementation_abi)
    node = rpc_node(snapshot)

    assert %Implementation{} = cached = refresh(contract)
    assert cached.address_hashes == [implementation.address_hash]
    assert cached.names == [implementation.name]
    assert Proxy.get_implementation_abi_from_proxy(contract, []) == @implementation_abi

    :ok = Bypass.down(node.server)
    assert Proxy.fetch_value({:call, "0x5c60da1b"}, contract.address_hash) == :error
    assert Proxy.fetch_implementation_address_hash(contract.address_hash, []) == :error
    assert refresh(contract) == cached
    assert Implementation.get_proxy_implementations(contract.address_hash) == cached
    assert Proxy.get_implementation_abi_from_proxy(contract, []) == @implementation_abi

    :ok = Bypass.up(node.server)
    assert refresh(contract).address_hashes == [implementation.address_hash]
    assert Proxy.get_implementation_abi_from_proxy(contract, []) == @implementation_abi

    # Only a successful snapshot with reverted getters authorizes clearing data.
    Agent.update(node.state, &Map.put(&1, :storage, %{}))
    assert %Implementation{address_hashes: [], names: []} = refresh(contract)
    assert Implementation.get_proxy_implementations(contract.address_hash).address_hashes == []
    assert Proxy.get_implementation_abi_from_proxy(contract, []) == []
  end

  test "proxy-call-error-cache only explicit execution reverts are absent getters" do
    snapshot = recorded_genesis()["PolygonZkEVMDeployer"]
    contract = seed_contract(snapshot, [])
    node = rpc_node(snapshot)
    call = {:call, "0x5c60da1b"}

    for error <- [
          @revert,
          %{"code" => -32000, "message" => "execution reverted: unsupported getter", "data" => "0x"},
          %{"code" => -32015, "message" => "execution reverted"},
          %{"code" => -32000, "message" => "VM Exception while processing transaction: revert"}
        ] do
      Agent.update(node.state, &Map.put(&1, :call_error, error))
      assert Proxy.fetch_value(call, contract.address_hash) == {:ok, nil}
      assert Proxy.fetch_values([call], contract.address_hash) == {:ok, %{call => nil}}
    end

    Agent.update(node.state, &Map.put(&1, :call_error, %{"code" => -32601, "message" => "Method not found"}))
    assert Proxy.fetch_value(call, contract.address_hash) == :error
    assert Proxy.fetch_values([call], contract.address_hash) == :error

    Agent.update(node.state, &Map.put(&1, :storage_error, @revert))
    assert Proxy.fetch_value({:storage, @implementation_slot}, contract.address_hash) == :error
    assert Proxy.fetch_values([{:storage, @implementation_slot}], contract.address_hash) == :error
  end

  test "contract-verification-uid simultaneous schema submissions keep separate status transitions" do
    address = recorded_genesis()["PolygonZkEVMBridge proxy"]["address"]
    parent = self()

    submit = fn ->
      Task.async(fn ->
        send(parent, {:ready, self()})

        receive do
          :submit -> :ok
        end

        uid = VerificationStatus.generate_uid(address)
        {:ok, row} = VerificationStatus.insert_status(uid, :pending, address)
        send(parent, {:pending, self(), uid})

        receive do
          {:finish, status} ->
            {:ok, updated} = VerificationStatus.update_status(uid, status)
            {row, updated}
        end
      end)
    end

    first = submit.()
    second = submit.()
    assert_receive {:ready, first_pid}, 5_000
    assert_receive {:ready, second_pid}, 5_000
    send(first_pid, :submit)
    send(second_pid, :submit)
    assert_receive {:pending, ^first_pid, first_uid}, 5_000
    assert_receive {:pending, ^second_pid, second_uid}, 5_000
    assert first_uid != second_uid
    assert VerificationStatus.fetch_status(first_uid) == :pending
    assert VerificationStatus.fetch_status(second_uid) == :pending

    send(first_pid, {:finish, :pass})
    {first_row, first_updated} = Task.await(if(first.pid == first_pid, do: first, else: second))
    assert first_row.uid == first_updated.uid
    assert VerificationStatus.fetch_status(first_uid) == :pass
    assert VerificationStatus.fetch_status(second_uid) == :pending

    send(second_pid, {:finish, :fail})
    {second_row, second_updated} = Task.await(if(second.pid == second_pid, do: second, else: first))
    assert second_row.uid == second_updated.uid
    assert VerificationStatus.fetch_status(first_uid) == :pass
    assert VerificationStatus.fetch_status(second_uid) == :fail
    assert Repo.aggregate(VerificationStatus, :count) == 2
  end

  test "contract-verification-uid duplicate worker admission cannot change an existing result" do
    address = recorded_genesis()["PolygonZkEVMBridge proxy"]["address"]
    uid = VerificationStatus.generate_uid(address)
    assert {:ok, row} = VerificationStatus.insert_status(uid, :pass, address)

    assert_raise Ecto.InvalidChangesetError, fn ->
      PublisherWorker.perform({"flattened_api", %{"address_hash" => address}, %{}, uid})
    end

    assert Repo.get!(VerificationStatus, uid) == row
    assert VerificationStatus.fetch_status(uid) == :pass
    assert Repo.aggregate(VerificationStatus, :count) == 1
  end

  test "contract-verification-uid random IDs fit storage and legacy pending IDs remain pollable" do
    address = recorded_genesis()["PolygonZkEVMBridge proxy"]["address"]
    ids = for _ <- 1..256, do: VerificationStatus.generate_uid(address)
    assert length(Enum.uniq(ids)) == 256

    for uid <- ids do
      assert byte_size(uid) == 64
      assert {:ok, <<_::256>>} = Base.decode16(uid, case: :lower)
      assert VerificationStatus.validate_uid(uid) == {:ok, uid}
    end

    {:ok, hash} = Chain.string_to_address_hash(address)
    timestamp = DateTime.to_unix(DateTime.utc_now())
    legacy = Base.encode16(hash.bytes, case: :lower) <> Integer.to_string(timestamp, 16)
    assert {:ok, _} = VerificationStatus.insert_status(legacy, :pending, address)
    assert VerificationStatus.fetch_status(legacy) == :pending
    assert {:ok, _} = VerificationStatus.update_status(legacy, :pass)
    assert VerificationStatus.fetch_status(legacy) == :pass

    for invalid <- [
          nil,
          "",
          String.duplicate("z", 64),
          String.duplicate("0", 63),
          String.duplicate("0", 65),
          String.duplicate("z", 40) <> Integer.to_string(timestamp, 16),
          Base.encode16(hash.bytes, case: :lower) <> Integer.to_string(timestamp + 3600, 16)
        ] do
      assert VerificationStatus.validate_uid(invalid) == :error
      assert VerificationStatus.fetch_status(invalid) == :unknown_uid
    end

    assert VerificationStatus.generate_uid("invalid address") == nil
  end

  defp recorded_genesis do
    __DIR__
    |> Path.join("../support/fixture/chain_spec/zkatana_genesis.json")
    |> File.read!()
    |> Jason.decode!()
    |> Map.fetch!("genesis")
    |> Map.new(&{&1["contractName"], &1})
  end

  defp seed_contract(snapshot, abi) do
    address = insert(:contract_address, hash: snapshot["address"], contract_code: snapshot["bytecode"])

    insert(:smart_contract,
      address_hash: address.hash,
      abi: abi,
      name: snapshot["contractName"],
      contract_code_md5: Explorer.SmartContract.Helper.md5(address.contract_code.bytes)
    )
  end

  defp refresh(contract) do
    Implementation.get_implementation(
      %{
        updated: contract,
        implementation_updated_at: nil,
        implementation_address_fetched?: false,
        refetch_necessity_checked?: false
      },
      timeout: 5_000
    )
  end

  defp rpc_node(snapshot) do
    server = Bypass.open()

    state =
      start_supervised!(
        {Agent,
         fn ->
           %{storage: Map.get(snapshot, "storage", %{}), call_error: @revert, storage_error: nil, requests: 0}
         end}
      )

    {:ok, hash} = Chain.string_to_address_hash(snapshot["address"])
    address = to_string(hash)

    Bypass.expect(server, "POST", "/", fn conn ->
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      request = Jason.decode!(body)
      snapshot = Agent.get_and_update(state, &{&1, Map.update!(&1, :requests, fn n -> n + 1 end)})
      respond = &rpc_response(&1, address, snapshot)
      response = if is_list(request), do: Enum.map(request, respond), else: respond.(request)

      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.resp(200, Jason.encode!(response))
    end)

    url = "http://127.0.0.1:#{server.port}"

    replace_env(:explorer, :json_rpc_named_arguments,
      variant: EthereumJSONRPC.Geth,
      transport: EthereumJSONRPC.HTTP,
      transport_options: [
        http: EthereumJSONRPC.HTTP.HTTPoison,
        urls: [url],
        fallback_urls: [url],
        http_options: [timeout: 500, recv_timeout: 500]
      ]
    )

    %{server: server, state: state}
  end

  defp rpc_response(%{"jsonrpc" => "2.0", "id" => id, "method" => method, "params" => params}, address, state) do
    response =
      case {method, params} do
        {"eth_blockNumber", []} ->
          %{"result" => "0x0"}

        {"eth_getStorageAt", [^address, slot, "latest"]} when slot in @storage_slots ->
          if state.storage_error do
            %{"error" => state.storage_error}
          else
            %{"result" => Map.get(state.storage, slot, @zero_word)}
          end

        {"eth_call", [%{"to" => ^address, "data" => selector}, "latest"]}
        when selector in ["0x52ef6b2c", "0x5c60da1b", "0xaaf10f42", "0xbb82aa5e"] ->
          %{"error" => state.call_error}
      end

    Map.merge(%{"jsonrpc" => "2.0", "id" => id}, response)
  end

  defp replace_env(app, key, value) do
    previous = Application.fetch_env(app, key)
    Application.put_env(app, key, value)

    on_exit(fn ->
      case previous do
        {:ok, old} -> Application.put_env(app, key, old)
        :error -> Application.delete_env(app, key)
      end
    end)
  end
end
