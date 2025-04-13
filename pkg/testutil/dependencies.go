package testutil

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/segmentio/kafka-go"
)

// SkipIfRedisUnavailable skips the test if Redis is unavailable on the specified address
func SkipIfRedisUnavailable(t *testing.T, redisAddr string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	_, err := client.Ping(ctx).Result()
	if err != nil {
		t.Skipf("Skipping test: Redis not available at %s - %v", redisAddr, err)
	}

	_ = client.Close()
}

// SkipIfKafkaUnavailable skips the test if Kafka is unavailable on the specified address
func SkipIfKafkaUnavailable(t *testing.T, kafkaAddr string) {
	t.Helper()

	conn, err := net.DialTimeout("tcp", kafkaAddr, 2*time.Second)
	if err != nil {
		t.Skipf("Skipping test: Kafka not available at %s - %v", kafkaAddr, err)
		return
	}
	_ = conn.Close()

	// Additional check - try to create a reader to verify the Kafka broker is responsive
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{kafkaAddr},
		Topic:       "matchingo-test", // Assume this topic exists from our setup script
		MinBytes:    10e3,
		MaxBytes:    10e6,
		MaxWait:     500 * time.Millisecond,
		StartOffset: kafka.LastOffset,
	})
	defer reader.Close()

	// Try to fetch a message - we don't expect to get one, just checking if Kafka responds
	_, err = reader.FetchMessage(ctx)
	if err != nil && err != context.DeadlineExceeded && err.Error() != "EOF" {
		// If error is not just a timeout or EOF, it might indicate a deeper issue
		t.Skipf("Skipping test: Kafka at %s is not responding correctly - %v", kafkaAddr, err)
	}
}

// SkipIfDependenciesUnavailable skips the test if either Redis or Kafka is unavailable
func SkipIfDependenciesUnavailable(t *testing.T, redisAddr, kafkaAddr string) {
	t.Helper()
	SkipIfRedisUnavailable(t, redisAddr)
	SkipIfKafkaUnavailable(t, kafkaAddr)
}
