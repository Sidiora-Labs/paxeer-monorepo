defmodule BlockScoutWeb.Repair22_33_510Test do
  @moduledoc """
  Repair record for audit 22.30.152:
  duplicate-ajax-header is proved by the RecentTransactionsController refusal
  and recorded-transaction cases; endpoint-prefix is proved by the mounted
  BlockController redirects, search redirect and URI preservation cases.
  The committed geth struct_logger recording supplies the transaction and block.
  MountedEndpoint exposes the production WebRouter with an in-memory session,
  including when the main API-only router disables web routes. Controllers and
  views are production code. No out-of-path wiring is required. The explicit bounded gate order
  forbids baseline verification; no baseline test result is claimed. The runner
  records the verified candidate revision and private evidence location.
  """

  use BlockScoutWeb.ConnCase, async: false

  alias BlockScoutWeb.{Controller, Endpoint}
  alias EthereumJSONRPC

  @recording Path.expand("../../../ethereum_jsonrpc/test/support/fixture/geth/trace/struct_logger.json", __DIR__)

  defmodule MountedEndpoint do
    use Plug.Router

    plug(:prepare)
    plug(:match)
    plug(:dispatch)

    forward("/explorer", to: BlockScoutWeb.Routers.WebRouter)
    forward("/explorer-other", to: BlockScoutWeb.Routers.WebRouter)
    forward("/address/explorer-token", to: BlockScoutWeb.Routers.WebRouter)
    forward("/parent/explorer", to: BlockScoutWeb.Routers.WebRouter)
    forward("/", to: BlockScoutWeb.Routers.WebRouter)

    defp prepare(conn, _opts) do
      conn = %{conn | secret_key_base: BlockScoutWeb.Endpoint.config(:secret_key_base)}

      conn
      |> Plug.Conn.put_private(:phoenix_endpoint, BlockScoutWeb.Endpoint)
      |> Plug.Test.init_test_session(%{})
    end
  end

  setup do
    config = Application.fetch_env!(:block_scout_web, Endpoint)
    on_exit(fn -> Application.put_env(:block_scout_web, Endpoint, config) end)

    set_mount("/explorer")
    :ok
  end

  test "duplicate-ajax-header: ambiguous, absent and invalid markers retain the controller's 422 refusal" do
    for values <- [
          [],
          [""],
          ["fetch"],
          ["XMLHttpRequest, XMLHttpRequest"],
          [" XMLHttpRequest"],
          ["XMLHttpRequest", "XMLHttpRequest"],
          ["xmlhttprequest", "XMLHTTPREQUEST"],
          ["XMLHttpRequest", "invalid"],
          ["invalid", "XMLHttpRequest"],
          ["XMLHttpRequest", "XMLHttpRequest", "XMLHttpRequest"]
        ] do
      conn = build_conn()
      conn = %{conn | req_headers: Enum.map(values, &{"x-requested-with", &1})}
      response = dispatch(conn, MountedEndpoint, :get, "/recent-transactions")

      assert response.status == 422
      assert response.halted
      assert get_resp_header(response, "content-type") |> hd() =~ "text/html"
    end
  end

  test "duplicate-ajax-header: one marker in any case returns the recorded transaction" do
    {tx, receipt} = recording()
    from = insert(:address, hash: tx["from"])
    to = insert(:address, hash: tx["to"])
    block = insert(:block, hash: tx["blockHash"], number: EthereumJSONRPC.quantity_to_integer(tx["blockNumber"]))

    transaction =
      :transaction
      |> insert(
        hash: tx["hash"],
        from_address: from,
        to_address: to,
        input: tx["input"],
        value: EthereumJSONRPC.quantity_to_integer(tx["value"])
      )
      |> with_block(block,
        index: EthereumJSONRPC.quantity_to_integer(tx["transactionIndex"]),
        gas_used: EthereumJSONRPC.quantity_to_integer(receipt["gasUsed"]),
        cumulative_gas_used: EthereumJSONRPC.quantity_to_integer(receipt["cumulativeGasUsed"]),
        status: :ok
      )

    assert receipt["transactionHash"] == to_string(transaction.hash)

    for marker <- ["XMLHttpRequest", "xmlhttprequest", "XMLHTTPREQUEST", "xMlHtTpReQuEsT"] do
      body =
        build_conn()
        |> put_req_header("x-requested-with", marker)
        |> dispatch(MountedEndpoint, :get, "/recent-transactions")
        |> Phoenix.ConnTest.json_response(200)

      assert [%{"transaction_hash" => hash, "transaction_html" => html}] = body["transactions"]
      assert hash == tx["hash"]
      assert html =~ hash
    end
  end

  test "endpoint-prefix: real mounted block redirects recognize only complete leading segments" do
    {tx, _receipt} = recording()
    suffix = "/block/#{tx["blockHash"]}/transactions"

    for mount <- ["/explorer", "/explorer/", "explorer/"] do
      set_mount(mount)

      for {incoming, expected} <- [
            {"", "/explorer"},
            {"/explorer", "/explorer"},
            {"/explorer-other", "/explorer/explorer-other"},
            {"/address/explorer-token", "/explorer/address/explorer-token"},
            {"/parent/explorer", "/explorer/parent/explorer"}
          ] do
        response = dispatch(build_conn(), MountedEndpoint, :get, incoming <> "/block/" <> tx["blockHash"])
        assert redirected_to(response, 302) == expected <> suffix
      end
    end
  end

  test "endpoint-prefix: a mounted search redirect preserves the query transport encoding" do
    query = "missing/explorer-token + %2F & next=/explorer"
    encoded_query = URI.encode_query(%{"q" => query})
    response = dispatch(build_conn(), MountedEndpoint, :get, "/explorer/search?" <> encoded_query)

    assert redirected_to(response, 302) == "/explorer/search-results?" <> encoded_query
  end

  test "endpoint-prefix: URI path boundaries preserve raw query and fragment bytes" do
    query = "?next=/explorer&encoded=%2f%2F%252F&space=a+b%20c&repeat=1&repeat=2#part/explorer"

    for mount <- ["/explorer", "/explorer/", "explorer/"] do
      set_mount(mount)

      for {path, expected} <- [
            {"/address/token", "/explorer/address/token"},
            {"/address/explorer-token", "/explorer/address/explorer-token"},
            {"/explorer-other", "/explorer/explorer-other"},
            {"/explorer%2Fother", "/explorer/explorer%2Fother"},
            {"/explorer", "/explorer"},
            {"/explorer/", "/explorer/"},
            {"/explorer/address/token/", "/explorer/address/token/"},
            {"address/token/", "/explorer/address/token/"},
            {"/", "/explorer/"},
            {"", "/explorer/"}
          ] do
        assert Controller.full_path(path <> query) == expected <> query
        assert Controller.full_path(expected <> query) == expected <> query
      end
    end

    for mount <- ["/", "", nil] do
      set_mount(mount)
      assert Controller.full_path("/address/token/" <> query) == "/address/token/" <> query
    end
  end

  defp set_mount(path) do
    config = Application.fetch_env!(:block_scout_web, Endpoint)
    url = Keyword.put(config[:url] || [], :path, path)
    Application.put_env(:block_scout_web, Endpoint, Keyword.put(config, :url, url))
  end

  defp recording do
    data = @recording |> File.read!() |> Jason.decode!()
    {data["tx"], data["receipt"]}
  end
end
