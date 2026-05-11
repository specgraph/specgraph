// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import "testing"

func TestProjectParamsValidate(t *testing.T) {
	cases := []struct {
		name    string
		params  ProjectParams
		wantErr bool
	}{
		{"valid", ProjectParams{Slug: "myproj", ServerURL: "http://localhost:9090"}, false},
		{"valid https", ProjectParams{Slug: "p", ServerURL: "https://specgraph.example.com"}, false},
		{"valid slug with dots", ProjectParams{Slug: "p.q.r_1-2", ServerURL: "http://h"}, false},
		{"empty slug", ProjectParams{Slug: "", ServerURL: "http://h"}, true},
		{"slug with space", ProjectParams{Slug: "my proj", ServerURL: "http://h"}, true},
		{"slug starts with dash", ProjectParams{Slug: "-x", ServerURL: "http://h"}, true},
		{"missing scheme", ProjectParams{Slug: "p", ServerURL: "localhost:9090"}, true},
		{"empty URL", ProjectParams{Slug: "p", ServerURL: ""}, true},
		{"non-http scheme", ProjectParams{Slug: "p", ServerURL: "ftp://h"}, true},
		{"empty host", ProjectParams{Slug: "p", ServerURL: "http://"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.params.Validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate() err = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}
