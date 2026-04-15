// internal/api/admin/static.go
package admin

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed dist
var adminFS embed.FS

// StaticHandler 静态文件处理器
type StaticHandler struct {
	fileServer http.Handler
}

// NewStaticHandler 创建静态文件处理器
func NewStaticHandler() *StaticHandler {
	// 获取 dist 子目录
	distFS, err := fs.Sub(adminFS, "dist")
	if err != nil {
		panic(err)
	}

	return &StaticHandler{
		fileServer: http.FileServer(http.FS(distFS)),
	}
}

// ServeHTTP 处理静态文件请求
// 对于 SPA 应用，所有非文件请求返回 index.html
func (h *StaticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// 移除 /admin 前缀
	path = strings.TrimPrefix(path, "/admin")
	if path == "" || path == "/" {
		path = "/index.html"
	}

	// 更新请求路径
	r.URL.Path = path

	// 检查文件是否存在
	if _, err := fs.Stat(adminFS, "dist"+path); err != nil {
		// 文件不存在，返回 index.html (SPA 路由)
		r.URL.Path = "/index.html"
	}

	h.fileServer.ServeHTTP(w, r)
}
