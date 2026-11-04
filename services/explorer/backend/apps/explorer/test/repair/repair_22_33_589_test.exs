defmodule Explorer.Repair.Repair2233589Test do
  use ExUnit.Case, async: false

  alias Explorer.Chain.Import.Runner.Optimism.InteropMessages
  alias Explorer.Chain.Optimism.InteropMessage
  alias Explorer.Repo

  # Committed interop transcript from repair_22_33_536_test.exs, also used by
  # the controller and MessageQueue recordings. Only export state varies.
  @recording ~S({
    "nonce": 1,
    "init_chain_id": 23,
    "relay_chain_id": 45,
    "sender_address_hash": "0x06012c8cf97bead5deae237070f9587f8e7a266d",
    "target_address_hash": "0x7b9ea9ac69b8fde875554321472c732eeff06ca0",
    "init_transaction_hash": "0xf463ce43ac52251f60f437a0346b7168874e6eb13a9689d7efde4ce907826897",
    "timestamp": 1700000000,
    "payload": "0x30787849217424792273712777707576696a6c5d"
  })
  @imported_at ~U[2023-11-14 22:13:20.000000Z]

  setup do
    original_repo = Repo.get_dynamic_repo()
    schema = "repair_2233589_#{System.unique_integer([:positive])}"
    Repo.query!("CREATE SCHEMA #{schema}")

    on_exit(fn ->
      Repo.put_dynamic_repo(original_repo)
      Repo.query!("DROP SCHEMA #{schema} CASCADE")
    end)

    repo = open_repo(schema)

    # The final Optimism migration schema, including the nullable export flag,
    # is isolated from the gate's default chain and survives pool restarts.
    Repo.query!("""
    CREATE TABLE op_interop_messages (
      sender_address_hash bytea, target_address_hash bytea,
      nonce bigint NOT NULL, init_chain_id integer NOT NULL,
      init_transaction_hash bytea, block_number bigint, timestamp timestamp(6),
      relay_chain_id integer NOT NULL, relay_transaction_hash bytea,
      payload bytea, failed boolean, transfer_token_address_hash bytea,
      transfer_from_address_hash bytea, transfer_to_address_hash bytea,
      transfer_amount numeric, sent_to_multichain boolean,
      inserted_at timestamp(6) NOT NULL, updated_at timestamp(6) NOT NULL,
      PRIMARY KEY (nonce, init_chain_id)
    )
    """)

    %{schema: schema, repo: repo}
  end

  test "interop-export-state-update: false-to-true correction survives repository reopening", context do
    params = recorded_message()
    assert [original] = import_message(Map.put(params, :sent_to_multichain, false))
    assert original.sent_to_multichain == false
    assert InteropMessage.get_messages_for_multichain_export(23, 10) == [original]

    corrected_params = Map.put(params, :sent_to_multichain, true)
    assert [corrected] = import_message(corrected_params)
    assert corrected == %{original | sent_to_multichain: true}
    assert InteropMessage.get_message(23, 1) == corrected
    assert InteropMessage.get_messages_for_multichain_export(23, 10) == []

    assert :ok = stop_supervised(Repo)
    refute Process.alive?(context.repo)
    reopened = open_repo(context.schema)
    refute reopened == context.repo

    assert InteropMessage.get_message(23, 1) == corrected
    assert InteropMessage.get_messages_for_multichain_export(23, 10) == []
    assert import_message(corrected_params, DateTime.add(@imported_at, 60)) == []
    assert InteropMessage.get_message(23, 1) == corrected
    assert Repo.aggregate(InteropMessage, :count) == 1
  end

  test "interop-export-state-update: nullable state accepts explicit true and false corrections" do
    for {nonce, state} <- [{1, true}, {2, false}] do
      params = Map.put(recorded_message(), :nonce, nonce)
      assert [original] = import_message(params)
      assert is_nil(original.sent_to_multichain)

      assert [corrected] = import_message(Map.put(params, :sent_to_multichain, state))
      assert corrected == %{original | sent_to_multichain: state}
      assert InteropMessage.get_message(23, nonce) == corrected
    end

    assert [%{nonce: 2, sent_to_multichain: false}] = InteropMessage.get_messages_for_multichain_export(23, 10)
    assert Repo.aggregate(InteropMessage, :count) == 2
  end

  test "interop-export-state-update: null replays preserve known state through COALESCE" do
    for {nonce, state} <- [{1, true}, {2, false}, {3, nil}] do
      params = Map.put(recorded_message(), :nonce, nonce)
      assert [original] = import_message(Map.put(params, :sent_to_multichain, state))
      assert original.sent_to_multichain == state

      import_message(Map.put(params, :sent_to_multichain, nil))
      assert InteropMessage.get_message(23, nonce) == original
      import_message(params)
      assert InteropMessage.get_message(23, nonce) == original

      assert import_message(Map.put(params, :sent_to_multichain, state), DateTime.add(@imported_at, 60)) == []
      assert InteropMessage.get_message(23, nonce) == original
    end

    assert Repo.aggregate(InteropMessage, :count) == 3
    assert Enum.sort(Enum.map(InteropMessage.get_messages_for_multichain_export(23, 10), & &1.nonce)) == [2, 3]
  end

  defp open_repo(schema) do
    repo =
      start_supervised!(
        {Repo, name: nil, pool: DBConnection.ConnectionPool, pool_size: 1, parameters: [search_path: schema]}
      )

    Repo.put_dynamic_repo(repo)
    repo
  end

  defp recorded_message do
    attrs = @recording |> Jason.decode!() |> Map.update!("timestamp", &DateTime.from_unix!/1)
    changeset = InteropMessage.changeset(%InteropMessage{}, attrs)
    assert changeset.valid?, inspect(changeset.errors)
    changeset.changes
  end

  defp import_message(params, timestamp \\ @imported_at) do
    multi =
      InteropMessages.run(Ecto.Multi.new(), [params], %{
        timestamps: %{inserted_at: timestamp, updated_at: timestamp}
      })

    assert {:ok, %{insert_interop_messages: messages}} = Repo.transaction(multi)
    messages
  end
end
