# Cora CLI

A command-line tool for uploading Terraform state to [Cora](https://thecora.app), the Terraform State Visualizer.

The Cora CLI is designed to integrate seamlessly with [Atlantis](https://www.runatlantis.io/) and other CI/CD workflows, allowing you to keep your infrastructure visualizations up-to-date automatically.

## Installation

### Download Binary

Download the latest release from the [Releases page](https://github.com/clairitydev/cora-cli/releases).

**macOS (Apple Silicon):**
```bash
curl -L https://github.com/clairitydev/cora-cli/releases/latest/download/cora_latest_darwin_arm64.tar.gz | tar xz
sudo mv cora /usr/local/bin/
```

**macOS (Intel):**
```bash
curl -L https://github.com/clairitydev/cora-cli/releases/latest/download/cora_latest_darwin_amd64.tar.gz | tar xz
sudo mv cora /usr/local/bin/
```

**Linux (amd64):**
```bash
curl -L https://github.com/clairitydev/cora-cli/releases/latest/download/cora_latest_linux_amd64.tar.gz | tar xz
sudo mv cora /usr/local/bin/
```

**Linux (arm64):**
```bash
curl -L https://github.com/clairitydev/cora-cli/releases/latest/download/cora_latest_linux_arm64.tar.gz | tar xz
sudo mv cora /usr/local/bin/
```

**Windows (amd64):**

Download `cora_latest_windows_amd64.zip` from the [Releases page](https://github.com/clairitydev/cora-cli/releases), extract, and add to your PATH.

### Build from Source

Requires Go 1.22 or later.

```bash
git clone https://github.com/clairitydev/cora-cli.git
cd cora-cli
make install
```

## Quick Start

### 1. Create an API Token

Visit [https://thecora.app/settings/tokens](https://thecora.app/settings/tokens) to create an API token.

### 2. Configure the CLI

```bash
cora configure --token YOUR_API_TOKEN
```

This stores your token securely in `~/.config/cora/credentials.json`.

### 3. Upload Terraform State

```bash
# From your Terraform directory
terraform show -json | cora upload --workspace my-app-prod
```

## Usage

### Upload Command

The `upload` command reads Terraform state JSON and uploads it to Cora.

```bash
# Pipe from terraform show (recommended)
terraform show -json | cora upload --workspace my-app-prod

# Read from a file
cora upload --workspace my-app-prod --file terraform.tfstate.json

# With explicit token (overrides stored config)
terraform show -json | cora upload --workspace my-app-prod --token YOUR_TOKEN
```

**Flags:**
| Flag | Short | Description |
|------|-------|-------------|
| `--workspace` | `-w` | Target workspace name (required) |
| `--file` | `-f` | Path to Terraform state file (reads from stdin if not provided) |
| `--token` | | API token (overrides CORA_TOKEN env var and stored config) |
| `--api-url` | | API URL (default: https://thecora.app) |

### Configure Command

The `configure` command stores your API token locally for future use.

```bash
# With flag
cora configure --token YOUR_API_TOKEN

# Interactive prompt
cora configure

# With custom API URL (for self-hosted instances)
cora configure --token YOUR_TOKEN --api-url https://cora.example.com
```

Configuration is stored in `~/.config/cora/credentials.json` with secure permissions (0600).

### Version Command

```bash
cora version
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `CORA_TOKEN` | API token (alternative to `--token` flag or stored config) |
| `CORA_API_URL` | API URL (alternative to `--api-url` flag) |

**Priority order:**
1. Command-line flags
2. Environment variables
3. Stored configuration (`~/.config/cora/credentials.json`)

## Atlantis Integration

The Cora CLI is designed to work seamlessly with [Atlantis](https://www.runatlantis.io/).

### Setup

1. **Create an API token** at [https://thecora.app/settings/tokens](https://thecora.app/settings/tokens)

2. **Add the token to your Atlantis environment** as `CORA_TOKEN`

3. **Install the CLI on your Atlantis server:**
   ```bash
   curl -L https://github.com/clairitydev/cora-cli/releases/latest/download/cora_linux_amd64.tar.gz | tar xz
   sudo mv cora /usr/local/bin/
   ```

4. **Update your `atlantis.yaml`:**

```yaml
version: 3
projects:
  - name: my-app
    dir: .
    workspace: prod
    workflow: cora

workflows:
  cora:
    apply:
      steps:
        - apply
        - run: terraform show -json | cora upload --workspace ${WORKSPACE}
```

### Dynamic Workspace Names

You can use Atlantis variables to generate dynamic workspace names:

```yaml
workflows:
  cora:
    apply:
      steps:
        - apply
        - run: terraform show -json | cora upload --workspace "${PROJECT_NAME}-${WORKSPACE}"
```

### Multiple Projects

For monorepos with multiple Terraform projects:

```yaml
version: 3
projects:
  - name: networking
    dir: terraform/networking
    workflow: cora
  - name: application
    dir: terraform/application
    workflow: cora

workflows:
  cora:
    apply:
      steps:
        - apply
        - run: terraform show -json | cora upload --workspace ${PROJECT_NAME}
```

## CI/CD Integration

### GitHub Actions

```yaml
name: Deploy Infrastructure

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup Terraform
        uses: hashicorp/setup-terraform@v3

      - name: Install Cora CLI
        run: |
          curl -L https://github.com/clairitydev/cora-cli/releases/latest/download/cora_linux_amd64.tar.gz | tar xz
          sudo mv cora /usr/local/bin/

      - name: Terraform Apply
        run: terraform apply -auto-approve
        env:
          AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
          AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}

      - name: Upload to Cora
        run: terraform show -json | cora upload --workspace production
        env:
          CORA_TOKEN: ${{ secrets.CORA_TOKEN }}
```

### GitLab CI

```yaml
deploy:
  stage: deploy
  image: hashicorp/terraform:latest
  before_script:
    - apk add --no-cache curl
    - curl -L https://github.com/clairitydev/cora-cli/releases/latest/download/cora_linux_amd64.tar.gz | tar xz
    - mv cora /usr/local/bin/
  script:
    - terraform apply -auto-approve
    - terraform show -json | cora upload --workspace $CI_ENVIRONMENT_NAME
  environment:
    name: production
  variables:
    CORA_TOKEN: $CORA_TOKEN
```

## Troubleshooting

### "No API token provided"

Make sure you have either:
- Set the `CORA_TOKEN` environment variable
- Run `cora configure --token YOUR_TOKEN`
- Passed `--token YOUR_TOKEN` to the command

### "Authentication failed"

Your token may be expired or revoked. Create a new token at [https://thecora.app/settings/tokens](https://thecora.app/settings/tokens).

### "Invalid Terraform state"

The CLI expects valid Terraform state JSON. Make sure you're using `terraform show -json` (not just `terraform show`).

### Large State Files

For very large state files, the upload may take a few seconds. The CLI has a 60-second timeout by default. If you're experiencing timeouts, check your network connection.

### "CLI Upgrade Required"

If you see this error, your CLI version is too old and no longer supported. Download the latest version from the [Releases page](https://github.com/clairitydev/cora-cli/releases).

### Upgrade Warnings

The CLI will display a warning if a newer version is available. These warnings don't block uploads but indicate you should upgrade soon for the best experience and latest features.

## Security

- API tokens are stored with `0600` permissions (user read/write only)
- Tokens are transmitted over HTTPS
- The CLI never logs or displays your full token

## License

MIT License - see [LICENSE](LICENSE) for details.

## Support

- **Documentation:** [https://thecora.app/docs](https://thecora.app/docs)
- **Issues:** [https://github.com/clairitydev/cora-cli/issues](https://github.com/clairitydev/cora-cli/issues)
- **Website:** [https://thecora.app](https://thecora.app)
