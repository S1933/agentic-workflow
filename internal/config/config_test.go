package config

import (
	"strings"
	"testing"
)

func TestRepositoryConfiguration(t *testing.T) {
	c, err := Load("../..")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Workflow.Workflows["feature"].Ordered()) != 13 {
		t.Fatal("missing feature steps")
	}
	candidates, err := c.Candidates("expert")
	if err != nil || len(candidates) != 2 {
		t.Fatalf("%v %v", candidates, err)
	}
	for _, tc := range []struct{ from, to string }{
		{"version: 1", "version: 999"},
		{"role: expert", "role: missing"},
		{"- functional_request", "- undefined_condition"},
		{"transitions: manual", "transitions: automatic"},
		{"optional: true", "optional_typo: true"},
	} {
		t.Run(tc.to, func(t *testing.T) {
			_, err := Parse([]byte(strings.Replace(string(c.WorkflowBytes), tc.from, tc.to, 1)), c.RuntimeBytes)
			if err == nil {
				t.Fatal("accepted invalid configuration")
			}
		})
	}
}
