package packet

import "time"

const (
	SessionWriteBufferSize = 256 * 1024
	SessionReadBufferSize  = 256 * 1024
	SessionSendInterval    = 10 * time.Millisecond
	SessionInitRto         = 1000 * time.Millisecond
	SessionAckInterval     = 10 * time.Millisecond
	MTU                    = 1280
	Ipv4Header             = 60
	Ipv6Header             = 40
	UdpHeader              = 20
	PacketHeader           = 20
	Ipv4PayloadSize        = MTU - Ipv4Header - UdpHeader
	Ipv6PayloadSize        = MTU - Ipv6Header - UdpHeader
	FastTransmitCount      = 3
)
