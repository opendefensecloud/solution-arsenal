/*
Copyright 2024 Open Defense Cloud Authors.

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

package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

func TestAlwaysAllowAuthorizer(t *testing.T) {
	t.Parallel()

	auth := AlwaysAllowAuthorizer{}
	ctx := context.Background()

	attrs := &testAttributes{
		user:     &user.DefaultInfo{Name: "test-user"},
		verb:     "get",
		resource: "catalogitems",
	}

	decision, reason, err := auth.Authorize(ctx, attrs)
	require.NoError(t, err)
	assert.Equal(t, authorizer.DecisionAllow, decision)
	assert.Empty(t, reason)
}

func TestRBACAuthorizer_NoDelegate(t *testing.T) {
	t.Parallel()

	auth := NewRBACAuthorizer(nil)
	ctx := context.Background()

	attrs := &testAttributes{
		user:     &user.DefaultInfo{Name: "test-user"},
		verb:     "get",
		resource: "catalogitems",
	}

	decision, _, err := auth.Authorize(ctx, attrs)
	require.NoError(t, err)
	assert.Equal(t, authorizer.DecisionAllow, decision)
}

func TestRBACAuthorizer_WithDelegate(t *testing.T) {
	t.Parallel()

	delegate := &mockAuthorizer{decision: authorizer.DecisionDeny, reason: "denied by policy"}
	auth := NewRBACAuthorizer(delegate)
	ctx := context.Background()

	attrs := &testAttributes{
		user:     &user.DefaultInfo{Name: "test-user"},
		verb:     "delete",
		resource: "releases",
	}

	decision, reason, err := auth.Authorize(ctx, attrs)
	require.NoError(t, err)
	assert.Equal(t, authorizer.DecisionDeny, decision)
	assert.Equal(t, "denied by policy", reason)
}

func TestUserInfoContext(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Should return false when no user info
	_, ok := UserInfo(ctx)
	assert.False(t, ok)

	// Add user info
	userInfo := &user.DefaultInfo{Name: "test-user", Groups: []string{"group1"}}
	ctx = WithUserInfo(ctx, userInfo)

	// Should return user info
	got, ok := UserInfo(ctx)
	require.True(t, ok)
	assert.Equal(t, "test-user", got.GetName())
	assert.Equal(t, []string{"group1"}, got.GetGroups())
}

func TestAnonymousUser(t *testing.T) {
	t.Parallel()

	u := AnonymousUser()
	assert.Equal(t, "system:anonymous", u.GetName())
	assert.Contains(t, u.GetGroups(), "system:unauthenticated")
}

func TestSystemAdmin(t *testing.T) {
	t.Parallel()

	u := SystemAdmin()
	assert.Equal(t, "system:admin", u.GetName())
	assert.Contains(t, u.GetGroups(), "system:masters")
}

func TestServiceAccountUser(t *testing.T) {
	t.Parallel()

	u := ServiceAccountUser("default", "my-sa")
	assert.Equal(t, "system:serviceaccount:default:my-sa", u.GetName())
	assert.Contains(t, u.GetGroups(), "system:serviceaccounts")
	assert.Contains(t, u.GetGroups(), "system:serviceaccounts:default")
}

// testAttributes implements authorizer.Attributes for testing.
type testAttributes struct {
	user      user.Info
	verb      string
	resource  string
	namespace string
	name      string
}

func (a *testAttributes) GetUser() user.Info           { return a.user }
func (a *testAttributes) GetVerb() string              { return a.verb }
func (a *testAttributes) GetNamespace() string         { return a.namespace }
func (a *testAttributes) GetResource() string          { return a.resource }
func (a *testAttributes) GetSubresource() string       { return "" }
func (a *testAttributes) GetName() string              { return a.name }
func (a *testAttributes) GetAPIGroup() string          { return "solar.odc.io" }
func (a *testAttributes) GetAPIVersion() string        { return "v1alpha1" }
func (a *testAttributes) IsResourceRequest() bool      { return true }
func (a *testAttributes) GetPath() string              { return "" }
func (a *testAttributes) IsReadOnly() bool             { return a.verb == "get" || a.verb == "list" || a.verb == "watch" }
func (a *testAttributes) GetFieldSelector() (fields.Requirements, error) { return nil, nil }
func (a *testAttributes) GetLabelSelector() (labels.Requirements, error) { return nil, nil }

// mockAuthorizer is a mock authorizer for testing.
type mockAuthorizer struct {
	decision authorizer.Decision
	reason   string
	err      error
}

func (m *mockAuthorizer) Authorize(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
	return m.decision, m.reason, m.err
}
