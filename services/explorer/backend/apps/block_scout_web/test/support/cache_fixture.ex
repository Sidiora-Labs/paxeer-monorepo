defmodule BlockScoutWeb.CacheFixture do
  @moduledoc """
  Serializes the lifetime of fixtures using Explorer's globally named caches.

  OrderedCache consumers use fixed names, so per-test processes cannot be routed
  without changing production code. Both connection and browser cases acquire
  the same local global lock before checkout/reset and retain it through all
  test cleanup callbacks, including cleanup after a test process crashes.
  """

  import ExUnit.Assertions, only: [flunk: 1]

  alias Explorer.Chain.Cache.{Accounts, Transactions}

  def checkout!(cleanup? \\ false)

  def checkout!(false) do
    true = :global.set_lock({__MODULE__, self()}, [node()], :infinity)
    prepare!()
  end

  def checkout!(true) do
    owner = self()
    ref = make_ref()

    lock =
      spawn(fn ->
        true = :global.set_lock({__MODULE__, self()}, [node()], :infinity)
        send(owner, {ref, :locked})

        receive do
          {^ref, :release, caller} ->
            true = :global.del_lock({__MODULE__, self()}, [node()])
            send(caller, {ref, :released})
        end
      end)

    ExUnit.Callbacks.on_exit(fn ->
      send(lock, {ref, :release, self()})

      receive do
        {^ref, :released} -> :ok
      end
    end)

    receive do
      {^ref, :locked} -> :ok
    end

    prepare!()
  end

  defp prepare! do
    true = :ets.delete_all_objects(BlockScoutWeb.RateLimit.Hammer.ETS)
    true = :ets.delete_all_objects(Explorer.Utility.Hammer.ETS)

    if :ets.whereis(:rate_limiter) != :undefined do
      true = :ets.delete_all_objects(:rate_limiter)
    end

    reset!(Explorer.Supervisor)
  end

  @doc false
  def reset!(supervisor) do
    Enum.each([Transactions, Accounts], fn cache ->
      id = cache.child_id()

      case Supervisor.terminate_child(supervisor, id) do
        :ok -> :ok
        error -> flunk("Cannot terminate cache #{inspect(id)}: #{inspect(error)}")
      end

      pid =
        case Supervisor.restart_child(supervisor, id) do
          {:ok, pid} when is_pid(pid) -> pid
          {:ok, pid, _info} when is_pid(pid) -> pid
          error -> flunk("Cannot restart cache #{inspect(id)}: #{inspect(error)}")
        end

      unless Process.alive?(pid) and Process.whereis(cache.cache_name()) == pid do
        flunk("Restarted cache #{inspect(id)} is not running under its production name")
      end
    end)

    :ok
  end
end
