defmodule Explorer.Repair.Repair2233318Test do
  @moduledoc """
  Task 22.33.318 / audit 22.30.136 repair record.

  backend-readme-retired-routes is proved by "documented archive and feed routes
  dispatch to the production controllers", "recorded native submissions reach
  the documented feed endpoints", "archive receipts expose frozen historical
  verification_status", and "retired routes and unknown archive ids return 404".
  The README lists explicit archive paths and registered feed paths, distinguishes
  their response shapes, and explains why historical verification_status must
  not be polled for current settlement. The tests exercise the production Cosmos
  decoder, persistence, receipt schema, router, controllers and JSON views.
  Chain inputs are committed Cosmos block/results and EVM transaction recordings;
  historical receipt statuses are schema test inputs, not new verification claims.

  Baseline at 876fbe3211af9ef09a8e5589f822c0e986d0a140: no prior gate ledger or
  scoped changes existed. Archive and feed paths were already documented; the
  frozen verification_status explanation was missing. No unchanged-tree verify
  ran because the explicit ordered gates require complete implementation first.
  No baseline test failure is claimed. hf-build and hf-done retain the exact
  qualified revision, command, exit code and private evidence log.
  No out-of-path wiring changes are required. This module is the scoped task
  record because spec and GOTCHA.kvx edits are outside the authorized paths.
  The builder mounts apps, but not the root README; the route inventory below
  checks the documented contracts against the compiled production router.
  """

  use Explorer.DataCase, async: false

  import Phoenix.ConnTest

  alias BlockScoutWeb.API.V2.PaxeerX
  alias EthereumJSONRPC.Transaction, as: JSONRPCTransaction
  alias Explorer.Chain.PaxeerX.{PriceFeed, Receipt}
  alias Explorer.Chain.Transaction
  alias Explorer.PaxeerX.CosmosRest

  @endpoint BlockScoutWeb.Endpoint
  @fixtures Path.expand("../support/fixture", __DIR__)
  @routes [
    {"/v2/paxeer-x/archive/anchors", PaxeerX.ArchiveController, :anchors},
    {"/v2/paxeer-x/archive/receipts", PaxeerX.ArchiveController, :receipts},
    {"/v2/paxeer-x/archive/receipts/:id", PaxeerX.ArchiveController, :receipt},
    {"/v2/paxeer-x/feed/markets", PaxeerX.FeedController, :markets},
    {"/v2/paxeer-x/feed/markets/:market_id/updates", PaxeerX.FeedController, :market_updates},
    {"/v2/paxeer-x/feed/updates", PaxeerX.FeedController, :updates},
    {"/v2/paxeer-x/feed/updates/:hash", PaxeerX.FeedController, :update}
  ]

  setup_all do
    {:ok, _} = Application.ensure_all_started(:block_scout_web)
    :ok
  end

  test "documented archive and feed routes dispatch to the production controllers" do
    registered = Phoenix.Router.routes(BlockScoutWeb.Routers.ApiRouter)

    actual =
      registered
      |> Enum.filter(&(&1.verb == :get and &1.plug in [PaxeerX.ArchiveController, PaxeerX.FeedController]))
      |> MapSet.new(&{&1.path, &1.plug, &1.plug_opts})

    assert actual == MapSet.new(@routes)
  end

  test "recorded native submissions reach the documented feed endpoints" do
    body = fixture("paxeer_x/price_feed/block_txs.json")
    results = fixture("paxeer_x/price_feed/block_results.json")
    raw = body["block"]["data"]["txs"]

    assert {:ok, submissions} = CosmosRest.decode_block_txs(%{"txs" => body["txs"], "raw" => raw})
    assert {:ok, [update]} = CosmosRest.decode_price_updates(submissions, results["txs_results"], length(raw))

    header = body["block"]["header"]
    {:ok, timestamp, _offset} = DateTime.from_iso8601(header["time"])
    hash = "0x" <> Base.encode16(Base.decode64!(body["block_id"]["hash"]), case: :lower)
    block = insert(:block, hash: hash, number: String.to_integer(header["height"]), timestamp: timestamp)
    assert {:ok, 1} = PriceFeed.replace_block_updates(block, [update])

    detail = request("/api/v2/paxeer-x/feed/updates/#{update.hash}")
    assert detail["hash"] == update.hash
    assert detail["block_number"] == block.number
    assert detail["block_hash"] == to_string(block.hash)
    assert detail["method"] == "submitPrices"
    assert detail["transaction_types"] == ["price_update"]
    assert detail["price_update"]["observation_count"] == 18
    assert length(detail["price_update"]["observations"]) == 18
    observation = Enum.find(detail["price_update"]["observations"], &(&1["market_id"] == 0))
    assert observation["index_price"] == "82830900000000000000000"
    assert observation["mark_price"] == "82830900000000000000000"
    assert observation["sequence"] == 63_784
    assert observation["source_timestamp_ms"] == 1_792_636_042_680
    assert observation["accepted"] == true

    assert request("/api/v2/paxeer-x/feed/updates", %{"feeder" => update.feeder_hash, "market_id" => "0"}) ==
             %{"items" => [detail], "next_page_params" => nil}

    assert request("/api/v2/paxeer-x/feed/markets") == %{"items" => [], "chain_head" => block.number}
    history = request("/api/v2/paxeer-x/feed/markets/0/updates")
    assert history["symbol"] == nil
    assert history["next_page_params"] == nil
    assert [item] = history["items"]
    assert item["update_hash"] == update.hash
    assert item["block_number"] == block.number
    assert item["index_price"] == observation["index_price"]
    assert item["sequence"] == observation["sequence"]

    assert request("/api/v2/paxeer-x/archive/anchors") == %{"items" => [], "next_page_params" => nil}
    assert request("/api/v2/paxeer-x/archive/receipts") == %{"items" => [], "next_page_params" => nil}
  end

  test "archive receipts expose frozen historical verification_status" do
    [recording] = fixture("vcr_cassettes/transaction_importer_saves_the_association.json")
    %{"result" => recorded} = Jason.decode!(recording["response"]["body"])
    params = recorded |> JSONRPCTransaction.to_elixir() |> JSONRPCTransaction.elixir_to_params()
    insert(:address, hash: params.from_address_hash)
    insert(:address, hash: params.to_address_hash)
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

    for {status, index} <- Enum.with_index(Receipt.statuses()) do
      attrs = %{
        receipt_id: recorded["hash"],
        transaction_hash: transaction.hash,
        block_hash: block.hash,
        block_number: block.number,
        block_consensus: true,
        log_index: index,
        status: status
      }

      receipt = %Receipt{} |> Ecto.Changeset.cast(attrs, Map.keys(attrs)) |> Repo.insert!()
      response = request("/api/v2/paxeer-x/archive/receipts/#{receipt.receipt_id}")
      assert response["id"] == recorded["hash"]
      assert response["transaction_hash"] == recorded["hash"]
      assert response["frozen"] == true
      assert response["verification_status"] == Atom.to_string(status)
      refute Map.has_key?(response, "status")
      assert request("/api/v2/paxeer-x/archive/receipts") == %{"items" => [response], "next_page_params" => nil}
      assert request("/api/v2/paxeer-x/archive/receipts/#{receipt.receipt_id}") == response
      assert Repo.get!(Receipt, receipt.id).status == status
      Repo.delete!(receipt)
    end
  end

  test "retired routes and unknown archive ids return 404" do
    for path <- ["anchors", "receipts", "receipts/0x" <> String.duplicate("00", 32)] do
      assert request("/api/v2/paxeer-x/#{path}", %{}, 404) == %{"message" => "Not found"}
    end

    for id <- ["invalid", "0x" <> String.duplicate("00", 32)] do
      assert request("/api/v2/paxeer-x/archive/receipts/#{id}", %{}, 404) == %{"message" => "Not found"}
    end
  end

  defp fixture(path), do: @fixtures |> Path.join(path) |> File.read!() |> Jason.decode!()

  defp request(path, params \\ %{}, status \\ 200) do
    build_conn() |> get(path, params) |> json_response(status)
  end
end
