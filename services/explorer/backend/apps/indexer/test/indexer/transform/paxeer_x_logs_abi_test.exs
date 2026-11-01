defmodule Indexer.Transform.PaxeerXLogsAbiTest do
  use ExUnit.Case, async: true

  alias Indexer.Transform.PaxeerXLogs

  @precompile_names %{
    "0x0000000000000000000000000000000000001004" => "addr",
    "0x0000000000000000000000000000000000001015" => "layerxexchange",
    "0x0000000000000000000000000000000000001016" => "layerxbridge",
    "0x0000000000000000000000000000000000001017" => "launchpad"
  }
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

    topics_by_address =
      Map.new(@precompile_names, fn {address, name} ->
        abi_paths =
          if name == "launchpad" do
            ["launchpad/abi.json", "launchpad/legacy/v610/abi.json"]
          else
            ["#{name}/abi.json"]
          end

        topics =
          abi_paths
          |> Enum.flat_map(fn abi_path ->
            events =
              precompiles_dir
              |> Path.join(abi_path)
              |> File.read!()
              |> Jason.decode!()
              |> Enum.filter(&(Map.fetch!(&1, "type") == "event"))

            assert events != [], "#{abi_path} must declare events"

            Enum.map(events, fn event ->
              types = Enum.map_join(Map.fetch!(event, "inputs"), ",", &Map.fetch!(&1, "type"))
              signature = Map.fetch!(event, "name") <> "(" <> types <> ")"
              "0x" <> Base.encode16(ExKeccak.hash_256(signature), case: :lower)
            end)
          end)
          |> MapSet.new()

        assert MapSet.size(topics) > 0, "#{name} ABI must declare events"
        {address, topics}
      end)

    definitions = PaxeerXLogs.definitions()
    assert definitions != []

    for definition <- definitions do
      assert Map.has_key?(topics_by_address, definition.address),
             "#{definition.name} targets an unknown precompile: #{definition.address}"

      assert MapSet.member?(Map.fetch!(topics_by_address, definition.address), definition.topic),
             "#{definition.name} topic #{definition.topic} is absent from the " <>
               "#{Map.fetch!(@precompile_names, definition.address)} ABI"
    end
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
end
