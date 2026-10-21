"""Source snapshots and resource limits shared by local test entry points."""
import hashlib
import os
from pathlib import Path
import shutil
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "scripts"))
from test_report import RunDirectory


def source_files(root):
    files = {}
    for current, directories, names in os.walk(root):
        relative = Path(current).relative_to(root)
        directories[:] = [n for n in directories if n not in {".git", ".gosim", "__pycache__"}
                          and not (relative == Path(".") and n in {"docs", "test-reports"})]
        for name in names:
            path = Path(current) / name
            rel = path.relative_to(root).as_posix()
            if name == ".git" or name.endswith(".md"):
                continue
            if rel == "internal/compat/profiles/evidence.json":
                continue
            if path.is_symlink() or not path.is_file():
                raise RuntimeError(f"unsupported source input {rel}")
            files[rel] = hashlib.sha256(path.read_bytes()).hexdigest()
    return files


def snapshot(root, destination, expected):
    """Copy source without changing compatibility or evidence metadata."""
    shutil.copytree(root, destination,
                    ignore=shutil.ignore_patterns(".git", ".gosim", "test-reports", "__pycache__"))
    if source_files(destination) != expected:
        raise RuntimeError("source changed while copying validation workspace")
    return {"input_count": len(expected)}


def memory_limit():
    if sys.platform == "darwin":
        return int(subprocess.check_output(["sysctl", "-n", "hw.memsize"], text=True))
    return os.sysconf("SC_PAGE_SIZE") * os.sysconf("SC_PHYS_PAGES")
