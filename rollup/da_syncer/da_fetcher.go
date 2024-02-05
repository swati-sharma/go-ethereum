package da_syncer

// DaFetcher encapsulates functions required to fetch data from l1
type DaFetcher interface {
	FetchDA() (DA, error)
}
