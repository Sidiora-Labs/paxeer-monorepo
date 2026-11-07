defmodule Explorer.Repair.Repair2233375Test do
  @moduledoc """
  Task 22.33.375 / audit 22.30.140 / proxy-verification-uid.
  Proof: "simultaneous submissions retain independent pending and terminal states"
  drives the production status schema, persistence, callback and polling path.
  "duplicate insertion explicitly refuses ownership of an existing attempt"
  proves the conflict result and preservation of the original row.

  Wiring request (domain 14, explorer RPC):
  apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex,
  verifyproxycontract/2 must match {:ok, _} from insert_status/3 before launching
  Implementation.get_implementation/2 or returning the guid; render an explicit
  admission error for {:error, changeset}. That caller is outside this task's scope.

  Baseline: the runner ledger contained no earlier build or verify runs and there
  were no repair edits. No unchanged-tree test was run: the ordered instructions
  prohibit verification before implementation and permit only hf-build/hf-done.
  The gate records the verified revision and private evidence location.
  The recorded Geth callTracer supplies the contract address values; all state
  transitions use production schemas and the gate-provided disposable repository.
  """

  use ExUnit.Case, async: false

  import Explorer.Factory

  alias Ecto.Adapters.SQL.Sandbox
  alias Explorer.{Chain, Repo}
  alias Explorer.Chain.SmartContract.Proxy.Models.Implementation
  alias Explorer.Chain.SmartContract.Proxy.VerificationStatus

  @recording Path.expand("../../../ethereum_jsonrpc/test/support/fixture/geth/trace/calltracer.json", __DIR__)

  setup do
    :ok = Sandbox.checkout(Repo)
    :ok = Sandbox.mode(Repo, {:shared, self()})
    %{"result" => %{"to" => proxy, "from" => implementation}} = @recording |> File.read!() |> Jason.decode!()
    {:ok, proxy_hash} = Chain.string_to_address_hash(proxy)
    {:ok, implementation_hash} = Chain.string_to_address_hash(implementation)
    insert(:smart_contract, address_hash: insert(:address, hash: proxy_hash).hash)
    insert(:address, hash: implementation_hash)

    %{proxy: proxy_hash, implementation: implementation_hash}
  end

  test "simultaneous submissions retain independent pending and terminal states", context do
    implementation =
      Implementation.save_implementation_data(
        %{proxy_address_hash: context.proxy, address_hashes: [context.implementation], proxy_type: :eip1167},
        []
      )

    assert %Implementation{address_hashes: [implementation_hash]} = implementation
    assert implementation_hash == context.implementation

    parent = self()

    tasks =
      for _ <- 1..2 do
        Task.async(fn ->
          send(parent, {:ready, self()})
          receive do: (:submit -> :ok)
          uid = VerificationStatus.generate_uid(context.proxy)
          insertion = VerificationStatus.insert_status(uid, :pending, to_string(context.proxy))
          send(parent, {:submitted, self(), uid, insertion})

          receive do
            {:complete, result} -> VerificationStatus.set_proxy_verification_result(result, uid)
          end
        end)
      end

    [first, second] = tasks
    first_pid = first.pid
    second_pid = second.pid
    assert_receive {:ready, ^first_pid}
    assert_receive {:ready, ^second_pid}
    Enum.each(tasks, &send(&1.pid, :submit))

    assert_receive {:submitted, ^first_pid, first_uid, {:ok, %VerificationStatus{status: :pending}}}, 5_000
    assert_receive {:submitted, ^second_pid, second_uid, {:ok, %VerificationStatus{status: :pending}}}, 5_000
    refute first_uid == second_uid
    assert byte_size(first_uid) == 64
    assert byte_size(second_uid) == 64
    assert poll(first_uid).status == :pending
    assert poll(second_uid).status == :pending

    send(first.pid, {:complete, implementation})
    assert {:ok, %VerificationStatus{uid: ^first_uid, status: :pass}} = Task.await(first)
    assert poll(first_uid).status == :pass
    assert poll(second_uid).status == :pending

    send(second.pid, {:complete, :empty})
    assert {:ok, %VerificationStatus{uid: ^second_uid, status: :fail}} = Task.await(second)
    assert poll(first_uid).status == :pass
    assert poll(second_uid).status == :fail
    assert poll(first_uid).contract_address_hash == context.proxy
    assert poll(second_uid).contract_address_hash == context.proxy
  end

  test "duplicate insertion explicitly refuses ownership of an existing attempt", context do
    uid = VerificationStatus.generate_uid(context.proxy)
    assert {:ok, original} = VerificationStatus.insert_status(uid, :pending, context.proxy)

    assert {:error, changeset} = VerificationStatus.insert_status(uid, :fail, context.proxy)
    assert {"has already been taken", options} = Keyword.fetch!(changeset.errors, :uid)
    assert options[:constraint] == :unique
    assert options[:constraint_name] == "proxy_smart_contract_verification_statuses_pkey"
    assert poll(uid) == original

    # The refused insert must leave the enclosing transaction usable.
    next_uid = VerificationStatus.generate_uid(context.proxy)
    refute next_uid == uid
    assert {:ok, _} = VerificationStatus.insert_status(next_uid, :pending, context.proxy)
    assert {:ok, _} = VerificationStatus.set_proxy_verification_result(:empty, next_uid)
    assert poll(uid).status == :pending
    assert poll(next_uid).status == :fail
  end

  test "generated identifiers satisfy the polling validator for concurrent same-address requests", context do
    identifiers =
      1..256
      |> Task.async_stream(fn _ -> VerificationStatus.generate_uid(context.proxy) end, max_concurrency: 32)
      |> Enum.map(fn {:ok, uid} -> uid end)

    assert MapSet.size(MapSet.new(identifiers)) == 256

    Enum.each(identifiers, fn uid ->
      assert byte_size(uid) == 64
      assert {:ok, <<_::256>>} = Base.decode16(uid, case: :lower)
      assert {:ok, ^uid} = VerificationStatus.validate_uid(uid)
    end)
  end

  test "polling retains durable legacy identifiers and refuses malformed input", context do
    address = Base.encode16(context.proxy.bytes, case: :lower)
    legacy_uid = address <> "6553f100"
    assert {:ok, _} = VerificationStatus.insert_status(legacy_uid, :pending, context.proxy)
    assert {:ok, ^legacy_uid} = VerificationStatus.validate_uid(legacy_uid)
    assert poll(legacy_uid).status == :pending
    assert {:ok, _} = VerificationStatus.set_proxy_verification_result(:empty, legacy_uid)
    assert poll(legacy_uid).status == :fail

    future = DateTime.utc_now() |> DateTime.to_unix() |> Kernel.+(86_400) |> Integer.to_string(16) |> String.downcase()

    for invalid <- [
          nil,
          12,
          "",
          address,
          address <> future,
          address <> "-1",
          address <> "+1",
          address <> "0001",
          String.duplicate("g", 40) <> "6553f100",
          String.duplicate("g", 64),
          String.duplicate("a", 65)
        ] do
      assert VerificationStatus.validate_uid(invalid) == :error
      assert VerificationStatus.fetch_status(invalid) == nil
    end
  end

  defp poll(uid) do
    %{"guid" => guid} = %{"guid" => uid} |> Jason.encode!() |> Jason.decode!()
    VerificationStatus.fetch_status(guid)
  end
end

