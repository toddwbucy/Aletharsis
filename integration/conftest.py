"""Black-box checks of an explicitly supplied executable; no auditor imports."""
import json
from pathlib import Path
import subprocess
import sys

import jsonschema
import pytest

ROOT = Path(__file__).resolve().parents[1]
PROCESS_TIMEOUT = 10


def pytest_addoption(parser):
    parser.addoption('--aletharsis-binary', help='Path to the compiled Go executable')


@pytest.fixture(scope='session')
def binary(pytestconfig):
    if sys.platform != 'linux':
        pytest.skip('Executable acquisition contract currently supports Linux only')
    supplied = pytestconfig.getoption('--aletharsis-binary')
    if not supplied:
        pytest.fail('Build Go and supply --aletharsis-binary; no PATH fallback is allowed')
    path = Path(supplied).resolve()
    if not path.is_file():
        pytest.fail(f'Candidate executable missing: {path}')
    return path


@pytest.fixture(scope='session')
def run_cli(binary):
    def run(*args, cwd=ROOT, **options):
        kwargs = {'stdout': subprocess.PIPE, 'stderr': subprocess.PIPE,
                  'timeout': PROCESS_TIMEOUT, 'check': False, **options}
        try:
            return subprocess.run([str(binary), *map(str, args)], cwd=cwd, **kwargs)
        except subprocess.TimeoutExpired:
            pytest.fail(f'CLI exceeded {PROCESS_TIMEOUT}s: {args!r}')
    return run


@pytest.fixture(scope='session')
def validator():
    # This is the live contract. The reference/ schema remains an immutable
    # migration oracle and must not silently substitute for this schema.
    schema = json.loads((ROOT / 'schemas/report.schema.json').read_bytes())
    cls = jsonschema.validators.validator_for(schema)
    cls.check_schema(schema)
    return cls(schema)
