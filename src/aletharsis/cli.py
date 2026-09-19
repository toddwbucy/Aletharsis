import argparse
import json
import logging
from pathlib import Path
import sys

from . import __version__
from .audit import audit
from .safety import escaped
from .reporters import console
from .reporters import json as json_report

LOGGER = logging.getLogger("aletharsis")
VIEWS = {"unicode": {"unicode", "possible_steganography"},
         "metadata": {"metadata", "identifier", "provenance"},
         "structure": {"document_structure", "embedded_content", "hidden_content", "visual_watermark"}}


class ArgumentParser(argparse.ArgumentParser):
    def error(self, message: str) -> None:
        self.print_usage(sys.stderr)
        self.exit(4, f"{self.prog}: error: {escaped(message)}\n")


def build_parser() -> argparse.ArgumentParser:
    parser = ArgumentParser(prog="aletharsis", description="Read-only text and Unicode forensic auditing (M0/M1)")
    parser.add_argument("--version", action="version", version=f"aletharsis {__version__}")
    commands = parser.add_subparsers(dest="command", required=True)
    for name in ("audit", "unicode", "metadata", "structure"):
        command = commands.add_parser(name, help=f"{name} findings for a single file")
        command.add_argument("file", type=Path)
        command.add_argument("--json", action="store_true", help="emit complete JSON evidence")
        command.add_argument("--verbose", action="store_true", help="show detailed evidence and structured diagnostics")
        command.add_argument("--output", type=Path, help="write JSON to a new report file (never overwrite)")
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    if args.verbose:
        logging.basicConfig(level=logging.INFO, format="%(message)s")
    report = audit(args.file)
    if args.command in VIEWS:
        report.findings = [f for f in report.findings if f.category in VIEWS[args.command] or f.category == "parser"]
        report.limitations.append(f"This is the {args.command} finding view; summary and exit code apply to this view. Full extracted evidence is retained.")
    LOGGER.info(json.dumps({"event": "audit_completed", "path": str(args.file),
                            "status": report.status, "exit_code": report.exit_code}, ensure_ascii=True, sort_keys=True))
    rendered = json_report.render(report) if args.json or args.output else console.render(report, verbose=args.verbose)
    if args.output:
        try:
            # O_EXCL rejects existing sources, hard links and symlinks alike.
            with args.output.open("x", encoding="utf-8", newline="\n") as stream:
                stream.write(rendered)
        except OSError as exc:
            print(f"aletharsis: could not create report: {escaped(str(exc))}", file=sys.stderr)
            return 4
    else:
        try:
            sys.stdout.write(rendered)
        except BrokenPipeError:
            return 4
    return report.exit_code
