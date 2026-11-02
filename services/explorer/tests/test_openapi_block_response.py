from pathlib import Path

import yaml


def test_block_response_declares_every_required_property():
    root = Path(__file__).resolve().parents[3]
    public_yaml = (root / "apps/explorer/public/openapi.yaml").read_text(
        encoding="utf-8"
    )
    swagger_yaml = (root / "services/explorer/swagger.yaml").read_text(
        encoding="utf-8"
    )
    public_spec = yaml.safe_load(public_yaml)
    swagger_spec = yaml.safe_load(swagger_yaml)

    assert public_yaml == swagger_yaml
    assert public_spec == swagger_spec

    schemas = public_spec["components"]["schemas"]
    for name, schema in schemas.items():
        if schema.get("additionalProperties") is False:
            properties = schema.get("properties", {})
            for required in schema.get("required", []):
                assert required in properties, f"{name} does not declare {required}"

    block_properties = schemas["BlockResponse"]["properties"]
    price_update_count = block_properties["price_update_count"]
    assert price_update_count["type"] == "integer"
    assert price_update_count["nullable"] is False
    assert "$ref" not in price_update_count
    assert block_properties["priority_fee"] == {
        "$ref": "#/components/schemas/IntegerStringNullable"
    }
