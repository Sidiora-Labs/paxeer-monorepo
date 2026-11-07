defmodule BlockScoutWeb.API.V2.PaxeerX.CapabilitiesController do
  @moduledoc """
  Publishes which Paxeer X Network surfaces the connected node answers today.

  The booleans come from `Explorer.Chain.PaxeerX.Capabilities`, which probes the
  precompile addresses on a timer. `addr` is the addr precompile, published
  under the name the unified surfaces read it by; a surface no probe has
  answered for yet reads as false. `price_feed` is present, and true, only
  while native price-feed submissions are served: this node runs the indexer
  with the price-feed fetcher enabled, or the database it reads holds scanned
  price-feed blocks within 1000 blocks of the current height written by an indexer
  elsewhere, while this node's indexer is disabled.
  """

  use BlockScoutWeb, :controller
  use OpenApiSpex.ControllerSpecs

  alias Explorer.Chain.PaxeerX.{Capabilities, PriceFeed}

  @price_feed_recent_blocks 1000

  plug(OpenApiSpex.Plug.CastAndValidate, json_render_error_v2: true)

  tags(["paxeer-x"])

  operation(:capabilities,
    summary: "Get the live Paxeer X Network surfaces",
    description:
      "Returns one boolean per LayerX surface precompile — addr, exchange, bridge, launchpad — " <>
        "as of the last probe of the connected node. `addr` is true when the addr precompile answers " <>
        "getUnifiedAccount. A surface that the running chain does not carry yet reads as false. " <>
        "`price_feed` is present, and true, only while native price-feed submissions are served: the node " <>
        "runs its indexer with the price-feed fetcher enabled, or, with its indexer disabled, its database " <>
        "holds a successfully scanned price-feed block within 1000 blocks of the current block height.",
    parameters: base_params(),
    responses: [
      ok: {"Surfaces the connected node answers.", "application/json", Schemas.PaxeerX.Capabilities},
      unprocessable_entity: JsonErrorResponse.response()
    ]
  )

  @doc """
    Function to handle GET requests to `/api/v2/paxeer-x/capabilities` endpoint.
  """
  @spec capabilities(Plug.Conn.t(), map()) :: Plug.Conn.t()
  def capabilities(conn, _params) do
    snapshot = Capabilities.all()

    conn
    |> put_status(200)
    |> json(
      Map.merge(price_feed(), %{
        "addr" => snapshot.unified_account,
        "exchange" => snapshot.exchange,
        "bridge" => snapshot.bridge,
        "launchpad" => snapshot.launchpad,
        "checked_at" => snapshot.checked_at
      })
    )
  end

  defp price_feed do
    indexer_enabled? = Application.get_env(:indexer, Indexer.Supervisor, [])[:enabled] == true
    fetcher_disabled? = Application.get_env(:indexer, Indexer.Fetcher.PaxeerX.PriceUpdates.Supervisor, [])[:disabled?]

    available? =
      if indexer_enabled?,
        do: fetcher_disabled? != true,
        else: PriceFeed.scanned_recently?(@price_feed_recent_blocks)

    if available?,
      do: %{"price_feed" => true},
      else: %{}
  end
end
