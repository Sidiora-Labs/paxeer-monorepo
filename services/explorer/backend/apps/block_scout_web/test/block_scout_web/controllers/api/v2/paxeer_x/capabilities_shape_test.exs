defmodule BlockScoutWeb.API.V2.PaxeerX.CapabilitiesShapeTest do
  use BlockScoutWeb.ConnCase, async: false

  alias BlockScoutWeb.Schemas.API.V2.PaxeerX.Capabilities
  alias Explorer.Chain.PaxeerX.Capabilities, as: CapabilityProbe
  alias OpenApiSpex.{Components, Info, OpenApi}

  @path "/api/v2/paxeer-x/capabilities"
  @keys ~w(addr exchange bridge launchpad checked_at)
  @fetcher Indexer.Fetcher.PaxeerX.PriceUpdates.Supervisor

  setup do
    for key <- [Indexer.Supervisor, @fetcher] do
      previous = Application.fetch_env(:indexer, key)

      on_exit(fn ->
        case previous do
          {:ok, value} -> Application.put_env(:indexer, key, value)
          :error -> Application.delete_env(:indexer, key)
        end
      end)
    end

    Application.put_env(:indexer, Indexer.Supervisor, enabled: true)
    :ok
  end

  test "capabilities_response_has_no_retired_keys", %{conn: conn} do
    for feed_enabled? <- [false, true] do
      Application.put_env(:indexer, @fetcher, disabled?: not feed_enabled?)
      snapshot = CapabilityProbe.all()
      response = conn |> get(@path) |> json_response(200)
      expected_keys = if feed_enabled?, do: ["price_feed" | @keys], else: @keys

      assert Enum.sort(Map.keys(response)) == Enum.sort(expected_keys)
      refute Map.has_key?(response, "custody")
      refute Map.has_key?(response, "anchor")
      assert response["addr"] == snapshot.unified_account
      assert response["exchange"] == snapshot.exchange
      assert response["bridge"] == snapshot.bridge
      assert response["launchpad"] == snapshot.launchpad
      assert response["checked_at"] == Jason.decode!(Jason.encode!(snapshot.checked_at))

      if feed_enabled?, do: assert(response["price_feed"] == true)
    end
  end

  test "capabilities_response_casts_against_schema", %{conn: conn} do
    spec =
      OpenApiSpex.resolve_schema_modules(%OpenApi{
        info: %Info{title: "Capabilities response test", version: "1"},
        paths: %{},
        components: %Components{schemas: %{"PaxeerXCapabilities" => Capabilities.schema()}}
      })

    schema = spec.components.schemas["PaxeerXCapabilities"]
    assert schema.additionalProperties == false

    for feed_enabled? <- [false, true] do
      Application.put_env(:indexer, @fetcher, disabled?: not feed_enabled?)
      response = conn |> get(@path) |> json_response(200)
      assert {:ok, _body} = OpenApiSpex.cast_value(response, schema, spec)

      for retired_key <- ["custody", "anchor"] do
        assert {:error, errors} = OpenApiSpex.cast_value(Map.put(response, retired_key, false), schema, spec)

        assert Enum.any?(errors, fn error ->
                 error.reason == :unexpected_field and error.name == retired_key
               end)
      end
    end
  end
end
