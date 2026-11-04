defmodule Indexer.Fetcher.PaxeerX.PriceAcceptanceReplay do
  @moduledoc """
  Advances the durable acceptance replay on startup, retrying held jobs until complete.
  """

  use GenServer, restart: :transient

  require Logger

  alias Indexer.Fetcher.PaxeerX.PriceUpdates

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
       retry_interval: Keyword.get(options, :acceptance_backfill_retry_interval, 5_000)
     }}
  end

  @impl GenServer
  def handle_info(:replay, state) do
    case apply(state.module, :step, [state.worker, state.batch_size]) do
      :complete ->
        {:stop, :normal, state}

      {:ok, _count} ->
        Process.send_after(self(), :replay, state.interval)
        {:noreply, state}

      {:error, reason} ->
        Logger.warning("price acceptance replay held: #{inspect(reason)}")
        Process.send_after(self(), :replay, state.retry_interval)
        {:noreply, state}
    end
  end

  @impl GenServer
  def terminate(_reason, state) do
    if Process.alive?(state.worker), do: GenServer.stop(state.worker)
    :ok
  end
end
