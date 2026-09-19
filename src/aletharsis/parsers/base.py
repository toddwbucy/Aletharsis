from typing import Protocol

from ..models import DocumentEvidence


class Parser(Protocol):
    name: str

    def parse(self, data: bytes) -> DocumentEvidence: ...
