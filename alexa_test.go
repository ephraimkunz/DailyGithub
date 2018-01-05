package main

import "testing"

func Test_requiresAccessToken(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"Require", args{"summary_intent"}, true},
		{"Require2", args{"notifications_intent"}, true},
		{"Require3", args{"assigned_issues_intent"}, true},
		{"NotRequired", args{"trending_repos_intent"}, false},
		{"NotRequired2", args{"fake_intent"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := requiresAccessToken(tt.args.name); got != tt.want {
				t.Errorf("requiresAccessToken() = %v, want %v", got, tt.want)
			}
		})
	}
}
