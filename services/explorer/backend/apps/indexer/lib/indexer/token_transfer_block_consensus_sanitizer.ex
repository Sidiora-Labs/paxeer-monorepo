defmodule Indexer.TokenTransferBlockConsensusSanitizer do
  @moduledoc """
  Periodically find token transfers with incorrect block_consensus and set refetch_needed for their blocks.
  """

  use GenServer

  require Logger

  import Ecto.Query

  alias Explorer.Chain.{Block, TokenTransfer}
  alias Explorer.Repo

  @batch_size 100
  @query_timeout 15_000

  def child_spec(_) do
    %{
      id: __MODULE__,
      start: {__MODULE__, :start_link, []},
      type: :worker,
      restart: :permanent,
      shutdown: Application.get_env(:indexer, :graceful_shutdown_period)
    }
  end

  def start_link do
    GenServer.start_link(__MODULE__, :ok, name: __MODULE__)
  end

  def init(_) do
    schedule_sanitize()

    {:ok, %{cursor: -1}}
  end

  def handle_info(:sanitize, %{cursor: cursor} = state) do
    block_numbers =
      TokenTransfer
      |> join(:inner, [tt], b in assoc(tt, :block))
      |> where([tt, b], tt.block_consensus != b.consensus)
      |> where([tt, b], b.number > ^cursor and (b.refetch_needed == false or is_nil(b.refetch_needed)))
      |> select([tt, b], b.number)
      |> distinct(true)
      |> order_by([tt, b], asc: b.number)
      |> limit(^@batch_size)
      |> Repo.all(timeout: @query_timeout)

    case block_numbers do
      [] ->
        Logger.debug("[TokenTransferBlockConsensusSanitizer] No inconsistent token transfer block consensus found")

        schedule_sanitize()
        {:noreply, %{state | cursor: -1}}

      numbers ->
        Logger.info("[TokenTransferBlockConsensusSanitizer] Marking #{length(numbers)} blocks for refetch")

        {:ok, :ok} =
          Repo.transaction(
            fn ->
              Repo.query!("SET LOCAL statement_timeout = '15s'", [], timeout: @query_timeout)
              Block.set_refetch_needed(numbers)
            end,
            timeout: @query_timeout
          )

        send(self(), :sanitize)
        {:noreply, %{state | cursor: List.last(numbers)}}
    end
  end

  defp schedule_sanitize do
    Process.send_after(self(), :sanitize, Application.get_env(:indexer, __MODULE__)[:interval])
  end
end
