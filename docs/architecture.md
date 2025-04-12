# Matchingo System Architecture

This document outlines the high-level architecture of the Matchingo order book system.

## Overview

Matchingo is a gRPC-based order book matching engine designed for high performance. It allows clients to create and manage multiple order books, submit various order types, and receive execution reports. A key architectural feature is the asynchronous publishing of detailed order execution results to a Kafka topic.

## Key Components

1.  **gRPC Server (`cmd/server`)**:
    *   The main entry point for the application.
    *   Initializes logging, configuration, and the `OrderBookManager`.
    *   Starts the gRPC server, exposing the `OrderBookService`.

2.  **gRPC Client (`cmd/client`)**:
    *   A command-line interface for interacting with the gRPC server.
    *   Provides commands for creating books, submitting orders, canceling orders, and viewing state.

3.  **API (`pkg/api`)**:
    *   Defines the gRPC service (`OrderBookService`) and message types (e.g., `Order`, `CreateOrderRequest`, `OrderBookStateResponse`, `DoneMessage`) using Protocol Buffers (`.proto` files).
    *   Contains generated Go code for the gRPC client and server stubs.

4.  **Server Implementation (`pkg/server`)**:
    *   `GRPCOrderBookService`: Implements the gRPC service handlers defined in `pkg/api`. It receives client requests, validates them, interacts with the `OrderBookManager`, and sends responses.
    *   `OrderBookManager`: Manages the lifecycle of multiple `core.OrderBook` instances. It handles the creation, retrieval, and deletion of order books, supporting different backends (memory, Redis).

5.  **Core Engine (`pkg/core`)**:
    *   `OrderBook`: Contains the central matching logic. It receives orders, processes them based on type (market, limit, stop-limit), and interacts with its configured `OrderBookBackend`.
    *   `Order`: Represents different order types and their properties.
    *   `OrderBookBackend`: An interface defining the operations required for storing and retrieving order book data (e.g., adding/removing orders, getting price levels). This allows plugging in different storage mechanisms.
    *   **Kafka Publishing**: After processing an order, the `OrderBook` is responsible for constructing a `DoneMessage` and publishing it to Kafka via the `MessageSender` interface.

6.  **Backends (`pkg/backend`)**:
    *   `memory`: An in-memory implementation of the `OrderBookBackend` interface. Fast but volatile.
    *   `redis`: A Redis-based implementation of the `OrderBookBackend` interface. Provides persistence.

7.  **Messaging (`pkg/messaging`)**:
    *   `MessageSender`: An interface defining the contract for sending messages (specifically `DoneMessage`). This decouples the core engine from specific message queue implementations.
    *   `DoneMessage`, `Trade`: Structs defining the format of messages sent to the queue, representing order execution results.

8.  **Kafka Queue (`pkg/db/queue`)**:
    *   `QueueMessageSender`: An implementation of the `MessageSender` interface using the `sarama` Kafka client library. It serializes `DoneMessage` into protobuf format and sends it to a configured Kafka topic.
    *   `QueueMessageConsumer`: Provides functionality to consume messages from the Kafka topic (potentially for use by other downstream services).

9.  **Logging (`pkg/logging`)**:
    *   Provides centralized logging configuration and utilities using the `zerolog` library.

## Data Flow: Order Creation and Execution

1.  A client sends a `CreateOrderRequest` via gRPC to the `GRPCOrderBookService`.
2.  The service handler validates the request and uses the `OrderBookManager` to retrieve the target `core.OrderBook` instance.
3.  The request details are used to create a `core.Order` object.
4.  The `core.OrderBook.Process()` method is called with the new order.
5.  The `OrderBook` interacts with its `OrderBookBackend` to fetch existing orders and perform matching according to price-time priority.
6.  Matched trades are recorded. Order quantities are updated or orders are removed from the book via the backend.
7.  A `core.Done` object summarizing the execution (fills, remaining quantity, cancellations) is created.
8.  The `core.Done` object is converted into a `messaging.DoneMessage`.
9.  The `core.OrderBook` uses the `queue.QueueMessageSender` to serialize the `DoneMessage` to protobuf and publish it to the configured Kafka topic.
10. The `GRPCOrderBookService` sends a basic confirmation response back to the client via gRPC.
11. Downstream services (external or potentially other parts of Matchingo not yet implemented) can consume the detailed `DoneMessage` from Kafka for further processing.

## Diagram (Conceptual)

```
+--------+       gRPC        +--------------------------+       +--------------------+       +--------------------+
| Client | <-------------->  | GRPCOrderBookService     | ----> | OrderBookManager   | ----> | core.OrderBook     |
+--------+                   | (pkg/server)             |       | (pkg/server)       |       | (pkg/core)         |
                             +--------------------------+       +--------------------+       +----------+---------+
                                                                                                        |
                                                                                                        | Uses
                                                                                                        v
                                                                    +-------------------------+     +--------------------+
                                                                    | MessageSender Interface | <-- | queue.QueueSender  |
                                                                    | (pkg/messaging)         |     | (pkg/db/queue)     |
                                                                    +-------------------------+     +----------+---------+
                                                                                                               | Publishes
                                                                                                               v
                                     +------------------------+                              +--------------------+
                                     | OrderBookBackend       | <--------------------------- | Kafka Topic        |
                                     | (memory/redis)         | Used by core.OrderBook       | (e.g., test-msg-queue) |
                                     | (pkg/backend)          |                              +--------------------+
                                     +------------------------+                                        | Consumed by
                                                                                                       v
                                                                                             +--------------------+
                                                                                             | Downstream Services|
                                                                                             +--------------------+

```

This architecture allows for a decoupled and scalable system where the core matching engine can operate efficiently, and detailed execution reports are handled asynchronously via a message queue. 