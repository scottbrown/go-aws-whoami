package awswhoami

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestFormatAssumedRole(t *testing.T) {
	info := Info{
		Account:         "123456789012",
		AccountAliases:  []string{"my-account-alias"},
		Arn:             "arn:aws:sts::123456789012:assumed-role/MY-ROLE/ben",
		Type:            PrincipalTypeAssumedRole,
		Name:            "MY-ROLE",
		RoleSessionName: "ben",
		UserID:          "SOMEOPAQUEID:ben",
		Region:          "us-east-2",
	}
	want := `Account:         123456789012
                 my-account-alias
Region:          us-east-2
AssumedRole:     MY-ROLE
RoleSessionName: ben
UserId:          SOMEOPAQUEID:ben
Arn:             arn:aws:sts::123456789012:assumed-role/MY-ROLE/ben`
	if got := Format(info); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatSSO(t *testing.T) {
	info := Info{
		Account:          "123456789012",
		Arn:              "arn:aws:sts::123456789012:assumed-role/AWSReservedSSO_Admin_abc/ben",
		Type:             PrincipalTypeAssumedRole,
		Name:             "AWSReservedSSO_Admin_abc",
		RoleSessionName:  "ben",
		UserID:           "ID:ben",
		Region:           "ca-central-1",
		SSOPermissionSet: "Admin",
	}
	want := `Account:         123456789012
Region:          ca-central-1
AWS SSO:         Admin
RoleSessionName: ben
UserId:          ID:ben
Arn:             arn:aws:sts::123456789012:assumed-role/AWSReservedSSO_Admin_abc/ben`
	if got := Format(info); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatUser(t *testing.T) {
	info := Info{
		Account: "123456789012",
		Arn:     "arn:aws:iam::123456789012:user/bob",
		Type:    PrincipalTypeUser,
		Name:    "bob",
		UserID:  "AIDAEXAMPLE",
		Region:  "us-west-2",
	}
	want := `Account: 123456789012
Region:  us-west-2
User:    bob
UserId:  AIDAEXAMPLE
Arn:     arn:aws:iam::123456789012:user/bob`
	if got := Format(info); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestMarshalJSONUsesNullForAbsentValues(t *testing.T) {
	info := Info{
		Account: "123456789012",
		Arn:     "arn:aws:iam::123456789012:user/bob",
		Type:    PrincipalTypeUser,
		Name:    "bob",
		UserID:  "AIDAEXAMPLE",
	}
	got, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"Account":"123456789012","AccountAliases":[],"Arn":"arn:aws:iam::123456789012:user/bob","Type":"user","Name":"bob","RoleSessionName":null,"UserId":"AIDAEXAMPLE","Region":null,"SSOPermissionSet":null}`
	if string(got) != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

func TestJSONRoundTrip(t *testing.T) {
	info := Info{
		Account:          "123456789012",
		AccountAliases:   []string{"a", "b"},
		Arn:              "arn:aws:sts::123456789012:assumed-role/AWSReservedSSO_Admin_abc/ben",
		Type:             PrincipalTypeAssumedRole,
		Name:             "AWSReservedSSO_Admin_abc",
		RoleSessionName:  "ben",
		UserID:           "ID:ben",
		Region:           "ca-central-1",
		SSOPermissionSet: "Admin",
	}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	var got Info
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, info) {
		t.Errorf("got %+v\nwant %+v", got, info)
	}
}

func TestUnmarshalJSONNullsAndErrors(t *testing.T) {
	var got Info
	if err := json.Unmarshal([]byte(`{"Account":"1","AccountAliases":null,"RoleSessionName":null}`), &got); err != nil {
		t.Fatal(err)
	}
	if got.AccountAliases == nil || got.RoleSessionName != "" {
		t.Errorf("unexpected result %+v", got)
	}
	if err := json.Unmarshal([]byte(`{"Account":1}`), &got); err == nil {
		t.Error("expected error for wrong type")
	}
}
