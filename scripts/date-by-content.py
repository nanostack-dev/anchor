#!/usr/bin/env python3
"""Dates every tracked file and directory by its content.

Why: Go's test cache recognizes a file or directory that a test opens by its
size and modification time. A fresh checkout gives everything a new time, so
CI could never replay a cached result for a test that reads the migrations or
the OpenAPI spec. This script sets each file's time from its git blob hash and
each directory's time from its git tree hash: the time changes exactly when
the content does (docs/development/ci.md).

Usage: python3 scripts/date-by-content.py   (in the repository root, after checkout)
"""
import os
import subprocess

# The times fall between 2001 and 2023: Go never caches a file newer than now.
EARLIEST = 1_000_000_000
SPAN = 700_000_000


def time_for(object_id: str) -> int:
    """A stable past timestamp derived from a git object id."""
    return EARLIEST + int(object_id[:8], 16) % SPAN


def git(*args: str) -> list[bytes]:
    """Runs git and returns its NUL-separated output records."""
    output = subprocess.run(["git", *args], check=True, capture_output=True).stdout
    return [record for record in output.split(b"\0") if record]


def set_time(path: bytes, object_id: str) -> None:
    timestamp = time_for(object_id)
    os.utime(path, (timestamp, timestamp))


def date_files() -> None:
    # Each record is "<mode> <blob id> <stage>\t<path>".
    for record in git("ls-files", "-s", "-z"):
        meta, path = record.split(b"\t", 1)
        if os.path.isfile(path) and not os.path.islink(path):
            set_time(path, meta.split()[1].decode())


def date_directories() -> None:
    # Each record is "<mode> tree <tree id>\t<path>". Deepest first: dating a
    # child never moves its parent's time, but writing into a parent would.
    trees = [record.split(b"\t", 1) for record in git("ls-tree", "-r", "-d", "-z", "HEAD")]
    for meta, path in sorted(trees, key=lambda tree: -tree[1].count(b"/")):
        if os.path.isdir(path):
            set_time(path, meta.split()[2].decode())


if __name__ == "__main__":
    date_files()
    date_directories()
