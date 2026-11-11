package awswhoami

import "strings"

// DisableAccountAliasEnvVar names the environment variable controlling
// whether the account alias lookup is performed.
const DisableAccountAliasEnvVar = "AWS_WHOAMI_DISABLE_ACCOUNT_ALIAS"

// AliasPolicy decides whether iam:ListAccountAliases should be called.
// The zero value always looks up aliases.
type AliasPolicy struct {
	disableAll bool
	matchers   []string
}

func EnableAccountAlias() AliasPolicy {
	return AliasPolicy{}
}

func DisableAccountAlias() AliasPolicy {
	return AliasPolicy{disableAll: true}
}

// DisableAccountAliasFor disables the lookup when any value matches the
// beginning or end of the account ID, or exactly matches the principal name,
// ARN, or role session name.
func DisableAccountAliasFor(values ...string) AliasPolicy {
	var matchers []string
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			matchers = append(matchers, v)
		}
	}
	return AliasPolicy{matchers: matchers}
}

// ParseAliasPolicy interprets the value of AWS_WHOAMI_DISABLE_ACCOUNT_ALIAS:
// "", "0" or "false" enable the lookup; "1" or "true" disable it; anything
// else is treated as a comma-separated list of matchers.
func ParseAliasPolicy(value string) AliasPolicy {
	switch strings.ToLower(value) {
	case "", "0", "false":
		return EnableAccountAlias()
	case "1", "true":
		return DisableAccountAlias()
	default:
		return DisableAccountAliasFor(strings.Split(value, ",")...)
	}
}

func (p AliasPolicy) shouldLookup(info Info) bool {
	if p.disableAll {
		return false
	}
	account := string(info.Account)
	for _, v := range p.matchers {
		if strings.HasPrefix(account, v) || strings.HasSuffix(account, v) {
			return false
		}
		if v == info.Name || v == string(info.Arn) || v == info.RoleSessionName {
			return false
		}
	}
	return true
}
