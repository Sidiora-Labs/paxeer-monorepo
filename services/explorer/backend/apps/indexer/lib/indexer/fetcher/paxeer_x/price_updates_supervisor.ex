defmodule Indexer.Fetcher.PaxeerX.PriceUpdates.Supervisor do
  use Supervisor

  alias Indexer.Fetcher.PaxeerX.{PriceAcceptanceReplay, PriceUpdates}

  def child_spec([]), do: child_spec([[], []])
  def child_spec([arguments]), do: child_spec([arguments, []])

  def child_spec([_arguments, _options] = arguments) do
    %{id: __MODULE__, start: {__MODULE__, :start_link, arguments}, restart: :permanent, type: :supervisor}
  end

  def start_link(arguments, options \\ []) do
    if disabled?() do
      :ignore
    else
      Supervisor.start_link(__MODULE__, arguments, Keyword.put_new(options, :name, __MODULE__))
    end
  end

  def disabled?, do: Application.get_env(:indexer, __MODULE__, [])[:disabled?] == true

  @impl Supervisor
  def init(arguments) do
    config = Application.get_env(:indexer, PriceUpdates, [])

    children = [
      {Task.Supervisor, name: PriceUpdates.TaskSupervisor},
      {PriceAcceptanceReplay, config},
      {PriceUpdates, [Keyword.put(arguments, :supervisor, self()), [name: PriceUpdates]]}
    ]

    Supervisor.init(children, strategy: :one_for_one)
  end
end
