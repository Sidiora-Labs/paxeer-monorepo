defmodule Explorer.Chain.PaxeerX.NativeSettlement do
  @moduledoc false
  import Bitwise

  @evm "/paxprotocol.paxchain.evm.MsgEVMTransaction"
  @eth "/paxprotocol.paxchain.eth."
  @max_bytes 1_048_576

  def transaction(bytes) when is_binary(bytes) and byte_size(bytes) <= @max_bytes do
    tx = fields(bytes)
    body = fields(one(tx, 1, <<>>))
    [message] = many(body, 1)
    message = fields(message)
    @evm = one(message, 1, <<>>)
    evm = fields(one(message, 2, <<>>))
    any = fields(one(evm, 1, <<>>))
    kind = one(any, 1, <<>>)
    values = fields(one(any, 2, <<>>))
    raw = ethereum(kind, values)
    {:ok, raw, ExKeccak.hash_256(raw)}
  rescue
    _ in [ArgumentError, MatchError, FunctionClauseError, CaseClauseError] -> :error
  end

  def transaction(_), do: :error

  def result_bytes(result) when is_map(result) do
    code = integer(Map.get(result, "code", 0))
    true = code >= 0 and code <= 0xFFFFFFFF
    data = Base.decode64!(Map.get(result, "data") || "")
    true = byte_size(data) <= @max_bytes
    wanted = integer(Map.get(result, "gas_wanted", 0))
    used = integer(Map.get(result, "gas_used", 0))
    true = wanted in 0..0x7FFFFFFFFFFFFFFF and used in 0..0x7FFFFFFFFFFFFFFF
    {:ok, scalar(1, code) <> blob(2, data) <> scalar(5, wanted) <> scalar(6, used)}
  rescue
    _ in [ArgumentError, MatchError, FunctionClauseError, CaseClauseError] -> :error
  end

  def result_bytes(_), do: :error

  def result_matches?(bytes, receipt, view) do
    result = fields(bytes)
    true = Enum.all?(result, fn {n, _} -> n in [1, 2, 5, 6] end)
    0 = one(result, 1, 0)
    gas_used = one(result, 6, 0)
    gas_wanted = one(result, 5, 0)
    true = is_integer(gas_wanted) and gas_wanted >= 0
    true = gas_used == quantity(view["gasUsed"])
    [{1, message}] = fields(one(result, 2, <<>>))
    [{1, @evm}, {2, response}] = fields(message)
    response = fields(response)
    true = Enum.all?(response, fn {n, _} -> n in 1..5 end)
    true = one(response, 1, 0) == quantity(view["gasUsed"])
    true = one(response, 2, <<>>) == view["vmError"]
    true = one(response, 2, <<>>) == "" == (receipt.result_status == 0)
    true = one(response, 3, <<>>) == hex(view["returnData"])
    true = hex(one(response, 4, <<>>)) == receipt.activity_id
    logs = Enum.map(many(response, 5), &decode_log/1)
    logs == Enum.map(view["logs"], &Map.take(&1, ["address", "topics", "data", "logIndex"]))
  rescue
    _ in [ArgumentError, MatchError, FunctionClauseError, CaseClauseError] -> false
  end

  defp decode_log(bytes) do
    f = fields(bytes)
    true = Enum.all?(f, fn {n, _} -> n in 1..5 end)
    synthetic = one(f, 5, 0)
    true = synthetic in [0, 1]

    %{
      "address" => normalized_hex(one(f, 1, <<>>)),
      "topics" => Enum.map(many(f, 2), &normalized_hex/1),
      "data" => encode_hex(one(f, 3, <<>>)),
      "logIndex" => ("0x" <> Integer.to_string(one(f, 4, 0), 16)) |> String.downcase()
    }
  end

  defp ethereum(@eth <> "LegacyTx", f) do
    rlp([
      one(f, 1, 0),
      decimal(f, 2),
      one(f, 3, 0),
      address(f, 4),
      decimal(f, 5),
      one(f, 6, <<>>),
      unsigned(one(f, 7, <<>>)),
      unsigned(one(f, 8, <<>>)),
      unsigned(one(f, 9, <<>>))
    ])
  end

  defp ethereum(@eth <> "AccessListTx", f) do
    <<1>> <>
      rlp([
        decimal(f, 1),
        one(f, 2, 0),
        decimal(f, 3),
        one(f, 4, 0),
        address(f, 5),
        decimal(f, 6),
        one(f, 7, <<>>),
        accesses(f, 8),
        unsigned(one(f, 9, <<>>)),
        unsigned(one(f, 10, <<>>)),
        unsigned(one(f, 11, <<>>))
      ])
  end

  defp ethereum(@eth <> "DynamicFeeTx", f) do
    <<2>> <>
      rlp([
        decimal(f, 1),
        one(f, 2, 0),
        decimal(f, 3),
        decimal(f, 4),
        one(f, 5, 0),
        address(f, 6),
        decimal(f, 7),
        one(f, 8, <<>>),
        accesses(f, 9),
        unsigned(one(f, 10, <<>>)),
        unsigned(one(f, 11, <<>>)),
        unsigned(one(f, 12, <<>>))
      ])
  end

  defp accesses(f, number) do
    Enum.map(many(f, number), fn bytes ->
      tuple = fields(bytes)
      <<_::160>> = address = hex(one(tuple, 1, <<>>))

      keys =
        Enum.map(many(tuple, 2), fn text ->
          <<_::256>> = key = hex(text)
          key
        end)

      [address, keys]
    end)
  end

  defp decimal(f, n) do
    value = integer(one(f, n, "0"))
    true = value >= 0 and value < 1 <<< 256
    value
  end

  defp address(f, n) do
    case one(f, n, "") do
      "" ->
        <<>>

      value ->
        <<_::160>> = bytes = hex(value)
        bytes
    end
  end

  defp unsigned(<<>>), do: 0
  defp unsigned(bytes) when is_binary(bytes), do: :binary.decode_unsigned(bytes)
  defp integer(value) when is_integer(value), do: value

  defp integer(value) when is_binary(value) do
    {number, ""} = Integer.parse(value)
    number
  end

  defp rlp(value) when is_integer(value) and value >= 0,
    do: rlp(if(value == 0, do: <<>>, else: :binary.encode_unsigned(value)))

  defp rlp(<<byte>>) when byte < 128, do: <<byte>>
  defp rlp(bytes) when is_binary(bytes), do: length_prefix(byte_size(bytes), 128) <> bytes

  defp rlp(list) when is_list(list) do
    bytes = list |> Enum.map(&rlp/1) |> IO.iodata_to_binary()
    length_prefix(byte_size(bytes), 192) <> bytes
  end

  defp length_prefix(length, base) when length <= 55, do: <<base + length>>

  defp length_prefix(length, base) do
    bytes = :binary.encode_unsigned(length)
    <<base + 55 + byte_size(bytes)>> <> bytes
  end

  defp fields(bytes), do: fields(bytes, [], 0)
  defp fields(<<>>, acc, _count), do: Enum.reverse(acc)

  defp fields(bytes, acc, count) when count < 100_000 do
    {key, rest} = read_varint(bytes, 0, 0)
    true = key >>> 3 > 0

    {value, rest} =
      case key &&& 7 do
        0 ->
          read_varint(rest, 0, 0)

        2 ->
          {length, rest} = read_varint(rest, 0, 0)
          <<value::binary-size(length), rest::binary>> = rest
          {value, rest}
      end

    fields(rest, [{key >>> 3, value} | acc], count + 1)
  end

  defp read_varint(<<byte, rest::binary>>, shift, acc) when shift < 64 do
    true = shift < 63 or byte <= 1
    value = acc ||| (byte &&& 127) <<< shift
    if byte < 128, do: {value, rest}, else: read_varint(rest, shift + 7, value)
  end

  defp one(fields, number, default) do
    case many(fields, number) do
      [] -> default
      [value] -> value
      _ -> raise ArgumentError, "duplicate protobuf field"
    end
  end

  defp many(fields, number), do: for({^number, value} <- fields, do: value)
  defp scalar(_number, 0), do: <<>>
  defp scalar(number, value), do: varint(number <<< 3) <> varint(value)
  defp blob(_number, <<>>), do: <<>>
  defp blob(number, bytes), do: varint(number <<< 3 ||| 2) <> varint(byte_size(bytes)) <> bytes
  defp varint(value) when value < 128, do: <<value>>
  defp varint(value), do: <<(value &&& 127) ||| 128>> <> varint(value >>> 7)
  defp quantity("0x" <> digits), do: String.to_integer(digits, 16)
  defp hex("0x" <> digits), do: Base.decode16!(digits, case: :mixed)
  defp hex(digits), do: Base.decode16!(digits, case: :mixed)
  defp encode_hex(bytes), do: "0x" <> Base.encode16(bytes, case: :lower)
  defp normalized_hex(text), do: text |> hex() |> encode_hex()
end
