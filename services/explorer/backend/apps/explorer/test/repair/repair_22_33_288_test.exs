Code.require_file(
  "../../../block_scout_web/test/block_scout_web/controllers/api/v2/paxeer_x/committed_evidence.exs",
  __DIR__
)

Code.require_file(
  "../../../block_scout_web/test/block_scout_web/controllers/api/v2/paxeer_x/repair_evidence.exs",
  __DIR__
)

defmodule Explorer.Repair.Repair2233288Test do
  @moduledoc """
  Task 22.33.288 closes audit 22.30.133 finding backend-guide-api.
  Proving cases: "native evidence serves live status and signature without
  populating frozen archives", "receipt signatures alone remain receipted",
  and "recorded native feed reaches all four REST reads and both live topics".

  The production endpoint/router, receipt verifier, indexer, database queries,
  realtime subscriber and V2 channels consume committed chain recordings.
  The loopback peers replay the committed evidence helpers; no production
  component is replaced, and no live network, compiler or credential is used.

  Baseline: no prior scoped edits or hf-build/hf-done ledger were present.
  No baseline tests ran or are claimed to have failed: the ordered instructions
  prohibit verification before implementation and require hf-build then hf-done.
  The exact verified revision, command, exit and private log are recorded by
  hf-done; no verification result is inferred from documentation or compilation.

  Wiring request: services/platform/hosted/gateway/src/explorer_proxy.rs,
  owning domain platform hosted gateway: admit the documented live status/signature/feed
  GET interfaces and V2 feed WebSocket upgrade/topics with unchanged semantics.
  This task proves the backend contract, not admission by an external deployment.
  """

  use Explorer.DataCase, async: false

  import Phoenix.ConnTest
  import Phoenix.ChannelTest, only: [socket: 3, subscribe_and_join: 2]

  alias BlockScoutWeb.API.V2.PaxeerX.CommittedEvidence, as: Receipts
  alias BlockScoutWeb.API.V2.PaxeerX.Repair2233416Evidence, as: Feed
  alias BlockScoutWeb.V2.UserSocket
  alias Explorer.Chain
  alias Explorer.Chain.Cache.Counters.AverageBlockTime
  alias Explorer.Chain.PaxeerX.{PriceFeed, PriceObservation, PriceUpdate, SequencerReceipt}
  alias Indexer.Fetcher.PaxeerX.PriceUpdates
  alias Phoenix.Socket.Message

  @endpoint BlockScoutWeb.Endpoint

  setup_all do
    {:ok, _} = Application.ensure_all_started(:block_scout_web)
    :ok
  end

  setup do
    previous = Application.fetch_env(:explorer, SequencerReceipt)
    adapter = Application.fetch_env(:tesla, :adapter)
    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)

    on_exit(fn ->
      Feed.restore_env(:explorer, SequencerReceipt, previous)
      Feed.restore_env(:tesla, :adapter, adapter)
    end)

    :ok
  end

  test "native evidence serves live status and signature without populating frozen archives" do
    vector = Receipts.settlement()
    hash = vector["receipt"]["txHash"]
    {:ok, full_hash} = Chain.string_to_full_hash(hash)
    assert {:error, :not_found} = Chain.hash_to_transaction(full_hash, api?: true)
    calls = Receipts.committed_calls(vector)
    Receipts.serve_rpc(calls)

    assert request("/api/v2/transactions/#{hash}/status") == %{
             "rung" => "committed",
             "block_number" => nil,
             "committed_height" => 10
           }

    Receipts.assert_calls(calls)

    signature = request("/api/v2/transactions/#{hash}/signature")
    key = Receipts.receipt_key()
    assert {:ok, decoded} = SequencerReceipt.decode(Receipts.bytes(vector["receipt"]["receipt"]))

    assert signature == %{
             "rung" => "committed",
             "committed_height" => 10,
             "receipt" => vector["receipt"]["receipt"],
             "receipt_key" => %{"key_id" => key["keyId"], "public_key" => key["publicKey"]},
             "signature" => Receipts.hex(decoded.signature)
           }

    assert {:ok, ^decoded} =
             SequencerReceipt.verify_signature(Receipts.bytes(signature["receipt"]), Receipts.bytes(key["publicKey"]))

    Receipts.assert_calls(calls)
    assert {:error, :not_found} = Chain.hash_to_transaction(full_hash, api?: true)

    for resource <- ~w(anchors receipts) do
      assert request("/api/v2/paxeer-x/archive/#{resource}") == %{"items" => [], "next_page_params" => nil}
      assert request("/api/v2/paxeer-x/#{resource}", 404) == %{"message" => "Not found"}
    end

    for id <- [hash, "invalid"] do
      assert request("/api/v2/paxeer-x/archive/receipts/#{id}", 404) == %{"message" => "Not found"}
    end
  end

  test "receipt signatures alone remain receipted" do
    vector = Receipts.settlement()
    hash = vector["receipt"]["txHash"]

    calls = [
      {"pax_sequencerReceipt", [hash], vector["receipt"]},
      {"px_getReceiptKeys", [], %{"keys" => [Receipts.receipt_key()]}},
      {"eth_getTransactionReceipt", [hash], nil}
    ]

    Receipts.serve_rpc(calls)

    assert request("/api/v2/transactions/#{hash}/status") == %{
             "rung" => "receipted",
             "block_number" => nil,
             "committed_height" => nil
           }

    Receipts.assert_calls(calls)
    signature = request("/api/v2/transactions/#{hash}/signature")
    assert signature["rung"] == "receipted"
    assert signature["receipt"] == vector["receipt"]["receipt"]
    refute Map.has_key?(signature, "committed_height")
    Receipts.assert_calls(calls)

    # Loss of receipt authority must produce HTTP not-found on both live paths.
    Application.put_env(:explorer, SequencerReceipt, router_url: nil)

    for resource <- ~w(status signature) do
      assert request("/api/v2/transactions/#{hash}/#{resource}", 404) == %{"message" => "Not found"}
    end
  end

  test "recorded native feed reaches all four REST reads and both live topics" do
    previous = Application.fetch_env(:explorer, AverageBlockTime)
    on_exit(fn -> Feed.restore_env(:explorer, AverageBlockTime, previous) end)
    Application.put_env(:explorer, AverageBlockTime, enabled: true, cache_period: 1_800_000)
    start_supervised!(AverageBlockTime)
    assert is_pid(Process.whereis(BlockScoutWeb.RealtimeEventHandlers.Main))

    for topic <- ["paxeer_x_feed:new_update", "paxeer_x_feed:market:0"] do
      assert {:ok, %{}, _socket} =
               UserSocket |> socket("guide-2233288", %{}) |> subscribe_and_join(topic)
    end

    for topic <- ["paxeer_x_feed:market:-1", "paxeer_x_feed:market:invalid"] do
      assert {:error, %{reason: "invalid market id"}} =
               UserSocket |> socket("guide-2233288-invalid", %{}) |> subscribe_and_join(topic)
    end

    recorded = Feed.fixture("block_txs.json")
    height = String.to_integer(recorded["block"]["header"]["height"])
    {:ok, timestamp, _} = DateTime.from_iso8601(recorded["block"]["header"]["time"])
    block = insert(:block, number: height, hash: Base.decode64!(recorded["block_id"]["hash"]), timestamp: timestamp)
    state = %{Feed.feed_node() | start_block: height}
    assert :ok = PriceUpdates.run([height], state)
    assert_received {:price_block_read, ^height}
    hash = "0xa57f246774eee2ec5ac6ac506b279d22682af01a62812d9aa642447751b0fadf"
    detail = request("/api/v2/paxeer-x/feed/updates/#{hash}")

    assert detail["hash"] == hash
    assert detail["block_hash"] == to_string(block.hash)
    assert detail["status"] == "ok"
    assert detail["price_update"]["observation_count"] == 18
    raw = get_in(recorded, ["txs", Access.at(0), "body", "messages", Access.at(0), "observations"])
    observations = detail["price_update"]["observations"]
    assert length(observations) == length(raw)

    for {observation, source} <- Enum.zip(observations, raw) do
      assert observation["market_id"] == String.to_integer(source["market_id"])
      assert observation["sequence"] == String.to_integer(source["sequence"])
      assert observation["index_price"] == source["index_price"]
      assert observation["mark_price"] == source["mark_price"]
      assert observation["source_timestamp_ms"] == String.to_integer(source["source_timestamp_ms"])
      assert observation["accepted"] == true
    end

    assert request("/api/v2/paxeer-x/feed/updates") == %{"items" => [detail], "next_page_params" => nil}
    markets = request("/api/v2/paxeer-x/feed/markets")
    assert markets["chain_head"] == height
    assert length(markets["items"]) == 18
    assert Enum.all?(markets["items"], &(&1["latest"]["update_hash"] == hash))
    history = request("/api/v2/paxeer-x/feed/markets/0/updates")
    assert [item] = history["items"]
    assert history["next_page_params"] == nil
    assert item["update_hash"] == hash

    assert_receive %Message{topic: "paxeer_x_feed:new_update", event: "new_update", payload: payload}, 5_000
    assert json(payload) == detail
    assert_receive %Message{topic: "paxeer_x_feed:market:0", event: "new_update", payload: market_payload}, 5_000
    assert json(market_payload) == item
    assert Repo.aggregate(PriceUpdate, :count) == 1
    assert Repo.aggregate(PriceObservation, :count) == 18

    assert request("/api/v2/paxeer-x/archive/receipts") == %{"items" => [], "next_page_params" => nil}
    Repo.update!(Ecto.Changeset.change(block, consensus: false))
    assert request("/api/v2/paxeer-x/feed/updates/#{hash}", 404) == %{"message" => "Not found"}
    assert request("/api/v2/paxeer-x/feed/updates")["items"] == []
    assert request("/api/v2/paxeer-x/feed/markets/0/updates")["items"] == []
    assert Enum.all?(request("/api/v2/paxeer-x/feed/markets")["items"], &is_nil(&1["latest"]))
    assert PriceFeed.block_update_hashes(height) == [hash]
  end

  defp request(path, status \\ 200), do: build_conn() |> get(path) |> json_response(status)
  defp json(value), do: value |> Jason.encode!() |> Jason.decode!()
end
