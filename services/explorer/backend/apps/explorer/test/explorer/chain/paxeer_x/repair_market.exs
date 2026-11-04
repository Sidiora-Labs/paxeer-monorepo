Code.require_file("repair_transport.exs", __DIR__)

defmodule Explorer.Chain.PaxeerX.Repair2233523.Market do
  import ExUnit.Assertions
  import Explorer.Factory
  import Ecto.Query
  alias Explorer.Repo
  alias Explorer.Chain.Import
  alias Explorer.Chain.PaxeerX.MarketEvent
  alias Explorer.Chain.PaxeerX.Repair2233523.Transport
  alias Indexer.Transform.PaxeerXLogs

  def market_producer_recovery_proof do
    corpus = Path.join(__DIR__, "fixtures/perps_v69_emissions.json") |> File.read!() |> Jason.decode!()
    emitted = for scenario <- corpus["cases"], call <- scenario["calls"], log <- call["logs"] || [], do: log
    bridge_recording = Path.join(__DIR__, "fixtures/node_bridge_out.json") |> File.read!() |> Jason.decode!()
    bridge_logs = bridge_recording["case"]["receiptJson"]["logs"]
    assert length(bridge_logs) == 1
    emitted = emitted ++ Enum.map(bridge_logs, &Map.take(&1, ["address", "topics", "data"]))
    assert length(emitted) == 7
    block = insert(:block)
    transaction = :transaction |> insert() |> with_block(block)
    # Only the indexer's local block/transaction context is attached here; the
    # producer's address, topics and ABI bytes are replayed without re-encoding.
    logs =
      emitted
      |> Enum.with_index()
      |> Enum.map(fn {log, index} ->
        Map.merge(log, %{
          "blockHash" => to_string(block.hash),
          "blockNumber" => EthereumJSONRPC.integer_to_quantity(block.number),
          "transactionHash" => to_string(transaction.hash),
          "transactionIndex" => "0x0",
          "logIndex" => EthereumJSONRPC.integer_to_quantity(index),
          "removed" => false
        })
      end)

    parent = self()
    server = Bypass.open()

    Bypass.expect(server, "POST", "/", fn conn ->
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      request = Jason.decode!(body)
      assert request["method"] == "eth_getLogs"
      assert request["params"] == [%{"blockHash" => to_string(block.hash)}]
      send(parent, :market_logs_fetched)
      Plug.Conn.resp(conn, 200, Jason.encode!(%{"jsonrpc" => "2.0", "id" => request["id"], "result" => logs}))
    end)

    arguments = Transport.arguments("http://127.0.0.1:#{server.port}/")
    previous = Application.get_env(:explorer, :chain_type)
    Application.put_env(:explorer, :chain_type, :paxeer_x)

    try do
      assert {:ok, received} = Transport.rpc("eth_getLogs", [%{blockHash: to_string(block.hash)}], arguments)
      assert received == logs
      assert_received :market_logs_fetched

      parsed =
        received
        |> Enum.map(&(EthereumJSONRPC.Log.to_elixir(&1) |> EthereumJSONRPC.Log.elixir_to_params()))
        |> PaxeerXLogs.parse()

      assert parsed.undecoded_log_count == 0
      rows = parsed.lx_market_events

      assert Enum.map(rows, & &1.event_name) == [
               "MarginDeposited",
               "MarginDeposited",
               "OrderPlaced",
               "OrderCancelRequested",
               "SettlementRequested",
               "MarginWithdrawalRequested",
               "BridgeOut"
             ]

      [native, token, order, cancel, settlement, withdrawal, bridge] = rows
      assert native.parameters["amount"] == 3000
      assert native.parameters["owner"] == "0x53e25771164e4b6d4e588c674d95fc2aacced362"
      assert token.parameters["amount"] == 900
      assert order.parameters["nonce"] == 1
      assert cancel.parameters["nonce"] == 2
      assert settlement.parameters["nonce"] == 3
      assert withdrawal.parameters["nonce"] == 1
      assert bridge.domain == :bridge

      assert bridge.parameters == %{
               "chain" => 1,
               "asset" => "0x5fbdb2315678afecb367f032d93f642f64180aa3",
               "nonce" => 7,
               "amount" => 5_000_000_000_000_000_000,
               "recipient" => "0x3c44cdddb6a900fa2b585dd299e03d12fa4293bc"
             }

      data = %{paxeer_x_market_events: %{params: rows}}
      assert {:ok, %{insert_paxeer_x_market_events: inserted}} = Import.all(data)
      assert length(inserted) == length(emitted)
      reader = Task.async(fn -> Repo.all(order_by(MarketEvent.only_consensus_query(), [row], asc: row.log_index)) end)
      stored = Task.await(reader)
      refute Process.alive?(reader.pid)
      assert length(stored) == length(rows)

      for {actual, expected} <- Enum.zip(stored, rows) do
        assert actual.transaction_hash == transaction.hash
        assert actual.block_hash == block.hash
        assert actual.block_number == block.number
        assert actual.log_index == expected.log_index
        assert actual.block_consensus == true
        assert actual.event_name == expected.event_name
        assert actual.kind == expected.kind
        assert actual.domain == expected.domain
        assert actual.parameters == expected.parameters

        for field <- [:address_hash, :account, :asset_id, :asset_address_hash] do
          value = Map.fetch!(actual, field)
          assert if(is_nil(value), do: nil, else: to_string(value)) == Map.fetch!(expected, field)
        end

        if is_nil(expected.amount),
          do: assert(is_nil(actual.amount)),
          else: assert(Decimal.equal?(actual.amount, Decimal.new(expected.amount)))
      end

      assert {:ok, replayed} = Transport.rpc("eth_getLogs", [%{blockHash: to_string(block.hash)}], arguments)
      assert replayed == received
      assert {:ok, %{insert_paxeer_x_market_events: []}} = Import.all(data)
      reopened = Task.async(fn -> Repo.all(order_by(MarketEvent.only_consensus_query(), [row], asc: row.log_index)) end)
      assert Task.await(reopened) == stored
      assert Repo.aggregate(MarketEvent, :count) == 7
    after
      Transport.restore(:explorer, :chain_type, previous)
    end
  end
end
