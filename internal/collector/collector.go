package collector

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/xfoucron/zyxel-1250-exporter/internal/zyxel"
)

const namespace = "zyxel"

var (
	scrapeSuccessDesc = prometheus.NewDesc(
		namespace+"_scrape_collector_success",
		"Whether the collector succeeded during the last scrape.",
		[]string{"collector"}, nil,
	)
	scrapeDurationDesc = prometheus.NewDesc(
		namespace+"_scrape_collector_duration_seconds",
		"Duration of the collector during the last scrape.",
		[]string{"collector"}, nil,
	)
)

type subCollector struct {
	name    string
	collect func(ctx context.Context, c *zyxel.Client, ch chan<- prometheus.Metric) error
}

var subCollectors = []subCollector{
	{"system", collectSystem},
	{"port", collectPort},
	{"vlan", collectVLAN},
}

type Collector struct {
	client  *zyxel.Client
	timeout time.Duration
	log     *slog.Logger
}

func New(client *zyxel.Client, timeout time.Duration, log *slog.Logger) *Collector {
	return &Collector{client: client, timeout: timeout, log: log}
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(c, ch)
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(len(subCollectors))
	for _, sub := range subCollectors {
		go func() {
			defer wg.Done()
			start := time.Now()
			err := sub.collect(ctx, c.client, ch)
			if err != nil {
				c.log.Error("collector failed", "collector", sub.name, "err", err)
			}
			ch <- prometheus.MustNewConstMetric(scrapeSuccessDesc, prometheus.GaugeValue, boolValue(err == nil), sub.name)
			ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(start).Seconds(), sub.name)
		}()
	}
	wg.Wait()
}

func desc(subsystem, name, help string, labels ...string) *prometheus.Desc {
	return prometheus.NewDesc(prometheus.BuildFQName(namespace, subsystem, name), help, labels, nil)
}

func boolValue(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
