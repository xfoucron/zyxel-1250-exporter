package zyxel_test

import (
	"context"
	"errors"
	"testing"

	"github.com/xfoucron/zyxel-1250-exporter/internal/zyxel"
	"github.com/xfoucron/zyxel-1250-exporter/internal/zyxel/zyxeltest"
)

var results = map[string]string{
	"home_systemData": `{"max_port":12,"sys_dev_name":"sw","sys_uptime":42,"sys_dhcp_state":"1","isTrunk0":0}`,
	"vlan_vlanList":   `{"qvlans":[[1,"0xfff","0x0"],[20,"0x00c","0x004"]]}`,
}

func newClient(t *testing.T, password string) (*zyxel.Client, *zyxeltest.Server) {
	t.Helper()
	srv := zyxeltest.NewServer(t, results)
	c, err := zyxel.New(srv.URL, password, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c, srv
}

func TestGetLogsInAndReusesSession(t *testing.T) {
	c, srv := newClient(t, zyxeltest.Password)
	for range 3 {
		sys, err := zyxel.Get[zyxel.SystemData](context.Background(), c, "home_systemData")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if sys.Uptime != 42 || sys.DHCP != 1 || sys.DeviceName != "sw" {
			t.Fatalf("unexpected data: %+v", sys)
		}
	}
	if got := srv.Logins(); got != 1 {
		t.Errorf("logins = %d, want 1", got)
	}
}

func TestGetLogsInAgainWhenSessionExpires(t *testing.T) {
	c, srv := newClient(t, zyxeltest.Password)
	if _, err := zyxel.Get[zyxel.SystemData](context.Background(), c, "home_systemData"); err != nil {
		t.Fatal(err)
	}
	srv.RevokeSessions()
	if _, err := zyxel.Get[zyxel.SystemData](context.Background(), c, "home_systemData"); err != nil {
		t.Fatalf("Get after expiry: %v", err)
	}
	if got := srv.Logins(); got != 2 {
		t.Errorf("logins = %d, want 2", got)
	}
}

func TestWrongPassword(t *testing.T) {
	c, _ := newClient(t, "nope")
	_, err := zyxel.Get[zyxel.SystemData](context.Background(), c, "home_systemData")
	if !errors.Is(err, zyxel.ErrAuth) {
		t.Fatalf("err = %v, want ErrAuth", err)
	}
}

func TestUnknownCommand(t *testing.T) {
	c, _ := newClient(t, zyxeltest.Password)
	if _, err := zyxel.Get[zyxel.SystemData](context.Background(), c, "nope"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestLogout(t *testing.T) {
	c, srv := newClient(t, zyxeltest.Password)
	if _, err := zyxel.Get[zyxel.SystemData](context.Background(), c, "home_systemData"); err != nil {
		t.Fatal(err)
	}
	if err := c.Logout(context.Background()); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if _, err := zyxel.Get[zyxel.SystemData](context.Background(), c, "home_systemData"); err != nil {
		t.Fatal(err)
	}
	if got := srv.Logins(); got != 2 {
		t.Errorf("logins = %d, want 2", got)
	}
}

func TestVLANMasks(t *testing.T) {
	c, _ := newClient(t, zyxeltest.Password)
	list, err := zyxel.Get[zyxel.VLANList](context.Background(), c, "vlan_vlanList")
	if err != nil {
		t.Fatal(err)
	}
	want := []zyxel.VLAN{{ID: 1, Members: 0xfff}, {ID: 20, Members: 0xc, Tagged: 0x4}}
	if len(list.QVLANs) != len(want) || list.QVLANs[0] != want[0] || list.QVLANs[1] != want[1] {
		t.Fatalf("qvlans = %+v, want %+v", list.QVLANs, want)
	}
}

func TestNewValidation(t *testing.T) {
	for _, tc := range []struct{ url, pass string }{{"", "x"}, {"ftp://h", "x"}, {"http://h", ""}} {
		if _, err := zyxel.New(tc.url, tc.pass, nil); err == nil {
			t.Errorf("New(%q, %q): expected an error", tc.url, tc.pass)
		}
	}
}
