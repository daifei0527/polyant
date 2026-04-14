package handler

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/daifei0527/agentwiki/internal/core/export"
	"github.com/daifei0527/agentwiki/internal/storage"
	awerrors "github.com/daifei0527/agentwiki/pkg/errors"
)

// ExportHandler 导出/导入处理器
type ExportHandler struct {
	exporter *export.Exporter
	importer *export.Importer
	store    *storage.Store
}

// NewExportHandler 创建导出处理器
func NewExportHandler(store *storage.Store, nodeID string) *ExportHandler {
	return &ExportHandler{
		exporter: export.NewExporter(store, nodeID),
		importer: export.NewImporter(store),
		store:    store,
	}
}

// ExportHandler 导出数据
// GET /api/v1/admin/export?include=entries,categories,users,ratings
func (h *ExportHandler) ExportHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, awerrors.New(100, awerrors.CategoryAPI, "method not allowed", http.StatusMethodNotAllowed))
		return
	}

	// 解析 include 参数
	includeParam := r.URL.Query().Get("include")
	if includeParam == "" {
		includeParam = "entries,categories"
	}

	opts := export.ExportOptions{
		IncludeEntries:    strings.Contains(includeParam, "entries"),
		IncludeCategories: strings.Contains(includeParam, "categories"),
		IncludeUsers:      strings.Contains(includeParam, "users"),
		IncludeRatings:    strings.Contains(includeParam, "ratings"),
	}

	// 执行导出
	zipData, err := h.exporter.Export(opts)
	if err != nil {
		writeError(w, awerrors.Wrap(900, awerrors.CategoryAPI, err.Error(), http.StatusInternalServerError, err))
		return
	}

	// 设置响应头
	filename := fmt.Sprintf("agentwiki-export-%s.zip", time.Now().Format("20060102"))
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(zipData)))

	w.Write(zipData)
}

// ImportHandler 导入数据
// POST /api/v1/admin/import
// Content-Type: multipart/form-data
// Fields: file (ZIP), conflict (skip|overwrite|merge)
func (h *ExportHandler) ImportHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, awerrors.New(100, awerrors.CategoryAPI, "method not allowed", http.StatusMethodNotAllowed))
		return
	}

	// 解析 multipart 表单
	maxSize := int64(100 << 20) // 100MB
	r.Body = http.MaxBytesReader(w, r.Body, maxSize)

	if err := r.ParseMultipartForm(maxSize); err != nil {
		writeError(w, awerrors.New(101, awerrors.CategoryAPI, "file too large or invalid form", http.StatusBadRequest))
		return
	}

	// 获取上传的文件
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, awerrors.New(102, awerrors.CategoryAPI, "missing file field", http.StatusBadRequest))
		return
	}
	defer file.Close()

	// 读取文件内容
	zipData, err := io.ReadAll(file)
	if err != nil {
		writeError(w, awerrors.New(103, awerrors.CategoryAPI, "failed to read file", http.StatusBadRequest))
		return
	}

	// 获取冲突策略
	conflictStr := r.FormValue("conflict")
	if conflictStr == "" {
		conflictStr = "skip"
	}

	strategy := export.ConflictStrategy(conflictStr)
	if strategy != export.ConflictSkip && strategy != export.ConflictOverwrite && strategy != export.ConflictMerge {
		writeError(w, awerrors.New(104, awerrors.CategoryAPI, "invalid conflict strategy", http.StatusBadRequest))
		return
	}

	// 获取操作者等级（用于权限检查）
	user := getUserFromContext(r.Context())
	var operatorLevel int32
	if user != nil {
		operatorLevel = user.UserLevel
	}

	// 执行导入
	opts := export.ImportOptions{
		ConflictStrategy: strategy,
		OperatorLevel:    operatorLevel,
	}
	result := h.importer.Import(zipData, opts)

	// 返回结果
	status := http.StatusOK
	if !result.Success {
		status = http.StatusBadRequest
	}

	writeJSON(w, status, &APIResponse{
		Code:    0,
		Message: "import completed",
		Data:    result,
	})
}
