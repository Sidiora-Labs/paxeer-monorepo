defmodule Indexer.Repair2233403Test do
  use Explorer.DataCase, async: false

  import ExUnit.CaptureLog

  alias ABI.TypeEncoder
  alias Explorer.Chain
  alias Explorer.Chain.Data
  alias Explorer.Chain.Optimism.InteropMessage
  alias Indexer.Fetcher.Optimism.Interop.{Message, MessageFailed, MessageQueue, MultichainExport}

  @private_key <<1::256>>
  @api_key "repair-2233403-private-export-credential"
  @moduletag :capture_log

  setup do
    # Shadow the optional chain-specific table on the sandbox connection so the
    # same production schema/import runner is exercised by the default profile.
    Ecto.Adapters.SQL.query!(
      Repo,
      """
      CREATE TEMP TABLE op_interop_messages (
        sender_address_hash bytea, target_address_hash bytea, nonce bigint NOT NULL,
        init_chain_id integer NOT NULL, init_transaction_hash bytea, block_number bigint,
        timestamp timestamp, relay_chain_id integer NOT NULL, relay_transaction_hash bytea,
        payload bytea, failed boolean, transfer_token_address_hash bytea,
        transfer_from_address_hash bytea, transfer_to_address_hash bytea, transfer_amount numeric,
        sent_to_multichain boolean, inserted_at timestamp NOT NULL, updated_at timestamp NOT NULL,
        PRIMARY KEY (nonce, init_chain_id)
      )
      """,
      []
    )

    old_chain = Application.get_env(:explorer, :chain_type)
    old_queue = Application.get_env(:indexer, MessageQueue)
    Application.put_env(:explorer, :chain_type, :optimism)
    Application.put_env(:indexer, MessageQueue, private_key: hex(@private_key))

    on_exit(fn ->
      Application.put_env(:explorer, :chain_type, old_chain)

      if is_nil(old_queue),
        do: Application.delete_env(:indexer, MessageQueue),
        else: Application.put_env(:indexer, MessageQueue, old_queue)
    end)

    :ok
  end

  test "37: ABI failed relay RPC data reaches import and replays after reopening" do
    inputs = [relay_calldata(1), String.upcase(relay_calldata(2)) |> String.replace_prefix("0X", "0x")]
    {args, transcript} = rpc_peer(inputs)
    pid = open_fetcher(MessageFailed, failed_state(args, 100))
    send(pid, :continue)
    assert :sys.get_state(pid, 10_000).mode == :realtime
    assert_failed_rows([1, 2])
    GenServer.stop(pid)

    assert {:ok, recovered_block} = Message.get_last_block_number(args, 10, true)
    assert recovered_block == 100
    reopened = open_fetcher(MessageFailed, failed_state(args, recovered_block))
    send(reopened, :continue)
    assert :sys.get_state(reopened, 10_000).mode == :realtime
    assert_failed_rows([1, 2])
    assert Agent.get(transcript, & &1.block_reads) == 2
    assert Agent.get(transcript, & &1.receipt_reads) == 4
    assert Agent.get(transcript, & &1.transaction_reads) == 1
    GenServer.stop(reopened)
  end

  test "37: malformed failed relay calldata is rejected without blocking valid imports" do
    valid = relay_calldata(7)
    <<_::binary-size(10), encoded::binary>> = valid
    {:ok, raw} = Base.decode16(encoded, case: :mixed)
    <<head::binary-size(160), _offset::256, tail::binary>> = raw
    bad_offset = "0x8d1d298f" <> Base.encode16(head <> <<999_999::256>> <> tail, case: :lower)
    <<head::binary-size(192), _length::256, tail::binary>> = raw
    bad_length = "0x8d1d298f" <> Base.encode16(head <> <<999_999::256>> <> tail, case: :lower)

    malformed = [
      "0x8d1d298f",
      "0x8d1d298f0",
      "0x8d1d298fzz",
      String.slice(valid, 0..-3//1),
      bad_offset,
      bad_length,
      relay_calldata(8, 10, 23, <<0::256>>),
      relay_calldata(9, 10, 23, nil, <<1::256>>),
      relay_calldata(10, 10, 23, nil, <<0::96, 1::160, 999_999::256, 0::256>>)
    ]

    {args, transcript} = rpc_peer(malformed ++ [valid, relay_calldata(11, 11), relay_calldata(12, 10, 2_147_483_648)])
    pid = open_fetcher(MessageFailed, failed_state(args, 100))
    send(pid, :continue)
    assert :sys.get_state(pid, 10_000).mode == :realtime
    assert_failed_rows([7])
    assert Agent.get(transcript, & &1.receipt_reads) == length(malformed) + 3
    GenServer.stop(pid)
  end

  test "38: real queue transport and import reject ambiguous relay numeric substitutions" do
    import_message(%{
      nonce: 1,
      init_chain_id: 23,
      relay_chain_id: 10,
      block_number: 100,
      relay_transaction_hash: hash(500),
      failed: true
    })

    {peer, requests} = import_peer()
    run_queue(peer)
    [wire] = Agent.get(requests, & &1)
    assert wire["signature_version"] == 2
    assert wire["nonce"] == 1
    assert wire["init_chain_id"] == 23
    assert wire["relay_transaction_hash"] == hash(500)
    assert InteropMessage.get_message(23, 1).failed

    expected =
      TypeEncoder.encode(
        [ExKeccak.hash_256("blockscout.optimism.interop"), 2, 1, 1, 23, 10, <<500::256>>, true],
        [{:bytes, 32}, {:uint, 8}, {:uint, 8}, {:uint, 256}, {:uint, 256}, {:uint, 256}, {:bytes, 32}, :bool]
      )

    assert {:ok, ^expected} = MessageQueue.signing_payload(wire)
    assert_accepted(peer, wire)

    substituted = %{wire | "nonce" => 12, "init_chain_id" => 3}
    assert legacy_relay_bytes(wire) == legacy_relay_bytes(substituted)
    assert_rejected(peer, substituted)
    assert InteropMessage.get_message(3, 12) == nil

    assert_rejected(peer, Map.put(wire, "signature_version", 1))
    assert_rejected(peer, Map.delete(wire, "signature_version"))
    assert_rejected(peer, Map.put(wire, "nonce", "1"))
    assert_rejected(peer, Map.put(wire, "failed", false))
    assert_rejected(peer, Map.put(wire, "relay_chain_id", 11))
    assert_rejected(peer, Map.put(wire, "relay_transaction_hash", hash(501)))
    assert_rejected(peer, Map.put(wire, "init_transaction_hash", hash(502)))

    {:ok, {legacy_signature, _}} = ExSecp256k1.sign_compact(ExKeccak.hash_256(legacy_relay_bytes(wire)), @private_key)
    assert_rejected(peer, Map.put(wire, "signature", hex(legacy_signature)))
    assert Repo.aggregate(InteropMessage, :count) == 1
  end

  test "38: outgoing queue signs typed init data across JSON and rejects timestamp chain substitution" do
    import_message(%{
      nonce: 2,
      init_chain_id: 10,
      relay_chain_id: 23,
      block_number: 100,
      sender_address_hash: address(1),
      target_address_hash: address(2),
      init_transaction_hash: hash(600),
      timestamp: DateTime.from_unix!(1),
      payload: %Data{bytes: <<0, 255, 1>>}
    })

    {peer, requests} = import_peer()
    run_queue(peer)
    [wire] = Agent.get(requests, & &1)
    assert wire["payload"] == "0x00ff01"
    assert wire["timestamp"] == 1
    assert wire["signature_version"] == 2
    assert_accepted(peer, wire)
    assert InteropMessage.get_message(10, 2).payload == %Data{bytes: <<0, 255, 1>>}

    substituted = %{wire | "timestamp" => 12, "relay_chain_id" => 3}

    assert "#{wire["timestamp"]}#{wire["relay_chain_id"]}" ==
             "#{substituted["timestamp"]}#{substituted["relay_chain_id"]}"

    assert_rejected(peer, substituted)
    assert_rejected(peer, Map.put(wire, "payload", "0x00ff02"))
    assert_rejected(peer, Map.put(wire, "sender_address_hash", address(3)))
    assert InteropMessage.get_message(10, 2).relay_chain_id == 23
  end

  test "39: failed multichain HTTP export retains retry state and never logs credentials or response objects" do
    import_message(%{
      nonce: 1,
      init_chain_id: 23,
      relay_chain_id: 10,
      block_number: 100,
      relay_transaction_hash: hash(700),
      failed: true
    })

    peer = Bypass.open()
    parent = self()

    Bypass.expect_once(peer, "POST", "/batch-import", fn conn ->
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      decoded = Jason.decode!(body)
      send(parent, {:export_request, decoded})
      # A transport response can itself contain secrets; neither it nor the
      # request body may be inspected in the error log.
      Plug.Conn.resp(conn, 503, Jason.encode!(%{api_key: decoded["api_key"], request: decoded}))
    end)

    state = export_state(url(peer) <> "/batch-import")
    log = capture_log(fn -> assert {:noreply, ^state} = MultichainExport.handle_info(:continue, state) end)
    assert_received {:export_request, request}
    assert request["api_key"] == @api_key
    assert request["chain_id"] == "10"
    assert [%{"relay" => relay}] = request["interop_messages"]
    assert relay["relay_transaction_hash"] == hash(700)
    assert relay["failed"]
    assert log =~ "chain_id=10 items=1 status=http_503"
    refute log =~ @api_key
    refute log =~ "api_key"
    refute log =~ hash(700)
    assert InteropMessage.get_message(23, 1).sent_to_multichain == nil
  end

  test "39: failed multichain transport logs only request identity and safe status" do
    import_message(%{
      nonce: 1,
      init_chain_id: 23,
      relay_chain_id: 10,
      block_number: 100,
      relay_transaction_hash: hash(701),
      failed: true
    })

    peer = Bypass.open()
    Bypass.down(peer)
    state = export_state(url(peer) <> "/#{@api_key}")
    log = capture_log(fn -> assert {:noreply, ^state} = MultichainExport.handle_info(:continue, state) end)
    assert log =~ "chain_id=10 items=1 status=transport_error"
    refute log =~ @api_key
    refute log =~ "api_key"
    refute log =~ "Tesla.Env"
    assert InteropMessage.get_message(23, 1).sent_to_multichain == nil
  end

  defp rpc_peer(inputs) do
    peer = Bypass.open()
    block = recorded_response("block_importer_import_1_pending.json")
    receipt = recorded_response("transaction_importer_import_1_receipt.json")
    prototype = hd(block["transactions"])

    transactions =
      Enum.with_index(inputs, 1000)
      |> Enum.map(fn {input, index} ->
        Map.merge(prototype, %{
          "hash" => hash(index),
          "blockNumber" => "0x64",
          "blockHash" => hash(100),
          "transactionIndex" => EthereumJSONRPC.integer_to_quantity(index - 1000),
          "to" => Message.l2tol2_cross_domain_messenger(),
          "input" => input
        })
      end)

    block = Map.merge(block, %{"hash" => hash(100), "number" => "0x64", "transactions" => transactions})
    {:ok, transcript} = Agent.start_link(fn -> %{block_reads: 0, receipt_reads: 0, transaction_reads: 0} end)

    Bypass.expect(peer, "POST", "/", fn conn ->
      {:ok, body, conn} = Plug.Conn.read_body(conn)

      decode = fn request ->
        result =
          case {request["method"], request["params"]} do
            {"eth_getBlockByNumber", ["0x64", true]} ->
              Agent.update(transcript, &Map.update!(&1, :block_reads, fn n -> n + 1 end))
              block

            {"eth_getTransactionReceipt", [tx_hash]} ->
              Agent.update(transcript, &Map.update!(&1, :receipt_reads, fn n -> n + 1 end))
              tx = Enum.find(transactions, &(&1["hash"] == tx_hash))

              Map.merge(receipt, %{
                "transactionHash" => tx_hash,
                "transactionIndex" => tx["transactionIndex"],
                "blockNumber" => "0x64",
                "blockHash" => hash(100),
                "status" => "0x0",
                "logs" => []
              })

            {"eth_getTransactionByHash", [tx_hash]} ->
              Agent.update(transcript, &Map.update!(&1, :transaction_reads, fn n -> n + 1 end))
              Enum.find(transactions, &(&1["hash"] == tx_hash))
          end

        %{jsonrpc: "2.0", id: request["id"], result: result}
      end

      request = Jason.decode!(body)
      response = if is_list(request), do: Enum.map(request, decode), else: decode.(request)
      Plug.Conn.resp(conn, 200, Jason.encode!(response))
    end)

    args = [
      transport: EthereumJSONRPC.HTTP,
      transport_options: [http: EthereumJSONRPC.HTTP.HTTPoison, http_options: [], urls: [url(peer)]],
      variant: EthereumJSONRPC.Geth
    ]

    {args, transcript}
  end

  defp relay_calldata(nonce, chain_id \\ 10, init_id \\ 23, signature \\ nil, data \\ nil) do
    signature = signature || Explorer.Helper.hash_to_binary(Message.sent_message_event_signature())
    topics = TypeEncoder.encode([chain_id, <<2::160>>, nonce], [{:uint, 256}, :address, {:uint, 256}])
    data = data || TypeEncoder.encode([<<1::160>>, <<0, 255, 1>>], [:address, :bytes])
    sent_message = signature <> topics <> data
    types = [{:tuple, [:address, {:uint, 256}, {:uint, 256}, {:uint, 256}, {:uint, 256}]}, :bytes]

    "0x8d1d298f" <>
      Base.encode16(
        TypeEncoder.encode(
          [
            {Explorer.Helper.hash_to_binary(Message.l2tol2_cross_domain_messenger()), 99, 0, 1_700_000_000, init_id},
            sent_message
          ],
          types
        ),
        case: :lower
      )
  end

  defp assert_failed_rows(nonces) do
    rows = Repo.all(from(m in InteropMessage, order_by: m.nonce))
    assert Enum.map(rows, & &1.nonce) == nonces
    assert Enum.all?(rows, &(&1.init_chain_id == 23 and &1.relay_chain_id == 10 and &1.failed))
    assert Enum.all?(rows, &(&1.block_number == 100))
    assert Enum.all?(rows, &(not is_nil(&1.relay_transaction_hash)))
  end

  defp failed_state(args, start_block) do
    %{
      start_block_number: start_block,
      end_block_number: 100,
      chunk_size: 1,
      mode: :catchup,
      realtime_range: nil,
      last_realtime_block_number: nil,
      json_rpc_named_arguments: args,
      chain_id: 10
    }
  end

  defp open_fetcher(module, state) do
    pid = :proc_lib.spawn(fn -> :gen_server.enter_loop(module, [], state) end)
    on_exit(fn -> if Process.alive?(pid), do: GenServer.stop(pid) end)
    pid
  end

  defp import_message(params) do
    assert InteropMessage.changeset(%InteropMessage{}, params).valid?
    assert {:ok, _} = Chain.import(%{optimism_interop_messages: %{params: [params]}, broadcast: false})
  end

  defp import_peer do
    peer = Bypass.open()
    {:ok, public_key} = ExSecp256k1.create_public_key(@private_key)
    {:ok, requests} = Agent.start_link(fn -> [] end)

    # A loopback peer runs the production v2 verifier, schema and import runner.
    # Wiring this codec into the web controller is assigned in the task record.
    Bypass.expect(peer, "POST", "/api/v2/import/optimism/interop/", fn conn ->
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      data = Jason.decode!(body)

      case MessageQueue.verify_post_data(data, public_key) do
        :ok ->
          params = Map.drop(data, ["signature", "signature_version"])

          params =
            if Map.has_key?(params, "timestamp"),
              do: Map.update!(params, "timestamp", &DateTime.from_unix!/1),
              else: params

          assert InteropMessage.changeset(%InteropMessage{}, params).valid?
          assert {:ok, _} = Chain.import(%{optimism_interop_messages: %{params: [params]}, broadcast: false})
          Agent.update(requests, &(&1 ++ [data]))

          response =
            if Map.has_key?(data, "init_transaction_hash"),
              do: InteropMessage.get_relay_part(data["init_chain_id"], data["nonce"]),
              else: InteropMessage.get_init_part(data["init_chain_id"], data["nonce"])

          Plug.Conn.resp(conn, 200, Jason.encode!(response))

        {:error, :invalid_signature} ->
          Plug.Conn.resp(conn, 401, Jason.encode!(%{message: "Wrong signature"}))
      end
    end)

    {peer, requests}
  end

  defp run_queue(peer) do
    state = %{
      chain_id: 10,
      chainscout_api_url: nil,
      chainscout_map: %{23 => url(peer)},
      timeout: 1000,
      recv_timeout: 1000,
      export_expiration_blocks: 100,
      iterations_done: 0,
      json_rpc_named_arguments: []
    }

    assert {:noreply, updated} = MessageQueue.handle_info(:continue, state)
    assert updated.iterations_done == 1
  end

  defp assert_accepted(peer, data), do: assert(peer_post(peer, data).status == 200)
  defp assert_rejected(peer, data), do: assert(peer_post(peer, data).status == 401)

  defp peer_post(peer, data) do
    client = Tesla.client([], Tesla.Adapter.Mint)

    assert {:ok, response} =
             Tesla.post(client, url(peer) <> "/api/v2/import/optimism/interop/", Jason.encode!(data),
               headers: [{"content-type", "application/json"}]
             )

    response
  end

  defp legacy_relay_bytes(data),
    do:
      "#{data["nonce"]}#{data["init_chain_id"]}#{data["relay_chain_id"]}#{data["relay_transaction_hash"]}#{data["failed"]}"

  defp export_state(endpoint),
    do: %{chain_id: 10, multichain_api_url: endpoint, multichain_api_key: @api_key, batch_size: 10}

  defp recorded_response(name) do
    path = Path.expand("../../../explorer/test/support/fixture/vcr_cassettes/#{name}", __DIR__)
    [record] = path |> File.read!() |> Jason.decode!()
    record["response"]["body"] |> Jason.decode!() |> Map.fetch!("result")
  end

  defp url(peer), do: "http://127.0.0.1:#{peer.port}"
  defp hex(bytes), do: "0x" <> Base.encode16(bytes, case: :lower)
  defp hash(number), do: hex(<<number::256>>)
  defp address(number), do: hex(<<number::160>>)
end
