defmodule Indexer.Transform.PaxeerXLogsAbiTest do
  use ExUnit.Case, async: true

  alias Indexer.Transform.PaxeerXLogs

  @precompile_abis %{
    "0x0000000000000000000000000000000000001004" => ["addr/abi.json"],
    "0x0000000000000000000000000000000000001015" => ["perps/abi/abi.json", "layerxexchange/abi.json"],
    "0x0000000000000000000000000000000000001016" => ["layerxbridge/abi.json"],
    "0x0000000000000000000000000000000000001017" => ["launchpad/abi.json", "launchpad/legacy/v610/abi.json"]
  }
  @required_perps_events ~w(CollateralDeposited CollateralWithdrawn CollateralTransferred OrderPlaced OrderCancelled
                            OrderExecuted PositionOpened PositionClosed Liquidation TradeSettled)
  @retired_addresses [
    "0x0000000000000000000000000000000000001013",
    "0x0000000000000000000000000000000000001014"
  ]

  test "kept_topics_match_precompile_abis" do
    kept_topics_match_precompile_abis()
  end

  def kept_topics_match_precompile_abis do
    precompiles_dir =
      System.get_env("PAXEER_PRECOMPILES_DIR") || Path.expand("../../../../../../../../precompiles", __DIR__)

    definitions = PaxeerXLogs.definitions()
    assert definitions != []

    required_keys =
      for {address, paths} <- @precompile_abis,
          path <- paths,
          reduce: MapSet.new() do
        keys ->
          events =
            precompiles_dir
            |> Path.join(path)
            |> File.read!()
            |> Jason.decode!()
            |> Enum.filter(&(Map.fetch!(&1, "type") == "event"))

          assert events != [], "#{path} must declare events"

          required =
            if path == "perps/abi/abi.json" do
              selected = Enum.filter(events, &(&1["name"] in @required_perps_events))
              assert Enum.sort(Enum.map(selected, & &1["name"])) == Enum.sort(@required_perps_events)
              selected
            else
              events
            end

          Enum.reduce(required, keys, fn event, acc ->
            inputs = Map.fetch!(event, "inputs")
            signature = event["name"] <> "(" <> Enum.map_join(inputs, ",", & &1["type"]) <> ")"
            topic = "0x" <> Base.encode16(ExKeccak.hash_256(signature), case: :lower)
            matches = Enum.filter(definitions, &(&1.address == address and &1.signature == signature))
            assert [definition] = matches, "#{path}: #{signature} must have exactly one decoder"
            assert definition.topic == topic

            layout = Enum.map(definition.arguments, fn {name, type, indexed} -> {name, abi_type(type), indexed} end)

            assert layout == Enum.map(inputs, &{&1["name"], &1["type"], &1["indexed"]}),
                   "#{path}: #{signature} must preserve argument names, types, order and indexed layout"

            MapSet.put(acc, {address, topic})
          end)
      end

    assert MapSet.new(definitions, &{&1.address, &1.topic}) == required_keys
  end

  test "no_definition_targets_retired_address" do
    no_definition_targets_retired_address()
  end

  def no_definition_targets_retired_address do
    definitions = PaxeerXLogs.definitions()
    assert definitions != []

    for definition <- definitions do
      refute String.downcase(definition.address) in @retired_addresses,
             "#{definition.name} targets retired precompile #{definition.address}"
    end
  end

  defp abi_type(:address), do: "address"
  defp abi_type(:bool), do: "bool"
  defp abi_type(:string), do: "string"
  defp abi_type({:bytes, size}), do: "bytes#{size}"
  defp abi_type({:uint, bits}), do: "uint#{bits}"
  defp abi_type({:int, bits}), do: "int#{bits}"
end
