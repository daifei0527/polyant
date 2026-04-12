package seed

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/daifei0527/agentwiki/internal/storage"
	"github.com/daifei0527/agentwiki/internal/storage/model"
)

// SeedDataInitializer 种子数据初始化器
type SeedDataInitializer struct {
	store       *storage.Store
	seedDataDir string
	mu          sync.Mutex
	initDone    bool
}

// NewSeedDataInitializer 创建种子数据初始化器
func NewSeedDataInitializer(store *storage.Store, seedDataDir string) *SeedDataInitializer {
	if seedDataDir == "" {
		seedDataDir = "./configs/seed-data"
	}
	return &SeedDataInitializer{
		store:       store,
		seedDataDir: seedDataDir,
	}
}

// Initialize 初始化种子数据
func (si *SeedDataInitializer) Initialize(ctx context.Context) error {
	si.mu.Lock()
	defer si.mu.Unlock()

	if si.initDone {
		return nil
	}

	// 检查是否已有数据
	count, err := si.store.Entry.Count(ctx)
	if err == nil && count > 0 {
		si.initDone = true
		log.Printf("[SeedData] Entries already exist: %d", count)
		return nil
	}

	// 加载并导入条目
	entriesFile := si.seedDataDir + "/default_entries.jsonl"
	imported, err := si.importEntries(ctx, entriesFile)
	if err != nil {
		log.Printf("[SeedData] Failed to import entries: %v", err)
	} else {
		log.Printf("[SeedData] Imported %d entries", imported)
	}

	si.initDone = true
	return nil
}

// importEntries 从 JSONL 文件导入条目
func (si *SeedDataInitializer) importEntries(ctx context.Context, filePath string) (int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	imported := 0

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry model.KnowledgeEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			log.Printf("[SeedData] Failed to parse entry: %v", err)
			continue
		}

		// 检查是否已存在
		existing, _ := si.store.Entry.Get(ctx, entry.ID)
		if existing != nil {
			continue
		}

		// 创建条目
		_, err := si.store.Entry.Create(ctx, &entry)
		if err != nil {
			log.Printf("[SeedData] Failed to create entry %s: %v", entry.ID, err)
			continue
		}

		// 建立索引
		if si.store.Search != nil {
			si.store.Search.IndexEntry(&entry)
		}

		imported++
	}

	if err := scanner.Err(); err != nil {
		return imported, fmt.Errorf("scan file: %w", err)
	}

	return imported, nil
}

// ImportFromFile 从指定文件导入数据
func (si *SeedDataInitializer) ImportFromFile(ctx context.Context, filePath string) (int, error) {
	return si.importEntries(ctx, filePath)
}

// GetSeedEntriesCount 获取种子条目数量
func (si *SeedDataInitializer) GetSeedEntriesCount() int {
	file, err := os.Open(si.seedDataDir + "/default_entries.jsonl")
	if err != nil {
		return 0
	}
	defer file.Close()

	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if len(scanner.Bytes()) > 0 {
			count++
		}
	}
	return count
}

// IsInitialized 检查是否已初始化
func (si *SeedDataInitializer) IsInitialized() bool {
	si.mu.Lock()
	defer si.mu.Unlock()
	return si.initDone
}
