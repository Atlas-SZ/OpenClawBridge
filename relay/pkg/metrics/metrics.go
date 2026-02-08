package metrics

import "sync/atomic"

type Collector struct {
	forwardedBytes atomic.Int64
	errors         atomic.Int64
}

func New() *Collector {
	return &Collector{}
}

func (c *Collector) AddForwardedBytes(n int) {
	c.forwardedBytes.Add(int64(n))
}

func (c *Collector) IncError() {
	c.errors.Add(1)
}

type Snapshot struct {
	ForwardedBytes int64
	Errors         int64
}

func (c *Collector) Snapshot() Snapshot {
	return Snapshot{
		ForwardedBytes: c.forwardedBytes.Load(),
		Errors:         c.errors.Load(),
	}
}
