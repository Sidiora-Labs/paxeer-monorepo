defmodule Explorer.Chain.PaxeerX.Finality.Heights do
  @moduledoc false

  @type t :: %__MODULE__{committed_height: non_neg_integer() | nil}

  defstruct committed_height: nil
end

defmodule Explorer.Chain.PaxeerX.Finality do
  @moduledoc false

  import Ecto.Query, only: [from: 2]

  alias Explorer.Chain
  alias Explorer.Chain.Block
  alias Explorer.Chain.PaxeerX.Finality.Heights

  @spec heights(keyword()) :: Heights.t()
  def heights(options \\ []) do
    committed_height = Keyword.get_lazy(options, :committed_height, fn -> committed_height(options) end)

    case committed_height do
      nil -> %Heights{}
      height when is_integer(height) and height >= 0 -> %Heights{committed_height: height}
      _invalid -> raise ArgumentError, "invalid committed height"
    end
  end

  defp committed_height(options) do
    query = from(block in Block, where: block.consensus == true, select: max(block.number))
    Chain.select_repo(options).one(query)
  end
end
