defmodule BlockScoutWeb.TestApiSchemaAssertions do
  @moduledoc """
  Validates API JSON responses against their OpenAPI operation and response schema.

  Non-API web routes and the separately typed GraphQL endpoint are outside this contract.
  The exact `/api`, `/api/`, `/api/v1` and `/api/v1/`
  legacy RPC entry points intentionally use the RPC tests' action-specific schemas;
  they have no single OpenAPI response shape. Router catch-all rejections have no declared operation.
  Every declared REST operation is checked.
  Missing declarations and ambiguous route templates fail the calling test.
  """

  import ExUnit.Assertions, only: [flunk: 1]

  alias OpenApiSpex.{MediaType, Operation, PathItem, Reference, Response, Schema}

  @spec json_response(Plug.Conn.t(), non_neg_integer()) :: map() | list()
  def json_response(%Plug.Conn{} = conn, status_code) when is_integer(status_code) do
    json = Phoenix.ConnTest.json_response(conn, status_code)

    if covered_path?(conn.request_path) and declared_route?(conn) do
      assert_response_schema(conn, status_code, json, [
        BlockScoutWeb.Specs.Public.spec(),
        BlockScoutWeb.Specs.Private.spec()
      ])
    end

    json
  end

  @doc "Validates a decoded producer response against the supplied OpenAPI documents."
  def assert_response_schema(%Plug.Conn{method: method, request_path: path}, status, json, specs) do
    {spec, item} = find_path_item!(specs, strip_api_prefix(path))

    method_atom =
      Enum.find(
        [:get, :put, :post, :delete, :options, :head, :patch, :trace],
        &(Atom.to_string(&1) == String.downcase(method))
      )

    operation =
      case Map.get(item, method_atom) do
        %Operation{} = operation -> operation
        _ -> flunk("Missing OpenAPI #{method} operation for path: #{path}")
      end

    schema = find_response_schema!(operation, status, spec, path)
    OpenApiSpex.TestAssertions.assert_raw_schema(json, schema, spec)
  end

  defp declared_route?(%Plug.Conn{private: %{phoenix_router: router}} = conn) do
    {prefix, _forwards} = Map.fetch!(conn.private, router)
    path = "/" <> Enum.join(Enum.drop(conn.path_info, length(prefix)), "/")

    case Phoenix.Router.route_info(router, conn.method, path, conn.host) do
      %{plug: BlockScoutWeb.API.V2.FallbackController} -> false
      _ -> true
    end
  end

  defp declared_route?(_conn), do: true

  defp covered_path?("/api/v1"), do: false
  defp covered_path?("/api/v1/"), do: false
  defp covered_path?("/api/v1/graphql"), do: false
  defp covered_path?("/api/v1/graphql/" <> _), do: false
  defp covered_path?(path), do: String.starts_with?(path, "/api/") and path != "/api/"

  defp strip_api_prefix("/api/" <> rest), do: "/" <> rest
  defp strip_api_prefix(path), do: path

  defp find_path_item!(specs, path) do
    candidates =
      for %{paths: paths} = spec <- specs,
          {template, %PathItem{} = item} <- paths,
          segments_match?(split_path(template), split_path(path)) do
        specificity = template |> split_path() |> Enum.count(&(not dynamic_segment?(&1)))
        {template, specificity, spec, item}
      end

    exact = Enum.filter(candidates, fn {template, _, _, _} -> template == path end)

    best =
      if exact == [] do
        case candidates do
          [] ->
            []

          _ ->
            {_, score, _, _} = Enum.max_by(candidates, &elem(&1, 1))
            Enum.filter(candidates, &(elem(&1, 1) == score))
        end
      else
        exact
      end

    case best do
      [{_, _, spec, item}] ->
        {spec, item}

      [] ->
        flunk("Missing OpenAPI path: #{path}")

      _ ->
        templates = best |> Enum.map(&elem(&1, 0)) |> Enum.sort() |> Enum.join(", ")
        flunk("Ambiguous OpenAPI path: #{path}; matching templates: #{templates}")
    end
  end

  defp split_path(path), do: String.split(path, "/", trim: true)

  defp segments_match?(templates, actual) when length(templates) == length(actual) do
    Enum.zip(templates, actual) |> Enum.all?(fn {t, a} -> dynamic_segment?(t) or t == a end)
  end

  defp segments_match?(_, _), do: false

  defp dynamic_segment?(segment), do: String.starts_with?(segment, "{") and String.ends_with?(segment, "}")

  defp find_response_schema!(%Operation{responses: responses}, status, spec, path) do
    response =
      if is_map(responses) do
        Map.get(responses, Integer.to_string(status)) || Map.get(responses, status) || Map.get(responses, "default")
      end

    response =
      case response do
        %Reference{"$ref": "#/components/responses/" <> name} ->
          responses = if spec.components, do: spec.components.responses || %{}, else: %{}
          Map.get(responses, name)

        other ->
          other
      end

    case response do
      %Response{content: %{"application/json" => %MediaType{schema: %Schema{} = schema}}} -> schema
      %Response{content: %{"application/json" => %MediaType{schema: %Reference{} = schema}}} -> schema
      _ -> flunk("Missing OpenAPI JSON response schema for path: #{path} and status: #{status}")
    end
  end
end
