defmodule Indexer.Repair2233548Test do
  @moduledoc """
  Task 22.33.548 repair record: anytrust-keyset-decode (audit 22.30.157).

  Proving cases: partial RPC data recovers without poisoning the cache; malformed
  keysets never reach the import runner; malformed ABI events yield typed errors;
  boundary committees retain signer order and encoding; chunk errors propagate.
  The committed Nova discovery snapshot supplies the on-chain committee keys.
  ABI.TypeEncoder serializes those keys into SetValidKeyset event payloads for a
  loopback JSON-RPC server; corrupt cases mutate that serialization deliberately.
  Certificate envelopes exercise both supported wire versions, without claiming
  cryptographic signature verification, which this importer does not perform.

  Wiring request: services/explorer/backend/apps/indexer/lib/indexer/fetcher/arbitrum/workers/batches/discovery.ex,
  domain 14: handle Common.prepare_for_import/2's {:error, {:anytrust_keyset, hash, reason}}
  by retaining retry eligibility before importing batches (owned by task 22.33.547).

  Baseline verification was not run: the explicit gate order requires the entire
  implementation before hf-build and hf-done and prohibits a manual verify run.
  No baseline test failure is claimed. The runner's gate ledger records the
  verified revision, command, exit code and private evidence location.
  """
  use ExUnit.Case, async: false

  alias Ecto.Adapters.SQL
  alias Explorer.Chain.Arbitrum.{BatchToDaBlob, DaMultiPurposeRecord}
  alias Explorer.Chain.Import.Runner.Arbitrum.DaMultiPurposeRecords
  alias Explorer.Repo
  alias Indexer.Fetcher.Arbitrum.DA.{Anytrust, Common}

  @fixture Path.expand("../../lib/indexer/fetcher/arbitrum/da/fixtures/nova_keyset.json", __DIR__)
  @event_topic "0xabca9b7986bc22ad0160eb0cb88ae75411eacfba4052af0b457a9335ef655722"
  @block 15_016_829

  setup do
    :ok = SQL.Sandbox.checkout(Repo)
    SQL.Sandbox.mode(Repo, {:shared, self()})

    # Match AddDaInfo's schema in a transaction-local table on every chain type.
    SQL.query!(Repo, """
    CREATE TEMPORARY TABLE arbitrum_da_multi_purpose (
      data_key bytea PRIMARY KEY NOT NULL,
      data_type integer NOT NULL,
      data jsonb NOT NULL,
      batch_number integer,
      inserted_at timestamp(6) NOT NULL,
      updated_at timestamp(6) NOT NULL
    ) ON COMMIT DROP
    """)

    snapshot = @fixture |> File.read!() |> Jason.decode!()
    committee = snapshot["dacKeyset"]
    keys = Enum.map(committee["blsSignatures"], &Base.decode64!/1)
    threshold = committee["membersCount"] + 1 - committee["requiredSignatures"]
    raw = keyset(threshold, keys)
    hash = :crypto.hash(:sha256, raw)
    server = Bypass.open()
    state = start_supervised!({Agent, fn -> %{response: %{"result" => []}, requests: []} end})
    address = snapshot["address"]

    Bypass.expect(server, "POST", "/", fn conn ->
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      request = Jason.decode!(body)
      respond = &rpc_response(&1, state, address, hash)
      response = if is_list(request), do: Enum.map(request, respond), else: respond.(request)

      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.resp(200, Jason.encode!(response))
    end)

    url = "http://127.0.0.1:#{server.port}"

    config = %{
      sequencer_inbox_address: address,
      json_rpc_named_arguments: [
        variant: EthereumJSONRPC.Geth,
        transport: EthereumJSONRPC.HTTP,
        transport_options: [
          http: EthereumJSONRPC.HTTP.HTTPoison,
          urls: [url],
          fallback_urls: [],
          http_options: [timeout: 1_000, recv_timeout: 1_000]
        ]
      ]
    }

    %{state: state, config: config, raw: raw, hash: hash, keys: keys, threshold: threshold}
  end

  test "anytrust-keyset-decode: partial RPC data recovers without poisoning the cache", context do
    info = certificate(context.hash)
    cache = MapSet.new()
    accumulator = {%{}, []}

    for {response, reason} <- [
          {%{"result" => []}, :keyset_not_found},
          {%{"result" => nil}, :invalid_keyset_event},
          {%{"result" => [%{"data" => "0x"}]}, :invalid_keyset_event},
          {event_response(binary_part(context.raw, 0, 16)), :truncated_public_keys}
        ] do
      respond_with(context, response)

      assert {:error, {:anytrust_keyset, hash, ^reason}} =
               Anytrust.prepare_for_import(accumulator, info, context.config, cache)

      assert hash == context.hash
      assert cache == MapSet.new()
      assert accumulator == {%{}, []}
      assert Repo.aggregate(DaMultiPurposeRecord, :count) == 0
    end

    respond_with(context, event_response(context.raw))

    assert {{records, [association]}, recovered_cache} =
             Anytrust.prepare_for_import(accumulator, info, context.config, cache)

    assert recovered_cache == MapSet.new([context.hash])
    assert Map.fetch!(records, context.hash).data == expected_keyset(context.threshold, context.keys)
    assert BatchToDaBlob.changeset(%BatchToDaBlob{}, association).valid?
    assert length(requests(context, "eth_getLogs")) == 5

    assert {{^records, [^association, ^association]}, ^recovered_cache} =
             Anytrust.prepare_for_import({records, [association]}, info, context.config, recovered_cache)

    assert length(requests(context, "eth_getLogs")) == 5
    import_records(Map.values(records))
    assert Repo.aggregate(DaMultiPurposeRecord, :count) == 2
    stored = Repo.get_by!(DaMultiPurposeRecord, data_key: context.hash)
    assert stored.data["threshold"] == context.threshold

    assert stored.data["pubkeys"] ==
             Jason.decode!(Jason.encode!(expected_keyset(context.threshold, context.keys).pubkeys))

    refute Enum.any?(stored.data["pubkeys"], &(&1 == "error"))

    assert {{existing_records, [_]}, _} =
             Anytrust.prepare_for_import(accumulator, info, context.config, MapSet.new())

    assert Map.keys(existing_records) == [info.data_hash]
    assert length(requests(context, "eth_getLogs")) == 5
  end

  test "anytrust-keyset-decode: malformed keysets never reach the import runner", context do
    [<<96, proof::binary-size(96), public_key::binary-size(192)>> | _] = context.keys

    malformed = [
      {<<>>, :invalid_keyset_header},
      {<<1::64>>, :invalid_keyset_header},
      {<<1::64, 0::64>>, :invalid_keyset_header},
      {<<1::64, 65::64>>, :invalid_keyset_header},
      {keyset(0, context.keys), :invalid_keyset_header},
      {keyset(7, context.keys), :invalid_keyset_header},
      {<<1::64, 1::64>>, :truncated_public_keys},
      {<<1::64, 1::64, 1>>, :truncated_public_keys},
      {<<1::64, 1::64, 289::16, 96>>, :truncated_public_keys},
      {keyset(1, [<<>>]), :invalid_public_key},
      {keyset(1, [<<0>>]), :invalid_public_key},
      {keyset(1, [<<0, public_key::binary-size(191)>>]), :invalid_public_key},
      {keyset(1, [<<0, public_key::binary, 0>>]), :invalid_public_key},
      {keyset(1, [<<96, proof::binary-size(95)>>]), :invalid_public_key},
      {keyset(1, [<<95, proof::binary, public_key::binary>>]), :invalid_public_key},
      {keyset(1, [<<96, proof::binary, public_key::binary-size(191)>>]), :invalid_public_key},
      {<<1::64, 2::64, 289::16, hd(context.keys)::binary>>, :truncated_public_keys},
      {context.raw <> <<0>>, :trailing_keyset_data}
    ]

    for {raw, reason} <- malformed do
      respond_with(context, event_response(raw))

      assert {:error, {:anytrust_keyset, hash, ^reason}} =
               Common.prepare_for_import([certificate(context.hash)], context.config)

      assert hash == context.hash
      assert Repo.aggregate(DaMultiPurposeRecord, :count) == 0
    end

    assert length(requests(context, "eth_getLogs")) == length(malformed)
    respond_with(context, event_response(context.raw))
    assert {records, [_]} = Common.prepare_for_import([certificate(context.hash)], context.config)
    import_records(records)
    assert Repo.aggregate(DaMultiPurposeRecord, :count) == 2
  end

  test "anytrust-keyset-decode: malformed ABI events yield typed errors and permit retry", context do
    for data <- [
          nil,
          "0x",
          "0xz0",
          "0x0",
          hex(<<0::256, 0::256>>),
          hex(<<32::256, 18_641::256>>),
          hex(<<32::256, 1::256, 0>>),
          hex(<<32::256, 1::256, 0, 1::248>>)
        ] do
      respond_with(context, %{"result" => [%{"data" => data}]})

      assert {:error, {:anytrust_keyset, _, :invalid_keyset_event}} =
               Common.prepare_for_import([certificate(context.hash)], context.config)

      assert Repo.aggregate(DaMultiPurposeRecord, :count) == 0
    end

    respond_with(context, event_response(context.raw))
    assert {records, [_]} = Common.prepare_for_import([certificate(context.hash)], context.config)
    import_records(records)
    assert Repo.aggregate(DaMultiPurposeRecord, :count) == 2
  end

  test "anytrust-keyset-decode: boundary committees retain signer order and encoding", context do
    [<<96, _proof::binary-size(96), public_key::binary-size(192)>> | _] = context.keys
    trusted = <<0, public_key::binary>>
    committees = [{1, [trusted]}, {1, context.keys}, {6, context.keys}, {64, List.duplicate(trusted, 64)}]

    for {threshold, keys} <- committees do
      respond_with(context, event_response(keyset(threshold, keys)))
      info = certificate(context.hash, :legacy)
      assert {{records, [_]}, cache} = Anytrust.prepare_for_import({%{}, []}, info, context.config, MapSet.new())
      assert MapSet.member?(cache, context.hash)
      assert Map.fetch!(records, context.hash).data == expected_keyset(threshold, keys)
      assert Enum.all?(Map.values(records), &DaMultiPurposeRecord.changeset(%DaMultiPurposeRecord{}, &1).valid?)
    end
  end

  test "anytrust-keyset-decode: chunk errors propagate and successful keysets are fetched once", context do
    info = certificate(context.hash)
    respond_with(context, event_response(context.raw))
    assert {records, associations} = Common.prepare_for_import([info, %{info | batch_number: 43}], context.config)
    assert length(records) == 2
    assert Enum.sort(Enum.map(associations, & &1.batch_number)) == [42, 43]
    assert length(requests(context, "eth_getLogs")) == 1

    respond_with(context, %{"result" => []})

    assert {:error, {:anytrust_keyset, _, :keyset_not_found}} =
             Common.prepare_for_import([info, %{info | batch_number: 43}], context.config)

    assert length(requests(context, "eth_getLogs")) == 2
    assert Repo.aggregate(DaMultiPurposeRecord, :count) == 0
  end

  test "anytrust-keyset-decode: exhausted log RPC errors remain typed and recover", context do
    respond_with(context, %{"error" => %{"code" => -32_005, "message" => "recorded response incomplete"}})

    assert {:error, {:anytrust_keyset, _, {:keyset_rpc, reason}}} =
             Common.prepare_for_import([certificate(context.hash)], context.config)

    assert reason[:code] == -32_005
    assert length(requests(context, "eth_getLogs")) == 3
    assert Repo.aggregate(DaMultiPurposeRecord, :count) == 0

    respond_with(context, event_response(context.raw))
    assert {records, [_]} = Common.prepare_for_import([certificate(context.hash)], context.config)
    import_records(records)
    assert Repo.aggregate(DaMultiPurposeRecord, :count) == 2
  end

  defp keyset(threshold, keys) do
    encoded = Enum.map(keys, fn key -> <<byte_size(key)::16, key::binary>> end)
    <<threshold::64, length(keys)::64, IO.iodata_to_binary(encoded)::binary>>
  end

  defp expected_keyset(threshold, keys) do
    pubkeys =
      Enum.map(keys, fn
        <<0, public_key::binary>> -> %{trusted: true, key: hex(public_key)}
        <<96, proof::binary-size(96), public_key::binary>> -> %{trusted: false, proof: hex(proof), key: hex(public_key)}
      end)

    %{threshold: threshold, pubkeys: pubkeys}
  end

  defp certificate(hash, version \\ :current) do
    tail = <<hash::binary, :crypto.hash(:sha256, "Nova DA certificate")::binary, 2_000_000_000::64>>
    version_byte = if version == :current, do: <<1>>, else: <<>>
    wire = <<136, tail::binary, version_byte::binary, 63::64, 0::768>>
    assert {:ok, :in_anytrust, info} = Common.examine_batch_accompanying_data(42, wire)
    info
  end

  defp event_response(raw) do
    %{
      "result" => [
        %{
          "data" => raw |> List.wrap() |> ABI.TypeEncoder.encode([:bytes]) |> hex(),
          "topics" => [@event_topic],
          "blockNumber" => EthereumJSONRPC.integer_to_quantity(@block)
        }
      ]
    }
  end

  defp respond_with(context, response), do: Agent.update(context.state, &%{&1 | response: response})

  defp requests(context, method) do
    Agent.get(context.state, fn state -> Enum.filter(state.requests, &(&1["method"] == method)) end)
  end

  defp rpc_response(
         %{"jsonrpc" => "2.0", "id" => id, "method" => method, "params" => params} = request,
         state,
         address,
         hash
       ) do
    response =
      Agent.get_and_update(state, fn current ->
        {current.response, %{current | requests: [request | current.requests]}}
      end)

    result =
      case {method, params} do
        {"eth_call", [%{"to" => ^address, "data" => data}, "latest"]} ->
          assert data == "0x258f0495" <> Base.encode16(hash, case: :lower)
          %{"result" => hex(<<@block::256>>)}

        {"eth_getLogs", [filter]} ->
          assert filter == %{
                   "fromBlock" => EthereumJSONRPC.integer_to_quantity(@block),
                   "toBlock" => EthereumJSONRPC.integer_to_quantity(@block),
                   "address" => address,
                   "topics" => [@event_topic, hex(hash)]
                 }

          response
      end

    Map.merge(result, %{"jsonrpc" => "2.0", "id" => id})
  end

  defp import_records(records) do
    changes =
      Enum.map(records, fn record ->
        changeset = DaMultiPurposeRecord.changeset(%DaMultiPurposeRecord{}, record)
        assert changeset.valid?
        changeset.changes
      end)

    now = DateTime.utc_now()
    multi = DaMultiPurposeRecords.run(Ecto.Multi.new(), changes, %{timestamps: %{inserted_at: now, updated_at: now}})
    assert {:ok, %{insert_da_multi_purpose_records: inserted}} = Repo.transaction(multi)
    assert length(inserted) == length(records)
  end

  defp hex(bytes), do: "0x" <> Base.encode16(bytes, case: :lower)
end
