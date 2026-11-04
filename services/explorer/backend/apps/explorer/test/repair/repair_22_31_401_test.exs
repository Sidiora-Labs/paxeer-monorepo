defmodule Explorer.Repair2231401Test do
  use ExUnit.Case, async: false

  alias Explorer.Chain.PaxeerX.{SequencerReceipt, Status}
  alias Explorer.Chain.PaxeerX.Finality.Heights
  alias Explorer.HttpClient

  @backend Path.expand("../../../..", __DIR__)
  @runtime Path.join(@backend, "config/runtime.exs")
  @vectors Path.expand("../../lib/explorer/chain/paxeer_x/fixtures/receipt_commit_inclusion.json", __DIR__)
  @endpoint_env ~w(PAXEER_X_NODE_RPC_URL PAXEER_X_GATEWAY_URL PAXEER_X_ROUTER_URL)

  defmodule RecordedRPC do
    @behaviour Plug

    def init(options), do: options

    def call(conn, {producer, responses, observer}) do
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      %{"jsonrpc" => "2.0", "id" => id, "method" => method, "params" => params} = Jason.decode!(body)
      {status, result} = Map.fetch!(responses, {method, params})
      send(observer, {:rpc_request, producer, method, params, conn.request_path})

      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.send_resp(status, Jason.encode!(%{jsonrpc: "2.0", id: id, result: result}))
    end
  end

  setup do
    original_env = Map.new(@endpoint_env, &{&1, System.get_env(&1)})
    original_receipts = Application.fetch_env(:explorer, SequencerReceipt)
    original_adapter = Application.fetch_env(:tesla, :adapter)

    Enum.each(@endpoint_env, &System.delete_env/1)
    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)
    {:ok, _} = Application.ensure_all_started(:plug_cowboy)

    on_exit(fn ->
      Enum.each(original_env, fn
        {key, nil} -> System.delete_env(key)
        {key, value} -> System.put_env(key, value)
      end)

      case original_receipts do
        {:ok, config} -> Application.put_env(:explorer, SequencerReceipt, config)
        :error -> Application.delete_env(:explorer, SequencerReceipt)
      end

      case original_adapter do
        {:ok, adapter} -> Application.put_env(:tesla, :adapter, adapter)
        :error -> Application.delete_env(:tesla, :adapter)
      end
    end)

    :ok
  end

  test "runtime configuration separates node-root and hosted-gateway endpoints" do
    System.put_env("PAXEER_X_NODE_RPC_URL", "https://node.example/")
    System.put_env("PAXEER_X_GATEWAY_URL", "https://gateway.example/rpc/")
    System.put_env("PAXEER_X_ROUTER_URL", "https://retired.example/")

    config = receipt_config()

    assert config[:node_rpc_url] == "https://node.example"
    assert URI.parse(config[:node_rpc_url]).path in [nil, "", "/"]
    assert config[:gateway_url] == "https://gateway.example/rpc"
    assert config[:router_url] == config[:gateway_url]
    refute "https://retired.example" in Keyword.values(config)

    for environment <- [:dev, :test, :prod] do
      assert receipt_config(environment) == config
    end
  end

  test "the retired router setting supplies neither endpoint" do
    System.put_env("PAXEER_X_ROUTER_URL", "https://retired.example/")

    assert receipt_config() == [node_rpc_url: nil, gateway_url: nil, router_url: nil]
    Application.put_env(:explorer, SequencerReceipt, receipt_config())
    assert SequencerReceipt.receipt_keys() == :disabled
    assert SequencerReceipt.commit(1_000) == :disabled
    assert SequencerReceipt.validators(1_000) == :disabled
  end

  test "missing or invalid endpoints do not borrow the other producer" do
    System.put_env("PAXEER_X_NODE_RPC_URL", "https://node.example/")
    assert receipt_config() == [node_rpc_url: "https://node.example", gateway_url: nil, router_url: nil]

    System.put_env("PAXEER_X_GATEWAY_URL", "invalid-gateway")
    assert receipt_config() == [node_rpc_url: "https://node.example", gateway_url: nil, router_url: nil]

    System.delete_env("PAXEER_X_NODE_RPC_URL")
    System.put_env("PAXEER_X_GATEWAY_URL", "https://gateway.example/rpc")

    assert receipt_config() == [
             node_rpc_url: nil,
             gateway_url: "https://gateway.example/rpc",
             router_url: "https://gateway.example/rpc"
           ]

    System.put_env("PAXEER_X_NODE_RPC_URL", "invalid-node")

    assert receipt_config() == [
             node_rpc_url: nil,
             gateway_url: "https://gateway.example/rpc",
             router_url: "https://gateway.example/rpc"
           ]
  end

  test "recorded chain evidence reaches the committed status consumer" do
    vector = @vectors |> File.read!() |> Jason.decode!()
    answer = vector["receiptJson"]
    height = String.to_integer(vector["batch"]["height"])
    decimal_height = Integer.to_string(height)

    eth_receipt =
      answer
      |> Map.take(["blockHash", "blockNumber", "status", "gasUsed", "logs"])
      |> Map.put("transactionHash", answer["txHash"])
      |> Map.put("transactionIndex", answer["index"])

    node_calls = [
      {"pax_sequencerReceipt", [answer["txHash"]], answer},
      {"eth_getTransactionReceipt", [answer["txHash"]], eth_receipt}
    ]

    validators = %{
      "height" => decimal_height,
      "total" => vector["validators"]["total"],
      "validators" =>
        Enum.map(vector["validators"]["validators"], fn validator ->
          %{
            "address" => validator["address"],
            "pubKey" => validator["pub_key"],
            "votingPower" => validator["voting_power"]
          }
        end)
    }

    gateway_calls = [
      {"px_getReceiptKeys", [], %{"keys" => [vector["pinnedKey"]]}},
      {"px_getCommit", [decimal_height],
       %{"height" => decimal_height, "signedHeader" => vector["commit"]["signed_header"]}},
      {"px_getValidators", [decimal_height], validators},
      {"px_getSequenceBatch", [decimal_height], vector["batch"]}
    ]

    node = replay(:node, recorded_responses(node_calls))
    gateway = replay(:gateway, recorded_responses(gateway_calls)) <> "/rpc"
    configure_endpoints(node, gateway)

    config = Application.fetch_env!(:explorer, SequencerReceipt)
    header = vector["commit"]["signed_header"]["header"]

    Application.put_env(
      :explorer,
      SequencerReceipt,
      Keyword.merge(config,
        chain_id: header["chain_id"],
        trusted_validator_hashes: %{height => Base.decode16!(header["validators_hash"])}
      )
    )

    "0x" <> raw_transaction = vector["rawTransaction"]
    tx_hash = raw_transaction |> Base.decode16!(case: :mixed) |> ExKeccak.hash_256()
    {:ok, [key] = keys} = SequencerReceipt.decode_receipt_keys([vector["pinnedKey"]])
    assert "0x" <> Base.encode16(tx_hash, case: :lower) == answer["txHash"]
    assert {:ok, ^answer} = rpc(node, "pax_sequencerReceipt", [answer["txHash"]])
    assert {:ok, ^keys} = SequencerReceipt.receipt_keys()
    assert {:ok, ^eth_receipt} = rpc(node, "eth_getTransactionReceipt", [answer["txHash"]])
    assert {:ok, commit} = SequencerReceipt.commit(height)
    assert {:ok, transported_validators} = SequencerReceipt.validators(height)
    assert {:ok, batch} = SequencerReceipt.sequence_batch(height)
    assert batch == vector["batch"]

    verification =
      SequencerReceipt.verify(tx_hash, answer, keys,
        eth_receipt: eth_receipt,
        batch: batch,
        batch_activity_ids: [tx_hash],
        chain_id: header["chain_id"],
        commit: commit,
        validators: transported_validators
      )

    assert height == 10
    assert verification == {:committed, key, height}
    assert %Status{rung: :committed, committed_height: ^height} = Status.of(nil, %Heights{}, verification)
    assert %Status{rung: :submitted, committed_height: nil} = Status.of(nil, %Heights{}, nil)

    for {method, params, _result} <- node_calls do
      assert_received {:rpc_request, :node, ^method, ^params, "/"}
    end

    for {method, params, _result} <- gateway_calls do
      assert_received {:rpc_request, :gateway, ^method, ^params, "/rpc"}
    end

    refute_received {:rpc_request, _, _, _, _}
  end

  test "default receipt keys use the configured hosted gateway" do
    gateway = configure_unavailable_endpoints()

    assert {:error, {:http_status, 503}} = SequencerReceipt.receipt_keys(gateway)
    assert {:error, {:http_status, 503}} = SequencerReceipt.receipt_keys()
    assert_gateway_requests("px_getReceiptKeys", [])
  end

  test "default commits use the configured hosted gateway" do
    gateway = configure_unavailable_endpoints()

    assert {:error, {:http_status, 503}} = SequencerReceipt.commit(1_000, gateway)
    assert {:error, {:http_status, 503}} = SequencerReceipt.commit(1_000)
    assert_gateway_requests("px_getCommit", ["1000"])
  end

  test "default validators use the configured hosted gateway" do
    gateway = configure_unavailable_endpoints()

    assert {:error, {:http_status, 503}} = SequencerReceipt.validators(1_000, gateway)
    assert {:error, {:http_status, 503}} = SequencerReceipt.validators(1_000)
    assert_gateway_requests("px_getValidators", ["1000"])
  end

  defp receipt_config(environment \\ :test) do
    File.cd!(@backend, fn ->
      @runtime
      |> Config.Reader.read!(env: environment, target: :host)
      |> Keyword.fetch!(:explorer)
      |> Keyword.fetch!(SequencerReceipt)
    end)
  end

  defp configure_unavailable_endpoints do
    calls = [
      {"px_getReceiptKeys", []},
      {"px_getCommit", ["1000"]},
      {"px_getValidators", ["1000"]}
    ]

    responses = Map.new(calls, &{&1, {503, nil}})
    node = replay(:node, %{})
    gateway = replay(:gateway, responses) <> "/rpc"
    configure_endpoints(node, gateway)
    gateway
  end

  defp configure_endpoints(node, gateway) do
    System.put_env("PAXEER_X_NODE_RPC_URL", node)
    System.put_env("PAXEER_X_GATEWAY_URL", gateway)
    config = receipt_config()
    assert config == [node_rpc_url: node, gateway_url: gateway, router_url: gateway]
    Application.put_env(:explorer, SequencerReceipt, config)
  end

  defp assert_gateway_requests(method, params) do
    assert_received {:rpc_request, :gateway, ^method, ^params, "/rpc"}
    assert_received {:rpc_request, :gateway, ^method, ^params, "/rpc"}
    refute_received {:rpc_request, _, _, _, _}
  end

  defp recorded_responses(calls) do
    Map.new(calls, fn {method, params, result} -> {{method, params}, {200, result}} end)
  end

  defp replay(producer, responses) do
    ref = {:repair_2231401, make_ref()}

    start_supervised!(
      Supervisor.child_spec(
        {Plug.Cowboy,
         scheme: :http,
         plug: {RecordedRPC, {producer, responses, self()}},
         options: [port: 0, ref: ref, ip: {127, 0, 0, 1}]},
        id: ref
      )
    )

    "http://127.0.0.1:#{:ranch.get_port(ref)}"
  end

  defp rpc(url, method, params) do
    body = Jason.encode!(%{jsonrpc: "2.0", id: 1, method: method, params: params})
    {:ok, %{status_code: 200, body: response}} = HttpClient.post(url, body, [{"Content-Type", "application/json"}])
    %{"result" => result} = Jason.decode!(response)
    {:ok, result}
  end
end
