defmodule Indexer.Repair2233554Test do
  use Explorer.DataCase, async: false

  import ExUnit.CaptureLog, only: [capture_log: 1]

  alias Explorer.Chain.Beacon.Deposit
  alias Indexer.Fetcher.Beacon.Deposit, as: DepositFetcher

  @fixture Path.expand("../indexer/fetcher/beacon/fixtures/deposit_recovery.json", __DIR__)
  @first_index 2_583_870

  setup_all do
    for name <- ["20250813001523_create_deposits.exs", "20251029090030_deposits_remove_foreign_keys.exs"] do
      Code.require_file(Application.app_dir(:explorer, "priv/beacon/migrations/#{name}"))
    end

    :ok
  end

  setup do
    # The declared gate uses the default chain configuration. Provision the real
    # Beacon schema in the sandbox transaction even when Ethereum is not selected.
    if Application.get_env(:explorer, :chain_type) != :ethereum do
      for {version, migration} <- [
            {20_250_813_001_523, Explorer.Repo.Beacon.Migrations.CreateDeposits},
            {20_251_029_090_030, Explorer.Repo.Beacon.Migrations.DepositsRemoveForeignKeys}
          ] do
        Ecto.Migration.Runner.run(Repo, Repo.config(), version, migration, :forward, :change, :up, log: false)
      end
    end

    transcript = @fixture |> File.read!() |> Jason.decode!()
    logs = transcript["logs"]["response"]["result"]
    blocks = Map.new(transcript["blocks"], &{&1["response"]["result"]["number"], &1["response"]["result"]})
    address = insert(:address, hash: hd(logs)["address"])

    %{logs: logs, blocks: blocks, address: address}
  end

  test "008: recovers a gap before the first log between batches", context do
    recover(context, [1, 2], 1, 0, 2)
  end

  test "008: recovers a gap between logs in separate batches", context do
    recover(context, [0, 2], 1, 1, 2)
  end

  test "008: recovers a gap before the first log within a batch", context do
    recover(context, [1, 2], 5, 0, 1)
  end

  test "008: recovers a gap between logs within a batch", context do
    recover(context, [0, 2], 5, 1, 1)
  end

  test "008: rejects a separately returned wrong-topic response without importing the batch", context do
    seed_logs(context, [1, 2])
    raw_log = hd(context.logs)
    poisoned = %{raw_log | "topics" => ["0x" <> String.duplicate("00", 32)]}
    state = state(context, 5, serve(context, poisoned, "0x0", Enum.at(context.logs, 1)["blockNumber"]))
    previous_trap_exit = Process.flag(:trap_exit, true)

    try do
      output =
        capture_log(fn ->
          assert {:noreply, ^state} = DepositFetcher.handle_info(:process_logs, state)
        end)

      assert output =~ "Failed to fetch deposit logs from node"
      assert Repo.all(Deposit) == []
      assert_rpc_requests(raw_log, "0x0", Enum.at(context.logs, 1)["blockNumber"])
    after
      Process.flag(:trap_exit, previous_trap_exit)
    end
  end

  defp recover(context, local_positions, batch_size, missing_position, passes) do
    seed_logs(context, local_positions)
    raw_log = Enum.at(context.logs, missing_position)
    from_block = if missing_position == 0, do: "0x0", else: hd(context.logs)["blockNumber"]
    to_block = Enum.at(context.logs, missing_position + 1)["blockNumber"]
    initial_state = state(context, batch_size, serve(context, raw_log, from_block, to_block))

    capture_log(fn ->
      final_state =
        Enum.reduce(1..passes, initial_state, fn pass, current_state ->
          assert {:noreply, next_state} = DepositFetcher.handle_info(:process_logs, current_state)

          if batch_size == 1 and pass == 1 do
            expected_position = if missing_position == 0, do: 1, else: 0
            assert next_state.deposit_index == @first_index + expected_position
            assert Repo.aggregate(Deposit, :count) == expected_position + 1
          end

          next_state
        end)

      assert final_state.deposit_index == @first_index + 2
      assert final_state.last_processed_log_block_number == quantity(List.last(context.logs)["blockNumber"])
      assert final_state.last_processed_log_index == quantity(List.last(context.logs)["logIndex"])
    end)

    deposits = Repo.all(from(deposit in Deposit, order_by: deposit.index))
    assert Enum.map(deposits, & &1.index) == Enum.to_list(@first_index..(@first_index + 2))

    for {deposit, raw} <- Enum.zip(deposits, context.logs) do
      assert to_string(deposit.transaction_hash) == raw["transactionHash"]
      assert to_string(deposit.block_hash) == raw["blockHash"]
      assert deposit.block_number == quantity(raw["blockNumber"])
      assert deposit.log_index == quantity(raw["logIndex"])

      assert DateTime.compare(
               deposit.block_timestamp,
               DateTime.from_unix!(quantity(context.blocks[raw["blockNumber"]]["timestamp"]))
             ) == :eq

      assert deposit.status == :pending
      assert Decimal.equal?(Explorer.Chain.Wei.to(deposit.amount, :gwei), 32_000_000_000)
    end

    assert to_string(Enum.at(deposits, missing_position).from_address_hash) == raw_log["address"]
    assert_rpc_requests(raw_log, from_block, to_block)
  end

  defp seed_logs(context, positions) do
    for position <- positions do
      raw = Enum.at(context.logs, position)
      recorded_block = context.blocks[raw["blockNumber"]]

      block =
        insert(:block,
          hash: raw["blockHash"],
          number: quantity(raw["blockNumber"]),
          timestamp: DateTime.from_unix!(quantity(recorded_block["timestamp"]))
        )

      transaction =
        insert(:transaction, hash: raw["transactionHash"])
        |> with_block(block, index: quantity(raw["transactionIndex"]), status: :ok)

      params = raw |> EthereumJSONRPC.Log.to_elixir() |> EthereumJSONRPC.Log.elixir_to_params()

      insert(:log,
        address: context.address,
        transaction: transaction,
        block: block,
        block_number: params.block_number,
        index: params.index,
        first_topic: params.first_topic,
        data: params.data
      )
    end
  end

  defp state(context, batch_size, json_rpc_named_arguments) do
    %DepositFetcher{
      interval: :timer.hours(1),
      batch_size: batch_size,
      deposit_contract_address_hash: to_string(context.address.hash),
      domain_deposit: <<3, 0, 0, 0>>,
      genesis_fork_version: <<0, 0, 0, 0>>,
      deposit_index: @first_index - 1,
      last_processed_log_block_number: -1,
      last_processed_log_index: -1,
      json_rpc_named_arguments: json_rpc_named_arguments
    }
  end

  defp serve(context, raw_log, from_block, to_block) do
    rpc = Bypass.open()
    owner = self()

    Bypass.expect(rpc, "POST", "/", fn conn ->
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      request = Jason.decode!(body)
      send(owner, {:rpc_request, request})

      response =
        case request do
          %{"id" => id, "method" => "eth_getLogs", "params" => [filter]} ->
            assert filter == %{
                     "address" => to_string(context.address.hash),
                     "topics" => [Deposit.event_signature()],
                     "fromBlock" => request_quantity(from_block),
                     "toBlock" => request_quantity(to_block)
                   }

            %{"jsonrpc" => "2.0", "id" => id, "result" => [raw_log]}

          [%{"id" => id, "method" => "eth_getBlockByNumber", "params" => [number, false]}] ->
            assert number == raw_log["blockNumber"]
            [%{"jsonrpc" => "2.0", "id" => id, "result" => Map.fetch!(context.blocks, number)}]
        end

      conn |> Plug.Conn.put_resp_content_type("application/json") |> Plug.Conn.resp(200, Jason.encode!(response))
    end)

    [
      transport: EthereumJSONRPC.HTTP,
      transport_options: [
        http: EthereumJSONRPC.HTTP.HTTPoison,
        urls: ["http://127.0.0.1:#{rpc.port}"],
        http_options: [recv_timeout: 5_000, timeout: 5_000]
      ]
    ]
  end

  defp assert_rpc_requests(raw_log, from_block, to_block) do
    assert_receive {:rpc_request, %{"method" => "eth_getLogs", "params" => [filter]}}, 1_000
    assert filter["fromBlock"] == request_quantity(from_block)
    assert filter["toBlock"] == request_quantity(to_block)
    assert String.downcase(hd(filter["topics"])) == String.downcase(Deposit.event_signature())

    assert_receive {:rpc_request, [%{"method" => "eth_getBlockByNumber", "params" => [number, false]}]},
                   1_000

    assert number == raw_log["blockNumber"]
    refute_receive {:rpc_request, _}
  end

  defp quantity(value), do: EthereumJSONRPC.quantity_to_integer(value)

  defp request_quantity(value), do: value |> quantity() |> EthereumJSONRPC.integer_to_quantity()
end
