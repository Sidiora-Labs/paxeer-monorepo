defmodule Explorer.Repair.Repair2233492Test do
  use Explorer.DataCase

  alias Explorer.Account
  alias Explorer.Account.Notifier.{Email, Notify, Summary}
  alias Explorer.Account.{WatchlistAddress, WatchlistNotification}
  alias Explorer.Chain.{TokenTransfer, Wei}
  alias Explorer.Mailer

  # Schema recording from indexer/test/indexer/transform/token_transfers_test.exs,
  # "parses erc7984 confidential token transfer". No plaintext amount exists.
  @confidential_recording ~S({
    "block_hash": "0xfedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321",
    "block_number": 12345678,
    "from_address_hash": "0xa1b2c3d4e5f6789012345678901234567890abcd",
    "log_index": 10,
    "to_address_hash": "0xb2c3d4e5f67890123456789012345678901abcde",
    "token_contract_address_hash": "0x1234567890123456789012345678901234567890",
    "amount": null,
    "token_ids": null,
    "token_type": "ERC-7984",
    "transaction_hash": "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
  })

  defmodule SendGridRecording do
    @behaviour Plug
    def init(observer), do: observer

    def call(conn, observer) do
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      send(observer, {:sendgrid_request, conn.method, conn.request_path, conn.req_headers, Jason.decode!(body)})
      Plug.Conn.send_resp(conn, 202, "")
    end
  end

  setup do
    keys = [
      {:explorer, Account},
      {:explorer, Mailer},
      {:bamboo, :sendgrid_base_uri},
      {:block_scout_web, BlockScoutWeb.Endpoint},
      {:block_scout_web, :chain_id}
    ]

    saved = Enum.map(keys, fn {app, key} -> {app, key, Application.fetch_env(app, key)} end)

    on_exit(fn ->
      Enum.each(saved, fn
        {app, key, {:ok, value}} -> Application.put_env(app, key, value)
        {app, key, :error} -> Application.delete_env(app, key)
      end)
    end)

    {:ok, _} = Application.ensure_all_started(:plug_cowboy)
    ref = make_ref()

    start_supervised!(
      Supervisor.child_spec(
        {Plug.Cowboy,
         scheme: :http, plug: {SendGridRecording, self()}, options: [port: 0, ref: ref, ip: {127, 0, 0, 1}]},
        id: ref
      )
    )

    Application.put_env(:bamboo, :sendgrid_base_uri, "http://127.0.0.1:#{:ranch.get_port(ref)}")
    Application.put_env(:explorer, Mailer, adapter: Bamboo.SendGridAdapter, api_key: "recorded-sendgrid-key")

    Application.put_env(:explorer, Account,
      sendgrid: [sender: "notifications@example.test", template: "d-recorded-watchlist"],
      watchlist_addresses_limit: 20,
      notifications_limit_for_30_days: 100
    )

    Application.put_env(:block_scout_web, BlockScoutWeb.Endpoint,
      url: [scheme: "https", host: "explorer.example.test", path: "/", port: 443]
    )

    Application.put_env(:block_scout_web, :chain_id, "125")
    {:ok, transaction: replay_confidential_transfer()}
  end

  test "notification-erc7984-amount: Summary reaches the real SendGrid transport with a confidential amount", %{
    transaction: transaction
  } do
    address = watch(transaction.to_address_hash, %{watch_erc_20_input: true})
    assert [%Summary{type: "ERC-7984", subject: "Confidential transfer", amount: amount}] = Summary.process(transaction)
    assert Decimal.equal?(amount, 0)
    assert Notify.call([transaction]) == [[:ok]]

    notification = Repo.account_repo().one!(WatchlistNotification)
    assert notification.watchlist_address_id == address.id
    assert notification.type == "ERC-7984"
    assert notification.subject == "Confidential transfer"
    assert notification.direction == "incoming"
    assert notification.transaction_hash == transaction.hash
    assert Decimal.equal?(notification.amount, 0)

    data = delivered_data()
    assert data["amount"] == "Confidential amount"
    assert data["name"] == "Recorded confidential token"
    assert data["direction"] == "received at"
    assert data["transaction_hash"] == to_string(transaction.hash)
    assert data["transaction_url"] == "https://explorer.example.test/tx/#{transaction.hash}"
    refute_receive {:sendgrid_request, _, _, _, _}
  end

  test "routing: mixed batch continues after a confidential transfer", %{transaction: confidential} do
    address = watch(confidential.to_address_hash, %{watch_erc_20_input: true, watch_coin_input: true})
    recipient = Repo.get!(Explorer.Chain.Address, address.address_hash)
    coin = insert(:transaction, to_address: recipient, value: 2_000_000_000_000_000_000) |> with_block()
    fungible = insert(:transaction, to_address: recipient, value: 0) |> with_block()
    token = insert(:token, type: "ERC-20", decimals: 6, name: "Recorded public token")

    insert(:token_transfer,
      transaction: fungible,
      block: fungible.block,
      block_number: fungible.block_number,
      to_address: recipient,
      token: token,
      token_contract_address: token.contract_address,
      token_type: "ERC-20",
      amount: Decimal.new(3_000_000)
    )

    assert Notify.call([confidential, coin, fungible]) == [[:ok], [:ok], [:ok]]

    assert Repo.account_repo().all(from(n in WatchlistNotification, order_by: n.id, select: n.type)) ==
             ["ERC-7984", "COIN", "ERC-20"]

    assert delivered_data()["amount"] == "Confidential amount"
    assert Decimal.equal?(Decimal.new(delivered_data()["amount"]), 2)
    assert Decimal.equal?(Decimal.new(delivered_data()["amount"]), 3)
    refute_receive {:sendgrid_request, _, _, _, _}
  end

  for direction <- [:incoming, :outgoing], enabled <- [true, false] do
    @direction direction
    @enabled enabled
    test "routing: persisted fungible preferences control each confidential direction #{@direction} #{@enabled}", %{
      transaction: transaction
    } do
      {hash, preferences} =
        case @direction do
          :incoming ->
            {transaction.to_address_hash, %{watch_erc_20_input: @enabled, watch_erc_20_output: !@enabled}}

          :outgoing ->
            {transaction.from_address_hash, %{watch_erc_20_input: !@enabled, watch_erc_20_output: @enabled}}
        end

      address = watch(hash, preferences)
      reloaded = Repo.account_repo().get!(WatchlistAddress, address.id)
      assert reloaded.watch_erc_20_input == preferences.watch_erc_20_input
      assert reloaded.watch_erc_20_output == preferences.watch_erc_20_output
      assert Notify.call([transaction]) == [[:ok]]

      if @enabled do
        notification = Repo.account_repo().one!(WatchlistNotification)
        assert notification.direction == to_string(@direction)
        data = delivered_data()
        assert data["amount"] == "Confidential amount"
        assert data["direction"] == if(@direction == :incoming, do: "received at", else: "sent from")
      else
        assert Repo.account_repo().all(WatchlistNotification) == []
        refute_receive {:sendgrid_request, _, _, _, _}
      end
    end
  end

  test "routing: notify_email false stores without sending", %{transaction: transaction} do
    address = watch(transaction.to_address_hash, %{watch_erc_20_input: true, notify_email: false})
    assert Notify.call([transaction]) == [[:ok]]
    notification = Repo.account_repo().one!(WatchlistNotification)
    assert notification.type == "ERC-7984"
    assert Email.compose(notification, address) == nil
    refute_receive {:sendgrid_request, _, _, _, _}
  end

  test "routing: every Summary token type has an explicit preference and email representation", %{
    transaction: transaction
  } do
    address =
      watch(transaction.to_address_hash, %{
        watch_coin_input: true,
        watch_coin_output: true,
        watch_erc_20_input: true,
        watch_erc_20_output: true
      })
      |> Repo.account_repo().preload(watchlist: :identity)

    transfer = Repo.one!(TokenTransfer) |> Repo.preload(:token)

    for type <- ["ERC-20", "ZRC-2", "ERC-721", "ERC-1155", "ERC-404", "ERC-7984"] do
      typed_transfer = %{
        transfer
        | token: %{transfer.token | type: type},
          amount: Decimal.new(5),
          token_ids: [Decimal.new(42)]
      }

      assert [%Summary{type: ^type} = summary] = Summary.handle_collection(transaction, [typed_transfer])

      for direction <- [:incoming, :outgoing] do
        notification = Notify.build_watchlist_notification(address, summary, direction)

        if type == "ZRC-2" and not Map.has_key?(address, :watch_zrc_2_input) do
          assert notification == nil
        else
          assert %WatchlistNotification{type: ^type} = notification
          email = Email.compose(%{notification | watchlist_address: address}, address)
          amount = email.private.send_grid_template.dynamic_template_data["amount"]

          case type do
            "ERC-7984" -> assert amount == "Confidential amount"
            type when type in ["ERC-721", "ERC-1155", "ERC-404"] -> assert amount == "Token ID: 42 of "
            _ -> assert Decimal.equal?(amount, summary.amount)
          end
        end
      end
    end

    coin = %{transaction | value: %Wei{value: Decimal.new(1_000_000_000_000_000_000)}, token_transfers: []}
    assert [%Summary{type: "COIN"} = summary] = Summary.process(coin)

    for direction <- [:incoming, :outgoing] do
      notification = Notify.build_watchlist_notification(address, summary, direction)
      assert notification.type == "COIN"
      email = Email.compose(%{notification | watchlist_address: address}, address)
      assert Decimal.equal?(email.private.send_grid_template.dynamic_template_data["amount"], 1)
    end
  end

  defp replay_confidential_transfer do
    recording = Jason.decode!(@confidential_recording, keys: :atoms)
    from = insert(:address, hash: recording.from_address_hash)
    to = insert(:address, hash: recording.to_address_hash)
    contract = insert(:contract_address, hash: recording.token_contract_address_hash)
    insert(:token, contract_address: contract, type: "ERC-7984", name: "Recorded confidential token")
    block = insert(:block, hash: recording.block_hash, number: recording.block_number, timestamp: DateTime.utc_now())

    transaction =
      insert(:transaction, hash: recording.transaction_hash, from_address: from, to_address: to, value: 0)
      |> with_block(block)

    %TokenTransfer{}
    |> TokenTransfer.changeset(Map.put(recording, :block_consensus, true))
    |> Repo.insert!()

    transaction
  end

  defp watch(hash, preferences) do
    watchlist = build(:account_watchlist) |> Repo.account_repo().insert!()

    attrs =
      Map.merge(
        %{
          watchlist_id: watchlist.id,
          address_hash: to_string(hash),
          name: "Recorded wallet",
          watch_coin_input: false,
          watch_coin_output: false,
          watch_erc_20_input: false,
          watch_erc_20_output: false,
          notify_email: true
        },
        preferences
      )

    assert {:ok, address} = WatchlistAddress.create(attrs)
    address
  end

  defp delivered_data do
    assert_receive {:sendgrid_request, "POST", "/mail/send", headers, body}, 1_000
    assert {"authorization", "Bearer recorded-sendgrid-key"} in headers
    assert body["from"]["email"] == "notifications@example.test"
    assert body["template_id"] == "d-recorded-watchlist"
    assert [%{"to" => [%{"email" => recipient}], "dynamic_template_data" => data}] = body["personalizations"]
    assert String.ends_with?(recipient, "@blockscout.com")
    data
  end
end
