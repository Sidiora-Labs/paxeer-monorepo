Code.require_file("../explorer/migrator/empty_bytecode_for_selfdestructed_smart_contracts_test.exs", __DIR__)

defmodule Explorer.Repair.Repair2231410Test do
  use Explorer.DataCase, async: false

  import Explorer.Migrator.SelfdestructChainData

  for fork <- ~w(shanghai cancun), creation <- ~w(create create2) do
    @recording "#{fork}_same_transaction_#{creation}"

    test "selfdestruct-rule: #{@recording} agrees with committed eth_getCode" do
      recording = import_recording(@recording)
      assert recording["at_destruction"]["response"]["result"] == "0x"
      assert recording["current"]["response"]["result"] == "0x"

      traces =
        Explorer.Repo.all(
          from(it in Explorer.Chain.InternalTransaction,
            where: it.created_contract_address_hash == ^recording["stored_address_hash"],
            select: {it.type, it.transaction_hash}
          )
        )

      assert [{type, transaction_hash}] = traces
      assert type in [:create, :create2]

      assert Explorer.Repo.exists?(
               from(it in Explorer.Chain.InternalTransaction,
                 where: it.transaction_hash == ^transaction_hash,
                 where: it.type == :selfdestruct,
                 where: it.from_address_hash == ^recording["stored_address_hash"]
               )
             )

      assert_committed_code(recording)
    end
  end

  test "selfdestruct-rule: historical destruction must not erase a redeployed CREATE2 contract" do
    recording = import_recording("shanghai_redeployed")
    assert recording["at_destruction"]["response"]["result"] == "0x"
    assert recording["current"]["response"]["result"] == recording["cached_code"]

    assert quantity(recording["current"]["request"]["params"] |> List.last()) >
             quantity(recording["destruction_block"])

    assert_committed_code(recording)
  end

  test "selfdestruct-rule: the same opcode deletes pre-Cancun code and preserves post-Cancun code" do
    history = import_recording("geth_history")
    historical = contract_recording(history, "third")
    current = contract_recording(history, "survivor")
    activation = history["chain_spec"]["config"]["cancunTime"]

    for {recording, comparison} <- [{historical, :lt}, {current, :gt}] do
      block =
        Enum.find(history["records"], &(&1["block"]["response"]["result"]["number"] == recording["destruction_block"]))

      timestamp = quantity(block["block"]["response"]["result"]["timestamp"])
      assert if(timestamp < activation, do: :lt, else: :gt) == comparison
    end

    assert historical["before"]["response"]["result"] == historical["cached_code"]
    assert historical["current"]["response"]["result"] == "0x"
    assert current["before"]["response"]["result"] == current["cached_code"]
    assert current["current"]["response"]["result"] == current["cached_code"]

    assert_committed_code(historical)
    assert_committed_code(current)
  end

  test "selfdestruct-rule: unavailable canonical state aborts the batch without clearing code" do
    history = import_recording("geth_history")
    first = contract_recording(history, "first")
    second = contract_recording(history, "second")
    head = List.last(history["records"])["block"]["response"]["result"]
    missing_block = hd(history["missing_canonical"])["request"]["params"] |> List.last()

    insert(:block,
      hash: missing_block["blockHash"],
      number: quantity(head["number"]) + 1,
      parent_hash: head["hash"],
      timestamp: DateTime.from_unix!(quantity(head["timestamp"]) + 1)
    )

    assert_raise RuntimeError, ~r/Cannot reconcile SELFDESTRUCT bytecode/, fn ->
      Explorer.Migrator.EmptyBytecodeForSelfdestructedSmartContracts.update_batch([
        quantity(first["destruction_block"])
      ])
    end

    for recording <- [first, second] do
      assert Repo.get!(Explorer.Chain.Address, recording["stored_address_hash"]).contract_code ==
               contract_data(recording["cached_code"])
    end
  end

  test "selfdestruct-rule: a noncanonical destruction cannot clear current code" do
    recording = import_recording("shanghai_existing")
    destruction = quantity(recording["destruction_block"])

    Repo.update_all(
      from(block in Explorer.Chain.Block, where: block.number == ^destruction),
      set: [consensus: false]
    )

    assert {:ok, []} =
             Explorer.Migrator.EmptyBytecodeForSelfdestructedSmartContracts.update_batch([destruction])

    assert Repo.get!(Explorer.Chain.Address, recording["stored_address_hash"]).contract_code ==
             contract_data(recording["cached_code"])
  end

  test "selfdestruct-rule: the migration reaches canonical reconciliation before updating code" do
    recording = import_recording("shanghai_existing")
    address = recording["address"]
    head_hash = List.last(recording["records"])["block"]["response"]["result"]["hash"]

    assert {:ok, 1} =
             Explorer.Migrator.EmptyBytecodeForSelfdestructedSmartContracts.update_batch([
               quantity(recording["destruction_block"])
             ])

    assert_received {:recorded_rpc_request, "eth_getCode",
                     [^address, %{"blockHash" => ^head_hash, "requireCanonical" => true}]}

    assert_committed_code(recording)
  end

  test "selfdestruct-rule: the chain spec timestamp selects the same mixed-fork history" do
    history = import_recording("geth_history")
    historical = contract_recording(history, "third")
    current = contract_recording(history, "survivor")
    migrator = Explorer.Migrator.EmptyBytecodeForSelfdestructedSmartContracts
    config = Application.fetch_env!(:explorer, migrator)
    Application.put_env(:explorer, migrator, Keyword.delete(config, :cancun_block))

    assert {:ok, 1} =
             migrator.update_batch(Enum.map([historical, current], &quantity(&1["destruction_block"])))

    assert_committed_code(historical)
    assert_committed_code(current)
  end

  test "selfdestruct-rule: without a fork schedule the migration refuses to run and preserves committed code" do
    recording = import_recording("cancun_existing")
    migrator = Explorer.Migrator.EmptyBytecodeForSelfdestructedSmartContracts
    config = Application.fetch_env!(:explorer, Explorer.ChainSpec.GenesisData)
    assert {:ok, %{status_code: 200}} = Explorer.HttpClient.get(config[:chain_spec_path])
    Application.put_env(:explorer, Explorer.ChainSpec.GenesisData, Keyword.put(config, :chain_spec_path, nil))
    migration_config = Application.fetch_env!(:explorer, migrator)
    Application.put_env(:explorer, migrator, Keyword.delete(migration_config, :cancun_block))

    assert_raise RuntimeError, ~r/Missing SELFDESTRUCT fork schedule/, fn ->
      migrator.update_batch([quantity(recording["destruction_block"])])
    end

    assert Repo.get!(Explorer.Chain.Address, recording["stored_address_hash"]).contract_code ==
             contract_data(recording["current"]["response"]["result"])

    Application.put_env(:explorer, migrator, migration_config)
    assert_committed_code(recording)
  end
end
