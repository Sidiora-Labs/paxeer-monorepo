defmodule BlockScoutWeb.API.V2.PaxeerX.UnifiedAccountViewGoneTest do
  use ExUnit.Case, async: true

  test "unified_account_view_is_gone" do
    unified_account_view_is_gone()
  end

  def unified_account_view_is_gone do
    refute Code.ensure_loaded?(BlockScoutWeb.API.V2.PaxeerX.UnifiedAccountView)
  end
end
