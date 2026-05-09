import importlib.util
import inspect
import os
import pathlib
import tempfile
import sys
import unittest


SCRIPT_PATH = pathlib.Path(__file__).with_name("openai_images_test.py")


def load_module():
    spec = importlib.util.spec_from_file_location("openai_images_test", SCRIPT_PATH)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"failed to load module from {SCRIPT_PATH}")
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


class OpenAIImagesTestScriptContractTest(unittest.TestCase):
    def test_script_exposes_expected_entrypoints(self):
        module = load_module()

        for name in (
            "call_images_generations",
            "call_images_edits",
            "call_images2api_generations",
            "call_images2api_edits",
            "main",
        ):
            self.assertTrue(hasattr(module, name), f"missing {name}")

    def test_script_uses_runtime_config_entrypoints(self):
        module = load_module()

        for name in ("DEFAULT_TIMEOUT", "DEFAULT_BASE_URL", "DEFAULT_API_KEY_ENV_VAR", "resolve_runtime_config"):
            self.assertTrue(hasattr(module, name), f"missing {name}")

        signature = inspect.signature(module.call_images_generations)
        self.assertIsNone(signature.parameters["base_url"].default)
        self.assertIsNone(signature.parameters["api_key"].default)

    def test_resolve_runtime_config_prefers_explicit_values_then_env(self):
        module = load_module()

        original_base_url = os.environ.get("SUB2API_BASE_URL")
        original_api_key = os.environ.get("SUB2API_API_KEY")
        try:
            os.environ["SUB2API_BASE_URL"] = "https://example.invalid/root/"
            os.environ["SUB2API_API_KEY"] = "env-key"

            base_url, api_key = module.resolve_runtime_config()
            self.assertEqual(base_url, "https://example.invalid/root")
            self.assertEqual(api_key, "env-key")

            base_url, api_key = module.resolve_runtime_config(
                base_url="https://override.invalid/api/",
                api_key="explicit-key",
            )
            self.assertEqual(base_url, "https://override.invalid/api")
            self.assertEqual(api_key, "explicit-key")
        finally:
            if original_base_url is None:
                os.environ.pop("SUB2API_BASE_URL", None)
            else:
                os.environ["SUB2API_BASE_URL"] = original_base_url
            if original_api_key is None:
                os.environ.pop("SUB2API_API_KEY", None)
            else:
                os.environ["SUB2API_API_KEY"] = original_api_key

    def test_resolve_runtime_config_requires_base_url_when_unset(self):
        module = load_module()

        original_base_url = os.environ.get("SUB2API_BASE_URL")
        original_api_key = os.environ.get("SUB2API_API_KEY")
        try:
            os.environ.pop("SUB2API_BASE_URL", None)
            os.environ["SUB2API_API_KEY"] = "env-key"

            with self.assertRaisesRegex(ValueError, "base_url is required"):
                module.resolve_runtime_config()
        finally:
            if original_base_url is None:
                os.environ.pop("SUB2API_BASE_URL", None)
            else:
                os.environ["SUB2API_BASE_URL"] = original_base_url
            if original_api_key is None:
                os.environ.pop("SUB2API_API_KEY", None)
            else:
                os.environ["SUB2API_API_KEY"] = original_api_key

    def test_script_no_longer_contains_hardcoded_secret(self):
        source = SCRIPT_PATH.read_text(encoding="utf-8")
        self.assertNotIn("sk-57aef36d46bdae6b6a9cc55ed069a92c52a7c316ad7fe0c6e1999b4fd2f94f9d", source)

    def test_validate_common_inputs_allows_blank_response_format(self):
        module = load_module()

        module.validate_common_inputs(
            model="gpt-image-2",
            n=1,
            size="1024x1024",
            response_format="",
            background=None,
            output_format=None,
            input_fidelity=None,
            output_compression=None,
            partial_images=None,
        )

    def test_validate_size_accepts_official_and_gateway_compatible_sizes(self):
        module = load_module()

        for size in ("auto", "1024x1024", "1536x1024", "1024x1536", "2048x2048", "3840x2160"):
            module.validate_size(size)

    def test_build_common_payload_includes_continuation_fields(self):
        module = load_module()

        payload = module.build_common_payload(
            model="gpt-image-2",
            prompt="continue",
            conversation_id="conv-123",
            parent_message_id="parent-123",
        )

        self.assertEqual(payload["conversation_id"], "conv-123")
        self.assertEqual(payload["parent_message_id"], "parent-123")

    def test_call_images2api_edits_exposes_legacy_inpainting_fields(self):
        module = load_module()
        signature = inspect.signature(module.call_images2api_edits)

        for name in ("original_file_id", "original_gen_id", "mask_file_id"):
            self.assertIn(name, signature.parameters)

    def test_save_artifacts_persists_b64_json_with_random_file_names(self):
        module = load_module()
        image_bytes = b"fake-png"
        payload = {
            "data": [
                {
                    "b64_json": module.base64.b64encode(image_bytes).decode("ascii"),
                    "output_format": "png",
                    "revised_prompt": "revised",
                }
            ]
        }

        with tempfile.TemporaryDirectory() as temp_dir:
            artifacts = module.save_artifacts(pathlib.Path(temp_dir), payload, None)

            self.assertEqual(len(artifacts), 1)
            artifact = artifacts[0]
            self.assertEqual(artifact.source_type, "b64_json")
            self.assertIsNotNone(artifact.saved_path)
            self.assertIsNotNone(artifact.base64_saved_path)
            self.assertTrue(pathlib.Path(artifact.saved_path).is_file())
            self.assertTrue(pathlib.Path(artifact.base64_saved_path).is_file())
            self.assertEqual(pathlib.Path(artifact.saved_path).read_bytes(), image_bytes)
            self.assertRegex(os.path.basename(artifact.saved_path), r"^image_[0-9a-f]{32}\.png$")
            self.assertRegex(os.path.basename(artifact.base64_saved_path), r"^image_base64_[0-9a-f]{32}\.txt$")

    def test_save_artifacts_persists_data_url_and_remote_url_sidecars(self):
        module = load_module()
        image_bytes = b"data-url-image"
        payload = {
            "data": [
                {
                    "url": "data:image/webp;base64," + module.base64.b64encode(image_bytes).decode("ascii"),
                    "output_format": "webp",
                },
                {
                    "url": "https://example.com/image.png",
                    "output_format": "png",
                },
            ]
        }

        with tempfile.TemporaryDirectory() as temp_dir:
            artifacts = module.save_artifacts(pathlib.Path(temp_dir), payload, None)

            self.assertEqual(len(artifacts), 2)

            data_url_artifact = artifacts[0]
            self.assertEqual(data_url_artifact.source_type, "data_url")
            self.assertIsNotNone(data_url_artifact.saved_path)
            self.assertIsNotNone(data_url_artifact.url_saved_path)
            self.assertEqual(pathlib.Path(data_url_artifact.saved_path).read_bytes(), image_bytes)
            self.assertRegex(os.path.basename(data_url_artifact.saved_path), r"^image_[0-9a-f]{32}\.webp$")
            self.assertRegex(os.path.basename(data_url_artifact.url_saved_path), r"^image_url_[0-9a-f]{32}\.txt$")

            remote_url_artifact = artifacts[1]
            self.assertEqual(remote_url_artifact.source_type, "url")
            self.assertIsNone(remote_url_artifact.saved_path)
            self.assertIsNotNone(remote_url_artifact.url_saved_path)
            self.assertEqual(
                pathlib.Path(remote_url_artifact.url_saved_path).read_text(encoding="utf-8"),
                "https://example.com/image.png",
            )


if __name__ == "__main__":
    unittest.main()
