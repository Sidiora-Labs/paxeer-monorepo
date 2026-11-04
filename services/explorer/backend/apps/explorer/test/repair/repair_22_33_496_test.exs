defmodule Explorer.Repair.Repair2233496Test do
  use ExUnit.Case, async: true

  import Bitwise

  alias Ecto.Changeset
  alias EthereumJSONRPC.Block, as: RPCBlock
  alias Explorer.Chain.{Block, Hash}

  @recording Path.expand(
               "../../../ethereum_jsonrpc/test/support/fixture/paxeer_x/consecutive_blocks.json",
               __DIR__
             )
  @hash_fields [
    {:hash, Hash.Full, 32},
    {:parent_hash, Hash.Full, 32},
    {:miner_hash, Hash.Address, 20},
    {:nonce, Hash.Nonce, 8}
  ]

  setup_all do
    blocks = @recording |> File.read!() |> Jason.decode!()

    params =
      blocks
      |> Map.fetch!("0x1714efd")
      |> RPCBlock.to_elixir()
      |> RPCBlock.elixir_to_params()
      |> Map.put(:consensus, true)

    %{params: params}
  end

  test "hash-integer-truncation: recorded RPC block reaches the real schema and hash codecs", %{params: params} do
    changeset = Block.changeset(%Block{}, params)
    assert changeset.valid?, inspect(changeset.errors)
    block = Changeset.apply_changes(changeset)
    assert block.number == 24_203_005

    for {field, type, byte_count} <- @hash_fields do
      hash = Map.fetch!(block, field)
      assert Hash.to_string(hash) == Map.fetch!(params, field)
      assert_round_trip(type, hash, Hash.to_integer(hash), byte_count)

      integer_changeset = Block.changeset(%Block{}, Map.put(params, field, Hash.to_integer(hash)))
      assert integer_changeset.valid?, inspect(integer_changeset.errors)
      assert Changeset.get_field(integer_changeset, field) == hash
    end
  end

  test "hash-integer-truncation: zero and the encoded maximum survive schema cast dump load and JSON", %{params: params} do
    for {field, type, byte_count} <- @hash_fields do
      maximum = (1 <<< (8 * byte_count)) - 1

      for integer <- [0, 1, maximum - 1, maximum] do
        changeset = Block.changeset(%Block{}, Map.put(params, field, integer))
        assert changeset.valid?, inspect({field, integer, changeset.errors})
        hash = changeset |> Changeset.apply_changes() |> Map.fetch!(field)
        assert_round_trip(type, hash, integer, byte_count)
      end
    end
  end

  test "hash-integer-truncation: both ends of the former truncation range and their neighbors are rejected", %{
    params: params
  } do
    for {field, type, byte_count} <- @hash_fields do
      maximum = (1 <<< (8 * byte_count)) - 1
      former_maximum = (1 <<< (8 * byte_count + 1)) - 1

      for integer <- [-1, maximum + 1, maximum + 2, former_maximum - 1, former_maximum, former_maximum + 1] do
        assert :error = type.cast(integer)
        changeset = Block.changeset(%Block{}, Map.put(params, field, integer))
        refute changeset.valid?
        assert {"is invalid", metadata} = Keyword.fetch!(changeset.errors, field)
        assert metadata[:validation] == :cast
        refute Map.has_key?(changeset.changes, field)
        assert {:error, rejected} = Changeset.apply_action(changeset, :insert)
        assert rejected == %{changeset | action: :insert}
      end
    end
  end

  test "hash-integer-truncation: an extra leading bit cannot alias a recorded chain identifier", %{params: params} do
    for {field, type, byte_count} <- @hash_fields do
      assert {:ok, recorded_hash} = type.cast(Map.fetch!(params, field))
      integer = Hash.to_integer(recorded_hash)
      overflow = integer + (1 <<< (8 * byte_count))

      assert {:ok, ^recorded_hash} = type.cast(integer)
      assert :error = type.cast(overflow)
      changeset = Block.changeset(%Block{}, Map.put(params, field, overflow))
      refute changeset.valid?
      assert Keyword.has_key?(changeset.errors, field)
      refute Map.has_key?(changeset.changes, field)
    end
  end

  defp assert_round_trip(type, hash, integer, byte_count) do
    assert hash.byte_count == byte_count
    assert Hash.to_integer(hash) == integer
    assert {:ok, bytes} = Ecto.Type.dump(type, hash)
    assert byte_size(bytes) == byte_count
    assert :binary.decode_unsigned(bytes) == integer
    assert {:ok, ^hash} = Ecto.Type.load(type, bytes)
    assert {:ok, ^hash} = type.cast(Jason.decode!(Jason.encode!(hash)))
    assert {:ok, ^hash} = type.cast(Poison.decode!(Poison.encode!(hash)))
  end
end
