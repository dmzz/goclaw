#!/usr/bin/env bash
# Build a dmzz release full image without running Vite or Go builds inside Docker.
#
# Usage:
#   ./scripts/build-dmzz-prebuilt-full.sh --version v3.10.0-dmzz.2
#   ./scripts/build-dmzz-prebuilt-full.sh --version v3.10.0-dmzz.2 \
#     --image goclaw:v3.10.0-dmzz.2-full \
#     --docker-host tcp://192.168.11.1:2375

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

VERSION=""
IMAGE=""
DOCKER_HOST_URI="${DOCKER_HOST_URI:-${DOCKER_HOST:-}}"
DOCKER_API_VERSION_VALUE="${DOCKER_API_VERSION:-}"
RUNTIME_BASE_IMAGE="${RUNTIME_BASE_IMAGE:-}"
OUTPUT_DIR="${OUTPUT_DIR:-out}"
SKIP_INSTALL=false
SKIP_UI_BUILD=false
SKIP_GO_BUILD=false
SKIP_IMAGE=false

usage() {
  cat <<'EOF'
Usage: build-dmzz-prebuilt-full.sh --version vX.Y.Z-dmzz.N [options]

Options:
  --version TAG            Release version embedded into the linux binary.
  --image NAME             Target image tag. Default: goclaw:${VERSION}-full
  --docker-host URI        Docker host passed as `docker -H URI`.
  --docker-api-version V   Export DOCKER_API_VERSION for docker build.
  --runtime-base-image N   Base runtime image. Default: ghcr.io/nextlevelbuilder/goclaw:${VERSION%%-dmzz*}-full
  --output-dir DIR         Local artifact directory. Default: out
  --skip-install           Skip `pnpm install --frozen-lockfile --force`.
  --skip-ui-build          Reuse existing `ui/web/dist` without running `pnpm build`.
  --skip-go-build          Reuse existing linux binaries from OUTPUT_DIR.
  --skip-image             Stop after local UI and Go builds.
  --help, -h               Show this help.
EOF
}

require_option_value() {
  local option="$1"
  if [[ $# -lt 2 ]]; then
    echo "Missing value for ${option}" >&2
    usage
    exit 1
  fi
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)
      require_option_value "$1" "$@"
      VERSION="$2"
      shift 2
      ;;
    --image)
      require_option_value "$1" "$@"
      IMAGE="$2"
      shift 2
      ;;
    --docker-host)
      require_option_value "$1" "$@"
      DOCKER_HOST_URI="$2"
      shift 2
      ;;
    --docker-api-version)
      require_option_value "$1" "$@"
      DOCKER_API_VERSION_VALUE="$2"
      shift 2
      ;;
    --runtime-base-image)
      require_option_value "$1" "$@"
      RUNTIME_BASE_IMAGE="$2"
      shift 2
      ;;
    --output-dir)
      require_option_value "$1" "$@"
      OUTPUT_DIR="$2"
      shift 2
      ;;
    --skip-install) SKIP_INSTALL=true; shift ;;
    --skip-ui-build) SKIP_UI_BUILD=true; shift ;;
    --skip-go-build) SKIP_GO_BUILD=true; shift ;;
    --skip-image) SKIP_IMAGE=true; shift ;;
    --help|-h)
      usage
      exit 0
      ;;
    *) echo "Unknown option: $1" >&2; usage; exit 1 ;;
  esac
done

if [[ -z "${VERSION}" ]]; then
  echo "--version is required" >&2
  usage
  exit 1
fi

BASE_VERSION="${VERSION%%-dmzz*}"

if [[ -z "${IMAGE}" ]]; then
  IMAGE="goclaw:${VERSION}-full"
fi

if [[ -z "${RUNTIME_BASE_IMAGE}" ]]; then
  RUNTIME_BASE_IMAGE="ghcr.io/nextlevelbuilder/goclaw:${BASE_VERSION}-full"
fi

if [[ "${OUTPUT_DIR}" = /* ]]; then
  OUT_DIR="${OUTPUT_DIR}"
else
  OUT_DIR="${REPO_ROOT}/${OUTPUT_DIR}"
fi
UI_DIST_DIR="${REPO_ROOT}/ui/web/dist"
EMBED_DIST_DIR="${REPO_ROOT}/internal/webui/dist"

cd "${REPO_ROOT}"

if [[ "${SKIP_UI_BUILD}" != "true" ]]; then
  echo "==> Building local web UI"
  if [[ "${SKIP_INSTALL}" != "true" ]]; then
    pnpm -C ui/web install --frozen-lockfile --force
  fi
  pnpm -C ui/web build
else
  echo "==> Reusing existing web UI dist"
  if [[ ! -d "${UI_DIST_DIR}" ]]; then
    echo "ui/web/dist is missing; drop --skip-ui-build or build the UI first" >&2
    exit 1
  fi
fi

echo "==> Syncing embedded UI dist"
rm -rf "${EMBED_DIST_DIR}"
mkdir -p "${EMBED_DIST_DIR}" "${OUT_DIR}"
cp -R "${UI_DIST_DIR}/." "${EMBED_DIST_DIR}/"

if [[ "${SKIP_GO_BUILD}" != "true" ]]; then
  echo "==> Building linux binaries"
  CGO_ENABLED=0 GOOS=linux go build \
    -tags embedui \
    -ldflags="-s -w -X github.com/nextlevelbuilder/goclaw/cmd.Version=${VERSION}" \
    -o "${OUT_DIR}/goclaw" .

  CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o "${OUT_DIR}/pkg-helper" ./cmd/pkg-helper
else
  echo "==> Reusing existing linux binaries"
  if [[ ! -f "${OUT_DIR}/goclaw" || ! -f "${OUT_DIR}/pkg-helper" ]]; then
    echo "Prebuilt binaries are missing in ${OUT_DIR}; drop --skip-go-build or build them first" >&2
    exit 1
  fi
fi

if [[ "${SKIP_IMAGE}" == "true" ]]; then
  echo "==> Skipped docker image build"
  exit 0
fi

echo "==> Building prebuilt full image ${IMAGE}"
echo "==> Runtime base image ${RUNTIME_BASE_IMAGE}"
DOCKER_CMD=(docker)
if [[ -n "${DOCKER_HOST_URI}" ]]; then
  DOCKER_CMD+=(-H "${DOCKER_HOST_URI}")
fi

if [[ -n "${DOCKER_API_VERSION_VALUE}" ]]; then
  export DOCKER_API_VERSION="${DOCKER_API_VERSION_VALUE}"
fi

# Docker Desktop / remote daemon setups with SOCKS proxy often fail during
# BuildKit metadata resolution even when the classic builder can pull the same
# base image successfully. Keep the strict dmzz release path on the legacy
# builder unless the caller explicitly overrides it.
if [[ -z "${DOCKER_BUILDKIT:-}" ]]; then
  export DOCKER_BUILDKIT=0
fi

echo "==> DOCKER_BUILDKIT=${DOCKER_BUILDKIT}"

if ! "${DOCKER_CMD[@]}" build \
  -t "${IMAGE}" \
  --build-arg VERSION="${VERSION}" \
  --build-arg GOCLAW_RUNTIME_BASE_IMAGE="${RUNTIME_BASE_IMAGE}" \
  -f Dockerfile.prebuilt-full .; then
  echo "docker build failed for runtime base ${RUNTIME_BASE_IMAGE}" >&2
  if [[ "${RUNTIME_BASE_IMAGE}" == "ghcr.io/nextlevelbuilder/goclaw:${BASE_VERSION}-full" ]]; then
    echo "If this daemon cannot pull ghcr.io through the current proxy, retry with --runtime-base-image pointing to a cached local full image, for example: goclaw:${VERSION%-dmzz.*}-dmzz.1-full" >&2
  fi
  exit 1
fi

echo "==> Done"
echo "Built image: ${IMAGE}"
