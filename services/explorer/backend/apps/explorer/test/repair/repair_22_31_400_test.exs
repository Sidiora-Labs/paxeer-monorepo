defmodule Explorer.Repair2231400Test do
  use ExUnit.Case, async: false

  alias Explorer.Chain.PaxeerX.{CometCommit, ReceiptInclusion, SequencerReceipt, Status}
  alias Explorer.Chain.PaxeerX.Finality.Heights
  alias Explorer.HttpClient

  @recording Path.expand("../../lib/explorer/chain/paxeer_x/fixtures/receipt_commit_inclusion.json", __DIR__)
  @old_vectors Path.expand("../support/fixture/paxeer_x/committed_vectors.json", __DIR__)

  defmodule RecordedRPC do
    @behaviour Plug

    def init(options), do: options

    def call(conn, {responses, observer}) do
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      %{"jsonrpc" => "2.0", "id" => id, "method" => method, "params" => params} = Jason.decode!(body)
      result = Map.fetch!(responses, {method, params})
      send(observer, {:recorded_rpc, method, params})

      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.send_resp(200, Jason.encode!(%{jsonrpc: "2.0", id: id, result: result}))
    end
  end

  setup do
    old_config = Application.get_env(:explorer, SequencerReceipt)
    old_adapter = Application.get_env(:tesla, :adapter)
    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)
    {:ok, _} = Application.ensure_all_started(:plug_cowboy)

    on_exit(fn ->
      restore_env(:explorer, SequencerReceipt, old_config)
      restore_env(:tesla, :adapter, old_adapter)
    end)

    vector = @recording |> File.read!() |> Jason.decode!()
    {:ok, [key]} = SequencerReceipt.decode_receipt_keys([vector["pinnedKey"]])
    answer = vector["receiptJson"]
    eth_receipt = landing_receipt(answer)
    height = String.to_integer(vector["batch"]["height"])
    hash = vector["rawTransaction"] |> bytes() |> ExKeccak.hash_256()
    assert hex(hash) == answer["txHash"]
    opts = options(vector, eth_receipt, [hash])
    configure_trust(vector["commit"])
    {:ok, vector: vector, answer: answer, eth_receipt: eth_receipt, height: height, hash: hash, key: key, opts: opts}
  end

  test "receipt-commit-inclusion: recorded producer survives HTTP transport and reaches committed consumer", ctx do
    %{vector: vector, answer: answer, height: height, hash: hash, key: key} = ctx
    calls = recorded_calls(vector, ctx.eth_receipt)
    url = replay(calls)

    assert {:ok, ^answer} = rpc(url, "pax_sequencerReceipt", [hex(hash)])
    assert {:ok, [^key]} = SequencerReceipt.receipt_keys(url)
    assert {:ok, eth_receipt} = rpc(url, "eth_getTransactionReceipt", [hex(hash)])
    assert {:ok, commit} = SequencerReceipt.commit(height, url)
    assert {:ok, validators} = SequencerReceipt.validators(height, url)
    assert {:ok, batch} = SequencerReceipt.sequence_batch(height, url)
    assert batch == vector["batch"]
    assert byte_size(bytes(batch["tx"])) == 241
    assert length(batch["proof"]["aunts"]) == 1

    opts = Keyword.merge(ctx.opts, eth_receipt: eth_receipt, commit: commit, validators: validators, batch: batch)
    assert {:ok, ^height} = CometCommit.verify(opts[:chain_id], commit, validators)
    verification = SequencerReceipt.verify(hash, answer, [key], opts)
    assert verification == {:committed, key, height}
    assert %Status{rung: :committed, committed_height: ^height} = Status.of(nil, %Heights{}, verification)

    for {method, params, _result} <- calls do
      assert_received {:recorded_rpc, ^method, ^params}
    end
  end

  test "receipt-commit-inclusion: a matching-height valid unrelated commit cannot authenticate a receipt" do
    %{"cases" => cases} = @old_vectors |> File.read!() |> Jason.decode!()
    vector = Enum.find(cases, &(&1["id"] == "committed-ok"))
    {:ok, [key]} = SequencerReceipt.decode_receipt_keys([vector["pinnedKey"]])
    answer = vector["receiptJson"]
    hash = bytes(answer["txHash"])
    commit = vector["commit"]
    configure_trust(commit)
    assert {:ok, 1_000} = CometCommit.verify(vector["chain_id"], commit, vector["validators"])
    assert {:receipted, ^key} = SequencerReceipt.verify(hash, answer, [key])

    eth_receipt =
      vector["eth_receipt"]
      |> Map.put("transactionIndex", answer["index"])
      |> Map.put("blockHash", "0x" <> String.downcase(commit["signed_header"]["commit"]["block_id"]["hash"]))

    opts = [
      eth_receipt: eth_receipt,
      batch: %{from_seq: 1, to_seq: 4},
      batch_activity_ids: [hash],
      chain_id: vector["chain_id"],
      commit: commit,
      validators: vector["validators"]
    ]

    assert SequencerReceipt.verify(hash, answer, [key], opts) == {:error, :r7}
  end

  test "receipt-commit-inclusion: proofs bind the exact batch bytes and signed data hash", ctx do
    %{vector: vector, opts: opts} = ctx
    batch = vector["batch"]
    proof = batch["proof"]

    invalid_batches = [
      Map.delete(batch, "proof"),
      Map.put(batch, "height", "11"),
      Map.put(batch, "tx", hex(bytes(batch["tx"]) <> <<0>>)),
      Map.put(batch, "tx", flip_hex(batch["tx"], 41)),
      Map.put(batch, "tx", flip_hex(batch["tx"], 49)),
      Map.put(batch, "tx", flip_hex(batch["tx"], 57)),
      Map.put(batch, "tx", flip_hex(batch["tx"], 89)),
      Map.put(batch, "tx", flip_hex(batch["tx"], 121)),
      Map.put(batch, "tx", flip_hex(batch["tx"], 153)),
      Map.put(batch, "tx", flip_hex(batch["tx"], 240)),
      Map.put(batch, "proof", Map.put(proof, "index", 1)),
      Map.put(batch, "proof", Map.put(proof, "total", 3)),
      Map.put(batch, "proof", Map.put(proof, "leafHash", flip_hex(proof["leafHash"], 0))),
      Map.put(batch, "proof", Map.put(proof, "aunts", [])),
      Map.put(batch, "proof", Map.put(proof, "aunts", proof["aunts"] ++ proof["aunts"])),
      Map.put(batch, "proof", Map.put(proof, "aunts", [flip_hex(hd(proof["aunts"]), 0)])),
      Map.put(batch, "proof", Map.put(proof, "aunts", [nil]))
    ]

    assert verify(ctx, opts) == {:committed, ctx.key, ctx.height}

    for batch <- invalid_batches do
      assert verify(ctx, Keyword.put(opts, :batch, batch)) == {:error, :r7}
    end
  end

  test "receipt-commit-inclusion: sequence range, activity root and receipt roots are authenticated", ctx do
    assert verify(ctx, Keyword.delete(ctx.opts, :batch_activity_ids)) == {:error, :r7}

    for ids <- [[], [<<0::256>>], [ctx.hash, ctx.hash], [nil]] do
      assert verify(ctx, Keyword.put(ctx.opts, :batch_activity_ids, ids)) == {:error, :r7}
    end

    {:ok, receipt} = ctx.answer["receipt"] |> bytes() |> SequencerReceipt.decode()

    for changed <- [
          %{receipt | previous_state_root: <<0::256>>},
          %{receipt | resulting_state_root: <<0::256>>},
          %{receipt | activity_id: <<0::256>>},
          %{receipt | global_sequence: receipt.global_sequence + 1}
        ] do
      assert ReceiptInclusion.verify(changed, ctx.key, ctx.height, ctx.opts) == {:error, :r7}
      answer = Map.put(ctx.answer, "receipt", hex(SequencerReceipt.encode(changed)))
      assert SequencerReceipt.verify(ctx.hash, answer, [ctx.key], ctx.opts) == {:error, :r2}
    end
  end

  test "receipt-commit-inclusion: receipt results and landing identity cannot be substituted", ctx do
    for {field, value} <- [
          {"status", "0x0"},
          {"gasUsed", "0x1"},
          {"transactionIndex", "0x2"},
          {"blockHash", hex(<<0::256>>)},
          {"transactionHash", hex(<<0::256>>)},
          {"logs", []}
        ] do
      changed = Map.put(ctx.eth_receipt, field, value)
      assert verify(ctx, Keyword.put(ctx.opts, :eth_receipt, changed)) == {:error, :r7}
    end

    for {field, value} <- [{"status", "0x0"}, {"gasUsed", "0x1"}, {"returnData", "0x00"}] do
      assert SequencerReceipt.verify(ctx.hash, Map.put(ctx.answer, field, value), [ctx.key], ctx.opts) == {:error, :r5}
    end

    {:ok, receipt} = ctx.answer["receipt"] |> bytes() |> SequencerReceipt.decode()
    changed = %{receipt | result_status: 1}
    answer = Map.put(ctx.answer, "receipt", hex(SequencerReceipt.encode(changed)))
    assert SequencerReceipt.verify(ctx.hash, answer, [ctx.key], ctx.opts) == {:error, :r2}
  end

  test "receipt-commit-inclusion: RPC evidence cannot choose its own chain or validator trust", ctx do
    config = Application.fetch_env!(:explorer, SequencerReceipt)

    for untrusted <- [
          [],
          Keyword.delete(config, :chain_id),
          Keyword.put(config, :chain_id, "another-chain"),
          Keyword.delete(config, :trusted_validator_hashes),
          Keyword.put(config, :trusted_validator_hashes, %{}),
          Keyword.put(config, :trusted_validator_hashes, %{ctx.height => <<0::256>>})
        ] do
      Application.put_env(:explorer, SequencerReceipt, untrusted)
      assert verify(ctx, ctx.opts) == {:error, :r8}
    end

    Application.put_env(:explorer, SequencerReceipt, config)
    assert verify(ctx, Keyword.put(ctx.opts, :chain_id, "another-chain")) == {:error, :r8}

    commit =
      put_in(
        ctx.vector["commit"],
        ["signed_header", "commit", "signatures", Access.at(0), "signature"],
        Base.encode64(<<0::512>>)
      )

    assert verify(ctx, Keyword.put(ctx.opts, :commit, commit)) == {:error, :r8}
  end

  test "receipt-commit-inclusion: actual controller rejects its discarded-proof range input", ctx do
    :ok = Ecto.Adapters.SQL.Sandbox.checkout(Explorer.Repo)
    :ok = Ecto.Adapters.SQL.Sandbox.checkout(Explorer.Repo.Account)
    calls = recorded_calls(ctx.vector, ctx.eth_receipt)
    url = replay(calls)
    config = Application.fetch_env!(:explorer, SequencerReceipt)
    Application.put_env(:explorer, SequencerReceipt, Keyword.put(config, :router_url, url))
    hash = hex(ctx.hash)
    {:ok, full_hash} = Explorer.Chain.string_to_full_hash(hash)
    assert {:error, :not_found} = Explorer.Chain.hash_to_transaction(full_hash, api?: true)
    path = "/api/v2/transactions/#{hash}/status"

    assert {:error, :not_found} =
             BlockScoutWeb.API.V2.PaxeerX.TransactionStatusController.status(Plug.Test.conn(:get, path), %{
               "transaction_hash_param" => hash
             })

    for {method, params, _result} <- calls do
      assert_received {:recorded_rpc, ^method, ^params}
    end
  end

  defp verify(ctx, opts), do: SequencerReceipt.verify(ctx.hash, ctx.answer, [ctx.key], opts)

  defp landing_receipt(answer) do
    answer
    |> Map.take(["transactionHash", "blockHash", "blockNumber", "status", "gasUsed", "logs"])
    |> Map.put("transactionHash", answer["txHash"])
    |> Map.put("transactionIndex", answer["index"])
  end

  defp options(vector, eth_receipt, ids) do
    [
      eth_receipt: eth_receipt,
      batch: vector["batch"],
      batch_activity_ids: ids,
      chain_id: vector["commit"]["signed_header"]["header"]["chain_id"],
      commit: vector["commit"],
      validators: vector["validators"]
    ]
  end

  defp configure_trust(commit) do
    header = commit["signed_header"]["header"]

    Application.put_env(:explorer, SequencerReceipt,
      chain_id: header["chain_id"],
      trusted_validator_hashes: %{String.to_integer(header["height"]) => Base.decode16!(header["validators_hash"])}
    )
  end

  defp recorded_calls(vector, eth_receipt) do
    height = vector["batch"]["height"]

    validators = %{
      "height" => height,
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

    [
      {"pax_sequencerReceipt", [vector["receiptJson"]["txHash"]], vector["receiptJson"]},
      {"px_getReceiptKeys", [], %{"keys" => [vector["pinnedKey"]]}},
      {"eth_getTransactionReceipt", [vector["receiptJson"]["txHash"]], eth_receipt},
      {"px_getCommit", [height], %{"height" => height, "signedHeader" => vector["commit"]["signed_header"]}},
      {"px_getValidators", [height], validators},
      {"px_getSequenceBatch", [height], vector["batch"]}
    ]
  end

  defp replay(calls) do
    responses = Map.new(calls, fn {method, params, result} -> {{method, params}, result} end)
    ref = {:receipt_recording, make_ref()}

    start_supervised!(
      {Plug.Cowboy, scheme: :http, plug: {RecordedRPC, {responses, self()}}, options: [port: 0, ref: ref]}
    )

    "http://127.0.0.1:#{:ranch.get_port(ref)}/"
  end

  defp rpc(url, method, params) do
    body = Jason.encode!(%{jsonrpc: "2.0", id: 1, method: method, params: params})
    {:ok, %{status_code: 200, body: response}} = HttpClient.post(url, body, [{"Content-Type", "application/json"}])
    %{"result" => result} = Jason.decode!(response)
    {:ok, result}
  end

  defp flip_hex(value, offset) do
    <<before::binary-size(offset), byte, rest::binary>> = bytes(value)
    hex(before <> <<Bitwise.bxor(byte, 1)>> <> rest)
  end

  defp bytes("0x" <> digits), do: Base.decode16!(digits, case: :mixed)
  defp hex(bytes), do: "0x" <> Base.encode16(bytes, case: :lower)

  defp restore_env(app, key, nil), do: Application.delete_env(app, key)
  defp restore_env(app, key, value), do: Application.put_env(app, key, value)
end
