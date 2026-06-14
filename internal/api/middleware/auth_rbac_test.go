package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daifei0527/polyant/internal/auth/rbac"
	"github.com/daifei0527/polyant/internal/storage/model"
)

// TestRBACMatrix_DocumentCumulativePermissions pins the rbac permission matrix
// so the access-control source of truth is explicit and tested. This documents
// the current matrix (PermWrite at Lv2) — the Lv1-write discrepancy with the
// handlers is tracked as a deferred product decision (see Plan 1B).
func TestRBACMatrix_DocumentCumulativePermissions(t *testing.T) {
	cases := []struct {
		level int32
		perms []int
	}{
		{model.UserLevelLv0, []int{rbac.PermRead, rbac.PermQuery}},
		{model.UserLevelLv1, []int{rbac.PermRead, rbac.PermQuery, rbac.PermRate}},
		{model.UserLevelLv3, []int{rbac.PermRead, rbac.PermQuery, rbac.PermRate, rbac.PermWrite, rbac.PermMirror, rbac.PermManageCategory}},
		{model.UserLevelLv4, []int{rbac.PermRead, rbac.PermQuery, rbac.PermRate, rbac.PermWrite, rbac.PermMirror, rbac.PermManageCategory, rbac.PermManageUser, rbac.PermAdmin}},
		{model.UserLevelLv5, []int{rbac.PermRead, rbac.PermQuery, rbac.PermRate, rbac.PermWrite, rbac.PermMirror, rbac.PermManageCategory, rbac.PermManageUser, rbac.PermAdmin}},
	}
	for _, c := range cases {
		for _, p := range c.perms {
			if !rbac.HasPermission(c.level, p) {
				t.Errorf("Lv%d should hold permission %d", c.level, p)
			}
		}
	}
	if rbac.HasPermission(model.UserLevelLv1, rbac.PermAdmin) {
		t.Error("Lv1 must not hold PermAdmin")
	}
	if rbac.HasPermission(model.UserLevelLv0, rbac.PermWrite) {
		t.Error("Lv0 must not hold PermWrite")
	}
}

// TestRequirePermission_AllowsAndDenies: RequirePermission admits a level that
// holds the permission and rejects one that does not.
func TestRequirePermission_AllowsAndDenies(t *testing.T) {
	mw := &AuthMiddleware{}

	// Lv5 holds PermAdmin -> allowed
	allowed := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserLevelKey, int32(model.UserLevelLv5)))
	mw.RequirePermission(rbac.PermAdmin, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(allowed, req)
	if allowed.Code != http.StatusOK {
		t.Errorf("Lv5 with PermAdmin should be allowed, got %d", allowed.Code)
	}

	// Lv1 lacks PermAdmin -> denied (403)
	denied := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2 = req2.WithContext(context.WithValue(req2.Context(), UserLevelKey, int32(model.UserLevelLv1)))
	mw.RequirePermission(rbac.PermAdmin, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(denied, req2)
	if denied.Code != http.StatusForbidden {
		t.Errorf("Lv1 without PermAdmin should be denied (403), got %d", denied.Code)
	}
}
