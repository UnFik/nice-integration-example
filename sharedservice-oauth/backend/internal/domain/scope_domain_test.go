package domain

import (
	"errors"
	"testing"
)

func TestScopeValidate(t *testing.T) {
	for _, c := range []struct {
		name  string
		scope Scope
		want  error
	}{
		{"complete", Scope{OrgID: "org_a", ProjectID: "prj_p", Subject: "usr_1"}, nil},
		{"without project", Scope{OrgID: "org_a", Subject: "usr_1"}, ErrProjectRequired},
		{"without organization", Scope{ProjectID: "prj_p", Subject: "usr_1"}, ErrOrgRequired},
	} {
		t.Run(c.name, func(t *testing.T) {
			if err := c.scope.Validate(); !errors.Is(err, c.want) {
				t.Fatalf("Validate = %v, want %v", err, c.want)
			}
		})
	}
}
