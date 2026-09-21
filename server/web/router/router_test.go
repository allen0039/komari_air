package router

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRemovedFeatureRoutesStayUnregistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	Register(router)

	routes := make(map[string]struct{})
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}

	removed := []string{
		"GET /api/oauth",
		"GET /api/oauth_callback",
		"GET /api/clients/terminal",
		"GET /api/clients/transfer/:id",
		"POST /api/clients/transfer/:id",
		"GET /api/admin/client/:uuid/terminal",
		"POST /api/admin/client/:uuid/file/upload",
		"GET /api/admin/client/:uuid/file/download",
		"GET /api/admin/settings/xtermjs",
		"GET /api/admin/settings/oidc",
		"GET /api/admin/plugin/market/catalog",
		"POST /api/admin/plugin/market/install",
	}
	for _, route := range removed {
		if _, exists := routes[route]; exists {
			t.Errorf("removed route is still registered: %s", route)
		}
	}

	kept := []string{
		"POST /api/login",
		"GET /api/admin/plugin/list",
		"POST /api/admin/plugin/enabled",
		"POST /api/admin/plugin/delete",
		"GET /api/clients/v2/rpc",
	}
	for _, route := range kept {
		if _, exists := routes[route]; !exists {
			t.Errorf("required route is not registered: %s", route)
		}
	}
}
