defmodule Explorer.Chain.SmartContract.Proxy.VerificationStatus do
  @moduledoc """
    Represents single proxy verification submission
  """

  use Explorer.Schema

  import Ecto.Changeset

  alias Explorer.{Chain, Repo}
  alias Explorer.Chain.Hash
  alias Explorer.Chain.SmartContract.Proxy.Models.Implementation

  @typep status :: integer() | atom()

  @typedoc """
  * `contract_address_hash` - address of the contract which was tried to verify
  * `status` - submission status: :pending | :pass | :fail
  * `uid` - unique verification identifier
  """
  @primary_key false
  typed_schema "proxy_smart_contract_verification_statuses" do
    field(:uid, :string, primary_key: true, null: false)
    field(:status, Ecto.Enum, values: [pending: 0, pass: 1, fail: 2], null: false)
    field(:contract_address_hash, Hash.Address, null: false)

    timestamps()
  end

  @required_fields ~w(uid status contract_address_hash)a

  @doc """
    Creates a changeset based on the `struct` and `params`.
  """
  @spec changeset(Explorer.Chain.SmartContract.Proxy.VerificationStatus.t()) :: Ecto.Changeset.t()
  def changeset(%__MODULE__{} = struct, params \\ %{}) do
    struct
    |> cast(params, @required_fields)
    |> validate_required(@required_fields)
    |> unique_constraint(:uid, name: :proxy_smart_contract_verification_statuses_pkey)
  end

  @doc """
    Inserts verification status. A duplicate uid returns an error changeset without
    overwriting the existing attempt or aborting the caller's transaction.
  """
  @spec insert_status(String.t(), status(), Hash.Address.t() | String.t()) :: any()
  def insert_status(uid, status, address_hash) do
    {:ok, hash} = if is_binary(address_hash), do: Chain.string_to_address_hash(address_hash), else: {:ok, address_hash}

    %__MODULE__{}
    |> changeset(%{uid: uid, status: status, contract_address_hash: hash})
    |> Repo.insert(mode: :savepoint)
  end

  @doc """
    Updates verification status
  """
  @spec update_status(String.t(), status()) :: __MODULE__.t()
  def update_status(uid, status) do
    __MODULE__
    |> Repo.get_by(uid: uid)
    |> changeset(%{status: status})
    |> Repo.update()
  end

  @doc """
    Fetches verification status
  """
  @spec fetch_status(binary()) :: __MODULE__.t() | nil
  def fetch_status(uid) do
    case validate_uid(uid) do
      {:ok, valid_uid} ->
        __MODULE__
        |> Repo.get_by(uid: valid_uid)

      _ ->
        nil
    end
  end

  @doc """
    Generates a 256-bit random attempt identifier within the 64-character database limit.
  """
  @spec generate_uid(Explorer.Chain.Hash.t()) :: String.t()
  def generate_uid(%Hash{byte_count: 20}) do
    32 |> :crypto.strong_rand_bytes() |> Base.encode16(case: :lower)
  end

  @doc """
    Validates uid
  """
  @spec validate_uid(term()) :: :error | {:ok, String.t()}
  def validate_uid(<<_::binary-size(64)>> = uid) do
    case Base.decode16(uid, case: :lower) do
      {:ok, _} -> {:ok, uid}
      :error -> :error
    end
  end

  # Keep previously issued address/timestamp identifiers pollable.
  def validate_uid(<<address::binary-size(40), timestamp_hex::binary>> = uid) when byte_size(uid) < 64 do
    with {:ok, _} <- Base.decode16(address, case: :lower),
         {timestamp, ""} when timestamp >= 0 <- Integer.parse(timestamp_hex, 16),
         true <- timestamp_hex == String.downcase(Integer.to_string(timestamp, 16)),
         true <- DateTime.to_unix(DateTime.utc_now()) >= timestamp do
      {:ok, uid}
    else
      _ -> :error
    end
  end

  def validate_uid(_), do: :error

  @doc """
    Sets proxy verification result
  """
  @spec set_proxy_verification_result(Implementation.t() | :empty | :error, String.t()) :: __MODULE__.t()
  def set_proxy_verification_result(%Implementation{}, uid), do: update_status(uid, :pass)

  def set_proxy_verification_result(_empty_or_error, uid), do: update_status(uid, :fail)
end

