"""Declare qualification cases that need the node's native libraries."""


def pytest_configure(config):
    config.addinivalue_line(
        "markers", "native: compiles node tests against the native libraries"
    )
