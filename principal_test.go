package awswhoami

import "testing"

func TestParsePrincipal(t *testing.T) {
	tests := []struct {
		name    string
		arn     ARN
		want    principal
		wantErr bool
	}{
		{
			name: "iam user",
			arn:  "arn:aws:iam::123456789012:user/bob",
			want: principal{Type: PrincipalTypeUser, Name: "bob"},
		},
		{
			name: "iam user with path",
			arn:  "arn:aws:iam::123456789012:user/division/team/bob",
			want: principal{Type: PrincipalTypeUser, Name: "division/team/bob"},
		},
		{
			name: "assumed role",
			arn:  "arn:aws:sts::123456789012:assumed-role/MY-ROLE/ben",
			want: principal{Type: PrincipalTypeAssumedRole, Name: "MY-ROLE", RoleSessionName: "ben"},
		},
		{
			name: "sso assumed role",
			arn:  "arn:aws:sts::123456789012:assumed-role/AWSReservedSSO_AdministratorAccess_0123abcd/ben@example.com",
			want: principal{
				Type:             PrincipalTypeAssumedRole,
				Name:             "AWSReservedSSO_AdministratorAccess_0123abcd",
				RoleSessionName:  "ben@example.com",
				SSOPermissionSet: "AdministratorAccess",
			},
		},
		{
			name: "federated user",
			arn:  "arn:aws:sts::123456789012:federated-user/alice",
			want: principal{Type: PrincipalTypeFederatedUser, Name: "alice"},
		},
		{
			name: "root",
			arn:  "arn:aws:iam::123456789012:root",
			want: principal{Type: PrincipalTypeRoot, Name: "root"},
		},
		{name: "no colon", arn: "garbage", wantErr: true},
		{name: "no name", arn: "arn:aws:iam::123456789012:user", wantErr: true},
		{name: "assumed role without session", arn: "arn:aws:sts::123456789012:assumed-role/MY-ROLE", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePrincipal(tt.arn)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSSOPermissionSet(t *testing.T) {
	tests := map[string]string{
		"AWSReservedSSO_AdministratorAccess_0123abcd": "AdministratorAccess",
		"AWSReservedSSO_My_Permission_Set_0123abcd":   "My_Permission_Set",
		"AWSReservedSSO_NoTag":                        "NoTag",
		"AWSReservedSSO":                              "",
		"MY-ROLE":                                     "",
	}
	for role, want := range tests {
		if got := ssoPermissionSet(role); got != want {
			t.Errorf("ssoPermissionSet(%q) = %q, want %q", role, got, want)
		}
	}
}

func TestPrincipalTypeLabel(t *testing.T) {
	tests := map[PrincipalType]string{
		PrincipalTypeUser:          "User",
		PrincipalTypeAssumedRole:   "AssumedRole",
		PrincipalTypeFederatedUser: "FederatedUser",
		PrincipalTypeRoot:          "Root",
		"a--b":                     "AB",
		"":                         "",
	}
	for typ, want := range tests {
		if got := typ.Label(); got != want {
			t.Errorf("%q.Label() = %q, want %q", typ, got, want)
		}
	}
}
