#!/bin/bash

BIN=$(pwd)/stand-up
TOKEN=$1

EXEC="$BIN -t $TOKEN"

SERVICE="[Unit]
Description=Stand-up automation service.
Wants=network-online.target
After=network.target network-online.target
StartLimitInterval=2000
StartLimitBurst=5

[Service]
Type=simple
ExecStart=$EXEC
Restart=on-failure
RestartSec=300

[Install]
WantedBy=multi-user.target"

TIMER="[Unit]
Description=Stand-up automation timer.

[Timer]
OnCalendar=Mon,Tue,Wed,Thu,Fri *-*-* 10:00:00

[Install]
WantedBy=timers.target
"

make build

sudo -s <<EOF
echo "$SERVICE" > /etc/systemd/system/stand-up.service
echo "$TIMER" > /etc/systemd/system/stand-up.timer
systemctl enable stand-up.timer
systemctl start stand-up.timer
EOF

echo "echo \"\$SERVICE\" > /etc/systemd/system/stand-up.service"
echo "echo \"\$TIMER\" > /etc/systemd/system/stand-up.timer"
echo "systemctl enable stand-up.timer"
echo "systemctl start stand-up.timer"
echo Cheers!
