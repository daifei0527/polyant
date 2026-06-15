// 本文件曾容纳基于 JSON 的线协议 codec（Codec / StreamReader / StreamWriter）。
// 生产协议路径已统一改用 protobuf（见 protobuf_codec.go 与 protocol.go 的
// ProtobufCodec），JSON codec 无任何生产调用方，故已移除。此处仅保留被
// protobuf 路径与消息构造共用的内存消息信封与头部 helper。
package protocol

import (
	"fmt"
	"time"
)

const (
	MaxMessageSize = 64 * 1024 * 1024
	HeaderSize     = 4
)

// Message 是协议消息的内存表示（头部 + 载荷）。生产路径把它经 toProtoMessage
// 转换为 protobuf 后上线路；本类型仅作为消息构造的中间表示。
type Message struct {
	Header  *MessageHeader
	Payload interface{}
}

// NewMessageHeader 构造带类型、消息 ID 与时间戳的消息头部。
func NewMessageHeader(msgType MessageType) *MessageHeader {
	return &MessageHeader{
		Type:      msgType,
		MessageID: generateMessageID(),
		Timestamp: currentTimeMillis(),
	}
}

func generateMessageID() string {
	return fmt.Sprintf("%d", currentTimeMillis())
}

func currentTimeMillis() int64 {
	return time.Now().UnixMilli()
}
