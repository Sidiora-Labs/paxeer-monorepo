defmodule Explorer.Market.Fetcher.History do
  @moduledoc """
  Fetches the daily market history.

  Market grabs the last 365 day's worth of market history for the configured
  coin in the explorer. Once that data is fetched, current day's values are
  checked every `MARKET_HISTORY_FETCH_INTERVAL` or every 60 minutes by default.
  Additionally, failed requests to the history source will follow exponential
  backoff `100ms * 2^(n+1)` where `n` is the number of failed requests.
  """

  use GenServer

  require Logger

  alias Explorer.History.Process, as: HistoryProcess
  alias Explorer.Market.{MarketHistory, Source}

  @types_to_default_state %{
    native_coin_price_history: %{
      max_failed_attempts: 10,
      source_function: :native_coin_price_history_source,
      fetch_function: :fetch_native_coin_price_history
    },
    secondary_coin_price_history: %{
      max_failed_attempts: 10,
      source_function: :secondary_coin_price_history_source,
      fetch_function: :fetch_secondary_coin_price_history
    },
    market_cap_history: %{
      max_failed_attempts: 3,
      source_function: :market_cap_history_source,
      fetch_function: :fetch_market_cap_history
    },
    tvl_history: %{
      max_failed_attempts: 3,
      source_function: :tvl_history_source,
      fetch_function: :fetch_tvl_history
    }
  }

  @types Map.keys(@types_to_default_state)

  @doc """
  Starts a process to continually fetch market history.
  """
  @spec start_link(term()) :: GenServer.on_start()
  def start_link(_) do
    GenServer.start_link(__MODULE__, :ok, name: __MODULE__)
  end

  @impl GenServer
  def init(:ok) do
    types_states =
      @types_to_default_state
      |> Map.new(fn {type, %{max_failed_attempts: max_failed_attempts, source_function: source_function}} ->
        {type,
         %{
           source: apply(Source, source_function, []),
           max_failed_attempts: max_failed_attempts,
           failed_attempts: 0,
           finished?: false,
           records: []
         }}
      end)

    if Enum.all?(types_states, fn {_type, type_state} -> is_nil(type_state.source) end) do
      Logger.info("No market history sources are configured")
      :ignore
    else
      state = %{types_states: types_states, tasks: %{}, cycle_finished?: true}

      send(self(), {:fetch_all, config(:first_fetch_day_count)})

      {:ok, state}
    end
  end

  @impl GenServer
  def handle_info({:fetch_all, day_count}, %{cycle_finished?: true} = state) do
    new_types_states =
      Map.new(state.types_states, fn {type, type_state} ->
        {type, %{type_state | failed_attempts: 0, finished?: false, records: []}}
      end)

    new_state = %{state | types_states: new_types_states, cycle_finished?: false} |> Map.put(:day_count, day_count)

    {:noreply, Enum.reduce(@types, new_state, &fetch/2)}
  end

  def handle_info({:fetch_all, _day_count}, state), do: {:noreply, state}

  @impl GenServer
  def handle_info(type, state) when type in @types do
    {:noreply, fetch(type, state)}
  end

  @impl GenServer
  def handle_info({ref, {type, result}}, state) when is_reference(ref) and type in @types do
    case Map.pop(state.tasks, ref) do
      {^type, tasks} ->
        Process.demonitor(ref, [:flush])
        handle_result(type, result, %{state | tasks: tasks})

      _ ->
        {:noreply, state}
    end
  end

  @impl GenServer
  def handle_info({:DOWN, ref, :process, _pid, reason}, state) do
    case Map.pop(state.tasks, ref) do
      {nil, _} -> {:noreply, state}
      {type, tasks} -> handle_result(type, {:error, reason}, %{state | tasks: tasks})
    end
  end

  defp handle_result(type, {:ok, records}, state) do
    new_types_states =
      state.types_states
      |> put_in([type, :records], records)
      |> put_in([type, :finished?], true)

    {:noreply, maybe_insert_and_schedule_refetch(%{state | types_states: new_types_states})}
  end

  defp handle_result(type, {:error, reason}, state) do
    Logger.error("Failed to fetch #{type}: #{inspect(reason)}")

    failed_attempts = get_in(state.types_states, [type, :failed_attempts]) + 1
    max_failed_attempts = get_in(state.types_states, [type, :max_failed_attempts])

    if failed_attempts <= max_failed_attempts do
      Process.send_after(self(), type, HistoryProcess.delay(failed_attempts))
    end

    new_types_states = put_in(state.types_states, [type, :failed_attempts], failed_attempts)
    {:noreply, maybe_insert_and_schedule_refetch(%{state | types_states: new_types_states})}
  end

  defp handle_result(type, :ignore, state) do
    Logger.info(
      "Selected source (#{inspect(get_in(state.types_states, [type, :source]))}) for #{type} is not implemented"
    )

    new_types_states =
      state.types_states
      |> put_in([type, :records], [])
      |> put_in([type, :source], nil)

    {:noreply, maybe_insert_and_schedule_refetch(%{state | types_states: new_types_states})}
  end

  @spec fetch(atom(), map()) :: map()
  defp fetch(type, state) do
    type_state = state.types_states[type]

    if type_state.source && !type_state.finished? &&
         type_state.failed_attempts <= type_state.max_failed_attempts && type not in Map.values(state.tasks) do
      task =
        Task.Supervisor.async_nolink(Explorer.MarketTaskSupervisor, fn ->
          {type, apply(type_state.source, @types_to_default_state[type].fetch_function, [state.day_count])}
        end)

      %{state | tasks: Map.put(state.tasks, task.ref, type)}
    else
      state
    end
  end

  defp maybe_insert_and_schedule_refetch(state) do
    if !state.cycle_finished? &&
         Enum.all?(state.types_states, fn {_type, type_state} ->
           is_nil(type_state.source) or type_state.finished? or
             type_state.failed_attempts > type_state.max_failed_attempts
         end) do
      state
      |> compile_records()
      |> MarketHistory.bulk_insert()

      Process.send_after(self(), {:fetch_all, 1}, config(:history_fetch_interval))
      %{state | cycle_finished?: true}
    else
      state
    end
  end

  defp config(key) do
    Application.get_env(:explorer, __MODULE__)[key]
  end

  defp compile_records(state) do
    @types
    |> Enum.map(fn type -> state.types_states[type].records end)
    |> Stream.concat()
    |> Enum.group_by(&{Map.get(&1, :date), Map.get(&1, :secondary_coin, false)})
    |> Enum.map(fn {_date, dates_with_data} ->
      Enum.reduce(dates_with_data, &Map.merge/2)
    end)
  end
end
