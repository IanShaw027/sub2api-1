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
    def assert_secret_finding(self, content, expected):
        module = load_module(SECRET_SCAN_SCRIPT, "secret_scan_test_module")
        with tempfile.TemporaryDirectory() as tmp:
            original_root = module.ROOT
            try:
                module.ROOT = pathlib.Path(tmp)
                path = module.ROOT / "secret.yaml"
                path.write_text(content, encoding="utf-8")
                findings = module.scan_file(pathlib.Path("secret.yaml"))
            finally:
                module.ROOT = original_root
        self.assertEqual([f"secret.yaml:1: possible {expected}"], findings)

    def assert_no_secret_finding(self, content):
        module = load_module(SECRET_SCAN_SCRIPT, "secret_scan_no_finding_test_module")
        with tempfile.TemporaryDirectory() as tmp:
            original_root = module.ROOT
            try:
                module.ROOT = pathlib.Path(tmp)
                path = module.ROOT / "config.example.env"
                path.write_text(content, encoding="utf-8")
                findings = module.scan_file(pathlib.Path("config.example.env"))
            finally:
                module.ROOT = original_root
        self.assertEqual([], findings)

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

    def test_secret_scan_detects_aws_access_key(self):
        self.assert_secret_finding("key=AKIA1234567890ABCDEF\n", "AWS access key ID")

    def test_secret_scan_detects_private_key_block(self):
        self.assert_secret_finding("-----BEGIN PRIVATE KEY-----\n", "private key block")

    def test_secret_scan_detects_jwt(self):
        token = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0." + ("a" * 32)
        self.assert_secret_finding(f"token={token}\n", "JWT")

    def test_secret_scan_detects_database_url_password(self):
        self.assert_secret_finding(
            "DATABASE_URL=postgres://sub2api:CorrectHorseBattery42@db.example/app\n",
            "database URL password",
        )

    def test_secret_scan_detects_configured_signing_secret(self):
        self.assert_secret_finding(
            'download_signing_secret: "HighlySensitiveSigningKey42"\n',
            "configured secret",
        )
        self.assert_secret_finding(
            "download_signing_secret: HighlySensitiveSigningKey42 # rotate regularly\n",
            "configured secret",
        )

    def test_secret_scan_detects_redis_url_password(self):
        self.assert_secret_finding(
            "REDIS_URL=rediss://default:CorrectHorseBattery42@cache.example:6379/0\n",
            "Redis URL password",
        )

    def test_secret_scan_detects_configured_redis_password(self):
        self.assert_secret_finding(
            "REDIS_PASSWORD=CorrectHorseBattery42\n",
            "configured secret",
        )
        self.assert_secret_finding(
            "redis_password: CorrectHorseBattery42, # migrated config\n",
            "configured secret",
        )

    def test_secret_scan_detects_payment_and_webhook_secrets(self):
        self.assert_secret_finding(
            'stripe_webhook_secret: "whsec_RealisticProductionSecret123456"\n',
            "Stripe webhook secret",
        )
        self.assert_secret_finding(
            "MERCHANT_PRIVATE_KEY=ProductionMerchantPrivateKeyMaterial42\n",
            "configured secret",
        )

    def test_secret_scan_detects_additional_provider_tokens(self):
        self.assert_secret_finding(
            "token=hf_" + ("A" * 32) + "\n",
            "Hugging Face token",
        )
        self.assert_secret_finding(
            "token=xoxb-" + ("1" * 12) + "-" + ("A" * 24) + "\n",
            "Slack token",
        )

    def test_secret_scan_allows_placeholders_and_variable_references(self):
        self.assert_no_secret_finding("REDIS_PASSWORD=change_me_for_production\n")
        self.assert_no_secret_finding("webhook_secret: your_webhook_secret_placeholder\n")
        self.assert_no_secret_finding("REDIS_PASSWORD=${REDIS_PASSWORD:?required}\n")
        self.assert_no_secret_finding("redis_password: ${REDIS_PASSWORD:?required}\n")

    def test_secret_scan_ignores_code_field_references(self):
        module = load_module(SECRET_SCAN_SCRIPT, "secret_scan_code_reference_test_module")
        with tempfile.TemporaryDirectory() as tmp:
            original_root = module.ROOT
            try:
                module.ROOT = pathlib.Path(tmp)
                path = module.ROOT / "oauth.go"
                path.write_text('"client_secret": cfg.ClientSecret,\n', encoding="utf-8")
                findings = module.scan_file(pathlib.Path("oauth.go"))
            finally:
                module.ROOT = original_root
        self.assertEqual([], findings)

    def test_secret_scan_skips_test_fixture_paths(self):
        module = load_module(SECRET_SCAN_SCRIPT, "secret_scan_fixture_test_module")
        self.assertTrue(module.is_test_fixture_path(pathlib.Path("testdata/production.env")))
        self.assertTrue(module.is_test_fixture_path(pathlib.Path("fixtures/payment.json")))
        self.assertTrue(module.is_test_fixture_path(pathlib.Path("sample_test.py")))


if __name__ == "__main__":
    unittest.main()
