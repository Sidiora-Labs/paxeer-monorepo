Code.require_file("../../utils/test/suite_helper.exs", __DIR__)
Backend.TestSuite.restore_configuration_after_suite()

{:ok, _} = Application.ensure_all_started(:image)
ExUnit.start()
