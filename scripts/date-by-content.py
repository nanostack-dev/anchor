#!/usr/bin/env python3
"""Dates each tracked file by its git blob hash and each directory by its tree
hash, so a modification time changes exactly when the content does.

Go's test cache knows a file or directory a test opens by its modification
time, and a fresh checkout gives everything a new one: CI could never reuse a
result for a test that reads the migrations or the OpenAPI spec. Run it in the
repository root after checkout.
"""
import os, subprocess

def stamp(object_id):
    # A past time (2001-2023): Go never caches a file newer than now.
    return 1_000_000_000 + int(object_id[:8], 16) % 700_000_000

def run(*args):
    return subprocess.run(["git", *args], check=True, capture_output=True).stdout

for entry in run("ls-files", "-s", "-z").split(b"\0"):
    if entry:
        meta, path = entry.split(b"\t", 1)
        if os.path.isfile(path) and not os.path.islink(path):
            t = stamp(meta.split()[1].decode())
            os.utime(path, (t, t))
# Directories by their tree hash: a test that lists one (the migrations)
# depends on its time too. Deepest first, as dating a child never moves a parent.
trees = [line.split(b"\t", 1) for line in run("ls-tree", "-r", "-d", "-z", "HEAD").split(b"\0") if line]
for meta, path in sorted(trees, key=lambda item: -item[1].count(b"/")):
    if os.path.isdir(path):
        t = stamp(meta.split()[2].decode())
        os.utime(path, (t, t))
