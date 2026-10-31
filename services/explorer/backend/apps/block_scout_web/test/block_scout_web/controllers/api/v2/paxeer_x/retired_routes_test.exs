defmodule BlockScoutWeb.API.V2.PaxeerX.RetiredRoutesTest do
  use BlockScoutWeb.ConnCase

  test "retired_paxeer_x_routes_answer_404", context do
    retired_paxeer_x_routes_answer_404(context)
  end

  defp retired_paxeer_x_routes_answer_404(%{conn: conn}) do
    for path <- ["anchors", "receipts", "receipts/0x01", "unknown/nested/path"] do
      assert json_response(get(conn, "/api/v2/paxeer-x/#{path}"), 404) == %{"message" => "Not found"}
    end
  end

  test "archive_routes_survive_retirement", context do
    archive_routes_survive_retirement(context)
  end

  defp archive_routes_survive_retirement(%{conn: conn}) do
    assert json_response(get(conn, "/api/v2/paxeer-x/archive/anchors"), 200) == %{
             "items" => [],
             "next_page_params" => nil
           }
  end
end
