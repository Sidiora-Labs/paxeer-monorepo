Code.require_file("../indexer/transform/token_transfers_test.exs", __DIR__)

defmodule Indexer.Repair2233562Test do
  use Explorer.DataCase, async: false

  alias Explorer.Chain.TokenTransfer
  alias Indexer.Transform.TokenTransfers

  @failure "deliberate assertion failure before WETH test teardown"
  @source Path.expand("../indexer/transform/token_transfers_test.exs", __DIR__)
  @external_resource @source

  setup do
    original = Application.fetch_env!(:explorer, TokenTransfer)
    on_exit(fn -> Application.put_env(:explorer, TokenTransfer, original) end)

    previous =
      Keyword.merge(original,
        weth_token_transfers_filtering_enabled: true,
        whitelisted_weth_contracts: ["0x000000000000000000000000000000000000dead"],
        retained_option: :preserve_me
      )

    Application.put_env(:explorer, TokenTransfer, previous)
    %{previous: previous}
  end

  {_, cases} =
    @source
    |> File.read!()
    |> Code.string_to_quoted!()
    |> Macro.prewalk([], fn
      {:test, _, [name, [do: {:__block__, _, expressions}]]} = node, cases
      when name in [
             "parse/1 parses logs for tokens and token transfers",
             "Filters WETH transfers from not whitelisted tokens",
             "Filters duplicates WETH transfers"
           ] ->
        {node, [{name, expressions} | cases]}

      node, cases ->
        {node, cases}
    end)

  if length(cases) != 3, do: raise("expected all three original WETH configuration cases")

  for {name, expressions} <- cases do
    cleanup_index = Enum.find_index(expressions, &match?({:on_exit, _, _}, &1))

    mutation_index =
      Enum.find_index(expressions, fn
        {{:., _, [{:__aliases__, _, [:Application]}, :put_env]}, _, _} -> true
        _ -> false
      end)

    body =
      Enum.flat_map(expressions, fn
        {:assert, _, _} = assertion ->
          [
            assertion,
            quote do
              for params <- var!(expected).token_transfers do
                changeset = TokenTransfer.changeset(%TokenTransfer{}, params)
                assert changeset.valid?, inspect(changeset.errors)
              end

              flunk(@failure)
            end
          ]

        expression ->
          [expression]
      end)

    test "026: #{name} restores configuration after an assertion failure", %{previous: previous} do
      assert is_integer(unquote(cleanup_index))
      assert unquote(cleanup_index) < unquote(mutation_index)

      assert_cleanup(previous, fn ->
        (unquote_splicing(body))
      end)
    end
  end

  defp assert_cleanup(previous, exercise) do
    owner = self()

    {pid, monitor} =
      spawn_monitor(fn ->
        :ok = ExUnit.OnExitHandler.register(self())

        try do
          exercise.()
        rescue
          error in ExUnit.AssertionError -> send(owner, {:assertion_failure, self(), error.message})
        end
      end)

    on_exit(fn ->
      if Process.alive?(pid), do: Process.exit(pid, :kill)
      if :ets.member(ExUnit.Server, pid), do: ExUnit.OnExitHandler.run(pid, 5_000)
    end)

    assert_receive {:assertion_failure, ^pid, @failure}, 5_000
    assert_receive {:DOWN, ^monitor, :process, ^pid, :normal}, 5_000
    refute Application.fetch_env!(:explorer, TokenTransfer) == previous
    assert :ok = ExUnit.OnExitHandler.run(pid, 5_000)
    assert Application.fetch_env!(:explorer, TokenTransfer) == previous
  end

  defp truncated_hash("0x000000000000000000000000" <> rest), do: "0x" <> rest
end
