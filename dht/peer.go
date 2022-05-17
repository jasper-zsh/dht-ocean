package dht

import "dht-ocean/bencode"

type GetPeersRequest struct {
	*Packet
}

func NewGetPeersRequestFromPacket(pkt *Packet) *GetPeersRequest {
	req := &GetPeersRequest{pkt}
	return req
}

func (r *GetPeersRequest) NodeID() []byte {
	return bencode.MustGetBytes(r.Data, "a.id")
}

func (r *GetPeersRequest) rawInfoHash() []byte {
	return bencode.MustGetBytes(r.Data, "a.info_hash")
}

func (r *GetPeersRequest) InfoHash() []byte {
	raw := r.rawInfoHash()
	if raw == nil {
		return nil
	}
	if len(raw) < 20 {
		return nil
	}
	return raw[20:]
}

func (r *GetPeersRequest) Token() []byte {
	raw := r.rawInfoHash()
	if raw == nil {
		return nil
	}
	if len(raw) < 20 {
		return nil
	}
	return raw[:20]
}

type GetPeersResponse struct {
	*Packet
}

func NewGetPeersResponse(nodeID, token []byte) *GetPeersResponse {
	pkt := NewPacket()
	pkt.SetY("r")
	pkt.Set("r", map[string]any{
		"id":    nodeID,
		"nodes": "",
		"token": token,
	})
	r := &GetPeersResponse{pkt}
	return r
}

type AnnouncePeerRequest struct {
	*Packet
}

func NewAnnouncePeerRequestFromPacket(pkt *Packet) *AnnouncePeerRequest {
	return &AnnouncePeerRequest{pkt}
}

func (r *AnnouncePeerRequest) NodeID() []byte {
	return bencode.MustGetBytes(r.Data, "a.id")
}

func (r *AnnouncePeerRequest) InfoHash() []byte {
	return bencode.MustGetBytes(r.Data, "a.info_hash")
}

func (r *AnnouncePeerRequest) ImpliedPort() int {
	i, _ := bencode.GetInt(r.Data, "a.implied_port")
	return i
}

func (r *AnnouncePeerRequest) Port() int {
	i, _ := bencode.GetInt(r.Data, "a.port")
	return i
}

func (r *AnnouncePeerRequest) Token() []byte {
	return bencode.MustGetBytes(r.Data, "a.token")
}

func (r *AnnouncePeerRequest) Name() string {
	s, _ := bencode.GetString(r.Data, "a.name")
	return s
}
