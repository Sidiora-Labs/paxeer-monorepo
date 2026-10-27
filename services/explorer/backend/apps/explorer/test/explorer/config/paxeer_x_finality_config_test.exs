defmodule Explorer.Config.PaxeerXFinalityConfigTest do
  use ExUnit.Case, async: false

  test "finality_config_is_not_set" do
    finality_config_is_not_set()
  end

  defp finality_config_is_not_set do
    assert is_nil(Application.get_env(:explorer, Explorer.Chain.PaxeerX.Finality))
  end
end
