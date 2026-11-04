defmodule Indexer.Transform.MintTransfers do
  @moduledoc """
  Helper functions to parse addresses from mint transfers.

  When a network receives a mint coin, we can identify it using the `bridge_hash` in the first_topic.
  Then we need to fetch the `from` and `to` address since there is no transaction or internal
  transaction for it. Otherwise, those address may not be indexed.
  """

  require Logger

  @bridge_hash "0x3c798bbcf33115b42c728b8504cff11dd58736e9fa789f1cda2738db7d696b2a"
  @supported_emitters ["0x867305d19606aadba405ce534e303d0e225f9556"]

  @doc """
  Parses logs to find mint transfers.

  ## Examples

        iex> Indexer.Transform.MintTransfers.parse([
        ...>       %{
        ...>         address_hash: "0x867305d19606aadba405ce534e303d0e225f9556",
        ...>         block_number: 137_194,
        ...>         data: "0x0000000000000000000000000000000000000000000000001bc16d674ec80000",
        ...>         first_topic: "0x3c798bbcf33115b42c728b8504cff11dd58736e9fa789f1cda2738db7d696b2a",
        ...>         fourth_topic: nil,
        ...>         index: 1,
        ...>         second_topic: "0x0000000000000000000000009a4a90e2732f3fa4087b0bb4bf85c76d14833df1",
        ...>         third_topic: "0x0000000000000000000000007301cfa0e1756b71869e93d4e4dca5c7d0eb0aa6",
        ...>         transaction_hash: "0x1d5066d30ff3404a9306733136103ac2b0b989951c38df637f464f3667f8d4ee"
        ...>        }
        ...>     ])
        %{
          mint_transfers: [
            %{
              block_number: 137194,
              from_address_hash: "0x7301cfa0e1756b71869e93d4e4dca5c7d0eb0aa6",
              to_address_hash: "0x9a4a90e2732f3fa4087b0bb4bf85c76d14833df1"
            }
          ]
        }

  """
  def parse(logs) do
    {addresses, rejected} =
      Enum.reduce(logs, {[], %{}}, fn
        %{first_topic: @bridge_hash} = log, {addresses, rejected} ->
          case parse_params(log) do
            {:ok, address} -> {[address | addresses], rejected}
            {:error, reason} -> {addresses, Map.update(rejected, reason, 1, &(&1 + 1))}
          end

        _, acc ->
          acc
      end)

    if map_size(rejected) > 0 do
      Logger.warning("Ignoring unsupported or malformed mint logs: #{inspect(rejected)}")
    end

    %{mint_transfers: Enum.reverse(addresses)}
  end

  defp parse_params(%{address_hash: address_hash} = log) when is_binary(address_hash) do
    if String.downcase(address_hash) in @supported_emitters do
      parse_supported_params(log)
    else
      {:error, :unsupported_emitter}
    end
  end

  defp parse_params(_), do: {:error, :unsupported_emitter}

  defp parse_supported_params(%{
         second_topic: second_topic,
         third_topic: third_topic,
         fourth_topic: nil,
         data: data,
         block_number: block_number
       })
       when is_integer(block_number) and block_number >= 0 do
    with {:ok, to_address_hash} <- decode_address(second_topic),
         {:ok, from_address_hash} <- decode_address(third_topic),
         {:ok, _amount} <- decode_word(data) do
      {:ok,
       %{
         to_address_hash: to_address_hash,
         from_address_hash: from_address_hash,
         block_number: block_number
       }}
    else
      _ -> {:error, :invalid_shape}
    end
  end

  defp parse_supported_params(_), do: {:error, :invalid_shape}

  defp decode_address(topic) do
    case decode_word(topic) do
      {:ok, <<0::96, address::binary-size(20)>>} -> {:ok, "0x" <> Base.encode16(address, case: :lower)}
      _ -> :error
    end
  end

  defp decode_word("0x" <> word) when byte_size(word) == 64, do: Base.decode16(word, case: :mixed)
  defp decode_word(_), do: :error
end
