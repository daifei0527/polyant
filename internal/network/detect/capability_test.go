// internal/network/detect/capability_test.go

package detect

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectPublicIP(t *testing.T) {
	// 这个测试可能会因为网络原因失败，所以标记为集成测试
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ip := detectPublicIP()
	// 如果有网络连接，应该返回一个 IP 地址
	if ip != "" {
		assert.Regexp(t, `^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$`, ip)
	}
}

func TestNetworkCapability(t *testing.T) {
	cap := &NetworkCapability{
		HasPublicIP:     true,
		PublicIP:        "1.2.3.4",
		NATType:         NATTypeNone,
		CanBeReached:    true,
		CanRelay:        true,
		RecommendedMode: "service",
	}

	assert.True(t, cap.HasPublicIP)
	assert.Equal(t, "service", cap.RecommendedMode)
}

func TestNATTypeString(t *testing.T) {
	tests := []struct {
		natType NATType
		want    string
	}{
		{NATTypeNone, "none"},
		{NATTypeFullCone, "full_cone"},
		{NATTypeRestricted, "restricted"},
		{NATTypePortRestricted, "port_restricted"},
		{NATTypeSymmetric, "symmetric"},
		{NATTypeUnknown, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, string(tt.natType))
		})
	}
}
