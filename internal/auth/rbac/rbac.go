// Package rbac 提供 Polyant 的基于角色的访问控制（RBAC）系统
// 定义权限常量和用户级别的权限矩阵
package rbac

// ==================== 权限常量 ====================

const (
	PermRead           = 1 // 读取权限：可以查看词条和分类
	PermQuery          = 2 // 查询权限：可以搜索和查询词条
	PermMirror         = 3 // 镜像权限：可以镜像/同步其他节点的数据
	PermWrite          = 4 // 写入权限：可以创建和编辑词条
	PermRate           = 5 // 评分权限：可以对词条进行评分
	PermManageCategory = 6 // 分类管理权限：可以创建和编辑分类
	PermManageUser     = 7 // 用户管理权限：可以管理用户账户
	PermAdmin          = 8 // 管理员权限：拥有所有权限
)

// ==================== 用户级别常量 ====================

const (
	LevelAnonymous = 0 // 匿名用户
	LevelReader    = 1 // 普通读者
	LevelEditor    = 2 // 编辑者
	LevelModerator = 3 // 版主
	LevelAdmin     = 4 // 管理员
	LevelSuperAdmin = 5 // 超级管理员
)

// permissionNames 权限名称映射，用于日志和调试
var permissionNames = map[int]string{
	PermRead:           "read",
	PermQuery:          "query",
	PermMirror:         "mirror",
	PermWrite:          "write",
	PermRate:           "rate",
	PermManageCategory: "manage_category",
	PermManageUser:     "manage_user",
	PermAdmin:          "admin",
}

// levelNames 用户级别名称映射
var levelNames = map[int32]string{
	LevelAnonymous:  "anonymous",
	LevelReader:     "reader",
	LevelEditor:     "editor",
	LevelModerator:  "moderator",
	LevelAdmin:      "admin",
	LevelSuperAdmin: "super_admin",
}

// permissionMatrix 权限矩阵
// 定义每个用户级别拥有的权限列表
// 键为用户级别，值为该级别拥有的权限切片
var permissionMatrix = map[int32][]int{
	LevelAnonymous: {
		PermRead,
		PermQuery,
	},
	LevelReader: {
		PermRead,
		PermQuery,
		PermRate,
	},
	LevelEditor: {
		PermRead,
		PermQuery,
		PermRate,
		PermWrite,
		PermMirror,
	},
	LevelModerator: {
		PermRead,
		PermQuery,
		PermRate,
		PermWrite,
		PermMirror,
		PermManageCategory,
	},
	LevelAdmin: {
		PermRead,
		PermQuery,
		PermRate,
		PermWrite,
		PermMirror,
		PermManageCategory,
		PermManageUser,
		PermAdmin,
	},
	LevelSuperAdmin: {
		PermRead,
		PermQuery,
		PermRate,
		PermWrite,
		PermMirror,
		PermManageCategory,
		PermManageUser,
		PermAdmin,
	},
}

// HasPermission 检查指定用户级别是否拥有某项权限
// userLevel: 用户级别
// permission: 要检查的权限
// 返回 true 表示该用户级别拥有该权限
func HasPermission(userLevel int32, permission int) bool {
	perms, ok := permissionMatrix[userLevel]
	if !ok {
		return false
	}
	for _, p := range perms {
		if p == permission {
			return true
		}
	}
	return false
}

// GetPermissions 获取指定用户级别的所有权限
// userLevel: 用户级别
// 返回该级别的权限切片
func GetPermissions(userLevel int32) []int {
	perms, ok := permissionMatrix[userLevel]
	if !ok {
		return []int{}
	}
	// 返回副本以避免外部修改
	result := make([]int, len(perms))
	copy(result, perms)
	return result
}

// GetLevelName 获取用户级别的名称
func GetLevelName(level int32) string {
	if name, ok := levelNames[level]; ok {
		return name
	}
	return "unknown"
}

// GetPermissionName 获取权限的名称
func GetPermissionName(perm int) string {
	if name, ok := permissionNames[perm]; ok {
		return name
	}
	return "unknown"
}

// HasAllPermissions 检查用户级别是否拥有所有指定的权限
func HasAllPermissions(userLevel int32, permissions []int) bool {
	for _, perm := range permissions {
		if !HasPermission(userLevel, perm) {
			return false
		}
	}
	return true
}

// HasAnyPermission 检查用户级别是否拥有任意一个指定的权限
func HasAnyPermission(userLevel int32, permissions []int) bool {
	for _, perm := range permissions {
		if HasPermission(userLevel, perm) {
			return true
		}
	}
	return false
}
