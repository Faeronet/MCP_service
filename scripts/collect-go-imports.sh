#!/usr/bin/env bash
set -u

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GENERATED_AT="$(date -Iseconds)"
DEPS_TEMPLATE='{{if not .Standard}}{{.ImportPath}}{{end}}'

echo "[INFO] Root: ${ROOT_DIR}"
echo "[INFO] Started: ${GENERATED_AT}"

while IFS= read -r -d '' go_mod; do
  module_dir="$(dirname "${go_mod}")"
  module_dir_abs="$(realpath "${module_dir}")"
  imports_file="${module_dir}/imports.txt"
  manifest_file="${module_dir}/go-deps-manifest.txt"
  error_file="${module_dir}/go-deps-error.log"

  echo "----------------------------------------"
  echo "[INFO] Module dir: ${module_dir_abs}"

  pushd "${module_dir}" >/dev/null || {
    printf "[%s] failed to enter module dir: %s\n" "$(date -Iseconds)" "${module_dir_abs}" >>"${error_file}"
    echo "[ERROR] Cannot enter module directory, skipped."
    continue
  }

  if ! module_path="$(go list -m 2>>"${error_file}")"; then
    {
      echo "[$(date -Iseconds)] go list -m failed"
      echo "dir=${module_dir_abs}"
      echo "cmd=go list -m"
      echo
    } >>"${error_file}"
    echo "[ERROR] go list -m failed, see ${error_file}"
    popd >/dev/null || true
    continue
  fi

  if ! raw_imports="$(go list -deps -f "${DEPS_TEMPLATE}" ./... 2>>"${error_file}")"; then
    {
      echo "[$(date -Iseconds)] go list deps failed"
      echo "dir=${module_dir_abs}"
      echo "cmd=go list -deps -f '${DEPS_TEMPLATE}' ./..."
      echo
    } >>"${error_file}"
    echo "[ERROR] go list -deps failed, see ${error_file}"
    popd >/dev/null || true
    continue
  fi

  printf "%s\n" "${raw_imports}" \
    | sed '/^[[:space:]]*$/d' \
    | awk -v mod="${module_path}" 'index($0, mod) != 1 && index($0, mod "/") != 1 { print }' \
    | sort -u >"${imports_file}"

  imports_count="$(wc -l <"${imports_file}" | tr -d ' ')"
  {
    echo "module_path=${module_path}"
    echo "module_dir_abs=${module_dir_abs}"
    echo "imports_file=${imports_file}"
    echo "imports_count=${imports_count}"
    echo "generated_at=${GENERATED_AT}"
    echo "command=go list -deps -f '${DEPS_TEMPLATE}' ./..."
  } >"${manifest_file}"

  echo "[OK] module=${module_path} imports=${imports_count}"
  echo "[OK] wrote ${imports_file}"
  echo "[OK] wrote ${manifest_file}"
  popd >/dev/null || true
done < <(
  find "${ROOT_DIR}" \
    \( -path "${ROOT_DIR}/.git" \
    -o -path "${ROOT_DIR}/vendor" \
    -o -path "${ROOT_DIR}/node_modules" \
    -o -path "${ROOT_DIR}/dist" \
    -o -path "${ROOT_DIR}/build" \
    -o -path "${ROOT_DIR}/tmp" \) -prune \
    -o -name "go.mod" -print0
)

echo "----------------------------------------"
echo "[INFO] Done."
