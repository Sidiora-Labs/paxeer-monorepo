defmodule Explorer.Chain.PaxeerX.ReceiptKeysConfig do
  @moduledoc false

  @max_height 18_446_744_073_709_551_615

  def parse(entries) when is_list(entries), do: Enum.map(entries, &parse_entry/1)

  def parse(_entries), do: raise(ArgumentError, "receipt_keys must be a list")

  defp parse_entry(entry) do
    id = entry |> field!("keyId") |> bytes32!("keyId")
    public = entry |> field!("publicKey") |> bytes32!("publicKey")
    first = entry |> field!("firstHeight") |> height!("firstHeight")
    last = entry |> field!("lastHeight") |> height!("lastHeight")

    if id == <<0::256>>, do: raise(ArgumentError, "keyId must be non-zero")
    if first > last, do: raise(ArgumentError, "firstHeight must not exceed lastHeight")

    %{key_id: id, public_key: public, first_height: first, last_height: last}
  end

  defp field!(entry, name) when is_map(entry) do
    case Map.fetch(entry, name) do
      {:ok, value} -> value
      :error -> raise ArgumentError, "missing #{name}"
    end
  end

  defp field!(_entry, name), do: raise(ArgumentError, "missing #{name} in receipt key entry")

  defp bytes32!(text, name) when is_binary(text) do
    case Base.decode16(String.replace_prefix(text, "0x", ""), case: :mixed) do
      {:ok, <<_::256>> = bytes} -> bytes
      _ -> raise ArgumentError, "#{name} must encode 32 bytes as hexadecimal"
    end
  end

  defp bytes32!(_text, name), do: raise(ArgumentError, "#{name} must encode 32 bytes as hexadecimal")

  defp height!(value, _name) when is_integer(value) and value > 0 and value <= @max_height, do: value

  defp height!(_value, name), do: raise(ArgumentError, "#{name} must be a positive uint64 integer")
end
