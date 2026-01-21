#!/usr/bin/env bash
set -euo pipefail

# version.sh - Semantic version management script
# Usage: ./version.sh <command> [options]
# Commands:
#   show              - Show current and next versions
#   tag <type>        - Create and push a new version tag (type: major|minor|patch)
#   next <type>       - Calculate next version without tagging (type: major|minor|patch)

get_current_tag() {
    git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"
}

parse_version() {
    local version=$1
    # Strip 'v' prefix
    version=${version#v}
    echo "$version"
}

get_next_version() {
    local current_tag=$1
    local bump_type=$2

    local version
    version=$(parse_version "$current_tag")

    IFS='.' read -r major minor patch <<< "$version"

    case "$bump_type" in
        major)
            echo "v$((major + 1)).0.0"
            ;;
        minor)
            echo "v${major}.$((minor + 1)).0"
            ;;
        patch)
            echo "v${major}.${minor}.$((patch + 1))"
            ;;
        *)
            echo "Error: Invalid bump type '$bump_type'. Must be: major, minor, or patch" >&2
            exit 1
            ;;
    esac
}

cmd_show() {
    local current_tag
    current_tag=$(get_current_tag)

    echo "Current version: $current_tag"
    echo "Next major:      $(get_next_version "$current_tag" major)"
    echo "Next minor:      $(get_next_version "$current_tag" minor)"
    echo "Next patch:      $(get_next_version "$current_tag" patch)"
}

cmd_next() {
    local bump_type=${1:-}

    if [[ -z "$bump_type" ]]; then
        echo "Error: bump type required (major|minor|patch)" >&2
        exit 1
    fi

    local current_tag
    current_tag=$(get_current_tag)
    get_next_version "$current_tag" "$bump_type"
}

cmd_tag() {
    local bump_type=${1:-}
    local push=${2:-true}

    if [[ -z "$bump_type" ]]; then
        echo "Error: bump type required (major|minor|patch)" >&2
        exit 1
    fi

    local current_tag
    current_tag=$(get_current_tag)

    local next_tag
    next_tag=$(get_next_version "$current_tag" "$bump_type")

    echo "Current tag: $current_tag"
    echo "Creating new $bump_type version tag: $next_tag"

    # Check if tag already exists
    if git rev-parse "$next_tag" >/dev/null 2>&1; then
        echo "Error: Tag $next_tag already exists" >&2
        exit 1
    fi

    # Create annotated tag
    git tag -a "$next_tag" -m "Release $next_tag"
    echo "Tag $next_tag created"

    # Push tag if requested
    if [[ "$push" == "true" ]]; then
        git push origin "$next_tag"
        echo "Tag $next_tag pushed to origin"
    fi
}

cmd_help() {
    cat <<EOF
version.sh - Semantic version management script

Usage: $0 <command> [options]

Commands:
  show              Show current and next versions
  tag <type>        Create and push a new version tag
  next <type>       Calculate next version without tagging
  help              Show this help message

Arguments:
  <type>            Version bump type: major, minor, or patch

Examples:
  $0 show                    # Show current and next versions
  $0 next patch              # Show next patch version
  $0 tag patch               # Create and push new patch version
  $0 tag minor               # Create and push new minor version
EOF
}

main() {
    local command=${1:-help}
    shift || true

    case "$command" in
        show)
            cmd_show
            ;;
        next)
            cmd_next "$@"
            ;;
        tag)
            cmd_tag "$@"
            ;;
        help|--help|-h)
            cmd_help
            ;;
        *)
            echo "Error: Unknown command '$command'" >&2
            echo "" >&2
            cmd_help
            exit 1
            ;;
    esac
}

main "$@"
