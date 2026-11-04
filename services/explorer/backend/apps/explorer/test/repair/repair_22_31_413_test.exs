defmodule Explorer.Repair.Repair2231413Test do
  @moduledoc """
  Task 22.31.413 repair record, audit finding 22.30.160/selfdestruct-cancun.
  The retained-code, constructor, reverted-call, legacy-fork, replay, backfill,
  historical-replay and unavailable-state tests below prove this finding through
  recorded Geth JSON-RPC, real HTTP transport, production trace decoding, schema
  casting, the import Multi and PostgreSQL.

  The embedded transcripts were recorded with Geth 1.14.13-eb00f169 (Cancun from
  genesis) and 1.13.15-c5ba367e (Shanghai), using --dev --history.transactions 0.
  The deployed runtime is CALLER SELFDESTRUCT (33ff); the reverting caller calls
  that runtime and then reverts. Responses, including receipt indexing errors,
  are replayed verbatim apart from the JSON-RPC correlation id. No node binary,
  download, environment variable or external endpoint is needed by this gate.

  Prior attempt: seven of eight tests failed before reaching import because
  eth_getTransactionReceipt returned -32000 transaction indexing is in progress.
  No additional baseline gate was run: the ordered instructions require complete
  implementation before the bounded build and done gates. Geth 1.13's Cancun
  dev sealer also reports Invalid parameters; the Cancun recording uses 1.14.

  Wiring request: domain 14, services/explorer/backend/apps/explorer/lib/explorer/migrator/refetch_contract_codes.ex: schedule batches of previously indexed SELFDESTRUCT addresses through InternalTransactions.reconcile_selfdestructed_contracts_bytecode/3 inside Repo.transaction/1.
  """

  use Explorer.DataCase

  alias Ecto.Multi
  alias Explorer.Chain.{Address, Block, Data, Hash, Import, InternalTransaction, PendingBlockOperation}
  alias Explorer.Chain.Import.Runner.InternalTransactions

  @runtime "33ff"
  @unknown_block "0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
  @moduletag timeout: 120_000

  setup context do
    node = fixture_node(Map.get(context, :fork, :cancun))
    replace_env(:explorer, :json_rpc_named_arguments, node.arguments)
    replace_env(:indexer, :json_rpc_named_arguments, node.arguments)

    config = Application.fetch_env!(:ethereum_jsonrpc, EthereumJSONRPC.Geth)

    replace_env(
      :ethereum_jsonrpc,
      EthereumJSONRPC.Geth,
      Keyword.merge(config, tracer: "call_tracer", block_traceable?: true, debug_trace_timeout: "5s")
    )

    {:ok, node: node}
  end

  test "selfdestruct-cancun retains a pre-existing contract's committed bytecode", %{node: node} do
    contract = deploy(node, @runtime)
    address = seed_code(contract, "0x" <> @runtime)
    receipt = send_transaction(node, %{to: contract})
    assert receipt["status"] == "0x1"
    changes = prepare_import(node, receipt)
    assert Enum.any?(changes, &(&1.type == :selfdestruct))
    assert code_at(node, contract, receipt["blockHash"]) == bytecode("0x" <> @runtime)

    assert {:ok, result} = import_traces(changes)
    assert result.empty_selfdestructed_contracts_bytecode == 0
    assert Repo.get!(Address, address.hash).contract_code == code_at(node, contract, receipt["blockHash"])
    assert Repo.exists?(from(trace in InternalTransaction, where: trace.type == :selfdestruct))
  end

  test "selfdestruct-cancun clears a contract created and destroyed in the same transaction", %{node: node} do
    receipt = send_transaction(node, %{data: "0x" <> @runtime})
    contract = receipt["contractAddress"]
    address = seed_code(contract, "0x" <> @runtime)
    changes = prepare_import(node, receipt)
    assert Enum.any?(changes, &(&1.type == :create and &1.created_contract_address_hash == address.hash))
    assert Enum.any?(changes, &(&1.type == :selfdestruct and &1.from_address_hash == address.hash))
    assert code_at(node, contract, receipt["blockHash"]) == bytecode("0x")

    assert {:ok, result} = import_traces(changes)
    assert result.empty_selfdestructed_contracts_bytecode == 1
    assert Repo.get!(Address, address.hash).contract_code == bytecode("0x")
  end

  @tag fork: :shanghai
  test "selfdestruct-cancun follows the pre-Cancun deletion rule for pre-existing contracts", %{node: node} do
    contract = deploy(node, @runtime)
    address = seed_code(contract, "0x" <> @runtime)
    receipt = send_transaction(node, %{to: contract})
    changes = prepare_import(node, receipt)
    assert code_at(node, contract, receipt["blockHash"]) == bytecode("0x")

    assert {:ok, result} = import_traces(changes)
    assert result.empty_selfdestructed_contracts_bytecode == 1
    assert Repo.get!(Address, address.hash).contract_code == bytecode("0x")
  end

  @tag fork: :shanghai
  test "selfdestruct-cancun preserves code when an ancestor reverts the destruction", %{node: node} do
    contract = deploy(node, @runtime)
    address = seed_code(contract, "0x" <> @runtime)
    caller = deploy(node, reverting_caller(contract))
    receipt = send_transaction(node, %{to: caller})
    assert receipt["status"] == "0x0"
    changes = prepare_import(node, receipt)
    assert Enum.any?(changes, &(&1.type == :selfdestruct and &1.from_address_hash == address.hash))
    assert Enum.any?(changes, &(not is_nil(Map.get(&1, :error))))
    assert code_at(node, contract, receipt["blockHash"]) == bytecode("0x" <> @runtime)

    assert {:ok, _} = import_traces(changes)
    assert Repo.get!(Address, address.hash).contract_code == bytecode("0x" <> @runtime)
  end

  test "selfdestruct-cancun replay restores misclassified code and is idempotent", %{node: node} do
    contract = deploy(node, @runtime)
    address = seed_code(contract, "0x")
    receipt = send_transaction(node, %{to: contract})
    changes = prepare_import(node, receipt)

    assert {:ok, result} = import_traces(changes)
    assert result.empty_selfdestructed_contracts_bytecode == 1
    restored = Repo.get!(Address, address.hash)
    assert restored.contract_code == code_at(node, contract, receipt["blockHash"])
    assert restored.contract_code == bytecode("0x" <> @runtime)
    trace_count = Repo.aggregate(InternalTransaction, :count)

    mark_pending(receipt)
    assert {:ok, result} = import_traces(changes)
    assert result.empty_selfdestructed_contracts_bytecode == 0
    assert Repo.get!(Address, address.hash).updated_at == restored.updated_at
    assert Repo.aggregate(InternalTransaction, :count) == trace_count
  end

  test "selfdestruct-cancun backfill restores nil code without retracing", %{node: node} do
    contract = deploy(node, @runtime)
    address = seed_code(contract, nil)
    receipt = send_transaction(node, %{to: contract})
    persist_block(node, receipt["blockHash"])

    assert {:ok, {:ok, 1}} =
             Repo.transaction(fn ->
               InternalTransactions.reconcile_selfdestructed_contracts_bytecode(
                 Repo,
                 [address.hash],
                 Import.timestamps()
               )
             end)

    assert Repo.get!(Address, address.hash).contract_code == code_at(node, contract, receipt["blockHash"])
    assert Repo.get!(Address, address.hash).contract_code == bytecode("0x" <> @runtime)
  end

  @tag fork: :shanghai
  test "selfdestruct-cancun historical replay uses the newest imported committed state", %{node: node} do
    contract = deploy(node, @runtime)
    address = seed_code(contract, "0x" <> @runtime)
    caller = deploy(node, reverting_caller(contract))
    reverted = send_transaction(node, %{to: caller})
    changes = prepare_import(node, reverted)
    assert code_at(node, contract, reverted["blockHash"]) == bytecode("0x" <> @runtime)

    destroyed = send_transaction(node, %{to: contract})
    persist_block(node, destroyed["blockHash"])
    assert code_at(node, contract, destroyed["blockHash"]) == bytecode("0x")

    assert {:ok, _} = import_traces(changes)
    assert Repo.get!(Address, address.hash).contract_code == bytecode("0x")
  end

  test "selfdestruct-cancun unavailable canonical state rolls back the entire import", %{node: node} do
    contract = deploy(node, @runtime)
    address = seed_code(contract, "0x" <> @runtime)
    receipt = send_transaction(node, %{to: contract})
    changes = prepare_import(node, receipt)
    insert(:block, hash: full_hash(@unknown_block), number: quantity(receipt["blockNumber"]) + 1)

    assert {:error, :empty_selfdestructed_contracts_bytecode, {:contract_code_fetch_failed, response}, _} =
             import_traces(changes)

    assert %{error: %{code: -32_000}} = response
    assert Repo.get!(Address, address.hash).contract_code == bytecode("0x" <> @runtime)
    refute Repo.exists?(InternalTransaction)

    assert Repo.exists?(
             from(pending in PendingBlockOperation, where: pending.block_hash == ^full_hash(receipt["blockHash"]))
           )
  end

  defp replace_env(app, key, value) do
    previous = Application.fetch_env(app, key)
    Application.put_env(app, key, value)

    on_exit(fn ->
      case previous do
        {:ok, config} -> Application.put_env(app, key, config)
        :error -> Application.delete_env(app, key)
      end
    end)
  end

  defp fixture_node(fork) do
    responses =
      recorded_transcripts()
      |> Map.fetch!(Atom.to_string(fork))
      |> Enum.group_by(fn %{"request" => request} -> {request["method"], request["params"]} end)

    counts = start_supervised!({Agent, fn -> %{} end})
    server = Bypass.open()

    Bypass.stub(server, "POST", "/", fn conn ->
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      request = Jason.decode!(body)

      respond = fn %{"jsonrpc" => "2.0", "id" => id, "method" => method, "params" => params} ->
        key = {method, params}
        assert Map.has_key?(responses, key), "Unrecorded RPC request: #{inspect(key)}"
        exchanges = Map.fetch!(responses, key)

        index =
          Agent.get_and_update(counts, fn counts ->
            {Map.get(counts, key, 0), Map.update(counts, key, 1, &(&1 + 1))}
          end)

        exchanges
        |> Enum.at(min(index, length(exchanges) - 1))
        |> Map.fetch!("response")
        |> Map.put("id", id)
      end

      response = if is_list(request), do: Enum.map(request, respond), else: respond.(request)

      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.resp(200, Jason.encode!(response))
    end)

    url = "http://127.0.0.1:#{server.port}"

    %{
      url: url,
      arguments: [
        variant: EthereumJSONRPC.Geth,
        transport: EthereumJSONRPC.HTTP,
        transport_options: [
          http: EthereumJSONRPC.HTTP.HTTPoison,
          urls: [url],
          fallback_urls: [],
          http_options: [timeout: 20_000, recv_timeout: 20_000]
        ]
      ]
    }
  end

  defp rpc_response!(node, method, params) do
    request = EthereumJSONRPC.request(%{id: 0, method: method, params: params})

    assert {:ok, %{status_code: 200, body: body}} =
             HTTPoison.post(node.url, Jason.encode!(request), [{"content-type", "application/json"}],
               timeout: 20_000,
               recv_timeout: 20_000
             )

    response = Jason.decode!(body)
    assert %{"jsonrpc" => "2.0", "id" => 0} = response
    response
  end

  defp rpc!(node, method, params) do
    assert %{"result" => result} = rpc_response!(node, method, params)
    result
  end

  defp send_transaction(node, attributes) do
    [from | _] = rpc!(node, "eth_accounts", [])
    transaction = Map.merge(%{from: from, gas: "0xf4240", gasPrice: "0x77359400", value: "0x0"}, attributes)
    hash = rpc!(node, "eth_sendTransaction", [transaction])
    await_receipt(node, hash, 100)
  end

  defp await_receipt(node, hash, attempts) when attempts > 0 do
    case rpc_response!(node, "eth_getTransactionReceipt", [hash]) do
      %{"result" => nil} ->
        Process.sleep(100)
        await_receipt(node, hash, attempts - 1)

      %{"error" => %{"code" => -32_000, "message" => "transaction indexing is in progress"}} ->
        Process.sleep(100)
        await_receipt(node, hash, attempts - 1)

      %{"result" => %{"transactionHash" => ^hash} = receipt} ->
        receipt
    end
  end

  defp await_receipt(_node, hash, 0), do: flunk("Transaction was not mined: #{hash}")

  defp deploy(node, runtime) do
    size = runtime |> Base.decode16!(case: :mixed) |> byte_size() |> Integer.to_string(16) |> String.pad_leading(2, "0")
    receipt = send_transaction(node, %{data: "0x60#{size}600c60003960#{size}6000f3#{runtime}"})
    assert receipt["status"] == "0x1"
    receipt["contractAddress"]
  end

  defp reverting_caller("0x" <> target), do: "6000600060006000600073#{target}5af15060006000fd"

  defp prepare_import(node, receipt) do
    transaction = rpc!(node, "eth_getTransactionByHash", [receipt["transactionHash"]])

    raw_trace =
      rpc!(node, "debug_traceTransaction", [receipt["transactionHash"], %{tracer: "callTracer", timeout: "5s"}])

    assert is_map(raw_trace)
    block = persist_block(node, receipt["blockHash"])

    insert(:transaction,
      hash: full_hash(transaction["hash"]),
      from_address: ensure_address(transaction["from"]),
      to_address: ensure_address(transaction["to"])
    )
    |> with_block(block,
      index: quantity(transaction["transactionIndex"]),
      cumulative_gas_used: quantity(receipt["cumulativeGasUsed"]),
      gas_used: quantity(receipt["gasUsed"]),
      status: if(receipt["status"] == "0x1", do: :ok, else: :error)
    )

    assert {:ok, params} =
             EthereumJSONRPC.fetch_internal_transactions(
               [
                 %{
                   block_number: block.number,
                   hash_data: transaction["hash"],
                   transaction_index: quantity(transaction["transactionIndex"])
                 }
               ],
               node.arguments
             )

    changes =
      Enum.map(params, fn params ->
        Enum.each([:from_address_hash, :to_address_hash, :created_contract_address_hash], fn key ->
          if params[key], do: ensure_address(params[key])
        end)

        changeset = InternalTransaction.blockless_changeset(%InternalTransaction{}, params)
        assert changeset.valid?, inspect(changeset.errors)
        changeset.changes
      end)

    assert Enum.any?(changes, &(&1.type == :selfdestruct))
    mark_pending(receipt)
    changes
  end

  defp persist_block(node, hash) do
    block = rpc!(node, "eth_getBlockByHash", [hash, false])

    Repo.get(Block, full_hash(hash)) ||
      insert(:block,
        hash: full_hash(hash),
        number: quantity(block["number"]),
        parent_hash: full_hash(block["parentHash"]),
        timestamp: DateTime.from_unix!(quantity(block["timestamp"]))
      )
  end

  defp mark_pending(receipt) do
    insert(:pending_block_operation,
      block_hash: full_hash(receipt["blockHash"]),
      block_number: quantity(receipt["blockNumber"])
    )
  end

  defp import_traces(changes) do
    Multi.new()
    |> InternalTransactions.run(changes, %{timestamps: Import.timestamps()})
    |> Repo.transaction()
  end

  defp code_at(node, contract, hash),
    do: bytecode(rpc!(node, "eth_getCode", [contract, %{blockHash: hash, requireCanonical: true}]))

  defp ensure_address(nil), do: nil

  defp ensure_address(hash) do
    {:ok, hash} = Hash.Address.cast(hash)
    Repo.get(Address, hash) || insert(:address, hash: hash)
  end

  defp seed_code(hash, code) do
    hash
    |> ensure_address()
    |> Ecto.Changeset.change(contract_code: if(is_nil(code), do: nil, else: bytecode(code)))
    |> Repo.update!()
  end

  defp full_hash(hash) do
    {:ok, hash} = Hash.Full.cast(hash)
    hash
  end

  defp bytecode(code) do
    {:ok, data} = Data.cast(code)
    data
  end

  defp quantity(value), do: EthereumJSONRPC.quantity_to_integer(value)

  defp recorded_transcripts do
    Jason.decode!(~S'''
    {
      "cancun": [
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_accounts",
            "params": []
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": [
              "0x757b6af7c6eb736f655302d054e2366e2df1241c"
            ]
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_sendTransaction",
            "params": [
              {
                "from": "0x757b6af7c6eb736f655302d054e2366e2df1241c",
                "gas": "0xf4240",
                "gasPrice": "0x77359400",
                "value": "0x0",
                "data": "0x6002600c60003960026000f333ff"
              }
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": "0x1b42b66d3bdf28b9c46e4bb01d8e10f4631452cde474780d87dc24a1e64ed389"
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getTransactionReceipt",
            "params": [
              "0x1b42b66d3bdf28b9c46e4bb01d8e10f4631452cde474780d87dc24a1e64ed389"
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "error": {
              "code": -32000,
              "message": "transaction indexing is in progress",
              "data": "transaction indexing is in progress"
            }
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getTransactionReceipt",
            "params": [
              "0x1b42b66d3bdf28b9c46e4bb01d8e10f4631452cde474780d87dc24a1e64ed389"
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": {
              "blockHash": "0xc33af3223c1d19691f4bd92845f60633cf3dcd4fe4a96fad5d996964ac678ede",
              "blockNumber": "0x1",
              "contractAddress": "0x19e924d7f7aa69fcef53fa2d42f6cc1fc301ce3f",
              "cumulativeGasUsed": "0xd17a",
              "effectiveGasPrice": "0x77359400",
              "from": "0x757b6af7c6eb736f655302d054e2366e2df1241c",
              "gasUsed": "0xd17a",
              "logs": [],
              "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
              "status": "0x1",
              "to": null,
              "transactionHash": "0x1b42b66d3bdf28b9c46e4bb01d8e10f4631452cde474780d87dc24a1e64ed389",
              "transactionIndex": "0x0",
              "type": "0x0"
            }
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getTransactionByHash",
            "params": [
              "0x1b42b66d3bdf28b9c46e4bb01d8e10f4631452cde474780d87dc24a1e64ed389"
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": {
              "blockHash": "0xc33af3223c1d19691f4bd92845f60633cf3dcd4fe4a96fad5d996964ac678ede",
              "blockNumber": "0x1",
              "from": "0x757b6af7c6eb736f655302d054e2366e2df1241c",
              "gas": "0xf4240",
              "gasPrice": "0x77359400",
              "hash": "0x1b42b66d3bdf28b9c46e4bb01d8e10f4631452cde474780d87dc24a1e64ed389",
              "input": "0x6002600c60003960026000f333ff",
              "nonce": "0x0",
              "to": null,
              "transactionIndex": "0x0",
              "value": "0x0",
              "type": "0x0",
              "chainId": "0x539",
              "v": "0xa96",
              "r": "0x444b48636436636559187fcf62922272d8b6df7c581172b10f7e3603cdd3a230",
              "s": "0x3234b6a38cd19a09af6b35f1f07135b094f2e26310ed03338db85fac1f4a7ec4"
            }
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "debug_traceTransaction",
            "params": [
              "0x1b42b66d3bdf28b9c46e4bb01d8e10f4631452cde474780d87dc24a1e64ed389",
              {
                "tracer": "callTracer",
                "timeout": "5s"
              }
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": {
              "from": "0x757b6af7c6eb736f655302d054e2366e2df1241c",
              "gas": "0xf4240",
              "gasUsed": "0xd17a",
              "to": "0x19e924d7f7aa69fcef53fa2d42f6cc1fc301ce3f",
              "input": "0x6002600c60003960026000f333ff",
              "output": "0x33ff",
              "value": "0x0",
              "type": "CREATE"
            }
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getBlockByHash",
            "params": [
              "0xc33af3223c1d19691f4bd92845f60633cf3dcd4fe4a96fad5d996964ac678ede",
              false
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": {
              "baseFeePerGas": "0x342770c0",
              "blobGasUsed": "0x0",
              "difficulty": "0x0",
              "excessBlobGas": "0x0",
              "extraData": "0xd883010e0d846765746888676f312e32332e33856c696e7578",
              "gasLimit": "0xafa5bd",
              "gasUsed": "0xd17a",
              "hash": "0xc33af3223c1d19691f4bd92845f60633cf3dcd4fe4a96fad5d996964ac678ede",
              "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
              "miner": "0x0000000000000000000000000000000000000000",
              "mixHash": "0x0523ac8062129fbe150422246203a98f84c4bb870d32eee926ddc2df420ca035",
              "nonce": "0x0000000000000000",
              "number": "0x1",
              "parentBeaconBlockRoot": "0x0000000000000000000000000000000000000000000000000000000000000000",
              "parentHash": "0x4878d483dd7080149942c9611120bf7b8a05e1dcaf256e26a51cc2d791f576f7",
              "receiptsRoot": "0x129b0163402aa35d7f2b43d8cb0a7b965ae0bd60a7a8308d33cff52f4bec278e",
              "requestsRoot": "0x6036c41849da9c076ed79654d434017387a88fb833c2856b32e18218b3341c5f",
              "sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
              "size": "0x2e5",
              "stateRoot": "0x528fbe51ab43b40c341949cd2ca281919830debfa1cb9db1570ea8d010bd8380",
              "timestamp": "0x6aeaa33b",
              "transactions": [
                "0x1b42b66d3bdf28b9c46e4bb01d8e10f4631452cde474780d87dc24a1e64ed389"
              ],
              "transactionsRoot": "0xee986984058c33ad3483d66e59e16c2298fed13590770a862ed45128c335976c",
              "uncles": [],
              "withdrawals": [],
              "withdrawalsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421"
            }
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_sendTransaction",
            "params": [
              {
                "from": "0x757b6af7c6eb736f655302d054e2366e2df1241c",
                "gas": "0xf4240",
                "gasPrice": "0x77359400",
                "value": "0x0",
                "to": "0x19e924d7f7aa69fcef53fa2d42f6cc1fc301ce3f"
              }
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": "0xc5f61a925a2085ffcb3f52eb4e90dd9d882cc2c83b31ae3f7660385e0b7b93b3"
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getTransactionReceipt",
            "params": [
              "0xc5f61a925a2085ffcb3f52eb4e90dd9d882cc2c83b31ae3f7660385e0b7b93b3"
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": null
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getTransactionReceipt",
            "params": [
              "0xc5f61a925a2085ffcb3f52eb4e90dd9d882cc2c83b31ae3f7660385e0b7b93b3"
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": {
              "blockHash": "0xeb925a6daa75198b6f8ce3221b59f0d4695160ec7cea653a498934d9aef77b17",
              "blockNumber": "0x2",
              "contractAddress": null,
              "cumulativeGasUsed": "0x6592",
              "effectiveGasPrice": "0x77359400",
              "from": "0x757b6af7c6eb736f655302d054e2366e2df1241c",
              "gasUsed": "0x6592",
              "logs": [],
              "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
              "status": "0x1",
              "to": "0x19e924d7f7aa69fcef53fa2d42f6cc1fc301ce3f",
              "transactionHash": "0xc5f61a925a2085ffcb3f52eb4e90dd9d882cc2c83b31ae3f7660385e0b7b93b3",
              "transactionIndex": "0x0",
              "type": "0x0"
            }
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getTransactionByHash",
            "params": [
              "0xc5f61a925a2085ffcb3f52eb4e90dd9d882cc2c83b31ae3f7660385e0b7b93b3"
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": {
              "blockHash": "0xeb925a6daa75198b6f8ce3221b59f0d4695160ec7cea653a498934d9aef77b17",
              "blockNumber": "0x2",
              "from": "0x757b6af7c6eb736f655302d054e2366e2df1241c",
              "gas": "0xf4240",
              "gasPrice": "0x77359400",
              "hash": "0xc5f61a925a2085ffcb3f52eb4e90dd9d882cc2c83b31ae3f7660385e0b7b93b3",
              "input": "0x",
              "nonce": "0x1",
              "to": "0x19e924d7f7aa69fcef53fa2d42f6cc1fc301ce3f",
              "transactionIndex": "0x0",
              "value": "0x0",
              "type": "0x0",
              "chainId": "0x539",
              "v": "0xa95",
              "r": "0xe32a3a8dfdb3299a59ea9ff6e228c631c5e49e8dce283c7dfd8a1b50c6c75b67",
              "s": "0x341b0be423f3cacd118b0d7f69f483fcb5aa5dfb6bbd07192db79d7d3e67dfce"
            }
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "debug_traceTransaction",
            "params": [
              "0xc5f61a925a2085ffcb3f52eb4e90dd9d882cc2c83b31ae3f7660385e0b7b93b3",
              {
                "tracer": "callTracer",
                "timeout": "5s"
              }
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": {
              "from": "0x757b6af7c6eb736f655302d054e2366e2df1241c",
              "gas": "0xf4240",
              "gasUsed": "0x6592",
              "to": "0x19e924d7f7aa69fcef53fa2d42f6cc1fc301ce3f",
              "input": "0x",
              "calls": [
                {
                  "from": "0x19e924d7f7aa69fcef53fa2d42f6cc1fc301ce3f",
                  "gas": "0x0",
                  "gasUsed": "0x0",
                  "to": "0x757b6af7c6eb736f655302d054e2366e2df1241c",
                  "input": "0x",
                  "value": "0x0",
                  "type": "SELFDESTRUCT"
                }
              ],
              "value": "0x0",
              "type": "CALL"
            }
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getBlockByHash",
            "params": [
              "0xeb925a6daa75198b6f8ce3221b59f0d4695160ec7cea653a498934d9aef77b17",
              false
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": {
              "baseFeePerGas": "0x2db20f61",
              "blobGasUsed": "0x0",
              "difficulty": "0x0",
              "excessBlobGas": "0x0",
              "extraData": "0xd883010e0d846765746888676f312e32332e33856c696e7578",
              "gasLimit": "0xafd1a5",
              "gasUsed": "0x6592",
              "hash": "0xeb925a6daa75198b6f8ce3221b59f0d4695160ec7cea653a498934d9aef77b17",
              "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
              "miner": "0x0000000000000000000000000000000000000000",
              "mixHash": "0xf160fe09ca3c240226cbc2bea8c9d22b78ac6016823bde048ca8e154511d936f",
              "nonce": "0x0000000000000000",
              "number": "0x2",
              "parentBeaconBlockRoot": "0x0000000000000000000000000000000000000000000000000000000000000000",
              "parentHash": "0xc33af3223c1d19691f4bd92845f60633cf3dcd4fe4a96fad5d996964ac678ede",
              "receiptsRoot": "0x9bca7248c8db2a528148b697507b96036d467dd3705dc10424bc435cb5bca30b",
              "requestsRoot": "0x6036c41849da9c076ed79654d434017387a88fb833c2856b32e18218b3341c5f",
              "sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
              "size": "0x2eb",
              "stateRoot": "0x54f91823adc0d1a919ad0d1d0f20911791814a085d2e95a6c3b81cc85b1f04fb",
              "timestamp": "0x6aeaa33c",
              "transactions": [
                "0xc5f61a925a2085ffcb3f52eb4e90dd9d882cc2c83b31ae3f7660385e0b7b93b3"
              ],
              "transactionsRoot": "0x6750b38363959d7c5a9519d8bd60444f896023de8b8cc63ac9ca35c4929b4d5c",
              "uncles": [],
              "withdrawals": [],
              "withdrawalsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421"
            }
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getCode",
            "params": [
              "0x19e924d7f7aa69fcef53fa2d42f6cc1fc301ce3f",
              {
                "blockHash": "0xeb925a6daa75198b6f8ce3221b59f0d4695160ec7cea653a498934d9aef77b17",
                "requireCanonical": true
              }
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": "0x33ff"
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_sendTransaction",
            "params": [
              {
                "from": "0x757b6af7c6eb736f655302d054e2366e2df1241c",
                "gas": "0xf4240",
                "gasPrice": "0x77359400",
                "value": "0x0",
                "data": "0x33ff"
              }
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": "0x1720543a6eef16d7b26d48da8233679a1b17407682ba55d10d698bea1bc5d691"
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getTransactionReceipt",
            "params": [
              "0x1720543a6eef16d7b26d48da8233679a1b17407682ba55d10d698bea1bc5d691"
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": null
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getTransactionReceipt",
            "params": [
              "0x1720543a6eef16d7b26d48da8233679a1b17407682ba55d10d698bea1bc5d691"
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": {
              "blockHash": "0x8ef5dedd37d2ffa9a3f5eceaa59815a686d94666afe05abbe05b3c72ae3cbf1d",
              "blockNumber": "0x3",
              "contractAddress": "0x08ec17357687d373a536057daf990a3b49742544",
              "cumulativeGasUsed": "0xe2b4",
              "effectiveGasPrice": "0x77359400",
              "from": "0x757b6af7c6eb736f655302d054e2366e2df1241c",
              "gasUsed": "0xe2b4",
              "logs": [],
              "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
              "status": "0x1",
              "to": null,
              "transactionHash": "0x1720543a6eef16d7b26d48da8233679a1b17407682ba55d10d698bea1bc5d691",
              "transactionIndex": "0x0",
              "type": "0x0"
            }
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getTransactionByHash",
            "params": [
              "0x1720543a6eef16d7b26d48da8233679a1b17407682ba55d10d698bea1bc5d691"
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": {
              "blockHash": "0x8ef5dedd37d2ffa9a3f5eceaa59815a686d94666afe05abbe05b3c72ae3cbf1d",
              "blockNumber": "0x3",
              "from": "0x757b6af7c6eb736f655302d054e2366e2df1241c",
              "gas": "0xf4240",
              "gasPrice": "0x77359400",
              "hash": "0x1720543a6eef16d7b26d48da8233679a1b17407682ba55d10d698bea1bc5d691",
              "input": "0x33ff",
              "nonce": "0x2",
              "to": null,
              "transactionIndex": "0x0",
              "value": "0x0",
              "type": "0x0",
              "chainId": "0x539",
              "v": "0xa96",
              "r": "0xffca709ce60ebfd64e5c58e8c9b514b42492a69906960677779bc262dc9566e1",
              "s": "0x42cd45d4cec062023f36993a230ce7d7eb19903aa9531d9da83ceb2a3361bd62"
            }
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "debug_traceTransaction",
            "params": [
              "0x1720543a6eef16d7b26d48da8233679a1b17407682ba55d10d698bea1bc5d691",
              {
                "tracer": "callTracer",
                "timeout": "5s"
              }
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": {
              "from": "0x757b6af7c6eb736f655302d054e2366e2df1241c",
              "gas": "0xf4240",
              "gasUsed": "0xe2b4",
              "to": "0x08ec17357687d373a536057daf990a3b49742544",
              "input": "0x33ff",
              "calls": [
                {
                  "from": "0x08ec17357687d373a536057daf990a3b49742544",
                  "gas": "0x0",
                  "gasUsed": "0x0",
                  "to": "0x757b6af7c6eb736f655302d054e2366e2df1241c",
                  "input": "0x",
                  "value": "0x0",
                  "type": "SELFDESTRUCT"
                }
              ],
              "value": "0x0",
              "type": "CREATE"
            }
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getBlockByHash",
            "params": [
              "0x8ef5dedd37d2ffa9a3f5eceaa59815a686d94666afe05abbe05b3c72ae3cbf1d",
              false
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": {
              "baseFeePerGas": "0x280266f2",
              "blobGasUsed": "0x0",
              "difficulty": "0x0",
              "excessBlobGas": "0x0",
              "extraData": "0xd883010e0d846765746888676f312e32332e33856c696e7578",
              "gasLimit": "0xaffd98",
              "gasUsed": "0xe2b4",
              "hash": "0x8ef5dedd37d2ffa9a3f5eceaa59815a686d94666afe05abbe05b3c72ae3cbf1d",
              "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
              "miner": "0x0000000000000000000000000000000000000000",
              "mixHash": "0x3fc96272fc22ef5cfbf84790c9c937d176bd089191a0a6964289a4c96e7db49e",
              "nonce": "0x0000000000000000",
              "number": "0x3",
              "parentBeaconBlockRoot": "0x0000000000000000000000000000000000000000000000000000000000000000",
              "parentHash": "0xeb925a6daa75198b6f8ce3221b59f0d4695160ec7cea653a498934d9aef77b17",
              "receiptsRoot": "0xbb6f8ba0d0dc4b3621685ae1ecdafd4eb9c4feeaa2f34323f93900ff0e9ee85c",
              "requestsRoot": "0x6036c41849da9c076ed79654d434017387a88fb833c2856b32e18218b3341c5f",
              "sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
              "size": "0x2d9",
              "stateRoot": "0xaea654e9f3063ff669f7cf1ea0f4967b1a4328c95001421ec807839e3479ed5e",
              "timestamp": "0x6aeaa33d",
              "transactions": [
                "0x1720543a6eef16d7b26d48da8233679a1b17407682ba55d10d698bea1bc5d691"
              ],
              "transactionsRoot": "0x5e12cbe7b0a0fd33299845cb6537dfdbbe5ba3346723924450360c618e72370d",
              "uncles": [],
              "withdrawals": [],
              "withdrawalsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421"
            }
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getCode",
            "params": [
              "0x08ec17357687d373a536057daf990a3b49742544",
              {
                "blockHash": "0x8ef5dedd37d2ffa9a3f5eceaa59815a686d94666afe05abbe05b3c72ae3cbf1d",
                "requireCanonical": true
              }
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": "0x"
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getCode",
            "params": [
              "0x19e924d7f7aa69fcef53fa2d42f6cc1fc301ce3f",
              {
                "blockHash": "0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
                "requireCanonical": true
              }
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "error": {
              "code": -32000,
              "message": "header for hash not found"
            }
          }
        }
      ],
      "shanghai": [
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_accounts",
            "params": []
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": [
              "0x57b4e1616ddcb5312d3fe708001b4425b1280b7c"
            ]
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_sendTransaction",
            "params": [
              {
                "from": "0x57b4e1616ddcb5312d3fe708001b4425b1280b7c",
                "gas": "0xf4240",
                "gasPrice": "0x77359400",
                "value": "0x0",
                "data": "0x6002600c60003960026000f333ff"
              }
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": "0x864a9af987e86d35aaa35538c22d8702a7ea40cbd66c409d08afdd6c3d1ce81c"
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getTransactionReceipt",
            "params": [
              "0x864a9af987e86d35aaa35538c22d8702a7ea40cbd66c409d08afdd6c3d1ce81c"
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "error": {
              "code": -32000,
              "message": "transaction indexing is in progress",
              "data": "transaction indexing is in progress"
            }
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getTransactionReceipt",
            "params": [
              "0x864a9af987e86d35aaa35538c22d8702a7ea40cbd66c409d08afdd6c3d1ce81c"
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": {
              "blockHash": "0xf317deb56a5f8722ab24d0edef086d844e910b5ad89c3d61d13c05e807458333",
              "blockNumber": "0x1",
              "contractAddress": "0x9b6650c4de61ac49d05eb30c3f10eb4693a55f1f",
              "cumulativeGasUsed": "0xd17a",
              "effectiveGasPrice": "0x77359400",
              "from": "0x57b4e1616ddcb5312d3fe708001b4425b1280b7c",
              "gasUsed": "0xd17a",
              "logs": [],
              "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
              "status": "0x1",
              "to": null,
              "transactionHash": "0x864a9af987e86d35aaa35538c22d8702a7ea40cbd66c409d08afdd6c3d1ce81c",
              "transactionIndex": "0x0",
              "type": "0x0"
            }
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getTransactionByHash",
            "params": [
              "0x864a9af987e86d35aaa35538c22d8702a7ea40cbd66c409d08afdd6c3d1ce81c"
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": {
              "blockHash": "0xf317deb56a5f8722ab24d0edef086d844e910b5ad89c3d61d13c05e807458333",
              "blockNumber": "0x1",
              "from": "0x57b4e1616ddcb5312d3fe708001b4425b1280b7c",
              "gas": "0xf4240",
              "gasPrice": "0x77359400",
              "hash": "0x864a9af987e86d35aaa35538c22d8702a7ea40cbd66c409d08afdd6c3d1ce81c",
              "input": "0x6002600c60003960026000f333ff",
              "nonce": "0x0",
              "to": null,
              "transactionIndex": "0x0",
              "value": "0x0",
              "type": "0x0",
              "chainId": "0x539",
              "v": "0xa96",
              "r": "0x566ebdb57b1880ed8aee32e579ecbccaf833ef7119afed34f50487bc484dce82",
              "s": "0x3e5d2d6a87a465a500857335f48d31822b3a0f7ae5145548d55c722acc5b22ea"
            }
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "debug_traceTransaction",
            "params": [
              "0x864a9af987e86d35aaa35538c22d8702a7ea40cbd66c409d08afdd6c3d1ce81c",
              {
                "tracer": "callTracer",
                "timeout": "5s"
              }
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": {
              "from": "0x57b4e1616ddcb5312d3fe708001b4425b1280b7c",
              "gas": "0xf4240",
              "gasUsed": "0xd17a",
              "to": "0x9b6650c4de61ac49d05eb30c3f10eb4693a55f1f",
              "input": "0x6002600c60003960026000f333ff",
              "output": "0x33ff",
              "value": "0x0",
              "type": "CREATE"
            }
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getBlockByHash",
            "params": [
              "0xf317deb56a5f8722ab24d0edef086d844e910b5ad89c3d61d13c05e807458333",
              false
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": {
              "baseFeePerGas": "0x342770c0",
              "difficulty": "0x0",
              "extraData": "0xd883010d0f846765746888676f312e32312e36856c696e7578",
              "gasLimit": "0xafa5bd",
              "gasUsed": "0xd17a",
              "hash": "0xf317deb56a5f8722ab24d0edef086d844e910b5ad89c3d61d13c05e807458333",
              "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
              "miner": "0x0000000000000000000000000000000000000000",
              "mixHash": "0x38930728ffe4a216c4c209de631d97ebe003a986fbeb5fd713308fda8095a7f8",
              "nonce": "0x0000000000000000",
              "number": "0x1",
              "parentHash": "0xa9d3e213a2fd76899c3b61e9a59dd5cf2a34e4da75f8d76f653d502747bc8a31",
              "receiptsRoot": "0x129b0163402aa35d7f2b43d8cb0a7b965ae0bd60a7a8308d33cff52f4bec278e",
              "sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
              "size": "0x2a1",
              "stateRoot": "0x2362d6c4ac0816cf98c5e817d5a4f89cdbacc25104591ee2b6566eb272d6e156",
              "timestamp": "0x6aeaa33f",
              "totalDifficulty": "0x1",
              "transactions": [
                "0x864a9af987e86d35aaa35538c22d8702a7ea40cbd66c409d08afdd6c3d1ce81c"
              ],
              "transactionsRoot": "0x6572472855409c2b138614754b8912b72f7eac7a464935677227fd3bd8ba8d15",
              "uncles": [],
              "withdrawals": [],
              "withdrawalsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421"
            }
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_sendTransaction",
            "params": [
              {
                "from": "0x57b4e1616ddcb5312d3fe708001b4425b1280b7c",
                "gas": "0xf4240",
                "gasPrice": "0x77359400",
                "value": "0x0",
                "data": "0x6027600c60003960276000f360006000600060006000739b6650c4de61ac49d05eb30c3f10eb4693a55f1f5af15060006000fd"
              }
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": "0x0019ec407056116f027837e6a02a65ab9356e6e4cda725707d96b9f62738cc18"
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getTransactionReceipt",
            "params": [
              "0x0019ec407056116f027837e6a02a65ab9356e6e4cda725707d96b9f62738cc18"
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": null
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getTransactionReceipt",
            "params": [
              "0x0019ec407056116f027837e6a02a65ab9356e6e4cda725707d96b9f62738cc18"
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": {
              "blockHash": "0x661d8d8d7a85684e590a7541665d14cd70093cf4708000e77d13b5766a63babc",
              "blockNumber": "0x2",
              "contractAddress": "0xbf67e961430f05b2d23b11e3aa5e883d15a85283",
              "cumulativeGasUsed": "0xf066",
              "effectiveGasPrice": "0x77359400",
              "from": "0x57b4e1616ddcb5312d3fe708001b4425b1280b7c",
              "gasUsed": "0xf066",
              "logs": [],
              "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
              "status": "0x1",
              "to": null,
              "transactionHash": "0x0019ec407056116f027837e6a02a65ab9356e6e4cda725707d96b9f62738cc18",
              "transactionIndex": "0x0",
              "type": "0x0"
            }
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getTransactionByHash",
            "params": [
              "0x0019ec407056116f027837e6a02a65ab9356e6e4cda725707d96b9f62738cc18"
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": {
              "blockHash": "0x661d8d8d7a85684e590a7541665d14cd70093cf4708000e77d13b5766a63babc",
              "blockNumber": "0x2",
              "from": "0x57b4e1616ddcb5312d3fe708001b4425b1280b7c",
              "gas": "0xf4240",
              "gasPrice": "0x77359400",
              "hash": "0x0019ec407056116f027837e6a02a65ab9356e6e4cda725707d96b9f62738cc18",
              "input": "0x6027600c60003960276000f360006000600060006000739b6650c4de61ac49d05eb30c3f10eb4693a55f1f5af15060006000fd",
              "nonce": "0x1",
              "to": null,
              "transactionIndex": "0x0",
              "value": "0x0",
              "type": "0x0",
              "chainId": "0x539",
              "v": "0xa96",
              "r": "0x9f7e029f3ffcbdd805df9d3954c23a67c32ab0e02955027c8193a9f4904ad1dd",
              "s": "0x5272a232da3f88459f381b2f653840a82fcf5bfad2fb21be440363dc93ceebd4"
            }
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "debug_traceTransaction",
            "params": [
              "0x0019ec407056116f027837e6a02a65ab9356e6e4cda725707d96b9f62738cc18",
              {
                "tracer": "callTracer",
                "timeout": "5s"
              }
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": {
              "from": "0x57b4e1616ddcb5312d3fe708001b4425b1280b7c",
              "gas": "0xf4240",
              "gasUsed": "0xf066",
              "to": "0xbf67e961430f05b2d23b11e3aa5e883d15a85283",
              "input": "0x6027600c60003960276000f360006000600060006000739b6650c4de61ac49d05eb30c3f10eb4693a55f1f5af15060006000fd",
              "output": "0x60006000600060006000739b6650c4de61ac49d05eb30c3f10eb4693a55f1f5af15060006000fd",
              "value": "0x0",
              "type": "CREATE"
            }
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getBlockByHash",
            "params": [
              "0x661d8d8d7a85684e590a7541665d14cd70093cf4708000e77d13b5766a63babc",
              false
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": {
              "baseFeePerGas": "0x2db20f61",
              "difficulty": "0x0",
              "extraData": "0xd883010d0f846765746888676f312e32312e36856c696e7578",
              "gasLimit": "0xafd1a5",
              "gasUsed": "0xf066",
              "hash": "0x661d8d8d7a85684e590a7541665d14cd70093cf4708000e77d13b5766a63babc",
              "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
              "miner": "0x0000000000000000000000000000000000000000",
              "mixHash": "0xb0883577f7bdbdbbd0368589b1496aed221948fcf23ccd2fd1fa805d024936b2",
              "nonce": "0x0000000000000000",
              "number": "0x2",
              "parentHash": "0xf317deb56a5f8722ab24d0edef086d844e910b5ad89c3d61d13c05e807458333",
              "receiptsRoot": "0xd86329f101e49b6e74451c7028dd2b03d3c3925e8de963af476e5ed18f9ca8d0",
              "sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
              "size": "0x2c6",
              "stateRoot": "0x058edd9553234032eccc3a00f534961eb30ec93675e8218076a59c963ff5aa29",
              "timestamp": "0x6aeaa340",
              "totalDifficulty": "0x1",
              "transactions": [
                "0x0019ec407056116f027837e6a02a65ab9356e6e4cda725707d96b9f62738cc18"
              ],
              "transactionsRoot": "0xc2b71755428de7fcbff58e1c5f0f433e1e8091c10682510c8da0a766cc63e353",
              "uncles": [],
              "withdrawals": [],
              "withdrawalsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421"
            }
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_sendTransaction",
            "params": [
              {
                "from": "0x57b4e1616ddcb5312d3fe708001b4425b1280b7c",
                "gas": "0xf4240",
                "gasPrice": "0x77359400",
                "value": "0x0",
                "to": "0xbf67e961430f05b2d23b11e3aa5e883d15a85283"
              }
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": "0xea227cfb11a0c20427c20cc17e4662c8b5528fa4e2158396d69e8b4487893d05"
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getTransactionReceipt",
            "params": [
              "0xea227cfb11a0c20427c20cc17e4662c8b5528fa4e2158396d69e8b4487893d05"
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": null
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getTransactionReceipt",
            "params": [
              "0xea227cfb11a0c20427c20cc17e4662c8b5528fa4e2158396d69e8b4487893d05"
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": {
              "blockHash": "0x11f833f1cbee9dd834eedb603eaf977eac04150fa57a1b23f05ab76b43a69486",
              "blockNumber": "0x3",
              "contractAddress": null,
              "cumulativeGasUsed": "0x6fd6",
              "effectiveGasPrice": "0x77359400",
              "from": "0x57b4e1616ddcb5312d3fe708001b4425b1280b7c",
              "gasUsed": "0x6fd6",
              "logs": [],
              "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
              "status": "0x0",
              "to": "0xbf67e961430f05b2d23b11e3aa5e883d15a85283",
              "transactionHash": "0xea227cfb11a0c20427c20cc17e4662c8b5528fa4e2158396d69e8b4487893d05",
              "transactionIndex": "0x0",
              "type": "0x0"
            }
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getTransactionByHash",
            "params": [
              "0xea227cfb11a0c20427c20cc17e4662c8b5528fa4e2158396d69e8b4487893d05"
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": {
              "blockHash": "0x11f833f1cbee9dd834eedb603eaf977eac04150fa57a1b23f05ab76b43a69486",
              "blockNumber": "0x3",
              "from": "0x57b4e1616ddcb5312d3fe708001b4425b1280b7c",
              "gas": "0xf4240",
              "gasPrice": "0x77359400",
              "hash": "0xea227cfb11a0c20427c20cc17e4662c8b5528fa4e2158396d69e8b4487893d05",
              "input": "0x",
              "nonce": "0x2",
              "to": "0xbf67e961430f05b2d23b11e3aa5e883d15a85283",
              "transactionIndex": "0x0",
              "value": "0x0",
              "type": "0x0",
              "chainId": "0x539",
              "v": "0xa96",
              "r": "0xaefeb422af7d0a8744d9130bf2d58e58283f0b5f518e137f2a8ea69574786a28",
              "s": "0x46cab4b0ce1bf03d6d8ed89b251a034d5fd125372967178f0036ecd6e880843c"
            }
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "debug_traceTransaction",
            "params": [
              "0xea227cfb11a0c20427c20cc17e4662c8b5528fa4e2158396d69e8b4487893d05",
              {
                "tracer": "callTracer",
                "timeout": "5s"
              }
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": {
              "from": "0x57b4e1616ddcb5312d3fe708001b4425b1280b7c",
              "gas": "0xf4240",
              "gasUsed": "0x6fd6",
              "to": "0xbf67e961430f05b2d23b11e3aa5e883d15a85283",
              "input": "0x",
              "error": "execution reverted",
              "calls": [
                {
                  "from": "0xbf67e961430f05b2d23b11e3aa5e883d15a85283",
                  "gas": "0xeaa65",
                  "gasUsed": "0x138a",
                  "to": "0x9b6650c4de61ac49d05eb30c3f10eb4693a55f1f",
                  "input": "0x",
                  "calls": [
                    {
                      "from": "0x9b6650c4de61ac49d05eb30c3f10eb4693a55f1f",
                      "gas": "0x0",
                      "gasUsed": "0x0",
                      "to": "0xbf67e961430f05b2d23b11e3aa5e883d15a85283",
                      "input": "0x",
                      "value": "0x0",
                      "type": "SELFDESTRUCT"
                    }
                  ],
                  "value": "0x0",
                  "type": "CALL"
                }
              ],
              "value": "0x0",
              "type": "CALL"
            }
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getBlockByHash",
            "params": [
              "0x11f833f1cbee9dd834eedb603eaf977eac04150fa57a1b23f05ab76b43a69486",
              false
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": {
              "baseFeePerGas": "0x280b6c2b",
              "difficulty": "0x0",
              "extraData": "0xd883010d0f846765746888676f312e32312e36856c696e7578",
              "gasLimit": "0xaffd98",
              "gasUsed": "0x6fd6",
              "hash": "0x11f833f1cbee9dd834eedb603eaf977eac04150fa57a1b23f05ab76b43a69486",
              "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
              "miner": "0x0000000000000000000000000000000000000000",
              "mixHash": "0x663ab9c4492329904cdffd62696dca1b301c0c7aec3f915a52095baab30175ac",
              "nonce": "0x0000000000000000",
              "number": "0x3",
              "parentHash": "0x661d8d8d7a85684e590a7541665d14cd70093cf4708000e77d13b5766a63babc",
              "receiptsRoot": "0x261c53ff5540428fdeac963885f93123b7efa67c46eec26a852bc6039cb31095",
              "sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
              "size": "0x2a7",
              "stateRoot": "0x5b13be8a6a852e49beeabd4d30e08ddcc113dcee9d60af65b90450bb1f2649ff",
              "timestamp": "0x6aeaa341",
              "totalDifficulty": "0x1",
              "transactions": [
                "0xea227cfb11a0c20427c20cc17e4662c8b5528fa4e2158396d69e8b4487893d05"
              ],
              "transactionsRoot": "0x7c47274f5f950e823f3b7591714409f1924a363f0ed3e2d421f8b9883b942437",
              "uncles": [],
              "withdrawals": [],
              "withdrawalsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421"
            }
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getCode",
            "params": [
              "0x9b6650c4de61ac49d05eb30c3f10eb4693a55f1f",
              {
                "blockHash": "0x11f833f1cbee9dd834eedb603eaf977eac04150fa57a1b23f05ab76b43a69486",
                "requireCanonical": true
              }
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": "0x33ff"
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_sendTransaction",
            "params": [
              {
                "from": "0x57b4e1616ddcb5312d3fe708001b4425b1280b7c",
                "gas": "0xf4240",
                "gasPrice": "0x77359400",
                "value": "0x0",
                "to": "0x9b6650c4de61ac49d05eb30c3f10eb4693a55f1f"
              }
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": "0x2b8ac3debae383f6a7aa7e92ef25932ac16126778cb52a22b89ca721f60ba019"
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getTransactionReceipt",
            "params": [
              "0x2b8ac3debae383f6a7aa7e92ef25932ac16126778cb52a22b89ca721f60ba019"
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": null
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getTransactionReceipt",
            "params": [
              "0x2b8ac3debae383f6a7aa7e92ef25932ac16126778cb52a22b89ca721f60ba019"
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": null
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getTransactionReceipt",
            "params": [
              "0x2b8ac3debae383f6a7aa7e92ef25932ac16126778cb52a22b89ca721f60ba019"
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": {
              "blockHash": "0x5cdddaa9bde04beb65ed0dbe53ad6e802d5a2eb5f48d1e847dc3f1790eab029e",
              "blockNumber": "0x4",
              "contractAddress": null,
              "cumulativeGasUsed": "0x6592",
              "effectiveGasPrice": "0x77359400",
              "from": "0x57b4e1616ddcb5312d3fe708001b4425b1280b7c",
              "gasUsed": "0x6592",
              "logs": [],
              "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
              "status": "0x1",
              "to": "0x9b6650c4de61ac49d05eb30c3f10eb4693a55f1f",
              "transactionHash": "0x2b8ac3debae383f6a7aa7e92ef25932ac16126778cb52a22b89ca721f60ba019",
              "transactionIndex": "0x0",
              "type": "0x0"
            }
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getTransactionByHash",
            "params": [
              "0x2b8ac3debae383f6a7aa7e92ef25932ac16126778cb52a22b89ca721f60ba019"
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": {
              "blockHash": "0x5cdddaa9bde04beb65ed0dbe53ad6e802d5a2eb5f48d1e847dc3f1790eab029e",
              "blockNumber": "0x4",
              "from": "0x57b4e1616ddcb5312d3fe708001b4425b1280b7c",
              "gas": "0xf4240",
              "gasPrice": "0x77359400",
              "hash": "0x2b8ac3debae383f6a7aa7e92ef25932ac16126778cb52a22b89ca721f60ba019",
              "input": "0x",
              "nonce": "0x3",
              "to": "0x9b6650c4de61ac49d05eb30c3f10eb4693a55f1f",
              "transactionIndex": "0x0",
              "value": "0x0",
              "type": "0x0",
              "chainId": "0x539",
              "v": "0xa95",
              "r": "0x150d9d27905303384e9d21d0ee3d54c20324354f2f28e883dba26a32c934a48c",
              "s": "0x54a97ee4bc8048b97645e54268ca341ccc50aac38d4f6c67957ed3829e87841b"
            }
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "debug_traceTransaction",
            "params": [
              "0x2b8ac3debae383f6a7aa7e92ef25932ac16126778cb52a22b89ca721f60ba019",
              {
                "tracer": "callTracer",
                "timeout": "5s"
              }
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": {
              "from": "0x57b4e1616ddcb5312d3fe708001b4425b1280b7c",
              "gas": "0xf4240",
              "gasUsed": "0x6592",
              "to": "0x9b6650c4de61ac49d05eb30c3f10eb4693a55f1f",
              "input": "0x",
              "calls": [
                {
                  "from": "0x9b6650c4de61ac49d05eb30c3f10eb4693a55f1f",
                  "gas": "0x0",
                  "gasUsed": "0x0",
                  "to": "0x57b4e1616ddcb5312d3fe708001b4425b1280b7c",
                  "input": "0x",
                  "value": "0x0",
                  "type": "SELFDESTRUCT"
                }
              ],
              "value": "0x0",
              "type": "CALL"
            }
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getBlockByHash",
            "params": [
              "0x5cdddaa9bde04beb65ed0dbe53ad6e802d5a2eb5f48d1e847dc3f1790eab029e",
              false
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": {
              "baseFeePerGas": "0x23105b41",
              "difficulty": "0x0",
              "extraData": "0xd883010d0f846765746888676f312e32312e36856c696e7578",
              "gasLimit": "0xb02996",
              "gasUsed": "0x6592",
              "hash": "0x5cdddaa9bde04beb65ed0dbe53ad6e802d5a2eb5f48d1e847dc3f1790eab029e",
              "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
              "miner": "0x0000000000000000000000000000000000000000",
              "mixHash": "0xb1683a21a5e575195b7ceac72c473d9c885e1490a653d7ae537fb597069801d9",
              "nonce": "0x0000000000000000",
              "number": "0x4",
              "parentHash": "0x11f833f1cbee9dd834eedb603eaf977eac04150fa57a1b23f05ab76b43a69486",
              "receiptsRoot": "0x9bca7248c8db2a528148b697507b96036d467dd3705dc10424bc435cb5bca30b",
              "sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
              "size": "0x2a7",
              "stateRoot": "0xf44787af4c8bb948c75883a4bcf6da4ef6028a92216235a1a8cd3eb96dbc2e9c",
              "timestamp": "0x6aeaa342",
              "totalDifficulty": "0x1",
              "transactions": [
                "0x2b8ac3debae383f6a7aa7e92ef25932ac16126778cb52a22b89ca721f60ba019"
              ],
              "transactionsRoot": "0x47c6caa5dbe0cb25f4c65570c1bcc27756cdc8bada8689871c00e42721a436a1",
              "uncles": [],
              "withdrawals": [],
              "withdrawalsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421"
            }
          }
        },
        {
          "request": {
            "jsonrpc": "2.0",
            "id": 0,
            "method": "eth_getCode",
            "params": [
              "0x9b6650c4de61ac49d05eb30c3f10eb4693a55f1f",
              {
                "blockHash": "0x5cdddaa9bde04beb65ed0dbe53ad6e802d5a2eb5f48d1e847dc3f1790eab029e",
                "requireCanonical": true
              }
            ]
          },
          "response": {
            "jsonrpc": "2.0",
            "id": 0,
            "result": "0x"
          }
        }
      ]
    }
    ''')
  end
end
