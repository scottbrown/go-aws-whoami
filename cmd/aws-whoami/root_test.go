package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	awswhoami "github.com/scottbrown/go-aws-whoami"
)

var sampleInfo = awswhoami.Info{
	Account:        "123456789012",
	AccountAliases: []string{"my-alias"},
	Arn:            "arn:aws:iam::123456789012:user/bob",
	Type:           awswhoami.PrincipalTypeUser,
	Name:           "bob",
	UserID:         "AIDAEXAMPLE",
	Region:         "ca-central-1",
}

type recorder struct {
	profile Profile
	policy  awswhoami.AliasPolicy
	info    awswhoami.Info
	err     error
}

func (r *recorder) whoami(_ context.Context, profile Profile, policy awswhoami.AliasPolicy) (awswhoami.Info, error) {
	r.profile = profile
	r.policy = policy
	return r.info, r.err
}

func env(values map[string]string) func(string) string {
	return func(k string) string { return values[k] }
}

func execute(t *testing.T, rec *recorder, environ map[string]string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), args, &stdout, &stderr, rec.whoami, env(environ))
	return code, stdout.String(), stderr.String()
}

func TestRunText(t *testing.T) {
	rec := &recorder{info: sampleInfo}
	code, stdout, stderr := execute(t, rec, nil, "--profile", "dev")
	if code != 0 {
		t.Fatalf("exit code %d, stderr %q", code, stderr)
	}
	if rec.profile != "dev" {
		t.Errorf("profile = %q, want dev", rec.profile)
	}
	want := awswhoami.Format(sampleInfo) + "\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunJSON(t *testing.T) {
	rec := &recorder{info: sampleInfo}
	code, stdout, _ := execute(t, rec, nil, "--json")
	if code != 0 {
		t.Fatalf("exit code %d", code)
	}
	var got awswhoami.Info
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("invalid JSON %q: %v", stdout, err)
	}
	if got.Account != sampleInfo.Account || got.Name != sampleInfo.Name {
		t.Errorf("got %+v", got)
	}
}

func TestRunPassesAliasPolicyFromEnvironment(t *testing.T) {
	rec := &recorder{info: sampleInfo}
	execute(t, rec, map[string]string{awswhoami.DisableAccountAliasEnvVar: "true"})
	if !reflect.DeepEqual(rec.policy, awswhoami.DisableAccountAlias()) {
		t.Errorf("policy = %+v, want disabled", rec.policy)
	}
}

func TestRunVersion(t *testing.T) {
	code, stdout, _ := execute(t, &recorder{}, nil, "--version")
	if code != 0 || stdout != version+"\n" {
		t.Errorf("code = %d, stdout = %q", code, stdout)
	}
}

func TestRunError(t *testing.T) {
	rec := &recorder{err: errors.New("no credentials")}
	code, _, stderr := execute(t, rec, nil)
	if code != 1 {
		t.Errorf("exit code %d, want 1", code)
	}
	if stderr != "ERROR [*errors.errorString]: no credentials\n" {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestRunDebugPrintsErrorChain(t *testing.T) {
	rec := &recorder{err: fmt.Errorf("outer: %w", errors.New("inner"))}
	code, _, stderr := execute(t, rec, nil, "--debug")
	if code != 1 {
		t.Errorf("exit code %d, want 1", code)
	}
	for _, want := range []string{"*fmt.wrapError: outer: inner", "  *errors.errorString: inner", "ERROR ["} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr %q missing %q", stderr, want)
		}
	}
}

func TestRunRejectsArguments(t *testing.T) {
	code, _, stderr := execute(t, &recorder{}, nil, "extra")
	if code != 1 || !strings.Contains(stderr, "unknown command") {
		t.Errorf("code = %d, stderr = %q", code, stderr)
	}
}

func TestAWSWhoamiInvalidProfile(t *testing.T) {
	t.Setenv("AWS_CONFIG_FILE", t.TempDir()+"/config")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", t.TempDir()+"/credentials")
	_, err := awsWhoami(context.Background(), "does-not-exist", awswhoami.DisableAccountAlias())
	if err == nil {
		t.Error("expected error for missing profile")
	}
}
