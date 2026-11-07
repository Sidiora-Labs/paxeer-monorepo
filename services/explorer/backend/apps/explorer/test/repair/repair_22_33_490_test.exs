defmodule Explorer.Repair.Repair2233490Test do
  @moduledoc """
  Task 22.33.490 / audit 22.30.150, finding 022 (doc-drift).
  Proving tests: "022: documented archive and current routes match the router",
  "022: historical checkpoint and receipt reads remain frozen beside committed status",
  and "022: recorded native feeds use current routes without creating archive history".
  Recorded EVM transaction and Cosmos block/results responses pass through the
  production decoders, schemas, database, Phoenix router, controllers and views.
  Historical checkpoint/status attributes are archive schema inputs, not claims
  of current verification. No production component or transport is mocked.

  Baseline b66a12fbea8d4662e36b70ae3e870d065605b3fd: no scoped edits or prior
  gate ledger existed. The README already listed the corrected archive paths;
  this repair clarifies their retired lifecycle and purpose beside live APIs.
  No unchanged-tree verification ran because the explicit gate order requires
  complete implementation before hf-build and hf-done; no pre-change test
  failure is claimed. Those gates retain revision, command, exit and private log.
  No out-of-path wiring is required. This module is the scoped repair record;
  spec and GOTCHA.kvx edits are outside the authorized paths. The builder mounts
  apps, not the root README, so the documented route inventory below is checked
  against the compiled router and exercised through real HTTP-shaped requests.
  """

  use Explorer.DataCase, async: false

  import Phoenix.ConnTest

  alias BlockScoutWeb.API.V2.PaxeerX
  alias EthereumJSONRPC.Transaction, as: RPCTransaction
  alias Explorer.Chain.Transaction
  alias Explorer.Chain.PaxeerX.{Anchor, PriceFeed, Receipt}
  alias Explorer.PaxeerX.CosmosRest

  @endpoint BlockScoutWeb.Endpoint
  @fixtures Path.expand("../support/fixture", __DIR__)
  @routes [
    {"/v2/paxeer-x/archive/anchors", PaxeerX.ArchiveController, :anchors},
    {"/v2/paxeer-x/archive/receipts", PaxeerX.ArchiveController, :receipts},
    {"/v2/paxeer-x/archive/receipts/:id", PaxeerX.ArchiveController, :receipt},
    {"/v2/transactions/:transaction_hash_param/status", PaxeerX.TransactionStatusController, :status},
    {"/v2/transactions/:transaction_hash_param/signature", PaxeerX.TransactionSignatureController, :signature},
    {"/v2/paxeer-x/feed/markets", PaxeerX.FeedController, :markets},
    {"/v2/paxeer-x/feed/markets/:market_id/updates", PaxeerX.FeedController, :market_updates},
    {"/v2/paxeer-x/feed/updates", PaxeerX.FeedController, :updates},
    {"/v2/paxeer-x/feed/updates/:hash", PaxeerX.FeedController, :update}
  ]

  setup_all do
    {:ok, _} = Application.ensure_all_started(:block_scout_web)
    :ok
  end

  test "022: documented archive and current routes match the router" do
    controllers = Enum.map(@routes, &elem(&1, 1))

    actual =
      BlockScoutWeb.Routers.ApiRouter
      |> Phoenix.Router.routes()
      |> Enum.filter(&(&1.verb == :get and &1.plug in controllers))
      |> MapSet.new(&{&1.path, &1.plug, &1.plug_opts})

    assert actual == MapSet.new(@routes)

    for schema <- [
          BlockScoutWeb.Schemas.API.V2.PaxeerX.ArchiveAnchor,
          BlockScoutWeb.Schemas.API.V2.PaxeerX.ArchiveReceipt
        ] do
      contract = schema.schema()
      assert :frozen in contract.required
      assert contract.properties.frozen.enum == [true]
      refute Map.has_key?(contract.properties, :status)
    end
  end

  test "022: historical checkpoint and receipt reads remain frozen beside committed status" do
    [exchange] = fixture("vcr_cassettes/transaction_importer_saves_the_association.json")
    %{"result" => recorded} = Jason.decode!(exchange["response"]["body"])
    params = recorded |> RPCTransaction.to_elixir() |> RPCTransaction.elixir_to_params()
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

    common = %{
      transaction_hash: transaction.hash,
      block_hash: block.hash,
      block_number: block.number,
      block_consensus: true,
      log_index: 0
    }

    anchor_attrs =
      Map.merge(common, %{
        batch_number: 4211,
        checkpoint_id: recorded["hash"],
        state_root: to_string(block.hash),
        kernel_height: 918_244,
        sealed_height: 918_100,
        status: :final
      })

    %Anchor{} |> Ecto.Changeset.cast(anchor_attrs, Map.keys(anchor_attrs)) |> Repo.insert!()

    assert %{"items" => [anchor], "next_page_params" => nil} = request("/api/v2/paxeer-x/archive/anchors")
    assert anchor["frozen"] == true
    assert anchor["batch_number"] == 4211
    assert anchor["checkpoint_id"] == recorded["hash"]
    assert anchor["checkpoint_height"] == 918_244
    assert anchor["sealed_height"] == 918_100
    assert anchor["block_number"] == block.number
    refute Map.has_key?(anchor, "rung")

    for {historical_status, index} <- Enum.with_index(Receipt.statuses()) do
      attrs =
        Map.merge(common, %{
          receipt_id: recorded["hash"],
          log_index: index,
          status: historical_status
        })

      receipt = %Receipt{} |> Ecto.Changeset.cast(attrs, Map.keys(attrs)) |> Repo.insert!()
      path = "/api/v2/paxeer-x/archive/receipts/#{receipt.receipt_id}"
      archived = request(path)
      assert archived["frozen"] == true
      assert archived["id"] == recorded["hash"]
      assert archived["transaction_hash"] == recorded["hash"]
      assert archived["verification_status"] == Atom.to_string(historical_status)
      refute Map.has_key?(archived, "status")
      refute Map.has_key?(archived, "rung")
      assert request("/api/v2/paxeer-x/archive/receipts") == %{"items" => [archived], "next_page_params" => nil}

      status = request("/api/v2/transactions/#{transaction.hash}/status")
      assert status["rung"] == "committed"
      assert status["block_number"] == block.number
      assert status["committed_height"] >= block.number
      refute Map.has_key?(status, "frozen")
      assert request(path) == archived
      assert Repo.get!(Receipt, receipt.id).status == historical_status
      Repo.delete!(receipt)
    end
  end

  test "022: recorded native feeds use current routes without creating archive history" do
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
    assert detail["method"] == "submitPrices"
    assert detail["transaction_types"] == ["price_update"]
    assert detail["price_update"]["observation_count"] == 18
    assert length(detail["price_update"]["observations"]) == 18
    observation = Enum.find(detail["price_update"]["observations"], &(&1["market_id"] == 0))
    assert observation["index_price"] == "82830900000000000000000"
    assert observation["accepted"] == true

    assert request("/api/v2/paxeer-x/feed/updates") == %{"items" => [detail], "next_page_params" => nil}
    assert request("/api/v2/paxeer-x/feed/markets") == %{"items" => [], "chain_head" => block.number}

    assert %{"items" => [market_update], "next_page_params" => nil} =
             request("/api/v2/paxeer-x/feed/markets/0/updates")

    assert market_update["update_hash"] == update.hash
    assert market_update["index_price"] == observation["index_price"]

    for path <- ["anchors", "receipts"] do
      assert request("/api/v2/paxeer-x/archive/#{path}") == %{"items" => [], "next_page_params" => nil}
      assert request("/api/v2/paxeer-x/#{path}", 404) == %{"message" => "Not found"}
    end

    for prefix <- ["/api/v2/paxeer-x/receipts", "/api/v2/paxeer-x/archive/receipts"] do
      assert request("#{prefix}/#{update.hash}", 404) == %{"message" => "Not found"}
    end

    assert request("/api/v2/paxeer-x/archive/receipts/invalid", 404) == %{"message" => "Not found"}
    assert request("/api/v2/paxeer-x/feed/updates/#{update.hash}") == detail
  end

  defp fixture(path), do: @fixtures |> Path.join(path) |> File.read!() |> Jason.decode!()

  defp request(path, status \\ 200), do: build_conn() |> get(path) |> json_response(status)
end
