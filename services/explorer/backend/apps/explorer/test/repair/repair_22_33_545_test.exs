Code.require_file("../explorer/smart_contract/solidity/chain_coverage_cases.exs", __DIR__)

defmodule Explorer.Repair.Repair2233545Test do
  @moduledoc """
  Task 22.33.545, audit 22.30.157:
  solidity-compiler-chain-coverage is proved by the compiler_cases tests;
  solidity-publisher-chain-coverage is proved by the publisher_cases tests;
  solidity-verifier-chain-coverage is proved by the verifier_cases tests.
  Each case is shared with its repaired suite, whose original tests and
  assertions remain enabled for both default and deployed paxeer_x builds.

  The committed Explorer.Factory Solidity compiler/chain recordings pass
  through CodeCompiler.get_contract_info, the production standard-JSON
  candidate consumer, indexed transaction and address schemas, bytecode
  comparison, Publisher and persisted SmartContract and ContractMethod rows.
  The candidate seam is the same consumer called after CodeCompiler.run/2;
  this focused gate needs no compiler download, subprocess or live endpoint.
  The retained full compiler suites still exercise compilation themselves.

  Baseline: no previous gate ledger or scoped implementation was present.
  No unchanged-tree gate ran: the explicit implementation-first gate order
  forbids it; no pre-change failures are claimed. hf-build and hf-done record
  the candidate revision, command, exit and private evidence location.
  No out-of-path wiring changes are required. Spec and gotcha ledger edits
  are outside the user-authorized scope, so this module is the task record.
  """

  use ExUnit.Case, async: false

  require Explorer.SmartContract.Solidity.ChainCoverageCases

  setup do
    assert Application.fetch_env!(:explorer, :chain_type) == :paxeer_x
    :ok = Ecto.Adapters.SQL.Sandbox.checkout(Explorer.Repo)
  end

  Explorer.SmartContract.Solidity.ChainCoverageCases.compiler_cases()
  Explorer.SmartContract.Solidity.ChainCoverageCases.publisher_cases()
  Explorer.SmartContract.Solidity.ChainCoverageCases.verifier_cases()
end
