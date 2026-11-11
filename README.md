# go-aws-whoami

**Show what AWS account and identity you're using.**

`aws-whoami` is a small command-line tool (and Go library) that tells you, at a
glance, which AWS account, principal and region your current credentials
resolve to. It is a Go port of Ben Kehoe's Python
[`aws-whoami`](https://github.com/benkehoe/aws-whoami); this repository is
named `go-aws-whoami` only to distinguish it from the original.

```
$ aws-whoami
Account:         123456789012
                 my-account-alias
Region:          us-east-2
AssumedRole:     MY-ROLE
RoleSessionName: ben
UserId:          SOMEOPAQUEID:ben
Arn:             arn:aws:sts::123456789012:assumed-role/MY-ROLE/ben
```

## Contents

- [Why?](#why)
- [Getting started](#getting-started)
- [How-to guides](#how-to-guides)
  - [Check a specific profile](#check-a-specific-profile)
  - [Use the output in scripts](#use-the-output-in-scripts)
  - [Guard a script against the wrong account](#guard-a-script-against-the-wrong-account)
  - [Disable the account alias lookup](#disable-the-account-alias-lookup)
  - [Troubleshoot a failure](#troubleshoot-a-failure)
  - [Use it as a Go library](#use-it-as-a-go-library)
  - [Test code that uses the library](#test-code-that-uses-the-library)
- [Reference](#reference)
  - [Command line](#command-line)
  - [Environment variables](#environment-variables)
  - [Exit status](#exit-status)
  - [Text output](#text-output)
  - [JSON output](#json-output)
  - [Principal types](#principal-types)
  - [IAM permissions](#iam-permissions)
  - [Library API](#library-api)
- [Explanation](#explanation)
  - [How it works](#how-it-works)
  - [Region resolution](#region-resolution)
  - [Why look up the account alias?](#why-look-up-the-account-alias)
  - [IAM Identity Center (SSO) detection](#iam-identity-center-sso-detection)
  - [Differences from the Python version](#differences-from-the-python-version)
- [Development](#development)
- [Licence](#licence)

## Why?

[`aws sts get-caller-identity`](https://docs.aws.amazon.com/cli/latest/reference/sts/get-caller-identity.html)
returns the identity of the caller, but:

- the ARN is a lot to parse visually;
- it doesn't tell you which region your credentials are configured for;
- account numbers are hard to remember, and the alias is far more recognisable.

`aws-whoami` splits the ARN into its meaningful parts, shows the configured
region and adds your account alias, so you can confirm where you are before you
do something you'll regret.

## Getting started

This walkthrough takes you from nothing to seeing your identity.

### 1. Install

Download a pre-built binary for your platform from the
[releases page](https://github.com/scottbrown/go-aws-whoami/releases). Archives
are published for Linux, macOS and Windows on both `amd64` and `arm64`:

```bash
# e.g. macOS on Apple silicon
VERSION=v1.0.0
curl -LO "https://github.com/scottbrown/go-aws-whoami/releases/download/${VERSION}/aws-whoami_${VERSION}_darwin_arm64.tar.gz"
curl -LO "https://github.com/scottbrown/go-aws-whoami/releases/download/${VERSION}/checksums.txt"
shasum -a 256 --check --ignore-missing checksums.txt
tar xzf "aws-whoami_${VERSION}_darwin_arm64.tar.gz" aws-whoami
sudo mv aws-whoami /usr/local/bin/
```

On macOS, binaries downloaded with a browser are quarantined by Gatekeeper; clear
it with `xattr -d com.apple.quarantine aws-whoami`. (`curl` downloads are not
quarantined.)

Or, with Go 1.27 or later:

```bash
go install github.com/scottbrown/go-aws-whoami/cmd/aws-whoami@latest
```

This places `aws-whoami` in `$(go env GOPATH)/bin`. Make sure that directory is
on your `PATH`.

Alternatively, build from source with [Task](https://taskfile.dev):

```bash
git clone https://github.com/scottbrown/go-aws-whoami.git
cd go-aws-whoami
task build
./.build/aws-whoami --version
```

### 2. Make sure you have credentials

`aws-whoami` uses the same credential sources as the AWS CLI. If
`aws sts get-caller-identity` works, so will `aws-whoami`. If you use IAM
Identity Center, sign in first:

```bash
aws sso login --profile my-profile
```

### 3. Run it

```bash
aws-whoami
```

### 4. Read the output

```
Account:         123456789012          ← the account ID
                 my-account-alias      ← its alias(es), if you can see them
Region:          us-east-2             ← the region your config resolves to
AssumedRole:     MY-ROLE               ← principal type and name
RoleSessionName: ben                   ← only for assumed roles
UserId:          SOMEOPAQUEID:ben      ← the unique ID from STS
Arn:             arn:aws:sts::123456789012:assumed-role/MY-ROLE/ben
```

That's it. The rest of this document covers specific tasks, the full reference
and the reasoning behind the behaviour.

## How-to guides

### Check a specific profile

```bash
aws-whoami --profile production
```

`--profile` takes precedence over `AWS_PROFILE`. You can also set the
environment variable instead:

```bash
AWS_PROFILE=production aws-whoami
```

To check every profile you have configured:

```bash
for p in $(aws configure list-profiles); do
  echo "== $p"
  aws-whoami --profile "$p" || true
done
```

### Use the output in scripts

`--json` prints a single-line JSON object, which is easy to consume with
[`jq`](https://jqlang.org):

```bash
aws-whoami --json | jq -r .Account
# 123456789012

aws-whoami --json | jq -r '.AccountAliases[0] // .Account'
# my-account-alias (or the account ID if there is no alias)

aws-whoami --json | jq -r '.SSOPermissionSet // .Name'
# AdministratorAccess (or the role/user name if not SSO)
```

### Guard a script against the wrong account

Fail fast before running something destructive:

```bash
#!/usr/bin/env bash
set -euo pipefail

expected="123456789012"
actual="$(aws-whoami --json | jq -r .Account)"

if [[ "$actual" != "$expected" ]]; then
  echo "Refusing to run: expected account $expected, got $actual" >&2
  exit 1
fi

terraform apply
```

### Disable the account alias lookup

Some organisations alert on `AccessDenied` errors from CloudTrail. If your
principal isn't allowed `iam:ListAccountAliases`, every run of `aws-whoami`
would generate one. Use `AWS_WHOAMI_DISABLE_ACCOUNT_ALIAS` to avoid the call.

Disable it everywhere:

```bash
export AWS_WHOAMI_DISABLE_ACCOUNT_ALIAS=true
```

Disable it only for particular accounts, roles or sessions by giving a
comma-separated list:

```bash
# Skip for one account, for any account ending in 9012, and for the ReadOnly role
export AWS_WHOAMI_DISABLE_ACCOUNT_ALIAS="111122223333,9012,ReadOnly"
```

See [Environment variables](#environment-variables) for the exact matching rules.

### Troubleshoot a failure

On failure, `aws-whoami` prints a single line and exits with status 1:

```
ERROR [config.SharedConfigProfileNotExistError]: failed to get shared config profile, does-not-exist
```

The value in square brackets is the Go type of the error, which usually tells
you which layer failed. Common cases:

| Error type / message                                   | Likely cause and fix                                            |
|--------------------------------------------------------|-----------------------------------------------------------------|
| `config.SharedConfigProfileNotExistError`              | Typo in `--profile` or `AWS_PROFILE`; run `aws configure list-profiles` |
| `*smithy.OperationError` … `failed to refresh cached credentials` | No usable credentials were found; configure some or log in |
| `*smithy.OperationError` … `token has expired` / `InvalidGrantException` | Your SSO session expired; run `aws sso login`         |
| `*smithy.OperationError` … `ExpiredToken`              | Temporary credentials expired; re-assume the role               |
| `*smithy.OperationError` … `InvalidClientTokenId`      | Access key is wrong, deleted or deactivated                     |

For more detail, add `--debug`. It prints every error in the wrapped chain,
indented by depth, before the final `ERROR` line:

```bash
aws-whoami --debug
```

### Use it as a Go library

```bash
go get github.com/scottbrown/go-aws-whoami
```

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	awswhoami "github.com/scottbrown/go-aws-whoami"
)

func main() {
	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile("production"))
	if err != nil {
		log.Fatal(err)
	}

	policy := awswhoami.ParseAliasPolicy(os.Getenv(awswhoami.DisableAccountAliasEnvVar))

	info, err := awswhoami.Whoami(ctx, cfg, policy)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(awswhoami.Format(info))

	if info.Type == awswhoami.PrincipalTypeAssumedRole {
		fmt.Println("session:", info.RoleSessionName)
	}

	_ = json.NewEncoder(os.Stdout).Encode(info)
}
```

### Test code that uses the library

`Client` depends on two small interfaces, `STSAPI` and `IAMAPI`, rather than
concrete SDK clients, so you can supply hand-written fakes without a mocking
library:

```go
type fakeSTS struct{}

func (fakeSTS) GetCallerIdentity(context.Context, *sts.GetCallerIdentityInput, ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{
		Account: aws.String("123456789012"),
		Arn:     aws.String("arn:aws:iam::123456789012:user/bob"),
		UserId:  aws.String("AIDAEXAMPLE"),
	}, nil
}

type fakeIAM struct{}

func (fakeIAM) ListAccountAliases(context.Context, *iam.ListAccountAliasesInput, ...func(*iam.Options)) (*iam.ListAccountAliasesOutput, error) {
	return &iam.ListAccountAliasesOutput{AccountAliases: []string{"test-alias"}}, nil
}

func TestSomething(t *testing.T) {
	c := &awswhoami.Client{STS: fakeSTS{}, IAM: fakeIAM{}, Region: "ca-central-1"}
	info, err := c.Whoami(context.Background(), awswhoami.EnableAccountAlias())
	// ...
}
```

## Reference

### Command line

```
aws-whoami [flags]
```

| Flag              | Type   | Default | Description                                         |
|-------------------|--------|---------|-----------------------------------------------------|
| `--profile`       | string | —       | Named profile from the shared config/credentials files |
| `--json`          | bool   | false   | Print a JSON object instead of text                 |
| `--debug`         | bool   | false   | On failure, print the full wrapped error chain      |
| `-v`, `--version` | bool   | false   | Print the version and exit                          |
| `-h`, `--help`    | bool   | false   | Print help and exit                                 |

Positional arguments are rejected.

### Environment variables

| Variable                           | Description |
|------------------------------------|-------------|
| `AWS_WHOAMI_DISABLE_ACCOUNT_ALIAS` | Controls the `iam:ListAccountAliases` call (see below) |
| `AWS_PROFILE`                      | Profile to use when `--profile` isn't given |
| `AWS_REGION`, `AWS_DEFAULT_REGION` | Region to report and use |
| `AWS_CONFIG_FILE`, `AWS_SHARED_CREDENTIALS_FILE` | Alternative locations for the shared config files |
| `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN` | Static credentials |

All `AWS_*` variables other than `AWS_WHOAMI_DISABLE_ACCOUNT_ALIAS` are handled by
the [AWS SDK for Go v2](https://docs.aws.amazon.com/sdkref/latest/guide/settings-reference.html);
see its documentation for the complete list.

`AWS_WHOAMI_DISABLE_ACCOUNT_ALIAS` values (case-insensitive for the keywords):

| Value                     | Effect                                               |
|---------------------------|------------------------------------------------------|
| unset, empty, `0`, `false`| Look up aliases (default)                            |
| `1`, `true`               | Never look up aliases                                |
| anything else             | Comma-separated list of matchers; skip the lookup if any matches |

A matcher matches when it is:

- a **prefix or suffix** of the account ID, or
- **exactly equal** (case-sensitive) to the principal name, the ARN, or the role
  session name.

Whitespace around each matcher is trimmed and empty entries are ignored.

### Exit status

| Code | Meaning                                                        |
|------|----------------------------------------------------------------|
| `0`  | Success (including `--help` and `--version`)                   |
| `1`  | Any failure: invalid flags, configuration, credentials or API errors |

Errors are written to stderr as `ERROR [<Go error type>]: <message>`.

### Text output

Lines are printed in this order, with values aligned to the longest label:

| Label              | When shown                                         | Value |
|--------------------|----------------------------------------------------|-------|
| `Account:`         | Always                                             | Account ID |
| *(blank)*          | Once per alias                                     | Account alias |
| `Region:`          | Always                                             | Configured region (blank if none) |
| `AWS SSO:`         | Role is an IAM Identity Center role                | Permission set name |
| `<Type>:`          | Otherwise                                          | Principal name; label from [Principal types](#principal-types) |
| `RoleSessionName:` | Assumed roles only                                 | Session name |
| `UserId:`          | Always                                             | `UserId` from STS |
| `Arn:`             | Always                                             | Caller ARN |

Example for an IAM user:

```
Account: 123456789012
Region:  ca-central-1
User:    bob
UserId:  AIDAEXAMPLE
Arn:     arn:aws:iam::123456789012:user/bob
```

Example for an IAM Identity Center role:

```
Account:         123456789012
Region:          ca-central-1
AWS SSO:         AdministratorAccess
RoleSessionName: ben@example.com
UserId:          AROAEXAMPLE:ben@example.com
Arn:             arn:aws:sts::123456789012:assumed-role/AWSReservedSSO_AdministratorAccess_0123abcd/ben@example.com
```

### JSON output

A single line containing one object, followed by a newline. Shown here
pretty-printed:

```json
{
  "Account": "123456789012",
  "AccountAliases": ["my-account-alias"],
  "Arn": "arn:aws:sts::123456789012:assumed-role/MY-ROLE/ben",
  "Type": "assumed-role",
  "Name": "MY-ROLE",
  "RoleSessionName": "ben",
  "UserId": "SOMEOPAQUEID:ben",
  "Region": "us-east-2",
  "SSOPermissionSet": null
}
```

| Key                | Type             | Notes |
|--------------------|------------------|-------|
| `Account`          | string           | |
| `AccountAliases`   | array of strings | Always present; `[]` when disabled, denied or none exist |
| `Arn`              | string           | |
| `Type`             | string           | See [Principal types](#principal-types) |
| `Name`             | string           | |
| `RoleSessionName`  | string or `null` | `null` unless `Type` is `assumed-role` |
| `UserId`           | string           | |
| `Region`           | string or `null` | `null` if no region is configured |
| `SSOPermissionSet` | string or `null` | `null` unless the role is an IAM Identity Center role |

The keys and their `null` behaviour match the Python `aws-whoami` exactly.

### Principal types

| ARN form                                              | `Type`           | Text label      | `Name`             |
|-------------------------------------------------------|------------------|-----------------|--------------------|
| `arn:aws:iam::<acct>:user/<path/>name`                | `user`           | `User`          | `<path/>name`      |
| `arn:aws:sts::<acct>:assumed-role/<role>/<session>`   | `assumed-role`   | `AssumedRole`   | `<role>`           |
| `arn:aws:sts::<acct>:federated-user/<name>`           | `federated-user` | `FederatedUser` | `<name>`           |
| `arn:aws:iam::<acct>:root`                            | `root`           | `Root`          | `root`             |

Other types are passed through as-is; the label is the type converted from
`kebab-case` to `PascalCase`.

### IAM permissions

| Action                    | Required? | Notes |
|---------------------------|-----------|-------|
| `sts:GetCallerIdentity`   | Yes       | Always allowed; cannot be denied by IAM policy |
| `iam:ListAccountAliases`  | No        | `AccessDenied` is silently ignored; other errors are reported |

### Library API

Package `awswhoami` (`github.com/scottbrown/go-aws-whoami`). Full documentation
is available via `go doc github.com/scottbrown/go-aws-whoami`.

**Functions**

| Signature | Description |
|-----------|-------------|
| `Whoami(ctx context.Context, cfg aws.Config, policy AliasPolicy) (Info, error)` | Convenience for `NewClient(cfg).Whoami(ctx, policy)` |
| `NewClient(cfg aws.Config) *Client` | Builds STS and IAM clients; uses `us-east-1` for API calls if `cfg.Region` is empty |
| `Format(info Info) string` | Human-readable text, as printed by the CLI (no trailing newline) |
| `ParseAliasPolicy(value string) AliasPolicy` | Interprets an `AWS_WHOAMI_DISABLE_ACCOUNT_ALIAS` value |
| `EnableAccountAlias() AliasPolicy` | Always look up aliases (same as the zero value) |
| `DisableAccountAlias() AliasPolicy` | Never look up aliases |
| `DisableAccountAliasFor(values ...string) AliasPolicy` | Skip the lookup when any value matches |

**Types**

| Type | Description |
|------|-------------|
| `Client` | Fields `STS STSAPI`, `IAM IAMAPI`, `Region Region`; method `Whoami(ctx, AliasPolicy) (Info, error)` |
| `STSAPI` | Interface with `GetCallerIdentity` |
| `IAMAPI` | Interface with `ListAccountAliases` (same as `iam.ListAccountAliasesAPIClient`) |
| `Info` | Result; implements `json.Marshaler` and `json.Unmarshaler` |
| `AliasPolicy` | Opaque policy controlling the alias lookup |
| `AccountID`, `ARN`, `UserID`, `Region` | String types for the corresponding values |
| `PrincipalType` | String type with constants `PrincipalTypeUser`, `PrincipalTypeAssumedRole`, `PrincipalTypeFederatedUser`, `PrincipalTypeRoot`, and a `Label()` method |

**Constants**

| Name | Value |
|------|-------|
| `DisableAccountAliasEnvVar` | `"AWS_WHOAMI_DISABLE_ACCOUNT_ALIAS"` |

**`Info` fields**

| Field              | Type            | Notes |
|--------------------|-----------------|-------|
| `Account`          | `AccountID`     | |
| `AccountAliases`   | `[]string`      | Never nil |
| `Arn`              | `ARN`           | |
| `Type`             | `PrincipalType` | |
| `Name`             | `string`        | |
| `RoleSessionName`  | `string`        | Empty unless `Type` is `PrincipalTypeAssumedRole` |
| `UserID`           | `UserID`        | Serialised as `UserId` in JSON |
| `Region`           | `Region`        | Empty if none configured |
| `SSOPermissionSet` | `string`        | Empty unless the role is an IAM Identity Center role |

## Explanation

### How it works

1. The AWS SDK for Go v2 loads configuration and credentials, using `--profile`
   if given, otherwise the SDK's default chain.
2. `sts:GetCallerIdentity` returns the account, ARN and user ID.
3. The resource part of the ARN (everything after the last `:`) is split on the
   first `/` into the principal type and name. For assumed roles, the name is
   split again on its last `/` into the role name and session name.
4. If the role name looks like an IAM Identity Center role, the permission set
   is extracted.
5. Unless the alias policy says otherwise, `iam:ListAccountAliases` is called,
   following pagination. An `AccessDenied` error yields an empty list rather
   than a failure.
6. The result is printed as text or JSON.

### Region resolution

The reported region is whatever the SDK resolves from `AWS_REGION`,
`AWS_DEFAULT_REGION` or the profile's `region` setting. It's shown because
credentials and region are configured together, and acting in the wrong region
is as easy a mistake as acting in the wrong account.

If no region is configured, the Python version still worked because botocore
uses global endpoints for STS and IAM. The Go SDK refuses to make a call
without a region, so `aws-whoami` uses `us-east-1` for the API calls *only*,
and still reports the region as blank (`null` in JSON). The tool never pretends
a region is configured when it isn't.

### Why look up the account alias?

Account IDs are twelve digits that most people can't tell apart. Aliases such as
`acme-prod` are what humans actually recognise. The lookup is best-effort:
`sts:GetCallerIdentity` is always allowed, but `iam:ListAccountAliases` needs
an IAM permission that many roles don't have, so `AccessDenied` is treated as
"no aliases visible" rather than an error.

Because a denied call is still recorded in CloudTrail, and some security teams
alert on it, the lookup can be switched off globally or selectively with
`AWS_WHOAMI_DISABLE_ACCOUNT_ALIAS`. The selective form lets you keep the alias
in accounts where you have permission, and skip it in the ones where you don't.

### IAM Identity Center (SSO) detection

IAM Identity Center provisions a role in each account for every permission set,
named `AWSReservedSSO_<permission-set>_<random-tag>`. The role name is noisy;
the permission set is what you chose when you signed in. When the role name
starts with `AWSReservedSSO`, the text between the first and last underscores is
reported as the permission set, so permission sets containing underscores (for
example `My_Permission_Set`) are preserved.

### Differences from the Python version

The output format, JSON keys and environment variable behave the same as
`aws-whoami` 1.2.0, with these deliberate changes:

- **Root user credentials** (`arn:aws:iam::<acct>:root`) work instead of
  crashing; `Type` and `Name` are both `root`.
- **Empty matchers are ignored** in `AWS_WHOAMI_DISABLE_ACCOUNT_ALIAS`. In the
  Python version a trailing comma created an empty matcher that matched every
  account, silently disabling the lookup.
- **Whitespace around matchers is trimmed**, so `"a, b"` works as expected.
- **`--debug`** prints the Go error chain instead of a Python traceback.
- **No region configured** shows a blank region in text output, not `None`.
- **Invalid flags** exit with status 1 rather than argparse's 2.

## Development

Requires Go 1.27+ and [Task](https://taskfile.dev).

```
.
├── .github/
│   ├── dependabot.yaml                 Weekly Go module (/ and /tools) and daily Actions updates
│   └── workflows/
│       ├── ci.yaml                     task check, build and sbom on every push/PR
│       ├── sast.yaml                   CodeQL scan on main, PRs and weekly
│       ├── automerge-dependabot.yaml   Auto-merges Dependabot PRs
│       ├── verify-lockfile.yaml        Checks actions.lock on every PR and push to main
│       ├── release.yaml                Tag-triggered multi-platform release
│       └── actions.lock                Commits that every workflow's actions are pinned to
├── cmd/aws-whoami/   CLI entry point (Cobra)
├── tools/            Separate Go module pinning build tools (cyclonedx-gomod)
├── principal.go      ARN parsing and principal types
├── policy.go         Account alias lookup policy
├── whoami.go         STS/IAM calls and the Client
└── format.go         Text and JSON output
```

| Task            | Description |
|-----------------|-------------|
| `task` / `task test` | Run unit tests with the race detector; coverage written to `.test/coverage.out` |
| `task coverage` | HTML coverage report at `.test/coverage.html` |
| `task vet`      | Run `go vet` |
| `task fmt`      | Format the source with `gofmt` |
| `task check`    | Formatting check, vet and tests (what CI should run) |
| `task build`    | Build `.build/aws-whoami`, stamping the version from `git describe` |
| `task release VERSION=vX.Y.Z` | Cross-compile every OS/arch into `.build/dist/` with checksums and an SBOM |
| `task sbom`     | Generate a CycloneDX SBOM at `.build/dist/bom.json` |
| `task actions:lock` | Regenerate `.github/workflows/actions.lock` |
| `task actions:verify` | Check `actions.lock` without modifying anything |
| `task clean`    | Remove `.build/` and `.test/` |

Tests use hand-written fakes for STS and IAM and never call AWS. Coverage must
stay at or above 70%.

Build tools live in their own module under `tools/`, declared with Go's `tool`
directive and run with `go -C tools tool <name>`. They're pinned and
checksum-verified in `tools/go.sum`, and Dependabot updates them. Their
dependencies stay out of the main `go.mod`, so projects importing `awswhoami`
don't inherit them.

Both modules declare `go 1.27.0` and `toolchain go1.27.4`. If your installed Go
is older, Go downloads 1.27.4 automatically (checksum-verified). CI and
releases build with the toolchain version, which `actions/setup-go` reads from
`go.mod`. When a new Go patch release comes out, bump it in both modules:

```bash
go mod edit -toolchain=go1.27.N && go -C tools mod edit -toolchain=go1.27.N
```

### Continuous integration

| Workflow | Trigger | What it does |
|----------|---------|--------------|
| `ci.yaml` | Every push and pull request | `task check` (gofmt, vet, tests), `task build` and `task sbom` |
| `sast.yaml` | Pushes/PRs to `main`, and Tuesdays at 18:18 UTC | CodeQL static analysis for Go |
| `automerge-dependabot.yaml` | Dependabot pull requests | Enables squash auto-merge for **Go module** updates (including major versions), so they merge once the required checks pass. GitHub Actions updates change `actions.lock` and wait for a human review |
| `verify-lockfile.yaml` | Every pull request, and pushes to `main` | `gh actions-lock --verify`: fails if `actions.lock` is stale or a pin no longer verifies upstream |
| `release.yaml` | `v*` tags | See [Releasing](#releasing) |

Dependabot checks Go modules weekly and GitHub Actions daily.

#### Workflow lockfile

Every action the workflows use, including transitive ones, is pinned to a
specific commit in
[`.github/workflows/actions.lock`](.github/workflows/actions.lock), using
GitHub's [workflow dependency lockfiles](https://docs.github.com/en/actions/concepts/security/workflow-dependency-lockfiles).
At job start, the runner executes the locked commit even if the tag in a
`uses:` line has since been moved upstream.

The repository's **Require lockfile** Actions policy is active for all
workflows, so GitHub refuses to run any workflow that isn't in `actions.lock`.
A new workflow file must be locked in the same commit that adds it.

- Install the tool once: `gh extension install github/gh-actions-lock --pin v0.4.1`.
- After adding a workflow or changing any `uses:` line, run `task actions:lock`
  and commit the workflow change and the lockfile together.
- Never edit `actions.lock` by hand.
- Dependabot updates the lockfile in the same PR as the version bump. Review
  the `commit` changes in those diffs: that's the code your workflows will run.

The **Protect main** ruleset on the default branch:

- requires the `test` (CI) and `CodeQL Scan (go)` checks to pass before merging;
- blocks force-pushes and branch deletion;
- lets repository admins bypass these rules.

Auto-merge relies on this ruleset: without required checks, a Dependabot PR
would merge as soon as auto-merge is enabled, before CI has run. It also needs
**Settings → General → Allow auto-merge** turned on.

### Releasing

Push a semantic-version tag:

```bash
git tag v1.0.0
git push origin v1.0.0
```

The [release workflow](.github/workflows/release.yaml) then:

1. rejects the tag unless it matches `vX.Y.Z` or `vX.Y.Z-prerelease`;
2. runs `task check`;
3. runs `task release`, which builds static (`CGO_ENABLED=0`) binaries for:

   | OS      | Architectures      |
   |---------|--------------------|
   | Linux   | `amd64`, `arm64`   |
   | macOS   | `amd64`, `arm64`   |
   | Windows | `amd64`, `arm64`   |

4. generates a CycloneDX SBOM (`bom.json`) with `task sbom`, as part of `task release`;
5. publishes a GitHub release with generated notes. The action creates the
   release as a draft, uploads the assets and only then publishes it, which is
   required because the repository has **immutable releases** enabled. Once
   published, a release's assets and tag can't be changed, so a bad release
   is fixed by tagging a new version. The assets are:
   - `aws-whoami_<version>_<os>_<arch>.tar.gz` (binary, `LICENSE`, `README.md`)
   - `checksums.txt` (SHA-256 of every archive)
   - `bom.json`

To add a platform, append `os/arch` to `PLATFORMS` in `Taskfile.yml`.

## Licence

Apache 2.0. See [LICENSE](LICENSE).

Based on [benkehoe/aws-whoami](https://github.com/benkehoe/aws-whoami),
copyright 2020 Ben Kehoe, licensed under Apache 2.0.
