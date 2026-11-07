defmodule Explorer.Repair.Repair2233280Test do
  @moduledoc """
  Task 22.33.280 / audit 22.30.132 repair record.

  backend-archive-route-doc is proved by "documented routes match the production
  router", "native feed routes publish the recorded submission", "unified account
  and status routes read the indexed transaction without archives", and
  "historical archive routes stay separate from native feed data".
  README.md separates current and historical route inventories and maps the
  frontend resources to registered account, status, signature and feed routes.
  Production Cosmos decoding, PriceFeed persistence, EVM transaction schema,
  Phoenix routing, controllers and JSON views consume committed chain recordings;
  no component is mocked and no live node or external program is called.

  Attempt 1 baseline: no scoped edits or previous gate ledger existed.
  No unchanged-tree verify ran: the explicit ordered gate instructions require
  complete implementation before hf-build and hf-done. No baseline failures
  are claimed. The base README already contained corrected archive paths;
  this change makes the frontend wiring and historical separation explicit.
  Continued from attempt 1 at cdcde9d5a7eeb772219c935d05597e923d3c68df.
  Its four cases passed, but qualification rejected the changed .cgignore and
  differing build/verify candidates. Attempt 2 restores .cgignore and excludes
  inherited deps, _build and .recovery artifacts from the source candidate
  (gotcha 3001); the documentation repair and all assertions are retained.
  hf-build/hf-done retain the verified revision, command, exit and private log.
  No out-of-path implementation or wiring changes are required. Spec and gotcha
  ledgers are outside the authorized scope, so this module is the task record.
  The builder mounts apps but not the root README; the route inventory below
  checks the documented contracts against the compiled production router.
  """

  use Explorer.DataCase, async: false

  import Phoenix.ConnTest

  alias BlockScoutWeb.API.V2.PaxeerX
  alias EthereumJSONRPC.Transaction, as: JSONRPCTransaction
  alias Explorer.Chain.{Address, Transaction}
  alias Explorer.Chain.PaxeerX.PriceFeed
  alias Explorer.PaxeerX.CosmosRest

  @endpoint BlockScoutWeb.Endpoint
  @fixtures Path.expand("../support/fixture", __DIR__)
  @routes [
    {"/v2/paxeer-x/capabilities", PaxeerX.CapabilitiesController, :capabilities},
    {"/v2/addresses/:address_hash_param/unified", PaxeerX.UnifiedAccountController, :unified},
    {"/v2/transactions/:transaction_hash_param/status", PaxeerX.TransactionStatusController, :status},
    {"/v2/transactions/:transaction_hash_param/signature", PaxeerX.TransactionSignatureController, :signature},
    {"/v2/paxeer-x/feed/markets", PaxeerX.FeedController, :markets},
    {"/v2/paxeer-x/feed/markets/:market_id/updates", PaxeerX.FeedController, :market_updates},
    {"/v2/paxeer-x/feed/updates", PaxeerX.FeedController, :updates},
    {"/v2/paxeer-x/feed/updates/:hash", PaxeerX.FeedController, :update},
    {"/v2/paxeer-x/archive/anchors", PaxeerX.ArchiveController, :anchors},
    {"/v2/paxeer-x/archive/receipts", PaxeerX.ArchiveController, :receipts},
    {"/v2/paxeer-x/archive/receipts/:id", PaxeerX.ArchiveController, :receipt}
  ]

  setup_all do
    {:ok, _} = Application.ensure_all_started(:block_scout_web)
    :ok
  end

  setup do
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

    %{update: update, block: block}
  end

  test "documented routes match the production router" do
    registered = Phoenix.Router.routes(BlockScoutWeb.Routers.ApiRouter)

    for {path, controller, action} <- @routes do
      assert Enum.any?(registered, fn route ->
               route.verb == :get and route.path == path and route.plug == controller and route.plug_opts == action
             end),
             "documented GET #{path} must dispatch to #{inspect(controller)}.#{action}"
    end

    controllers = @routes |> Enum.map(&elem(&1, 1)) |> MapSet.new()

    actual =
      registered
      |> Enum.filter(&(&1.verb == :get and MapSet.member?(controllers, &1.plug)))
      |> MapSet.new(&{&1.path, &1.plug, &1.plug_opts})

    assert actual == MapSet.new(@routes)
  end

  test "native feed routes publish the recorded submission", %{update: update, block: block} do
    detail = request("/api/v2/paxeer-x/feed/updates/#{update.hash}")
    assert detail["hash"] == update.hash
    assert detail["block_number"] == block.number
    assert detail["block_hash"] == to_string(block.hash)
    assert detail["method"] == "submitPrices"
    assert detail["transaction_types"] == ["price_update"]
    assert detail["type"] == 127
    assert detail["status"] == "ok"
    assert detail["result"] == "success"
    assert detail["from"]["hash"] == Address.checksum(update.feeder_hash)
    assert detail["price_update"]["feeder_bech32"] == update.feeder_bech32
    assert detail["price_update"]["observation_count"] == 18

    observations = detail["price_update"]["observations"]
    assert length(observations) == 18
    first = Enum.find(observations, &(&1["market_id"] == 0))
    assert first["index_price"] == "82830900000000000000000"
    assert first["mark_price"] == "82830900000000000000000"
    assert first["sequence"] == 63_784
    assert first["source_timestamp_ms"] == 1_792_636_042_680
    assert first["accepted"] == true

    assert request("/api/v2/paxeer-x/feed/updates", %{"feeder" => update.feeder_hash, "market_id" => "0"}) ==
             %{"items" => [detail], "next_page_params" => nil}

    assert request("/api/v2/paxeer-x/feed/markets") == %{"items" => [], "chain_head" => block.number}
    history = request("/api/v2/paxeer-x/feed/markets/0/updates")
    assert history["symbol"] == nil
    assert history["next_page_params"] == nil
    assert [item] = history["items"]
    assert item["update_hash"] == update.hash
    assert item["block_number"] == block.number
    assert item["index_price"] == first["index_price"]
    assert item["sequence"] == first["sequence"]
  end

  test "unified account and status routes read the indexed transaction without archives" do
    [recording] = fixture("vcr_cassettes/transaction_importer_saves_the_association.json")
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

    account = request("/api/v2/addresses/#{sender.hash}/unified")
    assert account["identities"] == %{"evm" => Address.checksum(sender.hash), "pax" => nil, "did" => nil}
    assert [%{"asset" => %{"id" => "native", "decimals" => 18}, "total" => "0"}] = account["balances"]
    assert [%{"kind" => "transaction", "status" => "committed", "result" => "ok"} = item] = account["activity"]
    assert item["hash"] == recorded["hash"]
    assert item["amount"] == Decimal.to_string(transaction.value.value)
    assert item["counterparty"] == Address.checksum(recipient.hash)
    assert account["activity_total"] == 1
    assert account["page_number"] == 1
    assert account["next_page_params"] == nil
    assert is_binary(account["page_cursor"])
    assert is_binary(account["first_page_cursor"])

    replay = request("/api/v2/addresses/#{sender.hash}/unified", %{"cursor" => account["first_page_cursor"]})
    assert replay["activity"] == account["activity"]

    assert %{"rung" => "committed", "block_number" => number, "committed_height" => height} =
             request("/api/v2/transactions/#{transaction.hash}/status")

    assert number == block.number
    assert height >= number
    assert request("/api/v2/paxeer-x/archive/receipts") == %{"items" => [], "next_page_params" => nil}
  end

  test "historical archive routes stay separate from native feed data", %{update: update} do
    for path <- ["anchors", "receipts"] do
      assert request("/api/v2/paxeer-x/archive/#{path}") == %{"items" => [], "next_page_params" => nil}
      assert request("/api/v2/paxeer-x/#{path}", %{}, 404) == %{"message" => "Not found"}
    end

    for prefix <- ["/api/v2/paxeer-x/receipts", "/api/v2/paxeer-x/archive/receipts"] do
      assert request("#{prefix}/#{update.hash}", %{}, 404) == %{"message" => "Not found"}
    end

    for schema <- [
          BlockScoutWeb.Schemas.API.V2.PaxeerX.ArchiveAnchor,
          BlockScoutWeb.Schemas.API.V2.PaxeerX.ArchiveReceipt
        ] do
      contract = schema.schema()
      assert :frozen in contract.required
      assert contract.properties.frozen.enum == [true]
      refute :status in Map.keys(contract.properties)
    end

    assert request("/api/v2/paxeer-x/feed/updates/#{update.hash}")["hash"] == update.hash
  end

  defp fixture(path), do: @fixtures |> Path.join(path) |> File.read!() |> Jason.decode!()

  defp request(path, params \\ %{}, status \\ 200) do
    build_conn() |> get(path, params) |> json_response(status)
  end
end
