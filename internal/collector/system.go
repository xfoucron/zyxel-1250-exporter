package collector

import (
	"context"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/xfoucron/zyxel-1250-exporter/internal/zyxel"
)

var (
	systemInfoDesc = desc("system", "info", "Constant 1, labelled with the identity of the switch.",
		"model", "name", "firmware", "build_date", "mac", "ip", "gateway")
	systemUptimeDesc = desc("system", "uptime_seconds", "Time since the switch booted.")
	systemLoopDesc   = desc("system", "loop_detected", "Whether the switch detected a network loop.")
	systemDHCPDesc   = desc("system", "dhcp_enabled", "Whether the management address is obtained by DHCP.")
	lagActiveDesc    = desc("lag", "active", "Whether the link aggregation group is configured.", "lag")
)

func collectSystem(ctx context.Context, c *zyxel.Client, ch chan<- prometheus.Metric) error {
	s, err := zyxel.Get[zyxel.SystemData](ctx, c, "home_systemData")
	if err != nil {
		return err
	}
	ch <- prometheus.MustNewConstMetric(systemInfoDesc, prometheus.GaugeValue, 1,
		s.ModelName, s.DeviceName, s.Firmware, s.BuildDate, s.MAC, s.IP, s.Gateway)
	ch <- prometheus.MustNewConstMetric(systemUptimeDesc, prometheus.GaugeValue, float64(s.Uptime))
	ch <- prometheus.MustNewConstMetric(systemLoopDesc, prometheus.GaugeValue, boolValue(loopDetected(s)))
	ch <- prometheus.MustNewConstMetric(systemDHCPDesc, prometheus.GaugeValue, boolValue(s.DHCP != 0))
	for i, active := range s.Trunks() {
		ch <- prometheus.MustNewConstMetric(lagActiveDesc, prometheus.GaugeValue, boolValue(active), strconv.Itoa(i+1))
	}
	return nil
}

func loopDetected(s zyxel.SystemData) bool {
	if s.Loop != "" && !strings.EqualFold(s.Loop, "normal") {
		return true
	}
	for _, st := range s.LoopStatus {
		if st != "" && !strings.EqualFold(st, "normal") {
			return true
		}
	}
	return false
}
