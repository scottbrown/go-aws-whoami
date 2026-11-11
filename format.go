package awswhoami

import (
	"encoding/json"
	"fmt"
	"strings"
)

type line struct {
	label string
	value string
}

// Format renders Info as aligned, human-readable lines.
func Format(info Info) string {
	lines := []line{{"Account: ", string(info.Account)}}
	for _, alias := range info.AccountAliases {
		lines = append(lines, line{"", alias})
	}
	lines = append(lines, line{"Region: ", string(info.Region)})
	if info.SSOPermissionSet != "" {
		lines = append(lines, line{"AWS SSO: ", info.SSOPermissionSet})
	} else {
		lines = append(lines, line{info.Type.Label() + ": ", info.Name})
	}
	if info.RoleSessionName != "" {
		lines = append(lines, line{"RoleSessionName: ", info.RoleSessionName})
	}
	lines = append(lines,
		line{"UserId: ", string(info.UserID)},
		line{"Arn: ", string(info.Arn)},
	)

	width := 0
	for _, l := range lines {
		width = max(width, len(l.label))
	}

	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = fmt.Sprintf("%-*s%s", width, l.label, l.value)
	}
	return strings.Join(out, "\n")
}

type jsonInfo struct {
	Account          string   `json:"Account"`
	AccountAliases   []string `json:"AccountAliases"`
	Arn              string   `json:"Arn"`
	Type             string   `json:"Type"`
	Name             string   `json:"Name"`
	RoleSessionName  *string  `json:"RoleSessionName"`
	UserID           string   `json:"UserId"`
	Region           *string  `json:"Region"`
	SSOPermissionSet *string  `json:"SSOPermissionSet"`
}

// MarshalJSON emits the same field names as the Python aws-whoami, using
// null for absent optional values.
func (i Info) MarshalJSON() ([]byte, error) {
	aliases := i.AccountAliases
	if aliases == nil {
		aliases = []string{}
	}
	return json.Marshal(jsonInfo{
		Account:          string(i.Account),
		AccountAliases:   aliases,
		Arn:              string(i.Arn),
		Type:             string(i.Type),
		Name:             i.Name,
		RoleSessionName:  nullable(i.RoleSessionName),
		UserID:           string(i.UserID),
		Region:           nullable(string(i.Region)),
		SSOPermissionSet: nullable(i.SSOPermissionSet),
	})
}

func (i *Info) UnmarshalJSON(data []byte) error {
	var j jsonInfo
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}
	*i = Info{
		Account:          AccountID(j.Account),
		AccountAliases:   j.AccountAliases,
		Arn:              ARN(j.Arn),
		Type:             PrincipalType(j.Type),
		Name:             j.Name,
		RoleSessionName:  deref(j.RoleSessionName),
		UserID:           UserID(j.UserID),
		Region:           Region(deref(j.Region)),
		SSOPermissionSet: deref(j.SSOPermissionSet),
	}
	if i.AccountAliases == nil {
		i.AccountAliases = []string{}
	}
	return nil
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
