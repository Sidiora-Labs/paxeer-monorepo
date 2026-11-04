"""Local calibration messages checked against the current frozen v1 bundle."""
from jsonschema import Draft202012Validator, FormatChecker
from phase2_types import ROOT, bound_document

CONTRACTS = ROOT / "sdk/launchpad/packages/production-contracts/src/schemas.gen.json"
CONTRACTS_SHA256 = "a2379b5862a29881c7fd9e764cd7c64ef3b92dc033986300fcca92606e8ff847"
MESSAGES = {"IF_SUBSCRIBE": "Subscribe", "IF_STREAM_EVENT": "StreamEvent", "IF_STREAM_CONTROL": "StreamControl"}


def validate(name, value):
    if name not in MESSAGES:
        raise ValueError("unsupported frozen message")
    definitions = bound_document(CONTRACTS, CONTRACTS_SHA256)
    schema = {**definitions, "$ref": "#/$defs/" + MESSAGES[name]}
    Draft202012Validator(schema, format_checker=FormatChecker()).validate(value)
    return value


def subscribe(topics, subscription_id="load-1", after_cursor=None):
    return validate("IF_SUBSCRIBE", dict(op="subscribe", subscriptionId=subscription_id, topics=topics, afterCursor=after_cursor))
