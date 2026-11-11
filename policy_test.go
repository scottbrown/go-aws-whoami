package awswhoami

import "testing"

func TestParseAliasPolicy(t *testing.T) {
	info := Info{
		Account:         "123456789012",
		Arn:             "arn:aws:sts::123456789012:assumed-role/MY-ROLE/ben",
		Name:            "MY-ROLE",
		RoleSessionName: "ben",
	}
	tests := []struct {
		value      string
		wantLookup bool
	}{
		{"", true},
		{"0", true},
		{"false", true},
		{"FALSE", true},
		{"1", false},
		{"true", false},
		{"True", false},
		{"1234", false},
		{"9012", false},
		{"5678", true},
		{"MY-ROLE", false},
		{"my-role", true},
		{"ben", false},
		{"arn:aws:sts::123456789012:assumed-role/MY-ROLE/ben", false},
		{"other, MY-ROLE", false},
		{"other,another", true},
		{",", true},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			if got := ParseAliasPolicy(tt.value).shouldLookup(info); got != tt.wantLookup {
				t.Errorf("shouldLookup = %v, want %v", got, tt.wantLookup)
			}
		})
	}
}

func TestAliasPolicyEmptyRoleSessionNameDoesNotMatch(t *testing.T) {
	info := Info{Account: "123456789012", Name: "bob"}
	if !DisableAccountAliasFor("").shouldLookup(info) {
		t.Error("empty matcher should not disable lookup")
	}
}

func TestAliasPolicyConstructors(t *testing.T) {
	info := Info{Account: "123456789012"}
	if !EnableAccountAlias().shouldLookup(info) {
		t.Error("EnableAccountAlias should look up aliases")
	}
	if DisableAccountAlias().shouldLookup(info) {
		t.Error("DisableAccountAlias should not look up aliases")
	}
	if !(AliasPolicy{}).shouldLookup(info) {
		t.Error("zero value should look up aliases")
	}
}
