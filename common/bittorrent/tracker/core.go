package tracker

type ScrapeResult struct {
	InfoHash []byte
	ScrapeResponse
}

type Tracker interface {
	Scrape(infoHashes [][]byte) error
	Start() error
	Stop()
	Result() chan []*ScrapeResult
}
