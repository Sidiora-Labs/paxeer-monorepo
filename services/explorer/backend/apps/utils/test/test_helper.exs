Code.require_file("suite_helper.exs", __DIR__)
Backend.TestSuite.restore_configuration_after_suite()

{:ok, _} = Application.ensure_all_started(:credo)
