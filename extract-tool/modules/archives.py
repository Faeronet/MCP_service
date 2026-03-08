"""Распаковка архивов (zip, tar.gz, tar.xz)."""
from __future__ import annotations

import io
import logging
import os
import tarfile
import zipfile

log = logging.getLogger("extract-tool.archives")


def file_extension(name: str) -> str:
    name = (name or "").lower()
    if name.endswith(".tar.gz") or name.endswith(".tgz"):
        return ".tar.gz"
    if name.endswith(".tar.xz") or name.endswith(".txz"):
        return ".tar.xz"
    return os.path.splitext(name)[1]


def extract_archive(data: bytes, filename: str) -> list[tuple[str, bytes]] | None:
    fn = (filename or "").lower().strip()
    try:
        if fn.endswith(".zip"):
            out = []
            with zipfile.ZipFile(io.BytesIO(data), "r") as z:
                for name in z.namelist():
                    if z.getinfo(name).is_dir():
                        continue
                    try:
                        out.append((name, z.read(name)))
                    except Exception as e:
                        log.warning("zip read %s: %s", name, e)
            return out
        if fn.endswith(".tar.xz") or fn.endswith(".txz"):
            with tarfile.open(fileobj=io.BytesIO(data), mode="r:*") as t:
                return [(m.name, t.extractfile(m).read()) for m in t.getmembers() if m.isfile()]
        if fn.endswith(".tar.gz") or fn.endswith(".tgz") or fn.endswith(".tar"):
            with tarfile.open(fileobj=io.BytesIO(data), mode="r:*") as t:
                return [(m.name, t.extractfile(m).read()) for m in t.getmembers() if m.isfile()]
    except Exception as e:
        log.warning("archive extract failed: %s", e)
    return None
