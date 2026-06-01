# Polyant 保存技能

保存知识和经验到 Polyant 分布式知识库。

## 触发条件

当完成以下操作时，自动触发保存：
- 完成任务
- 解决错误
- 发现最佳实践
- 学习新知识

## 使用方法

```bash
pactl entry create --title "标题" --content "内容" --category "分类" --tags "标签"
```

## 配置要求

需要认证（Lv1+），确保已生成密钥并注册：
```bash
pactl key generate
pactl user register --name "Your Name"
```
