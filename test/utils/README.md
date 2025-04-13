# Testing Utilities

This package provides utilities to help with testing, particularly for tests that require external dependencies like Redis and Kafka.

## Docker Container Management

The `docker.go` file contains utilities for starting and stopping Docker containers for Redis and Kafka, making it easy to run integration tests without needing manual setup of these services.

### Available Functions

#### Basic Container Management

- `StartRedisContainer(ctx context.Context) (*DockerContainer, error)` - Starts a Redis container and waits for it to be ready.
- `StartKafkaContainer(ctx context.Context) (*DockerContainer, error)` - Starts a Kafka container (and a linked Zookeeper container) and waits for it to be ready.
- `(*DockerContainer) Stop(ctx context.Context) error` - Stops and removes a Docker container.

#### Test Helpers

- `WithRedisOnly(t TestingT, testFunc func(redisAddr string))` - Runs a test with only Redis dependency.
- `WithKafkaOnly(t TestingT, testFunc func(kafkaAddr string))` - Runs a test with only Kafka dependency.
- `WithTestDependencies(t TestingT, testFunc func(redisAddr, kafkaAddr string))` - Runs a test with both Redis and Kafka dependencies.

#### Flexible Dependency Management

For more flexibility, use the `WithDependencies` function that lets you specify which dependencies your test needs:

```go
WithDependencies(t, depType DependencyType, testFunc interface{})
```

The `DependencyType` can be:
- `NoDependencies` - For tests without external dependencies
- `RedisOnly` - For tests requiring only Redis
- `KafkaOnly` - For tests requiring only Kafka
- `RedisAndKafka` - For tests requiring both Redis and Kafka

The `testFunc` parameter should be a function matching the dependency type:
- `func()` for `NoDependencies`
- `func(redisAddr string)` for `RedisOnly`
- `func(kafkaAddr string)` for `KafkaOnly`
- `func(redisAddr, kafkaAddr string)` for `RedisAndKafka`

### Usage Examples

Basic usage with Redis only:

```go
func TestWithRedis(t *testing.T) {
    testutil.WithRedisOnly(t, func(redisAddr string) {
        // Connect to Redis using redisAddr
        client := redis.NewClient(&redis.Options{
            Addr: redisAddr,
        })
        defer client.Close()
        
        // Run your test...
    })
}
```

Using the flexible dependency manager:

```go
func TestFlexibleDependencies(t *testing.T) {
    testutil.WithDependencies(t, testutil.RedisAndKafka, func(redisAddr, kafkaAddr string) {
        // Connect to Redis and Kafka
        // Run your test...
    })
}
```

## Dependency Checks

The `dependencies.go` file provides functions to skip tests if required dependencies are not available:

- `SkipIfRedisUnavailable(t *testing.T, redisAddr string)` - Skip test if Redis is not available
- `SkipIfKafkaUnavailable(t *testing.T, kafkaAddr string)` - Skip test if Kafka is not available  
- `SkipIfDependenciesUnavailable(t *testing.T, redisAddr, kafkaAddr string)` - Skip test if either dependency is unavailable

These functions are useful for tests that expect the services to be already running, rather than starting the services themselves. 
