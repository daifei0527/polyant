package export

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/daifei0527/agentwiki/internal/storage"
	"github.com/daifei0527/agentwiki/internal/storage/model"
)

// ConflictStrategy 冲突处理策略
type ConflictStrategy string

const (
	ConflictSkip      ConflictStrategy = "skip"      // 跳过冲突
	ConflictOverwrite ConflictStrategy = "overwrite" // 覆盖现有
	ConflictMerge     ConflictStrategy = "merge"     // 合并
)

// ImportOptions 导入选项
type ImportOptions struct {
	ConflictStrategy ConflictStrategy
	OperatorLevel    int32 // 操作者等级，用于权限检查
}

// ImportSummary 导入结果汇总
type ImportSummary struct {
	EntriesImported    int `json:"entries_imported"`
	EntriesSkipped     int `json:"entries_skipped"`
	EntriesUpdated     int `json:"entries_updated"`
	CategoriesImported int `json:"categories_imported"`
	UsersImported      int `json:"users_imported"`
	RatingsImported    int `json:"ratings_imported"`
}

// ImportError 导入错误
type ImportError struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Message string `json:"message"`
}

// ImportResult 导入结果
type ImportResult struct {
	Success bool           `json:"success"`
	Summary ImportSummary  `json:"summary"`
	Errors  []ImportError  `json:"errors,omitempty"`
}

// Importer 导入服务
type Importer struct {
	store *storage.Store
}

// NewImporter 创建导入服务
func NewImporter(store *storage.Store) *Importer {
	return &Importer{store: store}
}

// Import 从 ZIP 文件导入数据
func (i *Importer) Import(zipData []byte, opts ImportOptions) *ImportResult {
	result := &ImportResult{
		Success: true,
		Errors:  []ImportError{},
	}

	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		result.Success = false
		result.Errors = append(result.Errors, ImportError{
			Type:    "zip",
			Message: fmt.Sprintf("failed to read zip: %v", err),
		})
		return result
	}

	// 解析文件到内存
	files := make(map[string][]byte)
	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			continue
		}
		data, _ := io.ReadAll(rc)
		rc.Close()
		files[file.Name] = data
	}

	// 导入分类（先导入，因为条目依赖分类）
	if data, ok := files["categories.json"]; ok {
		i.importCategories(data, opts, result)
	}

	// 导入用户
	if data, ok := files["users.json"]; ok {
		i.importUsers(data, opts, result)
	}

	// 导入条目
	if data, ok := files["entries.json"]; ok {
		i.importEntries(data, opts, result)
	}

	// 导入评分
	if data, ok := files["ratings.json"]; ok {
		i.importRatings(data, opts, result)
	}

	return result
}

// importCategories 导入分类
func (i *Importer) importCategories(data []byte, opts ImportOptions, result *ImportResult) {
	var categories []*model.Category
	if err := json.Unmarshal(data, &categories); err != nil {
		result.Errors = append(result.Errors, ImportError{
			Type:    "category",
			Message: fmt.Sprintf("failed to parse categories: %v", err),
		})
		return
	}

	for _, cat := range categories {
		_, err := i.store.Category.Get(nil, cat.Path)
		if err == nil {
			// 分类已存在
			switch opts.ConflictStrategy {
			case ConflictSkip:
				continue
			case ConflictOverwrite:
				// 更新分类
				i.store.Category.Create(nil, cat) // 覆盖
			case ConflictMerge:
				// 保留现有分类
				continue
			}
		} else {
			// 分类不存在，创建
			i.store.Category.Create(nil, cat)
		}
		result.Summary.CategoriesImported++
	}
}

// importUsers 导入用户
func (i *Importer) importUsers(data []byte, opts ImportOptions, result *ImportResult) {
	var exportUsers []ExportUser
	if err := json.Unmarshal(data, &exportUsers); err != nil {
		result.Errors = append(result.Errors, ImportError{
			Type:    "user",
			Message: fmt.Sprintf("failed to parse users: %v", err),
		})
		return
	}

	for _, eu := range exportUsers {
		// 安全检查：不能导入高于操作者等级的用户
		if eu.UserLevel > opts.OperatorLevel {
			result.Errors = append(result.Errors, ImportError{
				Type:    "user",
				ID:      eu.PublicKey,
				Message: "cannot import user with higher level",
			})
			continue
		}

		existing, err := i.store.User.Get(nil, eu.PublicKey)
		if err == nil {
			// 用户已存在
			switch opts.ConflictStrategy {
			case ConflictSkip:
				continue
			case ConflictOverwrite, ConflictMerge:
				// 只更新公开字段，不修改等级
				existing.AgentName = eu.AgentName
				existing.Status = eu.Status
				i.store.User.Update(nil, existing)
			}
		} else {
			// 用户不存在，创建
			user := &model.User{
				PublicKey:    eu.PublicKey,
				AgentName:    eu.AgentName,
				UserLevel:    eu.UserLevel,
				RegisteredAt: eu.RegisteredAt,
				Status:       eu.Status,
			}
			i.store.User.Create(nil, user)
		}
		result.Summary.UsersImported++
	}
}

// importEntries 导入条目
func (i *Importer) importEntries(data []byte, opts ImportOptions, result *ImportResult) {
	var entries []*model.KnowledgeEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		result.Errors = append(result.Errors, ImportError{
			Type:    "entry",
			Message: fmt.Sprintf("failed to parse entries: %v", err),
		})
		return
	}

	for _, entry := range entries {
		existing, err := i.store.Entry.Get(nil, entry.ID)
		if err == nil {
			// 条目已存在
			switch opts.ConflictStrategy {
			case ConflictSkip:
				result.Summary.EntriesSkipped++
				continue
			case ConflictOverwrite:
				i.store.Entry.Update(nil, entry)
			case ConflictMerge:
				// 比较 version，保留更高版本
				if entry.Version > existing.Version {
					i.store.Entry.Update(nil, entry)
				} else {
					result.Summary.EntriesSkipped++
					continue
				}
			}
			result.Summary.EntriesUpdated++
		} else {
			// 条目不存在，创建
			i.store.Entry.Create(nil, entry)
			result.Summary.EntriesImported++
		}
	}
}

// importRatings 导入评分
func (i *Importer) importRatings(data []byte, opts ImportOptions, result *ImportResult) {
	var ratings []*model.Rating
	if err := json.Unmarshal(data, &ratings); err != nil {
		result.Errors = append(result.Errors, ImportError{
			Type:    "rating",
			Message: fmt.Sprintf("failed to parse ratings: %v", err),
		})
		return
	}

	for _, rating := range ratings {
		// 检查是否已存在评分
		existing, _ := i.store.Rating.GetByRater(nil, rating.EntryId, rating.RaterPubkey)
		if existing != nil {
			switch opts.ConflictStrategy {
			case ConflictSkip:
				continue
			case ConflictOverwrite:
				i.store.Rating.Create(nil, rating)
			case ConflictMerge:
				// 保留现有评分
				continue
			}
		} else {
			i.store.Rating.Create(nil, rating)
		}
		result.Summary.RatingsImported++
	}
}
