defmodule Explorer.Migrator.SelfdestructChainData do
  import ExUnit.Assertions
  import Explorer.Factory

  alias EthereumJSONRPC.Geth
  alias EthereumJSONRPC.Geth.Calls
  alias Explorer.Chain.{Address, Block, Data, InternalTransaction}
  alias Explorer.Migrator.EmptyBytecodeForSelfdestructedSmartContracts
  alias Explorer.Repo

  def recording(name) do
    __DIR__
    |> Path.join("selfdestruct_chain_data/#{name}.json")
    |> File.read!()
    |> Jason.decode!()
  end

  def import_recording(name, cached_code \\ nil) do
    recording = recording(name)
    config = Application.fetch_env!(:ethereum_jsonrpc, Geth)
    Application.put_env(:ethereum_jsonrpc, Geth, Keyword.put(config, :tracer, "call_tracer"))

    try do
      arguments = recorded_transport(recording)
      params = Enum.flat_map(recording["records"], &trace_params(&1, arguments))

      {params, _} =
        Enum.map_reduce(params, %{}, fn params, counts ->
          index = Map.get(counts, params.block_hash, 0)
          {Map.put(params, :block_index, index), Map.put(counts, params.block_hash, index + 1)}
        end)

      assert Enum.any?(params, fn params ->
               params[:created_contract_address_hash] == recording["address"] and
                 params[:created_contract_code] == recording["cached_code"]
             end)

      params
      |> Enum.flat_map(&[&1[:from_address_hash], &1[:to_address_hash], &1[:created_contract_address_hash]])
      |> Enum.reject(&is_nil/1)
      |> Enum.uniq()
      |> Enum.each(fn hash -> insert(:address, hash: hash) end)

      Enum.each(recording["records"], fn record ->
        block = record["block"]["response"]["result"]
        transaction = record["transaction"]["response"]["result"]
        receipt = record["receipt"]["response"]["result"]
        assert receipt["status"] == "0x1"
        assert receipt["transactionHash"] == transaction["hash"]
        assert receipt["blockHash"] == block["hash"]
        assert transaction["blockHash"] == block["hash"]
        assert record["trace"]["request"]["method"] == "debug_traceTransaction"
        assert [transaction["hash"], %{"tracer" => "callTracer"}] == record["trace"]["request"]["params"]

        stored_block =
          Repo.get(Block, block["hash"]) ||
            insert(:block,
              hash: block["hash"],
              number: quantity(block["number"]),
              parent_hash: block["parentHash"],
              timestamp: DateTime.from_unix!(quantity(block["timestamp"])),
              gas_limit: quantity(block["gasLimit"]),
              gas_used: quantity(block["gasUsed"])
            )

        insert(:transaction,
          hash: transaction["hash"],
          from_address: Repo.get!(Address, address_hash(transaction["from"])),
          to_address: transaction["to"] && Repo.get!(Address, address_hash(transaction["to"])),
          input: contract_data(transaction["input"]),
          nonce: quantity(transaction["nonce"]),
          value: quantity(transaction["value"]),
          gas: quantity(transaction["gas"]),
          gas_price: quantity(transaction["gasPrice"])
        )
        |> with_block(stored_block,
          index: quantity(transaction["transactionIndex"]),
          status: :ok,
          gas_used: quantity(receipt["gasUsed"]),
          cumulative_gas_used: quantity(receipt["cumulativeGasUsed"])
        )
      end)

      Enum.each(params, fn params ->
        changeset = InternalTransaction.changeset(%InternalTransaction{}, params)
        assert changeset.valid?, inspect(changeset.errors)
        Repo.insert!(changeset)
      end)

      contracts = Map.values(Map.get(recording, "contracts", %{"target" => recording}))

      Enum.each(contracts, fn contract ->
        Repo.get!(Address, address_hash(contract["address"]))
        |> Ecto.Changeset.change(contract_code: contract_data(cached_code || contract["cached_code"]))
        |> Repo.update!()
      end)

      recording
      |> Map.put("stored_address_hash", address_hash(recording["address"]))
      |> Map.put("rpc_arguments", arguments)
    after
      Application.put_env(:ethereum_jsonrpc, Geth, config)
    end
  end

  def assert_committed_code(recording, block_numbers \\ nil) do
    expected = recording["current"]["response"]["result"]
    request = recording["current"]["request"]
    assert request["method"] == "eth_getCode"

    assert [recording["address"], List.last(recording["records"])["block"]["response"]["result"]["number"]] ==
             request["params"]

    assert {:ok, [%{id: 0, result: ^expected}]} =
             EthereumJSONRPC.json_rpc(
               [EthereumJSONRPC.request(%{id: 0, method: request["method"], params: request["params"]})],
               recording["rpc_arguments"]
             )

    assert {:ok, _} =
             EmptyBytecodeForSelfdestructedSmartContracts.update_batch(
               block_numbers || [quantity(recording["destruction_block"])]
             )

    assert Repo.get!(Address, recording["stored_address_hash"]).contract_code == contract_data(expected)
  end

  def trace_params(record, arguments) do
    transaction = record["transaction"]["response"]["result"]
    block = record["block"]["response"]["result"]
    request = record["trace"]["request"]

    assert {:ok, [%{id: 0, result: calls}]} =
             EthereumJSONRPC.json_rpc(
               [EthereumJSONRPC.request(%{id: 0, method: request["method"], params: request["params"]})],
               arguments
             )

    assert calls == record["trace"]["response"]["result"]

    calls
    |> Geth.prepare_calls()
    |> Enum.with_index()
    |> Enum.map(fn {call, index} ->
      Map.merge(call, %{
        "blockNumber" => quantity(block["number"]),
        "transactionIndex" => quantity(transaction["transactionIndex"]),
        "transactionHash" => transaction["hash"],
        "index" => index
      })
    end)
    |> Calls.to_internal_transactions_params()
    |> Enum.map(&Map.merge(&1, %{block_hash: block["hash"], block_index: &1.index}))
  end

  defp recorded_transport(recording) do
    responses =
      (Enum.map(recording["records"], & &1["trace"]) ++
         Enum.flat_map(Map.values(Map.get(recording, "contracts", %{"target" => recording})), fn contract ->
           Enum.map(~w(before at_destruction current), &contract[&1])
         end) ++ Map.get(recording, "canonical", []) ++ Map.get(recording, "missing_canonical", []))
      |> Map.new(fn exchange ->
        {{exchange["request"]["method"], exchange["request"]["params"]}, exchange["response"]}
      end)

    server = Bypass.open()
    test_process = self()

    chain_spec =
      Map.get(recording, "chain_spec") ||
        if recording["fork"] == "cancun", do: %{"config" => %{"cancunTime" => 0}}, else: %{"config" => %{}}

    Bypass.expect(server, "GET", "/chain-spec", fn conn ->
      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.resp(200, Jason.encode!(chain_spec))
    end)

    block_numbers =
      Map.new(recording["records"], fn record ->
        block = record["block"]["response"]["result"]
        {block["hash"], block["number"]}
      end)

    Bypass.expect(server, "POST", "/", fn conn ->
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      requests = Jason.decode!(body)
      assert is_list(requests)

      responses =
        Enum.map(requests, fn %{"jsonrpc" => "2.0", "id" => id, "method" => method, "params" => params} ->
          send(test_process, {:recorded_rpc_request, method, params})
          # EIP-1898 identifies the same recorded state as its numeric block selector.
          key =
            if Map.has_key?(responses, {method, params}) do
              {method, params}
            else
              [address, %{"blockHash" => hash, "requireCanonical" => true}] = params
              {method, [address, Map.fetch!(block_numbers, hash)]}
            end

          assert Map.has_key?(responses, key), "Unrecorded RPC request: #{inspect({method, params})}"
          responses |> Map.fetch!(key) |> Map.put("id", id)
        end)

      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.resp(200, Jason.encode!(responses))
    end)

    arguments = [
      variant: Geth,
      transport: EthereumJSONRPC.HTTP,
      transport_options: [
        http: EthereumJSONRPC.HTTP.HTTPoison,
        urls: ["http://127.0.0.1:#{server.port}"],
        fallback_urls: [],
        http_options: [timeout: 5_000, recv_timeout: 5_000]
      ]
    ]

    replace_env(:explorer, :http_client, HTTPoison)
    replace_env(:explorer, :json_rpc_named_arguments, arguments)
    config = Application.get_env(:explorer, Explorer.ChainSpec.GenesisData, [])

    replace_env(
      :explorer,
      Explorer.ChainSpec.GenesisData,
      Keyword.put(config, :chain_spec_path, "http://127.0.0.1:#{server.port}/chain-spec")
    )

    assert {:ok, %{status_code: 200, body: body}} =
             Explorer.HttpClient.get("http://127.0.0.1:#{server.port}/chain-spec")

    assert Jason.decode!(body) == chain_spec

    activation_block =
      case chain_spec["config"]["cancunTime"] do
        nil ->
          :never

        activation ->
          recording["records"]
          |> Enum.map(& &1["block"]["response"]["result"])
          |> Enum.find(&(quantity(&1["timestamp"]) >= activation))
          |> case do
            nil -> quantity(List.last(recording["records"])["block"]["response"]["result"]["number"]) + 1
            block -> quantity(block["number"])
          end
      end

    configure_cancun_block(activation_block)

    arguments
  end

  def configure_cancun_block(height) do
    config = Application.get_env(:explorer, EmptyBytecodeForSelfdestructedSmartContracts, [])

    replace_env(
      :explorer,
      EmptyBytecodeForSelfdestructedSmartContracts,
      Keyword.put(config, :cancun_block, height)
    )
  end

  def contract_recording(recording, name) do
    recording["contracts"][name]
    |> Map.put("records", recording["records"])
    |> Map.put("stored_address_hash", address_hash(recording["contracts"][name]["address"]))
    |> Map.put("rpc_arguments", recording["rpc_arguments"])
  end

  defp replace_env(application, key, value) do
    previous = Application.fetch_env(application, key)
    Application.put_env(application, key, value)

    ExUnit.Callbacks.on_exit(fn ->
      case previous do
        {:ok, value} -> Application.put_env(application, key, value)
        :error -> Application.delete_env(application, key)
      end
    end)
  end

  def quantity(value), do: EthereumJSONRPC.quantity_to_integer(value)

  def contract_data(value) do
    {:ok, data} = Data.cast(value)
    data
  end

  defp address_hash(value) do
    {:ok, hash} = Explorer.Chain.Hash.Address.cast(value)
    hash
  end
end
