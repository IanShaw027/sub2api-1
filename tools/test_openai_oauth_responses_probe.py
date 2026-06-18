import importlib.util
import pathlib
import stat
import sys
import tempfile
import unittest


SCRIPT_PATH = pathlib.Path(__file__).with_name("openai_oauth_responses_probe.py")


def load_module():
    spec = importlib.util.spec_from_file_location("openai_oauth_responses_probe", SCRIPT_PATH)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"failed to load module from {SCRIPT_PATH}")
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


class OpenAIOAuthResponsesProbeContractTest(unittest.TestCase):
    def test_curl_command_keeps_sensitive_headers_out_of_argv(self):
        module = load_module()
        token = "sensitive-access-token"
        account_id = "sensitive-account-id"

        with tempfile.TemporaryDirectory() as temp_dir:
            root = pathlib.Path(temp_dir)
            command = module.build_curl_http2_command(
                "https://chatgpt.example.invalid/backend-api/codex/responses",
                root / "request_payload.json",
                {
                    "authorization": f"Bearer {token}",
                    "chatgpt-account-id": account_id,
                    "accept": "text/event-stream",
                    "content-type": "application/json",
                },
                root / "response_headers.txt",
                root / "response_body.txt",
                root / "curl_request.conf",
            )

            argv = " ".join(command)
            self.assertNotIn(token, argv)
            self.assertNotIn(account_id, argv)
            self.assertEqual(command[:2], ["curl", "--config"])

            config_path = pathlib.Path(command[2])
            config = config_path.read_text(encoding="utf-8")
            self.assertIn(token, config)
            self.assertIn(account_id, config)
            self.assertEqual(stat.S_IMODE(config_path.stat().st_mode), 0o600)


if __name__ == "__main__":
    unittest.main()
