defmodule Explorer.Repair.Repair2233379Test do
  @moduledoc """
  Repair record for audit 22.30.140:
  * chain-test-supply-state: the original supply examples restore an absent,
    configured, or explicitly nil setting after their real ExUnit lifecycle.
  * chain-test-disabled-ordering: the restored original example and recorded
    same-timestamp blocks select the highest block's zero balance in both orders.
  No out-of-path wiring change is required by the inspected history query.
  No pre-change gate ran: the ordered task instructions require implementation
  before hf-build/hf-done and prohibit a separate baseline verification run.
  The previous attempt's two recorded import-order cases failed on timestamp
  precision: JSON-RPC seconds versus the block schema's microseconds. Expected
  timestamps use that schema's cast while retaining exact equality assertions.
  The bounded gate ledger records the verified revision and evidence location.
  """
  use Explorer.DataCase

  alias EthereumJSONRPC.Block, as: JSONRPCBlock
  alias Explorer.Chain
  alias Explorer.Chain.{Block, Wei}
  alias Explorer.Chain.Address.{CoinBalance, CoinBalanceDaily}
  alias Explorer.Chain.Import.Runner.Address.{CoinBalances, CoinBalancesDaily}
  alias Explorer.Chain.Supply.ProofOfAuthority

  @chain_test_file Path.expand("../explorer/chain_test.exs", __DIR__)
  @external_resource @chain_test_file
  # Execute the repaired examples themselves without registering the unrelated suite.
  {_, examples} =
    @chain_test_file
    |> File.read!()
    |> Code.string_to_quoted!()
    |> Macro.prewalk(%{}, fn
      {:test, _, [name, [do: body]]} = node, acc when is_binary(name) ->
        {node, Map.put(acc, name, body)}

      node, acc ->
        {node, acc}
    end)

  for state <- [:absent, :configured, nil], name <- ["total_supply/0", "circulating_supply/0"] do
    @tag initial_supply: state
    test "chain-test-supply-state: #{name} restores #{state}" do
      unquote(Map.fetch!(examples, name))
    end
  end

  test "chain-test-disabled-ordering: the restored original regression executes" do
    unquote(Map.fetch!(examples, "uses last block value if there a couple of change in the same day"))
  end

  setup tags do
    if Map.has_key?(tags, :initial_supply) do
      original = Application.fetch_env(:explorer, :supply)

      case tags.initial_supply do
        :absent -> Application.delete_env(:explorer, :supply)
        :configured -> Application.put_env(:explorer, :supply, Explorer.Chain.Supply.ExchangeRate)
        nil -> Application.put_env(:explorer, :supply, nil)
      end

      expected = Application.fetch_env(:explorer, :supply)

      # Registered before the original example's on_exit, so this observes its cleanup.
      on_exit(fn ->
        observed = Application.fetch_env(:explorer, :supply)
        restore_supply(original)
        assert observed == expected
      end)
    end

    :ok
  end

  for order <- [:ascending, :descending] do
    test "chain-test-disabled-ordering: recorded blocks survive #{order} import order" do
      blocks = recorded_blocks()
      [earlier, latest] = Enum.sort_by(blocks, & &1.number)
      assert earlier.number + 1 == latest.number
      assert earlier.timestamp == latest.timestamp
      assert earlier.hash == latest.parent_hash
      insert(:address, hash: earlier.miner_hash)

      ordered = Enum.sort_by(blocks, & &1.number, unquote(if order == :ascending, do: :asc, else: :desc))

      for params <- ordered do
        %Block{}
        |> Block.changeset(Map.put(params, :consensus, true))
        |> Repo.insert!()
      end

      original_supply = Application.fetch_env(:explorer, :supply)
      on_exit(fn -> restore_supply(original_supply) end)
      Application.put_env(:explorer, :supply, ProofOfAuthority)
      assert Chain.total_supply() == ProofOfAuthority.initial_supply() + latest.number
      assert Chain.circulating_supply() == ProofOfAuthority.circulating()

      address = insert(:address)
      day = DateTime.to_date(latest.timestamp)
      config_key = BlockScoutWeb.Chain.Address.CoinBalance
      config = Application.fetch_env!(:block_scout_web, config_key)
      on_exit(fn -> Application.put_env(:block_scout_web, config_key, config) end)

      Application.put_env(
        :block_scout_web,
        config_key,
        Keyword.put(config, :coin_balance_history_days, Date.diff(Date.utc_today(), day) + 1)
      )

      changes =
        for block <- ordered do
          changeset =
            CoinBalance.changeset(%CoinBalance{}, %{
              address_hash: address.hash,
              block_number: block.number,
              value: if(block.number == latest.number, do: 0, else: 1),
              value_fetched_at: block.timestamp
            })

          assert changeset.valid?
          changeset.changes
        end

      daily = CoinBalanceDaily.changeset(%CoinBalanceDaily{}, %{address_hash: address.hash, day: day, value: 1})
      assert daily.valid?
      imported_at = DateTime.utc_now()
      options = %{timestamps: %{inserted_at: imported_at, updated_at: imported_at}}

      assert {:ok, %{address_coin_balances: imported, address_coin_balances_daily: [_]}} =
               Ecto.Multi.new()
               |> CoinBalances.run(changes, options)
               |> CoinBalancesDaily.run([daily.changes], options)
               |> Repo.transaction()

      assert Enum.sort(Enum.map(imported, & &1.block_number)) == [earlier.number, latest.number]

      assert %{timestamp: timestamp, value: %Wei{value: value}} =
               address.hash |> CoinBalance.last_coin_balance_timestamp() |> Repo.one!()

      assert {:ok, expected_timestamp} = Ecto.Type.cast(Block.__schema__(:type, :timestamp), latest.timestamp)
      assert timestamp == expected_timestamp
      assert Decimal.equal?(value, 0)

      assert Chain.address_to_balances_by_day(address.hash) ==
               Enum.map(Enum.uniq([Date.to_string(day), Date.to_string(Date.utc_today())]), fn date ->
                 %{date: date, value: Decimal.new(0)}
               end)
    end
  end

  defp recorded_blocks do
    Path.expand("../../../ethereum_jsonrpc/test/support/fixture/paxeer_x/consecutive_blocks.json", __DIR__)
    |> File.read!()
    |> Jason.decode!()
    |> Map.values()
    |> Enum.map(fn block -> block |> JSONRPCBlock.to_elixir() |> JSONRPCBlock.elixir_to_params() end)
  end

  defp restore_supply(original) do
    case original do
      {:ok, supply} -> Application.put_env(:explorer, :supply, supply)
      :error -> Application.delete_env(:explorer, :supply)
    end
  end
end
