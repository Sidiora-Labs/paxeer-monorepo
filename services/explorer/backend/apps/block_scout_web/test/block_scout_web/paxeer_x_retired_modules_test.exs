defmodule BlockScoutWeb.PaxeerXRetiredModulesTest do
  use ExUnit.Case, async: true

  test "live_kernel_controllers_are_gone" do
    refute Code.ensure_loaded?(BlockScoutWeb.API.V2.PaxeerX.AnchorController)
    refute Code.ensure_loaded?(BlockScoutWeb.API.V2.PaxeerX.ReceiptController)
    refute Code.ensure_loaded?(BlockScoutWeb.API.V2.PaxeerX.AnchorView)
    refute Code.ensure_loaded?(BlockScoutWeb.API.V2.PaxeerX.ReceiptView)
  end
end
