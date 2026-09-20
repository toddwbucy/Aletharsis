"""Explicit offline reproduction on Linux with bubblewrap and user systemd.

Provisioning is separate: supply the pinned archive, Go distribution and an
already downloaded x/text v0.14.0 module cache. This script never downloads.
"""
import argparse
import hashlib
import json
from pathlib import Path
import shutil
import subprocess
import tarfile

ARCHIVE_SHA256 = "e66fb4ff79e9807104620ae4797b6ad2ed17d097b85f9001eb086a81e487ce26"
COMMIT = "ad4eaee3705ea5edb610ab37041be496d012e583"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("archive", "go", "modules", "work"):
        parser.add_argument("--" + name, required=True, type=Path)
    args = parser.parse_args()
    if args.archive.stat().st_size > 100 * 1024**2:
        raise ValueError("archive exceeds provisioning limit")
    if hashlib.sha256(args.archive.read_bytes()).hexdigest() != ARCHIVE_SHA256:
        raise ValueError("archive identity mismatch")
    work = args.work.resolve()
    work.mkdir(exist_ok=False)
    (work / "results").mkdir()
    with tarfile.open(args.archive) as archive:
        members = archive.getmembers()
        if sum(m.size for m in members) > 64 * 1024**2 or not all(m.isfile() or m.isdir() for m in members):
            raise ValueError("unsafe or oversized source archive")
        archive.extractall(work, filter="data")
    source = work / ("c2pa-text-" + COMMIT)
    shutil.copytree(Path(__file__).parent, work / "probe", ignore=shutil.ignore_patterns("__pycache__"))
    namespace = ["bwrap", "--unshare-all", "--die-with-parent", "--new-session", "--cap-drop", "ALL", "--clearenv"]
    for path in ("/usr", "/lib", "/lib64"):
        namespace += ["--ro-bind", path, path]
    namespace += ["--symlink", "usr/bin", "/bin", "--proc", "/proc", "--dev", "/dev", "--tmpfs", "/tmp",
                  "--ro-bind", str(args.go.resolve()), "/toolchain", "--ro-bind", str(source), "/source",
                  "--bind", str(work), "/work", "--ro-bind", str(args.modules.resolve()), "/modules"]
    for key, value in {"PATH": "/toolchain/bin:/usr/bin:/bin", "GOTOOLCHAIN": "local", "GOPROXY": "off", "GOSUMDB": "off",
                       "GOMODCACHE": "/modules", "GOCACHE": "/work/cache", "CGO_ENABLED": "0",
                       "PYTHONDONTWRITEBYTECODE": "1", "GOMEMLIMIT": "700MiB"}.items():
        namespace += ["--setenv", key, value]

    def execute(name, command):
        argv = ["systemd-run", "--user", "--wait", "--pipe", "--collect", "-p", "MemoryMax=1G",
                "-p", "MemorySwapMax=0", "-p", "CPUQuota=100%", "-p", "RuntimeMaxSec=300", *namespace, *command]
        (work / "results" / (name + ".argv.json")).write_text(json.dumps(argv, indent=2) + "\n")
        with (work / "results" / (name + ".stdout")).open("wb") as out, (work / "results" / (name + ".stderr")).open("wb") as err:
            subprocess.run(argv, stdout=out, stderr=err, timeout=330, check=True)

    execute("build", ["--chdir", "/work/probe", "/bin/sh", "-c",
        "go version && go mod edit -replace github.com/encypherai/c2pa-text/go/v3=/source/go && "
        "go mod tidy && go mod verify && go build -p=1 -trimpath -o /work/probe.bin . && "
        "cd /source/go && go test -p=1 -count=1 -timeout=60s -json ./... > /work/results/upstream-tests.jsonl"])
    execute("cases", ["/usr/bin/python3", "/work/probe/run_cases.py", "/work/probe.bin", "/work/cases"])
    used = sum(p.stat().st_size for p in work.rglob("*") if p.is_file())
    if used > 2 * 1024**3:
        raise ValueError("retained disk budget exceeded")
    print(work / "cases/results.json")


if __name__ == "__main__":
    main()
