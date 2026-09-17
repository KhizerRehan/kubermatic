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

package vsphere

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
	_ "github.com/vmware/govmomi/vapi/simulator"
	vapitags "github.com/vmware/govmomi/vapi/tags"

	kubermaticv1 "k8c.io/kubermatic/sdk/v2/apis/kubermatic/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func TestValidateTagGroups(t *testing.T) {
	tests := []struct {
		name              string
		spec              *kubermaticv1.VSphereCloudSpec
		defaultCategoryID string
		wantErr           bool
	}{
		{
			name: "no tags",
			spec: &kubermaticv1.VSphereCloudSpec{},
		},
		{
			name: "primary group with category",
			spec: &kubermaticv1.VSphereCloudSpec{
				Tags: &kubermaticv1.VSphereTag{CategoryID: "cat-a", Tags: []string{"a"}},
			},
		},
		{
			name: "primary group without category and no default",
			spec: &kubermaticv1.VSphereCloudSpec{
				Tags: &kubermaticv1.VSphereTag{Tags: []string{"a"}},
			},
			wantErr: true,
		},
		{
			name: "primary group without category falls back to default",
			spec: &kubermaticv1.VSphereCloudSpec{
				Tags: &kubermaticv1.VSphereTag{Tags: []string{"a"}},
			},
			defaultCategoryID: "cat-default",
		},
		{
			name: "primary and additional groups with distinct categories",
			spec: &kubermaticv1.VSphereCloudSpec{
				Tags: &kubermaticv1.VSphereTag{CategoryID: "cat-a", Tags: []string{"a"}},
				AdditionalTags: []kubermaticv1.VSphereTag{
					{CategoryID: "cat-b", Tags: []string{"b"}},
					{CategoryID: "cat-c", Tags: []string{"c"}},
				},
			},
		},
		{
			name: "additional groups only",
			spec: &kubermaticv1.VSphereCloudSpec{
				AdditionalTags: []kubermaticv1.VSphereTag{
					{CategoryID: "cat-b", Tags: []string{"b"}},
				},
			},
		},
		{
			name: "additional group without category",
			spec: &kubermaticv1.VSphereCloudSpec{
				Tags: &kubermaticv1.VSphereTag{CategoryID: "cat-a", Tags: []string{"a"}},
				AdditionalTags: []kubermaticv1.VSphereTag{
					{Tags: []string{"b"}},
				},
			},
			wantErr: true,
		},
		{
			name: "additional group repeats primary category",
			spec: &kubermaticv1.VSphereCloudSpec{
				Tags: &kubermaticv1.VSphereTag{CategoryID: "cat-a", Tags: []string{"a"}},
				AdditionalTags: []kubermaticv1.VSphereTag{
					{CategoryID: "cat-a", Tags: []string{"b"}},
				},
			},
			wantErr: true,
		},
		{
			name: "additional group repeats defaulted primary category",
			spec: &kubermaticv1.VSphereCloudSpec{
				Tags: &kubermaticv1.VSphereTag{Tags: []string{"a"}},
				AdditionalTags: []kubermaticv1.VSphereTag{
					{CategoryID: "cat-default", Tags: []string{"b"}},
				},
			},
			defaultCategoryID: "cat-default",
			wantErr:           true,
		},
		{
			name: "two additional groups share a category",
			spec: &kubermaticv1.VSphereCloudSpec{
				AdditionalTags: []kubermaticv1.VSphereTag{
					{CategoryID: "cat-b", Tags: []string{"b"}},
					{CategoryID: "cat-b", Tags: []string{"c"}},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTagGroups(tt.spec, tt.defaultCategoryID)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTagGroups() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTagCategoryIDs(t *testing.T) {
	spec := &kubermaticv1.VSphereCloudSpec{
		Tags: &kubermaticv1.VSphereTag{Tags: []string{"a"}},
		AdditionalTags: []kubermaticv1.VSphereTag{
			{CategoryID: "cat-b", Tags: []string{"b"}},
			{CategoryID: "cat-c", Tags: []string{"c"}},
		},
	}

	if diff := cmp.Diff([]string{"cat-b", "cat-c"}, tagCategoryIDs(spec)); diff != "" {
		t.Errorf("unexpected category IDs (-want +got):\n%s", diff)
	}
}

func TestValidateCloudSpecUpdate(t *testing.T) {
	tests := []struct {
		name    string
		oldSpec *kubermaticv1.VSphereCloudSpec
		newSpec *kubermaticv1.VSphereCloudSpec
		wantErr bool
	}{
		{
			name:    "unchanged folder",
			oldSpec: &kubermaticv1.VSphereCloudSpec{Folder: "/dc/vm/cluster"},
			newSpec: &kubermaticv1.VSphereCloudSpec{Folder: "/dc/vm/cluster"},
		},
		{
			name:    "changed folder",
			oldSpec: &kubermaticv1.VSphereCloudSpec{Folder: "/dc/vm/cluster"},
			newSpec: &kubermaticv1.VSphereCloudSpec{Folder: "/dc/vm/other"},
			wantErr: true,
		},
		{
			name:    "valid multi-group update",
			oldSpec: &kubermaticv1.VSphereCloudSpec{Folder: "/dc/vm/cluster"},
			newSpec: &kubermaticv1.VSphereCloudSpec{
				Folder: "/dc/vm/cluster",
				Tags:   &kubermaticv1.VSphereTag{CategoryID: "cat-a", Tags: []string{"a"}},
				AdditionalTags: []kubermaticv1.VSphereTag{
					{CategoryID: "cat-b", Tags: []string{"b"}},
				},
			},
		},
		{
			name:    "additional group without category",
			oldSpec: &kubermaticv1.VSphereCloudSpec{},
			newSpec: &kubermaticv1.VSphereCloudSpec{
				AdditionalTags: []kubermaticv1.VSphereTag{{Tags: []string{"b"}}},
			},
			wantErr: true,
		},
		{
			name:    "repeated category",
			oldSpec: &kubermaticv1.VSphereCloudSpec{},
			newSpec: &kubermaticv1.VSphereCloudSpec{
				Tags: &kubermaticv1.VSphereTag{CategoryID: "cat-a", Tags: []string{"a"}},
				AdditionalTags: []kubermaticv1.VSphereTag{
					{CategoryID: "cat-a", Tags: []string{"b"}},
				},
			},
			wantErr: true,
		},
	}

	v := &VSphere{dc: &kubermaticv1.DatacenterSpecVSphere{}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateCloudSpecUpdate(context.Background(),
				kubermaticv1.CloudSpec{VSphere: tt.oldSpec}, kubermaticv1.CloudSpec{VSphere: tt.newSpec})
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCloudSpecUpdate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

type tagTestEnv struct {
	ctx         context.Context
	session     *Session
	restSession *RESTSession
	tagManager  *vapitags.Manager
}

func newTagTestEnv(t *testing.T) *tagTestEnv {
	t.Helper()

	sim := vSphereSimulator{t: t}
	sim.model = simulator.VPX()
	if err := sim.model.Create(); err != nil {
		t.Fatalf("failed to create simulator model: %v", err)
	}
	// The vapi (tagging) endpoints are only mounted when enabled before the server starts.
	sim.model.Service.RegisterEndpoints = true
	sim.server = sim.model.Service.NewServer()
	t.Cleanup(sim.tearDown)

	dc := &kubermaticv1.DatacenterSpecVSphere{}
	sim.fillClientInfo(dc)

	ctx := context.Background()
	session, err := newSession(ctx, dc, sim.username(), sim.password(), nil)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	t.Cleanup(func() { session.Logout(ctx) })

	restSession, err := newRESTSession(ctx, dc, sim.username(), sim.password(), nil)
	if err != nil {
		t.Fatalf("failed to create REST session: %v", err)
	}
	t.Cleanup(func() { restSession.Logout(ctx) })

	return &tagTestEnv{
		ctx:         ctx,
		session:     session,
		restSession: restSession,
		tagManager:  vapitags.NewManager(restSession.Client),
	}
}

func (e *tagTestEnv) createCategory(t *testing.T, name string) string {
	t.Helper()

	id, err := e.tagManager.CreateCategory(e.ctx, &vapitags.Category{
		Name:        name,
		Cardinality: "MULTIPLE",
	})
	if err != nil {
		t.Fatalf("failed to create category %q: %v", name, err)
	}
	return id
}

func (e *tagTestEnv) createFolder(t *testing.T, name string) *object.Folder {
	t.Helper()

	root, err := e.session.Finder.Folder(e.ctx, "/DC0/vm")
	if err != nil {
		t.Fatalf("failed to find root folder: %v", err)
	}
	folder, err := root.CreateFolder(e.ctx, name)
	if err != nil {
		t.Fatalf("failed to create folder: %v", err)
	}
	return folder
}

func (e *tagTestEnv) categoryTagNames(t *testing.T, categoryID string) sets.Set[string] {
	t.Helper()

	categoryTags, err := e.tagManager.GetTagsForCategory(e.ctx, categoryID)
	if err != nil {
		t.Fatalf("failed to list tags for category %q: %v", categoryID, err)
	}
	names := sets.New[string]()
	for _, tag := range categoryTags {
		names.Insert(tag.Name)
	}
	return names
}

func (e *tagTestEnv) attachedTagNames(t *testing.T, folder *object.Folder) sets.Set[string] {
	t.Helper()

	attached, err := e.tagManager.GetAttachedTags(e.ctx, folder.Reference())
	if err != nil {
		t.Fatalf("failed to list attached tags: %v", err)
	}
	names := sets.New[string]()
	for _, tag := range attached {
		names.Insert(tag.Name)
	}
	return names
}

func vsphereCluster(tags *kubermaticv1.VSphereTag, additional ...kubermaticv1.VSphereTag) *kubermaticv1.Cluster {
	return &kubermaticv1.Cluster{
		Spec: kubermaticv1.ClusterSpec{
			Cloud: kubermaticv1.CloudSpec{
				VSphere: &kubermaticv1.VSphereCloudSpec{
					Tags:           tags,
					AdditionalTags: additional,
				},
			},
		},
	}
}

func assertNames(t *testing.T, what string, want []string, got sets.Set[string]) {
	t.Helper()

	if diff := cmp.Diff(sets.List(sets.New(want...)), sets.List(got)); diff != "" {
		t.Errorf("unexpected %s (-want +got):\n%s", what, diff)
	}
}

func TestSyncCreatedClusterTagsMultipleGroups(t *testing.T) {
	env := newTagTestEnv(t)
	catA := env.createCategory(t, "cat-a")
	catB := env.createCategory(t, "cat-b")

	cluster := vsphereCluster(
		&kubermaticv1.VSphereTag{CategoryID: catA, Tags: []string{"a1", "a2"}},
		kubermaticv1.VSphereTag{CategoryID: catB, Tags: []string{"b1"}},
	)

	// Run twice to make sure existing tags are not created again.
	for range 2 {
		if err := syncCreatedClusterTags(env.ctx, env.restSession, cluster); err != nil {
			t.Fatalf("syncCreatedClusterTags failed: %v", err)
		}
	}

	assertNames(t, "tags in category A", []string{"a1", "a2"}, env.categoryTagNames(t, catA))
	assertNames(t, "tags in category B", []string{"b1"}, env.categoryTagNames(t, catB))
}

func TestEnsureFolderTagsMultipleGroups(t *testing.T) {
	env := newTagTestEnv(t)
	catA := env.createCategory(t, "cat-a")
	catB := env.createCategory(t, "cat-b")
	catUnmanaged := env.createCategory(t, "cat-unmanaged")

	unmanagedTagID, err := createTag(env.ctx, env.restSession, catUnmanaged, "u1")
	if err != nil {
		t.Fatalf("failed to create unmanaged tag: %v", err)
	}

	folder := env.createFolder(t, "cluster-folder")
	if err := env.tagManager.AttachTag(env.ctx, unmanagedTagID, folder.Reference()); err != nil {
		t.Fatalf("failed to attach unmanaged tag: %v", err)
	}

	cluster := vsphereCluster(
		&kubermaticv1.VSphereTag{CategoryID: catA, Tags: []string{"a1", "a2"}},
		kubermaticv1.VSphereTag{CategoryID: catB, Tags: []string{"b1"}},
	)
	if err := syncCreatedClusterTags(env.ctx, env.restSession, cluster); err != nil {
		t.Fatalf("syncCreatedClusterTags failed: %v", err)
	}

	if err := ensureFolderTags(env.ctx, env.session, env.restSession, "/DC0/vm/cluster-folder",
		cluster.Spec.Cloud.VSphere.AllTags(), folder); err != nil {
		t.Fatalf("ensureFolderTags failed: %v", err)
	}
	assertNames(t, "attached tags", []string{"a1", "a2", "b1", "u1"}, env.attachedTagNames(t, folder))

	cluster.Spec.Cloud.VSphere.Tags.Tags = []string{"a1"}
	if err := ensureFolderTags(env.ctx, env.session, env.restSession, "/DC0/vm/cluster-folder",
		cluster.Spec.Cloud.VSphere.AllTags(), folder); err != nil {
		t.Fatalf("ensureFolderTags failed: %v", err)
	}
	assertNames(t, "attached tags after removal", []string{"a1", "b1", "u1"}, env.attachedTagNames(t, folder))
}

func TestSyncDeletedClusterTagsMultipleGroups(t *testing.T) {
	env := newTagTestEnv(t)
	catA := env.createCategory(t, "cat-a")
	catB := env.createCategory(t, "cat-b")

	cluster := vsphereCluster(
		&kubermaticv1.VSphereTag{CategoryID: catA, Tags: []string{"a1", "a2"}},
		kubermaticv1.VSphereTag{CategoryID: catB, Tags: []string{"b1"}},
	)
	if err := syncCreatedClusterTags(env.ctx, env.restSession, cluster); err != nil {
		t.Fatalf("syncCreatedClusterTags failed: %v", err)
	}

	// a1 stays attached to another object and must survive deletion.
	a1, err := env.tagManager.GetTagForCategory(env.ctx, "a1", catA)
	if err != nil {
		t.Fatalf("failed to get tag a1: %v", err)
	}
	other := env.createFolder(t, "other-folder")
	if err := env.tagManager.AttachTag(env.ctx, a1.ID, other.Reference()); err != nil {
		t.Fatalf("failed to attach tag a1: %v", err)
	}

	now := metav1.Now()
	cluster.DeletionTimestamp = &now
	if err := syncDeletedClusterTags(env.ctx, env.restSession, cluster); err != nil {
		t.Fatalf("syncDeletedClusterTags failed: %v", err)
	}

	assertNames(t, "tags in category A", []string{"a1"}, env.categoryTagNames(t, catA))
	assertNames(t, "tags in category B", nil, env.categoryTagNames(t, catB))
}
