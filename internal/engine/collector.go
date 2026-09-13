package engine

import (
	"context"
	"sync"
	"time"

	"github.com/Coffey-Labs/WGX/internal/store"
)

// Live is what the data plane currently says about one peer, merged with
// the totals persisted across restarts.
type Live struct {
	PeerID         string    `json:"id"`
	Connected      bool      `json:"connected"`
	Endpoint       string    `json:"endpoint,omitempty"`
	LastHandshake  time.Time `json:"lastHandshake,omitempty"`
	Rx             int64     `json:"rx"`
	Tx             int64     `json:"tx"`
	RxRate         float64   `json:"rxRate"`
	TxRate         float64   `json:"txRate"`
	ConnectedSince time.Time `json:"connectedSince,omitempty"`
}

// Totals summarises the whole interface.
type Totals struct {
	Peers     int     `json:"peers"`
	Active    int     `json:"active"`
	Connected int     `json:"connected"`
	Rx        int64   `json:"rx"`
	Tx        int64   `json:"tx"`
	RxRate    float64 `json:"rxRate"`
	TxRate    float64 `json:"txRate"`
}

// Snapshot is the payload pushed to dashboards on every poll.
type Snapshot struct {
	At     time.Time       `json:"at"`
	Totals Totals          `json:"totals"`
	Peers  map[string]Live `json:"peers"`
}

type counterMemo struct {
	rx, tx int64 // raw device counters at the last poll
	at     time.Time
}

// collector polls the backend, turns raw counters into deltas and rates,
// and batches what needs writing.
type collector struct {
	e    *Engine
	mu   sync.Mutex
	live map[string]*Live         // by peer id
	memo map[string]counterMemo   // by public key
	// base is the persisted total per peer at the moment it was loaded, so
	// that live totals = base + everything seen since.
	pendingBuckets map[string]map[int64]*[2]int64 // peer id -> bucket -> [rx, tx]
	dirty          map[string]bool
	last           Snapshot
}

func newCollector(e *Engine) *collector {
	return &collector{e: e, live: map[string]*Live{}, memo: map[string]counterMemo{}, pendingBuckets: map[string]map[int64]*[2]int64{}, dirty: map[string]bool{}}
}

// Snapshot returns the last computed snapshot.
func (c *collector) Snapshot() Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.last
}

// LiveFor returns a copy of a peer's live state, if any.
func (c *collector) LiveFor(id string) (Live, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	l, ok := c.live[id]
	if !ok {
		return Live{}, false
	}
	return *l, true
}

func (c *collector) run(ctx context.Context) {
	t := time.NewTicker(c.e.cfg.PollInterval)
	defer t.Stop()
	c.poll(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c.poll(ctx)
		}
	}
}

// forget drops a peer's live state after it is deleted.
func (c *collector) forget(id, pubKey string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.live, id)
	delete(c.memo, pubKey)
	delete(c.pendingBuckets, id)
	delete(c.dirty, id)
}

// rekey moves the memo when a peer's key changes so the next poll does not
// count the new key's zeroed counters as a reset.
func (c *collector) rekey(oldKey string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.memo, oldKey)
}

func (c *collector) poll(ctx context.Context) {
	dev, err := c.e.be.Device(ctx)
	if err != nil {
		c.e.log.Warn("poll", "error", err)
		return
	}
	now := time.Now()
	window := time.Duration(c.e.Settings().ConnectedWindow) * time.Second

	c.e.mu.RLock()
	peers := make([]*store.Peer, 0, len(c.e.peers))
	for _, p := range c.e.peers {
		peers = append(peers, p)
	}
	c.e.mu.RUnlock()

	seen := map[string]struct{}{}
	for _, ps := range dev.Peers {
		seen[ps.PublicKey.String()] = struct{}{}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	snap := Snapshot{At: now, Peers: make(map[string]Live, len(peers))}
	snap.Totals.Peers = len(peers)
	byKey := map[string]int{}
	for i, ps := range dev.Peers {
		byKey[ps.PublicKey.String()] = i
	}
	for _, p := range peers {
		l, ok := c.live[p.ID]
		if !ok {
			l = &Live{PeerID: p.ID, Rx: p.RxTotal, Tx: p.TxTotal, LastHandshake: p.LastHandshake, Endpoint: p.LastEndpoint}
			c.live[p.ID] = l
		}
		l.RxRate, l.TxRate = 0, 0
		if active(p, now) {
			snap.Totals.Active++
		}
		if i, ok := byKey[p.PublicKey]; ok {
			ps := dev.Peers[i]
			m, had := c.memo[p.PublicKey]
			var drx, dtx int64
			if had {
				drx, dtx = ps.ReceiveBytes-m.rx, ps.TransmitBytes-m.tx
				// A counter smaller than last time means the peer was
				// removed and re-added: the new value is all new traffic.
				if drx < 0 {
					drx = ps.ReceiveBytes
				}
				if dtx < 0 {
					dtx = ps.TransmitBytes
				}
				dt := now.Sub(m.at).Seconds()
				if dt > 0 {
					l.RxRate = float64(drx) / dt
					l.TxRate = float64(dtx) / dt
				}
			} else {
				// First sight of this key since start: whatever the device
				// already counted happened before we were watching, unless
				// the peer was just created, in which case it is zero anyway.
				// Either way it must not be added to the persisted total, so
				// only the memo is set.
				drx, dtx = 0, 0
			}
			c.memo[p.PublicKey] = counterMemo{rx: ps.ReceiveBytes, tx: ps.TransmitBytes, at: now}
			if drx > 0 || dtx > 0 {
				l.Rx += drx
				l.Tx += dtx
				c.dirty[p.ID] = true
				bucket := now.Truncate(bucketSize).Unix()
				pb, ok := c.pendingBuckets[p.ID]
				if !ok {
					pb = map[int64]*[2]int64{}
					c.pendingBuckets[p.ID] = pb
				}
				b, ok := pb[bucket]
				if !ok {
					b = &[2]int64{}
					pb[bucket] = b
				}
				b[0] += drx
				b[1] += dtx
			}
			if !ps.LastHandshake.IsZero() && ps.LastHandshake.After(l.LastHandshake) {
				l.LastHandshake = ps.LastHandshake
				c.dirty[p.ID] = true
			}
			if ps.Endpoint != nil {
				ep := ps.Endpoint.String()
				if ep != l.Endpoint {
					l.Endpoint = ep
					c.dirty[p.ID] = true
				}
			}
			// Connected means the *interface* has a recent handshake, not
			// the remembered one: after a restart, or after a peer is
			// disabled and re-enabled, everyone is disconnected until the
			// client handshakes again, which is the truth.
			connected := !ps.LastHandshake.IsZero() && now.Sub(ps.LastHandshake) < window
			if connected && !l.Connected {
				l.ConnectedSince = l.LastHandshake
			}
			if !connected {
				l.ConnectedSince = time.Time{}
			}
			l.Connected = connected
		} else {
			// Not on the interface: disabled, expired or removed by hand.
			l.Connected = false
			l.ConnectedSince = time.Time{}
			delete(c.memo, p.PublicKey)
		}
		if l.Connected {
			snap.Totals.Connected++
		}
		snap.Totals.Rx += l.Rx
		snap.Totals.Tx += l.Tx
		snap.Totals.RxRate += l.RxRate
		snap.Totals.TxRate += l.TxRate
		snap.Peers[p.ID] = *l
	}
	c.last = snap
	c.e.hub.Publish("status", snap)
}

// flush writes dirty totals and pending buckets to the database.
func (c *collector) flush(ctx context.Context) error {
	c.mu.Lock()
	var counters []store.PeerCounters
	var samples []store.TrafficSample
	for id := range c.dirty {
		l := c.live[id]
		if l == nil {
			continue
		}
		counters = append(counters, store.PeerCounters{ID: id, RxTotal: l.Rx, TxTotal: l.Tx, LastHandshake: l.LastHandshake, LastEndpoint: l.Endpoint})
	}
	for id, pb := range c.pendingBuckets {
		for bucket, v := range pb {
			samples = append(samples, store.TrafficSample{PeerID: id, Bucket: time.Unix(bucket, 0), Rx: v[0], Tx: v[1]})
		}
	}
	c.dirty = map[string]bool{}
	c.pendingBuckets = map[string]map[int64]*[2]int64{}
	c.mu.Unlock()
	if err := c.e.st.FlushCounters(ctx, counters, samples); err != nil {
		return err
	}
	// Keep the in-memory peer records' totals current so a later UpdatePeer
	// does not carry stale numbers around (they are not written by it, but
	// the API reads them).
	c.e.mu.Lock()
	for _, k := range counters {
		if p, ok := c.e.peers[k.ID]; ok {
			p.RxTotal, p.TxTotal, p.LastHandshake, p.LastEndpoint = k.RxTotal, k.TxTotal, k.LastHandshake, k.LastEndpoint
		}
	}
	c.e.mu.Unlock()
	return nil
}
