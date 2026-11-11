package awswhoami

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"
)

type fakeSTS struct {
	out *sts.GetCallerIdentityOutput
	err error
}

func (f fakeSTS) GetCallerIdentity(context.Context, *sts.GetCallerIdentityInput, ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	return f.out, f.err
}

type fakeIAM struct {
	pages [][]string
	err   error
	calls int
}

func (f *fakeIAM) ListAccountAliases(_ context.Context, in *iam.ListAccountAliasesInput, _ ...func(*iam.Options)) (*iam.ListAccountAliasesOutput, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	idx := 0
	if in.Marker != nil {
		for i := range f.pages {
			if *in.Marker == string(rune('a'+i)) {
				idx = i
			}
		}
	}
	out := &iam.ListAccountAliasesOutput{AccountAliases: f.pages[idx]}
	if idx+1 < len(f.pages) {
		out.IsTruncated = true
		out.Marker = aws.String(string(rune('a' + idx + 1)))
	}
	return out, nil
}

func identity(arn string) fakeSTS {
	return fakeSTS{out: &sts.GetCallerIdentityOutput{
		Account: aws.String("123456789012"),
		Arn:     aws.String(arn),
		UserId:  aws.String("AROAEXAMPLE:ben"),
	}}
}

func TestWhoamiAssumedRoleWithPaginatedAliases(t *testing.T) {
	fiam := &fakeIAM{pages: [][]string{{"alias-one"}, {"alias-two"}}}
	c := &Client{
		STS:    identity("arn:aws:sts::123456789012:assumed-role/MY-ROLE/ben"),
		IAM:    fiam,
		Region: "us-east-2",
	}

	got, err := c.Whoami(context.Background(), EnableAccountAlias())
	if err != nil {
		t.Fatal(err)
	}
	want := Info{
		Account:         "123456789012",
		AccountAliases:  []string{"alias-one", "alias-two"},
		Arn:             "arn:aws:sts::123456789012:assumed-role/MY-ROLE/ben",
		Type:            PrincipalTypeAssumedRole,
		Name:            "MY-ROLE",
		RoleSessionName: "ben",
		UserID:          "AROAEXAMPLE:ben",
		Region:          "us-east-2",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
	if fiam.calls != 2 {
		t.Errorf("ListAccountAliases called %d times, want 2", fiam.calls)
	}
}

func TestWhoamiAliasDisabled(t *testing.T) {
	fiam := &fakeIAM{pages: [][]string{{"alias"}}}
	c := &Client{STS: identity("arn:aws:iam::123456789012:user/bob"), IAM: fiam}

	got, err := c.Whoami(context.Background(), DisableAccountAlias())
	if err != nil {
		t.Fatal(err)
	}
	if fiam.calls != 0 {
		t.Errorf("ListAccountAliases called %d times, want 0", fiam.calls)
	}
	if got.AccountAliases == nil || len(got.AccountAliases) != 0 {
		t.Errorf("AccountAliases = %#v, want empty non-nil slice", got.AccountAliases)
	}
}

func TestWhoamiAccessDeniedIsIgnored(t *testing.T) {
	fiam := &fakeIAM{err: &smithy.GenericAPIError{Code: "AccessDenied", Message: "nope"}}
	c := &Client{STS: identity("arn:aws:iam::123456789012:user/bob"), IAM: fiam}

	got, err := c.Whoami(context.Background(), EnableAccountAlias())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.AccountAliases) != 0 || got.AccountAliases == nil {
		t.Errorf("AccountAliases = %#v, want empty non-nil slice", got.AccountAliases)
	}
}

func TestWhoamiOtherIAMErrorIsReturned(t *testing.T) {
	apiErr := &smithy.GenericAPIError{Code: "Throttling", Message: "slow down"}
	c := &Client{STS: identity("arn:aws:iam::123456789012:user/bob"), IAM: &fakeIAM{err: apiErr}}

	_, err := c.Whoami(context.Background(), EnableAccountAlias())
	if !errors.Is(err, apiErr) {
		t.Errorf("err = %v, want wrapping %v", err, apiErr)
	}
}

func TestWhoamiSTSErrorIsReturned(t *testing.T) {
	stsErr := errors.New("no credentials")
	c := &Client{STS: fakeSTS{err: stsErr}, IAM: &fakeIAM{}}

	_, err := c.Whoami(context.Background(), EnableAccountAlias())
	if !errors.Is(err, stsErr) {
		t.Errorf("err = %v, want %v", err, stsErr)
	}
}

func TestWhoamiMalformedARN(t *testing.T) {
	c := &Client{STS: identity("garbage"), IAM: &fakeIAM{}}

	if _, err := c.Whoami(context.Background(), EnableAccountAlias()); err == nil {
		t.Error("expected error for malformed ARN")
	}
}

func TestWhoamiSSO(t *testing.T) {
	c := &Client{
		STS: identity("arn:aws:sts::123456789012:assumed-role/AWSReservedSSO_ReadOnly_abc123/ben"),
		IAM: &fakeIAM{pages: [][]string{{}}},
	}
	got, err := c.Whoami(context.Background(), EnableAccountAlias())
	if err != nil {
		t.Fatal(err)
	}
	if got.SSOPermissionSet != "ReadOnly" {
		t.Errorf("SSOPermissionSet = %q, want ReadOnly", got.SSOPermissionSet)
	}
}

func TestNewClient(t *testing.T) {
	c := NewClient(aws.Config{})
	if c.Region != "" {
		t.Errorf("Region = %q, want empty", c.Region)
	}
	if c.STS.(*sts.Client).Options().Region != fallbackRegion {
		t.Errorf("STS region = %q, want %q", c.STS.(*sts.Client).Options().Region, fallbackRegion)
	}

	c = NewClient(aws.Config{Region: "ca-central-1"})
	if c.Region != "ca-central-1" {
		t.Errorf("Region = %q, want ca-central-1", c.Region)
	}
	if got := c.IAM.(*iam.Client).Options().Region; got != "ca-central-1" {
		t.Errorf("IAM region = %q, want ca-central-1", got)
	}
}

func TestWhoamiPackageFunctionPropagatesErrors(t *testing.T) {
	cfg := aws.Config{
		Region: "ca-central-1",
		Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			return aws.Credentials{}, errors.New("no credentials")
		}),
	}
	if _, err := Whoami(context.Background(), cfg, EnableAccountAlias()); err == nil {
		t.Error("expected error when credentials are unavailable")
	}
}
