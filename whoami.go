// Package awswhoami determines which AWS account and identity are in use.
package awswhoami

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"
)

// fallbackRegion mirrors botocore, which uses the global STS and IAM
// endpoints when no region is configured.
const fallbackRegion = "us-east-1"

const accessDeniedCode = "AccessDenied"

type STSAPI interface {
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

type IAMAPI interface {
	iam.ListAccountAliasesAPIClient
}

// Info describes the current IAM principal, account, and region.
type Info struct {
	Account          AccountID
	AccountAliases   []string
	Arn              ARN
	Type             PrincipalType
	Name             string
	RoleSessionName  string
	UserID           UserID
	Region           Region
	SSOPermissionSet string
}

type Client struct {
	STS    STSAPI
	IAM    IAMAPI
	Region Region
}

// NewClient builds a Client from an AWS config, falling back to us-east-1
// for API calls when no region is configured.
func NewClient(cfg aws.Config) *Client {
	region := Region(cfg.Region)
	apiCfg := cfg.Copy()
	if apiCfg.Region == "" {
		apiCfg.Region = fallbackRegion
	}
	return &Client{
		STS:    sts.NewFromConfig(apiCfg),
		IAM:    iam.NewFromConfig(apiCfg),
		Region: region,
	}
}

func Whoami(ctx context.Context, cfg aws.Config, policy AliasPolicy) (Info, error) {
	return NewClient(cfg).Whoami(ctx, policy)
}

func (c *Client) Whoami(ctx context.Context, policy AliasPolicy) (Info, error) {
	identity, err := c.STS.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return Info{}, err
	}

	info := Info{
		Account:        AccountID(aws.ToString(identity.Account)),
		AccountAliases: []string{},
		Arn:            ARN(aws.ToString(identity.Arn)),
		UserID:         UserID(aws.ToString(identity.UserId)),
		Region:         c.Region,
	}

	p, err := parsePrincipal(info.Arn)
	if err != nil {
		return Info{}, err
	}
	info.Type = p.Type
	info.Name = p.Name
	info.RoleSessionName = p.RoleSessionName
	info.SSOPermissionSet = p.SSOPermissionSet

	if policy.shouldLookup(info) {
		aliases, err := c.accountAliases(ctx)
		if err != nil {
			return Info{}, err
		}
		info.AccountAliases = aliases
	}

	return info, nil
}

func (c *Client) accountAliases(ctx context.Context) ([]string, error) {
	aliases := []string{}
	paginator := iam.NewListAccountAliasesPaginator(c.IAM, &iam.ListAccountAliasesInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			if isAccessDenied(err) {
				return []string{}, nil
			}
			return nil, fmt.Errorf("listing account aliases: %w", err)
		}
		aliases = append(aliases, page.AccountAliases...)
	}
	return aliases, nil
}

func isAccessDenied(err error) bool {
	var apiErr smithy.APIError
	return errors.As(err, &apiErr) && apiErr.ErrorCode() == accessDeniedCode
}
