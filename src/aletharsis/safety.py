"""Safe presentation of untrusted text evidence."""


def escaped(text: str) -> str:
    """Escape controls, bidi characters and newlines for terminal display."""
    return text.encode("unicode_escape").decode("ascii")
