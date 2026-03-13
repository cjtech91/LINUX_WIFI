@echo off
setlocal EnableExtensions EnableDelayedExpansion

where ssh >nul 2>nul
if errorlevel 1 (
  echo ERROR: ssh.exe not found in PATH.
  echo Install "OpenSSH Client" on Windows or run this from a terminal where ssh is available.
  echo.
  echo Windows Settings ^> Apps ^> Optional features ^> Add a feature ^> OpenSSH Client
  exit /b 1
)

set "DEFAULT_REPO=https://github.com/cjtech91/LINUX_WIFI.git"
set "DEFAULT_USER=root"
set "DEFAULT_WAN_IF=AUTO"
set "DEFAULT_VLAN_ID=13"
set "DEFAULT_USB_IF="
set "DEFAULT_LAN_IP=10.0.0.1"
set "DEFAULT_LAN_PREFIX=24"
set "DEFAULT_LAN_CIDR=10.0.0.0/24"
set "DEFAULT_DHCP_START=10.0.0.5"
set "DEFAULT_DHCP_END=10.0.0.250"

echo.
echo === PiSoWiFi One-Click Installer (SSH) ===
echo.

set /p HOST=Device IP or hostname (example: 192.168.1.10) :
if "%HOST%"=="" (
  echo ERROR: host is required.
  exit /b 1
)

set /p SSH_USER=SSH user [%DEFAULT_USER%] :
if "%SSH_USER%"=="" set "SSH_USER=%DEFAULT_USER%"

set /p REPO_URL=Git repo URL [%DEFAULT_REPO%] :
if "%REPO_URL%"=="" set "REPO_URL=%DEFAULT_REPO%"

set /p WAN_IF=WAN interface (uplink) [AUTO] :
if "%WAN_IF%"=="" set "WAN_IF=%DEFAULT_WAN_IF%"

set /p VLAN_ID=Client VLAN ID [%DEFAULT_VLAN_ID%] :
if "%VLAN_ID%"=="" set "VLAN_ID=%DEFAULT_VLAN_ID%"

set /p USB_IF=USB-LAN interface (optional, example: enx00e04c68b637) [%DEFAULT_USB_IF%] :
if "%USB_IF%"=="" set "USB_IF=%DEFAULT_USB_IF%"

set /p LAN_IP=Hotspot gateway IP [%DEFAULT_LAN_IP%] :
if "%LAN_IP%"=="" set "LAN_IP=%DEFAULT_LAN_IP%"

set /p LAN_PREFIX=Hotspot prefix length [%DEFAULT_LAN_PREFIX%] :
if "%LAN_PREFIX%"=="" set "LAN_PREFIX=%DEFAULT_LAN_PREFIX%"

set /p LAN_CIDR=Hotspot CIDR [%DEFAULT_LAN_CIDR%] :
if "%LAN_CIDR%"=="" set "LAN_CIDR=%DEFAULT_LAN_CIDR%"

set /p DHCP_START=DHCP start [%DEFAULT_DHCP_START%] :
if "%DHCP_START%"=="" set "DHCP_START=%DEFAULT_DHCP_START%"

set /p DHCP_END=DHCP end [%DEFAULT_DHCP_END%] :
if "%DHCP_END%"=="" set "DHCP_END=%DEFAULT_DHCP_END%"

set /p SINGLE_LAN=Single-LAN mode (USB-LAN as LAN, no VLAN trunk) [Y/n] :
if "%SINGLE_LAN%"=="" set "SINGLE_LAN=Y"
if /I "%SINGLE_LAN%"=="Y" (set "SINGLE_LAN=1") else (set "SINGLE_LAN=0")

set "TMP_SCRIPT=%TEMP%\pisowifi_install_%RANDOM%.sh"

> "%TMP_SCRIPT%" (
  echo set -euo pipefail
  echo export DEBIAN_FRONTEND=noninteractive
  echo.
  echo WAN_IF="%WAN_IF%"
  echo VLAN_ID="%VLAN_ID%"
  echo USB_IF="%USB_IF%"
  echo LAN_IP="%LAN_IP%"
  echo LAN_PREFIX="%LAN_PREFIX%"
  echo LAN_CIDR="%LAN_CIDR%"
  echo DHCP_START="%DHCP_START%"
  echo DHCP_END="%DHCP_END%"
  echo REPO_URL="%REPO_URL%"
  echo SINGLE_LAN="%SINGLE_LAN%"
  echo.
  echo if [ -z "$WAN_IF" ] ^|^| [ "$WAN_IF" = "AUTO" ]; then
  echo ^  WAN_IF="$(ip route show default 2^>^/dev/null ^| awk '{for(i=1;i<=NF;i++) if($i=="dev"){print $(i+1); exit}}')"
  echo fi
  echo if [ -z "$WAN_IF" ]; then
  echo ^  echo "ERROR: Could not auto-detect WAN interface. Set it manually in install.bat prompt." 1^>^&2
  echo ^  exit 1
  echo fi
  echo echo "Detected WAN interface: $WAN_IF"
  echo.
  echo echo "== Installing packages =="
  echo sudo apt update
  echo sudo apt install -y git golang-go hostapd dnsmasq nftables iproute2 nginx
  echo.
  echo echo "== Enabling IP forwarding =="
  echo sudo sysctl -w net.ipv4.ip_forward=1
  echo echo "net.ipv4.ip_forward=1" ^| sudo tee /etc/sysctl.d/99-ipforward.conf ^>/dev/null
  echo.
  echo echo "== Getting source code =="
  echo sudo install -d -m 0755 /opt
  echo if [ -d /opt/pisowifi/.git ]; then
  echo ^  cd /opt/pisowifi ^&^& git pull --rebase
  echo else
  echo ^  sudo git clone "$REPO_URL" /opt/pisowifi
  echo fi
  echo sudo chown -R "$USER":"$USER" /opt/pisowifi
  echo.
  echo echo "== Building pisowifi =="
  echo cd /opt/pisowifi
  echo go mod tidy
  echo go test ./...
  echo go vet ./...
  echo go build -o pisowifi ./cmd/pisowifi
  echo sudo install -m 0755 pisowifi /usr/local/bin/pisowifi
  echo /usr/local/bin/pisowifi serve -h ^>/dev/null
  echo.
  echo echo "== Configuring LAN (Single-LAN mode: $SINGLE_LAN) =="
  echo if [ "${SINGLE_LAN}" = "1" ]; then
  echo ^  if [ -z "$USB_IF" ] ^|| ! ip link show "$USB_IF" ^>/dev/null 2^>^&1; then
  echo ^    echo "ERROR: SINGLE_LAN requires USB_IF (your LAN NIC) to be set and present" 1^>^&2; exit 1;
  echo ^  fi
  echo ^  sudo ip link add br10 type bridge 2^>^/dev/null ^|^| true
  echo ^  sudo ip link set br10 up
  echo ^  sudo ip addr flush dev "$USB_IF" ^|^| true
  echo ^  sudo ip link set "$USB_IF" up
  echo ^  sudo ip link set "$USB_IF" master br10
  echo ^  sudo ip addr replace "$LAN_IP/$LAN_PREFIX" dev br10
  echo else
  echo ^  echo "== Creating VLAN interface $WAN_IF.$VLAN_ID and bridging to br10 =="
  echo ^  sudo ip link show "$WAN_IF" ^>/dev/null
  echo ^  sudo ip link show "$WAN_IF.$VLAN_ID" ^>/dev/null 2^>^&1 ^|^| sudo ip link add link "$WAN_IF" name "$WAN_IF.$VLAN_ID" type vlan id "$VLAN_ID"
  echo ^  sudo ip link set "$WAN_IF.$VLAN_ID" up
  echo ^  sudo ip link add br10 type bridge 2^>^/dev/null ^|^| true
  echo ^  sudo ip link set br10 up
  echo ^  sudo ip addr flush dev "$WAN_IF.$VLAN_ID" ^|^| true
  echo ^  sudo ip link set "$WAN_IF.$VLAN_ID" master br10
  echo ^  if [ -n "$USB_IF" ] ^&^& ip link show "$USB_IF" ^>/dev/null 2^>^&1; then
  echo ^    sudo ip addr flush dev "$USB_IF" ^|^| true
  echo ^    sudo ip link set "$USB_IF" up
  echo ^    sudo ip link set "$USB_IF" master br10
  echo ^  fi
  echo ^  sudo ip addr replace "$LAN_IP/$LAN_PREFIX" dev br10
  echo fi
  echo.
  echo echo "== Persisting network setup at boot (systemd) =="
  echo sudo tee /etc/pisowifi.env ^>/dev/null ^<^< 'ENV'
  echo WAN_IF=$WAN_IF
  echo VLAN_ID=$VLAN_ID
  echo USB_IF=$USB_IF
  echo LAN_IP=$LAN_IP
  echo LAN_PREFIX=$LAN_PREFIX
  echo SINGLE_LAN=%SINGLE_LAN%
  echo ENV
  echo sudo tee /etc/systemd/system/pisowifi-net.service ^>/dev/null ^<^< 'UNIT'
  echo [Unit]
  echo Description=PiSoWiFi Network Setup
  echo Wants=network-online.target
  echo After=network-online.target
  echo Before=dnsmasq.service nginx.service nftables.service pisowifi.service
  echo.
  echo [Service]
  echo Type=oneshot
  echo RemainAfterExit=yes
  echo EnvironmentFile=-/etc/pisowifi.env
  echo ExecStart=/bin/bash -lc 'set -euxo pipefail; WAN_IF="${WAN_IF:-AUTO}"; VLAN_ID="${VLAN_ID:-13}"; USB_IF="${USB_IF:-}"; SINGLE_LAN="${SINGLE_LAN:-1}"; LAN_IP="${LAN_IP:-10.0.0.1}"; LAN_PREFIX="${LAN_PREFIX:-24}"; if [ -z "$WAN_IF" ] || [ "$WAN_IF" = "AUTO" ]; then if ip link show end0 >/dev/null 2>&1; then WAN_IF="end0"; elif ip link show eth0 >/dev/null 2>&1; then WAN_IF="eth0"; else WAN_IF="$(ip -o -4 route show to default 2>/dev/null | awk "{print \$5; exit}")"; fi; fi; [ -n "$WAN_IF" ]; ip link show "$WAN_IF" >/dev/null; ip link add br10 type bridge 2>/dev/null || true; ip link set br10 up; if [ "$SINGLE_LAN" = "1" ]; then if [ -z "$USB_IF" ] || ! ip link show "$USB_IF" >/dev/null 2>&1; then echo "SINGLE_LAN requires USB_IF" 1>&2; exit 1; fi; ip addr flush dev "$USB_IF" || true; ip link set "$USB_IF" up; ip link set "$USB_IF" master br10; else ip link show "$WAN_IF.$VLAN_ID" >/dev/null 2>&1 || ip link add link "$WAN_IF" name "$WAN_IF.$VLAN_ID" type vlan id "$VLAN_ID"; ip link set "$WAN_IF.$VLAN_ID" up; ip addr flush dev "$WAN_IF.$VLAN_ID" || true; ip link set "$WAN_IF.$VLAN_ID" master br10; if [ -n "$USB_IF" ] && ip link show "$USB_IF" >/dev/null 2>&1; then ip addr flush dev "$USB_IF" || true; ip link set "$USB_IF" up; ip link set "$USB_IF" master br10; fi; fi; ip addr replace "$LAN_IP/$LAN_PREFIX" dev br10'
  echo.
  echo [Install]
  echo WantedBy=multi-user.target
  echo UNIT
  echo sudo install -d -m 0755 /etc/systemd/system/dnsmasq.service.d /etc/systemd/system/nginx.service.d
  echo sudo tee /etc/systemd/system/dnsmasq.service.d/pisowifi.conf ^>/dev/null ^<^< 'OVR'
  echo [Unit]
  echo Requires=pisowifi-net.service
  echo After=pisowifi-net.service
  echo OVR
  echo sudo tee /etc/systemd/system/nginx.service.d/pisowifi.conf ^>/dev/null ^<^< 'OVR'
  echo [Unit]
  echo Requires=pisowifi-net.service
  echo After=pisowifi-net.service
  echo OVR
  echo.
  echo echo "== dnsmasq config =="
  echo sudo /usr/local/bin/pisowifi render dnsmasq ^
  echo ^  --if br10 ^
  echo ^  --start "$DHCP_START" ^
  echo ^  --end "$DHCP_END" ^
  echo ^  --lease 12h ^
  echo ^  --router "$LAN_IP" ^
  echo ^  --dns "$LAN_IP" ^
  echo ^  ^| sudo tee /etc/dnsmasq.d/pisowifi.conf ^>/dev/null
  echo sudo sed -i '1i bind-dynamic\nexcept-interface=lo\nlisten-address='"$LAN_IP"'\n' /etc/dnsmasq.d/pisowifi.conf
  echo sudo dnsmasq --test
  echo.
  echo echo "== nftables config (redirect to 80) =="
  echo sudo mkdir -p /etc/nftables.d
  echo sudo /usr/local/bin/pisowifi render nft ^
  echo ^  --lan-if br10 ^
  echo ^  --wan-if "$WAN_IF" ^
  echo ^  --portal-port 80 ^
  echo ^  --client-cidr "$LAN_CIDR" ^
  echo ^  ^| sudo tee /etc/nftables.d/pisowifi.nft ^>/dev/null
  echo if [ ! -f /etc/nftables.conf ]; then printf 'include "/etc/nftables.d/pisowifi.nft"\n' ^| sudo tee /etc/nftables.conf ^>/dev/null; fi
  echo if ! sudo grep -q 'include "/etc/nftables.d/pisowifi.nft"' /etc/nftables.conf; then printf 'include "/etc/nftables.d/pisowifi.nft"\n' ^| sudo tee -a /etc/nftables.conf ^>/dev/null; fi
  echo sudo nft -c -f /etc/nftables.conf
  echo.
  echo echo "== nginx config (portal on http://$LAN_IP/) =="
  echo sudo tee /etc/nginx/sites-available/pisowifi ^>/dev/null ^<^< 'NGINX'
  echo server ^{
  echo ^    listen 80;
  echo ^    server_name _;
  echo.
  echo ^    location / ^{
  echo ^        proxy_pass http://127.0.0.1:8080;
  echo ^        proxy_http_version 1.1;
  echo ^        proxy_set_header Host $host;
  echo ^        proxy_set_header X-Real-IP $remote_addr;
  echo ^        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
  echo ^    }
  echo }
  echo NGINX
  echo sudo rm -f /etc/nginx/sites-enabled/default
  echo sudo ln -sf /etc/nginx/sites-available/pisowifi /etc/nginx/sites-enabled/pisowifi
  echo sudo nginx -t
  echo.
  echo echo "== systemd service (pisowifi) =="
  echo sudo install -d -m 0755 /var/lib/pisowifi
  echo sudo tee /etc/systemd/system/pisowifi.service ^>/dev/null ^<^< 'UNIT'
  echo [Unit]
  echo Description=PiSoWiFi Portal
  echo Requires=pisowifi-net.service
  echo After=pisowifi-net.service network-online.target nftables.service nginx.service dnsmasq.service
  echo Wants=pisowifi-net.service network-online.target
  echo.
  echo [Service]
  echo ExecStart=/usr/local/bin/pisowifi serve --listen 127.0.0.1:8080 --db /var/lib/pisowifi/pisowifi.db --nft-enable --nft-table "inet pisowifi" --nft-allowed4-set allowed4 --title "PiSoWiFi"
  echo Restart=always
  echo RestartSec=2
  echo.
  echo [Install]
  echo WantedBy=multi-user.target
  echo UNIT
  echo sudo systemctl daemon-reload
  echo sudo systemctl enable --now pisowifi-net
  echo sudo systemctl enable --now dnsmasq nftables nginx pisowifi
  echo sudo systemctl restart pisowifi-net dnsmasq nftables nginx pisowifi
  echo.
  echo echo "== Done =="
  echo echo "Portal: http://%LAN_IP%/"
  echo echo "Check services: systemctl status dnsmasq nftables nginx pisowifi --no-pager"
  echo echo "If you add Wi-Fi later, plug a Wi-Fi adapter and run hostapd setup from deployment.md."
)

echo.
echo Connecting to %SSH_USER%@%HOST% ...
echo You may be prompted for password if SSH keys are not configured.
echo.

ssh -o StrictHostKeyChecking=accept-new %SSH_USER%@%HOST% "bash -s" < "%TMP_SCRIPT%"
set "ERR=%ERRORLEVEL%"

del "%TMP_SCRIPT%" >nul 2>nul

if not "%ERR%"=="0" (
  echo.
  echo ERROR: install failed with exit code %ERR%.
  exit /b %ERR%
)

echo.
echo SUCCESS.
exit /b 0
