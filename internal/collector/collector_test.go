package collector_test

import (
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/xfoucron/zyxel-1250-exporter/internal/collector"
	"github.com/xfoucron/zyxel-1250-exporter/internal/zyxel"
	"github.com/xfoucron/zyxel-1250-exporter/internal/zyxel/zyxeltest"
)

var fixtures = map[string]string{
	"home_systemData": `{"max_port":4,"model_name":"XGS1250-12","sys_dev_name":"lab-switch","sys_fmw_ver":"V1.00",
		"sys_bld_date":"Jan 01 2024","sys_MAC":"AA:BB:CC:DD:EE:FF","sys_IP":"192.0.2.10","sys_gateway":"192.0.2.1",
		"sys_dhcp_state":"0","sys_uptime":3600,"loop":"Normal","loop_status":["Normal","Normal","Normal","Normal","Normal"],
		"isTrunk0":1,"isTrunk1":0,"isTrunk2":0,"isTrunk3":0,"isTrunk4":0}`,
	"home_linkData": `{"portstatus":["Up","Down","Up","Down","Down"],
		"speed":["10G-Full","auto","2.5G-Full","auto","auto"],
		"pvids":[1,1,20,1,1],
		"stats":[[100,200,3],[0,0,0],[7,8,0],[0,0,0],[9,9,9]],
		"overheat":[0,0,1,0]}`,
	"port_portInfo": `{"portState":13,"isCopper":[1,1,1,0]}`,
	"vlan_vlanList": `{"qvlans":[[1,"0x1f","0x0"],[20,"0x5","0x4"]]}`,
}

func newCollector(t *testing.T, results map[string]string) *collector.Collector {
	t.Helper()
	srv := zyxeltest.NewServer(t, results)
	client, err := zyxel.New(srv.URL, zyxeltest.Password, nil)
	if err != nil {
		t.Fatalf("zyxel.New: %v", err)
	}
	return collector.New(client, 5*time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestCollect(t *testing.T) {
	expected := `
# HELP zyxel_system_info Constant 1, labelled with the identity of the switch.
# TYPE zyxel_system_info gauge
zyxel_system_info{build_date="Jan 01 2024",firmware="V1.00",gateway="192.0.2.1",ip="192.0.2.10",mac="AA:BB:CC:DD:EE:FF",model="XGS1250-12",name="lab-switch"} 1
# HELP zyxel_system_uptime_seconds Time since the switch booted.
# TYPE zyxel_system_uptime_seconds gauge
zyxel_system_uptime_seconds 3600
# HELP zyxel_system_loop_detected Whether the switch detected a network loop.
# TYPE zyxel_system_loop_detected gauge
zyxel_system_loop_detected 0
# HELP zyxel_lag_active Whether the link aggregation group is configured.
# TYPE zyxel_lag_active gauge
zyxel_lag_active{lag="1"} 1
zyxel_lag_active{lag="2"} 0
zyxel_lag_active{lag="3"} 0
zyxel_lag_active{lag="4"} 0
zyxel_lag_active{lag="5"} 0
# HELP zyxel_port_up Whether the link of the port is up.
# TYPE zyxel_port_up gauge
zyxel_port_up{port="1"} 1
zyxel_port_up{port="2"} 0
zyxel_port_up{port="3"} 1
zyxel_port_up{port="4"} 0
# HELP zyxel_port_enabled Whether the port is administratively enabled.
# TYPE zyxel_port_enabled gauge
zyxel_port_enabled{port="1"} 1
zyxel_port_enabled{port="2"} 0
zyxel_port_enabled{port="3"} 1
zyxel_port_enabled{port="4"} 1
# HELP zyxel_port_speed_bits_per_second Negotiated link speed, 0 when the link is down.
# TYPE zyxel_port_speed_bits_per_second gauge
zyxel_port_speed_bits_per_second{port="1"} 1e+10
zyxel_port_speed_bits_per_second{port="2"} 0
zyxel_port_speed_bits_per_second{port="3"} 2.5e+09
zyxel_port_speed_bits_per_second{port="4"} 0
# HELP zyxel_port_received_packets_total Packets received by the port.
# TYPE zyxel_port_received_packets_total counter
zyxel_port_received_packets_total{port="1"} 100
zyxel_port_received_packets_total{port="2"} 0
zyxel_port_received_packets_total{port="3"} 7
zyxel_port_received_packets_total{port="4"} 0
# HELP zyxel_port_transmitted_packets_total Packets transmitted by the port.
# TYPE zyxel_port_transmitted_packets_total counter
zyxel_port_transmitted_packets_total{port="1"} 200
zyxel_port_transmitted_packets_total{port="2"} 0
zyxel_port_transmitted_packets_total{port="3"} 8
zyxel_port_transmitted_packets_total{port="4"} 0
# HELP zyxel_port_crc_error_packets_total Packets received with a CRC error.
# TYPE zyxel_port_crc_error_packets_total counter
zyxel_port_crc_error_packets_total{port="1"} 3
zyxel_port_crc_error_packets_total{port="2"} 0
zyxel_port_crc_error_packets_total{port="3"} 0
zyxel_port_crc_error_packets_total{port="4"} 0
# HELP zyxel_port_overheat Whether the port was shut down by overheat protection.
# TYPE zyxel_port_overheat gauge
zyxel_port_overheat{port="1"} 0
zyxel_port_overheat{port="2"} 0
zyxel_port_overheat{port="3"} 1
zyxel_port_overheat{port="4"} 0
# HELP zyxel_port_info Constant 1, labelled with the media of the port.
# TYPE zyxel_port_info gauge
zyxel_port_info{media="copper",port="1"} 1
zyxel_port_info{media="copper",port="2"} 1
zyxel_port_info{media="copper",port="3"} 1
zyxel_port_info{media="fiber",port="4"} 1
# HELP zyxel_vlan_count Number of VLANs configured on the switch.
# TYPE zyxel_vlan_count gauge
zyxel_vlan_count 2
# HELP zyxel_vlan_member_ports Number of physical ports that are members of the VLAN.
# TYPE zyxel_vlan_member_ports gauge
zyxel_vlan_member_ports{vlan="1"} 4
zyxel_vlan_member_ports{vlan="20"} 2
# HELP zyxel_vlan_tagged_ports Number of physical ports that send the VLAN tagged.
# TYPE zyxel_vlan_tagged_ports gauge
zyxel_vlan_tagged_ports{vlan="1"} 0
zyxel_vlan_tagged_ports{vlan="20"} 1
# HELP zyxel_vlan_port_member Whether the port is a member of the VLAN (1 untagged, 2 tagged, 0 not a member).
# TYPE zyxel_vlan_port_member gauge
zyxel_vlan_port_member{port="1",vlan="20"} 1
zyxel_vlan_port_member{port="2",vlan="20"} 0
zyxel_vlan_port_member{port="3",vlan="20"} 2
zyxel_vlan_port_member{port="4",vlan="20"} 0
# HELP zyxel_scrape_collector_success Whether the collector succeeded during the last scrape.
# TYPE zyxel_scrape_collector_success gauge
zyxel_scrape_collector_success{collector="port"} 1
zyxel_scrape_collector_success{collector="system"} 1
zyxel_scrape_collector_success{collector="vlan"} 1
`
	err := testutil.CollectAndCompare(newCollector(t, fixtures), strings.NewReader(expected),
		"zyxel_system_info",
		"zyxel_system_uptime_seconds",
		"zyxel_system_loop_detected",
		"zyxel_lag_active",
		"zyxel_port_up",
		"zyxel_port_enabled",
		"zyxel_port_speed_bits_per_second",
		"zyxel_port_received_packets_total",
		"zyxel_port_transmitted_packets_total",
		"zyxel_port_crc_error_packets_total",
		"zyxel_port_overheat",
		"zyxel_port_info",
		"zyxel_vlan_count",
		"zyxel_vlan_member_ports",
		"zyxel_vlan_tagged_ports",
		"zyxel_vlan_port_member{vlan=\"20\"}",
		"zyxel_scrape_collector_success",
	)
	if err != nil {
		t.Error(err)
	}
}

func TestCollectReportsFailingCollector(t *testing.T) {
	partial := map[string]string{"home_systemData": fixtures["home_systemData"]}
	expected := `
# HELP zyxel_scrape_collector_success Whether the collector succeeded during the last scrape.
# TYPE zyxel_scrape_collector_success gauge
zyxel_scrape_collector_success{collector="port"} 0
zyxel_scrape_collector_success{collector="system"} 1
zyxel_scrape_collector_success{collector="vlan"} 0
`
	err := testutil.CollectAndCompare(newCollector(t, partial), strings.NewReader(expected), "zyxel_scrape_collector_success")
	if err != nil {
		t.Error(err)
	}
}

func TestLoopDetected(t *testing.T) {
	looped := map[string]string{"home_systemData": strings.Replace(fixtures["home_systemData"], `"loop":"Normal"`, `"loop":"Loop"`, 1)}
	expected := `
# HELP zyxel_system_loop_detected Whether the switch detected a network loop.
# TYPE zyxel_system_loop_detected gauge
zyxel_system_loop_detected 1
`
	if err := testutil.CollectAndCompare(newCollector(t, looped), strings.NewReader(expected), "zyxel_system_loop_detected"); err != nil {
		t.Error(err)
	}
}
