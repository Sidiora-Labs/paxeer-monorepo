def pytest_configure(config):
    config.addinivalue_line("python_files", "durable_release_evidence.py")
