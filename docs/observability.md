# Observability in Matchingo

This document outlines how metrics, traces, and logs are instrumented in the Matchingo codebase, providing guidance for developers and operators on monitoring and troubleshooting the system.

---

## 1. Logging

### Library Used
- [zerolog](https://github.com/rs/zerolog) for structured, leveled logging.

### Configuration
- Logging is configured via the `logging.Config` struct, which sets log level, pretty printing, and output destination.
- Global logging setup is performed by `logging.Setup(cfg)`.
- Log level and format are configurable via the main config (see `config/config.go`).

### Usage Patterns
- Context-aware logging is implemented. The logger can extract request IDs and gRPC metadata from the context (see `logging.FromContext`).
- gRPC interceptors (`UnaryServerInterceptor` and `StreamServerInterceptor`) automatically log request details, durations, and status codes for all gRPC calls.
- Logs include method names, status codes, durations, and errors where applicable.
- Application components (e.g., order book manager) use context-derived loggers for consistent, correlated logs.

---

## 2. Metrics

### Library Used
- [OpenTelemetry Metrics](https://opentelemetry.io/docs/instrumentation/go/)

### Metrics Instrumented
- Metrics are defined in `pkg/otel/metrics.go` and include:
  - `grpc.server.duration` (histogram): gRPC request latency
  - `requests_total` (counter): total number of gRPC requests
  - `requests_in_flight` (up-down counter): concurrent requests
  - `error_total` (counter): total errors
  - `goroutines_count` (up-down counter): Go runtime goroutine count

### Instrumentation Points
- gRPC server interceptors (`MetricsServerInterceptor` and `MetricsStreamServerInterceptor` in `pkg/otel/grpc.go`) automatically record metrics for all gRPC calls:
  - Request start/end, duration, in-flight count, and status code are tracked.
  - Goroutine count is periodically updated in the background.
- Metrics are labeled with method names and status codes for detailed analysis.

---

## 3. Tracing

### Library Used
- [OpenTelemetry Tracing](https://opentelemetry.io/docs/instrumentation/go/)

### Instrumentation Points
- **Explicit distributed tracing** is now implemented in `pkg/otel/order_tracing.go`:
  - The function `StartOrderSpan(ctx, name, attrs...)` creates spans for key operations such as order creation, processing, matching, and sending to Kafka. 
  - Span names are standardized as `create_order`, `process_order`, `match_order`, and `send_to_kafka`.
  - Span attributes include: `order.id`, `order.side`, `order.type`, `order.quantity`, `order.price`, `order.status`, `order.executed_quantity`, `order.remaining_quantity`, and `trade.count`.
  - The helper `AddAttributes(span, attrs...)` can be used to enrich spans with additional metadata.
- **OpenTelemetry initialization** (`pkg/otel/otel.go`):
  - Supports both the order service and matching engine, each with a dedicated resource and tracer provider.
  - The system can be configured to export traces and metrics to an OTLP collector, with resource attributes such as service name and version.
- **Trace context propagation** is set up for gRPC and Kafka, ensuring distributed traces can be correlated across services.

### How to Extend
- Use `StartOrderSpan` for any new business logic that should be traced.
- Add custom attributes using `AddAttributes` for richer trace data.
- Ensure all asynchronous and background tasks propagate context to maintain trace continuity.

---

## 3a. Recent Improvements in Tracing (April 2025)
- Tracing is now first-class: Key order lifecycle events (creation, processing, matching, Kafka publishing) are traced with explicit spans.
- Span and attribute naming conventions are standardized for easier querying and analysis in observability backends.
- OpenTelemetry setup is modular, supporting multi-service deployments with distinct resources and tracer providers.
- The codebase is ready for advanced distributed tracing scenarios, including multi-service and asynchronous workflows.

---

## 4. Summary Table

| Aspect     | Technology         | Where Instrumented                | Key Features                         |
|------------|--------------------|-----------------------------------|--------------------------------------|
| Logging    | zerolog            | pkg/logging, gRPC interceptors    | Structured, context-aware, leveled   |
| Metrics    | OpenTelemetry      | pkg/otel, gRPC interceptors       | Latency, traffic, errors, goroutines |
| Tracing    | OpenTelemetry      | pkg/otel, queue, (extendable)     | Context propagation, ready for spans |

---

## 5. Extending Observability
- To add new metrics, extend `GRPCServerMetrics` in `pkg/otel/metrics.go` and update interceptors.
- For custom traces, use the `StartOrderSpan` and `AddAttributes` helpers in `pkg/otel/order_tracing.go`, or use `go.opentelemetry.io/otel/trace` directly for more advanced use cases.
- To enrich logs, add more context fields via `zerolog.With()` and pass context appropriately.
- For new services, follow the resource and tracer provider setup in `pkg/otel/otel.go` for consistent observability across the stack.

---

## 6. References
- [OpenTelemetry for Go](https://opentelemetry.io/docs/instrumentation/go/)
- [zerolog Documentation](https://github.com/rs/zerolog)

---

This document reflects the state of observability as of April 2025. For changes or updates, review the code under `pkg/logging` and `pkg/otel`.
