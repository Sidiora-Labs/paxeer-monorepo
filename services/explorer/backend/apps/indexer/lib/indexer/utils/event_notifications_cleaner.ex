defmodule Indexer.Utils.EventNotificationsCleaner do
  @moduledoc """
  Expires transport payloads for disposable, best-effort chain notifications.

  `max_age` is a payload retention bound, not a delivery or replay guarantee.
  API consumers must reconcile authoritative state after startup or reconnect;
  see `Indexer.Utils.EventNotificationsRecovery`. Deleting a payload never
  deletes the authoritative chain data in `Explorer.Repo`.
  """

  alias Explorer.Repo.EventNotifications, as: EventNotificationsRepo
  alias Explorer.Utility.EventNotification

  import Ecto.Query

  use GenServer

  require Logger

  def start_link(args) do
    GenServer.start_link(__MODULE__, args, name: __MODULE__)
  end

  def init(args) do
    Process.send(self(), :clean_up_event_notifications, [])
    {:ok, args}
  end

  def handle_info(:clean_up_event_notifications, state) do
    clean_up_event_notifications()
    Process.send_after(self(), :clean_up_event_notifications, interval())
    {:noreply, state}
  end

  defp clean_up_event_notifications do
    {count, _} =
      EventNotification
      |> where([en], en.inserted_at < ago(^max_age(), "millisecond"))
      |> EventNotificationsRepo.delete_all()

    Logger.info("Deleted #{count} event notifications")
  end

  defp max_age do
    config()[:max_age]
  end

  defp interval do
    config()[:interval]
  end

  defp config do
    Application.get_env(:indexer, __MODULE__)
  end
end
