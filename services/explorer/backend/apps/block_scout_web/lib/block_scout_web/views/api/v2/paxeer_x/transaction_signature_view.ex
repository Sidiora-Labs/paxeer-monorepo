defmodule BlockScoutWeb.API.V2.PaxeerX.TransactionSignatureView do
  @moduledoc false

  use BlockScoutWeb, :view

  def render("signature.json", %{rung: rung, receipt: receipt, receipt_key: key, signature: signature}) do
    %{
      "rung" => to_string(rung),
      "receipt" => hex(receipt),
      "receipt_key" => %{"key_id" => hex(key.key_id), "public_key" => hex(key.public_key)},
      "signature" => hex(signature)
    }
  end

  defp hex(bytes), do: "0x" <> Base.encode16(bytes, case: :lower)
end
