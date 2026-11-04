defmodule Indexer.Fetcher.OnDemand.TestEnvironment do
  @moduledoc false

  def preserve(application, key) do
    previous = Application.fetch_env(application, key)

    ExUnit.Callbacks.on_exit(fn ->
      case previous do
        {:ok, value} -> Application.put_env(application, key, value)
        :error -> Application.delete_env(application, key)
      end
    end)
  end
end
