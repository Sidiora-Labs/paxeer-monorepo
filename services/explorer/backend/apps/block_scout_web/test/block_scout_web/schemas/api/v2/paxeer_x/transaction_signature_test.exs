defmodule BlockScoutWeb.Schemas.API.V2.PaxeerX.TransactionSignatureTest do
  use ExUnit.Case, async: true

  alias BlockScoutWeb.Schemas.API.V2.PaxeerX.TransactionSignature
  alias OpenApiSpex.{Components, Info, OpenApi}
  alias OpenApiSpex.Cast.Error

  test "signature_schema_requires_64_byte_signature" do
    signature_schema_requires_64_byte_signature()
  end

  def signature_schema_requires_64_byte_signature do
    spec =
      OpenApiSpex.resolve_schema_modules(%OpenApi{
        info: %Info{title: "Transaction signature schema test", version: "1"},
        paths: %{},
        components: %Components{schemas: %{"PaxeerXTransactionSignature" => TransactionSignature.schema()}}
      })

    schema = spec.components.schemas["PaxeerXTransactionSignature"]
    signature = "0x" <> String.duplicate("ab", 64)
    key_id = "0x" <> String.duplicate("cd", 32)
    public_key = "0x" <> String.duplicate("ef", 32)

    body = %{
      "rung" => "receipted",
      "receipt" => "0x01020304",
      "receipt_key" => %{"key_id" => key_id, "public_key" => public_key},
      "signature" => signature
    }

    assert schema.required == [:rung, :receipt, :receipt_key, :signature]
    assert schema.additionalProperties == false
    assert schema.properties.receipt_key.required == [:key_id, :public_key]
    assert schema.properties.receipt_key.additionalProperties == false

    assert {:ok, cast} = OpenApiSpex.cast_value(body, schema, spec)
    assert cast.rung == "receipted"
    assert cast.receipt == body["receipt"]
    assert cast.receipt_key == %{key_id: key_id, public_key: public_key}
    assert cast.signature == signature

    short_signature = "0x" <> String.duplicate("ab", 63)
    invalid_body = Map.put(body, "signature", short_signature)

    assert {:error, [%Error{reason: :invalid_format, path: [:signature], format: pattern, value: ^short_signature}]} =
             OpenApiSpex.cast_value(invalid_body, schema, spec)

    assert Regex.source(pattern) == schema.properties.signature.pattern
  end
end
