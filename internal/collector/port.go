package collector

import (
	"context"
	"strconv"
	"strings"
	"unicode"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/xfoucron/zyxel-1250-exporter/internal/zyxel"
)

var (
	portInfoDesc     = desc("port", "info", "Constant 1, labelled with the media of the port.", "port", "media")
	portUpDesc       = desc("port", "up", "Whether the link of the port is up.", "port")
	portEnabledDesc  = desc("port", "enabled", "Whether the port is administratively enabled.", "port")
	portSpeedDesc    = desc("port", "speed_bits_per_second", "Negotiated link speed, 0 when the link is down.", "port")
	portPVIDDesc     = desc("port", "pvid", "Port VLAN ID (untagged traffic).", "port")
	portOverheatDesc = desc("port", "overheat", "Whether the port was shut down by overheat protection.", "port")
	portRxDesc       = desc("port", "received_packets_total", "Packets received by the port.", "port")
	portTxDesc       = desc("port", "transmitted_packets_total", "Packets transmitted by the port.", "port")
	portCRCDesc      = desc("port", "crc_error_packets_total", "Packets received with a CRC error.", "port")
)

func collectPort(ctx context.Context, c *zyxel.Client, ch chan<- prometheus.Metric) error {
	system, err := zyxel.Get[zyxel.SystemData](ctx, c, "home_systemData")
	if err != nil {
		return err
	}
	link, err := zyxel.Get[zyxel.LinkData](ctx, c, "home_linkData")
	if err != nil {
		return err
	}
	info, err := zyxel.Get[zyxel.PortInfo](ctx, c, "port_portInfo")
	if err != nil {
		return err
	}

	for i := range min(system.MaxPort, len(link.PortStatus)) {
		port := strconv.Itoa(i + 1)
		up := strings.EqualFold(link.PortStatus[i], "up")

		if i < len(info.IsCopper) {
			media := "fiber"
			if info.IsCopper[i] != 0 {
				media = "copper"
			}
			ch <- prometheus.MustNewConstMetric(portInfoDesc, prometheus.GaugeValue, 1, port, media)
		}
		ch <- prometheus.MustNewConstMetric(portUpDesc, prometheus.GaugeValue, boolValue(up), port)
		ch <- prometheus.MustNewConstMetric(portEnabledDesc, prometheus.GaugeValue, boolValue(info.PortState>>i&1 == 1), port)

		speed := 0.0
		if up && i < len(link.Speed) {
			speed = parseSpeed(link.Speed[i])
		}
		ch <- prometheus.MustNewConstMetric(portSpeedDesc, prometheus.GaugeValue, speed, port)

		if i < len(link.PVIDs) {
			ch <- prometheus.MustNewConstMetric(portPVIDDesc, prometheus.GaugeValue, float64(link.PVIDs[i]), port)
		}
		if i < len(link.Overheat) {
			ch <- prometheus.MustNewConstMetric(portOverheatDesc, prometheus.GaugeValue, boolValue(link.Overheat[i] != 0), port)
		}
		if i < len(link.Stats) {
			ch <- prometheus.MustNewConstMetric(portRxDesc, prometheus.CounterValue, float64(link.Stats[i][0]), port)
			ch <- prometheus.MustNewConstMetric(portTxDesc, prometheus.CounterValue, float64(link.Stats[i][1]), port)
			ch <- prometheus.MustNewConstMetric(portCRCDesc, prometheus.CounterValue, float64(link.Stats[i][2]), port)
		}
	}
	return nil
}

func parseSpeed(s string) float64 {
	s = strings.TrimSpace(s)
	end := strings.IndexFunc(s, func(r rune) bool { return !unicode.IsDigit(r) && r != '.' })
	if end <= 0 {
		return 0
	}
	n, err := strconv.ParseFloat(s[:end], 64)
	if err != nil {
		return 0
	}
	switch unit := strings.ToUpper(strings.TrimLeft(s[end:], " ")); {
	case strings.HasPrefix(unit, "G"):
		return n * 1e9
	case strings.HasPrefix(unit, "M"):
		return n * 1e6
	case strings.HasPrefix(unit, "K"):
		return n * 1e3
	}
	return 0
}
