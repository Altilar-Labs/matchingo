# Matchingo Server Deployment Guide for Ubuntu 24.04

This guide provides step-by-step instructions for deploying the Matchingo trading server on Ubuntu 24.04 from source code.

## Prerequisites

- Ubuntu 24.04 LTS server
- Root or sudo access
- Git
- Go 1.21+ (will be installed as part of this guide)
- Redis (optional, for production deployments)
- Kafka (optional, for production deployments)

## 1. System Preparation

### Update the System

```bash
sudo apt update
sudo apt upgrade -y
```

### Install Required Packages

```bash
sudo apt install -y git make build-essential
```

### Install Go 1.21+

```bash
wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
rm go1.21.0.linux-amd64.tar.gz
```

Add Go to your PATH by adding these lines to ~/.profile:

```bash
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.profile
echo 'export PATH=$PATH:$HOME/go/bin' >> ~/.profile
source ~/.profile
```

Verify Go installation:

```bash
go version
```

## 2. Clone the Repository

Create a directory for the application and clone the repository:

```bash
mkdir -p /opt/matchingo
cd /opt/matchingo
git clone https://github.com/erain9/matchingo.git .
```

## 3. Build the Application

Compile the server binary:

```bash
cd /opt/matchingo
go build -o bin/matchingo-server cmd/server/main.go
```

## 4. Configure Dependencies

### For Development/Testing

The server can use in-memory backends for both the order book and messaging:

```bash
# No additional configuration required for memory backend
```

### For Production

#### Redis Setup

Install Redis:

```bash
sudo apt install -y redis-server
sudo systemctl enable redis-server
sudo systemctl start redis-server
```

Configure Redis (optional hardening):

```bash
sudo nano /etc/redis/redis.conf
# Set a password by uncommenting and editing the requirepass line
# requirepass your_strong_password
```

Restart Redis:

```bash
sudo systemctl restart redis-server
```

#### Kafka Setup

Install Kafka:

```bash
# Install Java
sudo apt install -y default-jre

# Download and extract Kafka
wget https://downloads.apache.org/kafka/3.6.0/kafka_2.13-3.6.0.tgz
tar -xzf kafka_2.13-3.6.0.tgz
sudo mv kafka_2.13-3.6.0 /opt/kafka
rm kafka_2.13-3.6.0.tgz

# Create systemd services for Zookeeper and Kafka
```

Create a systemd service file for Zookeeper:

```bash
sudo nano /etc/systemd/system/zookeeper.service
```

Add the following content:

```
[Unit]
Description=Apache Zookeeper server
Documentation=http://zookeeper.apache.org
Requires=network.target remote-fs.target
After=network.target remote-fs.target

[Service]
Type=simple
ExecStart=/opt/kafka/bin/zookeeper-server-start.sh /opt/kafka/config/zookeeper.properties
ExecStop=/opt/kafka/bin/zookeeper-server-stop.sh
Restart=on-abnormal

[Install]
WantedBy=multi-user.target
```

Create a systemd service file for Kafka:

```bash
sudo nano /etc/systemd/system/kafka.service
```

Add the following content:

```
[Unit]
Description=Apache Kafka Server
Documentation=http://kafka.apache.org/documentation.html
Requires=zookeeper.service
After=zookeeper.service

[Service]
Type=simple
Environment="JAVA_HOME=/usr/lib/jvm/default-java"
ExecStart=/opt/kafka/bin/kafka-server-start.sh /opt/kafka/config/server.properties
ExecStop=/opt/kafka/bin/kafka-server-stop.sh
Restart=on-abnormal

[Install]
WantedBy=multi-user.target
```

Enable and start the services:

```bash
sudo systemctl daemon-reload
sudo systemctl enable zookeeper.service
sudo systemctl start zookeeper.service
sudo systemctl enable kafka.service
sudo systemctl start kafka.service
```

Create the required Kafka topic:

```bash
/opt/kafka/bin/kafka-topics.sh --create --topic test-msg-queue --bootstrap-server localhost:9092 --partitions 1 --replication-factor 1
```

## 5. Configure the Application

Create a configuration directory:

```bash
mkdir -p /opt/matchingo/config
```

Create a basic configuration file:

```bash
nano /opt/matchingo/config/config.yaml
```

Add the following content (adjust as needed):

```yaml
server:
  grpc_addr: ":50051"
  http_addr: ":8080"

redis:
  addr: "localhost:6379"
  password: ""  # Set if using Redis auth
  db: 0

kafka:
  broker_addr: "localhost:9092"
  topic: "test-msg-queue"
```

## 6. Create a Systemd Service

Create a service file for the Matchingo server:

```bash
sudo nano /etc/systemd/system/matchingo.service
```

Add the following content:

```
[Unit]
Description=Matchingo Trading Server
After=network.target
Requires=redis-server.service kafka.service
After=redis-server.service kafka.service

[Service]
User=ubuntu
Group=ubuntu
WorkingDirectory=/opt/matchingo
ExecStart=/opt/matchingo/bin/matchingo-server --config /opt/matchingo/config/config.yaml
Restart=on-failure
RestartSec=5

# Optional security enhancements
PrivateTmp=true
ProtectSystem=full
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
```

Enable and start the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable matchingo.service
sudo systemctl start matchingo.service
```

Check the status of the service:

```bash
sudo systemctl status matchingo.service
```

View logs:

```bash
sudo journalctl -u matchingo.service -f
```

## 7. Updating the Application

To update the application with new code:

```bash
# Pull the latest code
cd /opt/matchingo
git pull

# Rebuild the application
go build -o bin/matchingo-server cmd/server/main.go

# Restart the service
sudo systemctl restart matchingo.service
```

## 8. Troubleshooting

### Service Won't Start

Check logs for detailed error messages:

```bash
sudo journalctl -u matchingo.service -e
```

### Connection Issues

Verify that Redis and Kafka are running:

```bash
sudo systemctl status redis-server
sudo systemctl status kafka
```

Check that the ports are open and listening:

```bash
sudo ss -tulpn | grep -E '9092|6379|50051|8080'
```

### Firewall Configuration

If you're using UFW (Ubuntu's firewall), allow the necessary ports:

```bash
sudo ufw allow 50051/tcp  # gRPC port
sudo ufw allow 8080/tcp   # HTTP port
```

## 9. Security Considerations

For production deployments:

1. Set up proper firewall rules to restrict access to the application
2. Use strong passwords for Redis
3. Configure TLS for both gRPC and HTTP interfaces
4. Consider implementing authentication for API access
5. Use secure configurations for Kafka (authentication, authorization)
6. Regularly update the system and dependencies

## 10. Monitoring

Consider setting up monitoring for the application:

- Prometheus for metrics
- Grafana for visualization
- Alert manager for notifications

You can integrate these with the systemd service for comprehensive monitoring.

## 11. Backup Strategy

For a production deployment, establish a regular backup strategy:

- Redis data snapshots
- Kafka topic replication
- Regular system backups

---

This completes the deployment guide for Matchingo server on Ubuntu 24.04. For additional support, refer to the project documentation or open an issue on the GitHub repository. 