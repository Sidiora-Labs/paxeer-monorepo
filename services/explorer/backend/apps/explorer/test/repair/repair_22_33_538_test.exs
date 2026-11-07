defmodule Explorer.Repair2233538Test do
  use ExUnit.Case, async: false

  alias EthereumJSONRPC.Block, as: RPCBlock
  alias Explorer.Chain.Block

  @fixtures Path.expand("../support/fixture/vcr_cassettes", __DIR__)
  @historical "block_importer_import_1_pending.json"
  @historical_sha256 "6cbdb078dff803c44b1db8db80c3366391a449099ebdd1ba7050f5614b9a87c0"

  defmodule Transcript do
    @behaviour Plug

    def init(options), do: options

    def call(conn, {exchanges, owner}) do
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      incoming = Jason.decode!(body)

      responses =
        for request <- List.wrap(incoming) do
          exchange =
            Enum.find(exchanges, fn exchange ->
              recorded = Jason.decode!(exchange["request"]["body"])
              Map.take(recorded, ["method", "params"]) == Map.take(request, ["method", "params"])
            end)

          if exchange do
            send(owner, {:replayed, request["method"], request["params"]})
            exchange["response"]["body"] |> Jason.decode!() |> Map.put("id", request["id"])
          else
            send(owner, {:unexpected_request, request})
            raise "request absent from producer transcript"
          end
        end

      response = if is_list(incoming), do: responses, else: hd(responses)

      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.send_resp(200, Jason.encode!(response))
    end
  end

  setup do
    {:ok, _} = Application.ensure_all_started(:plug_cowboy)
    :ok
  end

  test "historical_pending_shape preserves and labels the historical Parity regression" do
    bytes = File.read!(Path.join(@fixtures, @historical))
    assert Base.encode16(:crypto.hash(:sha256, bytes), case: :lower) == @historical_sha256
    provenance = read_json("block_importer_import_1_pending.provenance.json")
    assert provenance["classification"] == "historical Parity evidence"
    assert provenance["merged_pending_conformance"] == false
    assert provenance["sha256"] == @historical_sha256
    assert provenance["cassette"] == @historical

    [exchange] = Jason.decode!(bytes)
    assert Jason.decode!(exchange["request"]["body"])["params"] == ["pending", true]
    {:ok, block} = replay_rpc(exchange)
    assert is_binary(block["hash"])
    assert is_binary(block["stateRoot"])
    assert length(block["transactions"]) == 7

    for {transaction, index} <- Enum.with_index(block["transactions"]) do
      assert transaction["blockHash"] == block["hash"]
      assert transaction["blockNumber"] == block["number"]
      assert EthereumJSONRPC.quantity_to_integer(transaction["transactionIndex"]) == index
    end

    decoded = RPCBlock.to_elixir(block)
    assert decoded["number"] == 954_592
    assert length(decoded["transactions"]) == 7
    assert decoded["hash"] == block["hash"]
    assert decoded["stateRoot"] == block["stateRoot"]
    assert DateTime.to_unix(decoded["timestamp"]) == EthereumJSONRPC.quantity_to_integer(block["timestamp"])
    params = RPCBlock.elixir_to_params(decoded)
    changeset = Block.changeset(%Block{}, Map.put(params, :consensus, true))
    assert changeset.valid?, inspect(changeset.errors)
    assert Ecto.Changeset.get_field(changeset, :number) == decoded["number"]
  end

  # Attempt 2 explicitly permits recording a node's pending-tag refusal. These
  # cases exercise that observed contract; they do not fabricate an open block.
  test "historical_pending_shape rejects pending imports under both fullTx selections" do
    captures = producer_captures!()

    with_transcript(captures, fn arguments, _pool ->
      assert {:ok, "0x7d"} = rpc(phase!(captures, "chain_identity"), arguments)
      assert {:ok, "Geth/linux-amd64/go1.25.6"} = rpc(phase!(captures, "client_version"), arguments)

      for {phase, selection} <- [{"pending_hashes", false}, {"pending_full_transactions", true}] do
        exchange = phase!(captures, phase)
        assert request(exchange)["params"] == ["pending", selection]
        assert {:error, %{code: -32000, message: "pending served by sequencer"}} = rpc(exchange, arguments)
        refute Map.has_key?(Jason.decode!(exchange["response"]["body"]), "result")

        fetched = fetch_recorded_blocks(exchange, arguments)
        assert_refused_import(fetched)
      end

      # The semantic-tag entry point used by explorer follows the same error path.
      assert {:ok, fetched} = EthereumJSONRPC.fetch_block_by_tag("pending", arguments)
      assert_receive {:replayed, "eth_getBlockByNumber", ["pending", false]}
      assert_refused_import(fetched)
    end)
  end

  test "historical_pending_shape decodes one committed identity with both fullTx selections after refusal" do
    captures = producer_captures!()

    with_transcript(captures, fn arguments, _pool ->
      assert_refused_import(fetch_recorded_blocks(phase!(captures, "pending_full_transactions"), arguments))
      assert {:ok, number} = rpc(phase!(captures, "committed_head"), arguments)
      hashes = phase!(captures, "committed_hashes")
      full = phase!(captures, "committed_full_transactions")
      assert request(hashes)["params"] == [number, false]
      assert request(full)["params"] == [number, true]
      assert {:ok, committed_hashes} = rpc(hashes, arguments)
      assert {:ok, committed_full} = rpc(full, arguments)
      assert committed_full["number"] == number
      assert Enum.map(committed_full["transactions"], & &1["hash"]) == committed_hashes["transactions"]
      # The recorded node omits the legacy totalDifficulty field for hashes
      # and supplies exactly zero for full transactions. Pin both wire shapes.
      refute Map.has_key?(committed_hashes, "totalDifficulty")
      assert committed_full["totalDifficulty"] == "0x0"

      assert committed_hashes |> Map.delete("transactions") |> Map.put("totalDifficulty", "0x0") ==
               Map.delete(committed_full, "transactions")

      for key <- ["hash", "stateRoot", "parentHash", "receiptsRoot", "transactionsRoot"] do
        assert Regex.match?(~r/^0x[0-9a-fA-F]{64}$/, committed_full[key])
      end

      decoded = RPCBlock.to_elixir(committed_full)
      assert decoded["number"] == EthereumJSONRPC.quantity_to_integer(number)
      assert decoded["hash"] == committed_full["hash"]
      assert decoded["stateRoot"] == committed_full["stateRoot"]
      assert DateTime.to_unix(decoded["timestamp"]) == EthereumJSONRPC.quantity_to_integer(committed_full["timestamp"])
      assert decoded["gasUsed"] == EthereumJSONRPC.quantity_to_integer(committed_full["gasUsed"])

      for {transaction, index} <- Enum.with_index(decoded["transactions"]) do
        assert transaction["blockHash"] == committed_full["hash"]
        assert transaction["blockNumber"] == decoded["number"]
        assert transaction["transactionIndex"] == index
      end

      fetched = fetch_recorded_blocks(full, arguments)
      assert fetched.errors == []
      assert [params] = fetched.blocks_params
      changeset = Block.changeset(%Block{}, Map.put(params, :consensus, true))
      assert changeset.valid?, inspect(changeset.errors)
      assert Ecto.Changeset.get_field(changeset, :number) == decoded["number"]

      assert Ecto.Changeset.get_field(changeset, :hash).bytes ==
               Base.decode16!(String.slice(committed_full["hash"], 2..-1//1), case: :mixed)

      assert {:ok, multi} =
               Explorer.Chain.Import.all_single_multi([Explorer.Chain.Import.Runner.Blocks], %{
                 blocks: %{params: [Map.put(params, :consensus, true)]}
               })

      assert {:run, _} = Keyword.fetch!(Ecto.Multi.to_list(multi), :blocks)
    end)
  end

  test "historical_pending_shape retains committed identity and pending refusal after transport restart" do
    captures = producer_captures!()

    with_transcript(captures, fn arguments, pool ->
      before_restart =
        for selection <- ["hashes", "full_transactions"] do
          fetch_recorded_blocks(phase!(captures, "committed_#{selection}"), arguments)
        end

      assert_refused_import(fetch_recorded_blocks(phase!(captures, "pending_hashes"), arguments))
      old_pool = :hackney_pool.find_pool(pool)
      assert is_pid(old_pool)
      assert Process.alive?(old_pool)
      assert :ok = :hackney_pool.stop_pool(pool)
      refute Process.alive?(old_pool)
      assert :ok = :hackney_pool.start_pool(pool, [])
      new_pool = :hackney_pool.find_pool(pool)
      assert Process.alive?(new_pool)
      refute new_pool == old_pool

      for {selection, expected} <- Enum.zip(["hashes", "full_transactions"], before_restart) do
        recovered = fetch_recorded_blocks(phase!(captures, "committed_#{selection}"), arguments)
        assert recovered == expected
        assert recovered.errors == []
        assert [_] = recovered.blocks_params
      end

      for selection <- ["hashes", "full_transactions"] do
        assert_refused_import(fetch_recorded_blocks(phase!(captures, "pending_#{selection}"), arguments))
      end
    end)
  end

  defp assert_refused_import(fetched) do
    assert fetched.errors == [%{code: -32000, message: "pending served by sequencer", data: %{tag: "pending"}}]
    assert fetched.blocks_params == []
    assert fetched.transactions_params == []
    assert fetched.block_second_degree_relations_params == []
    assert fetched.withdrawals_params == []

    # The real import planner omits empty runners. No database is needed to
    # prove that a refused response cannot schedule a block or transaction write.
    assert {:ok, multi} =
             Explorer.Chain.Import.all_single_multi(
               [Explorer.Chain.Import.Runner.Blocks, Explorer.Chain.Import.Runner.Transactions],
               %{blocks: %{params: fetched.blocks_params}, transactions: %{params: fetched.transactions_params}}
             )

    assert Ecto.Multi.to_list(multi) == []
  end

  defp read_json(name), do: @fixtures |> Path.join(name) |> File.read!() |> Jason.decode!()

  defp producer_captures! do
    corpus = read_json("paxeer_pending_block_lifecycle.json")
    assert corpus["classification"] == "chain-125 node pending-tag refusal and committed block evidence"
    assert corpus["pending_outcome"] == "refused"
    assert corpus["source_revision"] == nil
    provenance = read_json("block_importer_import_1_pending.provenance.json")
    bytes = File.read!(Path.join(@fixtures, provenance["required_producer_capture"]))
    assert Base.encode16(:crypto.hash(:sha256, bytes), case: :lower) == provenance["producer_capture_sha256"]
    assert Enum.map(corpus["exchanges"], & &1["phase"]) == provenance["required_phases"]
    assert Enum.all?(corpus["exchanges"], &(&1["response"]["status_code"] == 200))
    corpus["exchanges"]
  end

  defp phase!(captures, name) do
    [exchange] = Enum.filter(captures, &(&1["phase"] == name))
    exchange
  end

  defp request(exchange), do: Jason.decode!(exchange["request"]["body"])

  defp rpc(exchange, arguments) do
    recorded = request(exchange)
    result = EthereumJSONRPC.json_rpc(rpc_request(recorded), arguments)
    assert_replayed(recorded)
    result
  end

  defp rpc_request(recorded) do
    EthereumJSONRPC.request(%{id: recorded["id"], method: recorded["method"], params: recorded["params"]})
  end

  defp fetch_recorded_blocks(exchange, arguments) do
    recorded = request(exchange)
    [tag_or_number, _selection] = recorded["params"]

    params =
      if tag_or_number == "pending",
        do: %{tag: "pending"},
        else: %{number: EthereumJSONRPC.quantity_to_integer(tag_or_number)}

    assert {:ok, responses} = EthereumJSONRPC.json_rpc([rpc_request(recorded)], arguments)
    assert_replayed(recorded)
    EthereumJSONRPC.Blocks.from_responses(responses, %{recorded["id"] => params})
  end

  defp assert_replayed(recorded) do
    method = recorded["method"]
    params = recorded["params"]
    assert_receive {:replayed, ^method, ^params}
    refute_received {:unexpected_request, _}
  end

  defp replay_rpc(exchange), do: with_transcript([exchange], fn arguments, _pool -> rpc(exchange, arguments) end)

  defp with_transcript(exchanges, fun) do
    ref = {__MODULE__, make_ref()}
    pool = {__MODULE__, make_ref()}

    pid =
      start_supervised!(
        {Plug.Cowboy,
         scheme: :http, plug: {Transcript, {exchanges, self()}}, options: [ip: {127, 0, 0, 1}, port: 0, ref: ref]},
        id: ref
      )

    assert Process.alive?(pid)
    assert :ok = :hackney_pool.start_pool(pool, [])
    url = "http://127.0.0.1:#{:ranch.get_port(ref)}/"

    arguments = [
      transport: EthereumJSONRPC.HTTP,
      transport_options: [
        http: EthereumJSONRPC.HTTP.HTTPoison,
        urls: [url],
        http_options: [timeout: 5_000, recv_timeout: 5_000, pool: pool]
      ]
    ]

    try do
      fun.(arguments, pool)
    after
      :hackney_pool.stop_pool(pool)
      stop_supervised!(ref)
      refute Process.alive?(pid)
    end
  end
end
