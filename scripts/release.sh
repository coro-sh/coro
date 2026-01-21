#!/usr/bin/env bash
set -euo pipefail

# release.sh - Release management script using goreleaser
# Usage: ./release.sh [options]
# Options:
#   --skip-tag        Skip automatic tagging (use existing tag)
#   --bump <type>     Bump version before release (major|minor|patch)
#   --snapshot        Build snapshot (no tag required)
#   --dry-run         Run goreleaser with --skip=publish
#   --help            Show this help message

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VERSION_SCRIPT="$SCRIPT_DIR/version.sh"

bump_type=""
skip_tag=false
snapshot=false
dry_run=false

show_help() {
    cat <<EOF
release.sh - Release management script using goreleaser

Usage: $0 [options]

Options:
  --skip-tag              Skip automatic tagging (use existing tag)
  --bump <type>           Bump version before release (major|minor|patch)
  --snapshot              Build snapshot (no tag required, no publish)
  --dry-run               Run goreleaser with --skip=publish
  --help                  Show this help message

Examples:
  $0 --bump patch         # Bump patch version, tag, and release
  $0 --skip-tag           # Release with existing tag
  $0 --snapshot           # Build snapshot without tagging
  $0 --dry-run            # Test release without publishing

Environment Variables:
  GITHUB_TOKEN            Required for GitHub releases
  DOCKER_USERNAME         Required for Docker Hub publishing
  DOCKER_PASSWORD         Required for Docker Hub publishing
EOF
}

parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --bump)
                bump_type="${2:-}"
                if [[ -z "$bump_type" ]] || [[ ! "$bump_type" =~ ^(major|minor|patch)$ ]]; then
                    echo "Error: --bump requires argument: major, minor, or patch" >&2
                    exit 1
                fi
                shift 2
                ;;
            --skip-tag)
                skip_tag=true
                shift
                ;;
            --snapshot)
                snapshot=true
                shift
                ;;
            --dry-run)
                dry_run=true
                shift
                ;;
            --help|-h)
                show_help
                exit 0
                ;;
            *)
                echo "Error: Unknown option '$1'" >&2
                echo "" >&2
                show_help
                exit 1
                ;;
        esac
    done
}

check_goreleaser() {
    if ! command -v goreleaser &> /dev/null; then
        echo "Error: goreleaser is not installed" >&2
        echo "Install it from: https://goreleaser.com/install/" >&2
        exit 1
    fi
}

check_git_clean() {
    if [[ -n "$(git status --porcelain)" ]]; then
        echo "Error: Working directory is not clean. Commit or stash changes first." >&2
        exit 1
    fi
}

run_release() {
    local goreleaser_args=("release" "--clean")

    if [[ "$snapshot" == "true" ]]; then
        echo "Building snapshot release..."
        goreleaser_args+=("--snapshot")
    elif [[ "$dry_run" == "true" ]]; then
        echo "Running dry-run release..."
        goreleaser_args+=("--skip=publish")
    fi

    echo "Running: goreleaser ${goreleaser_args[*]}"
    goreleaser "${goreleaser_args[@]}"
}

main() {
    parse_args "$@"

    check_goreleaser

    # Handle snapshot builds (no tagging needed)
    if [[ "$snapshot" == "true" ]]; then
        run_release
        exit 0
    fi

    # Check for clean git state
    check_git_clean

    # Handle version bumping
    if [[ -n "$bump_type" ]]; then
        if [[ "$skip_tag" == "true" ]]; then
            echo "Error: Cannot use both --bump and --skip-tag" >&2
            exit 1
        fi

        echo "Bumping $bump_type version..."
        "$VERSION_SCRIPT" tag "$bump_type"
        echo ""
    elif [[ "$skip_tag" == "false" ]]; then
        echo "Error: Must specify --bump <type> or --skip-tag" >&2
        exit 1
    fi

    # Run goreleaser
    run_release
}

main "$@"
