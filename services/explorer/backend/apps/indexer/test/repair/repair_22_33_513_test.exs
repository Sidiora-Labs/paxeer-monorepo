defmodule Indexer.Repair2233513Test do
  use Explorer.DataCase, async: false

  alias EthereumJSONRPC.Blocks
  alias Explorer.Chain
  alias Explorer.Chain.Block
  alias Explorer.Chain.Events.{DBSender, Publisher, Subscriber}
  alias Explorer.Repo.EventNotifications, as: EventNotificationsRepo
  alias Explorer.Utility.EventNotification
  alias Indexer.Transform.Blocks, as: TransformBlocks
  alias Indexer.Utils.{EventNotificationsCleaner, EventNotificationsRecovery}

  setup do
    cleaner_config = Application.get_env(:indexer, EventNotificationsCleaner)
    sender = Application.get_env(:explorer, :realtime_events_sender)

    Application.put_env(:indexer, EventNotificationsCleaner, max_age: 25, interval: 60_000)
    Application.put_env(:explorer, :realtime_events_sender, DBSender)

    # NOTIFY is delivered only at COMMIT. Keep the authoritative-chain sandbox,
    # but use committed writes for the real notification sender and cleaner.
    Ecto.Adapters.SQL.Sandbox.checkin(EventNotificationsRepo)
    :ok = Ecto.Adapters.SQL.Sandbox.checkout(EventNotificationsRepo, sandbox: false)
    Ecto.Adapters.SQL.Sandbox.mode(EventNotificationsRepo, {:shared, self()})

    on_exit(fn ->
      Application.put_env(:indexer, EventNotificationsCleaner, cleaner_config)
      Application.put_env(:explorer, :realtime_events_sender, sender)

      Ecto.Adapters.SQL.Sandbox.unboxed_run(EventNotificationsRepo, fn ->
        EventNotificationsRepo.delete_all(EventNotification)
      end)
    end)

    :ok
  end

  test "notification-recovery: expired outage payloads reconcile after listener stop and reopen" do
    block = import_recorded_block()
    Subscriber.to(:blocks, :realtime)
    Subscriber.to(:exchange_rate)
    listener = start_supervised!(EventNotificationsRecovery)
    assert EventNotificationsRecovery.subscribe() == %{head: %{number: block.number, hash: block.hash}}

    Publisher.broadcast(%{blocks: [block]}, :realtime)
    assert_receive {:chain_event, :blocks, :realtime, [^block]}, 2_000
    assert EventNotificationsRepo.aggregate(EventNotification, :count) == 1

    {connection, _, _} = :sys.get_state(EventNotificationsRecovery)
    listener_monitor = Process.monitor(listener)
    connection_monitor = Process.monitor(connection)
    :ok = stop_supervised(EventNotificationsRecovery)
    assert_receive {:DOWN, ^listener_monitor, :process, ^listener, :shutdown}, 2_000
    assert_receive {:DOWN, ^connection_monitor, :process, ^connection, :normal}, 2_000
    refute Process.alive?(listener)
    refute Process.alive?(connection)

    # This is a real producer event carrying an imported, recorded chain block.
    # No notification is queued by PostgreSQL while the listener is offline.
    Publisher.broadcast(%{blocks: [block]}, :realtime)
    assert EventNotificationsRepo.aggregate(EventNotification, :count) == 2
    Process.sleep(50)
    cleaner = start_supervised!(EventNotificationsCleaner)
    :sys.get_state(cleaner)
    assert EventNotificationsRepo.aggregate(EventNotification, :count) == 0

    recovered = start_supervised!(EventNotificationsRecovery)
    assert_receive {:chain_event, :event_notifications_reconcile, %{head: head}}, 2_000
    assert head == %{number: block.number, hash: block.hash}
    assert Process.alive?(recovered)
    refute_received {:chain_event, :blocks, :realtime, _}
    assert Repo.get!(Block, head.hash) == block
    assert Chain.max_consensus_block_number(api?: false) == {:ok, block.number}

    # Delivery resumes through DBSender -> PostgreSQL -> Listener -> Registry.
    Publisher.broadcast(%{blocks: [block]}, :realtime)
    assert_receive {:chain_event, :blocks, :realtime, [^block]}, 2_000
    Publisher.broadcast(:exchange_rate)
    assert_receive {:chain_event, :exchange_rate}, 2_000
  end

  test "notification-recovery: transport failure restarts LISTEN and invalidates unchanged-height state" do
    block = import_recorded_block()
    Subscriber.to(:blocks, :realtime)
    listener = start_supervised!(EventNotificationsRecovery)
    assert EventNotificationsRecovery.subscribe().head.hash == block.hash
    {connection, _, _} = :sys.get_state(listener)
    monitor = Process.monitor(listener)

    # Change the canonical hash at the same height: a numeric high-water mark
    # alone must not suppress reconciliation following reconnect.
    replacement_hash = "0x" <> String.duplicate("ab", 32)
    {:ok, replacement_hash} = Chain.string_to_full_hash(replacement_hash)
    {1, _} = Repo.update_all(from(b in Block, where: b.hash == ^block.hash), set: [consensus: false])
    replacement = %{block | hash: replacement_hash, consensus: true}
    {:ok, replacement} = replacement |> Ecto.Changeset.change() |> Repo.insert()

    Process.exit(connection, :kill)
    assert_receive {:DOWN, ^monitor, :process, ^listener, {:notification_connection_closed, :killed}}, 2_000
    assert_receive {:chain_event, :event_notifications_reconcile, %{head: head}}, 2_000
    assert head == %{number: block.number, hash: replacement.hash}
    assert Process.whereis(EventNotificationsRecovery) != listener
    assert Repo.get!(Block, block.hash).consensus == false
    assert Repo.get!(Block, replacement.hash).consensus

    Publisher.broadcast(%{blocks: [replacement]}, :realtime)
    assert_receive {:chain_event, :blocks, :realtime, [^replacement]}, 2_000

    # A normal transport exit also leaves the listener unable to receive events.
    # It must restart rather than continue with a dead connection.
    restarted = Process.whereis(EventNotificationsRecovery)
    {connection, _, _} = :sys.get_state(restarted)
    monitor = Process.monitor(restarted)
    :ok = GenServer.stop(connection)
    assert_receive {:DOWN, ^monitor, :process, ^restarted, {:notification_connection_closed, :normal}}, 2_000
    assert_receive {:chain_event, :event_notifications_reconcile, %{head: ^head}}, 2_000
    assert Process.whereis(EventNotificationsRecovery) != restarted
    Publisher.broadcast(%{blocks: [replacement]}, :realtime)
    assert_receive {:chain_event, :blocks, :realtime, [^replacement]}, 2_000
  end

  test "notification-recovery: empty authoritative state is explicit and payload retention stays bounded" do
    Subscriber.to(:exchange_rate)
    start_supervised!(EventNotificationsRecovery)
    assert EventNotificationsRecovery.subscribe() == %{head: nil}

    # Keep the young payload well inside retention while the old one expires.
    Application.put_env(:indexer, EventNotificationsCleaner, max_age: 60_000, interval: 60_000)
    old = DateTime.add(DateTime.utc_now(), -120, :second)

    Publisher.broadcast(%{blocks: []}, :realtime)
    old_id = EventNotificationsRepo.one!(from(n in EventNotification, select: n.id))

    {1, _} =
      EventNotificationsRepo.update_all(from(n in EventNotification, where: n.id == ^old_id), set: [inserted_at: old])

    Publisher.broadcast(%{blocks: []}, :realtime)
    cleaner = start_supervised!(EventNotificationsCleaner)
    :sys.get_state(cleaner)

    refute EventNotificationsRepo.get(EventNotification, old_id)
    assert EventNotificationsRepo.aggregate(EventNotification, :count) == 1
    assert Repo.aggregate(Block, :count) == 0
    Publisher.broadcast(:exchange_rate)
    assert_receive {:chain_event, :exchange_rate}, 2_000
  end

  defp import_recorded_block do
    path =
      Path.expand(
        "../../../explorer/test/support/fixture/vcr_cassettes/block_importer_import_1_pending.json",
        __DIR__
      )

    [record] = path |> File.read!() |> Jason.decode!()
    raw = record["response"]["body"] |> Jason.decode!() |> Map.fetch!("result")
    fetched = Blocks.from_responses([%{id: 0, result: raw}], %{0 => %{number: raw["number"]}})
    assert fetched.errors == []
    blocks = fetched.blocks_params |> TransformBlocks.transform_blocks() |> Enum.map(&Map.put(&1, :consensus, true))

    addresses = Indexer.Transform.Addresses.extract_addresses(%{blocks: blocks})

    assert {:ok, %{blocks: [block]}} =
             Chain.import(%{addresses: %{params: addresses}, blocks: %{params: blocks}, broadcast: false})

    Repo.get!(Block, block.hash)
  end
end
