defmodule BlockScoutWeb.Chain.MarketHistoryChartController do
  use BlockScoutWeb, :controller

  alias Explorer.{Chain, Market}

  def show(conn, _params) do
    if ajax?(conn) do
      exchange_rate = Market.get_coin_exchange_rate()

      recent_market_history = Market.fetch_recent_history()
      current_total_supply = available_supply(Chain.supply_for_days(), exchange_rate)

      price_history_data = update_closing_price(recent_market_history, exchange_rate)

      market_history_data = encode_market_history_data(price_history_data, current_total_supply)

      json(conn, %{
        history_data: market_history_data
      })
    else
      unprocessable_entity(conn)
    end
  end

  def update_closing_price([today | the_rest], %{fiat_value: fiat_value}) when not is_nil(fiat_value) do
    [%{today | closing_price: fiat_value} | the_rest]
  end

  def update_closing_price(market_history_data, _exchange_rate), do: market_history_data

  def available_supply(:ok, exchange_rate), do: exchange_rate.available_supply

  def available_supply({:ok, supply_for_days}, _exchange_rate) do
    supply_for_days
    |> Jason.encode()
    |> case do
      {:ok, _data} ->
        current_date =
          supply_for_days
          |> Map.keys()
          |> Enum.max(Date)

        Map.get(supply_for_days, current_date)

      _ ->
        nil
    end
  end

  def encode_market_history_data(market_history_data, current_total_supply) when is_binary(current_total_supply) do
    encode_market_history_data(market_history_data, Decimal.new(current_total_supply))
  end

  def encode_market_history_data(market_history_data, current_total_supply) do
    market_history_data
    |> Enum.map(fn day ->
      market_cap =
        cond do
          not is_nil(day.market_cap) -> day.market_cap
          is_nil(current_total_supply) or is_nil(day.closing_price) -> nil
          true -> Decimal.mult(current_total_supply, day.closing_price)
        end

      day
      |> Map.put(:market_cap, market_cap)
      |> Map.take([:closing_price, :market_cap, :tvl, :date])
    end)
    |> Jason.encode()
    |> case do
      {:ok, data} -> Jason.decode!(data)
      _ -> []
    end
  end
end
