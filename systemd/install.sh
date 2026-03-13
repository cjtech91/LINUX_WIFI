#!/bin/sh
set -eu

install -d -m 0755 /var/lib/pisowifi

install -m 0644 /opt/pisowifi/systemd/pisowifi-net.service /etc/systemd/system/pisowifi-net.service
install -m 0644 /opt/pisowifi/systemd/pisowifi.service /etc/systemd/system/pisowifi.service

install -d -m 0755 /etc/systemd/system/dnsmasq.service.d /etc/systemd/system/nginx.service.d

cat >/etc/systemd/system/dnsmasq.service.d/pisowifi.conf <<'OVR'
[Unit]
Requires=pisowifi-net.service
After=pisowifi-net.service
OVR

cat >/etc/systemd/system/nginx.service.d/pisowifi.conf <<'OVR'
[Unit]
Requires=pisowifi-net.service
After=pisowifi-net.service
OVR

systemctl daemon-reload
systemctl enable --now pisowifi-net pisowifi

