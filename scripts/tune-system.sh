#!/bin/bash

# Increase system-wide limits
sudo sysctl -w net.core.somaxconn=65535
sudo sysctl -w net.ipv4.tcp_max_syn_backlog=65535
sudo sysctl -w net.ipv4.tcp_fin_timeout=30
sudo sysctl -w net.ipv4.tcp_tw_reuse=1
sudo sysctl -w net.ipv4.tcp_timestamps=1
sudo sysctl -w net.core.netdev_max_backlog=65535

# Increase file descriptor limits
sudo sh -c 'echo "* soft nofile 1048576" >> /etc/security/limits.conf'
sudo sh -c 'echo "* hard nofile 1048576" >> /etc/security/limits.conf'

# Optimize TCP settings
sudo sysctl -w net.ipv4.tcp_slow_start_after_idle=0
sudo sysctl -w net.ipv4.tcp_rmem='4096 87380 16777216'
sudo sysctl -w net.ipv4.tcp_wmem='4096 87380 16777216'
sudo sysctl -w net.core.rmem_max=16777216
sudo sysctl -w net.core.wmem_max=16777216

# Apply changes
sudo sysctl -p

echo "System tuning complete. You may need to restart your application." 