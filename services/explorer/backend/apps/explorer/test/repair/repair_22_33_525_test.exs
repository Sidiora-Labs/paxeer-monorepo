Code.require_file("../explorer/chain/transaction/history/historian_test.exs", __DIR__)

defmodule Explorer.Repair.Repair2233525Test do
  @moduledoc """
  Task 22.33.525 closes audit 22.30.154 finding historian_configuration_leak.
  The configuration cases invoke HistorianTest's actual setup and assert exact
  restoration after its real on_exit callback, including an absent environment.
  The recorded-chain case decodes committed EVM and native price-feed data,
  stores it through production schemas and PriceFeed, then compiles and reads
  persisted Historian statistics. The original suite also runs unchanged in
  this gate, including its transaction, gas, fee and price-feed assertions.

  Baseline: no earlier gate ledger or scoped repair was present. No unchanged
  baseline was run: the explicit implementation-first gate order forbids it.
  hf-build and hf-done retain the candidate revision and private evidence.
  No out-of-path wiring is required; spec edits are outside the allowed scope.
  """

  use Explorer.DataCase, async: false

  alias Explorer.Chain.Transaction
  alias Explorer.Chain.Transaction.History.{Historian, HistorianTest, TransactionStats}
  alias Explorer.Chain.PaxeerX.PriceFeed
  alias Explorer.PaxeerX.CosmosRest

  @fixtures Path.expand("../support/fixture/paxeer_x/price_feed", __DIR__)
  @evm_recording Path.expand("../../../ethereum_jsonrpc/test/support/fixture/geth/trace/struct_logger.json", __DIR__)

  setup do
    previous = Application.fetch_env(:explorer, Historian)
    on_exit(fn -> restore_config(previous) end)
    :ok
  end

  test "historian_configuration_leak preserves startup keys and restores the exact configured date" do
    assert_configuration_lifecycle(
      {:ok,
       [
         days_to_compile_at_init: 7,
         utc_today: ~D[2026-10-23],
         enabled: false,
         init_lag_milliseconds: 120_000,
         base_backoff: 250
       ]}
    )
  end

  test "historian_configuration_leak removes the temporary date when the original config has none" do
    assert_configuration_lifecycle({:ok, [enabled: false, init_lag_milliseconds: 60_000, days_to_compile_at_init: 3]})
  end

  test "historian_configuration_leak restores an explicitly empty configuration" do
    assert_configuration_lifecycle({:ok, []})
  end

  test "historian_configuration_leak restores the original absence" do
    assert_configuration_lifecycle(:error)
  end

  test "historian_configuration_leak keeps recorded transaction and price-feed statistics intact" do
    assert_configuration_lifecycle({:ok, [enabled: false, days_to_compile_at_init: 2]})

    %{"tx" => tx, "receipt" => receipt} = read_json(@evm_recording)
    tx_params = tx |> EthereumJSONRPC.Transaction.to_elixir() |> EthereumJSONRPC.Transaction.elixir_to_params()
    receipt_params = receipt |> EthereumJSONRPC.Receipt.to_elixir() |> EthereumJSONRPC.Receipt.elixir_to_params()
    params = Map.merge(tx_params, Map.take(receipt_params, [:gas_used, :cumulative_gas_used, :status]))

    block = insert(:block, hash: params.block_hash, number: params.block_number, timestamp: ~U[1970-01-03 12:00:00Z])

    for hash <- Enum.uniq([params.from_address_hash, params.to_address_hash]), not is_nil(hash) do
      insert(:address, hash: hash)
    end

    changeset =
      Transaction.changeset(
        %Transaction{},
        Map.merge(params, %{block_consensus: true, block_timestamp: block.timestamp})
      )

    assert changeset.valid?, inspect(changeset.errors)
    transaction = Repo.insert!(changeset)

    body = read_json(Path.join(@fixtures, "block_txs.json"))
    results = read_json(Path.join(@fixtures, "block_results.json"))
    raw = body["block"]["data"]["txs"]
    assert {:ok, submissions} = CosmosRest.decode_block_txs(%{"txs" => body["txs"], "raw" => raw})
    assert {:ok, [update]} = CosmosRest.decode_price_updates(submissions, results["txs_results"], length(raw))
    assert {:ok, 1} = PriceFeed.replace_block_updates(block, [update])

    assert {:ok, [today, day]} = Historian.compile_records(2)
    assert today == %{date: ~D[1970-01-04], number_of_transactions: 0, gas_used: 0, total_fee: 0}
    assert day.date == ~D[1970-01-03]
    assert day.number_of_transactions == 2
    assert Decimal.equal?(day.gas_used, transaction.gas_used)
    expected_fee = Decimal.mult(transaction.gas_price.value, transaction.gas_used)
    assert Decimal.equal?(day.total_fee, expected_fee)

    assert Historian.save_records([today, day]) == 2
    assert [%TransactionStats{} = stored] = TransactionStats.by_date_range(day.date, day.date)
    assert stored.number_of_transactions == 2
    assert Decimal.equal?(stored.gas_used, transaction.gas_used)
    assert Decimal.equal?(stored.total_fee, expected_fee)
  end

  defp assert_configuration_lifecycle(previous) do
    restore_config(previous)

    # ExUnit runs callbacks in reverse order: the actual setup's restoration
    # runs before this assertion, and the outer setup restores the suite value.
    on_exit(fn ->
      assert Application.fetch_env(:explorer, Historian) == previous

      date =
        case previous do
          {:ok, config} -> Keyword.get(config, :utc_today, Date.utc_today())
          :error -> Date.utc_today()
        end

      assert {:ok, [%{date: ^date, number_of_transactions: 0, gas_used: 0, total_fee: 0}]} =
               Historian.compile_records(1)
    end)

    assert :ok = HistorianTest.configure_historian(%{})

    config =
      case previous do
        {:ok, config} -> config
        :error -> []
      end

    assert Application.fetch_env(:explorer, Historian) == {:ok, Keyword.put(config, :utc_today, ~D[1970-01-04])}

    assert {:ok, [%{date: ~D[1970-01-04], number_of_transactions: 0, gas_used: 0, total_fee: 0}]} =
             Historian.compile_records(1)
  end

  defp restore_config({:ok, config}), do: Application.put_env(:explorer, Historian, config)
  defp restore_config(:error), do: Application.delete_env(:explorer, Historian)
  defp read_json(path), do: path |> File.read!() |> Jason.decode!()
end
