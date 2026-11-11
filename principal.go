package awswhoami

import (
	"fmt"
	"strings"
)

type AccountID string

type ARN string

type UserID string

type Region string

type PrincipalType string

const (
	PrincipalTypeUser          PrincipalType = "user"
	PrincipalTypeAssumedRole   PrincipalType = "assumed-role"
	PrincipalTypeFederatedUser PrincipalType = "federated-user"
	PrincipalTypeRoot          PrincipalType = "root"
)

// Label converts a hyphenated principal type into its display form,
// e.g. "assumed-role" becomes "AssumedRole".
func (t PrincipalType) Label() string {
	var b strings.Builder
	for _, part := range strings.Split(string(t), "-") {
		if part == "" {
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]))
		b.WriteString(part[1:])
	}
	return b.String()
}

const ssoRolePrefix = "AWSReservedSSO"

type principal struct {
	Type             PrincipalType
	Name             string
	RoleSessionName  string
	SSOPermissionSet string
}

func parsePrincipal(arn ARN) (principal, error) {
	s := string(arn)
	idx := strings.LastIndex(s, ":")
	if idx < 0 {
		return principal{}, fmt.Errorf("malformed ARN %q", s)
	}
	resource := s[idx+1:]

	typ, name, hasName := strings.Cut(resource, "/")
	if !hasName {
		if PrincipalType(typ) == PrincipalTypeRoot {
			return principal{Type: PrincipalTypeRoot, Name: typ}, nil
		}
		return principal{}, fmt.Errorf("malformed ARN %q: no principal name", s)
	}

	p := principal{Type: PrincipalType(typ), Name: name}
	if p.Type != PrincipalTypeAssumedRole {
		return p, nil
	}

	slash := strings.LastIndex(name, "/")
	if slash < 0 {
		return principal{}, fmt.Errorf("malformed assumed-role ARN %q: no role session name", s)
	}
	p.Name, p.RoleSessionName = name[:slash], name[slash+1:]
	p.SSOPermissionSet = ssoPermissionSet(p.Name)
	return p, nil
}

// ssoPermissionSet extracts the permission set from a role name of the form
// AWSReservedSSO_{permission-set}_{random-tag}.
func ssoPermissionSet(roleName string) string {
	if !strings.HasPrefix(roleName, ssoRolePrefix) {
		return ""
	}
	_, rest, ok := strings.Cut(roleName, "_")
	if !ok {
		return ""
	}
	if idx := strings.LastIndex(rest, "_"); idx >= 0 {
		rest = rest[:idx]
	}
	return rest
}
