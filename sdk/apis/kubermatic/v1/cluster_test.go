/*
Copyright 2026 The Kubermatic Kubernetes Platform contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestVSphereCloudSpecAllTags(t *testing.T) {
	primary := VSphereTag{CategoryID: "cat-a", Tags: []string{"a"}}
	additional := []VSphereTag{
		{CategoryID: "cat-b", Tags: []string{"b"}},
		{CategoryID: "cat-c", Tags: []string{"c1", "c2"}},
	}

	tests := []struct {
		name string
		spec *VSphereCloudSpec
		want []VSphereTag
	}{
		{
			name: "nil spec",
			spec: nil,
			want: nil,
		},
		{
			name: "no tags",
			spec: &VSphereCloudSpec{},
			want: []VSphereTag{},
		},
		{
			name: "primary only",
			spec: &VSphereCloudSpec{Tags: &primary},
			want: []VSphereTag{primary},
		},
		{
			name: "additional only",
			spec: &VSphereCloudSpec{AdditionalTags: additional},
			want: additional,
		},
		{
			name: "primary first, then additional in order",
			spec: &VSphereCloudSpec{Tags: &primary, AdditionalTags: additional},
			want: append([]VSphereTag{primary}, additional...),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := cmp.Diff(tt.want, tt.spec.AllTags()); diff != "" {
				t.Errorf("unexpected tag groups (-want +got):\n%s", diff)
			}
		})
	}
}
