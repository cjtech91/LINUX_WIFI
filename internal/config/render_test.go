package config

import (
	"net/netip"
	"strings"
	"testing"
)

func TestRenderNFTables(t *testing.T) {
	pfx := netip.MustParsePrefix("10.10.0.0/16")
	out, err := RenderNFTables(NFTablesConfig{
		TableFamily:  "inet",
		TableName:    "pisowifi",
		Allowed4Set:  "allowed4",
		WANInterface: "eth0",
		LANInterface: "br0",
		PortalPort:   8080,
		ClientCIDR:   pfx,
	})
	if err != nil {
		t.Fatalf("RenderNFTables: %v", err)
	}
	if !strings.Contains(out, `table inet pisowifi`) {
		t.Fatalf("missing table: %s", out)
	}
	if !strings.Contains(out, `redirect to :8080`) {
		t.Fatalf("missing portal port: %s", out)
	}
	if !strings.Contains(out, `iifname "br0"`) || !strings.Contains(out, `oifname "eth0"`) {
		t.Fatalf("missing interfaces: %s", out)
	}
}

func TestRenderDNSMasq(t *testing.T) {
	out, err := RenderDNSMasq(DNSMasqConfig{
		Ranges: []DHCPRange{
			{
				Interface: "br0.10",
				StartIP:   netip.MustParseAddr("10.10.10.50"),
				EndIP:     netip.MustParseAddr("10.10.10.200"),
				Lease:     "12h",
				RouterIP:  netip.MustParseAddr("10.10.10.1"),
				DNSIP:     netip.MustParseAddr("10.10.10.1"),
			},
		},
	})
	if err != nil {
		t.Fatalf("RenderDNSMasq: %v", err)
	}
	if !strings.Contains(out, "interface=br0.10") {
		t.Fatalf("missing interface: %s", out)
	}
	if !strings.Contains(out, "dhcp-range=br0.10,10.10.10.50,10.10.10.200,12h") {
		t.Fatalf("missing range: %s", out)
	}
	if !strings.Contains(out, "dhcp-option=br0.10,3,10.10.10.1") {
		t.Fatalf("missing router option: %s", out)
	}
}

func TestRenderHostapdMultiBSS(t *testing.T) {
	out, err := RenderHostapdMultiBSS(HostapdConfig{
		RadioInterface: "wlan0",
		CountryCode:    "PH",
		Channel:        6,
		SSIDs: []SSIDConfig{
			{Name: "PISO", Bridge: "br0.10", Password: "password123"},
		},
	})
	if err != nil {
		t.Fatalf("RenderHostapdMultiBSS: %v", err)
	}
	if !strings.Contains(out, "interface=wlan0") {
		t.Fatalf("missing radio: %s", out)
	}
	if !strings.Contains(out, "ssid=PISO") || !strings.Contains(out, "bridge=br0.10") {
		t.Fatalf("missing ssid/bridge: %s", out)
	}
}

