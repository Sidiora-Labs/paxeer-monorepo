defmodule Explorer.Market.Source.Chainflow do
  @moduledoc """
  Fetches coin and token market data from the Chainflow price feed.
  """

  alias Explorer.{Chain, Repo}
  alias Explorer.Chain.Hash.Address
  alias Explorer.Chain.Token, as: ChainToken
  alias Explorer.Market.{Source, Token}

  @behaviour Source

  @impl Source
  def native_coin_fetching_enabled?, do: present?(config(:coin_symbol))

  @impl Source
  def secondary_coin_fetching_enabled?, do: present?(config(:secondary_coin_symbol))

  @impl Source
  def tokens_fetching_enabled?, do: config(:tokens) not in [nil, []]

  @impl Source
  def native_coin_price_history_fetching_enabled?, do: native_coin_fetching_enabled?()

  @impl Source
  def secondary_coin_price_history_fetching_enabled?, do: secondary_coin_fetching_enabled?()

  @impl Source
  def market_cap_history_fetching_enabled?, do: native_coin_fetching_enabled?()

  @impl Source
  def tvl_history_fetching_enabled?, do: :ignore

  @impl Source
  def fetch_tvl_history(_previous_days), do: :ignore

  @impl Source
  def fetch_native_coin, do: fetch_coin(config(:coin_symbol), Chain.circulating_supply())

  @impl Source
  def fetch_secondary_coin do
    symbol = config(:secondary_coin_symbol)
    fetch_coin(symbol, supply_for_symbol(symbol))
  end

  @impl Source
  def fetch_tokens(state, batch_size) when is_integer(batch_size) and batch_size > 0 do
    {batch, remaining} = Enum.split(state || config(:tokens) || [], batch_size)

    Enum.reduce_while(batch, {:ok, []}, fn pair, {:ok, tokens} ->
      with [address, symbol] when is_binary(symbol) <- pair,
           {:ok, hash} <- Address.cast(address),
           %ChainToken{} = token <- Repo.get(ChainToken, hash),
           {:ok, quote} <- fetch_coin(symbol, token_supply(token)) do
        params = %{
          contract_address_hash: hash,
          type: token.type,
          fiat_value: quote.fiat_value,
          circulating_market_cap: quote.market_cap,
          volume_24h: quote.volume_24h
        }

        {:cont, {:ok, [params | tokens]}}
      else
        {:error, _} = error -> {:halt, error}
        _ -> {:halt, {:error, "Invalid or unindexed Chainflow token"}}
      end
    end)
    |> case do
      {:ok, tokens} -> {:ok, if(remaining == [], do: nil, else: remaining), remaining == [], Enum.reverse(tokens)}
      error -> error
    end
  end

  @impl Source
  def fetch_native_coin_price_history(previous_days), do: fetch_history(previous_days, false)

  @impl Source
  def fetch_secondary_coin_price_history(previous_days), do: fetch_history(previous_days, true)

  @impl Source
  def fetch_market_cap_history(previous_days) do
    with {:ok, history} <- fetch_history(previous_days, false) do
      {:ok, Enum.map(history, &Map.take(&1, [:date, :market_cap]))}
    end
  end

  defp fetch_coin(symbol, supply) do
    with {:ok, price} <- request(symbol, "price"),
         {:ok, stats} <- request(symbol, "stats"),
         true <-
           is_map(price) and is_map(stats) and not Map.has_key?(price, "error") and
             not Map.has_key?(stats, "error") do
      fiat_value = Source.to_decimal(price["price"])

      {:ok,
       %Token{
         Token.null()
         | symbol: symbol,
           fiat_value: fiat_value,
           available_supply: supply,
           market_cap: multiply(fiat_value, supply),
           volume_24h: Source.to_decimal(stats["volume_24h"]),
           last_updated: Source.maybe_get_date(price["timestamp"])
       }}
    else
      {:error, _} = error -> error
      _ -> {:error, "Invalid Chainflow quote response"}
    end
  end

  defp fetch_history(previous_days, secondary_coin?) do
    symbol = config(if(secondary_coin?, do: :secondary_coin_symbol, else: :coin_symbol))
    now = DateTime.to_unix(DateTime.utc_now())
    query = %{symbol: symbol, resolution: "1D", from: now - previous_days * 86_400, to: now}
    supply = if secondary_coin?, do: supply_for_symbol(symbol), else: Chain.circulating_supply()

    with {:ok, response} <- request(symbol, "history", query) do
      case response do
        %{"s" => "no_data"} ->
          {:ok, []}

        %{"s" => "ok", "t" => times, "o" => opens, "c" => closes}
        when is_list(times) and is_list(opens) and is_list(closes) and
               length(times) == length(opens) and length(times) == length(closes) ->
          history_records(times, opens, closes, secondary_coin?, supply)

        _ ->
          {:error, "Invalid Chainflow candle response"}
      end
    end
  end

  defp history_records(times, opens, closes, secondary_coin?, supply) do
    Enum.zip([times, opens, closes])
    |> Enum.reduce_while({:ok, []}, fn {time, open, close}, {:ok, records} ->
      with true <- is_integer(time) and not is_nil(open) and not is_nil(close),
           {:ok, datetime} <- DateTime.from_unix(time) do
        record = %{
          date: DateTime.to_date(datetime),
          opening_price: Source.to_decimal(open),
          closing_price: Source.to_decimal(close),
          secondary_coin: secondary_coin?,
          market_cap: multiply(Source.to_decimal(close), supply)
        }

        {:cont, {:ok, [record | records]}}
      else
        _ -> {:halt, {:error, "Invalid Chainflow candle"}}
      end
    end)
    |> case do
      {:ok, records} -> {:ok, Enum.sort_by(records, & &1.date, Date)}
      error -> error
    end
  end

  defp supply_for_symbol(symbol) do
    case Enum.find(config(:tokens) || [], fn [_address, feed_symbol] -> feed_symbol == symbol end) do
      [address, _] ->
        with {:ok, hash} <- Address.cast(address),
             %ChainToken{} = token <- Repo.get(ChainToken, hash) do
          token_supply(token)
        else
          _ -> nil
        end

      nil ->
        nil
    end
  end

  defp token_supply(%ChainToken{total_supply: supply, decimals: decimals})
       when not is_nil(supply) and not is_nil(decimals) do
    Decimal.div(supply, Decimal.new(Integer.pow(10, Decimal.to_integer(decimals))))
  end

  defp token_supply(_token), do: nil

  defp multiply(nil, _supply), do: nil
  defp multiply(_price, nil), do: nil
  defp multiply(price, supply), do: Decimal.mult(price, supply)

  defp request(symbol, route, query \\ %{}) do
    if present?(symbol) do
      market =
        if symbol == config(:coin_symbol) and present?(config(:market)),
          do: config(:market),
          else: String.downcase(symbol)

      url =
        String.trim_trailing(config(:base_url), "/") <>
          "/api/" <> URI.encode(market, &URI.char_unreserved?/1) <> "/" <> route

      Source.http_request(if(query == %{}, do: url, else: url <> "?" <> URI.encode_query(query)), [])
    else
      {:error, "Chainflow symbol not specified"}
    end
  end

  defp present?(value), do: is_binary(value) and value != ""
  defp config(key), do: Application.get_env(:explorer, __MODULE__, [])[key]
end
