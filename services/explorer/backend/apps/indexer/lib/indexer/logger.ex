defmodule Indexer.Logger do
  @moduledoc """
  Helper for `Logger`.
  """

  @doc """
  Sets `keyword` in `Logger.metadata/1` around `fun`.
  """
  def metadata(fun, keyword) when is_function(fun, 0) and is_list(keyword) do
    metadata_before = Logger.metadata()

    try do
      Logger.metadata(keyword)
      fun.()
    after
      Logger.reset_metadata(metadata_before)
    end
  end

  @doc """
  The PID and its registered name (if it has one) as `t:iodata/0`.
  """
  def process(pid) when is_pid(pid) do
    prefix = [inspect(pid)]

    case Process.info(pid, :registered_name) do
      nil -> prefix
      {:registered_name, []} -> prefix
      {:registered_name, registered_name} -> [prefix, " (", inspect(registered_name), ")"]
    end
  end
end
