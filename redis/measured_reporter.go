package redis

import (
	"fmt"
	"net"

	"gopkg.in/redis.v3"

	"github.com/getlantern/measured"
)

type measuredReporter struct {
	redisClient *redis.Client
}

func NewMeasuredReporter(redisAddr string) (measured.Reporter, error) {
	rc, err := getClient(redisAddr)
	if err != nil {
		return nil, err
	}
	return &measuredReporter{rc}, nil
}

func (rp *measuredReporter) ReportError(s map[*measured.Error]int) error {
	return nil
}
func (rp *measuredReporter) ReportLatency(s []*measured.LatencyTracker) error {
	return nil
}
func (rp *measuredReporter) ReportTraffic(tt []*measured.TrafficTracker) error {
	for _, t := range tt {
		key := t.ID
		if key == "" {
			panic("empty key is not allowed")
		}

		// Don't report IDs in the form ip:port, so no connection that isn't
		// associated to a request that passes through devicefilter gets reported
		if _, _, err := net.SplitHostPort(key); err == nil {
			continue
		}

		tx := rp.redisClient.Multi()
		defer tx.Close()

		_, err := tx.Exec(func() error {
			err := tx.HIncrBy("client:"+string(key), "bytesIn", int64(t.LastIn)).Err()
			if err != nil {
				return err
			}
			err = tx.HIncrBy("client:"+string(key), "bytesOut", int64(t.LastOut)).Err()
			if err != nil {
				return err
			}
			// An auxiliary ordered set for aggregated bytesIn+bytesOut
			// Redis stores scores as float64
			err = tx.ZAdd("client->bytes",
				redis.Z{
					float64(t.TotalIn + t.TotalOut),
					key,
				}).Err()
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("Error in MULTI command: %v\n", err)
		}
	}
	return nil
}
