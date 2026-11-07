defmodule Explorer.Config.PaxeerXReceiptKeysConfigTest do
  use ExUnit.Case, async: true

  alias Explorer.Chain.PaxeerX.ReceiptKeysConfig

  @public_key Base.decode16!("D75A980182B10AB7D54BFED3C964073A0EE172F3DAA62325AF021A68F707511A")
  @key_id <<1::256>>

  test "an id independent of the public key digest parses into the governed map" do
    refute @key_id == :crypto.hash(:sha256, @public_key)

    assert ReceiptKeysConfig.parse([entry()]) == [
             %{key_id: @key_id, public_key: @public_key, first_height: 1, last_height: 100}
           ]
  end

  test "zero keyId is refused with its field name" do
    assert_raise ArgumentError, ~r/keyId/, fn ->
      ReceiptKeysConfig.parse([Map.put(entry(), "keyId", hex(<<0::256>>))])
    end
  end

  test "a publicKey of 31 bytes is refused with its field name" do
    assert_raise ArgumentError, ~r/publicKey/, fn ->
      ReceiptKeysConfig.parse([Map.put(entry(), "publicKey", hex(binary_part(@public_key, 0, 31)))])
    end
  end

  test "firstHeight above lastHeight is refused with its field name" do
    assert_raise ArgumentError, ~r/firstHeight/, fn ->
      ReceiptKeysConfig.parse([Map.put(entry(), "firstHeight", 101)])
    end
  end

  test "malformed hexadecimal fields and wrong lengths identify the field" do
    for name <- ["keyId", "publicKey"], value <- [nil, 32, "zz", "0x1", hex(<<1::248>>), hex(<<1::264>>)] do
      assert_raise ArgumentError, ~r/#{name}/, fn ->
        ReceiptKeysConfig.parse([Map.put(entry(), name, value)])
      end
    end
  end

  test "missing fields identify the field" do
    for name <- ["keyId", "publicKey", "firstHeight", "lastHeight"] do
      assert_raise ArgumentError, ~r/#{name}/, fn ->
        ReceiptKeysConfig.parse([Map.delete(entry(), name)])
      end
    end
  end

  test "heights must be positive uint64 integers" do
    for name <- ["firstHeight", "lastHeight"], value <- [nil, 0, -1, 1.0, "1", 18_446_744_073_709_551_616] do
      assert_raise ArgumentError, ~r/#{name}/, fn ->
        ReceiptKeysConfig.parse([Map.put(entry(), name, value)])
      end
    end
  end

  test "inclusive height bounds and the uint64 maximum are accepted" do
    for height <- [1, 18_446_744_073_709_551_615] do
      key = Map.merge(entry(), %{"firstHeight" => height, "lastHeight" => height})

      assert [%{first_height: ^height, last_height: ^height}] = ReceiptKeysConfig.parse([key])
    end
  end

  test "multiple entries preserve order and accept mixed case unprefixed hexadecimal" do
    second = Map.put(entry(), "keyId", Base.encode16(<<2::256>>, case: :lower))
    second = Map.put(second, "publicKey", Base.encode16(@public_key))

    assert [%{key_id: @key_id}, %{key_id: <<2::256>>, public_key: @public_key}] =
             ReceiptKeysConfig.parse([entry(), second])

    assert ReceiptKeysConfig.parse([]) == []
  end

  test "malformed collections and entries raise ArgumentError" do
    assert_raise ArgumentError, ~r/receipt_keys/, fn -> ReceiptKeysConfig.parse(%{}) end
    assert_raise ArgumentError, ~r/keyId/, fn -> ReceiptKeysConfig.parse([nil]) end
  end

  defp entry do
    %{"keyId" => hex(@key_id), "publicKey" => hex(@public_key), "firstHeight" => 1, "lastHeight" => 100}
  end

  defp hex(bytes), do: "0x" <> Base.encode16(bytes, case: :lower)
end
