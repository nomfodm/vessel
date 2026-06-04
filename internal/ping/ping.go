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
// failure. (ctx is accepted for the Wails binding; minequery uses its own timeout.)
func (s *Service) Ping(ctx context.Context, host string, port int) Status {
	res, err := s.pinger.Ping17(host, port)
	if err != nil {
		s.log.Debug("ping failed", "host", host, "port", port, "err", err)
		return Status{Online: false}
	}
	return Status{
		Online:        true,
		PlayersOnline: res.OnlinePlayers,
		PlayersMax:    res.MaxPlayers,
		Version:       res.VersionName,
	}
}
