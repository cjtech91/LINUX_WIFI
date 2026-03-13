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
sudo chown -R "$USER":"$USER" /opt/pisowifi
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
go mod tidy
go test ./...
go vet ./...
go build -o pisowifi ./cmd/pisowifi
sudo install -m 0755 pisowifi /usr/local/bin/pisowifi
command -v pisowifi
ls -l /usr/local/bin/pisowifi
```

## 4) Install Linux dependencies (on the device)

Debian/Ubuntu/Armbian/Raspberry Pi OS:

```bash
sudo apt update
sudo apt install -y hostapd dnsmasq nftables iproute2
sudo sysctl -w net.ipv4.ip_forward=1
echo "net.ipv4.ip_forward=1" | sudo tee /etc/sysctl.d/99-ipforward.conf
```

## 5) Identify your real interface names (important)

On many Linux images, interfaces are not named `eth0`/`wlan0`. Before applying any network commands, check:

```bash
ip -br link
ip route show default
iw dev || true
```

- Use the `dev` shown by `ip route show default` as your WAN/uplink interface (example: `end0`, `enp1s0`, `eth0`).
- Use the interface shown by `iw dev` as your Wi‑Fi radio (example: `wlan0`, `wlp2s0`). If `iw dev` shows nothing, you currently have no Wi‑Fi interface available for hostapd.
- If you plugged a USB-LAN adapter, it typically shows up as `enx...` (example: `enx00e04c68b637`).

Important:

- In the commands below, replace placeholders like `<WAN_IF>` with the real interface name (example: `end0`). Do not include `<` and `>`.

## 6) Recommended topology (tested: end0 WAN + VLAN10 + USB-LAN + optional Wi‑Fi AP)

This setup matches what you already have working on Orange Pi:

- WAN/uplink: `end0` (DHCP from upstream)
- Client VLAN: `end0.10` (VLAN ID 10, gateway 10.0.0.1/24)
- Client bridge: `br10` (bridge `end0.10` + USB-LAN + Wi‑Fi AP interface when available)

Do not bridge your WAN interface into br10.

Create VLAN 10 on end0:

```bash
sudo ip link show end0.10 >/dev/null 2>&1 || sudo ip link add link end0 name end0.10 type vlan id 10
sudo ip link set end0.10 up
```

Create bridge br10, add end0.10 and USB-LAN to it, and put the gateway IP on br10:

```bash
USBIF="<YOUR_USB_LAN_INTERFACE>"   # example: enx00e04c68b637 (leave as-is if you don't have USB-LAN)

sudo ip link add br10 type bridge 2>/dev/null || true
sudo ip link set br10 up

sudo ip addr flush dev end0.10
sudo ip link set end0.10 master br10

if ip link show "$USBIF" >/dev/null 2>&1; then
  sudo ip addr flush dev "$USBIF" || true
  sudo ip link set "$USBIF" up
  sudo ip link set "$USBIF" master br10
fi

sudo ip addr replace 10.0.0.1/24 dev br10
ip -br addr show br10 end0.10 "$USBIF" 2>/dev/null || true
```

## 7) Generate configs (nftables + dnsmasq)

### nftables (captive portal redirect + allowlist + NAT)

```bash
sudo mkdir -p /etc/nftables.d
sudo /usr/local/bin/pisowifi render nft \
  --lan-if br10 \
  --wan-if end0 \
  --portal-port 80 \
  --client-cidr 10.0.0.0/24 \
  | sudo tee /etc/nftables.d/pisowifi.nft

echo 'include "/etc/nftables.d/pisowifi.nft"' | sudo tee /etc/nftables.conf
sudo nft -c -f /etc/nftables.conf
sudo systemctl enable --now nftables
```

### dnsmasq (DHCP/DNS for VLAN 10)

On Ubuntu, `systemd-resolved` listens on 127.0.0.53:53. To avoid port conflicts, bind dnsmasq only to the hotspot gateway IP.

```bash
sudo /usr/local/bin/pisowifi render dnsmasq \
  --if br10 \
  --start 10.0.0.50 \
  --end 10.0.0.200 \
  --lease 12h \
  --router 10.0.0.1 \
  --dns 10.0.0.1 \
  | sudo tee /etc/dnsmasq.d/pisowifi.conf

sudo sed -i '1i bind-dynamic\nexcept-interface=lo\nlisten-address=10.0.0.1\n' /etc/dnsmasq.d/pisowifi.conf
sudo dnsmasq --test
sudo systemctl restart dnsmasq
```

## 8) Install Nginx (recommended: portal on port 80)

Nginx listens on port 80 and proxies to the app running on 127.0.0.1:8080.

```bash
sudo apt update
sudo apt install -y nginx
```

```bash
sudo tee /etc/nginx/sites-available/pisowifi >/dev/null <<'NGINX'
server {
    listen 80;
    server_name _;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
NGINX
```

```bash
sudo rm -f /etc/nginx/sites-enabled/default
sudo ln -sf /etc/nginx/sites-available/pisowifi /etc/nginx/sites-enabled/pisowifi
sudo nginx -t
sudo systemctl enable --now nginx
sudo systemctl restart nginx
```

## 9) hostapd (optional: only if you have a Wi‑Fi interface)

If `iw dev` shows a Wi‑Fi interface `<WIFI_IF>`, map the SSID to the bridge `br10`:

```bash
sudo /usr/local/bin/pisowifi render hostapd \
  --radio <WIFI_IF> \
  --ssid PISO \
  --bridge br10 \
  --pass "password123" \
  | sudo tee /etc/hostapd/hostapd.conf

echo 'DAEMON_CONF="/etc/hostapd/hostapd.conf"' | sudo tee /etc/default/hostapd
sudo systemctl enable --now hostapd
```

## 10) Run the portal as a system service (systemd)

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
ExecStart=/usr/local/bin/pisowifi serve --listen 127.0.0.1:8080 --db /var/lib/pisowifi/pisowifi.db --nft-enable --nft-table "inet pisowifi" --nft-allowed4-set allowed4 --title "PiSoWiFi"
Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
UNIT
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now pisowifi-net
sudo systemctl enable --now pisowifi
sudo systemctl status pisowifi --no-pager
```

## 11) Quick test

Create a voucher:

```bash
curl -sS -X POST http://127.0.0.1:8080/api/v1/vouchers \
  -H 'Content-Type: application/json' \
  -d '{"minutes":60}'
```

Connect a client (USB-LAN or Wi‑Fi). Open an HTTP site; it should redirect to the portal at http://10.0.0.1/.

## 12) Adding more VLANs

Repeat for VLAN/SSID 20, 30, etc (use a new bridge per VLAN/SSID):

```bash
sudo ip link add br20 type bridge
sudo ip addr add 10.10.20.1/24 dev br20
sudo ip link set br20 up
```

Add additional dnsmasq ranges (another `render dnsmasq` output block per VLAN).

## 13) Where to upload this app (recommended)

If you want an SSH-only deployment flow, the simplest is:

1. Upload the source code to GitHub or GitLab.
2. SSH into the device with MobaXterm.
3. `git clone`, then `go build` on the device.

If you want faster installs (no build step on the device), you can also upload prebuilt binaries to GitHub/GitLab Releases and download them via `curl`/`wget` in the SSH session.
