package tracker

type ScrapeResponse struct {
	Seeders   int32
	Completed int32
	Leechers  int32
}

type Tracker interface {
	Scrape(infoHashes [][]byte) ([]*ScrapeResponse, error)
	Start() error
	Stop() error
}
