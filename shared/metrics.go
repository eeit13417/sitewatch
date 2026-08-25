package shared

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

// PostgresPoolCollector exports pgxpool's live stats on every Prometheus
// scrape instead of a periodically-updated gauge that goes stale between
// updates. A struct wrapping the pool and implementing prometheus.Collector
// is the right shape here — real state (the pool reference), exposed
// through the interface Prometheus's client library expects, which is
// how Go does this kind of "object" rather than through inheritance.
type PostgresPoolCollector struct {
	pool      *pgxpool.Pool
	subsystem string // e.g. "api" or "ingestion" — keeps the two services' pool metrics distinct

	acquired  *prometheus.Desc
	idle      *prometheus.Desc
	total     *prometheus.Desc
	maxConns  *prometheus.Desc
	newConns  *prometheus.Desc
	acquireCt *prometheus.Desc
}

// NewPostgresPoolCollector builds a collector for one service's pool.
// Register it once: prometheus.MustRegister(shared.NewPostgresPoolCollector(pool, "api")).
func NewPostgresPoolCollector(pool *pgxpool.Pool, subsystem string) *PostgresPoolCollector {
	labels := prometheus.Labels{"service": subsystem}
	desc := func(name, help string) *prometheus.Desc {
		return prometheus.NewDesc("sitewatch_postgres_pool_"+name, help, nil, labels)
	}
	return &PostgresPoolCollector{
		pool:      pool,
		subsystem: subsystem,
		acquired:  desc("acquired_conns", "Connections currently checked out by the application"),
		idle:      desc("idle_conns", "Connections idle in the pool"),
		total:     desc("total_conns", "Total connections currently open (acquired + idle)"),
		maxConns:  desc("max_conns", "Configured maximum pool size"),
		newConns:  desc("new_conns_total", "Cumulative connections established since pool creation"),
		acquireCt: desc("acquires_total", "Cumulative successful Acquire() calls"),
	}
}

func (c *PostgresPoolCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.acquired
	ch <- c.idle
	ch <- c.total
	ch <- c.maxConns
	ch <- c.newConns
	ch <- c.acquireCt
}

func (c *PostgresPoolCollector) Collect(ch chan<- prometheus.Metric) {
	stat := c.pool.Stat()
	ch <- prometheus.MustNewConstMetric(c.acquired, prometheus.GaugeValue, float64(stat.AcquiredConns()))
	ch <- prometheus.MustNewConstMetric(c.idle, prometheus.GaugeValue, float64(stat.IdleConns()))
	ch <- prometheus.MustNewConstMetric(c.total, prometheus.GaugeValue, float64(stat.TotalConns()))
	ch <- prometheus.MustNewConstMetric(c.maxConns, prometheus.GaugeValue, float64(stat.MaxConns()))
	ch <- prometheus.MustNewConstMetric(c.newConns, prometheus.CounterValue, float64(stat.NewConnsCount()))
	ch <- prometheus.MustNewConstMetric(c.acquireCt, prometheus.CounterValue, float64(stat.AcquireCount()))
}
