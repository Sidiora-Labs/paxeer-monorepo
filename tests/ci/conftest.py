"""Declare CI checks that require native toolchains or host privileges."""


def pytest_configure(config):
    config.addinivalue_line(
        "markers", "native: requires native toolchains, services or host privileges"
    )
