defmodule Explorer.Chain.Fetcher.AddressesBlacklist.Blockaid do
  @moduledoc """
  Fetcher for addresses blacklist from blockaid provider
  """
  alias Explorer.{Chain, HttpClient}
  alias Explorer.Chain.Fetcher.AddressesBlacklist

  @behaviour AddressesBlacklist

  require Logger

  defmodule FetchError do
    @moduledoc false
    defexception [:reason]

    @impl true
    def message(%{reason: reason}), do: "Blockaid blacklist refresh failed: #{inspect(reason)}"
  end

  @keys_to_blacklist ["OFAC", "Malicious"]
  @timeout 60_000

  @impl AddressesBlacklist
  def fetch_addresses_blacklist do
    case fetch_result() do
      {:ok, blacklist} ->
        Logger.info("Blockaid blacklist refresh status=fresh")
        blacklist

      {:error, reason} ->
        Logger.warning(
          "Blockaid blacklist refresh status=stale_or_unavailable; retaining last successful snapshot; " <>
            "retrying after #{retry_interval()}ms; reason=#{inspect(reason)}"
        )

        raise FetchError, reason: reason
    end
  end

  @doc """
  Fetches a validated snapshot, distinguishing provider failure from an empty blacklist.
  """
  @spec fetch_result() ::
          {:ok, MapSet.t()} | {:error, {:http_status, non_neg_integer()} | :transport | :invalid_json | :invalid_schema}
  def fetch_result do
    case HttpClient.get(AddressesBlacklist.url(), [], recv_timeout: @timeout, timeout: @timeout) do
      {:ok, %{status_code: 200, body: body}} ->
        body
        |> Jason.decode()
        |> parse_blacklist()

      {:ok, %{status_code: status}} ->
        {:error, {:http_status, status}}

      {:error, _reason} ->
        {:error, :transport}
    end
  end

  defp parse_blacklist({:ok, json}) when is_map(json) do
    if Enum.any?(@keys_to_blacklist, &Map.has_key?(json, &1)) do
      Enum.reduce_while(@keys_to_blacklist, {:ok, MapSet.new()}, fn key, {:ok, blacklist} ->
        case parse_addresses(Map.get(json, key, []), blacklist) do
          {:ok, _} = result -> {:cont, result}
          {:error, _} = error -> {:halt, error}
        end
      end)
    else
      {:error, :invalid_schema}
    end
  end

  defp parse_blacklist({:ok, _}), do: {:error, :invalid_schema}
  defp parse_blacklist({:error, _}), do: {:error, :invalid_json}

  defp parse_addresses(addresses, blacklist) when is_list(addresses) do
    Enum.reduce_while(addresses, {:ok, blacklist}, fn address, {:ok, acc} ->
      case is_binary(address) && Chain.string_to_address_hash_or_nil(address) do
        %Explorer.Chain.Hash{} = hash -> {:cont, {:ok, MapSet.put(acc, {hash, nil})}}
        _ -> {:halt, {:error, :invalid_schema}}
      end
    end)
  end

  defp parse_addresses(_, _), do: {:error, :invalid_schema}

  defp retry_interval do
    Application.fetch_env!(:explorer, AddressesBlacklist)[:retry_interval]
  end
end
