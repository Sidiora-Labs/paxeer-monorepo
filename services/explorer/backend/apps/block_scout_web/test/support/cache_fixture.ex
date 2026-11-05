defmodule BlockScoutWeb.CacheFixture do
  @moduledoc """
  Serializes the lifetime of fixtures using Explorer's globally named caches.

  OrderedCache consumers use fixed names, so per-test processes cannot be routed
  without changing production code. Both connection and browser cases acquire
  the same local global lock before checkout/reset and retain it until the test
  process exits. OTP releases the lock even when the test crashes.
  """

  import ExUnit.Assertions, only: [flunk: 1]

  alias Explorer.Chain.Cache.{Accounts, Transactions}

  def checkout! do
    true = :global.set_lock({__MODULE__, self()}, [node()], :infinity)
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
