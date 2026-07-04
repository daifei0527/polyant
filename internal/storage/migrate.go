package storage

import (
	"context"
	"log"
)

// tsMillisThreshold 区分秒 vs 毫秒的阈值。毫秒值 ~1.7e12，秒值 ~1.7e9，阈值居中无歧义。
const tsMillisThreshold int64 = 1_000_000_000_000 // 1e12

// migrateTimestampsToMillis 把历史秒级时间戳（< 1e12）修正为毫秒（×1000）。
// 幂等：已毫秒（>= 1e12）或零值（==0）的不动。
func migrateTimestampsToMillis(entryStore EntryStore) error {
	entries, _, err := entryStore.List(context.Background(), EntryFilter{Limit: 100000})
	if err != nil {
		return err
	}
	migrated := 0
	for _, e := range entries {
		changed := false
		if e.CreatedAt != 0 && e.CreatedAt < tsMillisThreshold {
			e.CreatedAt *= 1000
			changed = true
		}
		if e.UpdatedAt != 0 && e.UpdatedAt < tsMillisThreshold {
			e.UpdatedAt *= 1000
			changed = true
		}
		if changed {
			if _, err := entryStore.Update(context.Background(), e); err != nil {
				return err
			}
			migrated++
		}
	}
	if migrated > 0 {
		log.Printf("[Store] migrated %d entries' timestamps seconds→millis", migrated)
	}
	return nil
}
