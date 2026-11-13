Code.require_file("../../../support/fixture/paxeer_x/governed_receipt_fixtures.exs", __DIR__)

Code.require_file("repair_gateway.exs", __DIR__)

defmodule Explorer.Chain.PaxeerX.SequencerReceiptTest do
  use ExUnit.Case, async: false

  alias Explorer.Chain.PaxeerX.{CometCommit, GovernedReceiptFixtures, ReceiptKeysConfig, SequencerReceipt}
  alias Explorer.Chain.PaxeerX.Repair2233523.Transport

  setup do
    GovernedReceiptFixtures.restore_on_exit()
    GovernedReceiptFixtures.configure_receipted()
    :ok
  end

  @vectors Path.expand("../../../support/fixture/paxeer_x/receipt_vectors.json", __DIR__)
  @commits Path.expand("../../../support/fixture/paxeer_x/comet_commit.json", __DIR__)
  @uint64_max 18_446_744_073_709_551_615

  setup_all do
    vectors = @vectors |> File.read!() |> Jason.decode!()
    commits = @commits |> File.read!() |> Jason.decode!()

    {:ok,
     kernel: Map.new(vectors["receipts"], &{&1["name"], &1}),
     cases: Map.new(vectors["sequencer"]["cases"], &{&1["id"], &1}),
     chain_id: Integer.to_string(vectors["sequencer"]["chainId"]),
     accepted_count: Enum.count(vectors["sequencer"]["cases"], &(&1["vector"] in ["V1", "V2"])),
     commits: commits,
     recorded: GovernedReceiptFixtures.read("committed_vectors.json") |> Map.fetch!("cases") |> hd()}
  end

  defp bytes("0x" <> hex), do: Base.decode16!(hex, case: :mixed)
  defp bytes(hex), do: Base.decode16!(hex, case: :mixed)

  defp hex(bytes), do: "0x" <> Base.encode16(bytes, case: :lower)

  defp pinned(%{"pinnedKey" => key}) do
    {:ok, [pinned]} = SequencerReceipt.decode_receipt_keys([key])
    pinned
  end

  defp tx_hash(%{"rawTransaction" => raw}), do: raw |> bytes() |> ExKeccak.hash_256()

  defp kernel_answer(vector) do
    %{"rung" => "receipted", "receipt" => "0x" <> vector["receipt"], "txHash" => hex(activity_id(vector))}
  end

  defp kernel_key(key_id, public_key) do
    %{key_id: key_id, public_key: bytes(public_key), first_height: 1, last_height: @uint64_max}
  end

  defp decoded(vector) do
    {:ok, receipt} = vector["receipt"] |> bytes() |> SequencerReceipt.decode()
    receipt
  end

  defp context_hash(vector), do: decoded(vector).context_hash

  defp activity_id(vector), do: decoded(vector).activity_id

  defp verify_case(test_case, keys \\ nil, opts \\ []) do
    SequencerReceipt.verify(
      tx_hash(test_case),
      test_case["receiptJson"],
      keys || [pinned(test_case)],
      Keyword.put_new(opts, :prev_seq_hash, test_case["prevSeqHash"])
    )
  end

  describe "layerxproof receipt vectors" do
    test "each valid vector decodes, re-encodes byte-exact and carries its signed digest", %{kernel: kernel} do
      for name <- ["valid-0", "valid-1", "valid-2"] do
        vector = kernel[name]
        receipt_bytes = bytes(vector["receipt"])

        assert {:ok, receipt} = SequencerReceipt.decode(receipt_bytes)
        assert SequencerReceipt.encode(receipt) == receipt_bytes
        assert receipt.protocol_version == 3
        assert receipt.activity_id == bytes(vector["activity_id"])
        assert receipt.global_sequence == String.to_integer(vector["global_sequence"])
        assert receipt.result_status == String.to_integer(vector["result_code"])
        assert receipt.module_id == String.to_integer(vector["module_id"])
        assert receipt.operation == String.to_integer(vector["operation"])
        assert receipt.asset == bytes(vector["asset"])
        assert receipt.amount == String.to_integer(vector["amount"])
        assert receipt.resulting_state_root == bytes(vector["resulting_state_root"])
        assert receipt.timestamp == String.to_integer(vector["timestamp"])
        assert SequencerReceipt.digest(receipt) == bytes(vector["digest"])
        assert byte_size(receipt.signature) == 64
      end
    end

    test "each valid vector verifies R2 against its pinned key", %{kernel: kernel} do
      for name <- ["valid-0", "valid-1", "valid-2"] do
        vector = kernel[name]
        receipt_bytes = bytes(vector["receipt"])

        assert {:ok, receipt} = SequencerReceipt.verify_signature(receipt_bytes, bytes(vector["public_key"]))
        assert SequencerReceipt.encode(receipt) == receipt_bytes
      end
    end

    test "kernel-layout vectors stop at R3 because their batch_id is not a sequencer height", %{kernel: kernel} do
      for name <- ["valid-0", "valid-1", "valid-2"] do
        vector = kernel[name]
        key = kernel_key(context_hash(vector), vector["public_key"])
        GovernedReceiptFixtures.configure_keys([key])
        refute match?(<<0::192, _::64>>, decoded(vector).batch_id)

        assert SequencerReceipt.verify(activity_id(vector), kernel_answer(vector), [key]) == {:error, :r3}
      end
    end

    test "a bit-flipped signature fails R2", %{kernel: kernel} do
      vector = kernel["bit-flipped-signature"]
      key = kernel_key(context_hash(vector), vector["public_key"])
      GovernedReceiptFixtures.configure_keys([key])

      assert SequencerReceipt.verify_signature(bytes(vector["receipt"]), bytes(vector["public_key"])) ==
               {:error, :r2}

      assert SequencerReceipt.verify(activity_id(vector), kernel_answer(vector), [key]) == {:error, :r2}
    end

    test "a bit-flipped body fails R2", %{kernel: kernel} do
      vector = kernel["bit-flipped-body"]
      key = kernel_key(context_hash(vector), vector["public_key"])
      GovernedReceiptFixtures.configure_keys([key])

      assert SequencerReceipt.verify_signature(bytes(vector["receipt"]), bytes(vector["public_key"])) ==
               {:error, :r2}

      assert SequencerReceipt.verify(activity_id(vector), kernel_answer(vector), [key]) == {:error, :r2}
    end

    test "a wrong sequencer key fails R3 under its own key id and R2 under the receipt's key id", %{kernel: kernel} do
      vector = kernel["wrong-sequencer-key"]
      other_sequencer = kernel_key(:crypto.hash(:sha256, bytes(vector["public_key"])), vector["public_key"])
      same_key_id = kernel_key(context_hash(vector), vector["public_key"])

      GovernedReceiptFixtures.configure_keys([other_sequencer])

      assert SequencerReceipt.verify(activity_id(vector), kernel_answer(vector), [other_sequencer]) ==
               {:error, :r3}

      GovernedReceiptFixtures.configure_keys([same_key_id])

      assert SequencerReceipt.verify(activity_id(vector), kernel_answer(vector), [same_key_id]) ==
               {:error, :r2}
    end
  end

  describe "node-signed sequencer receipts" do
    test "every V1 and V2 receipt decodes, re-encodes byte-exact and verifies receipted", %{
      cases: cases,
      accepted_count: accepted_count
    } do
      accepted = cases |> Map.values() |> Enum.filter(&(&1["vector"] in ["V1", "V2"]))

      assert length(accepted) == accepted_count

      for test_case <- accepted do
        receipt_bytes = bytes(test_case["signedReceipt"])

        assert {:ok, receipt} = SequencerReceipt.decode(receipt_bytes)
        assert SequencerReceipt.encode(receipt) == receipt_bytes
        assert hex(SequencerReceipt.digest(receipt)) == test_case["receiptDigest"]
        assert receipt.activity_id == tx_hash(test_case)
        assert verify_case(test_case) == {:receipted, pinned(test_case)}
      end
    end

    test "every V3 receipt is refused at its named step", %{cases: cases} do
      refused = cases |> Map.values() |> Enum.filter(&(&1["vector"] == "V3"))

      assert Enum.map(refused, &{&1["id"], &1["expected"]["refusedAt"]}) |> Enum.sort() == [
               {"V3-activity-id-not-raw-hash", "R4"},
               {"V3-height-outside-key-range", "R3"},
               {"V3-key-not-pinned", "R3"},
               {"V3-non-canonical-bytes", "R2"},
               {"V3-seq-hash-mismatch", "R6"},
               {"V3-tampered-log", "R5"}
             ]

      for test_case <- refused do
        step = test_case["expected"]["refusedAt"] |> String.downcase() |> String.to_existing_atom()

        assert {test_case["id"], verify_case(test_case)} == {test_case["id"], {:error, step}}
      end
    end

    test "a key whose height range excludes the batch fails R3", %{cases: cases} do
      test_case = cases["V1-pax-transfer"]
      key = pinned(test_case)

      assert verify_case(test_case, [%{key | last_height: 999}]) == {:error, :r3}
      assert verify_case(test_case, [%{key | first_height: 1001, last_height: 1999}]) == {:error, :r3}
    end

    test "a wrong transaction hash fails R4", %{cases: cases} do
      test_case = cases["V1-pax-transfer"]
      other_hash = tx_hash(cases["V1-erc20-approve"])

      assert SequencerReceipt.verify(other_hash, test_case["receiptJson"], [pinned(test_case)]) == {:error, :r4}
    end

    test "an answer field that differs from the receipt bytes fails R5", %{cases: cases} do
      test_case = cases["V1-pax-transfer"]
      answer = %{test_case["receiptJson"] | "gasUsed" => "0x1"}

      assert SequencerReceipt.verify(tx_hash(test_case), answer, [pinned(test_case)]) == {:error, :r5}
    end

    test "an answer without a receipted rung fails R1", %{cases: cases} do
      test_case = cases["V1-pax-transfer"]

      assert SequencerReceipt.verify(tx_hash(test_case), %{"rung" => "submitted"}, [pinned(test_case)]) ==
               {:error, :r1}
    end
  end

  describe "committed rung" do
    setup %{recorded: recorded} do
      GovernedReceiptFixtures.configure_committed()
      {:ok, test_case: recorded, eth_receipt: recorded["eth_receipt"]}
    end

    test "a recorded landing block verifies committed and an unrelated four-of-four commit is refused", context do
      %{test_case: test_case, commits: commits} = context
      four = commits["four_of_four"]

      assert CometCommit.verify(commits["chain_id"], four["commit"], four["validators"]) == {:ok, 1_000}

      assert verify_case(test_case, nil, GovernedReceiptFixtures.committed_opts(test_case)) ==
               {:committed, pinned(test_case), 10}

      unrelated =
        GovernedReceiptFixtures.committed_opts(test_case)
        |> Keyword.merge(chain_id: commits["chain_id"], commit: four["commit"], validators: four["validators"])

      assert verify_case(test_case, nil, unrelated) == {:error, :r8}
    end

    test "R8 refuses the two-of-four commit", context do
      %{test_case: test_case, commits: commits} = context
      two = commits["two_of_four"]

      assert CometCommit.verify(commits["chain_id"], two["commit"], two["validators"]) ==
               {:error, :insufficient_voting_power}

      insufficient =
        GovernedReceiptFixtures.committed_opts(test_case)
        |> Keyword.merge(chain_id: commits["chain_id"], commit: two["commit"], validators: two["validators"])

      assert verify_case(test_case, nil, insufficient) == {:error, :r8}
    end

    test "R7 refuses a landing receipt with another gasUsed or a batch that does not cover seq", context do
      %{test_case: test_case, eth_receipt: eth_receipt} = context
      opts = GovernedReceiptFixtures.committed_opts(test_case)
      changed = %{eth_receipt | "gasUsed" => "0x1"}

      assert verify_case(test_case, nil, Keyword.put(opts, :eth_receipt, changed)) == {:error, :r7}

      uncovered = Map.put(test_case["batch"], "from_seq", 10)
      assert verify_case(test_case, nil, Keyword.put(opts, :batch, uncovered)) == {:error, :r7}
    end
  end

  test "receipt_gateway_transport_proof" do
    Explorer.Chain.PaxeerX.Repair2233523.Gateway.receipt_gateway_transport_proof()
  end

  @tag :issue1806
  test "issue1806_governed_receipt_keys", %{cases: cases, chain_id: chain_id, kernel: kernel} do
    test_case = cases["V1-pax-transfer"]
    wire_key = test_case["pinnedKey"]
    [pin] = ReceiptKeysConfig.parse([wire_key])
    refute pin.key_id == :crypto.hash(:sha256, pin.public_key)
    assert test_case["receiptJson"]["receipt"] == test_case["signedReceipt"]
    assert {:ok, receipt} = SequencerReceipt.verify_signature(bytes(test_case["signedReceipt"]), pin.public_key)
    assert SequencerReceipt.encode(receipt) == bytes(test_case["signedReceipt"])
    assert receipt.context_hash == pin.key_id

    config = Application.get_env(:explorer, SequencerReceipt)
    adapter = Application.get_env(:tesla, :adapter)
    http_client = Application.get_env(:explorer, :http_client)

    on_exit(fn ->
      Transport.restore(:explorer, SequencerReceipt, config)
      Transport.restore(:tesla, :adapter, adapter)
      Transport.restore(:explorer, :http_client, http_client)
    end)

    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)
    Application.put_env(:explorer, :http_client, Explorer.HttpClient.Tesla)
    state = start_supervised!({Agent, fn -> [wire_key] end})
    server = Bypass.open()
    observer = self()

    Bypass.expect(server, "POST", "/", fn conn ->
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      request = Jason.decode!(body)
      assert request["jsonrpc"] == "2.0"
      assert request["method"] == "px_getReceiptKeys"
      assert request["params"] == []
      send(observer, :issue1806_discovery)

      response = %{
        "jsonrpc" => "2.0",
        "id" => request["id"],
        "result" => %{"keys" => Agent.get(state, & &1)}
      }

      Plug.Conn.resp(conn, 200, Jason.encode!(response))
    end)

    url = "http://127.0.0.1:#{server.port}/"

    configure = fn pins ->
      Application.put_env(:explorer, SequencerReceipt, chain_id: chain_id, router_url: url, receipt_keys: pins)
    end

    for _ <- 1..2 do
      Application.delete_env(:explorer, SequencerReceipt)
      assert SequencerReceipt.receipt_keys(url) == {:error, :untrusted_receipt_keys}
      assert verify_case(test_case, [pin]) == {:error, :r3}

      [reloaded] = ReceiptKeysConfig.parse([wire_key])
      configure.([reloaded])
      assert {:ok, [^pin]} = SequencerReceipt.receipt_keys()
      assert_received :issue1806_discovery
      assert verify_case(test_case, [reloaded]) == {:receipted, pin}
    end

    [restricted] = ReceiptKeysConfig.parse([%{wire_key | "lastHeight" => 1_500}])
    configure.([restricted])
    assert {:ok, [^restricted]} = SequencerReceipt.receipt_keys()
    assert verify_case(test_case, [pin]) == {:receipted, restricted}

    for {first, last} <- [{1, 999}, {1_001, 1_999}] do
      [outside] = ReceiptKeysConfig.parse([%{wire_key | "firstHeight" => first, "lastHeight" => last}])
      configure.([outside])
      Agent.update(state, fn _ -> [%{wire_key | "firstHeight" => 1, "lastHeight" => 1_999}] end)
      assert {:ok, [^outside]} = SequencerReceipt.receipt_keys()
      assert verify_case(test_case, [outside]) == {:error, :r3}
    end

    configure.([pin])

    for uncovered <- [
          %{wire_key | "firstHeight" => 1_001},
          %{wire_key | "lastHeight" => 1_998}
        ] do
      Agent.update(state, fn _ -> [uncovered] end)
      assert SequencerReceipt.receipt_keys() == {:error, :untrusted_receipt_keys}
      {:ok, discovered} = SequencerReceipt.decode_receipt_keys([uncovered])
      assert verify_case(test_case, discovered) == {:error, :r3}
    end

    other_public = bytes(kernel["valid-0"]["public_key"])
    refute other_public == pin.public_key
    substituted = %{pin | public_key: other_public}
    unpinned = %{substituted | key_id: :crypto.hash(:sha256, other_public)}

    for untrusted <- [substituted, unpinned] do
      Agent.update(state, fn _ ->
        [%{wire_key | "keyId" => hex(untrusted.key_id), "publicKey" => hex(untrusted.public_key)}]
      end)

      assert SequencerReceipt.receipt_keys() == {:error, :untrusted_receipt_keys}
      assert verify_case(test_case, [untrusted]) == {:error, :r3}
    end

    configure.([substituted])
    assert verify_case(test_case, [substituted]) == {:error, :r2}
    configure.([unpinned])
    assert verify_case(test_case, [unpinned]) == {:error, :r3}

    for {id, expected} <- [
          {hex(<<0::256>>), :untrusted_receipt_keys},
          {"0x01", :malformed_receipt_keys},
          {hex(<<1::264>>), :malformed_receipt_keys},
          {"0x" <> String.duplicate("gg", 32), :malformed_receipt_keys},
          {nil, :malformed_receipt_keys}
        ] do
      invalid = %{wire_key | "keyId" => id}
      assert_raise ArgumentError, fn -> ReceiptKeysConfig.parse([invalid]) end
      configure.([pin])
      Agent.update(state, fn _ -> [invalid] end)
      assert SequencerReceipt.receipt_keys() == {:error, expected}
    end

    for id <- [<<0::256>>, <<1::248>>, <<1::264>>, nil, 1] do
      invalid = %{pin | key_id: id}
      configure.([invalid])
      assert verify_case(test_case, [invalid]) == {:error, :r3}
    end

    Agent.update(state, fn _ -> [wire_key] end)

    for {field, wire_field, value} <- [
          {:first_height, "firstHeight", 0},
          {:first_height, "firstHeight", -1},
          {:first_height, "firstHeight", @uint64_max + 1},
          {:first_height, "firstHeight", "1000"},
          {:first_height, "firstHeight", 1_000.0},
          {:last_height, "lastHeight", 0},
          {:last_height, "lastHeight", -1},
          {:last_height, "lastHeight", @uint64_max + 1},
          {:last_height, "lastHeight", "1999"},
          {:last_height, "lastHeight", nil},
          {:first_height, "firstHeight", 2_000}
        ] do
      assert_raise ArgumentError, fn -> ReceiptKeysConfig.parse([Map.put(wire_key, wire_field, value)]) end
      invalid = Map.put(pin, field, value)
      configure.([invalid])
      assert SequencerReceipt.receipt_keys() == {:error, :untrusted_receipt_keys}
      assert verify_case(test_case, [invalid]) == {:error, :r3}
    end

    [maximum] = ReceiptKeysConfig.parse([%{wire_key | "firstHeight" => 1, "lastHeight" => @uint64_max}])
    configure.([maximum])
    Agent.update(state, fn _ -> [%{wire_key | "firstHeight" => 1, "lastHeight" => @uint64_max}] end)
    assert {:ok, [^maximum]} = SequencerReceipt.receipt_keys()
    assert verify_case(test_case, [maximum]) == {:receipted, maximum}

    configure.([pin])
    signed = bytes(test_case["signedReceipt"])
    prefix_size = byte_size(signed) - 1
    <<prefix::binary-size(prefix_size), final_byte>> = signed
    altered = hex(prefix <> <<Bitwise.bxor(final_byte, 1)>>)
    altered_case = put_in(test_case, ["receiptJson", "receipt"], altered)
    assert verify_case(altered_case, [pin]) == {:error, :r2}
    assert verify_case(test_case, [pin]) == {:receipted, pin}
  end

  describe "router reads" do
    test "an unset router URL disables the upgrade" do
      assert SequencerReceipt.receipt_keys(nil) == :disabled
      assert SequencerReceipt.commit(1_000, nil) == :disabled
      assert SequencerReceipt.validators(1_000, nil) == :disabled
    end

    test "receipt keys decode from the router answer and malformed entries are refused" do
      key_id = String.duplicate("6b", 32)
      public_key = String.duplicate("01", 32)

      assert SequencerReceipt.decode_receipt_keys([
               %{
                 "keyId" => "0x" <> key_id,
                 "publicKey" => "0x" <> public_key,
                 "firstHeight" => 1_000,
                 "lastHeight" => "0x7cf"
               }
             ]) ==
               {:ok,
                [
                  %{
                    key_id: Base.decode16!(key_id, case: :lower),
                    public_key: Base.decode16!(public_key),
                    first_height: 1_000,
                    last_height: 1_999
                  }
                ]}

      assert SequencerReceipt.decode_receipt_keys([
               %{"keyId" => "0x" <> key_id, "publicKey" => "0x01", "firstHeight" => 1, "lastHeight" => 2}
             ]) == {:error, :malformed_receipt_keys}

      assert SequencerReceipt.decode_receipt_keys([
               %{"keyId" => "0x" <> key_id, "publicKey" => "0x" <> public_key, "firstHeight" => 3, "lastHeight" => 2}
             ]) == {:error, :malformed_receipt_keys}
    end
  end
end
