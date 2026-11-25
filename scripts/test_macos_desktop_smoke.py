import json
from pathlib import Path
import runpy
import tempfile
import unittest


ready = runpy.run_path(str(Path(__file__).with_name("smoke-macos-desktop.py")))["ready"]


class ReadyMarkerTests(unittest.TestCase):
    def setUp(self):
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        self.profile = Path(directory.name)
        self.marker = self.profile / "logs" / "sessions" / "run-fixture.session"
        self.marker.parent.mkdir(parents=True)

    def write_marker(self, stage=b"R", **metadata):
        self.marker.write_bytes(stage + json.dumps(
            {"version": "v1.2.3", "os": "darwin", "arch": "arm64", **metadata}
        ).encode())

    def test_waits_for_ready(self):
        self.assertFalse(ready(self.profile, "v1.2.3", "arm64"))
        self.write_marker(b"S")
        self.assertFalse(ready(self.profile, "v1.2.3", "arm64"))
        self.write_marker()
        self.assertTrue(ready(self.profile, "v1.2.3", "arm64"))

    def test_rejects_mismatched_runtime(self):
        for metadata in ({"os": "windows"}, {"arch": "amd64"}, {"version": "v1.2.2"}):
            with self.subTest(metadata=metadata):
                self.write_marker(**metadata)
                with self.assertRaisesRegex(RuntimeError, "Running app reported"):
                    ready(self.profile, "v1.2.3", "arm64")
        self.write_marker(arch="amd64")
        self.assertTrue(ready(self.profile, "v1.2.3", "amd64"))

    def test_rejects_oversize_and_truncated_version(self):
        self.write_marker(unexpected="x" * 512)
        with self.assertRaisesRegex(RuntimeError, "size limit"):
            ready(self.profile, "v1.2.3", "arm64")
        version = "v1.2.3+" + "x" * 121
        self.write_marker(version=version)
        self.assertTrue(ready(self.profile, version, "arm64"))
        version += "x"
        self.write_marker(version=version[:125] + "...")
        with self.assertRaisesRegex(ValueError, "marker limit"):
            ready(self.profile, version, "arm64")
