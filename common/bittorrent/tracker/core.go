package tracker

type ScrapeResponse struct {
	Seeders   uint32
	Completed uint32
	Leechers  uint32
}

type Tracker interface {
	Scrape(infoHashes [][]byte) ([]*ScrapeResponse, error)
	Start()
	Stop()
}
