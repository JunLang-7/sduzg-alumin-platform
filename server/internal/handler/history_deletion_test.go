package handler

import (
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/middleware"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/service"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeleteHistoryEntryAccessBoundary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name   string
		access *common.AccessContext
		id     string
		want   int
	}{
		{"unauthenticated", nil, "1", http.StatusUnauthorized},
		{"alumni", &common.AccessContext{UserID: 1, Role: common.RoleAlumni}, "1", http.StatusForbidden},
		{"ordinary administrator", &common.AccessContext{UserID: 2, Role: common.RoleAdmin, DomainIDs: []uint64{1}}, "1", http.StatusForbidden},
		{"invalid id", &common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin}, "0", http.StatusBadRequest},
		{"missing database", &common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin}, "1", http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			if tc.access != nil {
				router.Use(func(c *gin.Context) { c.Set(middleware.AccessContextKey, tc.access); c.Next() })
			}
			h := NewHistoryHandler(service.NewHistoryService(nil, nil))
			router.DELETE("/entries/:id", h.DeleteEntry)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/entries/"+tc.id, nil))
			if rec.Code != tc.want {
				t.Fatalf("status %d, want %d; %s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}
