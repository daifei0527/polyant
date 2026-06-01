package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	configPath := flag.String("config", "", "配置文件路径")
	flag.Parse()

	if *configPath == "" {
		homeDir, _ := os.UserHomeDir()
		*configPath = homeDir + "/.polyant/config.json"
	}

	config, err := LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	server, err := NewServer(config)
	if err != nil {
		log.Fatalf("创建服务器失败: %v", err)
	}

	fmt.Fprintf(os.Stderr, "Polyant MCP 服务器启动中...\n")
	fmt.Fprintf(os.Stderr, "API 地址: %s\n", config.BaseURL)

	if err := server.Run(); err != nil {
		log.Fatalf("服务器运行失败: %v", err)
	}
}
