defmodule Indexer.Fetcher.Blackfort.Validator do
  @moduledoc """
  GenServer responsible for updating the list of blackfort validators in the database.
  """
  use GenServer

  alias Explorer.Chain.Blackfort.Validator
  alias Explorer.Repo

  require Logger

  @max_retries 3
  @retry_interval :timer.seconds(1)

  def start_link(_) do
    GenServer.start_link(__MODULE__, [], name: __MODULE__)
  end

  @impl true
  def init(_) do
    GenServer.cast(self(), :update_validators_list)

    {:ok, %{retry_count: 0, retry_timer: nil}}
  end

  @impl true
  def handle_cast(:update_validators_list, state) do
    cancel_retry(state.retry_timer)
    update_validators_list(%{state | retry_count: 0, retry_timer: nil})
  end

  @impl true
  def handle_info({:retry_update_validators_list, token}, %{retry_timer: {_timer, token}} = state) do
    update_validators_list(%{state | retry_timer: nil})
  end

  def handle_info({:retry_update_validators_list, _token}, state), do: {:noreply, state}

  defp update_validators_list(state) do
    case fetch_and_replace_validators() do
      {:ok, _result} ->
        {:noreply, %{state | retry_count: 0, retry_timer: nil}}

      error ->
        Logger.error("Blackfort validator snapshot update failed: #{inspect(error)}")
        {:noreply, schedule_retry(state)}
    end
  end

  defp fetch_and_replace_validators do
    with {:ok, validators} <- Validator.fetch_validators_list() do
      Repo.transaction(fn ->
        validators_map = MapSet.new(validators, & &1.address_hash.bytes)

        address_hashes_to_drop_from_db =
          Validator.get_all_validators()
          |> Enum.reject(&MapSet.member?(validators_map, &1.address_hash.bytes))
          |> Enum.map(& &1.address_hash)

        Validator.delete_validators_by_address_hashes(address_hashes_to_drop_from_db)

        validators
        |> Enum.map(&Validator.append_timestamps/1)
        |> Validator.insert_validators()
      end)
    end
  rescue
    error -> {:error, Exception.format(:error, error, __STACKTRACE__)}
  end

  defp schedule_retry(%{retry_count: count} = state) when count < @max_retries do
    delay = @retry_interval * Integer.pow(2, count)
    token = make_ref()
    timer = Process.send_after(self(), {:retry_update_validators_list, token}, delay)
    Logger.warning("Retrying Blackfort validator snapshot update #{count + 1}/#{@max_retries} in #{delay}ms")
    %{state | retry_count: count + 1, retry_timer: {timer, token}}
  end

  defp schedule_retry(state) do
    Logger.error("Blackfort validator snapshot update exhausted #{@max_retries} retries")
    state
  end

  defp cancel_retry(nil), do: :ok
  defp cancel_retry({timer, _token}), do: Process.cancel_timer(timer)

  @spec trigger_update_validators_list() :: :ok
  def trigger_update_validators_list do
    GenServer.cast(__MODULE__, :update_validators_list)
  end
end
