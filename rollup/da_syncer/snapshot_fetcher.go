package da_syncer

type SnapshotFetcher struct {
	fetchBlockRange uint64
}

func newSnapshotFetcher(fetchBlockRange uint64) (DaFetcher, error) {
	daFetcher := SnapshotFetcher{
		fetchBlockRange: fetchBlockRange,
	}
	return &daFetcher, nil
}

func (f *SnapshotFetcher) FetchDA() (DA, error) {
	return nil, nil
}
