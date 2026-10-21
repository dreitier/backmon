#!/usr/bin/env python3
"""Pins the base images in the Dockerfile to the digest their tag currently points to.

    FROM golang:1.26 AS builder   ->   FROM golang:1.26@sha256:<digest> AS builder

Already pinned images are refreshed. Requires `docker buildx` (uses `docker buildx imagetools inspect`).
Afterwards Dependabot keeps tags and digests up to date.

Usage: pin-images.py [Dockerfile]
"""
import json
import os
import re
import subprocess
import sys

FROM = re.compile(r"^(FROM\s+(?:--\S+\s+)*)(\S+)(.*)$", re.IGNORECASE)


def digest_of(ref):
    docker = os.environ.get("DOCKER", "docker")
    out = subprocess.run(
        [docker, "buildx", "imagetools", "inspect", ref, "--format", "{{json .Manifest}}"],
        check=True, capture_output=True, text=True,
    ).stdout
    return json.loads(out)["digest"]


def main():
    path = sys.argv[1] if len(sys.argv) > 1 else "Dockerfile"
    with open(path, encoding="utf-8") as f:
        lines = f.readlines()

    stages = set()
    for i, line in enumerate(lines):
        m = FROM.match(line.rstrip("\n"))
        if not m:
            continue
        prefix, image, rest = m.groups()
        alias = re.search(r"\s+AS\s+(\S+)", rest, re.IGNORECASE)
        ref = image.split("@", 1)[0]
        # skip build stages ("FROM builder") and images given as build args
        if ref.lower() in stages or "$" in ref:
            pass
        else:
            digest = digest_of(ref)
            lines[i] = f"{prefix}{ref}@{digest}{rest}\n"
            print(f"{ref} -> {digest}")
        if alias:
            stages.add(alias.group(1).lower())

    with open(path, "w", encoding="utf-8") as f:
        f.writelines(lines)


if __name__ == "__main__":
    main()
