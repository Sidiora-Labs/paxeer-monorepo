defmodule Explorer.Market.Fetcher.CoinTest do
  use Explorer.DataCase, async: false

  import Mox

  alias Explorer.Market.MarketHistoryCache
  alias Explorer.Market.Token
  alias Explorer.Market.Fetcher.Coin
  alias Explorer.Market.Source.TestSource

  @moduletag :capture_log

  setup :verify_on_exit!

  setup do
    Supervisor.terminate_child(Explorer.Supervisor, {ConCache, MarketHistoryCache.cache_name()})
    Supervisor.restart_child(Explorer.Supervisor, {ConCache, MarketHistoryCache.cache_name()})

    source_configuration = Application.get_env(:explorer, Explorer.Market.Source)
    fetcher_configuration = Application.get_env(:explorer, Coin)

    Application.put_env(:explorer, Explorer.Market.Source,
      native_coin_source: TestSource,
      secondary_coin_source: TestSource
    )

    Application.put_env(
      :explorer,
      Coin,
      Keyword.merge(fetcher_configuration,
        table_name: :coin_fetcher_test_rates,
        enabled: true,
        enable_consolidation: false
      )
    )

    on_exit(fn ->
      Application.put_env(:explorer, Explorer.Market.Source, source_configuration)
      Application.put_env(:explorer, Coin, fetcher_configuration)
      Supervisor.terminate_child(Explorer.Supervisor, Explorer.Chain.Cache.Blocks.child_id())
      Supervisor.restart_child(Explorer.Supervisor, Explorer.Chain.Cache.Blocks.child_id())
    end)
  end

  test "init" do
    assert :ets.info(Coin.table_name()) == :undefined

    assert {:ok, %Coin{native_coin_source: TestSource, secondary_coin_source: TestSource}} == Coin.init([])
    assert_received {:update, false}
    assert_received {:update, true}
    table = :ets.info(Coin.table_name())
    refute table == :undefined
    assert table[:name] == Coin.table_name()
    assert table[:named_table]
    assert table[:read_concurrency]
    assert table[:type] == :set
    assert table[:write_concurrency]
  end

  test "handle_info with :update" do
    {:ok, state} = Coin.init([])

    expect(TestSource, :fetch_native_coin, fn -> {:ok, Token.null()} end)
    set_mox_global()

    assert {:noreply, fetching_state} = Coin.handle_info({:update, false}, state)
    assert_receive {ref, {{:ok, %Token{}}, false}} = reply
    assert fetching_state == %{state | tasks: %{ref => false}}
    assert {:noreply, ^state} = Coin.handle_info(reply, fetching_state)
  end

  describe "ticker fetch task" do
    setup do
      {:ok, state} = Coin.init([])
      {:ok, state: state}
    end

    test "with successful fetch", %{state: state} do
      expected_token = %Token{
        available_supply: Decimal.new("1000000.0"),
        total_supply: Decimal.new("1000000.0"),
        btc_value: Decimal.new("1.000"),
        last_updated: DateTime.utc_now(),
        market_cap: Decimal.new("1000000.0"),
        tvl: Decimal.new("2000000.0"),
        name: "test_name",
        symbol: "test_symbol",
        fiat_value: Decimal.new("1.0"),
        volume_24h: Decimal.new("1000.0"),
        image_url: nil
      }

      ref = make_ref()
      fetching_state = %{state | tasks: %{ref => false}}
      assert {:noreply, ^state} = Coin.handle_info({ref, {{:ok, expected_token}, false}}, fetching_state)

      assert [false: ^expected_token] = :ets.lookup(Coin.table_name(), false)
    end

    test "with failed fetch", %{state: state} do
      expect(TestSource, :fetch_native_coin, fn -> {:ok, Token.null()} end)
      set_mox_global()

      ref = make_ref()
      fetching_state = %{state | tasks: %{ref => false}}
      failed_state = %{state | failures: %{false => %{attempts: 1, reason: "some error"}}}

      assert {:noreply, ^failed_state} = Coin.handle_info({ref, {{:error, "some error"}, false}}, fetching_state)
      assert failed_state.tasks == %{}
      assert_received {:update, false}
      assert_received {:update, true}
      assert_receive {:update, false}, 1_000

      assert {:noreply, retrying_state} = Coin.handle_info({:update, false}, failed_state)
      assert_receive {retry_ref, {{:ok, %Token{}}, false}} = reply
      assert retrying_state == %{failed_state | tasks: %{retry_ref => false}}
      assert {:noreply, ^state} = Coin.handle_info(reply, retrying_state)
    end
  end
end
