package config

import (
	"bytes"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"text/template"
)

type NFTablesConfig struct {
	TableFamily string
	TableName   string
	Allowed4Set string

	WANInterface string
	LANInterface string

	PortalPort uint16
	ClientCIDR netip.Prefix
}

func RenderNFTables(cfg NFTablesConfig) (string, error) {
	if cfg.TableFamily == "" {
		cfg.TableFamily = "inet"
	}
	if cfg.TableName == "" {
		cfg.TableName = "pisowifi"
	}
	if cfg.Allowed4Set == "" {
		cfg.Allowed4Set = "allowed4"
	}
	if cfg.WANInterface == "" || cfg.LANInterface == "" {
		return "", errors.New("WANInterface and LANInterface are required")
	}
	if cfg.PortalPort == 0 {
		return "", errors.New("PortalPort is required")
	}
	if !cfg.ClientCIDR.IsValid() {
		return "", errors.New("ClientCIDR is required")
	}

	t := template.Must(template.New("nft").Parse(nftablesTemplate))
	var buf bytes.Buffer
	if err := t.Execute(&buf, cfg); err != nil {
		return "", err
	}
	out := strings.TrimSpace(buf.String()) + "\n"
	return out, nil
}

type DHCPRange struct {
	Interface string
	StartIP   netip.Addr
	EndIP     netip.Addr
	Lease     string
	RouterIP  netip.Addr
	DNSIP     netip.Addr
	Domain    string
}

type DNSMasqConfig struct {
	Ranges []DHCPRange
}

func RenderDNSMasq(cfg DNSMasqConfig) (string, error) {
	if len(cfg.Ranges) == 0 {
		return "", errors.New("at least one DHCP range is required")
	}
	t := template.Must(template.New("dnsmasq").Funcs(template.FuncMap{
		"addr": func(a netip.Addr) string {
			return a.String()
		},
	}).Parse(dnsmasqTemplate))
	var buf bytes.Buffer
	if err := t.Execute(&buf, cfg); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()) + "\n", nil
}

type SSIDConfig struct {
	Name     string
	VLANID   int
	Bridge   string
	Password string
}

type HostapdConfig struct {
	RadioInterface string
	CountryCode    string
	Channel        int
	SSIDs          []SSIDConfig
}

func RenderHostapdMultiBSS(cfg HostapdConfig) (string, error) {
	if cfg.RadioInterface == "" {
		return "", errors.New("RadioInterface is required")
	}
	if len(cfg.SSIDs) == 0 {
		return "", errors.New("at least one SSID is required")
	}
	for i, s := range cfg.SSIDs {
		if strings.TrimSpace(s.Name) == "" {
			return "", fmt.Errorf("SSIDs[%d].Name is required", i)
		}
		if strings.TrimSpace(s.Bridge) == "" {
			return "", fmt.Errorf("SSIDs[%d].Bridge is required", i)
		}
		if s.Password != "" && len(s.Password) < 8 {
			return "", fmt.Errorf("SSIDs[%d].Password must be at least 8 characters", i)
		}
	}
	if cfg.CountryCode == "" {
		cfg.CountryCode = "US"
	}
	if cfg.Channel == 0 {
		cfg.Channel = 6
	}

	t := template.Must(template.New("hostapd").Parse(hostapdMultiBSSTemplate))
	var buf bytes.Buffer
	if err := t.Execute(&buf, cfg); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()) + "\n", nil
}

const nftablesTemplate = `
flush ruleset

table {{.TableFamily}} {{.TableName}} {
  set {{.Allowed4Set}} {
    type ipv4_addr
    flags timeout
  }

  chain input {
    type filter hook input priority filter; policy drop;
    ct state established,related accept
    iifname "lo" accept

    iifname "{{.WANInterface}}" tcp dport 22 accept

    iifname "{{.LANInterface}}" udp dport { 53, 67, 68 } accept
    iifname "{{.LANInterface}}" tcp dport {{.PortalPort}} accept
  }

  chain prerouting {
    type nat hook prerouting priority dstnat; policy accept;
    iifname "{{.LANInterface}}" ip saddr {{.ClientCIDR}} ip saddr != @{{.Allowed4Set}} tcp dport 80 redirect to :{{.PortalPort}}
  }

  chain forward {
    type filter hook forward priority filter; policy drop;
    ct state established,related accept

    iifname "{{.LANInterface}}" oifname "{{.WANInterface}}" ip saddr {{.ClientCIDR}} ip saddr @{{.Allowed4Set}} accept
  }

  chain postrouting {
    type nat hook postrouting priority srcnat; policy accept;
    oifname "{{.WANInterface}}" masquerade
  }
}
`

const dnsmasqTemplate = `
domain-needed
bogus-priv
expand-hosts
no-resolv
server=1.1.1.1
server=8.8.8.8

{{range .Ranges}}
interface={{.Interface}}
dhcp-range={{.Interface}},{{addr .StartIP}},{{addr .EndIP}},{{.Lease}}
{{if .RouterIP.IsValid}}dhcp-option={{.Interface}},3,{{addr .RouterIP}}{{end}}
{{if .DNSIP.IsValid}}dhcp-option={{.Interface}},6,{{addr .DNSIP}}{{end}}
{{if .Domain}}domain={{.Domain}}{{end}}
{{end}}
`

const hostapdMultiBSSTemplate = `
driver=nl80211
interface={{.RadioInterface}}
country_code={{.CountryCode}}
hw_mode=g
channel={{.Channel}}
wmm_enabled=1
auth_algs=1
ignore_broadcast_ssid=0

{{- range $i, $s := .SSIDs}}
{{if eq $i 0}}
ssid={{$s.Name}}
bridge={{$s.Bridge}}
{{else}}
bss={{$.RadioInterface}}
ssid={{$s.Name}}
bridge={{$s.Bridge}}
{{end}}
{{if $s.Password}}
wpa=2
wpa_key_mgmt=WPA-PSK
rsn_pairwise=CCMP
wpa_passphrase={{$s.Password}}
{{end}}

{{- end}}
`
