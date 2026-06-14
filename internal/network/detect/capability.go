package detect

import (
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
	httpClient  *http.Client
	httpClients []string
	timeout     time.Duration
}

// NewDetector 创建检测器
func NewDetector() *Detector {
	timeout := 10 * time.Second
	return &Detector{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		httpClients: []string{
			"https://api.ipify.org",
			"https://ifconfig.me/ip",
			"https://icanhazip.com",
		},
		timeout: timeout,
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

	// 2. 如果有公网 IP：外部可达性无法从本机可靠判定（旧 testPortReachability 自拨本机
	//    公网 IP 会假阳性——许多 NAT 允许内部回环）。诚实降级：有公网 IP 视为"可能可达"，
	//    真实外部可达性需外部探测服务确认。
	if cap.HasPublicIP {
		cap.CanBeReached = true
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
	for _, url := range d.httpClients {
		resp, err := d.httpClient.Get(url)
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

// detectNATType 检测 NAT 类型
// 简化版本：实际需要 STUN 服务器配合
func (d *Detector) detectNATType() NATType {
	// 简化处理：当前未实现真正的 NAT 类型检测
	// 返回 NATTypeUnknown 而非 NATTypeNone，因为我们无法确定
	// 实际的 NAT 类型。真正的检测需要 STUN 服务器配合。
	return NATTypeUnknown
}

// DetectNetworkCapability 便捷函数
func DetectNetworkCapability() *NetworkCapability {
	return NewDetector().Detect()
}
