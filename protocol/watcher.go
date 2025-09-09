package protocol

type Watcher interface {
	FeesPaid() bool
}
