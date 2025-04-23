#!/bin/bash

# Revert network connection limits to default values
sudo sysctl -w net.core.somaxconn=4096
sudo sysctl -w net.ipv4.tcp_max_syn_backlog=1024
sudo sysctl -w net.ipv4.tcp_fin_timeout=60
sudo sysctl -w net.ipv4.tcp_tw_reuse=0
sudo sysctl -w net.ipv4.tcp_timestamps=1
sudo sysctl -w net.core.netdev_max_backlog=1000

# Revert TCP memory settings to default values
sudo sysctl -w net.ipv4.tcp_rmem='4096 87380 6291456'
sudo sysctl -w net.ipv4.tcp_wmem='4096 16384 4194304'
sudo sysctl -w net.core.rmem_max=212992
sudo sysctl -w net.core.wmem_max=212992

# Reset TCP slow start
sudo sysctl -w net.ipv4.tcp_slow_start_after_idle=1

# Apply changes
sudo sysctl -p

# Remove the added lines from limits.conf
# Create a temporary file without the lines we added
sudo sed -i.bak '/\* soft nofile 1048576/d' /etc/security/limits.conf
sudo sed -i.bak '/\* hard nofile 1048576/d' /etc/security/limits.conf

echo "System parameters reverted to default values. You may need to restart your application." 