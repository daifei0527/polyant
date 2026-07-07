package handler

import (
	"net/http"

	"github.com/daifei0527/polyant/internal/core/backup"
	awerrors "github.com/daifei0527/polyant/pkg/errors"
)

// BackupHandler exposes the KV backup admin endpoints.
type BackupHandler struct {
	svc *backup.Service
}

// NewBackupHandler creates a BackupHandler backed by the given backup service.
func NewBackupHandler(svc *backup.Service) *BackupHandler {
	return &BackupHandler{svc: svc}
}

// CreateBackupHandler handles POST /api/v1/admin/backup
func (h *BackupHandler) CreateBackupHandler(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.CreateBackup(r.Context())
	if err != nil {
		writeError(w, awerrors.Wrap(320, awerrors.CategoryStorage, "backup failed", http.StatusInternalServerError, err))
		return
	}
	writeJSON(w, http.StatusOK, &APIResponse{Code: 0, Message: "success", Data: map[string]interface{}{
		"dir": res.Dir, "size_bytes": res.SizeBytes, "key_count": res.KeyCount, "created_at": res.CreatedAt,
	}})
}

// ListBackupsHandler handles GET /api/v1/admin/backups
func (h *BackupHandler) ListBackupsHandler(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.ListBackups()
	if err != nil {
		writeError(w, awerrors.Wrap(321, awerrors.CategoryStorage, "list backups failed", http.StatusInternalServerError, err))
		return
	}
	writeJSON(w, http.StatusOK, &APIResponse{Code: 0, Message: "success", Data: map[string]interface{}{
		"backups": list,
	}})
}
