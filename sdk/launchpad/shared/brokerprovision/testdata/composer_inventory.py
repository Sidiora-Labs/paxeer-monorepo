"""Read broker bindings from composed launchpad example documents."""

import importlib
import json
from pathlib import Path
import sys


def inventory(root: Path) -> dict:
    directory = root / "deploy/launchpad/documents"
    sys.path.insert(0, str(directory))
    modules = {
        "api": "api", "api-sockets": "api_sockets", "charts": "charts",
        "gateway": "gateway", "indexer": "indexer", "livestream": "livestream",
        "metadata": "metadata", "pnl": "pnl", "ranking": "ranking",
        "social": "social", "stats": "stats", "user": "user",
    }
    docs = {service: importlib.import_module(module).documents(service, directory / "values")
            for service, module in modules.items()}
    docs = {service: {key: json.loads(value) if isinstance(value, bytes) else value
                      for key, value in document.items()}
            for service, document in docs.items()}
    consumers = []
    streams = {}

    def stream(name, subjects):
        streams.setdefault(name, set()).update(subjects)

    routes = docs["api"]["STREAM_RELAY_CONFIG_DOCUMENT"]["routes"]
    for route in routes:
        stream(route["stream"], route["subjects"])

    def matches(pattern, subject):
        expected, actual = pattern.split("."), subject.split(".")
        for index, token in enumerate(expected):
            if token == ">":
                return len(actual) > index
            if index >= len(actual) or (token != "*" and token != actual[index]):
                return False
        return len(expected) == len(actual)

    def walk(value):
        if isinstance(value, dict):
            if {"Stream", "Durable", "Subject", "Batch"} <= value.keys():
                consumers.append(value)
                if not any(matches(pattern, value["Subject"])
                           for pattern in streams.get(value["Stream"], ())):
                    stream(value["Stream"], [value["Subject"]])
            else:
                for child in value.values():
                    walk(child)
        elif isinstance(value, list):
            for child in value:
                walk(child)

    for doc in docs.values():
        walk(doc)
    indexer = docs["indexer"]["DELIVERY_CONFIG_DOCUMENT"]
    stream(indexer["Stream"], indexer["Policy"]["Subjects"])
    charts = docs["charts"]["DELIVERY_CONFIG_DOCUMENT"]["chartOutbox"]
    stream(charts["stream"], [charts["subject"]])
    stats = docs["stats"]["DELIVERY_CONFIG_DOCUMENT"]["phase2"]
    stream(stats["outputStream"], [stats["statsSubject"], stats["searchInvalidationSubject"]])

    def fixed(name, durable, subject):
        consumers.append(dict(Stream=name, Durable=durable, Subject=subject,
                              Batch=32, MaxAckPending=128, MaxWaiting=4,
                              MaxDeliver=10, FetchWait=500000000, AckWait=30000000000))
        stream(name, [subject])

    # socialConnectedRuntime and newGatewayConnectedRuntime use these fixed bounds.
    social = docs["social"]["SOCIAL_RUNTIME_DOCUMENT"]
    for kind in ("chat", "erasure", "policy"):
        fixed(social[kind + "Stream"], social[kind + "Durable"], social[kind + "Subject"])
    stream(social["eventStream"], [social["erasureSubject"], social["policySubject"],
                                 social["mentionSubject"], *social["interactionSubjects"].values()])
    gateway = docs["gateway"]["GATEWAY_RUNTIME_DOCUMENT"]
    fixed(gateway["stream"], gateway["durable"], "identity.account.v1")
    audit = gateway["audit"]
    fixed(audit["stream"], audit["durable"], audit["subject"])
    ordered_streams = {name: sorted(subjects) for name, subjects in streams.items()}
    for route in routes:
        ordered_streams[route["stream"]] = list(route["subjects"]) + sorted(
            streams[route["stream"]] - set(route["subjects"]))
    return {"streams": ordered_streams, "durables": consumers}


if __name__ == "__main__":
    print(json.dumps(inventory(Path(sys.argv[1]))))
