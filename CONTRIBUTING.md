# Contributing to Cora CLI

Thank you for your interest in contributing to Cora CLI! This document provides guidelines and instructions for contributing.

## Development Setup

### Prerequisites

- Go 1.22 or later
- Make (optional, but recommended)

### Getting Started

1. Fork the repository on GitHub
2. Clone your fork locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/cora-cli.git
   cd cora-cli
   ```

3. Install dependencies:
   ```bash
   go mod download
   ```

4. Build the CLI:
   ```bash
   make build
   # or
   go build -o bin/cora .
   ```

5. Run tests:
   ```bash
   make test
   # or
   go test -v ./...
   ```

## Making Changes

### Branch Naming

Use descriptive branch names:
- `feat/add-new-command` for new features
- `fix/upload-timeout` for bug fixes
- `docs/update-readme` for documentation changes

### Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

- `feat: add new command` for new features
- `fix: resolve upload timeout issue` for bug fixes
- `docs: update installation instructions` for documentation
- `chore: update dependencies` for maintenance

### Code Style

- Run `go fmt ./...` before committing
- Run `go vet ./...` to check for issues
- If you have `golangci-lint` installed, run `make lint`

### Testing

- Add tests for new functionality
- Ensure all existing tests pass: `make test`
- Test your changes manually:
  ```bash
  make build
  echo '{"version": 4, "resources": []}' | ./bin/cora upload --workspace test --token YOUR_TOKEN
  ```

## Pull Request Process

1. Update the README.md if you're adding new commands or changing behavior
2. Ensure CI passes (build, lint, tests)
3. Request a review from a maintainer
4. Once approved, a maintainer will merge your PR

## Releasing

Releases are automated via GitHub Actions. To create a new release:

1. Update the version in any relevant places
2. Create and push a new tag:
   ```bash
   git tag v0.2.0
   git push origin v0.2.0
   ```
3. GitHub Actions will automatically build binaries and create a release

## Questions?

- Open an issue for bugs or feature requests
- Visit [https://thecora.app](https://thecora.app) for product documentation
