defmodule BlockScoutWeb.PageAssetsTest do
  use BlockScoutWeb.ConnCase, async: false

  test "serves the page scripts and styles", %{conn: conn} do
    for path <- ["/js/app.js", "/js/chain.js", "/js/address-transactions.js", "/css/app.css"] do
      result = get(conn, path)
      assert byte_size(response(result, 200)) > 0
      assert get_resp_header(result, "content-type") != []
    end
  end

  test "loads the wallet sign-in native library" do
    nonce = Siwe.generate_nonce()
    assert is_binary(nonce)
    assert String.match?(nonce, ~r/^[A-Za-z0-9]{8,}$/)
  end
end
