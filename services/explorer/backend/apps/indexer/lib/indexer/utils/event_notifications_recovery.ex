defmodule Indexer.Utils.EventNotificationsRecovery do
  @moduledoc """
  A PostgreSQL event listener with explicit state reconciliation.

  Chain events are disposable, best-effort invalidations, not an acknowledged
  event log. Notification retention only allows a live listener to expand an id;
  it does not promise replay. Authoritative chain data remains in Explorer.Repo.

  This listener completes LISTEN before reading the canonical head. Subscribers
  receive `{:chain_event, :event_notifications_reconcile, %{head: head}}` on each
  listener start, including after a lost connection. They must reload their
  resources from the authoritative API, even when the head number is unchanged
  (a reorg can replace its hash). An empty chain has a nil head.

  Consumers call subscribe/0 on their own startup, before accepting live events.
  Its returned snapshot requires the same reload. Notifications queued during
  the snapshot are subsequently dispatched by the existing Explorer listener.
  """

  use GenServer

  import Ecto.Query

  alias Explorer.Chain.Block
  alias Explorer.Chain.Events.Listener
  alias Explorer.Repo
  alias Explorer.Repo.ConfigHelper
  alias Explorer.Repo.EventNotifications, as: EventNotificationsRepo
  alias Postgrex.Notifications

  @reconciliation_key :event_notifications_reconcile

  def start_link(args) do
    GenServer.start_link(__MODULE__, args, name: __MODULE__)
  end

  @doc """
  Registers the caller for reconnect invalidations and returns current state.

  Resource event subscriptions must be installed before this call. Snapshots
  and notifications can overlap; consumers reload state rather than applying
  notifications as an exactly-once delta stream.
  """
  def subscribe do
    {:ok, _} = Registry.register(Registry.ChainEvents, @reconciliation_key, [])
    GenServer.call(__MODULE__, :snapshot)
  end

  @impl true
  def init(_args) do
    # A supervisor shuts this process down with an exit signal. Trap it so
    # terminate/2 closes our linked notification connection before we return.
    Process.flag(:trap_exit, true)
    config = Application.get_env(:explorer, EventNotificationsRepo)

    options =
      config
      |> Keyword.merge(ConfigHelper.extract_parameters(config[:url]))
      |> Keyword.put(:sync_connect, true)
      |> Keyword.put(:auto_reconnect, false)

    # A lost connection must restart this listener and repeat LISTEN + snapshot.
    # Transparent Postgrex reconnects would hide the missing-notification window.
    {:ok, pid} = Notifications.start_link(options)
    ref = Notifications.listen!(pid, "chain_event")

    snapshot = snapshot()

    Registry.dispatch(Registry.ChainEvents, @reconciliation_key, fn entries ->
      for {subscriber, _} <- entries do
        send(subscriber, {:chain_event, @reconciliation_key, snapshot})
      end
    end)

    {:ok, {pid, ref, "chain_event"}}
  end

  @impl true
  def handle_call(:snapshot, _from, state), do: {:reply, snapshot(), state}

  @impl true
  def handle_info({:notification, pid, ref, topic, _payload} = notification, {pid, ref, topic} = state) do
    Listener.handle_info(notification, state)
  end

  def handle_info({:EXIT, pid, reason}, {pid, _ref, _topic} = state) do
    # Even a normally closed transport must repeat LISTEN and reconciliation.
    {:stop, {:notification_connection_closed, reason}, state}
  end

  @impl true
  def terminate(_reason, {pid, _ref, _topic}) do
    if Process.alive?(pid), do: GenServer.stop(pid)
    :ok
  end

  defp snapshot do
    head =
      Block
      |> where([block], block.consensus == true)
      |> order_by([block], desc: block.number)
      |> limit(1)
      |> select([block], %{number: block.number, hash: block.hash})
      |> Repo.one()

    %{head: head}
  end
end
