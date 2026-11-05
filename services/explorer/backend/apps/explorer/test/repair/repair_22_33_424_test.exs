Code.require_file(
  "../../../block_scout_web/test/block_scout_web/controllers/api/v2/paxeer_x/committed_evidence.exs",
  __DIR__
)

defmodule Explorer.Repair.Repair2233424Test do
  @moduledoc """
  Task 22.33.424 / audit 22.30.145 repair record.

  Finding backend-archive-route-docs is proved by "documented archive and live
  routes dispatch to the production controllers", "archive lists and receipt
  detail publish frozen historical schema values", "committed node receipts
  reach live consumers without populating archives", and "capabilities and
  unified accounts retain their documented native contracts".

  The README already listed the three explicit archive paths and frozen: true
  at base e1ddbb5a35bde06506c24046b3b4e33370a9b8a5. This repair adds the explicit
  committed-node receipt interface and its explorer signature consumer. Tests
  drive the production router, controllers, schemas, database queries and native
  receipt verifier. Committed EVM transaction and signed native block/result
  recordings supply chain data; archive schema inputs exercise historical fields
  without claiming that the recorded EVM transaction emitted an anchor event.
  RPC peers replay the committed native evidence over loopback HTTP.

  Baseline: no scoped edits or prior hf-build/hf-done ledger existed. No tests
  ran on the unchanged tree and no baseline failures are claimed: the explicit
  ordered instructions prohibit verification before complete implementation and
  require hf-build followed by hf-done. The gates record the exact verified
  revision, command, exit code and private evidence log. This module is the
  scoped task record because spec/ and GOTCHA.kvx changes are prohibited.
  No out-of-path wiring changes are required. The builder mounts apps/ but not
  the root README; the route inventory below checks its documented contracts
  against the compiled production router.
  """

  use Explorer.DataCase, async: false

  import Phoenix.ConnTest

  alias BlockScoutWeb.API.V2.PaxeerX
  alias BlockScoutWeb.API.V2.PaxeerX.CommittedEvidence, as: Evidence
  alias EthereumJSONRPC.Transaction, as: JSONRPCTransaction
  alias Explorer.Chain
  alias Explorer.Chain.{Address, Transaction}
  alias Explorer.Chain.PaxeerX.{Anchor, Capabilities, Receipt, SequencerReceipt}

  @endpoint BlockScoutWeb.Endpoint
  @routes [
    {"/v2/paxeer-x/archive/anchors", PaxeerX.ArchiveController, :anchors},
    {"/v2/paxeer-x/archive/receipts", PaxeerX.ArchiveController, :receipts},
    {"/v2/paxeer-x/archive/receipts/:id", PaxeerX.ArchiveController, :receipt},
    {"/v2/paxeer-x/capabilities", PaxeerX.CapabilitiesController, :capabilities},
    {"/v2/addresses/:address_hash_param/unified", PaxeerX.UnifiedAccountController, :unified},
    {"/v2/transactions/:transaction_hash_param/status", PaxeerX.TransactionStatusController, :status},
    {"/v2/transactions/:transaction_hash_param/signature", PaxeerX.TransactionSignatureController, :signature}
  ]

  setup_all do
    {:ok, _} = Application.ensure_all_started(:block_scout_web)
    :ok
  end

  test "documented archive and live routes dispatch to the production controllers" do
    controllers = MapSet.new(@routes, &elem(&1, 1))

    registered =
      BlockScoutWeb.Routers.ApiRouter
      |> Phoenix.Router.routes()
      |> Enum.filter(&(&1.verb == :get and MapSet.member?(controllers, &1.plug)))
      |> MapSet.new(&{&1.path, &1.plug, &1.plug_opts})

    assert registered == MapSet.new(@routes)

    for path <- ["anchors", "receipts", "receipts/" <> Evidence.settlement()["receipt"]["txHash"]] do
      assert request("/api/v2/paxeer-x/#{path}", %{}, 404) == %{"message" => "Not found"}
    end
  end

  test "archive lists and receipt detail publish frozen historical schema values" do
    %{transaction: transaction, block: block} = recorded_transaction()

    anchor_attrs = %{
      transaction_hash: transaction.hash,
      block_hash: block.hash,
      block_number: block.number,
      block_consensus: true,
      log_index: 0,
      batch_number: block.number,
      checkpoint_id: block.hash,
      state_root: block.hash,
      kernel_height: block.number,
      sealed_height: block.number,
      status: :final
    }

    anchor = %Anchor{} |> Ecto.Changeset.cast(anchor_attrs, Map.keys(anchor_attrs)) |> Repo.insert!()

    assert request("/api/v2/paxeer-x/archive/anchors") == %{
             "items" => [
               %{
                 "batch_number" => anchor.batch_number,
                 "checkpoint_id" => to_string(anchor.checkpoint_id),
                 "checkpoint_height" => anchor.kernel_height,
                 "sealed_height" => anchor.sealed_height,
                 "state_root" => to_string(anchor.state_root),
                 "block_number" => block.number,
                 "timestamp" => DateTime.to_iso8601(block.timestamp),
                 "frozen" => true
               }
             ],
             "next_page_params" => nil
           }

    for status <- Receipt.statuses() do
      attrs = %{
        receipt_id: transaction.hash,
        transaction_hash: transaction.hash,
        block_hash: block.hash,
        block_number: block.number,
        block_consensus: true,
        log_index: 1,
        status: status
      }

      receipt = %Receipt{} |> Ecto.Changeset.cast(attrs, Map.keys(attrs)) |> Repo.insert!()
      detail = request("/api/v2/paxeer-x/archive/receipts/#{receipt.receipt_id}")
      assert detail["id"] == to_string(transaction.hash)
      assert detail["transaction_hash"] == to_string(transaction.hash)
      assert detail["verification_status"] == Atom.to_string(status)
      assert detail["frozen"] == true
      refute Map.has_key?(detail, "status")
      refute Map.has_key?(detail, "rung")
      assert request("/api/v2/paxeer-x/archive/receipts") == %{"items" => [detail], "next_page_params" => nil}
      assert request("/api/v2/paxeer-x/archive/receipts/#{receipt.receipt_id}") == detail
      assert Repo.get!(Receipt, receipt.id).status == status
      Repo.delete!(receipt)
    end

    for schema <- [
          BlockScoutWeb.Schemas.API.V2.PaxeerX.ArchiveAnchor,
          BlockScoutWeb.Schemas.API.V2.PaxeerX.ArchiveReceipt
        ] do
      assert :frozen in schema.schema().required
      assert schema.schema().properties.frozen.enum == [true]
    end

    for id <- ["invalid", Evidence.settlement()["receipt"]["txHash"]] do
      assert request("/api/v2/paxeer-x/archive/receipts/#{id}", %{}, 404) == %{"message" => "Not found"}
    end
  end

  test "committed node receipts reach live consumers without populating archives" do
    previous = Application.fetch_env(:explorer, SequencerReceipt)
    adapter = Application.fetch_env(:tesla, :adapter)
    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)

    on_exit(fn ->
      restore_env(:explorer, SequencerReceipt, previous)
      restore_env(:tesla, :adapter, adapter)
    end)

    vector = Evidence.settlement()
    hash = vector["receipt"]["txHash"]
    {:ok, full_hash} = Chain.string_to_full_hash(hash)
    assert {:error, :not_found} = Chain.hash_to_transaction(full_hash, api?: true)
    calls = Evidence.committed_calls(vector)
    Evidence.serve_rpc(calls)

    assert request("/api/v2/transactions/#{hash}/status") == %{
             "rung" => "committed",
             "block_number" => nil,
             "committed_height" => 10
           }

    Evidence.assert_calls(calls)
    signature = request("/api/v2/transactions/#{hash}/signature")
    key = Evidence.receipt_key()
    assert {:ok, receipt} = SequencerReceipt.decode(Evidence.bytes(vector["receipt"]["receipt"]))

    assert signature == %{
             "receipt" => vector["receipt"]["receipt"],
             "receipt_key" => %{"key_id" => key["keyId"], "public_key" => key["publicKey"]},
             "signature" => Evidence.hex(receipt.signature),
             "rung" => "committed",
             "committed_height" => 10
           }

    Evidence.assert_calls(calls)

    assert {:ok, ^receipt} =
             SequencerReceipt.verify_signature(Evidence.bytes(signature["receipt"]), Evidence.bytes(key["publicKey"]))

    assert {:error, :not_found} = Chain.hash_to_transaction(full_hash, api?: true)

    for resource <- ~w(anchors receipts) do
      assert request("/api/v2/paxeer-x/archive/#{resource}") == %{"items" => [], "next_page_params" => nil}
    end

    assert request("/api/v2/paxeer-x/archive/receipts/#{hash}", %{}, 404) == %{"message" => "Not found"}
  end

  test "capabilities and unified accounts retain their documented native contracts" do
    snapshot = Capabilities.all()
    capabilities = request("/api/v2/paxeer-x/capabilities")

    assert Map.take(capabilities, ~w(addr exchange bridge launchpad checked_at)) == %{
             "addr" => snapshot.unified_account,
             "exchange" => snapshot.exchange,
             "bridge" => snapshot.bridge,
             "launchpad" => snapshot.launchpad,
             "checked_at" => snapshot.checked_at && DateTime.to_iso8601(snapshot.checked_at)
           }

    %{transaction: transaction, sender: sender, recipient: recipient} = recorded_transaction()
    account = request("/api/v2/addresses/#{sender.hash}/unified")
    assert account["identities"] == %{"evm" => Address.checksum(sender.hash), "pax" => nil, "did" => nil}
    assert [%{"asset" => %{"id" => "native", "decimals" => 18}, "total" => "0"}] = account["balances"]
    assert [%{"kind" => "transaction", "status" => "committed", "result" => "ok"} = activity] = account["activity"]
    assert activity["hash"] == to_string(transaction.hash)
    assert activity["amount"] == Decimal.to_string(transaction.value.value)
    assert activity["counterparty"] == Address.checksum(recipient.hash)
    assert account["activity_total"] == 1
    assert account["page_number"] == 1
    assert account["next_page_params"] == nil
    assert is_binary(account["page_cursor"])
    assert is_binary(account["first_page_cursor"])

    assert request("/api/v2/addresses/#{sender.hash}/unified", %{"cursor" => account["first_page_cursor"]})[
             "activity"
           ] == account["activity"]

    assert request("/api/v2/paxeer-x/archive/receipts") == %{"items" => [], "next_page_params" => nil}
  end

  defp recorded_transaction do
    [recording] =
      "../support/fixture/vcr_cassettes/transaction_importer_saves_the_association.json"
      |> Path.expand(__DIR__)
      |> File.read!()
      |> Jason.decode!()

    %{"result" => recorded} = Jason.decode!(recording["response"]["body"])
    params = recorded |> JSONRPCTransaction.to_elixir() |> JSONRPCTransaction.elixir_to_params()
    sender = insert(:address, hash: params.from_address_hash, fetched_coin_balance: 0)
    recipient = insert(:address, hash: params.to_address_hash)
    block = insert(:block, hash: params.block_hash, number: params.block_number)

    transaction =
      %Transaction{}
      |> Transaction.changeset(
        Map.merge(params, %{
          cumulative_gas_used: params.gas,
          gas_used: params.gas,
          status: :ok,
          block_consensus: true,
          block_timestamp: block.timestamp
        })
      )
      |> Repo.insert!()

    %{transaction: transaction, block: block, sender: sender, recipient: recipient}
  end

  defp request(path, params \\ %{}, status \\ 200), do: build_conn() |> get(path, params) |> json_response(status)
  defp restore_env(app, key, :error), do: Application.delete_env(app, key)
  defp restore_env(app, key, {:ok, value}), do: Application.put_env(app, key, value)
end
