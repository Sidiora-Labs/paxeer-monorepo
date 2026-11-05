
import json as _lifecycle_json

PROGRAM_LIFECYCLE_METADATA_JSON = '{{LIFECYCLE_METADATA}}'
PROGRAM_LIFECYCLE_TRANSPORT = _lifecycle_json.loads(PROGRAM_LIFECYCLE_METADATA_JSON)
ProgramLifecycleOperation = Literal["program.deploy", "program.upgrade", "program.wind-down"]
PROGRAM_LIFECYCLE_REQUEST_JSON = '{"jsonrpc":"2.0","id":{{request_id}},"method":"eth_sendRawTransaction","params":["{{signed_activity}}"]}'

def program_lifecycle_request(operation: ProgramLifecycleOperation, signed_activity: str, request_id: int) -> str:
    # The caller binds and signs exact ABI calldata; RPC acceptance establishes submission only.
    if operation not in PROGRAM_LIFECYCLE_TRANSPORT["operations"]:
        raise ValueError("invalid lifecycle operation")
    if type(request_id) is not int or not 1 <= request_id <= 9007199254740991:
        raise ValueError("invalid request id")
    if not isinstance(signed_activity, str) or not signed_activity.startswith("0x02") or not 6 <= len(signed_activity) <= 2097154 or len(signed_activity) % 2 or any(c not in "0123456789abcdef" for c in signed_activity[2:]):
        raise ValueError("invalid signed EIP-1559 transaction")
    return PROGRAM_LIFECYCLE_REQUEST_JSON.replace("{{request_id}}", str(request_id)).replace("{{signed_activity}}", signed_activity)
