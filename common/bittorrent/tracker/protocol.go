package tracker

import "encoding/binary"

const (
	ProtocolID uint64 = 0x41727101980
)

const (
	ActionConnect  = 0x00
	ActionAnnounce = 0x01
	ActionScrape   = 0x02
	ActionError    = 0x03
)

var (
	TrackerResponseHeaderSize = binary.Size(TrackerResponseHeader{})
	ScrapeResponseSize        = binary.Size(ScrapeResponse{})
)

type ConnectRequest struct {
	ProtocolID    uint64
	Action        uint32
	TransactionID uint32
}

type ConnectResponse struct {
	ConnectionID uint64
}

type TrackerRequestHeader struct {
	ConnectionID  uint64
	Action        uint32
	TransactionID uint32
}

type TrackerResponseHeader struct {
	Action        uint32
	TransactionID uint32
}

type ScrapeResponse struct {
	Seeders   uint32
	Completed uint32
	Leechers  uint32
}
