defmodule Explorer.Chain.SmartContract.VerificationStatus do
  @moduledoc """
  Represents single verification try
  """

  use Explorer.Schema

  import Ecto.Changeset

  alias Explorer.{Chain, Repo}
  alias Explorer.Chain.Hash
  alias Explorer.SmartContract.Solidity.PublisherWorker, as: SolidityPublisherWorker
  alias Que.Persistence, as: QuePersistence

  @typedoc """
  * `contract_address_hash` - address of the contract which was tried to verify
  * `status` - try status: :pending | :pass | :fail
  * `uid` - unique verification try identifier
  """
  @primary_key false
  typed_schema "smart_contract_verification_statuses" do
    field(:uid, :string, primary_key: true, null: false)
    field(:status, :integer, null: false)
    field(:contract_address_hash, Hash.Address, null: false)

    timestamps()
  end

  @required_fields ~w(uid status contract_address_hash)a

  def changeset(%__MODULE__{} = struct, params \\ %{}) do
    casted_params = encode_status(params)

    struct
    |> cast(casted_params, @required_fields)
    |> validate_required(@required_fields)
    |> validate_change(:uid, fn :uid, uid ->
      case validate_uid(uid) do
        {:ok, _} -> []
        :error -> [uid: "is invalid"]
      end
    end)
    |> unique_constraint(:uid, name: :smart_contract_verification_statuses_pkey)
  end

  def encode_status(%{status: status} = changeset) do
    case status do
      :pending ->
        Map.put(changeset, :status, 0)

      :pass ->
        Map.put(changeset, :status, 1)

      :fail ->
        Map.put(changeset, :status, 2)

      _ ->
        changeset
    end
  end

  def encode_status(changeset), do: changeset

  def decode_status(number) when number in [0, 1, 2, 3] do
    case number do
      0 ->
        :pending

      1 ->
        :pass

      2 ->
        :fail

      3 ->
        :unknown_uid
    end
  end

  def insert_status(uid, status, address_hash) do
    {:ok, hash} = if is_binary(address_hash), do: Chain.string_to_address_hash(address_hash), else: address_hash

    result =
      %__MODULE__{}
      |> changeset(%{uid: uid, status: status, contract_address_hash: hash})
      |> Repo.insert(mode: :savepoint)

    case result do
      {:error, changeset} ->
        if Enum.any?(changeset.errors, fn {field, {_, options}} ->
             field == :uid and options[:constraint] == :unique
           end) do
          raise Ecto.InvalidChangesetError, action: :insert, changeset: changeset
        else
          result
        end

      _ ->
        result
    end
  end

  def update_status(uid, status) do
    __MODULE__
    |> Repo.get_by(uid: uid)
    |> changeset(%{status: status})
    |> Repo.update()
  end

  def fetch_status(uid) do
    case validate_uid(uid) do
      {:ok, valid_uid} ->
        __MODULE__
        |> Repo.get_by(uid: valid_uid)
        |> (&if(is_nil(&1), do: 3, else: Map.get(&1, :status))).()
        |> decode_status()
        |> mb_find_uid_in_queue(uid)

      _ ->
        :unknown_uid
    end
  end

  def generate_uid(address_hash) do
    case Chain.string_to_address_hash(address_hash) do
      :error ->
        nil

      {:ok, %Hash{byte_count: 20}} ->
        32 |> :crypto.strong_rand_bytes() |> Base.encode16(case: :lower)
    end
  end

  def validate_uid(<<_::binary-size(64)>> = uid) do
    case Base.decode16(uid, case: :lower) do
      {:ok, _} -> {:ok, uid}
      :error -> :error
    end
  end

  def validate_uid(<<address::binary-size(40), timestamp_hex::binary>> = uid)
      when byte_size(timestamp_hex) in 1..16 do
    with {:ok, _} <- Base.decode16(address, case: :mixed),
         {:ok, _} <- Chain.string_to_address_hash("0x" <> address),
         true <- Regex.match?(~r/\A[0-9a-fA-F]+\z/, timestamp_hex),
         {timestamp, ""} <- Integer.parse(timestamp_hex, 16),
         true <- timestamp <= DateTime.to_unix(DateTime.utc_now()) do
      {:ok, uid}
    else
      _ -> :error
    end
  end

  def validate_uid(_), do: :error

  defp mb_find_uid_in_queue(:unknown_uid, uid) do
    SolidityPublisherWorker
    |> QuePersistence.all()
    |> Enum.find_value(fn
      %Que.Job{arguments: {"flattened_api", _, _, ^uid}} ->
        :pending

      %Que.Job{arguments: {"json_api", _, _, ^uid}} ->
        :pending

      _ ->
        nil
    end) || :unknown_uid
  end

  defp mb_find_uid_in_queue(other_status, _), do: other_status
end
