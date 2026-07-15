#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
RUNTIME_DIR="${SCRIPT_DIR}/runtime"

targets=("$@")
if [ "${#targets[@]}" -eq 0 ]; then
  targets=(python node)
fi

for target in "${targets[@]}"; do
  case "${target}" in
    python | node)
      ;;
    *)
      echo "unsupported runtime target: ${target}" >&2
      exit 1
      ;;
  esac

  input_dir="${RUNTIME_DIR}/${target}/input"
  output_dir="${RUNTIME_DIR}/${target}/output"
  response_file="${output_dir}/response.json"
  request_file="${input_dir}/request.json"

  mkdir -p "${input_dir}" "${output_dir}"
  chmod 0755 "${RUNTIME_DIR}/${target}" "${input_dir}"
  # The fixed container uid needs create access, but it does not need to list or
  # read other host files in this directory.
  chmod 0733 "${output_dir}"

  if [ -f "${request_file}" ]; then
    chmod 0644 "${request_file}"
  fi

  rm -f "${response_file}"
done

printf 'prepared scratch directories under %s\n' "${RUNTIME_DIR}"
