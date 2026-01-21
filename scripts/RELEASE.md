# Release Scripts

This directory contains scripts for managing version tagging and releases.

## Overview

- **version.sh** - Manages semantic versioning and git tags
- **release.sh** - Handles the release process using goreleaser

## Quick Start

### View Current and Next Versions

```bash
make show-version
```

### Create and Push Version Tags

```bash
make tag-patch  # Bump patch version (0.0.X)
make tag-minor  # Bump minor version (0.X.0)
make tag-major  # Bump major version (X.0.0)
```

### Release with Automatic Versioning

```bash
make release-patch  # Tag patch version and release
make release-minor  # Tag minor version and release
make release-major  # Tag major version and release
```

### Release with Existing Tag

```bash
make release  # Use existing tag (must be on a tagged commit)
```

### Test Releases

```bash
make release-snapshot  # Build snapshot without tagging/publishing
make release-dry-run   # Test release process without publishing
```

## Script Usage

### version.sh

```bash
# Show current and next versions
./scripts/version.sh show

# Calculate next version without tagging
./scripts/version.sh next patch

# Create and push a new version tag
./scripts/version.sh tag patch
./scripts/version.sh tag minor
./scripts/version.sh tag major
```

### release.sh

```bash
# Release with automatic version bump
./scripts/release.sh --bump patch
./scripts/release.sh --bump minor
./scripts/release.sh --bump major

# Release with existing tag
./scripts/release.sh --skip-tag

# Build snapshot (no tag required)
./scripts/release.sh --snapshot

# Dry run (test without publishing)
./scripts/release.sh --bump patch --dry-run
```

## Requirements

- **git** - For version tagging
- **goreleaser** - For building and publishing releases
  - Install: https://goreleaser.com/install/

## Environment Variables

For full releases (not snapshots), you may need:

- `GITHUB_TOKEN` - For GitHub releases
- `DOCKER_USERNAME` - For Docker Hub publishing
- `DOCKER_PASSWORD` - For Docker Hub publishing

## How It Works

1. **Version Calculation**: The script finds the latest git tag (or defaults to v0.0.0)
2. **Semver Bumping**: Increments major, minor, or patch version according to semver
3. **Git Tagging**: Creates an annotated git tag and pushes to origin
4. **Goreleaser**: Runs goreleaser to build binaries, create archives, and publish

## Versioning Rules

- **Patch** (0.0.X): Bug fixes, minor changes
- **Minor** (0.X.0): New features, backward-compatible changes
- **Major** (X.0.0): Breaking changes, major rewrites

All tags follow the format `vX.Y.Z` (e.g., v1.2.3).
