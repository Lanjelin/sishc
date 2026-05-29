#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  scripts/release.sh [--version vX.Y.Z] [--out-dir dist] [--docker-platforms linux/amd64,linux/arm64] [--publish]

Builds release binaries and checksums.
With --publish, it also creates or updates the GitHub release and pushes the Docker image to GHCR.

Options:
  --version            Release tag to publish. Defaults to the exact tag on HEAD.
  --out-dir            Output directory for release artifacts. Default: dist
  --docker-platforms   Platforms for the Docker image when publishing. Default: linux/amd64,linux/arm64
  --publish            Upload assets to GitHub and push Docker images.
  -h, --help           Show this help.
EOF
}

die() {
  printf 'release: %s\n' "$*" >&2
  exit 1
}

VERSION=""
OUT_DIR="dist"
DOCKER_PLATFORMS="linux/amd64,linux/arm64"
BUILDX_BUILDER="${SISHC_BUILDX_BUILDER:-sishc-release}"
PUBLISH=0

while (($#)); do
  case "$1" in
    --version)
      [[ $# -ge 2 ]] || die "--version requires a value"
      VERSION="$2"
      shift 2
      ;;
    --out-dir)
      [[ $# -ge 2 ]] || die "--out-dir requires a value"
      OUT_DIR="$2"
      shift 2
      ;;
    --docker-platforms)
      [[ $# -ge 2 ]] || die "--docker-platforms requires a value"
      DOCKER_PLATFORMS="$2"
      shift 2
      ;;
    --publish)
      PUBLISH=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unknown argument: $1"
      ;;
  esac
done

if [[ -z "$VERSION" ]]; then
  VERSION="$(git describe --tags --exact-match 2>/dev/null || true)"
fi
[[ -n "$VERSION" ]] || die "set --version or run on a tagged commit"
[[ "$VERSION" == v* ]] || die "version should look like v1.0.0"

git diff --quiet || die "working tree has uncommitted changes"
git diff --cached --quiet || die "index has staged changes"

if ! git show-ref --verify --quiet "refs/tags/$VERSION"; then
  die "tag $VERSION does not exist"
fi

TAG_SHA="$(git rev-list -n1 "$VERSION")"
HEAD_SHA="$(git rev-parse HEAD)"
[[ "$TAG_SHA" == "$HEAD_SHA" ]] || die "tag $VERSION must point at HEAD"

if ! command -v go >/dev/null 2>&1; then
  die "go is required"
fi

if ! command -v sha256sum >/dev/null 2>&1; then
  die "sha256sum is required"
fi

mkdir -p "$OUT_DIR/$VERSION"

targets=(
  "linux amd64"
  "linux arm64"
  "darwin amd64"
  "darwin arm64"
)

for target in "${targets[@]}"; do
  read -r goos goarch <<<"$target"
  out="$OUT_DIR/$VERSION/sishc_${goos}_${goarch}"
  printf 'building %s/%s -> %s\n' "$goos" "$goarch" "$out"
  GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 go build -trimpath -o "$out" ./cmd/sishc
done

(
  cd "$OUT_DIR/$VERSION"
  sha256sum sishc_* > checksums.txt
)

if (( PUBLISH )); then
  if ! command -v gh >/dev/null 2>&1; then
    die "gh is required for --publish"
  fi
  if ! command -v docker >/dev/null 2>&1; then
    die "docker is required for --publish"
  fi

  printf 'publishing GitHub release %s\n' "$VERSION"
  if gh release view "$VERSION" >/dev/null 2>&1; then
    gh release upload "$VERSION" "$OUT_DIR/$VERSION"/sishc_* "$OUT_DIR/$VERSION/checksums.txt" --clobber
  else
    gh release create "$VERSION" "$OUT_DIR/$VERSION"/sishc_* "$OUT_DIR/$VERSION/checksums.txt" --title "$VERSION" --generate-notes
  fi

  printf 'pushing Docker image ghcr.io/lanjelin/sishc:%s\n' "$VERSION"
  if ! docker buildx inspect "$BUILDX_BUILDER" >/dev/null 2>&1; then
    docker buildx create --name "$BUILDX_BUILDER" --driver docker-container --use >/dev/null
  else
    docker buildx use "$BUILDX_BUILDER" >/dev/null
  fi
  docker buildx inspect --bootstrap "$BUILDX_BUILDER" >/dev/null
  docker buildx build \
    --platform "$DOCKER_PLATFORMS" \
    -t "ghcr.io/lanjelin/sishc:$VERSION" \
    -t "ghcr.io/lanjelin/sishc:latest" \
    --push .
fi

printf 'done: %s\n' "$OUT_DIR/$VERSION"
