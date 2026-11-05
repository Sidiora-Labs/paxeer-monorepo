defmodule BlockscoutClient.ContractState do
  import Ecto.Query

  def export(origin, output, source_revision) do
    unless Regex.match?(~r/\A[0-9a-f]{40}\z/, source_revision) do
      raise "source_revision must be the running backend's full Git SHA"
    end

    blocks =
      Explorer.Repo.all(
        from(block in Explorer.Chain.Block,
          where: block.consensus == true,
          order_by: [desc: block.number],
          limit: 51,
          select: %{height: block.number, hash: block.hash, parent_hash: block.parent_hash}
        )
      )

    if length(blocks) > 50 do
      raise "use a controlled database with at most 50 consensus blocks"
    end

    %{blocks: health} = BlockScoutWeb.API.HealthController.get_indexing_status()

    state = %{
      origin: origin,
      source_revision: source_revision,
      health: health,
      blocks:
        Enum.map(blocks, fn block ->
          %{height: block.height, hash: to_string(block.hash), parent_hash: to_string(block.parent_hash)}
        end)
    }

    File.write!(output, Jason.encode!(state, pretty: true))
  end
end
