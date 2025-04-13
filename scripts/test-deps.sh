#!/bin/bash

# Script to manage test dependencies (Redis, Kafka) for integration tests

function start_deps() {
    echo "Starting test dependencies (Redis, Kafka)..."
    
    # Start Redis container
    docker run --name matchingo-test-redis -d -p 6380:6379 redis:alpine

    # Start Kafka (using Confluent's image which includes Zookeeper)
    docker run --name matchingo-test-kafka -d \
        -p 9092:9092 \
        -p 2181:2181 \
        -e KAFKA_ADVERTISED_LISTENERS=PLAINTEXT://localhost:9092 \
        -e KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR=1 \
        confluentinc/cp-kafka:latest

    # Wait for services to be ready
    echo "Waiting for services to be ready..."
    sleep 10
    
    # Create test topic in Kafka
    docker exec matchingo-test-kafka kafka-topics --create --topic matchingo-test --bootstrap-server localhost:9092 --partitions 1 --replication-factor 1
    
    echo "Test dependencies are ready"
}

function stop_deps() {
    echo "Stopping test dependencies..."
    
    # Stop and remove containers
    docker rm -f matchingo-test-redis 2>/dev/null || true
    docker rm -f matchingo-test-kafka 2>/dev/null || true
    
    echo "Test dependencies stopped and removed"
}

# Parse command line arguments
case "$1" in
    start)
        start_deps
        ;;
    stop)
        stop_deps
        ;;
    restart)
        stop_deps
        start_deps
        ;;
    *)
        echo "Usage: $0 {start|stop|restart}"
        exit 1
        ;;
esac

exit 0 