defmodule Explorer.Repair.Repair2233536Test do
  use ExUnit.Case, async: false

  alias Ecto.Adapters.SQL.Sandbox
  alias Explorer.Chain.Import.Runner.Optimism.InteropMessages
  alias Explorer.Chain.Optimism.InteropMessage
  alias Explorer.{PagingOptions, Repo}

  # Schema transcript from the committed Optimism interop recordings in
  # block_scout_web/test/repair/repair_22_33_398_test.exs. Regression variants
  # vary the message identity, sender, target and relay outcome explicitly.
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

  setup do
    owner = Sandbox.start_owner!(Repo, shared: true)
    on_exit(fn -> Sandbox.stop_owner(owner) end)

    # Reproduce the production Optimism migration schema on the connection's
    # temporary search path, independent of the gate's default chain type.
    Repo.query!("""
    CREATE TEMP TABLE op_interop_messages (
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

    :ok
  end

  test "interop-page-boundary: query-string cursors traverse a multi-message transaction exactly once" do
    import_messages([
      %{"init_chain_id" => 3, "nonce" => 1},
      %{"nonce" => 2},
      %{"init_chain_id" => 3, "nonce" => 2},
      %{"nonce" => 3},
      %{"nonce" => 1}
    ])

    [first, boundary] = page(%PagingOptions{page_size: 2})
    assert identities([first, boundary]) == [{23, 3}, {23, 2}]

    cursor = transported_cursor(boundary)
    assert cursor["nonce"] == "2"
    assert cursor["init_chain_id"] == "23"
    assert cursor["timestamp"] == "1700000000"
    assert cursor["init_transaction_hash"] == to_string(first.init_transaction_hash)
    assert {:ok, paging} = InteropMessage.paging_options(cursor)
    [third, fourth] = page(%{paging | page_size: 2})
    assert identities([third, fourth]) == [{23, 1}, {3, 2}]

    assert {:ok, paging} = fourth |> transported_cursor() |> InteropMessage.paging_options()
    assert [last] = page(%{paging | page_size: 2})
    assert identities([last]) == [{3, 1}]

    assert {:ok, paging} = last |> transported_cursor() |> InteropMessage.paging_options()
    assert page(%{paging | page_size: 2}) == []

    traversed = [first, boundary, third, fourth, last]
    assert traversed == InteropMessage.list(statuses: [])
    assert length(Enum.uniq(identities(traversed))) == 5
    assert Enum.uniq_by(traversed, &{&1.timestamp, &1.init_transaction_hash}) == [first]
  end

  test "interop-empty-inclusion: complete sender or target overlap matches no imported messages" do
    [original, other] =
      import_messages([
        %{},
        %{
          "nonce" => 2,
          "sender_address_hash" => "0x7b9ea9ac69b8fde875554321472c732eeff06ca0",
          "target_address_hash" => "0x06012c8cf97bead5deae237070f9587f8e7a266d"
        }
      ])

    assert length(InteropMessage.list(statuses: [])) == 2

    for {option, field} <- [senders: :sender_address_hash, targets: :target_address_hash] do
      address = Map.fetch!(original, field)
      other_address = Map.fetch!(other, field)

      assert InteropMessage.list([statuses: []] ++ [{option, [include: [address], exclude: [address]]}]) == []

      assert [found] = InteropMessage.list([statuses: []] ++ [{option, [include: [address], exclude: []]}])
      assert found.nonce == original.nonce

      for filter <- [
            [include: [], exclude: [address]],
            [include: [address, other_address], exclude: [address]]
          ] do
        assert [found] = InteropMessage.list([statuses: []] ++ [{option, filter}])
        assert found.nonce == other.nonce
      end

      assert length(InteropMessage.list([statuses: []] ++ [{option, [include: [], exclude: []]}])) == 2
    end
  end

  test "interop-cursor-filter: tied timestamps cannot bypass sender, target, status or direction" do
    [cursor | _] =
      import_messages([
        %{"nonce" => 10, "relay_transaction_hash" => transaction(90), "failed" => false},
        %{
          "nonce" => 11,
          "init_transaction_hash" => transaction(8),
          "relay_transaction_hash" => transaction(90),
          "failed" => false
        },
        %{
          "nonce" => 12,
          "init_transaction_hash" => transaction(7),
          "relay_transaction_hash" => transaction(90),
          "failed" => false,
          "sender_address_hash" => "0x7b9ea9ac69b8fde875554321472c732eeff06ca0"
        },
        %{
          "nonce" => 13,
          "init_transaction_hash" => transaction(6),
          "relay_transaction_hash" => transaction(90),
          "failed" => true
        },
        %{"nonce" => 14, "timestamp" => 1_699_999_999, "relay_transaction_hash" => transaction(90), "failed" => false},
        %{
          "nonce" => 15,
          "init_transaction_hash" => transaction(5),
          "relay_transaction_hash" => transaction(90),
          "failed" => false,
          "target_address_hash" => "0x06012c8cf97bead5deae237070f9587f8e7a266d"
        },
        %{
          "nonce" => 16,
          "init_chain_id" => 45,
          "relay_chain_id" => 23,
          "init_transaction_hash" => transaction(4),
          "relay_transaction_hash" => transaction(90),
          "failed" => false
        },
        %{"nonce" => 17, "init_transaction_hash" => transaction(3)}
      ])

    filters = [
      statuses: ["RELAYED"],
      senders: [include: [cursor.sender_address_hash]],
      targets: [include: [cursor.target_address_hash]],
      direction: :out,
      current_chain_id: 23
    ]

    assert identities(InteropMessage.list(filters)) == [{23, 10}, {23, 11}, {23, 14}]
    assert {:ok, paging} = cursor |> transported_cursor() |> InteropMessage.paging_options()
    assert identities(InteropMessage.list([{:paging_options, paging} | filters])) == [{23, 11}, {23, 14}]

    legacy = %PagingOptions{page_size: 50, key: {DateTime.to_unix(cursor.timestamp), cursor.init_transaction_hash}}
    assert identities(InteropMessage.list([{:paging_options, legacy} | filters])) == [{23, 11}, {23, 14}]

    # The time branch must also remain inside the existing predicates.
    Repo.update!(
      Ecto.Changeset.change(InteropMessage.get_message(23, 12), timestamp: DateTime.add(cursor.timestamp, -2))
    )

    assert identities(InteropMessage.list([{:paging_options, paging} | filters])) == [{23, 11}, {23, 14}]
  end

  test "interop-page-boundary: malformed or incomplete cursors fail explicitly" do
    [message] = import_messages([%{}])
    params = transported_cursor(message)

    for key <- ~w(timestamp init_transaction_hash init_chain_id nonce) do
      assert :error = InteropMessage.paging_options(Map.delete(params, key))
      assert :error = InteropMessage.paging_options(Map.put(params, key, "invalid"))
      assert :error = InteropMessage.paging_options(Map.put(params, key, nil))
    end

    assert :error = InteropMessage.paging_options(Map.put(params, "nonce", "-1"))
    assert :error = InteropMessage.paging_options(Map.put(params, "init_chain_id", "0"))
    assert :error = InteropMessage.paging_options(Map.put(params, "timestamp", "999999999999999999"))
  end

  defp import_messages(variants) do
    recorded = Jason.decode!(@recording)

    changes =
      Enum.map(variants, fn variant ->
        attrs = Map.merge(recorded, variant)
        attrs = Map.update!(attrs, "timestamp", &DateTime.from_unix!/1)
        changeset = InteropMessage.changeset(%InteropMessage{}, attrs)
        assert changeset.valid?, inspect(changeset.errors)
        changeset.changes
      end)

    now = DateTime.utc_now()

    multi =
      InteropMessages.run(Ecto.Multi.new(), changes, %{
        timestamps: %{inserted_at: now, updated_at: now}
      })

    assert {:ok, %{insert_interop_messages: inserted}} = Repo.transaction(multi)
    assert length(inserted) == length(variants)

    Enum.map(changes, fn attrs ->
      message = InteropMessage.get_message(attrs.init_chain_id, attrs.nonce)
      assert message.payload == attrs.payload
      message
    end)
  end

  defp page(paging), do: InteropMessage.list(statuses: [], paging_options: paging)
  defp identities(messages), do: Enum.map(messages, &{&1.init_chain_id, &1.nonce})

  defp transported_cursor(message) do
    message |> InteropMessage.paging_params() |> URI.encode_query() |> URI.decode_query()
  end

  defp transaction(number), do: "0x" <> String.pad_leading(Integer.to_string(number, 16), 64, "0")
end
