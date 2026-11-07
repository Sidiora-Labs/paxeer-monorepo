defmodule BlockScoutWeb.Specs.Private do
  @moduledoc """
  This module defines the private API specification for the BlockScoutWeb application.
  """

  alias BlockScoutWeb.Routers.AccountRouter
  alias BlockScoutWeb.Specs
  alias BlockScoutWeb.Account.API.V2.{AddressController, EmailController, TagsController, UserController}
  alias BlockScoutWeb.Schemas.API.V2.{Account, Address, General}

  alias OpenApiSpex.{
    Components,
    Contact,
    Header,
    Info,
    MediaType,
    OpenApi,
    Operation,
    Parameter,
    PathItem,
    Paths,
    RequestBody,
    Response,
    Schema,
    SecurityScheme,
    Server
  }

  alias Utils.Helper

  @behaviour OpenApi

  @impl OpenApi
  def spec do
    %OpenApi{
      servers: [
        %Server{url: to_string(Helper.instance_url() |> URI.append_path("/api"))}
      ],
      info: %Info{
        title: "Blockscout Private API",
        description:
          "Account v2 authentication and user preferences. Retain the sign-in session cookie and obtain " <>
            "x-bs-account-csrf from GET /account/v2/get_csrf; send it as x-csrf-token on session-protected writes.",
        version: to_string(Application.spec(:block_scout_web, :vsn)),
        contact: %Contact{
          email: "info@blockscout.com"
        }
      },
      paths: account_paths(:private),
      components: %Components{
        securitySchemes: %{"dynamic_jwt" => %SecurityScheme{type: "http", scheme: "bearer", bearerFormat: "JWT"}}
      }
    }
    |> OpenApiSpex.resolve_schema_modules()
  end

  @doc "Account v2 operations, partitioned between session management and public tag lookups."
  @spec account_paths(:private | :public) :: Paths.t()
  def account_paths(visibility) when visibility in [:private, :public] do
    routes =
      AccountRouter
      |> Specs.routes_with_prefix("/account")
      |> Enum.filter(fn route ->
        String.starts_with?(route.path, "/account/v2/") and
          route.plug == TagsController == (visibility == :public)
      end)

    Enum.reduce(routes, Paths.from_routes(routes), fn route, paths ->
      if route.plug in [UserController, TagsController, AddressController, EmailController] do
        path = Regex.replace(~r/:([^\/]+)/, route.path, "{\\1}")
        item = Map.get(paths, path, %PathItem{})
        Map.put(paths, path, Map.put(item, route.verb, account_operation(route)))
      else
        paths
      end
    end)
  end

  defp account_operation(route) do
    {summary, schema, request_schema} = contract(route.plug, route.plug_opts)

    parameters =
      for [_, name] <- Regex.scan(~r/:([^\/]+)/, route.path) do
        schema =
          case name do
            "address_hash" -> General.AddressHash
            "transaction_hash" -> General.FullHash
            "api_key" -> %Schema{type: :string, format: :uuid}
            "id" -> %Schema{type: :integer, minimum: 1}
          end

        %Parameter{name: name, in: :path, required: true, schema: schema}
      end

    parameters =
      if route.plug == UserController and route.plug_opts in [:watchlist, :tags_address, :tags_transaction] do
        parameters ++
          [
            %Parameter{name: :id, in: :query, schema: %Schema{type: :integer, minimum: 1}},
            %Parameter{name: :items_count, in: :query, schema: %Schema{type: :integer, minimum: 0}}
          ]
      else
        parameters
      end

    success = response(summary, schema)

    success =
      if route.plug_opts == :get_csrf do
        %{
          success
          | headers: %{
              "x-bs-account-csrf" => %Header{
                description: "Session CSRF token; send as x-csrf-token on writes",
                schema: %Schema{type: :string}
              }
            }
        }
      else
        success
      end

    %Operation{
      operationId: "#{route.plug}.#{route.plug_opts}.#{route.verb}.#{route.path}",
      summary: summary,
      tags: ["account v2"],
      description: "Explorer account v2. Session-protected writes require cookies and x-csrf-token from get_csrf.",
      parameters: parameters,
      requestBody: if(request_schema, do: %RequestBody{required: true, content: json_content(request_schema)}),
      responses: %{
        200 => success,
        401 => response("Missing authenticated session", message()),
        403 =>
          response(
            "Rejected session or invalid CSRF token",
            object(%{message: %Schema{type: :string}, email: %Schema{type: :string, format: :email}}, [:message])
          ),
        404 => response("Account disabled, provider unconfigured, or record not found", message()),
        422 =>
          response("Invalid parameters or account validation errors", %Schema{
            oneOf: [
              message(),
              object(%{
                errors: %Schema{
                  type: :object,
                  additionalProperties: %Schema{type: :array, items: %Schema{type: :string}}
                }
              })
            ]
          }),
        429 =>
          response(
            "Email resend throttled",
            object(%{message: %Schema{type: :string}, seconds_before_next_resend: %Schema{type: :integer, minimum: 0}})
          ),
        500 => response("Provider or unexpected error", message())
      }
    }
  end

  defp contract(UserController, :info), do: {"Current user profile", Account.User, nil}
  defp contract(UserController, :get_csrf), do: {"Session CSRF token", message(), nil}
  defp contract(UserController, :watchlist), do: {"Watchlist addresses", page(watchlist()), nil}
  defp contract(UserController, :tags_address), do: {"Private address tags", page(address_tag()), nil}
  defp contract(UserController, :tags_transaction), do: {"Private transaction tags", page(transaction_tag()), nil}
  defp contract(UserController, :api_keys), do: {"API keys", array(api_key()), nil}
  defp contract(UserController, :custom_abis), do: {"Custom ABIs", array(custom_abi()), nil}

  defp contract(UserController, action)
       when action in [
              :delete_watchlist,
              :delete_tag_address,
              :delete_tag_transaction,
              :delete_api_key,
              :delete_custom_abi
            ],
       do: {"Delete owned account record", message(), nil}

  defp contract(UserController, action) when action in [:create_watchlist, :update_watchlist],
    do: {"Save watchlist address", watchlist(), watchlist_request()}

  defp contract(UserController, action) when action in [:create_tag_address, :update_tag_address],
    do:
      {"Save private address tag", address_tag(),
       tag_request(:address_hash, General.AddressHash, action == :create_tag_address)}

  defp contract(UserController, action) when action in [:create_tag_transaction, :update_tag_transaction],
    do:
      {"Save private transaction tag", transaction_tag(),
       tag_request(:transaction_hash, General.FullHash, action == :create_tag_transaction)}

  defp contract(UserController, action) when action in [:create_api_key, :update_api_key],
    do: {"Save API key", api_key(), object(%{name: %Schema{type: :string}})}

  defp contract(UserController, action) when action in [:create_custom_abi, :update_custom_abi] do
    properties = %{name: %Schema{type: :string}, contract_address_hash: General.AddressHash, abi: abi()}

    {"Save custom ABI", custom_abi(),
     object(properties, if(action == :create_custom_abi, do: Map.keys(properties), else: []))}
  end

  defp contract(TagsController, :tags_address), do: {"Tags attached to an address", tags(), nil}

  defp contract(TagsController, :tags_transaction) do
    schema = tags()

    properties =
      Map.put(schema.properties, :personal_transaction_tag, %{object(%{label: %Schema{type: :string}}) | nullable: true})

    {"Tags attached to a transaction", object(properties), nil}
  end

  defp contract(AddressController, :link_address),
    do:
      {"Link signed wallet to current user", Account.User,
       object(%{message: %Schema{type: :string}, signature: %Schema{type: :string}})}

  defp contract(EmailController, :link_email),
    do:
      {"Link verified email to current user", Account.User,
       object(%{email: %Schema{type: :string, format: :email}, otp: %Schema{type: :string}})}

  defp contract(EmailController, :resend_email), do: {"Resend verification email", message(), nil}

  defp object(properties), do: object(properties, Map.keys(properties))

  defp object(properties, required),
    do: %Schema{type: :object, properties: properties, required: required, additionalProperties: false}

  defp array(items), do: %Schema{type: :array, items: items}
  defp message, do: object(%{message: %Schema{type: :string}})
  defp json_content(schema), do: %{"application/json" => %MediaType{schema: schema}}
  defp response(description, schema), do: %Response{description: description, content: json_content(schema)}
  defp page(items), do: General.paginated_response(items: items, next_page_params_example: %{id: 1, items_count: 50})
  defp api_key, do: object(%{api_key: %Schema{type: :string, format: :uuid}, name: %Schema{type: :string}})
  defp abi, do: array(%Schema{type: :object, additionalProperties: true, description: "Solidity JSON ABI entry"})

  defp custom_abi do
    object(%{
      id: %Schema{type: :integer, minimum: 1},
      name: %Schema{type: :string},
      contract_address_hash: General.AddressHash,
      contract_address: Address,
      abi: abi()
    })
  end

  defp address_tag do
    object(%{
      id: %Schema{type: :integer, minimum: 1},
      name: %Schema{type: :string},
      address_hash: General.AddressHash,
      address: Address
    })
  end

  defp transaction_tag do
    object(%{id: %Schema{type: :integer, minimum: 1}, name: %Schema{type: :string}, transaction_hash: General.FullHash})
  end

  defp tag_request(hash_key, hash_schema, create?) do
    object(%{hash_key => hash_schema, :name => %Schema{type: :string}}, if(create?, do: [hash_key, :name], else: []))
  end

  defp tags do
    object(%{
      personal_tags: array(General.Tag),
      common_tags: array(General.Tag),
      watchlist_names: array(General.WatchlistName)
    })
  end

  defp notification_settings do
    direction = object(%{incoming: %Schema{type: :boolean}, outcoming: %Schema{type: :boolean}})
    groups = ["native", "ERC-20", "ERC-721", "ERC-404"]
    groups = if Application.get_env(:explorer, :chain_type) == :zilliqa, do: groups ++ ["ZRC-2"], else: groups
    object(Map.new(groups, &{&1, direction}))
  end

  defp watchlist_request do
    object(%{
      address_hash: General.AddressHash,
      name: %Schema{type: :string},
      notification_settings: notification_settings(),
      notification_methods: object(%{email: %Schema{type: :boolean}})
    })
  end

  defp watchlist do
    properties =
      Map.merge(watchlist_request().properties, %{
        id: %Schema{type: :integer, minimum: 1},
        address: Address,
        address_balance: General.IntegerStringNullable,
        exchange_rate: %Schema{type: :string, nullable: true, pattern: General.float_pattern()},
        tokens_fiat_value: %Schema{type: :string, nullable: true, pattern: General.float_pattern()},
        tokens_count: %Schema{type: :integer, nullable: true, minimum: 0, maximum: 150},
        tokens_overflow: %Schema{type: :boolean, nullable: true}
      })

    object(properties)
  end
end
