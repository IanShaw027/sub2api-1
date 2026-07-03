import importlib.util
import json
import pathlib
import subprocess
import sys
import tempfile
import unittest


TOOLS_DIR = pathlib.Path(__file__).resolve().parent
PNPM_AUDIT_SCRIPT = TOOLS_DIR / "check_pnpm_audit_exceptions.py"
SECRET_SCAN_SCRIPT = TOOLS_DIR / "secret_scan.py"


def load_module(path: pathlib.Path, module_name: str):
    spec = importlib.util.spec_from_file_location(module_name, path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"failed to load module from {path}")
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


class SecurityScanToolsTest(unittest.TestCase):
    def test_pnpm_audit_top_level_error_fails(self):
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = pathlib.Path(tmp)
            audit_path = tmp_path / "audit.json"
            exceptions_path = tmp_path / "exceptions.yml"
            audit_path.write_text(
                json.dumps({"error": {"code": "ERR_PNPM_AUDIT", "summary": "audit failed"}}),
                encoding="utf-8",
            )
            exceptions_path.write_text("version: 1\nexceptions: []\n", encoding="utf-8")

            result = subprocess.run(
                [
                    sys.executable,
                    str(PNPM_AUDIT_SCRIPT),
                    "--audit",
                    str(audit_path),
                    "--exceptions",
                    str(exceptions_path),
                ],
                check=False,
                text=True,
                capture_output=True,
            )

        self.assertNotEqual(result.returncode, 0)
        self.assertIn("pnpm audit returned an error", result.stderr)
        self.assertIn("ERR_PNPM_AUDIT", result.stderr)

    def test_secret_scan_detects_fine_grained_github_pat(self):
        module = load_module(SECRET_SCAN_SCRIPT, "secret_scan")
        token = "github_pat_" + ("A" * 22) + "_" + ("B" * 59)

        with tempfile.TemporaryDirectory() as tmp:
            original_root = module.ROOT
            try:
                module.ROOT = pathlib.Path(tmp)
                path = module.ROOT / "secret.txt"
                path.write_text(f"token={token}\n", encoding="utf-8")

                findings = module.scan_file(pathlib.Path("secret.txt"))
            finally:
                module.ROOT = original_root

        self.assertEqual(["secret.txt:1: possible GitHub fine-grained PAT"], findings)


if __name__ == "__main__":
    unittest.main()
