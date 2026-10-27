defmodule Explorer.Chain.Import.Stage.PaxeerXRunnersTest do
  use ExUnit.Case, async: false

  alias Explorer.Chain.Import.Runner.PaxeerX.{AccountBindings, MarketEvents}
  alias Explorer.Chain.Import.Stage.ChainTypeSpecific

  setup do
    previous_env = Enum.map([:chain_type, :chain_identity], &{&1, Application.fetch_env(:explorer, &1)})

    Application.put_env(:explorer, :chain_type, :paxeer_x)
    Application.put_env(:explorer, :chain_identity, nil)

    on_exit(fn ->
      Enum.each(previous_env, fn
        {key, {:ok, value}} -> Application.put_env(:explorer, key, value)
        {key, :error} -> Application.delete_env(:explorer, key)
      end)
    end)

    :ok
  end

  test "paxeer_x_stage_lists_two_runners" do
    paxeer_x_stage_lists_two_runners()
  end

  defp paxeer_x_stage_lists_two_runners do
    assert ChainTypeSpecific.runners() == [AccountBindings, MarketEvents]
  end
end
