# Deployment (PiSoWiFi) via SSH (MobaXterm)

This project is a lightweight PiSoWiFi-style portal for Linux. The app is a single Go binary (control plane) and relies on standard Linux components for the data plane:

- hostapd (Wi‑Fi AP)
- dnsmasq (DHCP/DNS)
- nftables (captive portal redirect + allowlisting + NAT)
- iproute2 (VLAN/bridge setup)

This guide uses only your **MobaXterm SSH session** to run commands on the device.

## 1) SSH into the device

In MobaXterm, open an SSH session to your device:

- Host: `<DEVICE_IP>`
- User: `root` (or a sudo user)

## 2) Get the source code onto the device (SSH-only)

Recommended (simplest): use `git clone` from inside the SSH session.

```bash
sudo apt update
sudo apt install -y git
cd /opt
sudo git clone https://github.com/cjtech91/LINUX_WIFI.git pisowifi
```

If your code is private, use a private GitHub/GitLab repo and clone using HTTPS with a token, or configure SSH keys.

## 3) Install Go on the device and build (SSH-only)

Install Go:

```bash
sudo apt update
sudo apt install -y golang-go
go version
```

Build and install the binary:

```bash
cd /opt/pisowifi
go test ./...
go vet ./...
go build -o pisowifi ./cmd/pisowifi
sudo install -m 0755 pisowifi /usr/local/bin/pisowifi
```

## 4) Install Linux dependencies (on the device)

Debian/Ubuntu/Armbian/Raspberry Pi OS:

```bash
sudo apt update
sudo apt install -y hostapd dnsmasq nftables iproute2
sudo sysctl -w net.ipv4.ip_forward=1
echo "net.ipv4.ip_forward=1" | sudo tee /etc/sysctl.d/99-ipforward.conf
```

## 5) Network topology (example: bridge + VLAN 10)

Assumptions:

- WAN/uplink interface: `eth0`
- Wi‑Fi radio interface: `wlan0`
- Bridge: `br0`
- Client VLAN: `10` (interface `br0.10`)
- Client subnet: `10.10.10.0/24` (gateway `10.10.10.1`)

Create bridge and VLAN interface:

```bash
sudo ip link add br0 type bridge vlan_filtering 1
sudo ip link set br0 up

sudo ip link set eth0 master br0
sudo ip link set wlan0 master br0

sudo ip link add link br0 name br0.10 type vlan id 10
sudo ip addr add 10.10.10.1/24 dev br0.10
sudo ip link set br0.10 up
```

## 6) Generate configs (nftables + dnsmasq + hostapd)

### nftables (captive portal redirect + allowlist + NAT)

```bash
sudo /usr/local/bin/pisowifi render nft \
  --lan-if br0 \
  --wan-if eth0 \
  --portal-port 8080 \
  --client-cidr 10.10.0.0/16 \
  | sudo tee /etc/nftables.d/pisowifi.nft

echo 'include "/etc/nftables.d/pisowifi.nft"' | sudo tee /etc/nftables.conf
sudo systemctl enable --now nftables
```

### dnsmasq (DHCP/DNS for VLAN 10)

```bash
sudo /usr/local/bin/pisowifi render dnsmasq \
  --if br0.10 \
  --start 10.10.10.50 \
  --end 10.10.10.200 \
  --lease 12h \
  --router 10.10.10.1 \
  --dns 10.10.10.1 \
  | sudo tee /etc/dnsmasq.d/pisowifi.conf

sudo systemctl restart dnsmasq
```

### hostapd (SSID mapped to bridge br0.10)

```bash
sudo /usr/local/bin/pisowifi render hostapd \
  --radio wlan0 \
  --ssid PISO \
  --bridge br0.10 \
  --pass "password123" \
  | sudo tee /etc/hostapd/hostapd.conf

echo 'DAEMON_CONF="/etc/hostapd/hostapd.conf"' | sudo tee /etc/default/hostapd
sudo systemctl enable --now hostapd
```

## 7) Run the portal as a system service (systemd)

Create storage directory:

```bash
sudo install -d -m 0755 /var/lib/pisowifi
```

Create service file:

```bash
sudo tee /etc/systemd/system/pisowifi.service >/dev/null <<'UNIT'
[Unit]
Description=PiSoWiFi Portal
After=network-online.target nftables.service
Wants=network-online.target

[Service]
ExecStart=/usr/local/bin/pisowifi serve --listen :8080 --db /var/lib/pisowifi/pisowifi.db --nft-enable --nft-table "inet pisowifi" --nft-allowed4-set allowed4 --title "PiSoWiFi"
Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
UNIT
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now pisowifi
sudo systemctl status pisowifi --no-pager
```

## 8) Quick test

Create a voucher:

```bash
curl -sS -X POST http://127.0.0.1:8080/api/v1/vouchers \
  -H 'Content-Type: application/json' \
  -d '{"minutes":60}'
```

Connect a client to the SSID and open any HTTP site; it should redirect to the portal.

## 9) Adding more VLANs

Repeat for VLAN 20, 30, etc:

```bash
sudo ip link add link br0 name br0.20 type vlan id 20
sudo ip addr add 10.10.20.1/24 dev br0.20
sudo ip link set br0.20 up
```

Add additional dnsmasq ranges (another `render dnsmasq` output block per VLAN).

## 10) Where to upload this app (recommended)

If you want an SSH-only deployment flow, the simplest is:

1. Upload the source code to GitHub or GitLab.
2. SSH into the device with MobaXterm.
3. `git clone`, then `go build` on the device.

If you want faster installs (no build step on the device), you can also upload prebuilt binaries to GitHub/GitLab Releases and download them via `curl`/`wget` in the SSH session.
