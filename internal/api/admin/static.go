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
	distFS     fs.FS
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
		distFS:     distFS,
	}
}

// ServeHTTP 处理静态文件请求
// 对于 SPA 应用，所有非文件请求返回 index.html
func (h *StaticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// 移除 /admin 前缀
	path = strings.TrimPrefix(path, "/admin")
	if path == "" || path == "/" {
		h.serveIndexHTML(w, r)
		return
	}

	// 检查文件是否存在
	if _, err := fs.Stat(h.distFS, strings.TrimPrefix(path, "/")); err != nil {
		// 文件不存在，返回 index.html (SPA 路由)
		h.serveIndexHTML(w, r)
		return
	}

	// 更新请求路径并交给文件服务器处理
	r.URL.Path = path
	h.fileServer.ServeHTTP(w, r)
}

// serveIndexHTML 直接返回 index.html 内容
// 避免 http.FileServer 对 /index.html 路径的 301 重定向行为
func (h *StaticHandler) serveIndexHTML(w http.ResponseWriter, r *http.Request) {
	data, err := fs.ReadFile(h.distFS, "index.html")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}
