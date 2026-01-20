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
| `--workspace` | `-w` | Target workspace name (auto-detected in Atlantis/GitHub Actions) |
| `--file` | `-f` | Path to Terraform state file (reads from stdin if not provided) |
| `--source` | | Source identifier (auto-detected: 'atlantis', 'github-actions', or 'cli') |
| `--no-filter` | | Disable sensitive data filtering |
| `--filter-dry-run` | | Show what would be filtered without uploading |
| `--output-format` | | Output format for dry-run: `text` or `json` (default: text) |
| `--token` | | API token (overrides CORA_TOKEN env var and stored config) |
| `--api-url` | | API URL (default: https://thecora.app) |
| `--verbose` | `-v` | Enable verbose output |

### Review Command

The `review` command uploads Terraform plan JSON for PR risk assessment. This is useful for analyzing infrastructure changes before they are applied.

```bash
# Pipe from terraform show (with a plan file)
terraform show -json tfplan | cora review --workspace my-app-prod

# Read from a file
cora review --workspace my-app-prod --file plan.json

# With GitHub PR context (enables automatic PR comments)
terraform show -json tfplan | cora review \
  --workspace my-app-prod \
  --github-owner myorg \
  --github-repo myrepo \
  --pr-number 123 \
  --commit-sha abc123
```

**Flags:**
| Flag | Short | Description |
|------|-------|-------------|
| `--workspace` | `-w` | Target workspace name (auto-detected in Atlantis/GitHub Actions) |
| `--file` | `-f` | Path to Terraform plan JSON file (reads from stdin if not provided) |
| `--source` | | Source identifier (auto-detected: 'atlantis', 'github-actions', or 'cli') |
| `--github-owner` | | GitHub repository owner (auto-detected in Atlantis/GitHub Actions) |
| `--github-repo` | | GitHub repository name (auto-detected in Atlantis/GitHub Actions) |
| `--pr-number` | | GitHub PR number (auto-detected in Atlantis/GitHub Actions) |
| `--commit-sha` | | Git commit SHA (auto-detected in Atlantis/GitHub Actions) |
| `--no-filter` | | Disable sensitive data filtering |
| `--filter-dry-run` | | Show what would be filtered without uploading |
| `--output-format` | | Output format for dry-run: `text` or `json` (default: text) |
| `--token` | | API token (overrides CORA_TOKEN env var and stored config) |
| `--api-url` | | API URL (default: https://thecora.app) |
| `--verbose` | `-v` | Enable verbose output |

**Output:**
```
✅ Plan analyzed successfully
   Plan ID: abc123-def456

📊 Risk Assessment
   Level: 🟡 Medium
   Score: 45.0
   Rules triggered: 3

🔗 View details: https://thecora.app/pr-reviews/abc123-def456

💬 GitHub comment posted: https://github.com/myorg/myrepo/pull/123#issuecomment-12345
```

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

### Init Command

The `init` command creates a `.cora.yaml` configuration file in the current directory with default settings and helpful comments.

```bash
# Create a new config file with full documentation
cora init

# Overwrite an existing config file
cora init --force

# Create a minimal config without comments
cora init --minimal
```

**Flags:**
| Flag | Short | Description |
|------|-------|-------------|
| `--force` | `-f` | Overwrite existing .cora.yaml file |
| `--minimal` | | Generate minimal config without comments |

The generated `.cora.yaml` file customizes how sensitive data is filtered from your Terraform state before uploading. See [Configuration documentation](https://thecora.app/docs/sensitive-data#project-config) for details on all available options.

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

## Environment Auto-Detection

The Cora CLI automatically detects when it runs inside Atlantis or GitHub Actions and populates context from native environment variables. This eliminates the need to manually pass flags like `--source`, `--workspace`, and GitHub PR context.

### How It Works

**Detection priority:**
1. Atlantis (detected via `ATLANTIS_TERRAFORM_VERSION`)
2. GitHub Actions (detected via `GITHUB_ACTIONS=true`)

**Auto-populated values:**

| Flag | Atlantis Source | GitHub Actions Source |
|------|-----------------|----------------------|
| `--source` | `"atlantis"` | `"github-actions"` |
| `--workspace` | `PROJECT_NAME-WORKSPACE` (or just `WORKSPACE`) | `GITHUB_HEAD_REF` or `GITHUB_REF_NAME` |
| `--github-owner` | `BASE_REPO_OWNER` | Parsed from `GITHUB_REPOSITORY` |
| `--github-repo` | `BASE_REPO_NAME` | Parsed from `GITHUB_REPOSITORY` |
| `--pr-number` | `PULL_NUM` | Extracted from `GITHUB_REF` or event payload |
| `--commit-sha` | `HEAD_COMMIT` | `GITHUB_SHA` |

### Override Behavior

Explicit flags always override auto-detected values. For example:

```bash
# In Atlantis, this will use "custom-workspace" instead of the auto-detected value
terraform show -json | cora upload --workspace custom-workspace
```

Unset flags are auto-populated while explicitly set flags are preserved.

### Verbose Output

Use `--verbose` to see what was auto-detected:

```bash
terraform show -json | cora upload -v
```

**Example output:**
```
🔍 Auto-detected: Atlantis, repo=myorg/infra, PR=#123, workspace=my-app-prod
   → source=atlantis (auto-detected)
   → workspace=my-app-prod (auto-detected)
   → github-owner=myorg (auto-detected)
   → github-repo=infra (auto-detected)
   → pr-number=123 (auto-detected)
   → commit-sha=abc123 (auto-detected)
```

### Warnings

If the CLI detects an environment but cannot extract complete context (e.g., running in GitHub Actions on a `push` event with no PR), it will print a warning:

```
⚠️  GitHub Actions detected but no PR context found (event: push). GitHub PR comments will be disabled.
```

## Verbose Mode

Use the `-v` or `--verbose` flag to see detailed output about what the CLI is doing:

```bash
terraform show -json | cora upload --workspace my-app -v
```

**Example output:**
```
🔑 Using token from CORA_TOKEN environment variable
🌐 Using default API URL: https://thecora.app
📡 Fetching service discovery from /.well-known/cora.json
🔒 Filter config source: defaults
🔒 Applying sensitive data filter...
🔒 Filtering sensitive data: 0 resources omitted, 3 attributes omitted
   🚫 aws_db_instance.main.password
   🚫 aws_db_instance.main.master_password
   🚫 aws_secretsmanager_secret.api_key.secret_string
📊 Filtered state size: 45123 bytes (original: 45892 bytes)
📤 POST https://thecora.app/api/terraform-state?workspace=my-app
📥 Response: 201 Created
State uploaded successfully to workspace 'my-app'
Resources: 47
```

Verbose output is written to stderr so it doesn't interfere with piped JSON output.

## Sensitive Data Filtering

The Cora CLI automatically filters sensitive data from your Terraform state and plan files before uploading. This helps ensure passwords, secrets, API keys, and other sensitive values never leave your environment.

### How It Works

Filtering is **enabled by default**. The CLI:

1. **Omits entire resources** of sensitive types (e.g., `aws_secretsmanager_secret_version`, `random_password`)
2. **Omits attributes** that match sensitive patterns (e.g., `password`, `secret`, `api_key`)
3. **Honors Terraform's `sensitive_attributes`** markers from the state file

### Dry Run Mode

Preview what would be filtered without uploading:

```bash
terraform show -json | cora upload --workspace my-app --filter-dry-run
```

**Example output:**
```
🔒 Sensitive Data Filter - Dry Run Report
──────────────────────────────────────────────────

📊 Summary
   Resources: 47 total, 2 omitted
   Attributes: 156 total, 5 omitted
   Config source: defaults

🗑️  Omitted Resources
   ⛔ aws_secretsmanager_secret_version.api_key
      resource type 'aws_secretsmanager_secret_version' is in omit list
   ⛔ random_password.db_password
      resource type 'random_password' is in omit list

🔐 Omitted Attributes
   🚫 aws_db_instance.main.password
      attribute name matches omit pattern
   🚫 aws_db_instance.main.master_password
      attribute name matches omit pattern

ℹ️  Use --no-filter to upload without filtering (if allowed by your organization)
```

For machine-readable output (useful in CI):

```bash
terraform show -json | cora upload --workspace my-app --filter-dry-run --output-format json
```

### Disabling Filtering

To upload without filtering (not recommended):

```bash
terraform show -json | cora upload --workspace my-app --no-filter
```

> **Note:** Your organization may enforce filtering. If so, using `--no-filter` will result in an error.

### Default Sensitive Patterns

**Resource types omitted entirely:**
- `aws_secretsmanager_secret_version`
- `aws_ssm_parameter`
- `random_password`
- `random_string`
- `tls_private_key`
- `acme_certificate`
- `vault_generic_secret`
- `vault_kv_secret`
- `vault_kv_secret_v2`
- `azurerm_key_vault_secret`
- `google_secret_manager_secret_version`

**Attribute patterns omitted:**
- `password`, `master_password`, `admin_password`
- `secret`, `secret_string`, `secret_binary`
- `api_key`, `api_secret`
- `token`, `auth_token`, `access_token`
- `private_key`, `private_key_pem`
- `access_key`, `secret_key`, `secret_access_key`
- `credential`, `credentials`
- `connection_string`, `connection_url`

### Configuration File (.cora.yaml)

You can customize filtering by creating a `.cora.yaml` file in your project directory (or any parent directory):

```yaml
version: 1

filtering:
  # Additional resource types to omit (merged with defaults)
  omit_resource_types:
    - custom_secret_resource
    - my_internal_credential

  # Additional attribute patterns to omit (merged with defaults)
  omit_attributes:
    - internal_api_key
    - my_custom_secret

  # Attributes to never omit (overrides defaults)
  preserve_attributes:
    - public_dns_name
    - public_ip

  # Whether to honor Terraform's sensitive_attributes markers (default: true)
  honor_terraform_sensitive: true
```

The CLI searches for `.cora.yaml` or `.cora.yml` starting from the current directory and walking up to parent directories.

### Configuration Priority

1. Command-line flags (`--no-filter`)
2. Environment variables
3. Project config file (`.cora.yaml`)
4. Platform settings (from your organization)
5. Built-in defaults

## Atlantis Integration

The Cora CLI is designed to work seamlessly with [Atlantis](https://www.runatlantis.io/). When running inside Atlantis, the CLI **automatically detects** the environment and extracts context from Atlantis native environment variables.

### Quick Setup with `cora atlantis init`

The fastest way to add Cora to your Atlantis configuration is with the `atlantis init` command:

```bash
# Navigate to your repo root
cd /path/to/your/repo

# Automatically add Cora steps to your atlantis.yaml
cora atlantis init
```

This command:
- Finds your `atlantis.yaml` file automatically
- Adds `cora review` after plan steps (for PR risk assessment)
- Adds `cora upload` after apply steps (to capture state)
- Is idempotent - running it twice won't duplicate steps
- Shows you exactly what will change before modifying

**Flags:**
| Flag | Description |
|------|-------------|
| `--config <path>` | Path to atlantis.yaml (auto-detected by default) |
| `--dry-run` | Preview changes without modifying the file |
| `--backup` | Create a backup file before modifying |
| `--force` | Skip confirmation prompt |

**Examples:**

```bash
# Preview what would change
cora atlantis init --dry-run

# Create a backup before modifying
cora atlantis init --backup

# Specify a custom config path
cora atlantis init --config ./infra/atlantis.yaml
```

### Manual Setup

If you prefer to configure manually:

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
        # Workspace is auto-detected from PROJECT_NAME and WORKSPACE
        - run: terraform show -json | cora upload
    plan:
      steps:
        - init
        - plan
        # PR context is auto-detected - GitHub comments work automatically
        - run: terraform show -json $PLANFILE | cora review
```

### What Gets Auto-Detected

In Atlantis, the CLI reads these native environment variables:
- `PROJECT_NAME` + `WORKSPACE` → combined into workspace name
- `BASE_REPO_OWNER`, `BASE_REPO_NAME` → GitHub repo context
- `PULL_NUM`, `HEAD_COMMIT` → PR context for automated comments

### Manual Override

You can still override any auto-detected value:

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

When running in GitHub Actions, the CLI **automatically detects** the environment and extracts PR context from GitHub's native environment variables. This enables automatic PR comments without manual configuration.

```yaml
name: Deploy Infrastructure

on:
  push:
    branches: [main]
  pull_request:
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

      - name: Terraform Plan
        run: terraform plan -out=tfplan
        env:
          AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
          AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}

      # PR context is auto-detected - no need to pass --github-owner, --github-repo, etc.
      - name: Review Plan
        if: github.event_name == 'pull_request'
        run: terraform show -json tfplan | cora review --workspace production
        env:
          CORA_TOKEN: ${{ secrets.CORA_TOKEN }}

      - name: Terraform Apply
        if: github.ref == 'refs/heads/main'
        run: terraform apply -auto-approve tfplan
        env:
          AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
          AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}

      # Source is auto-detected as "github-actions"
      - name: Upload State to Cora
        if: github.ref == 'refs/heads/main'
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

### "Invalid Terraform plan"

For the `review` command, make sure you're providing a plan file, not state:
```bash
# First, create a plan
terraform plan -out=tfplan

# Then, show the plan as JSON
terraform show -json tfplan | cora review --workspace my-app
```

### Large State Files

For very large state files, the upload may take a few seconds. The CLI has a 60-second timeout by default. If you're experiencing timeouts, check your network connection.

### "CLI Upgrade Required"

If you see this error, your CLI version is too old and no longer supported. Download the latest version from the [Releases page](https://github.com/clairitydev/cora-cli/releases).

### Upgrade Warnings

The CLI will display a warning if a newer version is available. These warnings don't block uploads but indicate you should upgrade soon for the best experience and latest features.

## Service Discovery

The Cora CLI uses service discovery to dynamically fetch API endpoints from the server. On first request, the CLI fetches `/.well-known/cora.json` from the API URL and caches the endpoint configuration for 1 hour.

This allows the server to:
- Version API endpoints without breaking older CLIs
- Provide version requirements and upgrade guidance
- Enable or disable features dynamically

If service discovery fails (e.g., for older server versions), the CLI falls back to default endpoints.

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
