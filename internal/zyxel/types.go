package zyxel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

type Int int64

func (i *Int) UnmarshalJSON(data []byte) error {
	data = bytes.Trim(bytes.TrimSpace(data), `"`)
	if len(data) == 0 {
		*i = 0
		return nil
	}
	v, err := strconv.ParseInt(string(data), 0, 64)
	if err != nil {
		return fmt.Errorf("decode integer %q: %w", data, err)
	}
	*i = Int(v)
	return nil
}

type SystemData struct {
	MaxPort    int      `json:"max_port"`
	ModelName  string   `json:"model_name"`
	DeviceName string   `json:"sys_dev_name"`
	Firmware   string   `json:"sys_fmw_ver"`
	BuildDate  string   `json:"sys_bld_date"`
	MAC        string   `json:"sys_MAC"`
	IP         string   `json:"sys_IP"`
	Netmask    string   `json:"sys_sbnt_msk"`
	Gateway    string   `json:"sys_gateway"`
	DHCP       Int      `json:"sys_dhcp_state"`
	Uptime     Int      `json:"sys_uptime"`
	Loop       string   `json:"loop"`
	LoopStatus []string `json:"loop_status"`
	Trunk0     Int      `json:"isTrunk0"`
	Trunk1     Int      `json:"isTrunk1"`
	Trunk2     Int      `json:"isTrunk2"`
	Trunk3     Int      `json:"isTrunk3"`
	Trunk4     Int      `json:"isTrunk4"`
}

func (s SystemData) Trunks() []bool {
	return []bool{s.Trunk0 != 0, s.Trunk1 != 0, s.Trunk2 != 0, s.Trunk3 != 0, s.Trunk4 != 0}
}

type LinkData struct {
	PortStatus []string `json:"portstatus"`
	Speed      []string `json:"speed"`
	PVIDs      []Int    `json:"pvids"`
	Stats      [][3]Int `json:"stats"`
	Overheat   []Int    `json:"overheat"`
}

type PortInfo struct {
	PortState Int   `json:"portState"`
	IsCopper  []Int `json:"isCopper"`
}

type VLANList struct {
	QVLANs []VLAN `json:"qvlans"`
}

type VLAN struct {
	ID      int
	Members uint64
	Tagged  uint64
}

func (v *VLAN) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw) < 3 {
		return fmt.Errorf("unexpected vlan entry %s", data)
	}
	var id Int
	if err := json.Unmarshal(raw[0], &id); err != nil {
		return err
	}
	members, err := parseMask(raw[1])
	if err != nil {
		return err
	}
	tagged, err := parseMask(raw[2])
	if err != nil {
		return err
	}
	*v = VLAN{ID: int(id), Members: members, Tagged: tagged}
	return nil
}

func parseMask(raw json.RawMessage) (uint64, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		var n uint64
		if err := json.Unmarshal(raw, &n); err != nil {
			return 0, fmt.Errorf("decode vlan mask %s", raw)
		}
		return n, nil
	}
	v, err := strconv.ParseUint(s, 0, 64)
	if err != nil {
		return 0, fmt.Errorf("decode vlan mask %q: %w", s, err)
	}
	return v, nil
}
