defmodule Explorer.Repair.Repair2233405Test do
  @moduledoc """
  Task 22.33.405 closes audit 22.30.142 finding 54 (doc-drift).
  The route-table case proves the documented paths; the archive schema case
  proves frozen receipt semantics from committed receipt bytes; the signature
  controller case proves the separate live response through loopback JSON-RPC.
  The production builder services/explorer/deploy/tools/mix-in-builder.sh
  mounts services/explorer/backend/README.md read-only at /app/README.md.
  This module is the repair record because spec files are outside scope.
  The bounded gate ledger records the verified
  revision, command, exit code and private evidence location.

  No unchanged-tree verification ran: the ordered gate instructions require
  complete implementation before hf-build and verification only via hf-done.
  No baseline test failure is claimed.
  """

  use ExUnit.Case, async: false

  alias BlockScoutWeb.API.V2.PaxeerX.{ArchiveController, ArchiveView, TransactionSignatureController}
  alias BlockScoutWeb.Schemas.API.V2.PaxeerX.{ArchiveReceipt, TransactionSignature}
  alias Explorer.Chain.PaxeerX.{Receipt, SequencerReceipt}
  alias OpenApiSpex.{Cast, Components, Info, OpenApi}

  @readme Path.expand("../../../../README.md", __DIR__)
  @vectors Path.expand("../support/fixture/paxeer_x/receipt_vectors.json", __DIR__)
  @committed Path.expand("../support/fixture/paxeer_x/committed_vectors.json", __DIR__)
  @external_resource @readme
  @external_resource @vectors
  @external_resource @committed

  test "finding 54: README read paths match the production router" do
    readme = File.read!(@readme)

    for {path, controller, action} <- [
          {"/paxeer-x/archive/anchors", ArchiveController, :anchors},
          {"/paxeer-x/archive/receipts", ArchiveController, :receipts},
          {"/paxeer-x/archive/receipts/:id", ArchiveController, :receipt},
          {"/transactions/:transaction_hash_param/signature", TransactionSignatureController, :signature}
        ] do
      assert readme =~ "| `#{path}` |"
      request_path = path |> String.replace(":id", hash(1)) |> String.replace(":transaction_hash_param", hash(2))
      outer_route = Phoenix.Router.route_info(BlockScoutWeb.Router, "GET", "/api/v2" <> request_path, "localhost")
      assert outer_route.plug == BlockScoutWeb.Routers.ApiRouter

      route = Phoenix.Router.route_info(BlockScoutWeb.Routers.ApiRouter, "GET", "/v2" <> request_path, "localhost")
      assert route.plug == controller
      assert route.plug_opts == action
    end

    for path <- ["/paxeer-x/anchors", "/paxeer-x/receipts", "/paxeer-x/receipts/:id"] do
      refute readme =~ "| `#{path}` |"
    end

    assert readme =~ "frozen items with no live settlement status"
    assert readme =~ "with no live `status` field"
    assert readme =~ "do not poll it for settlement"
  end

  test "finding 54: frozen receipt JSON preserves historical verification without live status" do
    vector = Enum.find(vectors()["receipts"], &(&1["name"] == "valid-0"))
    bytes = Base.decode16!(vector["receipt"], case: :mixed)
    public_key = Base.decode16!(vector["public_key"], case: :mixed)
    assert {:ok, decoded} = SequencerReceipt.verify_signature(bytes, public_key)

    attrs = %{
      origin: :kernel,
      receipt_id: "0x" <> vector["digest"],
      status: :sequencer_signed,
      kernel_verification: :sequencer_verified,
      kernel_batch_id: hex(decoded.batch_id),
      kernel_batch_number: 1,
      kernel_sequence: decoded.global_sequence,
      kernel_activity_id: hex(decoded.activity_id),
      kernel_result_code: decoded.result_status,
      kernel_receipt_sha256: hex(:crypto.hash(:sha256, bytes)),
      kernel_canonical_sha256: hex(:crypto.hash(:sha256, Base.decode16!(vector["unsigned"], case: :mixed))),
      kernel_batch_raw_sha256: hex(:crypto.hash(:sha256, bytes)),
      kernel_state_root: hex(decoded.resulting_state_root),
      kernel_proof: %{
        "header_hex" => vector["unsigned"],
        "signature_hex" => Base.encode16(decoded.signature, case: :lower)
      }
    }

    changeset = Ecto.Changeset.cast(%Receipt{}, attrs, Map.keys(attrs))
    assert changeset.valid?
    receipt = Ecto.Changeset.apply_changes(changeset)
    list = ArchiveView.render("receipts.json", %{receipts: [receipt], next_page_params: nil}) |> json_roundtrip()
    item = ArchiveView.render("receipt.json", %{receipt: receipt}) |> json_roundtrip()

    assert list == %{"items" => [item], "next_page_params" => nil}
    assert item["id"] == "0x" <> vector["digest"]
    assert item["frozen"] == true
    assert item["verification_status"] == "sequencer_signed"
    assert item["provenance"]["kernel"]["activity_id"] == "0x" <> vector["activity_id"]
    refute Map.has_key?(item, "status")
    refute Map.has_key?(item, "rung")
    refute Map.has_key?(item, "committed_height")

    spec = schema_spec(ArchiveReceipt)
    schema = spec.components.schemas[ArchiveReceipt.schema().title]
    assert {:ok, cast} = Cast.cast(schema, item, spec.components.schemas)
    assert cast.frozen == true
    assert {:error, _} = Cast.cast(schema, Map.put(item, "status", "committed"), spec.components.schemas)
    assert {:error, _} = Cast.cast(schema, Map.delete(item, "frozen"), spec.components.schemas)
  end

  test "finding 54: signature controller returns verified live receipt evidence separately" do
    vector = @committed |> File.read!() |> Jason.decode!() |> Map.fetch!("cases") |> hd()
    transaction_hash = vector["rawTransaction"] |> decode_hex() |> ExKeccak.hash_256() |> hex()
    assert {:ok, [key]} = SequencerReceipt.decode_receipt_keys([vector["pinnedKey"]])

    for {app, setting, value} <- [
          {:explorer, :http_client, HTTPoison},
          {:tesla, :adapter, Tesla.Adapter.Mint}
        ] do
      original = Application.fetch_env(app, setting)
      Application.put_env(app, setting, value)
      on_exit(fn -> restore_env(app, setting, original) end)
    end

    original = Application.fetch_env(:explorer, SequencerReceipt)
    on_exit(fn -> restore_env(:explorer, SequencerReceipt, original) end)
    server = Bypass.open()
    observer = self()

    results = %{
      "pax_sequencerReceipt" => {[transaction_hash], vector["receiptJson"]},
      "px_getReceiptKeys" => {[], %{"keys" => [vector["pinnedKey"]]}},
      "eth_getTransactionReceipt" => {[transaction_hash], nil}
    }

    Bypass.expect(server, "POST", "/", fn conn ->
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      assert %{"jsonrpc" => "2.0", "id" => id, "method" => method, "params" => params} = Jason.decode!(body)
      assert {^params, result} = Map.fetch!(results, method)
      send(observer, {:rpc_call, method, params})
      Plug.Conn.resp(conn, 200, Jason.encode!(%{jsonrpc: "2.0", id: id, result: result}))
    end)

    Application.put_env(:explorer, SequencerReceipt,
      router_url: "http://127.0.0.1:#{server.port}/",
      chain_id: vector["chain_id"],
      receipt_keys: [key]
    )

    response =
      Plug.Test.conn("GET", "/api/v2/transactions/#{transaction_hash}/signature")
      |> TransactionSignatureController.signature(%{"transaction_hash_param" => transaction_hash})

    assert response.status == 200
    body = Jason.decode!(response.resp_body)
    assert {:ok, receipt} = vector["receiptJson"]["receipt"] |> decode_hex() |> SequencerReceipt.decode()

    assert body == %{
             "rung" => "receipted",
             "receipt" => vector["receiptJson"]["receipt"],
             "receipt_key" => %{
               "key_id" => vector["pinnedKey"]["keyId"],
               "public_key" => vector["pinnedKey"]["publicKey"]
             },
             "signature" => hex(receipt.signature)
           }

    spec = schema_spec(TransactionSignature)

    assert {:ok, _} =
             Cast.cast(spec.components.schemas[TransactionSignature.schema().title], body, spec.components.schemas)

    refute Map.has_key?(body, "frozen")
    refute Map.has_key?(body, "committed_height")

    for {method, {params, _result}} <- results do
      assert_received {:rpc_call, ^method, ^params}
    end
  end

  defp vectors, do: @vectors |> File.read!() |> Jason.decode!()
  defp json_roundtrip(value), do: value |> Jason.encode!() |> Jason.decode!()
  defp hash(value), do: hex(<<value::256>>)
  defp hex(value), do: "0x" <> Base.encode16(value, case: :lower)
  defp decode_hex("0x" <> value), do: Base.decode16!(value, case: :mixed)

  defp schema_spec(module) do
    %OpenApi{info: %Info{title: "Finding 54", version: "1"}, paths: %{}, components: %Components{schemas: %{}}}
    |> OpenApiSpex.add_schemas([module])
  end

  defp restore_env(app, key, {:ok, value}), do: Application.put_env(app, key, value)
  defp restore_env(app, key, :error), do: Application.delete_env(app, key)
end
