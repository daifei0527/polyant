// internal/network/detect/capability_test.go

package detect

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewDetector(t *testing.T) {
	d := NewDetector()
	assert.NotNil(t, d)
	assert.NotNil(t, d.httpClient)
	assert.NotEmpty(t, d.httpClients)
	assert.Equal(t, 10*time.Second, d.timeout)
}

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

func TestDetector_detectPublicIP(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	d := NewDetector()
	ip := d.detectPublicIP()
	// May return empty if no network
	if ip != "" {
		assert.Regexp(t, `^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$`, ip)
	}
}

func TestDetector_Detect(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	d := NewDetector()
	cap := d.Detect()

	assert.NotNil(t, cap)
	assert.NotEmpty(t, cap.NATType)
	assert.NotEmpty(t, cap.RecommendedMode)
}

func TestDetector_detectNATType(t *testing.T) {
	d := NewDetector()
	natType := d.detectNATType()

	// Current implementation returns NATTypeUnknown
	assert.Equal(t, NATTypeUnknown, natType)
}

func TestDetectNetworkCapability(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cap := DetectNetworkCapability()
	assert.NotNil(t, cap)
	assert.NotEmpty(t, cap.RecommendedMode)
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
	assert.Equal(t, "1.2.3.4", cap.PublicIP)
	assert.Equal(t, NATTypeNone, cap.NATType)
	assert.True(t, cap.CanBeReached)
	assert.True(t, cap.CanRelay)
}

func TestNetworkCapability_NoPublicIP(t *testing.T) {
	cap := &NetworkCapability{
		HasPublicIP:     false,
		PublicIP:        "",
		NATType:         NATTypeUnknown,
		CanBeReached:    false,
		CanRelay:        false,
		RecommendedMode: "normal",
	}

	assert.False(t, cap.HasPublicIP)
	assert.Equal(t, "normal", cap.RecommendedMode)
	assert.Empty(t, cap.PublicIP)
	assert.False(t, cap.CanBeReached)
	assert.False(t, cap.CanRelay)
}

func TestNetworkCapability_CanRelay(t *testing.T) {
	tests := []struct {
		name          string
		cap           *NetworkCapability
		expectedRelay bool
	}{
		{
			name: "reachable with non-symmetric NAT",
			cap: &NetworkCapability{
				CanBeReached: true,
				NATType:      NATTypeFullCone,
			},
			expectedRelay: true,
		},
		{
			name: "reachable with symmetric NAT",
			cap: &NetworkCapability{
				CanBeReached: true,
				NATType:      NATTypeSymmetric,
			},
			expectedRelay: false,
		},
		{
			name: "not reachable",
			cap: &NetworkCapability{
				CanBeReached: false,
				NATType:      NATTypeNone,
			},
			expectedRelay: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the logic from Detect()
			tt.cap.CanRelay = tt.cap.CanBeReached && tt.cap.NATType != NATTypeSymmetric
			assert.Equal(t, tt.expectedRelay, tt.cap.CanRelay)
		})
	}
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

func TestNetworkCapability_RecommendedMode(t *testing.T) {
	tests := []struct {
		name           string
		canBeReached   bool
		expectedMode   string
	}{
		{
			name:         "reachable becomes service",
			canBeReached: true,
			expectedMode: "service",
		},
		{
			name:         "not reachable becomes normal",
			canBeReached: false,
			expectedMode: "normal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cap := &NetworkCapability{
				CanBeReached: tt.canBeReached,
			}
			// Simulate the logic from Detect()
			if cap.CanBeReached {
				cap.RecommendedMode = "service"
			} else {
				cap.RecommendedMode = "normal"
			}
			assert.Equal(t, tt.expectedMode, cap.RecommendedMode)
		})
	}
}
