defmodule Indexer.Fetcher.PaxeerX.PriceAcceptanceReplay do
  @moduledoc """
  Advances the durable acceptance replay on startup, retrying held and deferred jobs until complete.
  """

  use GenServer, restart: :transient

  require Logger

  alias Indexer.Fetcher.PaxeerX.PriceUpdates

  @held_warning_interval 60_000

  def start_link(options), do: GenServer.start_link(__MODULE__, options)

  @impl GenServer
  def init(options) do
    module = PriceUpdates.load_acceptance_backfill()
    {:ok, worker} = apply(module, :start_link, [options])
    send(self(), :replay)

    {:ok,
     %{
       module: module,
       worker: worker,
       batch_size: Keyword.get(options, :acceptance_backfill_batch_size, 500),
       interval: Keyword.get(options, :acceptance_backfill_interval, 100),
       retry_interval: Keyword.get(options, :acceptance_backfill_retry_interval, 5_000),
       last_held_warning_at: nil
     }}
  end

  @impl GenServer
  def handle_info(:replay, state) do
    result = apply(state.module, :step, [state.worker, state.batch_size])
    state = log_held_jobs(state)

    case result do
      :complete ->
        {:stop, :normal, state}

      {:ok, _count} ->
        Process.send_after(self(), :replay, state.interval)
        {:noreply, state}

      :deferred ->
        Process.send_after(self(), :replay, state.retry_interval)
        {:noreply, state}

      {:error, _reason} ->
        Process.send_after(self(), :replay, state.retry_interval)
        {:noreply, state}
    end
  end

  defp log_held_jobs(state) do
    now = System.monotonic_time(:millisecond)

    if is_nil(state.last_held_warning_at) or now - state.last_held_warning_at >= @held_warning_interval do
      held_jobs =
        try do
          {:ok, apply(state.module, :held_jobs, [])}
        rescue
          error in [Postgrex.Error, DBConnection.ConnectionError] -> {:error, error}
        end

      case held_jobs do
        {:ok, {count, number, error}} ->
          Logger.warning("price acceptance replay held: #{count} rows; lowest block #{number}: #{error}")
          %{state | last_held_warning_at: now}

        {:ok, nil} ->
          state

        {:error, error} ->
          Logger.warning("price acceptance replay held-job diagnostic unavailable: #{Exception.message(error)}")
          state
      end
    else
      state
    end
  end

  @impl GenServer
  def terminate(_reason, state) do
    if Process.alive?(state.worker), do: GenServer.stop(state.worker)
    :ok
  end
end
