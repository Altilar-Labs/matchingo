package testutil

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

// DockerContainer represents a Docker container used for testing
type DockerContainer struct {
	ID        string
	Name      string
	Type      string
	Port      string
	HostPort  string
	StartedAt time.Time
}

// StartRedisContainer starts a Redis container for testing
func StartRedisContainer(ctx context.Context) (*DockerContainer, error) {
	containerName := fmt.Sprintf("matchingo-redis-test-%d", time.Now().Unix())
	hostPort := "6380"

	cmd := exec.CommandContext(ctx, "docker", "run", "--rm", "-d",
		"--name", containerName,
		"-p", hostPort+":6379",
		"redis:alpine")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to start Redis container: %w, output: %s", err, output)
	}

	containerID := strings.TrimSpace(string(output))
	container := &DockerContainer{
		ID:        containerID,
		Name:      containerName,
		Type:      "redis",
		Port:      "6379",
		HostPort:  hostPort,
		StartedAt: time.Now(),
	}

	// Wait for Redis to be ready
	redisClient := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("localhost:%s", hostPort),
	})
	defer redisClient.Close()

	// Try to ping Redis with timeout
	pingCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	for {
		select {
		case <-pingCtx.Done():
			// Clean up container if timeout
			_ = container.Stop(ctx)
			return nil, fmt.Errorf("timed out waiting for Redis to be ready")
		default:
			_, err := redisClient.Ping(pingCtx).Result()
			if err == nil {
				return container, nil
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// StartKafkaContainer starts a Kafka container for testing
func StartKafkaContainer(ctx context.Context) (*DockerContainer, error) {
	containerName := fmt.Sprintf("matchingo-kafka-test-%d", time.Now().Unix())
	hostPort := "9092"

	// Start Zookeeper container first
	zookeeperName := fmt.Sprintf("matchingo-zookeeper-test-%d", time.Now().Unix())
	zkCmd := exec.CommandContext(ctx, "docker", "run", "--rm", "-d",
		"--name", zookeeperName,
		"-e", "ZOOKEEPER_CLIENT_PORT=2181",
		"confluentinc/cp-zookeeper:latest")

	output, err := zkCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to start Zookeeper container: %w, output: %s", err, output)
	}
	zookeeperID := strings.TrimSpace(string(output))

	// Start Kafka container
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm", "-d",
		"--name", containerName,
		"--link", zookeeperName+":zookeeper",
		"-p", hostPort+":9092",
		"-e", "KAFKA_ZOOKEEPER_CONNECT=zookeeper:2181",
		"-e", "KAFKA_ADVERTISED_LISTENERS=PLAINTEXT://localhost:"+hostPort,
		"-e", "KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR=1",
		"confluentinc/cp-kafka:latest")

	output, err = cmd.CombinedOutput()
	if err != nil {
		// Clean up zookeeper if kafka fails to start
		zkCleanup := exec.CommandContext(ctx, "docker", "rm", "-f", zookeeperID)
		_ = zkCleanup.Run()
		return nil, fmt.Errorf("failed to start Kafka container: %w, output: %s", err, output)
	}

	containerID := strings.TrimSpace(string(output))
	container := &DockerContainer{
		ID:        containerID,
		Name:      containerName,
		Type:      "kafka",
		Port:      "9092",
		HostPort:  hostPort,
		StartedAt: time.Now(),
	}

	// Wait for Kafka to be ready (we'll use a simple tcp connection test)
	for i := 0; i < 40; i++ {
		select {
		case <-ctx.Done():
			// Clean up containers if context canceled
			zkCleanup := exec.CommandContext(context.Background(), "docker", "rm", "-f", zookeeperID)
			_ = zkCleanup.Run()
			_ = container.Stop(context.Background())
			return nil, ctx.Err()
		default:
			createTopicCmd := exec.CommandContext(
				ctx,
				"docker", "exec", containerName,
				"kafka-topics", "--create",
				"--bootstrap-server", "localhost:9092",
				"--replication-factor", "1",
				"--partitions", "1",
				"--topic", "matchingo-test",
			)

			if err := createTopicCmd.Run(); err == nil {
				// Successfully created topic, container is ready
				return container, nil
			}

			time.Sleep(1 * time.Second)
		}
	}

	// Clean up if we couldn't create the topic
	zkCleanup := exec.CommandContext(context.Background(), "docker", "rm", "-f", zookeeperID)
	_ = zkCleanup.Run()
	_ = container.Stop(context.Background())
	return nil, fmt.Errorf("timed out waiting for Kafka to be ready")
}

// Stop stops and removes the Docker container
func (c *DockerContainer) Stop(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", c.ID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to stop container %s: %w, output: %s", c.ID, err, output)
	}

	// If this is a Kafka container, also stop the linked Zookeeper container
	if c.Type == "kafka" {
		// We need to find any zookeeper container that might be linked
		cmd := exec.CommandContext(ctx, "docker", "ps", "-a", "--filter", "name=matchingo-zookeeper-test", "--format", "{{.ID}}")
		output, err := cmd.CombinedOutput()
		if err == nil && len(output) > 0 {
			zkIDs := strings.Fields(string(output))
			for _, zkID := range zkIDs {
				zkCleanup := exec.CommandContext(ctx, "docker", "rm", "-f", strings.TrimSpace(zkID))
				_ = zkCleanup.Run()
			}
		}
	}

	return nil
}

// WithRedisOnly runs a test function with only Redis dependency
// It starts the Redis container, runs the test, then stops the container regardless of the test outcome
func WithRedisOnly(t interface {
	Helper()
	Skip(args ...interface{})
	Cleanup(f func())
	Errorf(format string, args ...interface{})
}, testFunc func(redisAddr string)) {
	ctx := context.Background()

	// Try to start Redis
	redisContainer, err := StartRedisContainer(ctx)
	if err != nil {
		t.Skip("Skipping test: could not start Redis container:", err)
		return
	}

	// Ensure cleanup happens after test
	t.Cleanup(func() {
		_ = redisContainer.Stop(context.Background())
	})

	// Run the test with Redis address
	redisAddr := fmt.Sprintf("localhost:%s", redisContainer.HostPort)
	testFunc(redisAddr)
}

// WithKafkaOnly runs a test function with only Kafka dependency
// It starts the Kafka container, runs the test, then stops the container regardless of the test outcome
func WithKafkaOnly(t interface {
	Helper()
	Skip(args ...interface{})
	Cleanup(f func())
	Errorf(format string, args ...interface{})
}, testFunc func(kafkaAddr string)) {
	ctx := context.Background()

	// Try to start Kafka
	kafkaContainer, err := StartKafkaContainer(ctx)
	if err != nil {
		t.Skip("Skipping test: could not start Kafka container:", err)
		return
	}

	// Ensure cleanup happens after test
	t.Cleanup(func() {
		_ = kafkaContainer.Stop(context.Background())
	})

	// Run the test with Kafka address
	kafkaAddr := fmt.Sprintf("localhost:%s", kafkaContainer.HostPort)
	testFunc(kafkaAddr)
}

// WithTestDependencies runs a test function with Redis and Kafka dependencies
// It starts the containers, runs the test, then stops the containers regardless of test outcome
func WithTestDependencies(t interface {
	Helper()
	Skip(args ...interface{})
	Cleanup(f func())
	Errorf(format string, args ...interface{})
}, testFunc func(redisAddr, kafkaAddr string)) {
	ctx := context.Background()

	// Try to start Redis
	redisContainer, err := StartRedisContainer(ctx)
	if err != nil {
		t.Skip("Skipping test: could not start Redis container:", err)
		return
	}

	// Try to start Kafka
	kafkaContainer, err := StartKafkaContainer(ctx)
	if err != nil {
		// Clean up Redis container
		_ = redisContainer.Stop(ctx)
		t.Skip("Skipping test: could not start Kafka container:", err)
		return
	}

	// Ensure cleanup happens after test
	t.Cleanup(func() {
		_ = redisContainer.Stop(context.Background())
		_ = kafkaContainer.Stop(context.Background())
	})

	// Run the test with Redis and Kafka addresses
	redisAddr := fmt.Sprintf("localhost:%s", redisContainer.HostPort)
	kafkaAddr := fmt.Sprintf("localhost:%s", kafkaContainer.HostPort)

	testFunc(redisAddr, kafkaAddr)
}

// DependencyType specifies which dependencies are needed for a test
type DependencyType int

const (
	// NoDependencies indicates that no external dependencies are needed
	NoDependencies DependencyType = iota
	// RedisOnly indicates that only Redis is needed
	RedisOnly
	// KafkaOnly indicates that only Kafka is needed
	KafkaOnly
	// RedisAndKafka indicates that both Redis and Kafka are needed
	RedisAndKafka
)

// WithDependencies runs a test with the specified dependencies.
// It automatically sets up and tears down the required containers.
func WithDependencies(t interface {
	Helper()
	Skip(args ...interface{})
	Cleanup(f func())
	Errorf(format string, args ...interface{})
}, depType DependencyType, testFunc interface{}) {
	t.Helper()

	switch depType {
	case NoDependencies:
		tf, ok := testFunc.(func())
		if !ok {
			t.Errorf("Invalid function type for NoDependencies: expected func(), got %T", testFunc)
			return
		}
		tf()

	case RedisOnly:
		tf, ok := testFunc.(func(redisAddr string))
		if !ok {
			t.Errorf("Invalid function type for RedisOnly: expected func(string), got %T", testFunc)
			return
		}
		WithRedisOnly(t, tf)

	case KafkaOnly:
		tf, ok := testFunc.(func(kafkaAddr string))
		if !ok {
			t.Errorf("Invalid function type for KafkaOnly: expected func(string), got %T", testFunc)
			return
		}
		WithKafkaOnly(t, tf)

	case RedisAndKafka:
		tf, ok := testFunc.(func(redisAddr, kafkaAddr string))
		if !ok {
			t.Errorf("Invalid function type for RedisAndKafka: expected func(string, string), got %T", testFunc)
			return
		}
		WithTestDependencies(t, tf)

	default:
		t.Errorf("Unknown dependency type: %v", depType)
	}
}

// RunIntegrationTest runs an integration test with both Redis and Kafka containers,
// configuring them for the matchingo integration testing. This is specifically designed
// for integration tests that need to verify the end-to-end functionality.
func RunIntegrationTest(t interface {
	Helper()
	Skip(args ...interface{})
	Cleanup(f func())
	Errorf(format string, args ...interface{})
	Logf(format string, args ...interface{})
}, testFunc func(redisAddr, kafkaAddr string)) {
	t.Helper()

	// Log that we're starting containers for this test
	t.Logf("Starting Redis and Kafka containers for integration testing...")

	WithTestDependencies(t, func(redisAddr, kafkaAddr string) {
		// Log the addresses for debugging purposes
		t.Logf("Redis available at: %s", redisAddr)
		t.Logf("Kafka available at: %s", kafkaAddr)

		// Run the actual test function
		testFunc(redisAddr, kafkaAddr)

		// Additional logging on test completion
		t.Logf("Integration test completed, cleaning up containers...")
	})
}
