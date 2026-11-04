defmodule Explorer.Chain.PaxeerX.SettlementProof do
  @moduledoc false
  import Bitwise

  def transaction(data, proof, root), do: verify(:crypto.hash(:sha256, data), proof, root)

  def verify(data, %{"total" => total, "index" => index, "leaf_hash" => leaf, "aunts" => aunts}, root)
      when is_binary(data) and is_list(aunts) and length(aunts) <= 17 do
    with {:ok, total} <- counter(total),
         {:ok, index} <- counter(index),
         true <- total > 0 and total <= 100_001 and index < total,
         {:ok, leaf} <- decode(leaf),
         true <- leaf == :crypto.hash(:sha256, <<0>> <> data),
         {:ok, aunts} <- decode_all(aunts),
         {:ok, ^root} <- root(leaf, total, index, Enum.reverse(aunts)) do
      true
    else
      _ -> false
    end
  end

  def verify(_, _, _), do: false

  def make(items, index) when is_list(items) and items != [] and index >= 0 and index < length(items) do
    leaves = Enum.map(items, &:crypto.hash(:sha256, <<0>> <> &1))
    {hash, aunts} = branch(leaves, index)

    %{
      "root_hash" => Base.encode16(hash),
      "proof" => %{
        "total" => Integer.to_string(length(items)),
        "index" => Integer.to_string(index),
        "leaf_hash" => leaves |> Enum.at(index) |> Base.encode64(),
        "aunts" => Enum.map(aunts, &Base.encode64/1)
      }
    }
  end

  defp branch([leaf], 0), do: {leaf, []}

  defp branch(leaves, index) do
    split = split_point(length(leaves), 1)
    {left, right} = Enum.split(leaves, split)

    if index < split do
      {hash, aunts} = branch(left, index)
      sibling = tree(right)
      {inner(hash, sibling), aunts ++ [sibling]}
    else
      {hash, aunts} = branch(right, index - split)
      sibling = tree(left)
      {inner(sibling, hash), aunts ++ [sibling]}
    end
  end

  defp tree([leaf]), do: leaf

  defp tree(leaves) do
    {left, right} = Enum.split(leaves, split_point(length(leaves), 1))
    inner(tree(left), tree(right))
  end

  defp root(leaf, 1, 0, []), do: {:ok, leaf}

  defp root(leaf, total, index, [aunt | rest]) when total > 1 do
    split = split_point(total, 1)

    if index < split do
      with {:ok, left} <- root(leaf, split, index, rest), do: {:ok, inner(left, aunt)}
    else
      with {:ok, right} <- root(leaf, total - split, index - split, rest), do: {:ok, inner(aunt, right)}
    end
  end

  defp root(_, _, _, _), do: :error
  defp inner(left, right), do: :crypto.hash(:sha256, <<1>> <> left <> right)
  defp split_point(total, split) when split <<< 1 < total, do: split_point(total, split <<< 1)
  defp split_point(_, split), do: split
  defp counter(n) when is_integer(n) and n >= 0, do: {:ok, n}

  defp counter(n) when is_binary(n) do
    case Integer.parse(n) do
      {value, ""} when value >= 0 -> {:ok, value}
      _ -> :error
    end
  end

  defp counter(_), do: :error

  defp decode(text) when is_binary(text) do
    case Base.decode64(text) do
      {:ok, <<_::256>> = bytes} -> {:ok, bytes}
      _ -> :error
    end
  end

  defp decode(_), do: :error

  defp decode_all(list) do
    Enum.reduce_while(list, {:ok, []}, fn text, {:ok, acc} ->
      case decode(text) do
        {:ok, bytes} -> {:cont, {:ok, acc ++ [bytes]}}
        _ -> {:halt, :error}
      end
    end)
  end
end
