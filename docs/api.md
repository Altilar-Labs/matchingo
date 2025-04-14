# Matchingo gRPC API Documentation

## Overview

This document describes the gRPC API for interacting with the Matchingo order book service. This API allows clients to manage order books, submit orders, cancel orders, and retrieve state information. It's designed for programmatic access, suitable for trading bots, market makers, or other automated systems.

## Getting Started

*   **Service:** `matchingo.OrderBookService`
*   **Default Endpoint:** The server typically listens on `localhost:50051` (this might vary depending on deployment).
*   **Protocol:** gRPC
*   **Proto Definition:** `pkg/api/proto/orderbook.proto`

Clients need gRPC libraries for their respective language and the generated code from the `.proto` file to interact with the service.

## Service: `OrderBookService`

### RPC Methods

---

#### `CreateOrderBook`

Creates a new order book for a specific trading instrument.

*   **Request:** `CreateOrderBookRequest`
    *   `name` (string, required): A unique identifier for the order book (e.g., "BTC-USD").
*   **Response:** `CreateOrderBookResponse` (empty)
*   **Errors:**
    *   `codes.InvalidArgument`: If the name is empty.
    *   `codes.AlreadyExists`: If an order book with the given name already exists.
*   **Side Effects:** None.
*   **CLI Example:**
    ```bash
    orderbook-client --cmd=create-book --book=BTC-USD
    ```

---

#### `DeleteOrderBook`

Deletes an existing order book.

*   **Request:** `DeleteOrderBookRequest`
    *   `name` (string, required): The unique identifier of the order book to delete.
*   **Response:** `DeleteOrderBookResponse` (empty)
*   **Errors:**
    *   `codes.InvalidArgument`: If the name is empty.
    *   `codes.NotFound`: If no order book with the given name exists.
*   **Side Effects:** All orders within the book are implicitly removed.
*   **CLI Example:**
    ```bash
    orderbook-client --cmd=delete-book --book=BTC-USD
    ```

---

#### `ListOrderBooks`

Lists all currently active order books.

*   **Request:** `ListOrderBooksRequest` (empty)
*   **Response:** `ListOrderBooksResponse`
    *   `books` (repeated `OrderBookInfo`): A list of active order books.
        *   `OrderBookInfo`: Contains `name` (string).
*   **Errors:**
    *   `codes.Internal`: For unexpected server errors during retrieval.
*   **Side Effects:** None.
*   **CLI Example:**
    ```bash
    orderbook-client --cmd=list-books
    ```

---

#### `GetOrderBookState`

Retrieves the current aggregated state (price levels) of a specific order book.

*   **Request:** `GetOrderBookStateRequest`
    *   `name` (string, required): The identifier of the order book.
*   **Response:** `GetOrderBookStateResponse`
    *   `bids` (repeated `PriceLevel`): A list of aggregated bid levels, sorted highest price first.
    *   `asks` (repeated `PriceLevel`): A list of aggregated ask levels, sorted lowest price first.
*   **Errors:**
    *   `codes.InvalidArgument`: If the name is empty.
    *   `codes.NotFound`: If no order book with the given name exists.
*   **Side Effects:** None.
*   **CLI Example:**
    ```bash
    orderbook-client --cmd=get-state --book=BTC-USD
    ```

---

#### `CreateOrder`

Submits a new order to a specific order book.

*   **Request:** `CreateOrderRequest`
    *   `book_name` (string, required): The identifier of the target order book.
    *   `order` (`Order`, required): The order details (see `Order` definition below).
*   **Response:** `CreateOrderResponse`
    *   `order_id` (string): The unique ID assigned to the created order.
*   **Errors:**
    *   `codes.InvalidArgument`: If `book_name` is empty, or if `order` details are invalid (e.g., zero/negative quantity, zero/negative limit price, zero/negative stop price, invalid side/type/TIF, missing required fields for type).
    *   `codes.NotFound`: If the specified `book_name` does not exist.
    *   `codes.AlreadyExists`: If an order with the same `id` already exists in the book.
    *   `codes.Internal`: For unexpected server errors during processing.
*   **Side Effects:**
    *   May result in immediate matching and trade execution.
    *   Publishes a `DoneMessage` to the configured Kafka topic for:
        *   Each fill (partial or full).
        *   Cancellation due to IOC/FOK Time-in-Force constraints.
        *   *(Note: Activation of stop orders and explicit cancellations via `CancelOrder` do NOT currently trigger a Kafka message based on testing)*.
*   **CLI Examples:**
    *   **Limit Buy:**
        ```bash
        orderbook-client --cmd=create-order --book=BTC-USD --id=buy001 --side=buy --type=limit --qty=0.5 --price=50000 --tif=GTC
        ```
    *   **Market Sell:**
        ```bash
        orderbook-client --cmd=create-order --book=BTC-USD --id=sell002 --side=sell --type=market --qty=1.0
        ```
    *   **Stop-Limit Buy:**
        ```bash
        orderbook-client --cmd=create-order --book=BTC-USD --id=stop003 --side=buy --type=stop-limit --qty=0.1 --price=51000 --stop=50950 --tif=GTC
        ```

---

#### `CancelOrder`

Cancels a pending (resting) order.

*   **Request:** `CancelOrderRequest`
    *   `book_name` (string, required): The identifier of the order book containing the order.
    *   `order_id` (string, required): The unique ID of the order to cancel.
*   **Response:** `CancelOrderResponse` (empty)
*   **Errors:**
    *   `codes.InvalidArgument`: If `book_name` or `order_id` is empty.
    *   `codes.NotFound`: If the `book_name` does not exist or the `order_id` does not exist within that book (or was already fully filled/canceled).
    *   `codes.Internal`: For unexpected server errors during cancellation.
*   **Side Effects:** Removes the specified order from the book if it's active. *(Note: Does NOT currently publish a Kafka message)*.
*   **CLI Example:**
    ```bash
    orderbook-client --cmd=cancel-order --book=BTC-USD --id=buy001
    ```

---

#### `GetOrder`

Retrieves the details and current status of a specific order.

*   **Request:** `GetOrderRequest`
    *   `book_name` (string, required): The identifier of the order book containing the order.
    *   `order_id` (string, required): The unique ID of the order to retrieve.
*   **Response:** `GetOrderResponse`
    *   `order` (`Order`): The details of the requested order, including its current `status`.
*   **Errors:**
    *   `codes.InvalidArgument`: If `book_name` or `order_id` is empty.
    *   `codes.NotFound`: If the `book_name` does not exist or the `order_id` does not exist within that book.
*   **Side Effects:** None.
*   **CLI Example:**
    ```bash
    orderbook-client --cmd=get-order --book=BTC-USD --id=buy001
    ```

---

## Message Definitions

#### `Order`

Represents an order in the system.

*   `id` (string): Unique identifier for the order (client-provided or generated).
*   `side` (`Side` enum): `BUY` or `SELL`.
*   `type` (`OrderType` enum): `MARKET`, `LIMIT`, `STOP_LIMIT`.
*   `quantity` (string): The total quantity of the order (decimal string).
*   `price` (string): The limit price for LIMIT or STOP_LIMIT orders (decimal string). Ignored for MARKET orders.
*   `stop_price` (string): The price at which a STOP_LIMIT order becomes active (decimal string). Only used for STOP_LIMIT orders.
*   `time_in_force` (`TimeInForce` enum): `GTC` (Good 'Til Canceled), `IOC` (Immediate Or Cancel), `FOK` (Fill Or Kill). Defaults typically to GTC if not specified or applicable.
*   `status` (`OrderStatus` enum): Current status, e.g., `OPEN`, `FILLED`, `CANCELED`, `PENDING` (for non-triggered stops). Read-only field returned by `GetOrder`.
*   `filled_quantity` (string): Quantity that has been executed. Read-only field returned by `GetOrder`.
*   `created_at` (google.protobuf.Timestamp): Time the order was created/received. Read-only.
*   `updated_at` (google.protobuf.Timestamp): Time the order was last modified (e.g., filled, canceled). Read-only.

#### `PriceLevel`

Represents an aggregated price level in the order book state.

*   `price` (string): The price level (decimal string).
*   `quantity` (string): The total quantity available at this price level (decimal string).
*   `order_count` (int64): The number of individual orders resting at this price level.

#### `DoneMessage` (Published to Kafka)

Represents the final state or a significant event (like a fill) for an order. This is the primary way for external consumers to track trade executions.

*   `order_id` (string): The ID of the order this message relates to.
*   `book_name` (string): The order book this order belongs to.
*   `status` (`OrderStatus` enum): The status of the order after the event (e.g., `PARTIALLY_FILLED`, `FILLED`, `CANCELED`).
*   `reason` (string): A code or description indicating why the order is done/changed (e.g., "filled", "ioc_canceled", "fok_canceled").
*   `price` (string): The price at which the last fill occurred (if applicable).
*   `quantity` (string): The quantity executed in the last fill (if applicable).
*   `remaining_quantity` (string): The quantity remaining for the order after this event.
*   `timestamp` (google.protobuf.Timestamp): Timestamp of the event.
*   `trade_id` (string, optional): Unique ID for the trade if this message represents a fill.
*   `taker_order_id` (string, optional): Order ID of the taker order in a fill event.
*   `maker_order_id` (string, optional): Order ID of the maker order in a fill event.

## Kafka Integration

The server publishes `DoneMessage` records to a configured Kafka topic whenever an order reaches a final state or experiences a fill. Consumers should monitor this topic to receive real-time updates on order executions and cancellations triggered by TIF.

*   **Key Fields:** `order_id`, `status`, `reason`, `price`, `quantity`, `remaining_quantity`, `trade_id`, `taker_order_id`, `maker_order_id`.
*   **Events Triggering Messages:**
    *   Full order fills.
    *   Partial order fills.
    *   IOC order partial fills (followed by cancellation).
    *   IOC order cancellation (if no fill).
    *   FOK order cancellation (if full fill not possible).
*   **Events NOT Triggering Messages (Current Implementation):**
    *   Explicit cancellation via `CancelOrder` RPC.
    *   Activation of a `STOP_LIMIT` order.

## Error Handling

The API uses standard gRPC status codes:

*   `InvalidArgument`: Invalid request parameters (e.g., bad format, missing required fields, zero quantity/price).
*   `NotFound`: Entity not found (e.g., unknown order book name, unknown order ID).
*   `AlreadyExists`: Entity creation failed because it already exists (e.g., duplicate order book name, duplicate order ID).
*   `Internal`: Unexpected server-side error.

## Known Issues / Limitations

Refer to `docs/testing.md` for details, but key points for API consumers include:

*   Potential inaccuracies or bugs related to IOC/FOK partial fills and cancellations.
*   Potential issues with stop order activation and cancellation logic.
*   Kafka messages are not currently sent for explicit cancellations or stop activations. 