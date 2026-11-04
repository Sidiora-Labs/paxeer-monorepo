defmodule Explorer.Repair.Repair2233334Test do
  @moduledoc """
  Task 22.33.334, audit 22.30.137: solidity_optional_optimizer is proved by
  "omitted optimizer publishes compiler defaults" and
  "disabled optimizer without runs publishes compiler defaults".
  The remaining cases cover omitted enabled, empty optimizer, explicit disabled
  settings, malformed settings and rejection of mismatched chain bytecode.

  The committed SimpleStorage compiler/chain recording in Explorer.Factory is
  replayed through the production standard-JSON candidate consumer, bytecode
  comparison, Solidity publisher and persisted SmartContract schema. The local
  compiler entry point uses this same consumer after CodeCompiler.run/2; the
  replay needs no compiler download, subprocess or live endpoint.

  Baseline: no earlier hf-build/hf-done ledger or scoped changes were present.
  No unchanged-tree verification was run: the ordered gate instructions require
  the entire implementation before formatting, hf-build and hf-done. There is
  no claim that any baseline test ran or failed. The runner records the exact
  verified revision and private logs. No out-of-path wiring changes are needed.

  Qualification attempt 1 at cefb7952fe901f23529de02401e8c05be8c90a41:
  cg spec done 22.33.334 -f layerx-merge --root /work/paxeer-monorepo
  exited 1 (verify command exited 2), evidence /out/logs/verify-1.log.
  Gotcha: SmartContract uses an integer id primary key; address-based record
  assertions must use get_by(address_hash: hash), retaining all assertions.
  """

  use ExUnit.Case, async: false

  import Explorer.Factory

  alias Ecto.Adapters.SQL.Sandbox
  alias Explorer.Chain.{Address, SmartContract}
  alias Explorer.Repo
  alias Explorer.SmartContract.Solidity.{Publisher, Verifier}

  setup do
    :ok = Sandbox.checkout(Repo)
    recording = contract_code_info()
    address = insert(:contract_address, contract_code: recording.bytecode)

    :transaction
    |> insert(created_contract_address_hash: address.hash, input: recording.tx_input)
    |> with_block(status: :ok)

    candidate = %{
      "abi" => recording.abi,
      "bytecode" => String.replace_prefix(recording.tx_input, "0x", ""),
      "deployedBytecode" => String.replace_prefix(recording.bytecode, "0x", ""),
      "name" => recording.name,
      "file_path" => "SimpleStorage.sol"
    }

    input = %{
      "language" => "Solidity",
      "sources" => %{"SimpleStorage.sol" => %{"content" => recording.source_code}},
      "settings" => %{}
    }

    params = %{"name" => recording.name, "compiler_version" => recording.version}
    %{address: address, recording: recording, candidate: candidate, input: input, params: params}
  end

  test "omitted optimizer publishes compiler defaults", context do
    assert_published(context, %{})
  end

  test "disabled optimizer without runs publishes compiler defaults", context do
    assert_published(context, %{"optimizer" => %{"enabled" => false}})
  end

  test "empty optimizer publishes compiler defaults", context do
    assert_published(context, %{"optimizer" => %{}})
  end

  test "omitted enabled defaults to disabled", context do
    assert_published(context, %{"optimizer" => %{"runs" => 200}})
  end

  test "explicit disabled settings remain in compiler metadata", context do
    assert_published(context, %{"optimizer" => %{"enabled" => false, "runs" => 200}})
  end

  test "invalid optimizer settings return a typed JSON error without publishing", context do
    for optimizer <- [
          nil,
          false,
          [],
          "false",
          %{"enabled" => nil},
          %{"enabled" => "false"},
          %{"enabled" => 0},
          %{"enabled" => true, "runs" => nil},
          %{"enabled" => false, "runs" => -1},
          %{"enabled" => true, "runs" => 1.5},
          %{"enabled" => true, "runs" => "200"}
        ] do
      input = Map.put(context.input, "settings", %{"optimizer" => optimizer})

      assert {:error, :json} =
               Verifier.verify_standard_json_candidates(
                 context.address.hash,
                 context.params,
                 Jason.encode!(input),
                 [context.candidate]
               )
    end

    assert Repo.get_by(SmartContract, address_hash: context.address.hash) == nil
    refute Repo.get!(Address, context.address.hash).verified
  end

  test "invalid settings containers and JSON return typed errors", context do
    inputs = [
      "{",
      "null",
      "[]",
      Jason.encode!(Map.put(context.input, "settings", nil)),
      Jason.encode!(Map.put(context.input, "settings", [])),
      Jason.encode!(Map.put(context.input, "settings", "invalid"))
    ]

    for input <- inputs do
      assert {:error, :json} =
               Verifier.verify_standard_json_candidates(
                 context.address.hash,
                 context.params,
                 input,
                 [context.candidate]
               )
    end

    assert Repo.get_by(SmartContract, address_hash: context.address.hash) == nil
  end

  test "default optimizer does not bypass chain bytecode verification", context do
    mismatched = Map.put(context.candidate, "bytecode", "deadbeef")

    assert {:error, {:error, :generated_bytecode}} =
             Verifier.verify_standard_json_candidates(
               context.address.hash,
               context.params,
               Jason.encode!(context.input),
               [mismatched]
             )

    assert Repo.get_by(SmartContract, address_hash: context.address.hash) == nil
    refute Repo.get!(Address, context.address.hash).verified
  end

  defp assert_published(context, settings) do
    input = Map.put(context.input, "settings", settings)

    assert {:ok, %{abi: abi}, additional_params} =
             Verifier.verify_standard_json_candidates(
               context.address.hash,
               context.params,
               Jason.encode!(input),
               [context.candidate]
             )

    assert abi == context.recording.abi
    assert additional_params["optimization"] == false
    refute Map.has_key?(additional_params, "optimization_runs")
    assert additional_params["compiler_settings"] == settings
    assert additional_params["contract_source_code"] == context.recording.source_code
    assert additional_params["name"] == context.recording.name
    assert additional_params["file_path"] == "SimpleStorage.sol"
    assert additional_params["secondary_sources"] == []
    assert additional_params["external_libraries"] == %{}

    assert {:ok, %SmartContract{} = published} =
             Publisher.publish_smart_contract(
               context.address.hash,
               Map.merge(context.params, additional_params),
               abi,
               true
             )

    stored = Repo.get_by!(SmartContract, address_hash: context.address.hash)
    assert stored.address_hash == published.address_hash
    assert stored.optimization == false
    assert stored.optimization_runs == nil
    assert stored.compiler_settings == if(settings == %{}, do: nil, else: settings)
    assert stored.abi == context.recording.abi
    assert stored.compiler_version == context.recording.version
    assert stored.contract_source_code == context.recording.source_code
    assert stored.file_path == "SimpleStorage.sol"
    assert Repo.get!(Address, context.address.hash).verified
  end
end
