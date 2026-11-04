
ProgramLifecycleOperation: TypeAlias = Literal["program.deploy", "program.upgrade", "program.wind-down"]
PROGRAM_LIFECYCLE_METADATA_JSON: str
PROGRAM_LIFECYCLE_TRANSPORT: dict[str, object]
PROGRAM_LIFECYCLE_REQUEST_JSON: str
def program_lifecycle_request(operation: ProgramLifecycleOperation, signed_activity: str, request_id: int) -> str: ...
