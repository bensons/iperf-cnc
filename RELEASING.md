# Release Process

This document describes how to create a new release of iperf-cnc.

## Prerequisites

- Push access to the repository
- All tests passing on main branch
- All changes committed and pushed

## Creating a Release

### 1. Update Version Information

Update the version in README.md:

```bash
# After creating the tag, update README.md
make update-version

# Commit the version update
git add README.md
git commit -m "Update README.md to version vX.Y.Z"
git push
```

**Note:** The version in README.md should be updated after creating the tag, as the `update-version` script reads the latest git tag.

### 2. Create and Push a Tag

```bash
# Make sure you're on main and up to date
git checkout main
git pull

# Create a tag (use semantic versioning: v1.0.0, v1.1.0, v2.0.0, etc.)
git tag -a v1.0.0 -m "Release v1.0.0"

# Push the tag to GitHub
git push origin v1.0.0
```

### 3. Automated Build

Once you push the tag, GitHub Actions will automatically:

1. Checkout the code
2. Set up Go 1.22
3. Install protoc and required plugins
4. Generate protobuf code
5. Build binaries for:
   - Linux (amd64, arm64)
   - macOS (amd64, arm64)
   - Windows (amd64, arm64)
6. Create archives (tar.gz for Unix, zip for Windows)
7. Generate checksums
8. Create a GitHub Release with all artifacts

### 4. Monitor the Build

1. Go to: https://github.com/bensons/iperf-cnc/actions
2. Click on the "Release" workflow run
3. Wait for it to complete (usually 2-5 minutes)

### 5. Verify the Release

1. Go to: https://github.com/bensons/iperf-cnc/releases
2. Find your new release
3. Verify that all expected artifacts are present:
   - `iperf-cnc-daemon_v1.0.0_Linux_x86_64.tar.gz`
   - `iperf-cnc-daemon_v1.0.0_Linux_arm64.tar.gz`
   - `iperf-cnc-daemon_v1.0.0_Darwin_x86_64.tar.gz`
   - `iperf-cnc-daemon_v1.0.0_Darwin_arm64.tar.gz`
   - `iperf-cnc-daemon_v1.0.0_Windows_x86_64.zip`
   - `iperf-cnc-daemon_v1.0.0_Windows_arm64.zip`
   - `iperf-cnc-controller_v1.0.0_Linux_x86_64.tar.gz`
   - `iperf-cnc-controller_v1.0.0_Linux_arm64.tar.gz`
   - `iperf-cnc-controller_v1.0.0_Darwin_x86_64.tar.gz`
   - `iperf-cnc-controller_v1.0.0_Darwin_arm64.tar.gz`
   - `iperf-cnc-controller_v1.0.0_Windows_x86_64.zip`
   - `iperf-cnc-controller_v1.0.0_Windows_arm64.zip`
   - `checksums.txt`

### 6. Edit Release Notes (Optional)

The release will be created with auto-generated notes. You can edit them to add:
- Highlights of new features
- Breaking changes
- Bug fixes
- Known issues
- Upgrade instructions

## Testing a Release Locally

Before creating an official release, you can test the build process locally:

```bash
# Install GoReleaser (if not already installed)
brew install goreleaser

# Test the build (creates binaries in dist/ directory)
goreleaser build --snapshot --clean

# Test the full release process (without publishing)
goreleaser release --snapshot --clean
```

## Versioning Guidelines

We follow [Semantic Versioning](https://semver.org/):

- **MAJOR** version (v2.0.0): Incompatible API changes
- **MINOR** version (v1.1.0): New functionality in a backward compatible manner
- **PATCH** version (v1.0.1): Backward compatible bug fixes

### Examples

- `v1.0.0` - First stable release
- `v1.0.1` - Bug fix release
- `v1.1.0` - New features added
- `v2.0.0` - Breaking changes

## Troubleshooting

### Build Fails

1. Check the GitHub Actions logs for errors
2. Ensure all tests pass on main branch
3. Verify the tag format is correct (must start with 'v')
4. Check that protobuf definitions are valid

### Missing Artifacts

1. Check the GoReleaser configuration in `.goreleaser.yml`
2. Verify the build matrix includes all desired platforms
3. Check for build errors in the GitHub Actions logs

### Release Not Created

1. Ensure the tag was pushed to GitHub (not just created locally)
2. Verify the tag name starts with 'v'
3. Check that the GitHub Actions workflow has write permissions
4. Look for errors in the workflow run

## Rolling Back a Release

If you need to remove a release:

1. Go to the Releases page
2. Click "Delete" on the release
3. Delete the tag locally: `git tag -d v1.0.0`
4. Delete the tag remotely: `git push origin :refs/tags/v1.0.0`

## Support

For issues with the release process, check:
- [GoReleaser Documentation](https://goreleaser.com/)
- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- Project issues: https://github.com/bensons/iperf-cnc/issues

