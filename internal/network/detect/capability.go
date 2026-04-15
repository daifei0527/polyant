package detect

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// NATType NAT 类型
type NATType string

const (
	NATTypeNone           NATType = "none"            // 有公网 IP，无 NAT
	NATTypeFullCone       NATType = "full_cone"       // 全锥形 NAT
	NATTypeRestricted     NATType = "restricted"      // 受限锥形 NAT
	NATTypePortRestricted NATType = "port_restricted" // 端口受限 NAT
	NATTypeSymmetric      NATType = "symmetric"       // 对称 NAT
	NATTypeUnknown        NATType = "unknown"         // 未知
)

// NetworkCapability 网络能力检测结果
type NetworkCapability struct {
	HasPublicIP     bool    `json:"has_public_ip"`    // 是否有公网 IP
	PublicIP        string  `json:"public_ip"`        // 公网 IP 地址
	NATType         NATType `json:"nat_type"`         // NAT 类型
	CanBeReached    bool    `json:"can_be_reached"`   // 是否可被直接访问
	CanRelay        bool    `json:"can_relay"`        // 是否可提供中继
	RecommendedMode string  `json:"recommended_mode"` // 推荐模式：normal / service
}

// Detector 网络能力检测器
type Detector struct {
	stunServers []string
	httpClients []string
	timeout     time.Duration
}

// NewDetector 创建检测器
func NewDetector() *Detector {
	return &Detector{
		httpClients: []string{
			"https://api.ipify.org",
			"https://ifconfig.me/ip",
			"https://icanhazip.com",
		},
		timeout: 10 * time.Second,
	}
}

// Detect 检测网络能力
func (d *Detector) Detect() *NetworkCapability {
	cap := &NetworkCapability{
		NATType: NATTypeUnknown,
	}

	// 1. 检测公网 IP
	cap.PublicIP = d.detectPublicIP()
	cap.HasPublicIP = cap.PublicIP != ""

	// 2. 如果有公网 IP，测试端口可达性
	if cap.HasPublicIP {
		cap.CanBeReached = d.testPortReachability(cap.PublicIP)
		cap.NATType = d.detectNATType()
	}

	// 3. 判断是否可提供中继
	cap.CanRelay = cap.CanBeReached && cap.NATType != NATTypeSymmetric

	// 4. 推荐模式
	if cap.CanBeReached {
		cap.RecommendedMode = "service"
	} else {
		cap.RecommendedMode = "normal"
	}

	return cap
}

// detectPublicIP 检测公网 IP
func (d *Detector) detectPublicIP() string {
	client := &http.Client{
		Timeout: d.timeout,
	}

	for _, url := range d.httpClients {
		resp, err := client.Get(url)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		ip, err := io.ReadAll(resp.Body)
		if err != nil {
			continue
		}

		ipStr := strings.TrimSpace(string(ip))
		if net.ParseIP(ipStr) != nil {
			return ipStr
		}
	}

	return ""
}

// detectPublicIP 导出函数（用于测试）
func detectPublicIP() string {
	return NewDetector().detectPublicIP()
}

// testPortReachability 测试端口可达性
func (d *Detector) testPortReachability(publicIP string) bool {
	// 1. 监听临时端口
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		return false
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port

	// 2. 在后台等待连接
	acceptCh := make(chan bool, 1)
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			conn.Close()
			acceptCh <- true
		} else {
			acceptCh <- false
		}
	}()

	// 3. 尝试从外部连接（这里简化处理，实际需要外部服务配合）
	// 如果本地能连接到自己的公网 IP:port，说明端口可达
	conn, err := net.DialTimeout("tcp",
		net.JoinHostPort(publicIP, fmt.Sprintf("%d", port)),
		5*time.Second)
	if err == nil {
		conn.Close()
		return <-acceptCh
	}

	return false
}

// detectNATType 检测 NAT 类型
// 简化版本：实际需要 STUN 服务器配合
func (d *Detector) detectNATType() NATType {
	// 简化处理：
	// - 如果能获取公网 IP 且能连接，假设为 NATTypeNone
	// - 否则需要 STUN 服务器检测
	return NATTypeNone
}

// DetectNetworkCapability 便捷函数
func DetectNetworkCapability() *NetworkCapability {
	return NewDetector().Detect()
}
