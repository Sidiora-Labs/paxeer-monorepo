defmodule Explorer.Repair.Repair2233527Test do
  @moduledoc """
  Task 22.33.527 / audit 22.30.154 repair record.

  Finding backend_archive_routes is proved by the three cases named with that
  finding below: README examples match the production router; archive reads
  preserve historical schema values; recorded native feed submissions and an
  indexed EVM transaction reach the separate live controllers.

  Chain inputs are committed JSON-RPC and Cosmos/CometBFT recordings decoded by
  production adapters. Historical anchor/receipt schema rows are explicit test
  inputs associated with the recorded transaction, not a claim that its receipt
  emitted an anchor event. Requests traverse the production Phoenix endpoint,
  router, controllers, queries and JSON views without replacing those components.

  Baseline: no scoped edits or previous gate ledger existed. The base README
  already used relative archive paths; this repair makes their full paths and
  read-only lifecycle explicit and links the live controllers separately.
  No unchanged-tree tests ran and no baseline failures are claimed: the ordered
  implementation-first instructions require hf-build then hf-done and prohibit
  a separate preliminary verify. The gates retain the verified revision,
  command, exit code and private evidence. This module holds the repair record
  because editing spec/ is prohibited. No out-of-path wiring is required.
  """

  use Explorer.DataCase, async: false

  import Phoenix.ConnTest

  alias BlockScoutWeb.API.V2.PaxeerX
  alias EthereumJSONRPC.Transaction, as: JSONRPCTransaction
  alias Explorer.Chain.Transaction
  alias Explorer.Chain.PaxeerX.{Anchor, Capabilities, PriceFeed, Receipt}
  alias Explorer.PaxeerX.CosmosRest

  @endpoint BlockScoutWeb.Endpoint
  @readme Path.expand("../../../../README.md", __DIR__)
  @fixtures Path.expand("../support/fixture/paxeer_x/price_feed", __DIR__)

  setup_all do
    {:ok, _} = Application.ensure_all_started(:block_scout_web)
    :ok
  end

  test "backend_archive_routes README examples resolve to their separate production controllers" do
    readme = File.read!(@readme)
    [_, tables] = String.split(readme, "The registered read APIs below", parts: 2)
    [tables, _] = String.split(tables, "### Frontend integration", parts: 2)

    examples =
      for [_, path] <- Regex.scan(~r/^\| `([^`]+)` \|/m, tables) do
        if String.starts_with?(path, "/api/v2/"), do: path, else: "/api/v2" <> path
      end

    expected = [
      {"/api/v2/paxeer-x/capabilities", PaxeerX.CapabilitiesController, :capabilities},
      {"/api/v2/addresses/:address_hash_param/unified", PaxeerX.UnifiedAccountController, :unified},
      {"/api/v2/transactions/:transaction_hash_param/status", PaxeerX.TransactionStatusController, :status},
      {"/api/v2/transactions/:transaction_hash_param/signature", PaxeerX.TransactionSignatureController, :signature},
      {"/api/v2/paxeer-x/feed/markets", PaxeerX.FeedController, :markets},
      {"/api/v2/paxeer-x/feed/markets/:market_id/updates", PaxeerX.FeedController, :market_updates},
      {"/api/v2/paxeer-x/feed/updates", PaxeerX.FeedController, :updates},
      {"/api/v2/paxeer-x/feed/updates/:hash", PaxeerX.FeedController, :update},
      {"/api/v2/paxeer-x/archive/anchors", PaxeerX.ArchiveController, :anchors},
      {"/api/v2/paxeer-x/archive/receipts", PaxeerX.ArchiveController, :receipts},
      {"/api/v2/paxeer-x/archive/receipts/:id", PaxeerX.ArchiveController, :receipt}
    ]

    assert examples == Enum.map(expected, &elem(&1, 0))
    routes = Phoenix.Router.routes(BlockScoutWeb.Routers.ApiRouter)

    for {path, controller, action} <- expected do
      assert Enum.any?(routes, fn route ->
               route.verb == :get and "/api" <> route.path == path and route.plug == controller and
                 route.plug_opts == action
             end)
    end

    archive_routes = Enum.filter(routes, &String.starts_with?(&1.path, "/v2/paxeer-x/archive/"))
    assert length(archive_routes) == 3
    assert Enum.all?(archive_routes, &(&1.verb == :get))
    assert tables =~ "Historical anchor and old receipt rows are read-only data."

    for resource <- ["anchors", "receipts", "receipts/:id"] do
      assert tables =~ "`/api/v2/paxeer-x/archive/#{resource}`"
      refute tables =~ "`/api/v2/paxeer-x/#{resource}`"
      refute tables =~ "`/paxeer-x/#{resource}`"
    end

    for controller <- ~w(capabilities feed transaction_status) do
      assert tables =~
               "(apps/block_scout_web/lib/block_scout_web/controllers/api/v2/paxeer_x/#{controller}_controller.ex)"
    end

    for resource <- ["anchors", "receipts", "receipts/" <> to_string(recorded_transaction().hash)] do
      assert request("/api/v2/paxeer-x/#{resource}", 404) == %{"message" => "Not found"}
    end
  end

  test "backend_archive_routes publish historical rows as frozen read-only data" do
    transaction = recorded_transaction()
    block = Repo.get!(Explorer.Chain.Block, transaction.block_hash)

    anchor =
      persist(Anchor, %{
        transaction_hash: transaction.hash,
        log_index: 0,
        block_hash: block.hash,
        block_number: block.number,
        block_consensus: true,
        batch_number: block.number,
        checkpoint_id: block.hash,
        state_root: block.hash,
        kernel_height: block.number,
        sealed_height: block.number,
        status: :final
      })

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
      receipt =
        persist(Receipt, %{
          receipt_id: transaction.hash,
          transaction_hash: transaction.hash,
          log_index: 1,
          block_hash: block.hash,
          block_number: block.number,
          block_consensus: true,
          status: status
        })

      detail = request("/api/v2/paxeer-x/archive/receipts/#{receipt.receipt_id}")
      assert detail["id"] == to_string(transaction.hash)
      assert detail["transaction_hash"] == to_string(transaction.hash)
      assert detail["verification_status"] == Atom.to_string(status)
      assert detail["frozen"] == true
      refute Map.has_key?(detail, "status")
      refute Map.has_key?(detail, "rung")
      assert request("/api/v2/paxeer-x/archive/receipts") == %{"items" => [detail], "next_page_params" => nil}

      assert request("/api/v2/transactions/#{transaction.hash}/status") == %{
               "rung" => "committed",
               "block_number" => block.number,
               "committed_height" => block.number
             }

      assert request("/api/v2/paxeer-x/archive/receipts/#{receipt.receipt_id}") == detail
      assert Repo.get!(Receipt, receipt.id) == receipt
      Repo.delete!(receipt)
    end

    assert Repo.get_by!(Anchor, transaction_hash: transaction.hash, log_index: 0) == anchor

    for id <- ["invalid", "0x" <> String.duplicate("0", 64)] do
      assert request("/api/v2/paxeer-x/archive/receipts/#{id}", 404) == %{"message" => "Not found"}
    end
  end

  test "backend_archive_routes remain separate from recorded native feed and capability reads" do
    body = read_fixture("block_txs.json")
    results = read_fixture("block_results.json")
    raw = body["block"]["data"]["txs"]
    assert {:ok, submissions} = CosmosRest.decode_block_txs(%{"txs" => body["txs"], "raw" => raw})
    assert {:ok, [update]} = CosmosRest.decode_price_updates(submissions, results["txs_results"], length(raw))
    assert {:ok, timestamp, _} = DateTime.from_iso8601(body["block"]["header"]["time"])

    block =
      insert(:block,
        hash: "0x" <> Base.encode16(Base.decode64!(body["block_id"]["hash"]), case: :lower),
        number: String.to_integer(body["block"]["header"]["height"]),
        timestamp: timestamp
      )

    assert {:ok, 1} = PriceFeed.replace_block_updates(block, [update])
    assert %{"items" => [item], "next_page_params" => nil} = request("/api/v2/paxeer-x/feed/updates")
    assert item["block_number"] == block.number
    assert item["hash"] == update.hash
    assert item["method"] == "submitPrices"
    assert item["transaction_types"] == ["price_update"]
    assert item["status"] == "ok"
    assert item["price_update"]["observation_count"] == 18
    assert request("/api/v2/paxeer-x/feed/updates/#{item["hash"]}") == item

    snapshot = Capabilities.all()
    capabilities = request("/api/v2/paxeer-x/capabilities")

    assert Map.take(capabilities, ~w(addr exchange bridge launchpad checked_at)) == %{
             "addr" => snapshot.unified_account,
             "exchange" => snapshot.exchange,
             "bridge" => snapshot.bridge,
             "launchpad" => snapshot.launchpad,
             "checked_at" => snapshot.checked_at && DateTime.to_iso8601(snapshot.checked_at)
           }

    for resource <- ~w(anchors receipts) do
      assert request("/api/v2/paxeer-x/archive/#{resource}") == %{"items" => [], "next_page_params" => nil}
    end
  end

  defp recorded_transaction do
    [recording] =
      "../support/fixture/vcr_cassettes/transaction_importer_saves_the_association.json"
      |> Path.expand(__DIR__)
      |> File.read!()
      |> Jason.decode!()

    %{"result" => recorded} = Jason.decode!(recording["response"]["body"])
    params = recorded |> JSONRPCTransaction.to_elixir() |> JSONRPCTransaction.elixir_to_params()
    insert(:address, hash: params.from_address_hash)
    insert(:address, hash: params.to_address_hash)
    block = insert(:block, hash: params.block_hash, number: params.block_number)

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
  end

  defp persist(schema, attrs), do: schema |> struct() |> Ecto.Changeset.cast(attrs, Map.keys(attrs)) |> Repo.insert!()
  defp request(path, status \\ 200), do: build_conn() |> get(path) |> json_response(status)
  defp read_fixture(name), do: @fixtures |> Path.join(name) |> File.read!() |> Jason.decode!()
end
