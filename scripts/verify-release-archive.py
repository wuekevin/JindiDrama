"""Check an Actions ZIP against the expected size and digests supplied by the release verifier."""

import hashlib
import json
import os
import stat
import sys
import zipfile


def sha256(stream):
    digest = hashlib.sha256()
    for chunk in iter(lambda: stream.read(1024 * 1024), b""):
        digest.update(chunk)
    return digest.hexdigest()


def verify_archive(path, expected):
    with open(path, "rb") as source:
        if os.fstat(source.fileno()).st_size != expected["size"]:
            raise ValueError("GitHub handoff ZIP size mismatch")
        if sha256(source) != expected["digest"]:
            raise ValueError("GitHub handoff ZIP SHA-256 mismatch")

        with zipfile.ZipFile(source) as archive:
            files = expected["files"]
            entries = archive.infolist()
            if len(entries) != len(files) or {entry.filename for entry in entries} != files.keys():
                raise ValueError("GitHub handoff ZIP has missing, duplicate or unexpected entries")
            for entry in entries:
                file = files[entry.filename]
                file_type = stat.S_IFMT(entry.external_attr >> 16)
                if file_type not in (0, stat.S_IFREG) or entry.file_size != file["size"]:
                    raise ValueError(f"GitHub handoff ZIP entry type or size mismatch: {entry.filename}")
                with archive.open(entry) as contents:
                    if sha256(contents) != file["digest"]:
                        raise ValueError(f"GitHub handoff ZIP entry SHA-256 mismatch: {entry.filename}")


if __name__ == "__main__":
    try:
        verify_archive(sys.argv[1], json.load(sys.stdin))
    except (OSError, ValueError, RuntimeError, zipfile.BadZipFile) as error:
        sys.exit(str(error))
