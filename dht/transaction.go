package dht

import (
	"encoding/binary"
	"time"
)

type TransactionContext struct {
	Tid       []byte
	QueryType string
	CreatedAt time.Time
}

type TransactionStorage map[string]*TransactionContext

func (s TransactionStorage) Add(ctx *TransactionContext) {
	sTid := string(ctx.Tid)
	s[sTid] = ctx
}

func (s TransactionStorage) Get(tid []byte) *TransactionContext {
	sTid := string(tid)
	return s[sTid]
}

type TransactionManager struct {
	nextTransactionID uint32
}

func (tm *TransactionManager) NextTransactionID() []byte {
	tid := make([]byte, 4)
	binary.BigEndian.PutUint32(tid, tm.nextTransactionID)
	tm.nextTransactionID += 1
	return tid
}
