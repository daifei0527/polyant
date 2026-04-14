// Package export 提供数据导出和导入功能
package export

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/daifei0527/polyant/internal/storage"
	"github.com/daifei0527/polyant/internal/storage/model"
	"github.com/daifei0527/polyant/pkg/logger"
)

// Manifest 导出文件元数据
type Manifest struct {
	Version    string         `json:"version"`
	ExportedAt int64          `json:"exported_at"`
	NodeID     string         `json:"node_id"`
	Counts     map[string]int `json:"counts"`
}

// ExportUser 导出用户格式（隐私保护）
type ExportUser struct {
	PublicKey    string `json:"public_key"`
	AgentName    string `json:"agent_name"`
	UserLevel    int32  `json:"user_level"`
	RegisteredAt int64  `json:"registered_at"`
	Status       string `json:"status"`
}

// Exporter 导出服务
type Exporter struct {
	store  *storage.Store
	nodeID string
}

// NewExporter 创建导出服务
func NewExporter(store *storage.Store, nodeID string) *Exporter {
	return &Exporter{
		store:  store,
		nodeID: nodeID,
	}
}

// ExportOptions 导出选项
type ExportOptions struct {
	IncludeEntries    bool
	IncludeCategories bool
	IncludeUsers      bool
	IncludeRatings    bool
}

// Export 导出数据到 ZIP 文件
func (e *Exporter) Export(opts ExportOptions) ([]byte, error) {
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// 创建 manifest
	manifest := &Manifest{
		Version:    "1.0",
		ExportedAt: time.Now().UnixMilli(),
		NodeID:     e.nodeID,
		Counts:     make(map[string]int),
	}

	// 导出条目
	if opts.IncludeEntries {
		entries, _, err := e.store.Entry.List(nil, storage.EntryFilter{Limit: 100000})
		if err != nil {
			zipWriter.Close()
			return nil, fmt.Errorf("failed to list entries: %w", err)
		}
		if err := e.writeJSONToZip(zipWriter, "entries.json", entries); err != nil {
			zipWriter.Close()
			return nil, err
		}
		manifest.Counts["entries"] = len(entries)
	}

	// 导出分类
	if opts.IncludeCategories {
		categories, err := e.store.Category.ListAll(nil)
		if err != nil {
			zipWriter.Close()
			return nil, fmt.Errorf("failed to list categories: %w", err)
		}
		if err := e.writeJSONToZip(zipWriter, "categories.json", categories); err != nil {
			zipWriter.Close()
			return nil, err
		}
		manifest.Counts["categories"] = len(categories)
	}

	// 导出用户
	if opts.IncludeUsers {
		users, _, err := e.store.User.List(nil, storage.UserFilter{Limit: 100000})
		if err != nil {
			zipWriter.Close()
			return nil, fmt.Errorf("failed to list users: %w", err)
		}
		// 转换为导出格式（去除敏感信息）
		exportUsers := make([]ExportUser, len(users))
		for i, u := range users {
			exportUsers[i] = ExportUser{
				PublicKey:    u.PublicKey,
				AgentName:    u.AgentName,
				UserLevel:    u.UserLevel,
				RegisteredAt: u.RegisteredAt,
				Status:       u.Status,
			}
		}
		if err := e.writeJSONToZip(zipWriter, "users.json", exportUsers); err != nil {
			zipWriter.Close()
			return nil, err
		}
		manifest.Counts["users"] = len(users)
	}

	// 导出评分
	if opts.IncludeRatings {
		entries, _, err := e.store.Entry.List(nil, storage.EntryFilter{Limit: 100000})
		if err != nil {
			zipWriter.Close()
			return nil, fmt.Errorf("failed to list entries for ratings: %w", err)
		}
		var allRatings []*model.Rating
		for _, entry := range entries {
			ratings, err := e.store.Rating.ListByEntry(nil, entry.ID)
			if err != nil {
				logger.Warn("failed to list ratings for entry %s: %v", entry.ID, err)
				continue
			}
			allRatings = append(allRatings, ratings...)
		}
		if err := e.writeJSONToZip(zipWriter, "ratings.json", allRatings); err != nil {
			zipWriter.Close()
			return nil, err
		}
		manifest.Counts["ratings"] = len(allRatings)
	}

	// 写入 manifest
	if err := e.writeJSONToZip(zipWriter, "manifest.json", manifest); err != nil {
		zipWriter.Close()
		return nil, err
	}

	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close zip: %w", err)
	}

	return buf.Bytes(), nil
}

// writeJSONToZip 写入 JSON 文件到 ZIP
func (e *Exporter) writeJSONToZip(zipWriter *zip.Writer, filename string, data interface{}) error {
	writer, err := zipWriter.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create %s in zip: %w", filename, err)
	}

	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode %s: %w", filename, err)
	}

	return nil
}
