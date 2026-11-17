defmodule Backend.TestSuite do
  @moduledoc false

  def stop_background_pollers do
    for {supervisor, pollers} <- [
          {Explorer.Supervisor,
           [
             Explorer.Chain.Health.Monitor,
             Explorer.Chain.PaxeerX.Capabilities,
             Explorer.Chain.Cache.MinMissingBlockNumber,
             Explorer.ChainSpec.GenesisData,
             Explorer.Utility.ReplicaAccessibilityManager
           ]},
          {EthereumJSONRPC.Supervisor, [EthereumJSONRPC.Utility.EndpointAvailabilityChecker]}
        ],
        Process.whereis(supervisor) != nil,
        {id, _pid, _type, _modules} <- Supervisor.which_children(supervisor),
        id in pollers do
      :ok = Supervisor.terminate_child(supervisor, id)
    end

    :ok
  end

  def restore_configuration_after_suite do
    configuration = Keyword.take(ExUnit.configuration(), [:formatters, :max_cases, :after_suite])

    settings =
      for {application, key} <- [{:junit_formatter, :report_dir}, {:wallaby, :base_url}] do
        {application, key, Application.fetch_env(application, key)}
      end

    ExUnit.after_suite(fn _result ->
      ExUnit.configure(configuration)

      for {application, key, setting} <- settings do
        case setting do
          {:ok, value} -> Application.put_env(application, key, value)
          :error -> Application.delete_env(application, key)
        end
      end
    end)
  end
end
