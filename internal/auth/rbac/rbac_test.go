// Package rbac_test 提供基于角色的访问控制系统的单元测试
package rbac_test

import (
	"testing"

	"github.com/daifei0527/polyant/internal/auth/rbac"
)

// TestHasPermission 测试权限检查
func TestHasPermission(t *testing.T) {
	tests := []struct {
		name       string
		level      int32
		permission int
		expected   bool
	}{
		// 匿名用户
		{"匿名用户-读取", rbac.LevelAnonymous, rbac.PermRead, true},
		{"匿名用户-查询", rbac.LevelAnonymous, rbac.PermQuery, true},
		{"匿名用户-写入", rbac.LevelAnonymous, rbac.PermWrite, false},
		{"匿名用户-评分", rbac.LevelAnonymous, rbac.PermRate, false},
		{"匿名用户-管理", rbac.LevelAnonymous, rbac.PermAdmin, false},

		// 读者
		{"读者-读取", rbac.LevelReader, rbac.PermRead, true},
		{"读者-查询", rbac.LevelReader, rbac.PermQuery, true},
		{"读者-评分", rbac.LevelReader, rbac.PermRate, true},
		{"读者-写入", rbac.LevelReader, rbac.PermWrite, false},
		{"读者-管理分类", rbac.LevelReader, rbac.PermManageCategory, false},

		// 编辑者
		{"编辑者-读取", rbac.LevelEditor, rbac.PermRead, true},
		{"编辑者-评分", rbac.LevelEditor, rbac.PermRate, true},
		{"编辑者-写入", rbac.LevelEditor, rbac.PermWrite, true},
		{"编辑者-镜像", rbac.LevelEditor, rbac.PermMirror, true},
		{"编辑者-管理分类", rbac.LevelEditor, rbac.PermManageCategory, false},

		// 版主
		{"版主-写入", rbac.LevelModerator, rbac.PermWrite, true},
		{"版主-管理分类", rbac.LevelModerator, rbac.PermManageCategory, true},
		{"版主-管理用户", rbac.LevelModerator, rbac.PermManageUser, false},

		// 管理员
		{"管理员-管理分类", rbac.LevelAdmin, rbac.PermManageCategory, true},
		{"管理员-管理用户", rbac.LevelAdmin, rbac.PermManageUser, true},
		{"管理员-管理员权限", rbac.LevelAdmin, rbac.PermAdmin, true},

		// 超级管理员
		{"超级管理员-所有权限", rbac.LevelSuperAdmin, rbac.PermAdmin, true},
		{"超级管理员-管理用户", rbac.LevelSuperAdmin, rbac.PermManageUser, true},

		// 无效级别
		{"无效级别", 999, rbac.PermRead, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := rbac.HasPermission(tc.level, tc.permission)
			if result != tc.expected {
				t.Errorf("HasPermission(%d, %d) = %v, want %v",
					tc.level, tc.permission, result, tc.expected)
			}
		})
	}
}

// TestGetPermissions 测试获取权限列表
func TestGetPermissions(t *testing.T) {
	tests := []struct {
		name          string
		level         int32
		expectedCount int
	}{
		{"匿名用户", rbac.LevelAnonymous, 2},
		{"读者", rbac.LevelReader, 3},
		{"编辑者", rbac.LevelEditor, 5},
		{"版主", rbac.LevelModerator, 6},
		{"管理员", rbac.LevelAdmin, 8},
		{"超级管理员", rbac.LevelSuperAdmin, 8},
		{"无效级别", 999, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			perms := rbac.GetPermissions(tc.level)
			if len(perms) != tc.expectedCount {
				t.Errorf("GetPermissions(%d) 返回 %d 个权限, 期望 %d",
					tc.level, len(perms), tc.expectedCount)
			}
		})
	}
}

// TestGetPermissionsReturnsCopy 测试返回的是副本
func TestGetPermissionsReturnsCopy(t *testing.T) {
	perms1 := rbac.GetPermissions(rbac.LevelEditor)
	perms2 := rbac.GetPermissions(rbac.LevelEditor)

	// 修改一个不应影响另一个
	perms1[0] = 999

	if perms2[0] == 999 {
		t.Error("GetPermissions 应返回副本而非引用")
	}
}

// TestGetLevelName 测试获取级别名称
func TestGetLevelName(t *testing.T) {
	tests := []struct {
		level    int32
		expected string
	}{
		{rbac.LevelAnonymous, "anonymous"},
		{rbac.LevelReader, "reader"},
		{rbac.LevelEditor, "editor"},
		{rbac.LevelModerator, "moderator"},
		{rbac.LevelAdmin, "admin"},
		{rbac.LevelSuperAdmin, "super_admin"},
		{999, "unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			name := rbac.GetLevelName(tc.level)
			if name != tc.expected {
				t.Errorf("GetLevelName(%d) = %q, want %q",
					tc.level, name, tc.expected)
			}
		})
	}
}

// TestGetPermissionName 测试获取权限名称
func TestGetPermissionName(t *testing.T) {
	tests := []struct {
		permission int
		expected   string
	}{
		{rbac.PermRead, "read"},
		{rbac.PermQuery, "query"},
		{rbac.PermMirror, "mirror"},
		{rbac.PermWrite, "write"},
		{rbac.PermRate, "rate"},
		{rbac.PermManageCategory, "manage_category"},
		{rbac.PermManageUser, "manage_user"},
		{rbac.PermAdmin, "admin"},
		{999, "unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			name := rbac.GetPermissionName(tc.permission)
			if name != tc.expected {
				t.Errorf("GetPermissionName(%d) = %q, want %q",
					tc.permission, name, tc.expected)
			}
		})
	}
}

// TestHasAllPermissions 测试检查所有权限
func TestHasAllPermissions(t *testing.T) {
	tests := []struct {
		name        string
		level       int32
		permissions []int
		expected    bool
	}{
		{
			name:        "编辑者拥有读取和写入",
			level:       rbac.LevelEditor,
			permissions: []int{rbac.PermRead, rbac.PermWrite},
			expected:    true,
		},
		{
			name:        "编辑者缺少管理权限",
			level:       rbac.LevelEditor,
			permissions: []int{rbac.PermRead, rbac.PermWrite, rbac.PermManageUser},
			expected:    false,
		},
		{
			name:        "管理员拥有所有基础权限",
			level:       rbac.LevelAdmin,
			permissions: []int{rbac.PermRead, rbac.PermWrite, rbac.PermRate, rbac.PermManageCategory},
			expected:    true,
		},
		{
			name:        "空权限列表",
			level:       rbac.LevelAnonymous,
			permissions: []int{},
			expected:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := rbac.HasAllPermissions(tc.level, tc.permissions)
			if result != tc.expected {
				t.Errorf("HasAllPermissions(%d, %v) = %v, want %v",
					tc.level, tc.permissions, result, tc.expected)
			}
		})
	}
}

// TestHasAnyPermission 测试检查任意权限
func TestHasAnyPermission(t *testing.T) {
	tests := []struct {
		name        string
		level       int32
		permissions []int
		expected    bool
	}{
		{
			name:        "匿名用户有读取或写入",
			level:       rbac.LevelAnonymous,
			permissions: []int{rbac.PermRead, rbac.PermWrite},
			expected:    true, // 有读取权限
		},
		{
			name:        "匿名用户有写入或管理",
			level:       rbac.LevelAnonymous,
			permissions: []int{rbac.PermWrite, rbac.PermManageUser},
			expected:    false, // 都没有
		},
		{
			name:        "编辑者有任意权限",
			level:       rbac.LevelEditor,
			permissions: []int{rbac.PermManageUser, rbac.PermWrite},
			expected:    true, // 有写入权限
		},
		{
			name:        "空权限列表",
			level:       rbac.LevelEditor,
			permissions: []int{},
			expected:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := rbac.HasAnyPermission(tc.level, tc.permissions)
			if result != tc.expected {
				t.Errorf("HasAnyPermission(%d, %v) = %v, want %v",
					tc.level, tc.permissions, result, tc.expected)
			}
		})
	}
}

// TestPermissionHierarchy 测试权限层级关系
func TestPermissionHierarchy(t *testing.T) {
	// 高级别用户应拥有低级别用户的所有权限
	levels := []int32{
		rbac.LevelAnonymous,
		rbac.LevelReader,
		rbac.LevelEditor,
		rbac.LevelModerator,
		rbac.LevelAdmin,
		rbac.LevelSuperAdmin,
	}

	for i := 1; i < len(levels); i++ {
		lowerPerms := rbac.GetPermissions(levels[i-1])
		higherPerms := rbac.GetPermissions(levels[i])

		// 检查高级别用户是否拥有低级别的所有权限
		for _, perm := range lowerPerms {
			if !rbac.HasPermission(levels[i], perm) {
				t.Errorf("级别 %d 应拥有级别 %d 的权限 %d",
					levels[i], levels[i-1], perm)
			}
		}

		// 高级别权限数量应 >= 低级别
		if len(higherPerms) < len(lowerPerms) {
			t.Errorf("级别 %d 的权限数量 (%d) 不应少于级别 %d (%d)",
				levels[i], len(higherPerms), levels[i-1], len(lowerPerms))
		}
	}
}

// TestPermissionConstants 测试权限常量值
func TestPermissionConstants(t *testing.T) {
	// 确保权限常量是唯一且递增的
	perms := []int{
		rbac.PermRead,
		rbac.PermQuery,
		rbac.PermMirror,
		rbac.PermWrite,
		rbac.PermRate,
		rbac.PermManageCategory,
		rbac.PermManageUser,
		rbac.PermAdmin,
	}

	seen := make(map[int]bool)
	for i, p := range perms {
		if seen[p] {
			t.Errorf("权限常量 %d 重复", p)
		}
		seen[p] = true

		if i > 0 && p <= perms[i-1] {
			t.Errorf("权限常量不是严格递增: %d -> %d", perms[i-1], p)
		}
	}
}

// TestLevelConstants 测试级别常量值
func TestLevelConstants(t *testing.T) {
	// 确保级别常量是递增的
	levels := []int32{
		rbac.LevelAnonymous,
		rbac.LevelReader,
		rbac.LevelEditor,
		rbac.LevelModerator,
		rbac.LevelAdmin,
		rbac.LevelSuperAdmin,
	}

	for i := 1; i < len(levels); i++ {
		if levels[i] <= levels[i-1] {
			t.Errorf("级别常量不是严格递增: %d -> %d", levels[i-1], levels[i])
		}
	}
}
