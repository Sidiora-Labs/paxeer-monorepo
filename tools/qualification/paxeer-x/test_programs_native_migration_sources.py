import pytest

import programs_native_migration as producer


@pytest.mark.parametrize('source_path', producer.SOURCE_PATHS)
def test_hashed_source_exists(source_path):
    assert (producer.ROOT / source_path).is_file(), source_path
