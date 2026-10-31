package collector

import (
	"context"
	"math/bits"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/xfoucron/zyxel-1250-exporter/internal/zyxel"
)

var (
	vlansDesc          = desc("vlan", "count", "Number of VLANs configured on the switch.")
	vlanMembersDesc    = desc("vlan", "member_ports", "Number of physical ports that are members of the VLAN.", "vlan")
	vlanTaggedDesc     = desc("vlan", "tagged_ports", "Number of physical ports that send the VLAN tagged.", "vlan")
	vlanPortMemberDesc = desc("vlan", "port_member", "Whether the port is a member of the VLAN (1 untagged, 2 tagged, 0 not a member).", "vlan", "port")
)

func collectVLAN(ctx context.Context, c *zyxel.Client, ch chan<- prometheus.Metric) error {
	system, err := zyxel.Get[zyxel.SystemData](ctx, c, "home_systemData")
	if err != nil {
		return err
	}
	list, err := zyxel.Get[zyxel.VLANList](ctx, c, "vlan_vlanList")
	if err != nil {
		return err
	}

	ch <- prometheus.MustNewConstMetric(vlansDesc, prometheus.GaugeValue, float64(len(list.QVLANs)))
	ports := min(system.MaxPort, 63)
	physical := uint64(1)<<ports - 1
	for _, v := range list.QVLANs {
		id := strconv.Itoa(v.ID)
		members, tagged := v.Members&physical, v.Tagged&physical
		ch <- prometheus.MustNewConstMetric(vlanMembersDesc, prometheus.GaugeValue, float64(bits.OnesCount64(members)), id)
		ch <- prometheus.MustNewConstMetric(vlanTaggedDesc, prometheus.GaugeValue, float64(bits.OnesCount64(members&tagged)), id)
		for p := range ports {
			state := 0.0
			switch {
			case members>>p&1 == 0:
			case tagged>>p&1 == 1:
				state = 2
			default:
				state = 1
			}
			ch <- prometheus.MustNewConstMetric(vlanPortMemberDesc, prometheus.GaugeValue, state, id, strconv.Itoa(p+1))
		}
	}
	return nil
}
