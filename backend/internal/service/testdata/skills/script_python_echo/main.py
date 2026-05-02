import json
import os
from pathlib import Path


def main() -> None:
    input_path = Path(os.environ["SUB2API_SKILL_INPUT"])
    output_path = Path(os.environ["SUB2API_SKILL_OUTPUT"])
    payload = json.loads(input_path.read_text(encoding="utf-8"))

    response = {
        "ok": True,
        "runtime": "python3.11",
        "echo": payload.get("message", ""),
        "upper": payload.get("message", "").upper(),
    }

    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(response, ensure_ascii=True), encoding="utf-8")
    print("python skill completed")


if __name__ == "__main__":
    main()
