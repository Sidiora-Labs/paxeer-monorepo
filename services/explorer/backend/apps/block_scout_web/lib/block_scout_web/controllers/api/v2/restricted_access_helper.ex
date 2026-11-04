defmodule BlockScoutWeb.API.V2.RestrictedAccessHelper do
  @moduledoc false

  alias BlockScoutWeb.AccessHelper

  @spec restricted_access?(binary(), nil | map()) :: {:ok, false} | {:restricted_access, true}
  def restricted_access?(address_hash, %{key: key} = params) do
    AccessHelper.restricted_access?(address_hash, Map.put(params, "key", key))
  end

  def restricted_access?(address_hash, params) do
    AccessHelper.restricted_access?(address_hash, params)
  end
end
