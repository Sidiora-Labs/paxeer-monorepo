defmodule Explorer.Repair.Repair2233511Test do
  use ExUnit.Case, async: false

  alias EthereumJSONRPC.Block
  alias Explorer.Chain.{Address, Hash}
  alias Explorer.EnvVarTranslator
  alias Explorer.Repo
  alias Explorer.Tags.{AddressTag, AddressToTag}
  alias Explorer.Tags.AddressTag.Cataloger

  @recording Path.expand("../../../ethereum_jsonrpc/test/support/fixture/paxeer_x/consecutive_blocks.json", __DIR__)

  setup do
    original = Application.fetch_env(:block_scout_web, :new_tags)
    owner = Ecto.Adapters.SQL.Sandbox.start_owner!(Repo, shared: true)

    on_exit(fn ->
      Ecto.Adapters.SQL.Sandbox.stop_owner(owner)

      case original do
        {:ok, value} -> Application.put_env(:block_scout_web, :new_tags, value)
        :error -> Application.delete_env(:block_scout_web, :new_tags)
      end
    end)

    :ok
  end

  test "tag-config valid configuration reaches cataloger storage and recorded address consumer" do
    config = ~s([{"tag":"repair miner","title":"Recorded Paxeer miner"},{"tag":"repair unicode","title":"Vérifié"}])
    Application.put_env(:block_scout_web, :new_tags, config)
    tags = EnvVarTranslator.map_array_env_var_to_list(:new_tags)

    assert tags == [
             %{tag: "repair miner", title: "Recorded Paxeer miner"},
             %{tag: "repair unicode", title: "Vérifié"}
           ]

    # Drive the production initialization and the message it emits, stopping
    # before the independent validator RPC/binding phase of the cataloger.
    assert {:ok, state} = Cataloger.init(tags)
    assert_receive fetch_message
    assert fetch_message == :fetch_tags

    # Startup owns a validated snapshot, even if application config changes.
    Application.put_env(:block_scout_web, :new_tags, "malformed after initialization")
    assert {:noreply, ^state} = Cataloger.handle_info(fetch_message, state)
    assert_receive :bind_addresses

    for %{tag: label, title: title} <- tags do
      assert %AddressTag{display_name: ^title} = Repo.get_by!(AddressTag, label: label)
    end

    recorded_block = @recording |> File.read!() |> Jason.decode!() |> Map.fetch!("0x1714efd") |> Block.to_elixir()
    {:ok, miner_hash} = Hash.Address.cast(recorded_block["miner"])
    Repo.insert!(%Address{hash: miner_hash})
    tag_id = AddressTag.get_id_by_label("repair miner")
    assert {1, nil} = AddressToTag.set_tag_to_addresses(tag_id, [recorded_block["miner"]])
    assert %AddressToTag{address_hash: ^miner_hash} = Repo.get_by!(AddressToTag, tag_id: tag_id)

    Application.put_env(:block_scout_web, :new_tags, ~s([{"tag":"repair miner","title":"Updated miner"}]))
    updated_tags = EnvVarTranslator.map_array_env_var_to_list(:new_tags)
    assert {:ok, updated_state} = Cataloger.init(updated_tags)
    assert_receive updated_message
    assert updated_message == :fetch_tags
    assert {:noreply, ^updated_state} = Cataloger.handle_info(updated_message, updated_state)
    assert_receive :bind_addresses
    assert %AddressTag{id: ^tag_id, display_name: "Updated miner"} = Repo.get_by!(AddressTag, label: "repair miner")
  end

  test "tag-config absent setting and explicit empty list are empty successful cataloger inputs" do
    original_count = Repo.aggregate(AddressTag, :count)

    for config <- [nil, "[]"] do
      Application.put_env(:block_scout_web, :new_tags, config)
      assert [] = tags = EnvVarTranslator.map_array_env_var_to_list(:new_tags)
      assert {:ok, state} = Cataloger.init(tags)
      assert_receive fetch_message
      assert fetch_message == :fetch_tags
      assert {:noreply, ^state} = Cataloger.handle_info(fetch_message, state)
      assert_receive :bind_addresses
      assert Repo.aggregate(AddressTag, :count) == original_count
    end
  end

  test "tag-config objects and scalars fail explicitly before cataloger startup" do
    for config <- [~s({"tag":"repair","title":"Repair"}), "{}", "null", "true", "false", "42", "3.5", ~s("tags")] do
      assert_rejected_before_start(config, "NEW_TAGS must be a JSON list of tag/title records")
    end
  end

  test "tag-config every entry must have exactly two non-empty string fields" do
    invalid_entries = [
      %{},
      %{"tag" => "repair"},
      %{"title" => "Repair"},
      %{"tag" => nil, "title" => "Repair"},
      %{"tag" => "repair", "title" => nil},
      %{"tag" => 1, "title" => "Repair"},
      %{"tag" => "repair", "title" => false},
      %{"tag" => [], "title" => "Repair"},
      %{"tag" => "repair", "title" => %{}},
      [],
      "repair",
      nil,
      1
    ]

    # A valid first entry must never be partially imported before a bad entry.
    for entry <- invalid_entries do
      config = Jason.encode!([%{"tag" => "repair must not persist", "title" => "Valid first entry"}, entry])
      assert_rejected_before_start(config, "NEW_TAGS[1] must contain exactly tag and title as string fields")
    end

    for {tag, title} <- [{"", "Repair"}, {" \t\n", "Repair"}, {"repair", ""}, {"repair", " \r\n"}] do
      config = Jason.encode!([%{"tag" => tag, "title" => title}])
      assert_rejected_before_start(config, "NEW_TAGS[0] tag and title must be non-empty strings")
    end
  end

  test "tag-config unknown string keys are rejected without creating atoms" do
    unknown_key = "__repair_22_33_511_unknown_config_key__"
    assert_raise ArgumentError, fn -> String.to_existing_atom(unknown_key) end

    for key <- [unknown_key, "Tag", "extra"] do
      config = Jason.encode!([%{"tag" => "repair", "title" => "Repair", key => "unexpected"}])
      assert_rejected_before_start(config, "NEW_TAGS[0] must contain exactly tag and title as string fields")
    end

    assert_raise ArgumentError, fn -> String.to_existing_atom(unknown_key) end
  end

  test "tag-config malformed JSON and non-string configuration never silently erase tags" do
    for config <- ["", " ", "[", ~s([{"tag":"repair","title":"Repair"}), "[] trailing", "[{} ,]", <<255>>] do
      assert_rejected_before_start(config, "NEW_TAGS contains malformed JSON")
    end

    for config <- [false, true, 1, [], [%{tag: "repair", title: "Repair"}]] do
      assert_rejected_before_start(config, "NEW_TAGS must be a JSON string containing a list of tag/title records")
    end
  end

  defp assert_rejected_before_start(config, message) do
    {:ok, existing} = AddressTag.set("repair existing", "Must survive invalid configuration")
    count = Repo.aggregate(AddressTag, :count)
    refute Process.whereis(Cataloger)
    Application.put_env(:block_scout_web, :new_tags, config)

    assert_raise ArgumentError, message, fn -> EnvVarTranslator.map_array_env_var_to_list(:new_tags) end
    assert_raise ArgumentError, message, fn -> Cataloger.start_link([]) end

    refute Process.whereis(Cataloger)
    refute_receive :fetch_tags, 0
    assert Repo.aggregate(AddressTag, :count) == count
    assert Repo.get_by!(AddressTag, id: existing.id) == existing
  end
end
