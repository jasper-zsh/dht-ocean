package dht

import "time"

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
