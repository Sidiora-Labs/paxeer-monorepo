Code.require_file("../indexer/migrator/recovery_weth_token_transfers_test.exs", __DIR__)

defmodule Indexer.Repair2233560Test do
  @moduledoc """
  Repair 22.33.560 closes finding 22.30.158/018 through the recorded receipt decoding
  and import case below, including exact restoration of both application configurations.
  The recovery cases exercise the real migrator and its persisted completion marker.
  The supplied attempt-4 baseline reports three tests with one failure: the original
  recovery case exceeded its 1000ms normal-exit bound; the other two cases passed.
  No unchanged gate is rerun. No wiring requests are needed. The qualification runner
  records the verified revision and private gate evidence in /out/run/gates.jsonl.
  """

  use Explorer.DataCase, async: false

  alias EthereumJSONRPC.{Logs, Receipts}
  alias Explorer.Chain
  alias Explorer.Chain.TokenTransfer
  alias Explorer.Migrator.MigrationStatus
  alias Indexer.Migrator.{RecoveryWETHTokenTransfers, RecoveryWETHTokenTransfersTest}
  alias Indexer.Transform.TokenTransfers

  @fixture Path.expand("../../../ethereum_jsonrpc/test/support/fixture/geth/trace/struct_logger.json", __DIR__)
  @burn "0x0000000000000000000000000000000000000000"

  setup do
    original_migrator = Application.fetch_env!(:indexer, RecoveryWETHTokenTransfers)
    original_transfers = Application.fetch_env!(:explorer, TokenTransfer)

    migrator = Keyword.merge(original_migrator, batch_size: 7, concurrency: 3, blocks_batch_size: 11)

    transfers =
      Keyword.merge(original_transfers,
        whitelisted_weth_contracts: [@burn],
        weth_token_transfers_filtering_enabled: true
      )

    Application.put_env(:indexer, RecoveryWETHTokenTransfers, migrator)
    Application.put_env(:explorer, TokenTransfer, transfers)

    # Registered first so the audited setup's on_exit must restore both values before these assertions.
    on_exit(fn ->
      try do
        assert Application.fetch_env!(:indexer, RecoveryWETHTokenTransfers) == migrator
        assert Application.fetch_env!(:explorer, TokenTransfer) == transfers
        assert TokenTransfer.whitelisted_weth_contract?(@burn)
        refute TokenTransfer.whitelisted_weth_contract?("0x175940b39014cd3a9c87cd6b1d7616a097db958e")
      after
        Application.put_env(:indexer, RecoveryWETHTokenTransfers, original_migrator)
        Application.put_env(:explorer, TokenTransfer, original_transfers)
      end
    end)

    :ok = RecoveryWETHTokenTransfersTest.configure_recovery(%{})
    %{migrator: migrator, transfers: transfers}
  end

  test "018: recovery preserves token-transfer settings through recorded receipt decoding and import", context do
    assert Application.fetch_env!(:explorer, TokenTransfer) ==
             Keyword.merge(context.transfers, weth_token_transfers_filtering_enabled: false)

    assert Application.fetch_env!(:indexer, RecoveryWETHTokenTransfers) ==
             Keyword.merge(context.migrator, batch_size: 1, concurrency: 2, blocks_batch_size: 1, timeout: 0)

    %{"receipt" => receipt} = @fixture |> File.read!() |> Jason.decode!()

    logs = receipt |> List.wrap() |> Receipts.to_elixir() |> Receipts.elixir_to_logs() |> Logs.elixir_to_params()
    withdrawal = Enum.find(logs, &(&1.first_topic == TokenTransfer.weth_withdrawal_signature()))
    assert withdrawal.address_hash == "0x175940b39014cd3a9c87cd6b1d7616a097db958e"
    assert TokenTransfer.whitelisted_weth_contract?(withdrawal.address_hash)

    insert(:token, contract_address: insert(:address, hash: withdrawal.address_hash), type: "ERC-20")
    sender = insert(:address, hash: "0x87132c7dbd0d6d6a73c9df24382f546c2e6fa21a")
    insert(:address, hash: @burn)
    block = insert(:block, hash: withdrawal.block_hash, number: withdrawal.block_number)
    insert(:transaction, hash: withdrawal.transaction_hash) |> with_block(block)

    assert %{token_transfers: [transfer]} = TokenTransfers.parse([withdrawal], true)
    assert transfer.from_address_hash == to_string(sender.hash)
    assert transfer.to_address_hash == @burn
    assert transfer.token_contract_address_hash == withdrawal.address_hash
    assert transfer.transaction_hash == withdrawal.transaction_hash
    assert transfer.log_index == 59
    assert transfer.block_number == 8_196_714
    assert Decimal.equal?(transfer.amount, Decimal.new(341_793_296_793_755))

    assert {:ok, %{token_transfers: [_]}} = Chain.import(%{token_transfers: %{params: [transfer]}})
    recovered = Repo.one!(TokenTransfer)
    assert to_string(recovered.transaction_hash) == withdrawal.transaction_hash
    assert to_string(recovered.block_hash) == withdrawal.block_hash
    assert recovered.log_index == transfer.log_index
    assert recovered.from_address_hash == sender.hash
    assert to_string(recovered.to_address_hash) == @burn
    assert to_string(recovered.token_contract_address_hash) == withdrawal.address_hash
    assert Decimal.equal?(recovered.amount, transfer.amount)
  end

  test "recovery exits after committing its last batch before the next configured interval" do
    config = Application.fetch_env!(:indexer, RecoveryWETHTokenTransfers)
    Application.put_env(:indexer, RecoveryWETHTokenTransfers, Keyword.put(config, :timeout, 2000))
    interval = Application.fetch_env!(:indexer, RecoveryWETHTokenTransfers)[:timeout]

    %{contract_address: token_address} = insert(:token, type: "ERC-20")
    sender = insert(:address)
    block = insert(:block)
    transaction = insert(:transaction) |> with_block(block)

    logs =
      for _ <- 1..2 do
        insert(:withdrawal_log,
          from_address: sender,
          token_contract_address: token_address,
          amount: 1000,
          transaction: transaction,
          block: block
        )
      end

    {:ok, migrator} = RecoveryWETHTokenTransfers.start_link([])
    monitor = Process.monitor(migrator)
    assert_receive {:DOWN, ^monitor, :process, ^migrator, :normal}, div(interval, 2)

    assert %{status: "completed", meta: nil} = MigrationStatus.fetch(RecoveryWETHTokenTransfers.migration_name())
    transfers = Repo.all(TokenTransfer |> order_by([tt], asc: tt.log_index))
    assert length(transfers) == 2

    Enum.zip(transfers, logs)
    |> Enum.each(fn {transfer, log} ->
      RecoveryWETHTokenTransfersTest.check_withdrawal_token_transfer(transfer, log)
    end)
  end
end
