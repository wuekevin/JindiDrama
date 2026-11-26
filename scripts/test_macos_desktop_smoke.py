import json
import os
from pathlib import Path
import runpy
import subprocess
import tempfile
import unittest
from unittest.mock import patch


smoke = runpy.run_path(str(Path(__file__).with_name("smoke-macos-desktop.py")))
ready = smoke["ready"]


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


class DatabaseProbeTests(unittest.TestCase):
    def test_captures_probe_failure_for_smoke_diagnostics(self):
        with tempfile.TemporaryDirectory() as directory:
            profile = Path(directory)
            probe = profile / "database-probe"
            command = [probe, "verify-db", profile / "jindi.db"]
            with (profile / "native-output.log").open("wb") as output:
                with patch("subprocess.run", side_effect=subprocess.CalledProcessError(1, command)) as run:
                    with self.assertRaises(subprocess.CalledProcessError):
                        smoke["verify_database"](profile, probe, output)
                    run.assert_called_once_with(command, check=True, timeout=30,
                                                stdout=output, stderr=subprocess.STDOUT)


class LaunchEnvironmentTests(unittest.TestCase):
    def test_ignores_operator_sources_without_changing_parent_environment(self):
        inherited = {
            "JINDIDRAMA_CMS_URL": "https://cms.example.test/?key=fixture",
            "JINDIDRAMA_LICENSED_MEDIA_DIR": "/fixture/media",
            "JINDIDRAMA_DATA_DIR": "/fixture/profile",
        }
        with tempfile.TemporaryDirectory() as directory:
            profile = Path(directory)
            with patch.dict(os.environ, inherited), patch(
                "subprocess.Popen", side_effect=RuntimeError("stop before startup")
            ) as launch:
                with self.assertRaisesRegex(RuntimeError, "stop before startup"):
                    smoke["run_app"](
                        profile / "JindiDrama", "window-probe", "db-probe", profile,
                        profile / "output.log", 5, "v1.2.3", "arm64",
                    )
                child = launch.call_args.kwargs["env"]
                self.assertNotIn("JINDIDRAMA_CMS_URL", child)
                self.assertNotIn("JINDIDRAMA_LICENSED_MEDIA_DIR", child)
                self.assertEqual(child["JINDIDRAMA_DATA_DIR"], str(profile))
                self.assertEqual({key: os.environ[key] for key in inherited}, inherited)
