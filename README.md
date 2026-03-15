# VersionConductor

[![Go CI][go-ci-svg]][go-ci-url]
[![Go Lint][go-lint-svg]][go-lint-url]
[![Go SAST][go-sast-svg]][go-sast-url]
[![Go Report Card][goreport-svg]][goreport-url]
[![Docs][docs-godoc-svg]][docs-godoc-url]
[![Visualization][viz-svg]][viz-url]
[![License][license-svg]][license-url]

 [go-ci-svg]: https://github.com/grokify/versionconductor/actions/workflows/go-ci.yaml/badge.svg?branch=main
 [go-ci-url]: https://github.com/grokify/versionconductor/actions/workflows/go-ci.yaml
 [go-lint-svg]: https://github.com/grokify/versionconductor/actions/workflows/go-lint.yaml/badge.svg?branch=main
 [go-lint-url]: https://github.com/grokify/versionconductor/actions/workflows/go-lint.yaml
 [go-sast-svg]: https://github.com/grokify/versionconductor/actions/workflows/go-sast-codeql.yaml/badge.svg?branch=main
 [go-sast-url]: https://github.com/grokify/versionconductor/actions/workflows/go-sast-codeql.yaml
 [goreport-svg]: https://goreportcard.com/badge/github.com/grokify/versionconductor
 [goreport-url]: https://goreportcard.com/report/github.com/grokify/versionconductor
 [docs-godoc-svg]: https://pkg.go.dev/badge/github.com/grokify/versionconductor
 [docs-godoc-url]: https://pkg.go.dev/github.com/grokify/versionconductor
 [viz-svg]: https://img.shields.io/badge/visualizaton-Go-blue.svg
 [viz-url]: https://mango-dune-07a8b7110.1.azurestaticapps.net/?repo=grokify%2Fversionconductor
 [loc-svg]: https://tokei.rs/b1/github/grokify/versionconductor
 [repo-url]: https://github.com/grokify/versionconductor
 [license-svg]: https://img.shields.io/badge/license-MIT-blue.svg
 [license-url]: https://github.com/grokify/versionconductor/blob/master/LICENSE

Automated dependency PR management and maintenance releases for GitHub repositories.

Part of the DevOpsOrchestra suite alongside [PipelineConductor](https://github.com/grokify/pipelineconductor).

## Features

- **Scan** - Find Renovate/Dependabot PRs across organizations
- **Review** - Auto-approve dependency PRs based on Cedar policies
- **Merge** - Auto-merge approved PRs with configurable strategies
- **Release** - Create maintenance releases when dependencies are updated

## Installation

```bash
go install github.com/grokify/versionconductor/cmd/versionconductor@latest
```

## Quick Start

Set your GitHub token:

```bash
export GITHUB_TOKEN=ghp_your_token_here
```

Scan for dependency PRs:

```bash
versionconductor scan --orgs myorg
```

Review PRs (dry-run by default):

```bash
versionconductor review --orgs myorg
```

Merge approved PRs:

```bash
versionconductor merge --orgs myorg --execute
```

Create maintenance releases:

```bash
versionconductor release --orgs myorg --execute
```

## Commands

### scan

List all open dependency PRs across repositories.

```bash
# Scan an organization
versionconductor scan --orgs myorg

# Scan specific repositories
versionconductor scan --repos owner/repo1,owner/repo2

# Filter by dependency bot
versionconductor scan --orgs myorg --bot renovate

# Filter by update type
versionconductor scan --orgs myorg --update-type patch,minor

# Output as JSON
versionconductor scan --orgs myorg --format json
```

### review

Auto-approve dependency PRs that meet policy criteria.

```bash
# Dry-run (default)
versionconductor review --orgs myorg

# Actually approve
versionconductor review --orgs myorg --execute

# Use specific profile
versionconductor review --orgs myorg --profile conservative --execute
```

### merge

Merge approved dependency PRs.

```bash
# Dry-run (default)
versionconductor merge --orgs myorg

# Actually merge
versionconductor merge --orgs myorg --execute

# Use squash merge
versionconductor merge --orgs myorg --strategy squash --execute

# Limit merges per run
versionconductor merge --orgs myorg --max-prs 5 --execute
```

### release

Create maintenance releases for repositories with merged dependency PRs.

```bash
# Dry-run (default)
versionconductor release --orgs myorg

# Create releases
versionconductor release --orgs myorg --execute

# Only PRs merged since a date
versionconductor release --orgs myorg --since 2025-01-01 --execute

# Create as drafts for review
versionconductor release --orgs myorg --draft --execute
```

## Merge Profiles

VersionConductor includes three built-in merge profiles:

| Profile | Description |
|---------|-------------|
| `aggressive` | Merge all passing PRs immediately |
| `balanced` | Wait 24h, auto-merge patch and minor only |
| `conservative` | Wait 48h, auto-merge patch only, require approval for others |

Use profiles with the `--profile` flag:

```bash
versionconductor merge --orgs myorg --profile balanced --execute
```

## Configuration

Create a `.versionconductor.yaml` file in your home directory or project root:

```yaml
orgs:
  - myorg
  - anotherorg

token: ${GITHUB_TOKEN}  # Will read from environment

merge:
  profile: balanced
  strategy: squash
  delete-branch: true

release:
  generate-notes: true
  prefix: v
```

## Cedar Policies

VersionConductor uses [Cedar](https://www.cedarpolicy.com/) for fine-grained policy control.

Example policy for auto-merging patch updates:

```cedar
permit(
    principal,
    action == Action::"merge",
    resource
)
when {
    context.pr.isDependency == true &&
    context.ci.allPassed == true &&
    context.pr.ageHours >= 1 &&
    context.dependency.isPatch == true &&
    context.pr.mergeable == true &&
    context.pr.draft == false
};
```

## Output Formats

All commands support multiple output formats:

- `table` (default) - Human-readable text table
- `json` - JSON for programmatic consumption
- `markdown` - Markdown for reports and documentation
- `csv` - CSV for spreadsheet import

```bash
versionconductor scan --orgs myorg --format json
```

## Safety Features

1. **Dry-run by default** - All write operations require `--execute`
2. **Policy-driven** - No auto-merge without explicit policy
3. **Rate limiting** - Respects GitHub API limits
4. **Audit trail** - All actions logged with timestamps

## Development

```bash
# Clone
git clone https://github.com/grokify/versionconductor
cd versionconductor

# Build
go build ./cmd/versionconductor

# Test
go test -v ./...

# Lint
golangci-lint run
```

## License

MIT License - see [LICENSE](LICENSE) for details.
