# Performance Load Test Analysis (`cmd/loadtest`)

This document outlines the behavior of the performance load test located in `cmd/loadtest/main.go`.

## Overview

The primary goal of the test is to stress the gRPC server by sending a high volume of orders. It measures the time taken to process a large number of order creation requests and is configured to maximize the probability of order matching.

## Execution Flow (`main` function)

The test executes the following steps:

1.  **Configuration:** Parses the `-grpc-addr` command-line flag for the server address (defaults to `localhost:50051`).
2.  **Setup:**
    *   Establishes a gRPC connection to the server.
    *   Creates a temporary order book named `load-test-order-book` using the `MEMORY` backend via the `CreateOrderBook` RPC.
    *   Sets up signal handling for graceful shutdown on interrupt (Ctrl+C).
    *   Initializes a rate limiter (`rate.Limiter`) to control concurrency (`maxConcurrentReqs`, default 100).
    *   Initializes a wait group (`sync.WaitGroup`) and an error channel.
    *   **Initializes a thread-safe HDR histogram and atomic counters for real-time metrics.**
3.  **Load Generation Loop:**
    *   Records the start time.
    *   Starts a configurable number of worker goroutines (`numWorkers`, default 10000).
    *   Each worker runs a loop to send a fixed number of orders (`ordersPerWorker`, default 100):
        *   Waits for the rate limiter to allow the next request.
        *   Calls `generateOrder` to create the order details.
        *   Sends the order using the `CreateOrder` gRPC call.
        *   **Records the latency of each request into the shared HDR histogram (protected by a mutex for thread safety).**
        *   Collects any errors from `CreateOrder` into the error channel.
4.  **Synchronization and Reporting:**
    *   Waits for all worker goroutines to complete using the wait group.
    *   Calculates and logs the total test duration.
    *   Closes and drains the error channel, collecting all errors.
    *   Logs the total number of orders attempted (`numWorkers` * `ordersPerWorker`).
    *   Logs the total count of errors encountered during order submission.
    *   **Logs real-time interval metrics every 30 seconds, including request count, error count, requests per second (RPS), and latency percentiles (p50, p75, p90, p95) calculated from the HDR histogram.**
5.  **Cleanup:**
    *   Deletes the `load-test-order-book` using the `DeleteOrderBook` RPC.
    *   Logs success or failure of the cleanup.
    *   Exits with status 1 if any errors occurred during the load generation phase, 0 otherwise.

---

## Observability and Real-Time Metrics

The load test now features enhanced observability:

- **Latency Measurement:**
  - Uses a single `hdrhistogram.Histogram` (from `github.com/HdrHistogram/hdrhistogram-go`) to record per-request latencies (in microseconds).
  - The histogram is protected by a `sync.Mutex` to ensure thread safety across concurrent goroutines.
  - Each worker records the latency of every order submission.

- **Interval Metrics Logging:**
  - A background goroutine logs interval metrics every 30 seconds during the test run.
  - Metrics include:
    - Number of requests and errors in the interval
    - Requests per second (RPS)
    - Latency percentiles: p50, p75, p90, p95 (computed from a snapshot of the histogram)
  - After each interval, the histogram is reset for the next interval, ensuring metrics reflect recent activity.

- **Thread Safety:**
  - All accesses to the shared histogram are guarded by a mutex to avoid race conditions.

This approach provides real-time visibility into system performance under load, helping to quickly identify bottlenecks and regressions.

## Order Generation (`generateOrder` function)

*   **Order ID:** Sequentially generated based on worker ID and order number within the worker (e.g., `order-0`, `order-1`, ...).
*   **Order Book Name:** Uses the created `load-test-order-book`.
*   **Side:** Randomly chosen between `BUY` and `SELL` (50% probability each).
*   **Price:** Fixed at `"100.00"`.
*   **Quantity:** Fixed at `"10.00"`.
*   **Order Type:** Always `LIMIT`.
*   **Time In Force:** Always `GTC` (Good 'Til Canceled).

This fixed price and quantity strategy ensures that BUY and SELL orders are highly likely to match and execute.

## API Usage (`pkg/api/proto/orderbook.proto`)

The test primarily interacts with the following gRPC components:

*   **Service:** `OrderBookService`
*   **RPCs:**
    *   `CreateOrderBook`: To set up the test environment.
    *   `CreateOrder`: To submit the load test orders.
    *   `DeleteOrderBook`: To clean up the test environment.
*   **Key Messages:**
    *   `CreateOrderRequest`: Used to structure the order data sent to the server. Fields used: `order_book_name`, `order_id`, `side`, `quantity`, `price`, `order_type`, `time_in_force`.
    *   `OrderResponse`: Received after `CreateOrder`, but its details (like status or fills) are not currently processed or analyzed by the load test.

## Limitations

*   **No State Check / Match Verification:** The test doesn't query the order book state (`GetOrderBookState`), check individual order statuses (`GetOrder`), or analyze the `OrderResponse` fills to explicitly verify or quantify the number of matches that actually occurred. It assumes matches happen due to the fixed price/quantity but doesn't measure them.
*   **Simplified Market Dynamics:** Uses only LIMIT GTC orders at a single price point, which doesn't fully represent diverse, real-world market activity.

## Next Steps

*   Consider adding logic to analyze the `OrderResponse` from `CreateOrder` to count filled orders or trades.
*   Alternatively, periodically call `GetOrderBookState` or `GetOrder` to observe the matching process and quantify results.
*   Introduce more varied order generation (e.g., different prices, quantities, order types) to simulate more complex scenarios if needed. 