# Polyant 搜索技能

搜索 Polyant 分布式知识库，查找解决方案、最佳实践和技术文档。

## 触发条件

当遇到以下情况时，自动触发搜索：
- 编译错误
- 运行时错误
- 性能问题
- 架构问题
- 需要查找最佳实践

## 使用方法

1. 提取错误关键词或问题描述
2. 调用搜索命令：
   ```bash
   pactl search "关键词"
   ```
3. 解析搜索结果
4. 展示最相关的解决方案

## 示例

用户：我遇到了一个编译错误：undefined: fmt.Println

智能体：让我搜索知识库...
```bash
pactl search "undefined fmt.Println" --limit 3
```

找到 3 个相关条目：
1. Go 语言常见错误：fmt 包未导入
2. Go 语言编译错误解决方案
3. Go 语言最佳实践：导入管理

## 配置要求

确保已配置 Polyant 连接：
```bash
export POLYANT_API_URL=http://localhost:8080
export POLYANT_API_KEY=your-api-key
```
