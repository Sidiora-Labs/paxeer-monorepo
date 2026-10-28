defmodule Explorer.Chain.PaxeerX.CapabilitiesRetiredProbeTest do
  use ExUnit.Case, async: true

  alias Explorer.Chain.PaxeerX.Capabilities

  test "surfaces_omit_retired_precompiles" do
    surfaces_omit_retired_precompiles()
  end

  test "address_refuses_retired_surfaces" do
    address_refuses_retired_surfaces()
  end

  defp surfaces_omit_retired_precompiles do
    assert Capabilities.surfaces() == [:unified_account, :exchange, :bridge, :launchpad]
  end

  defp address_refuses_retired_surfaces do
    assert_raise FunctionClauseError, fn -> Capabilities.address(:custody) end
    assert_raise FunctionClauseError, fn -> Capabilities.address(:anchor) end

    assert Capabilities.address(:exchange) == "0x0000000000000000000000000000000000001015"
  end
end
