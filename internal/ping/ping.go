// Package ping wraps minequery's Server List Ping so the launcher can show live
// online/max players on profile cards.
package ping

import (
	"context"
	"log/slog"
	"time"

	"github.com/dreamscached/minequery/v2"
)

type Status struct {
	Online        bool   `json:"online"`
	PlayersOnline int    `json:"playersOnline"`
	PlayersMax    int    `json:"playersMax"`
	Version       string `json:"version"`
}

type Service struct {
	pinger *minequery.Pinger
	log    *slog.Logger
}

func New(log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	pinger := minequery.NewPinger(
		minequery.WithTimeout(5*time.Second),
		minequery.WithPreferSRVRecord(true), // honour _minecraft._tcp SRV like real clients
	)
	return &Service{pinger: pinger, log: log.With("svc", "ping")}
}

// Ping returns the server status. A down or unreachable server yields
// Status{Online:false} without an error — a normal state the UI renders, not a
// failure. ctx cancellation is honoured: if the caller cancels before minequery's
// own timeout fires, Ping returns immediately (the underlying goroutine still
// finishes within the configured 5 s timeout).
func (s *Service) Ping(ctx context.Context, host string, port int) Status {
	type result struct{ st Status }
	ch := make(chan result, 1)
	go func() {
		res, err := s.pinger.Ping17(host, port)
		if err != nil {
			s.log.Debug("ping failed", "host", host, "port", port, "err", err)
			ch <- result{Status{Online: false}}
			return
		}
		ch <- result{Status{
			Online:        true,
			PlayersOnline: res.OnlinePlayers,
			PlayersMax:    res.MaxPlayers,
			Version:       res.VersionName,
		}}
	}()
	select {
	case r := <-ch:
		return r.st
	case <-ctx.Done():
		return Status{Online: false}
	}
}
