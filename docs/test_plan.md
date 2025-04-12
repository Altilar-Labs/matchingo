# Matchingo Test Plan: Core Engine & Order Management

## 1. Introduction

This document outlines the test plan for verifying the core functionalities of the Matchingo order book system. The focus is on the matching engine logic, handling of different order types, order book management, and the integrity of execution reporting via Kafka.

## 2. Objectives

*   Verify the correctness of the price-time priority matching algorithm.
*   Ensure accurate processing and state management for all supported order types (Market, Limit, Stop-Limit) and Time-in-Force options (GTC, IOC, FOK).
*   Validate the lifecycle management of orders and order books (creation, cancellation, retrieval, deletion).
*   Confirm that accurate and complete execution reports (`DoneMessage`) are published to the configured Kafka topic.
*   Prevent regressions in core functionality as new features are added or changes are made.
*   Ensure robustness and correct error handling for invalid inputs or states.

## 3. Scope

### In Scope

*   Core matching logic (price-time priority).
*   Processing of Limit, Market, and Stop-Limit orders.
*   Handling of GTC, IOC, and FOK Time-in-Force options.
*   Order book state management and retrieval.
*   Order creation, cancellation, and retrieval.
*   Partial and full order fills.
*   Execution report generation and publishing to Kafka.
*   Basic error handling for invalid operations.
*   Functionality with both Memory and Redis backends.

### Out of Scope (for this plan)

*   Detailed performance/load testing (covered separately).
*   UI/Client application testing beyond basic command execution.
*   Infrastructure testing (Kafka cluster health, Redis scaling, etc.).
*   Security testing.
*   Specific downstream consumer logic for Kafka messages.

## 4. Key Testing Areas & Scenarios

### 4.1. Matching Engine Core Logic

| Scenario ID | Description                                                                 | Expected Result                                                                                                |
| :---------- | :-------------------------------------------------------------------------- | :------------------------------------------------------------------------------------------------------------- |
| ME-001      | **Price Priority (Buy)**: Buy limit order matches lowest ask price first.     | Order matches against the lowest price ask level available.                                                    |
| ME-002      | **Price Priority (Sell)**: Sell limit order matches highest bid price first.  | Order matches against the highest price bid level available.                                                   |
| ME-003      | **Time Priority**: Orders at the same price level match based on FIFO.        | Older order at a price level is filled before newer orders at the same level.                                  |
| ME-004      | **Basic Match (Full Fill)**: Taker limit order fully fills a single maker order. | Both orders are removed from the book. Correct `DoneMessage` published for both taker and maker.                |
| ME-005      | **Basic Match (Partial Fill - Taker)**: Taker limit order partially fills a maker order. | Maker order quantity is reduced. Taker order is fully filled. Correct `DoneMessage` published.           |
| ME-006      | **Basic Match (Partial Fill - Maker)**: Taker limit order is larger than the maker order. | Maker order is fully filled and removed. Taker order quantity is reduced and remains/matches further. Correct `DoneMessage`. |
| ME-007      | **Multi-Level Match**: Taker order consumes liquidity across multiple price levels. | Order matches sequentially against orders at improving price levels until filled or liquidity exhausted. Correct `DoneMessage`. |
| ME-008      | **Self-Matching Prevention**: Attempt to submit an order that would match an existing order from the same user/source (if identifiable) or under specific conditions. | The system should prevent or handle self-matching according to defined rules (e.g., reject, cancel both). *Requires clarification on self-matching rules.* |

### 4.2. Order Types

| Scenario ID | Description                                                                    | Expected Result                                                                                                         |
| :---------- | :----------------------------------------------------------------------------- | :---------------------------------------------------------------------------------------------------------------------- |
| OT-001      | **Limit Order (Add)**: Create a limit order that does not cross the spread.    | Order is added to the correct side (bid/ask) at the specified price level. Order book state updates correctly.        |
| OT-002      | **Limit Order (Match - Taker)**: Create a limit order that crosses the spread.   | Order matches against existing orders according to ME rules. `DoneMessage` published.                                   |
| OT-003      | **Market Order (Full Fill)**: Create a market order that is fully filled by available liquidity. | Order matches against best available prices until filled. Order is not added to the book. `DoneMessage` published.        |
| OT-004      | **Market Order (Partial Fill)**: Create a market order larger than available liquidity. | Order fills against all available liquidity and the remaining quantity is effectively canceled (market orders don't rest). `DoneMessage` published. |
| OT-005      | **Market Order (No Liquidity)**: Create a market order when the opposite side is empty. | Order is rejected or effectively canceled immediately. `DoneMessage` might indicate zero fills.                   |
| OT-006      | **Stop-Limit Order (Placement)**: Place a buy stop-limit order above the market, sell stop-limit below. | Order is accepted but not added to the active order book. It should be retrievable via `get-order`.                   |
| OT-007      | **Stop-Limit Order (Activation - Buy)**: Market price trades at or above the buy stop price. | Stop-limit order becomes a standard limit order at its limit price and is added to the book/processed. `DoneMessage` may indicate activation. |
| OT-008      | **Stop-Limit Order (Activation - Sell)**: Market price trades at or below the sell stop price. | Stop-limit order becomes a standard limit order at its limit price and is added to the book/processed. `DoneMessage` may indicate activation. |
| OT-009      | **Stop-Limit Order (Cancellation)**: Cancel a pending (non-activated) stop-limit order. | Order is removed successfully.                                                                                         |

### 4.3. Order Book Management

| Scenario ID | Description                                                                      | Expected Result                                                                                                                            |
| :---------- | :------------------------------------------------------------------------------- | :----------------------------------------------------------------------------------------------------------------------------------------- |
| OB-001      | **Create Book (Memory)**: Create a new order book using the memory backend.        | `create-book` command succeeds. Book appears in `list-books`.                                                                                |
| OB-002      | **Create Book (Redis)**: Create a new order book using the Redis backend.          | `create-book` command succeeds (assuming Redis is running). Book appears in `list-books`.                                                    |
| OB-003      | **Create Book (Duplicate)**: Attempt to create a book with an existing name.       | Command fails with an "Already Exists" error.                                                                                              |
| OB-004      | **Get State (Empty)**: Get the state of an empty order book.                       | Command succeeds, showing empty bids and asks.                                                                                             |
| OB-005      | **Get State (Populated)**: Get the state of a book with bids and asks.             | Command succeeds, displaying correct price levels, aggregated quantities, and order counts per level, sorted correctly.                 |
| OB-006      | **List Books**: List all currently active order books.                             | Command succeeds, showing the names of all created books.                                                                                  |
| OB-007      | **Delete Book**: Delete an existing order book.                                    | Command succeeds. Book no longer appears in `list-books`. Operations on the deleted book fail with "Not Found".                         |
| OB-008      | **Delete Book (Non-Existent)**: Attempt to delete a book that does not exist.      | Command fails with a "Not Found" error.                                                                                                    |

### 4.4. Order Lifecycle & Execution

| Scenario ID | Description                                                                               | Expected Result                                                                                                                                |
| :---------- | :---------------------------------------------------------------------------------------- | :--------------------------------------------------------------------------------------------------------------------------------------------- |
| LC-001      | **TIF (GTC)**: Place a GTC limit order. Cancel it later.                                  | Order rests on the book until explicitly canceled. Cancellation succeeds. `DoneMessage` reflects cancellation.                                   |
| LC-002      | **TIF (IOC - Partial Match)**: Place an IOC limit order that partially matches.             | Order fills the available quantity immediately. The remaining quantity is canceled. `DoneMessage` reflects partial fill and cancellation.      |
| LC-003      | **TIF (IOC - Full Match)**: Place an IOC limit order that fully matches immediately.        | Order fills completely. `DoneMessage` reflects full fill.                                                                                    |
| LC-004      | **TIF (IOC - No Match)**: Place an IOC limit order that cannot match immediately.           | Order is canceled immediately. `DoneMessage` reflects cancellation with zero fills.                                                            |
| LC-005      | **TIF (FOK - Match Possible)**: Place an FOK limit order where the full quantity can be filled. | Order fills completely. `DoneMessage` reflects full fill.                                                                                    |
| LC-006      | **TIF (FOK - Match Not Possible)**: Place an FOK limit order where the full quantity cannot be filled. | Order is canceled immediately. `DoneMessage` reflects cancellation with zero fills.                                                            |
| LC-007      | **Cancel Order (Active Limit)**: Cancel a resting limit order.                            | `cancel-order` succeeds. Order removed from book state. `get-order` shows canceled status. `DoneMessage` (from cancellation) published.        |
| LC-008      | **Cancel Order (Non-Existent)**: Attempt to cancel an order ID that does not exist.       | `cancel-order` fails with a "Not Found" error.                                                                                                 |
| LC-009      | **Cancel Order (Already Filled)**: Attempt to cancel an order that has already fully filled. | `cancel-order` fails, possibly with "Not Found" or a specific "Already Filled" error.                                                       |
| LC-010      | **Get Order (Active)**: Retrieve details of an active resting order.                        | `get-order` succeeds, showing correct ID, side, type, price, quantity, status (e.g., "OPEN").                                                |
| LC-011      | **Get Order (Filled)**: Retrieve details of a fully filled order.                           | `get-order` succeeds, showing correct details and status (e.g., "FILLED").                                                                   |
| LC-012      | **Get Order (Canceled)**: Retrieve details of a canceled order.                             | `get-order` succeeds, showing correct details and status (e.g., "CANCELED").                                                                 |

### 4.5. Kafka Integration

| Scenario ID | Description                                                                          | Expected Result                                                                                                                                 |
| :---------- | :----------------------------------------------------------------------------------- | :---------------------------------------------------------------------------------------------------------------------------------------------- |
| KI-001      | **Kafka Message (Limit Order Fill)**: A limit order is partially or fully filled.      | A `DoneMessage` is published to Kafka containing correct taker/maker IDs, fill quantity, price, remaining quantities, and order statuses.       |
| KI-002      | **Kafka Message (Market Order Fill)**: A market order is partially or fully filled.    | A `DoneMessage` is published reflecting the executed quantity, average price (if applicable), remaining quantity (should be 0), and status.      |
| KI-003      | **Kafka Message (IOC/FOK Cancellation)**: An IOC or FOK order is canceled due to TIF. | A `DoneMessage` is published reflecting the cancellation and any partial fill that might have occurred (for IOC).                                 |
| KI-004      | **Kafka Message (Explicit Cancellation)**: An order is explicitly canceled via API.    | A `DoneMessage` might be published reflecting the cancellation status. *Verify if cancellations trigger Kafka messages.*                      |
| KI-005      | **Kafka Message (Stop Activation)**: A stop-limit order is activated.                  | A `DoneMessage` might be published indicating the order activation. *Verify if stop activations trigger Kafka messages.*                        |
| KI-006      | **Message Format Validation**: Consume messages from Kafka.                          | Messages correctly deserialize according to the `orderbook.proto` definition. All expected fields are present and populated appropriately. |

### 4.6. Error Handling

| Scenario ID | Description                                                                      | Expected Result                                                                                                        |
| :---------- | :------------------------------------------------------------------------------- | :--------------------------------------------------------------------------------------------------------------------- |
| EH-001      | **Invalid Quantity (Zero)**: Submit order with quantity 0.                       | Request fails with `InvalidArgument` error.                                                                            |
| EH-002      | **Invalid Quantity (Negative)**: Submit order with negative quantity.            | Request fails with `InvalidArgument` error.                                                                            |
| EH-003      | **Invalid Price (Limit Order)**: Submit limit order with negative or zero price. | Request fails with `InvalidArgument` error.                                                                            |
| EH-004      | **Invalid Type**: Submit order with an unrecognized type string.                 | Request fails with `InvalidArgument` error.                                                                            |
| EH-005      | **Duplicate Order ID**: Submit an order with an ID that already exists in the book. | Request fails with `AlreadyExists` error.                                                                              |
| EH-006      | **Operation on Non-Existent Book**: `create-order`, `get-state`, etc. on unknown book name. | Request fails with `NotFound` error.                                                                                   |
| EH-007      | **Operation on Non-Existent Order**: `cancel-order`, `get-order` on unknown order ID. | Request fails with `NotFound` error.                                                                                   |

### 4.7. Concurrency (Basic)

| Scenario ID | Description                                                                                   | Expected Result                                                                                                                  |
| :---------- | :-------------------------------------------------------------------------------------------- | :------------------------------------------------------------------------------------------------------------------------------- |
| CC-001      | **Concurrent Additions**: Multiple clients add non-matching limit orders concurrently.        | All orders are added correctly to the book. Final book state reflects all added orders. No deadlocks or race conditions observed. |
| CC-002      | **Concurrent Matching Orders**: Submit crossing buy and sell orders from different clients concurrently. | Orders match correctly according to price-time priority. Final book state is consistent. No deadlocks or lost updates.         |
| CC-003      | **Concurrent Add/Cancel**: One client adds orders while another cancels orders concurrently. | Cancellations only affect existing orders. Additions work correctly. Final state is consistent.                                |

## 5. Test Execution Strategy

*   **Manual Testing**: Use the `orderbook-client` CLI to execute scenarios defined above against a locally running server. Verify outputs and Kafka messages (using a Kafka consumer tool).
*   **Automated Testing**: Develop unit tests (`pkg/core`, `pkg/server`) and integration tests that cover these scenarios programmatically. Leverage existing test files (`_test.go`) and add new ones as needed.
*   **Backend Variation**: Key scenarios (especially those involving state) should be tested with both `memory` and `redis` backends.

## 6. Tools

*   `orderbook-server`: The gRPC server binary.
*   `orderbook-client`: The gRPC client CLI binary.
*   Go testing framework (`go test`).
*   Kafka Consumer Tool (e.g., `kcat`, custom Go consumer) to inspect messages.
*   Redis client (`redis-cli`) to inspect state for Redis backend tests (optional). 