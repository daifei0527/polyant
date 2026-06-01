# Polyant 配置技能

配置 Polyant 知识库连接设置。

## 配置方法

### 环境变量

```bash
export POLYANT_API_URL="http://your-node:8080"
export POLYANT_API_KEY="your-api-key"
```

### 配置文件

创建 `~/.polyant/config.json`：
```json
{
  "base_url": "http://your-node:8080",
  "api_key": "your-api-key"
}
```

## 快速设置

```bash
pactl key generate
pactl user register --name "Your Name"
pactl status
```
