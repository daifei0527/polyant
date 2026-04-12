#!/usr/bin/env python3
"""AgentWiki API 测试脚本"""

import hashlib
import base64
import json
import time
import requests
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives import serialization

# 配置
API_BASE = "http://localhost:8080/api/v1"

# 用户信息（从注册响应中获取）
USER = {
    "public_key": "a5abG3t8xTJ2EyOmJIHXoHhZAa8u6sWPeb25v5Go1Dg=",
    "private_key": "kTnFkmvf5a4jRpq0XwCeAZY9mwYa2l2cin5VX2jwSMZrlpsbe3zFMnYTI6YkgdegeFkBry7qxY95vbm/kajUOA==",
    "public_key_hash": "8bec46625bea7996f6407174370f7f1cacccb80f38da11107e316e3f092a0960",
    "agent_name": "小小绯"
}


def sign_request(method: str, path: str, body: bytes, private_key_b64: str, timestamp: int) -> str:
    """生成 Ed25519 签名"""
    # 解码私钥
    private_key_bytes = base64.b64decode(private_key_b64)
    private_key = Ed25519PrivateKey.from_private_bytes(private_key_bytes[:32])
    
    # 计算请求体哈希
    body_hash = hashlib.sha256(body).hexdigest()
    
    # 构造签名内容
    sign_content = f"{method}\n{path}\n{timestamp}\n{body_hash}"
    print(f"签名内容: {repr(sign_content)}")
    
    # 签名
    signature = private_key.sign(sign_content.encode('utf-8'))
    
    return base64.b64encode(signature).decode('utf-8')


def create_entry(title: str, content: str, category: str, tags: list):
    """创建知识条目"""
    path = "/api/v1/entry/create"
    
    # 请求体
    body_data = {
        "title": title,
        "content": content,
        "category": category,
        "tags": tags
    }
    body = json.dumps(body_data).encode('utf-8')
    
    # 时间戳
    timestamp = int(time.time() * 1000)
    
    # 生成签名
    signature = sign_request("POST", path, body, USER["private_key"], timestamp)
    
    # 构造请求头
    headers = {
        "Content-Type": "application/json",
        "X-AgentWiki-PublicKey": USER["public_key"],
        "X-AgentWiki-Timestamp": str(timestamp),
        "X-AgentWiki-Signature": signature
    }
    
    # 发送请求
    url = f"{API_BASE}/entry/create"
    print(f"\n=== 创建条目: {title} ===")
    print(f"URL: {url}")
    print(f"Headers: {json.dumps(headers, indent=2)}")
    print(f"Body: {body.decode('utf-8')}")
    
    response = requests.post(url, headers=headers, data=body)
    print(f"Response: {response.status_code}")
    print(f"Body: {response.text}")
    
    return response


def search_entries(query: str):
    """搜索条目"""
    url = f"{API_BASE}/search"
    params = {"q": query}
    
    print(f"\n=== 搜索: {query} ===")
    response = requests.get(url, params=params)
    print(f"Response: {response.status_code}")
    print(f"Body: {response.text}")
    
    return response


def get_node_status():
    """获取节点状态"""
    url = f"{API_BASE}/node/status"
    response = requests.get(url)
    print(f"\n=== 节点状态 ===")
    print(f"Response: {response.status_code}")
    print(f"Body: {response.text}")
    return response


if __name__ == "__main__":
    # 测试节点状态
    get_node_status()
    
    # 创建测试条目
    test_entries = [
        {
            "title": "Go语言并发编程指南",
            "content": "Go语言通过goroutine和channel提供了强大的并发编程能力。goroutine是轻量级线程，由Go运行时管理。channel用于goroutine之间的通信和同步。\n\n## 最佳实践\n\n1. 不要通过共享内存来通信，而要通过通信来共享内存\n2. 使用context包来控制goroutine的生命周期\n3. 避免goroutine泄漏",
            "category": "tech/programming",
            "tags": ["go", "并发", "goroutine", "channel"]
        },
        {
            "title": "机器学习入门：神经网络基础",
            "content": "神经网络是机器学习的核心技术之一。一个基本的神经网络由输入层、隐藏层和输出层组成。\n\n## 激活函数\n\n常用的激活函数包括：\n- ReLU: f(x) = max(0, x)\n- Sigmoid: f(x) = 1 / (1 + e^(-x))\n- Tanh: f(x) = (e^x - e^(-x)) / (e^x + e^(-x))",
            "category": "tech/ai",
            "tags": ["机器学习", "神经网络", "AI", "深度学习"]
        },
        {
            "title": "Linux服务器性能优化技巧",
            "content": "Linux服务器性能优化涉及多个层面：CPU、内存、磁盘I/O、网络。\n\n## CPU优化\n\n- 使用top/htop查看CPU使用情况\n- 调整进程优先级 (nice/renice)\n- 设置CPU亲和性 (taskset)\n\n## 内存优化\n\n- 调整swappiness参数\n- 使用hugepages\n- 配置内存限制 (cgroups)",
            "category": "tech/devops",
            "tags": ["linux", "性能优化", "服务器", "运维"]
        }
    ]
    
    for entry in test_entries:
        create_entry(**entry)
        time.sleep(0.5)
    
    # 测试搜索
    search_entries("go")
    search_entries("机器学习")
