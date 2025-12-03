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

// Package auth provides authentication and authorization for the solar-index API server.
package auth

import (
	"context"
	"net/http"

	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/request/headerrequest"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/klog/v2"
)

// HeaderAuthenticator authenticates requests using headers (for proxy auth).
type HeaderAuthenticator struct {
	requestHeader authenticator.Request
}

// NewHeaderAuthenticator creates a new header-based authenticator.
func NewHeaderAuthenticator(userHeader string, groupHeaders []string) (*HeaderAuthenticator, error) {
	rh, err := headerrequest.New([]string{userHeader}, nil, groupHeaders, nil)
	if err != nil {
		return nil, err
	}
	return &HeaderAuthenticator{
		requestHeader: rh,
	}, nil
}

// AuthenticateRequest authenticates the request using headers.
func (a *HeaderAuthenticator) AuthenticateRequest(req *http.Request) (*authenticator.Response, bool, error) {
	return a.requestHeader.AuthenticateRequest(req)
}

// AlwaysAllowAuthorizer always allows all requests.
// This is useful for development and testing.
type AlwaysAllowAuthorizer struct{}

// Authorize always returns allowed.
func (a AlwaysAllowAuthorizer) Authorize(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
	klog.V(4).InfoS("AlwaysAllowAuthorizer allowing request",
		"user", attrs.GetUser().GetName(),
		"verb", attrs.GetVerb(),
		"resource", attrs.GetResource(),
	)
	return authorizer.DecisionAllow, "", nil
}

// RBACAuthorizer uses Kubernetes RBAC for authorization decisions.
// This delegates to the kube-apiserver for RBAC checks.
type RBACAuthorizer struct {
	delegate authorizer.Authorizer
}

// NewRBACAuthorizer creates a new RBAC authorizer.
func NewRBACAuthorizer(delegate authorizer.Authorizer) *RBACAuthorizer {
	return &RBACAuthorizer{delegate: delegate}
}

// Authorize checks if the request is authorized using RBAC.
func (a *RBACAuthorizer) Authorize(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
	if a.delegate == nil {
		// Fall back to allow if no delegate configured
		klog.V(2).InfoS("No RBAC delegate configured, allowing request")
		return authorizer.DecisionAllow, "", nil
	}
	return a.delegate.Authorize(ctx, attrs)
}

// UserInfo extracts user info from context.
func UserInfo(ctx context.Context) (user.Info, bool) {
	info, ok := ctx.Value(userInfoKey).(user.Info)
	return info, ok
}

// WithUserInfo adds user info to context.
func WithUserInfo(ctx context.Context, info user.Info) context.Context {
	return context.WithValue(ctx, userInfoKey, info)
}

type contextKey string

const userInfoKey contextKey = "userInfo"

// AnonymousUser returns anonymous user info.
func AnonymousUser() user.Info {
	return &user.DefaultInfo{
		Name:   "system:anonymous",
		Groups: []string{"system:unauthenticated"},
	}
}

// SystemAdmin returns system admin user info (for internal operations).
func SystemAdmin() user.Info {
	return &user.DefaultInfo{
		Name:   "system:admin",
		Groups: []string{"system:masters"},
	}
}

// ServiceAccountUser returns a service account user info.
func ServiceAccountUser(namespace, name string) user.Info {
	return &user.DefaultInfo{
		Name:   "system:serviceaccount:" + namespace + ":" + name,
		Groups: []string{"system:serviceaccounts", "system:serviceaccounts:" + namespace},
	}
}
