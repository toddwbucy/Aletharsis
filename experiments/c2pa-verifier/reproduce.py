"""Offline Linux reproduction with supplied pinned sources/toolchains/cache.

No provisioning/network requests are made. Requires bwrap and user systemd.
"""
import argparse
import hashlib
import json
from pathlib import Path
import shutil
import subprocess
import tarfile

COMMIT = "da85f07f2e0d084854be30dd3dc89dc0d2a9cde9"
ARCHIVE_SHA256 = "9dcc42ea327ae057b7e8d3a2e338dbb540ddd005ed0042e2931cc6f8953aace8"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("archive", "rust", "cargo-home", "go", "work"):
        parser.add_argument("--" + name, required=True, type=Path)
    args = parser.parse_args()
    if args.archive.stat().st_size > 100 * 1024**2 or hashlib.sha256(args.archive.read_bytes()).hexdigest() != ARCHIVE_SHA256:
        raise ValueError("archive size or identity mismatch")
    work = args.work.resolve()
    work.mkdir(exist_ok=False)
    (work / "results").mkdir()
    with tarfile.open(args.archive) as archive:
        members = archive.getmembers()
        if sum(m.size for m in members) > 256 * 1024**2 or not all(m.isfile() or m.isdir() for m in members):
            raise ValueError("unsafe source archive")
        archive.extractall(work, filter="data")
    source = work / ("encypher-c2pa-" + COMMIT)
    # Re-extract registry packages offline from provisioned archives, rather than
    # trusting a caller's previously expanded crate source tree.
    for part in ("cache", "index"):
        shutil.copytree(args.cargo_home / "registry" / part, work / "cargo/registry" / part)
    shutil.copytree(source / "bindings/go", work / "bindings/go")
    shutil.copytree(source / "bindings/c/include", work / "bindings/c/include")
    shutil.copytree(Path(__file__).parent, work / "probe", ignore=shutil.ignore_patterns("__pycache__"))
    sandbox = ["bwrap", "--unshare-all", "--die-with-parent", "--new-session", "--cap-drop", "ALL", "--clearenv"]
    for path in ("/usr", "/lib", "/lib64"):
        sandbox += ["--ro-bind", path, path]
    sandbox += ["--symlink", "usr/bin", "/bin", "--proc", "/proc", "--dev", "/dev", "--tmpfs", "/tmp",
                "--ro-bind", str(args.rust.resolve()), "/rust", "--ro-bind", str(args.go.resolve()), "/go",
                "--ro-bind", str(source), "/source", "--bind", str(work), "/work"]
    for key, value in {"PATH": "/rust/bin:/go/bin:/usr/bin:/bin", "CARGO_HOME": "/work/cargo",
                       "CARGO_TARGET_DIR": "/work/target", "GOTOOLCHAIN": "local", "GOPROXY": "off", "GOSUMDB": "off",
                       "GOMODCACHE": "/work/modules", "GOCACHE": "/work/gocache", "CGO_ENABLED": "1",
                       "PYTHONDONTWRITEBYTECODE": "1"}.items():
        sandbox += ["--setenv", key, value]

    def execute(name, command, limit):
        argv = ["systemd-run", "--user", "--wait", "--pipe", "--collect", "-p", "MemoryMax=2G", "-p", "MemorySwapMax=0",
                "-p", "CPUQuota=100%", "-p", f"RuntimeMaxSec={limit}", *sandbox, *command]
        (work / "results" / (name + ".argv.json")).write_text(json.dumps(argv, indent=2) + "\n")
        with (work / "results" / (name + ".stdout")).open("wb") as out, (work / "results" / (name + ".stderr")).open("wb") as err:
            subprocess.run(argv, stdout=out, stderr=err, timeout=limit + 30, check=True)
        if sum(p.stat().st_size for p in work.rglob("*") if p.is_file()) > 8 * 1024**3:
            raise ValueError("retained storage budget exceeded")

    execute("build", ["--chdir", "/source", "/bin/sh", "-c",
                      "rustc --version && cargo --version && cargo build --frozen --release --jobs 1 -p encypher-c2pa-ffi -p encypher-c2pa-cli"], 1800)
    execute("go-build", ["--chdir", "/work/probe", "/bin/sh", "-c",
                         "go version && go mod edit -replace github.com/encypherai/encypher-c2pa/bindings/go=/work/bindings/go && "
                         "go mod tidy && go build -p=1 -trimpath -o /work/probe.bin ."], 300)
    execute("cases", ["/usr/bin/python3", "/work/probe/run_cases.py", "/work/probe.bin", "/source", "/work/cases"], 600)
    print(work / "cases/results.json")


if __name__ == "__main__":
    main()
