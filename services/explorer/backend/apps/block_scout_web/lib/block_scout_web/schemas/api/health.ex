defmodule BlockScoutWeb.Schemas.API.Health do
  @moduledoc false

  alias OpenApiSpex.Schema

  def error do
    %Schema{
      type: :object,
      properties: %{code: %Schema{type: :integer}, message: %Schema{type: :string}},
      required: [:code, :message],
      additionalProperties: false
    }
  end

  def indexing(healthy) do
    %Schema{
      type: :object,
      properties: %{
        healthy: %Schema{type: :boolean, enum: [healthy]},
        metadata: %Schema{
          type: :object,
          properties: %{blocks: blocks(healthy), batches: batches()},
          required: [:blocks],
          additionalProperties: false
        },
        error: if(healthy, do: %Schema{nullable: true, enum: [nil]}, else: error())
      },
      required: [:healthy, :metadata, :error],
      additionalProperties: false
    }
  end

  defp blocks(healthy) do
    properties = %{
      healthy: %Schema{type: :boolean, enum: [healthy]},
      latest_block: %Schema{
        type: :object,
        properties: %{
          db: timestamped_number(),
          cache: timestamped_number(),
          node: %Schema{
            type: :object,
            properties: %{number: number()},
            required: [:number],
            additionalProperties: false
          }
        },
        required: [:db, :cache, :node],
        additionalProperties: false
      }
    }

    %Schema{
      type: :object,
      properties: if(healthy, do: properties, else: Map.put(properties, :error, error())),
      required: if(healthy, do: [:healthy, :latest_block], else: [:healthy, :latest_block, :error]),
      additionalProperties: false
    }
  end

  defp batches do
    %Schema{
      oneOf:
        for healthy <- [true, false] do
          properties = %{
            healthy: %Schema{type: :boolean, enum: [healthy]},
            latest_batch: %Schema{
              type: :object,
              properties: %{
                db: %Schema{
                  type: :object,
                  properties: %{
                    number: number(),
                    timestamp: %Schema{type: :string},
                    average_batch_time: %Schema{type: :string}
                  },
                  required: [:number, :timestamp, :average_batch_time],
                  additionalProperties: false
                }
              },
              required: [:db],
              additionalProperties: false
            }
          }

          %Schema{
            type: :object,
            properties: if(healthy, do: properties, else: Map.put(properties, :error, error())),
            required: if(healthy, do: [:healthy, :latest_batch], else: [:healthy, :latest_batch, :error]),
            additionalProperties: false
          }
        end
    }
  end

  defp number, do: %Schema{type: :string, pattern: "^[0-9]*$"}

  defp timestamped_number do
    %Schema{
      type: :object,
      properties: %{number: number(), timestamp: %Schema{type: :string}},
      required: [:number, :timestamp],
      additionalProperties: false
    }
  end
end

defmodule BlockScoutWeb.Schemas.API.Health.Healthy do
  @moduledoc false
  require OpenApiSpex
  OpenApiSpex.schema(BlockScoutWeb.Schemas.API.Health.indexing(true), struct?: false)
end

defmodule BlockScoutWeb.Schemas.API.Health.Unhealthy do
  @moduledoc false
  require OpenApiSpex
  OpenApiSpex.schema(BlockScoutWeb.Schemas.API.Health.indexing(false), struct?: false)
end

defmodule BlockScoutWeb.Schemas.API.Health.Standalone do
  @moduledoc false
  require OpenApiSpex
  alias OpenApiSpex.Schema

  OpenApiSpex.schema(
    %Schema{
      type: :object,
      description: "Standalone media worker health; no blockchain indexing progress is reported.",
      properties: %{
        healthy: %Schema{type: :boolean, enum: [true]},
        data: %Schema{type: :object, properties: %{}, additionalProperties: false}
      },
      required: [:healthy, :data],
      additionalProperties: false
    },
    struct?: false
  )
end

defmodule BlockScoutWeb.Schemas.API.Health.Success do
  @moduledoc false
  require OpenApiSpex
  alias BlockScoutWeb.Schemas.API.Health

  OpenApiSpex.schema(%OpenApiSpex.Schema{oneOf: [Health.Healthy, Health.Standalone]}, struct?: false)
end
