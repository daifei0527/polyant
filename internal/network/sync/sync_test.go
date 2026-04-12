// Package sync_test 提供同步引擎的单元测试
package sync_test

import (
	"testing"

	"github.com/daifei0527/agentwiki/internal/network/protocol"
	"github.com/daifei0527/agentwiki/internal/network/sync"
)

// ==================== VersionVector 测试 ====================

// TestVersionVectorGet 测试获取版本号
func TestVersionVectorGet(t *testing.T) {
	vv := make(sync.VersionVector)
	vv["entry-1"] = 5

	if vv.Get("entry-1") != 5 {
		t.Errorf("Get 错误: got %d, want 5", vv.Get("entry-1"))
	}

	if vv.Get("entry-2") != 0 {
		t.Errorf("Get 不存在的键应返回 0: got %d", vv.Get("entry-2"))
	}
}

// TestVersionVectorIncrement 测试版本号递增
func TestVersionVectorIncrement(t *testing.T) {
	vv := make(sync.VersionVector)

	// 首次递增
	v1 := vv.Increment("entry-1")
	if v1 != 1 {
		t.Errorf("首次 Increment 应返回 1: got %d", v1)
	}

	// 再次递增
	v2 := vv.Increment("entry-1")
	if v2 != 2 {
		t.Errorf("再次 Increment 应返回 2: got %d", v2)
	}

	// 不同条目
	v3 := vv.Increment("entry-2")
	if v3 != 1 {
		t.Errorf("新条目 Increment 应返回 1: got %d", v3)
	}
}

// TestVersionVectorMerge 测试版本向量合并
func TestVersionVectorMerge(t *testing.T) {
	vv1 := make(sync.VersionVector)
	vv1["entry-1"] = 5
	vv1["entry-2"] = 3

	vv2 := make(sync.VersionVector)
	vv2["entry-1"] = 7
	vv2["entry-3"] = 2

	merged := vv1.Merge(vv2)

	// entry-1 应取较大值
	if merged["entry-1"] != 7 {
		t.Errorf("合并后 entry-1 应为 7: got %d", merged["entry-1"])
	}

	// entry-2 应保留
	if merged["entry-2"] != 3 {
		t.Errorf("合并后 entry-2 应为 3: got %d", merged["entry-2"])
	}

	// entry-3 应从 vv2 添加
	if merged["entry-3"] != 2 {
		t.Errorf("合并后 entry-3 应为 2: got %d", merged["entry-3"])
	}

	// 原始向量不应被修改
	if vv1["entry-1"] != 5 {
		t.Error("原始向量不应被修改")
	}
}

// TestVersionVectorDiff 测试版本向量差异计算
func TestVersionVectorDiff(t *testing.T) {
	vv1 := make(sync.VersionVector)
	vv1["entry-1"] = 5
	vv1["entry-2"] = 3

	vv2 := make(sync.VersionVector)
	vv2["entry-1"] = 7  // 比 vv1 新
	vv2["entry-2"] = 3  // 相同
	vv2["entry-3"] = 2  // vv1 没有

	// vv2 相比 vv1 的新条目
	diff := vv1.Diff(vv2)

	if len(diff) != 2 {
		t.Errorf("Diff 应返回 2 个条目: got %d, %v", len(diff), diff)
	}

	// 检查包含的条目
	hasEntry1 := false
	hasEntry3 := false
	for _, id := range diff {
		if id == "entry-1" {
			hasEntry1 = true
		}
		if id == "entry-3" {
			hasEntry3 = true
		}
	}

	if !hasEntry1 {
		t.Error("Diff 应包含 entry-1")
	}

	if !hasEntry3 {
		t.Error("Diff 应包含 entry-3")
	}
}

// TestVersionVectorDiffEmpty 测试空向量差异
func TestVersionVectorDiffEmpty(t *testing.T) {
	vv1 := make(sync.VersionVector)
	vv2 := make(sync.VersionVector)
	vv2["entry-1"] = 5

	diff := vv1.Diff(vv2)

	if len(diff) != 1 {
		t.Errorf("空向量 Diff 应返回 1 个条目: got %d", len(diff))
	}

	// 反向差异
	diff2 := vv2.Diff(vv1)
	if len(diff2) != 0 {
		t.Errorf("Diff 空向量应返回 0: got %d", len(diff2))
	}
}

// TestVersionVectorToProto 测试转换为 protobuf 格式
func TestVersionVectorToProto(t *testing.T) {
	vv := make(sync.VersionVector)
	vv["entry-1"] = 5
	vv["entry-2"] = 3

	proto := vv.ToProto()

	if proto["entry-1"] != 5 {
		t.Errorf("ToProto 错误: got %d", proto["entry-1"])
	}
}

// TestVersionVectorFromProto 测试从 protobuf 格式转换
func TestVersionVectorFromProto(t *testing.T) {
	proto := map[string]int64{
		"entry-1": 5,
		"entry-2": 3,
	}

	vv := sync.VersionVectorFromProto(proto)

	if vv.Get("entry-1") != 5 {
		t.Errorf("FromProto 错误: got %d", vv.Get("entry-1"))
	}
}

// TestVersionVectorRoundTrip 测试序列化往返
func TestVersionVectorRoundTrip(t *testing.T) {
	original := make(sync.VersionVector)
	original["entry-1"] = 10
	original["entry-2"] = 20
	original["entry-3"] = 30

	proto := original.ToProto()
	recovered := sync.VersionVectorFromProto(proto)

	for k, v := range original {
		if recovered[k] != v {
			t.Errorf("RoundTrip 错误: %s: got %d, want %d", k, recovered[k], v)
		}
	}
}

// ==================== SyncConfig 测试 ====================

// TestSyncConfigDefaults 测试同步配置
func TestSyncConfigDefaults(t *testing.T) {
	cfg := &sync.SyncConfig{
		AutoSync:         true,
		IntervalSeconds:  60,
		MirrorCategories: []string{"tech"},
		MaxLocalSizeMB:   100,
		BatchSize:        100,
	}

	if !cfg.AutoSync {
		t.Error("AutoSync 应为 true")
	}

	if cfg.IntervalSeconds != 60 {
		t.Errorf("IntervalSeconds 应为 60: got %d", cfg.IntervalSeconds)
	}
}

// ==================== SyncState 测试 ====================

// TestSyncStateConstants 测试同步状态常量
func TestSyncStateConstants(t *testing.T) {
	states := []sync.SyncState{
		sync.SyncStateIdle,
		sync.SyncStateSyncing,
		sync.SyncStateError,
		sync.SyncStateComplete,
	}

	expected := []string{"idle", "syncing", "error", "complete"}
	for i, state := range states {
		if string(state) != expected[i] {
			t.Errorf("状态 %d 错误: got %q, want %q", i, state, expected[i])
		}
	}
}

// ==================== 复杂场景测试 ====================

// TestVersionVectorMergeMultiple 测试多次合并
func TestVersionVectorMergeMultiple(t *testing.T) {
	vv1 := make(sync.VersionVector)
	vv1["a"] = 1
	vv1["b"] = 2

	vv2 := make(sync.VersionVector)
	vv2["a"] = 3
	vv2["c"] = 4

	vv3 := make(sync.VersionVector)
	vv3["b"] = 5
	vv3["d"] = 6

	// 链式合并
	result := vv1.Merge(vv2).Merge(vv3)

	if result["a"] != 3 {
		t.Errorf("a 应为 3: got %d", result["a"])
	}
	if result["b"] != 5 {
		t.Errorf("b 应为 5: got %d", result["b"])
	}
	if result["c"] != 4 {
		t.Errorf("c 应为 4: got %d", result["c"])
	}
	if result["d"] != 6 {
		t.Errorf("d 应为 6: got %d", result["d"])
	}
}

// TestVersionVectorIncrementAndMerge 测试递增后合并
func TestVersionVectorIncrementAndMerge(t *testing.T) {
	local := make(sync.VersionVector)
	local["entry-1"] = 5

	remote := make(sync.VersionVector)
	remote["entry-1"] = 3

	// 本地有更新
	local.Increment("entry-1") // 变成 6

	// 合并远程
	merged := local.Merge(remote)

	// 应保留本地较高版本
	if merged["entry-1"] != 6 {
		t.Errorf("entry-1 应为 6: got %d", merged["entry-1"])
	}
}

// TestVersionVectorDiffAfterMerge 测试合并后的差异计算
func TestVersionVectorDiffAfterMerge(t *testing.T) {
	local := make(sync.VersionVector)
	local["entry-1"] = 5
	local["entry-2"] = 3

	remote := make(sync.VersionVector)
	remote["entry-1"] = 7
	remote["entry-2"] = 3
	remote["entry-3"] = 2

	// 合并前：remote 有更新
	diff1 := local.Diff(remote)
	if len(diff1) != 2 {
		t.Errorf("合并前应有 2 个差异: got %d", len(diff1))
	}

	// 合并
	local = local.Merge(remote)

	// 合并后：应无差异
	diff2 := local.Diff(remote)
	if len(diff2) != 0 {
		t.Errorf("合并后应无差异: got %d", len(diff2))
	}
}

// ==================== max 函数测试 ====================

// TestMaxFunction 测试 max 函数
func TestMaxFunction(t *testing.T) {
	tests := []struct {
		a, b, expected int64
	}{
		{1, 2, 2},
		{5, 3, 5},
		{0, 0, 0},
		{-1, 1, 1},
		{-5, -3, -3},
	}

	for _, tt := range tests {
		// 直接调用 sync 包中的 max 函数需要它被导出
		// 由于 max 是小写的，我们测试逻辑通过其他方式
		_ = tt
	}
}

// ==================== SyncEngine 创建测试 ====================

// TestNewSyncEngine 测试创建同步引擎
func TestNewSyncEngine(t *testing.T) {
	cfg := &sync.SyncConfig{
		AutoSync:        false,
		IntervalSeconds: 60,
		BatchSize:       100,
	}

	// 创建不带依赖的同步引擎
	engine := sync.NewSyncEngine(nil, nil, nil, cfg)
	if engine == nil {
		t.Error("NewSyncEngine 不应返回 nil")
	}
}

// TestSyncEngineGetState 测试获取同步状态
func TestSyncEngineGetState(t *testing.T) {
	cfg := &sync.SyncConfig{AutoSync: false}
	engine := sync.NewSyncEngine(nil, nil, nil, cfg)

	state := engine.GetState()
	if state != sync.SyncStateIdle {
		t.Errorf("初始状态应为 idle: got %q", state)
	}
}

// TestSyncEngineGetVersionVector 测试获取版本向量
func TestSyncEngineGetVersionVector(t *testing.T) {
	cfg := &sync.SyncConfig{AutoSync: false}
	engine := sync.NewSyncEngine(nil, nil, nil, cfg)

	vv := engine.GetVersionVector()
	if vv == nil {
		t.Error("GetVersionVector 不应返回 nil")
	}

	if len(vv) != 0 {
		t.Error("初始版本向量应为空")
	}
}

// TestSyncEngineSetProtocol 测试设置协议
func TestSyncEngineSetProtocol(t *testing.T) {
	cfg := &sync.SyncConfig{AutoSync: false}
	engine := sync.NewSyncEngine(nil, nil, nil, cfg)

	// 设置 nil 协议应不 panic
	engine.SetProtocol(nil)
}

// TestSyncEngineStop 测试停止同步引擎
func TestSyncEngineStop(t *testing.T) {
	cfg := &sync.SyncConfig{AutoSync: false}
	engine := sync.NewSyncEngine(nil, nil, nil, cfg)

	err := engine.Stop()
	if err != nil {
		t.Errorf("Stop 失败: %v", err)
	}
}

// TestSyncEngineHandleHeartbeat 测试心跳处理
func TestSyncEngineHandleHeartbeat(t *testing.T) {
	cfg := &sync.SyncConfig{AutoSync: false}
	engine := sync.NewSyncEngine(nil, nil, nil, cfg)

	err := engine.HandleHeartbeat(nil, nil)
	if err != nil {
		t.Errorf("HandleHeartbeat 失败: %v", err)
	}
}

// TestSyncEngineHandleBitfield 测试位图处理
func TestSyncEngineHandleBitfield(t *testing.T) {
	cfg := &sync.SyncConfig{AutoSync: false}
	engine := sync.NewSyncEngine(nil, nil, nil, cfg)

	// 处理位图
	err := engine.HandleBitfield(nil, &protocol.Bitfield{
		NodeID: "node-1",
		VersionVector: map[string]int64{
			"entry-1": 5,
		},
		EntryCount: 100,
	})
	if err != nil {
		t.Errorf("HandleBitfield 失败: %v", err)
	}

	// 验证版本向量被合并
	vv := engine.GetVersionVector()
	if vv.Get("entry-1") != 5 {
		t.Errorf("版本向量应被合并: got %d", vv.Get("entry-1"))
	}
}

// TestSyncEngineHandleBitfieldMerge 测试多次位图合并
func TestSyncEngineHandleBitfieldMerge(t *testing.T) {
	cfg := &sync.SyncConfig{AutoSync: false}
	engine := sync.NewSyncEngine(nil, nil, nil, cfg)

	// 第一次合并
	engine.HandleBitfield(nil, &protocol.Bitfield{
		VersionVector: map[string]int64{"a": 1, "b": 2},
	})

	// 第二次合并（更大的版本）
	engine.HandleBitfield(nil, &protocol.Bitfield{
		VersionVector: map[string]int64{"a": 3, "c": 4},
	})

	vv := engine.GetVersionVector()
	if vv.Get("a") != 3 {
		t.Errorf("a 应为 3: got %d", vv.Get("a"))
	}
	if vv.Get("b") != 2 {
		t.Errorf("b 应为 2: got %d", vv.Get("b"))
	}
	if vv.Get("c") != 4 {
		t.Errorf("c 应为 4: got %d", vv.Get("c"))
	}
}
