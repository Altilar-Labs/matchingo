# Matchingo Testing Guide

## 1. Introduction

This document outlines the test plan and current status for verifying the core functionalities of the Matchingo order book system. The focus is on the matching engine logic, handling of different order types, order book management, backend interactions, gRPC service operations, and the integrity of execution reporting via Kafka.

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
*   Order creation, cancellation, and retrieval via gRPC service.
*   Partial and full order fills.
*   Execution report generation and publishing to Kafka (via integration tests).
*   Basic error handling for invalid operations at core and gRPC levels.
*   Functionality with both Memory and Redis backends (unit and integration tests).

### Out of Scope

*   Detailed performance/load testing (covered separately).
*   UI/Client application testing beyond basic `orderbook-client` command execution.
*   Infrastructure testing (Kafka cluster health, Redis scaling, etc.).
*   Security testing.
*   Specific downstream consumer logic for Kafka messages.
*   Concurrency stress testing (basic race detection is run).

## 4. Key Testing Areas & Scenarios

This section details specific test scenarios and their current automated test coverage.

*   ✅: Covered by automated tests.
*   ❌: Not covered or partially covered by automated tests / Requires manual verification / TBD.
*   ⚠️: Covered, but with known issues or specific behavior notes.

### 4.1. Matching Engine Core Logic (`pkg/core/orderbook_test.go`)

| Scenario ID | Description                                                                 | Expected Result                                                                                                | Coverage                                                                                                                                                                        | Status |
| :---------- | :-------------------------------------------------------------------------- | :------------------------------------------------------------------------------------------------------------- | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | :----: |
| ME-001      | **Price Priority (Buy)**: Buy limit order matches lowest ask price first.     | Order matches against the lowest price ask level available.                                                    | `orderbook_test.go:TestPriceTimePriority`, `orderbook_test.go:TestMultiLevelMatching` (implicit), `marketorder_test.go:TestMarketOrderMatch` (implicit, via integration) |   ✅   |
| ME-002      | **Price Priority (Sell)**: Sell limit order matches highest bid price first.  | Order matches against the highest price bid level available.                                                   | `orderbook_test.go:TestPriceTimePriority` (implicit), `orderbook_test.go:TestMultiLevelMatching` (implicit)                                                                        |   ✅   |
| ME-003      | **Time Priority**: Orders at the same price level match based on FIFO.        | Older order at a price level is filled before newer orders at the same level.                                  | `orderbook_test.go:TestPriceTimePriority`                                                                                                                                         |   ✅   |
| ME-004      | **Basic Match (Full Fill)**: Taker limit order fully fills a single maker order. | Both orders are removed from the book. Correct `DoneMessage` published for both taker and maker.                | `orderbook_test.go:TestCompleteOrderExecution` (concept), `integration_v2_test.go:TestIntegrationV2_IOC_FOK/FOK_Success`                                                      |   ✅   |
| ME-005      | **Basic Match (Partial Fill - Taker)**: Taker limit order partially fills a maker order. | Maker order quantity is reduced. Taker order is fully filled. Correct `DoneMessage` published.           | `orderbook_test.go:TestLimitOrderMatching`, `integration_v2_test.go:TestIntegrationV2_LimitOrderMatch`                                                                             |   ✅   |
| ME-006      | **Basic Match (Partial Fill - Maker)**: Taker limit order is larger than the maker order. | Maker order is fully filled and removed. Taker order quantity is reduced and remains/matches further. Correct `DoneMessage`. | `orderbook_test.go:TestMarketOrderExecution`, `orderbook_test.go:TestLimitOrderMatching`, `integration_v2_test.go:TestIntegrationV2_LimitOrderMatch`                              |   ✅   |
| ME-007      | **Multi-Level Match**: Taker order consumes liquidity across multiple price levels. | Order matches sequentially against orders at improving price levels until filled or liquidity exhausted. Correct `DoneMessage`. | `orderbook_test.go:TestMultiLevelMatching`                                                                                                                                         |   ✅   |
| ME-008      | **Self-Matching Prevention**: Attempt to submit an order that would match itself. | The system should prevent or handle self-matching according to defined rules (e.g., reject, cancel both). *Requires clarification on self-matching rules.* | N/A (Rules TBD)                                                                                                                                                                 |   ❌   |

### 4.2. Order Types (`pkg/core/order_test.go`, `pkg/core/orderbook_test.go`, `test/integration/`)

| Scenario ID | Description                                                                    | Expected Result                                                                                                         | Coverage                                                                                                                                                               | Status |
| :---------- | :----------------------------------------------------------------------------- | :---------------------------------------------------------------------------------------------------------------------- | :--------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :----: |
| OT-001      | **Limit Order (Add)**: Create a limit order that does not cross the spread.    | Order is added to the correct side (bid/ask) at the specified price level. Order book state updates correctly.        | `integration_v2_test.go:TestIntegrationV2_BasicLimitOrder` (setup in many others)                                                                                      |   ✅   |
| OT-002      | **Limit Order (Match - Taker)**: Create a limit order that crosses the spread.   | Order matches against existing orders according to ME rules. `DoneMessage` published.                                   | `orderbook_test.go:TestLimitOrderMatching`, `integration_v2_test.go:TestIntegrationV2_LimitOrderMatch`                                                                   |   ✅   |
| OT-003      | **Market Order (Full Fill)**: Create a market order that is fully filled by available liquidity. | Order matches against best available prices until filled. Order is not added to the book. `DoneMessage` published.        | Need specific integration test (Partial fill covered in OT-004)                                                                                                         |   ❌   |
| OT-004      | **Market Order (Partial Fill)**: Create a market order larger than available liquidity. | Order fills against all available liquidity and the remaining quantity is effectively canceled. `DoneMessage` published. | `orderbook_test.go:TestMarketOrderExecution`, `marketorder_test.go:TestMarketOrderMatch`                                                                             |   ✅   |
| OT-005      | **Market Order (No Liquidity)**: Create a market order when the opposite side is empty. | Order is rejected or effectively canceled immediately. `DoneMessage` might indicate zero fills.                   | `orderbook_test.go:TestMarketOrderNoLiquidity`                                                                                                                           |   ✅   |
| OT-006      | **Stop-Limit Order (Placement)**: Place a buy stop-limit order above the market, sell stop-limit below. | Order is accepted but not added to the active order book. It should be retrievable via `get-order`.                   | `orderbook_test.go:TestStopOrder`, `stoplimit_test.go:TestStopLimit`                                                                                                   |   ✅   |
| OT-007      | **Stop-Limit Order (Activation - Buy)**: Market price trades at or above the buy stop price. | Stop-limit order becomes a standard limit order at its limit price and is added to the book/processed. `DoneMessage` may indicate activation. | `orderbook_test.go:TestStopOrder`, `stoplimit_test.go:TestStopLimit` (implicit activation)                                                                            |   ⚠️    |
| OT-008      | **Stop-Limit Order (Activation - Sell)**: Market price trades at or below the sell stop price. | Stop-limit order becomes a standard limit order at its limit price and is added to the book/processed. `DoneMessage` may indicate activation. | `orderbook_test.go:TestStopOrder`                                                                                                                                        |   ⚠️    |
| OT-009      | **Stop-Limit Order (Cancellation)**: Cancel a pending (non-activated) stop-limit order. | Order is removed successfully.                                                                                         | `orderbook_test.go:TestCancelPendingStopOrder`                                                                                                                           |   ⚠️    |

*Note on Stop Orders (OT-007, OT-008, OT-009)*: Core tests exist, but integration tests show activation/cancellation might not be fully correct or trigger expected events. Marked as ⚠️ due to known issues.

### 4.3. Order Book Management (`pkg/server/grpc_orderbook_service_test.go`, `test/integration/`)

| Scenario ID | Description                                                                      | Expected Result                                                                                                                            | Coverage                                                                                                                                                                 | Status |
| :---------- | :------------------------------------------------------------------------------- | :----------------------------------------------------------------------------------------------------------------------------------------- | :----------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :----: |
| OB-001      | **Create Book (Memory)**: Create a new order book using the memory backend.        | `create-book` command succeeds. Book appears in `list-books`.                                                                                | `grpc_orderbook_service_test.go:TestGRPCOrderBookService/CreateOrderBook`, Setup in `integration_v2_test.go`                                                               |   ✅   |
| OB-002      | **Create Book (Redis)**: Create a new order book using the Redis backend.          | `create-book` command succeeds (assuming Redis is running). Book appears in `list-books`.                                                    | `test/integration/redis_integration_test.go` (Likely covers via setup/teardown)                                                                                           |   ✅   |
| OB-003      | **Create Book (Duplicate)**: Attempt to create a book with an existing name.       | Command fails with an "Already Exists" error.                                                                                              | `grpc_orderbook_service_test.go:TestGRPCOrderBookService/CreateOrderBook_Duplicate`                                                                                        |   ✅   |
| OB-004      | **Get State (Empty)**: Get the state of an empty order book.                       | Command succeeds, showing empty bids and asks.                                                                                             | `test/integration/*_test.go` (Verified implicitly before orders added)                                                                                                    |   ✅   |
| OB-005      | **Get State (Populated)**: Get the state of a book with bids and asks.             | Command succeeds, displaying correct price levels, aggregated quantities, and order counts per level, sorted correctly.                 | `grpc_orderbook_service_test.go:TestGRPCOrderBookService/GetOrderBookState` (basic), `test/integration/*_test.go` (Verified in multiple tests)                              |   ✅   |
| OB-006      | **List Books**: List all currently active order books.                             | Command succeeds, showing the names of all created books.                                                                                  | `grpc_orderbook_service_test.go:TestGRPCOrderBookService/ListOrderBooks` (basic). Pagination not tested.                                                                 |   ⚠️   |
| OB-007      | **Delete Book**: Delete an existing order book.                                    | Command succeeds. Book no longer appears in `list-books`. Operations on the deleted book fail with "Not Found".                         | `grpc_orderbook_service_test.go:TestGRPCOrderBookService/DeleteOrderBook`                                                                                                |   ✅   |
| OB-008      | **Delete Book (Non-Existent)**: Attempt to delete a book that does not exist.      | Command fails with a "Not Found" error.                                                                                                    | Need specific test case in `grpc_orderbook_service_test.go`                                                                                                                |   ❌   |

### 4.4. Order Lifecycle & Execution (`pkg/core/orderbook_test.go`, `pkg/server/grpc_orderbook_service_test.go`, `test/integration/`)

| Scenario ID | Description                                                                               | Expected Result                                                                                                                                | Coverage                                                                                                                                                         | Status |
| :---------- | :---------------------------------------------------------------------------------------- | :--------------------------------------------------------------------------------------------------------------------------------------------- | :--------------------------------------------------------------------------------------------------------------------------------------------------------------- | :----: |
| LC-001      | **TIF (GTC)**: Place a GTC limit order. Cancel it later.                                  | Order rests on the book until explicitly canceled. Cancellation succeeds. `DoneMessage` reflects cancellation.                                   | `integration_v2_test.go:TestIntegrationV2_CancelOrder`, `orderbook_test.go:TestCancelOrder`                                                                     |   ✅   |
| LC-002      | **TIF (IOC - Partial Match)**: Place an IOC limit order that partially matches.             | Order fills the available quantity immediately. The remaining quantity is canceled. `DoneMessage` reflects partial fill and cancellation.      | `orderbook_test.go:TestIOCOrder`, `integration_v2_test.go:TestIntegrationV2_IOC_FOK/IOC_PartialFill`                                                            |   ⚠️   |
| LC-003      | **TIF (IOC - Full Match)**: Place an IOC limit order that fully matches immediately.        | Order fills completely. `DoneMessage` reflects full fill.                                                                                    | `orderbook_test.go:TestIOCOrder`                                                                                                                               |   ⚠️   |
| LC-004      | **TIF (IOC - No Match)**: Place an IOC limit order that cannot match immediately.           | Order is canceled immediately. `DoneMessage` reflects cancellation with zero fills.                                                            | `orderbook_test.go:TestIOCOrder`                                                                                                                               |   ⚠️   |
| LC-005      | **TIF (FOK - Match Possible)**: Place an FOK limit order where the full quantity can be filled. | Order fills completely. `DoneMessage` reflects full fill.                                                                                    | `orderbook_test.go:TestFOKLimitOrder`, `integration_v2_test.go:TestIntegrationV2_IOC_FOK/FOK_Success`                                                          |   ⚠️   |
| LC-006      | **TIF (FOK - Match Not Possible)**: Place an FOK limit order where the full quantity cannot be filled. | Order is canceled immediately. `DoneMessage` reflects cancellation with zero fills.                                                            | `orderbook_test.go:TestFOKLimitOrder`, `integration_v2_test.go:TestIntegrationV2_IOC_FOK/FOK_Fail`                                                               |   ⚠️   |
| LC-007      | **Cancel Order (Active Limit)**: Cancel a resting limit order.                            | `cancel-order` succeeds. Order removed from book state. `get-order` shows canceled status. `DoneMessage` (from cancellation) published.        | `orderbook_test.go:TestCancelOrder`, `integration_v2_test.go:TestIntegrationV2_CancelOrder`, `grpc_orderbook_service_test.go:TestGRPCOrderBookService/CancelOrder` |   ✅   |
| LC-008      | **Cancel Order (Non-Existent)**: Attempt to cancel an order ID that does not exist.       | `cancel-order` fails with a "Not Found" error.                                                                                                 | Need specific test case in `grpc_orderbook_service_test.go`                                                                                                      |   ❌   |
| LC-009      | **Cancel Order (Already Filled)**: Attempt to cancel an order that has already fully filled. | `cancel-order` fails, possibly with "Not Found" or a specific "Already Filled" error.                                                       | Need specific test case in `grpc_orderbook_service_test.go`                                                                                                      |   ❌   |
| LC-010      | **Get Order (Active)**: Retrieve details of an active resting order.                        | `get-order` succeeds, showing correct ID, side, type, price, quantity, status (e.g., "OPEN").                                                | `grpc_orderbook_service_test.go:TestGRPCOrderBookService/GetOrder`, `integration_v2_test.go:TestIntegrationV2_CancelOrder` (before cancel)                      |   ✅   |
| LC-011      | **Get Order (Filled)**: Retrieve details of a fully filled order.                           | `get-order` succeeds, showing correct details and status (e.g., "FILLED").                                                                   | Need integration test verification                                                                                                                             |   ❌   |
| LC-012      | **Get Order (Canceled)**: Retrieve details of a canceled order.                             | `get-order` succeeds, showing correct details and status (e.g., "CANCELED").                                                                 | `integration_v2_test.go:TestIntegrationV2_CancelOrder` (verifies NotFound, as expected by current impl)                                                       |   ✅   |

*Note on IOC/FOK (LC-002 to LC-006)*: Core tests exist, but integration tests indicate potential issues with partial fills or cancellation logic. Marked as ⚠️ due to known issues.

### 4.5. Kafka Integration (`test/integration/`)

| Scenario ID | Description                                                                          | Expected Result                                                                                                                                 | Coverage                                                                                                                                                                  | Status |
| :---------- | :----------------------------------------------------------------------------------- | :---------------------------------------------------------------------------------------------------------------------------------------------- | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | :----: |
| KI-001      | **Kafka Message (Limit Order Fill)**: A limit order is partially or fully filled.      | A `DoneMessage` is published to Kafka containing correct taker/maker IDs, fill quantity, price, remaining quantities, and order statuses.       | `integration_v2_test.go:TestIntegrationV2_LimitOrderMatch`, `integration_v2_test.go:TestIntegrationV2_IOC_FOK`                                                              |   ✅   |
| KI-002      | **Kafka Message (Market Order Fill)**: A market order is partially or fully filled.    | A `DoneMessage` is published reflecting the executed quantity, average price (if applicable), remaining quantity (should be 0), and status.      | `marketorder_test.go:TestMarketOrderMatch`                                                                                                                                  |   ✅   |
| KI-003      | **Kafka Message (IOC/FOK Cancellation)**: An IOC or FOK order is canceled due to TIF. | A `DoneMessage` is published reflecting the cancellation and any partial fill that might have occurred (for IOC).                                 | `integration_v2_test.go:TestIntegrationV2_IOC_FOK`                                                                                                                        |   ⚠️   |
| KI-004      | **Kafka Message (Explicit Cancellation)**: An order is explicitly canceled via API.    | A `DoneMessage` might be published reflecting the cancellation status. *Current impl: no message sent.*                                         | `integration_v2_test.go:TestIntegrationV2_CancelOrder` (verifies *no* message sent)                                                                                       |   ✅   |
| KI-005      | **Kafka Message (Stop Activation)**: A stop-limit order is activated.                  | A `DoneMessage` might be published indicating the order activation. *Current impl: activation message check marked TODO.*                       | `stoplimit_test.go:TestStopLimit` (Verifies initial placement message. Activation message check TODO)                                                                   |   ⚠️   |
| KI-006      | **Message Format Validation**: Consume messages from Kafka.                          | Messages correctly deserialize according to the `orderbook.proto` definition. All expected fields are present and populated appropriately. | Implicitly covered by all integration tests using `mockSender` and asserting on message fields.                                                                           |   ✅   |

### 4.6. Error Handling (`pkg/core/order_test.go`, `pkg/server/grpc_orderbook_service_test.go`)

| Scenario ID | Description                                                                      | Expected Result                                                                                                        | Coverage                                                                                                                                                                            | Status |
| :---------- | :------------------------------------------------------------------------------- | :--------------------------------------------------------------------------------------------------------------------- | :---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :----: |
| EH-001      | **Invalid Quantity (Zero)**: Submit order with quantity 0.                       | Request fails with `InvalidArgument` error.                                                                            | `order_test.go:TestOrderConstructors_PanicConditions` (core), Need `grpc_orderbook_service_test.go` case                                                                           |   ❌   |
| EH-002      | **Invalid Quantity (Negative)**: Submit order with negative quantity.            | Request fails with `InvalidArgument` error.                                                                            | `order_test.go:TestOrderConstructors_PanicConditions` (core), Need `grpc_orderbook_service_test.go` case                                                                           |   ❌   |
| EH-003      | **Invalid Price (Limit Order)**: Submit limit order with negative or zero price. | Request fails with `InvalidArgument` error.                                                                            | `order_test.go:TestOrderConstructors_PanicConditions` (core), `grpc_orderbook_service_test.go:TestGRPCOrderBookService/CreateOrder_InvalidPrice` (gRPC format)                      |   ✅   |
| EH-004      | **Invalid Type**: Submit order with an unrecognized type string.                 | Request fails with `InvalidArgument` error.                                                                            | Need `grpc_orderbook_service_test.go` case                                                                                                                                        |   ❌   |
| EH-005      | **Duplicate Order ID**: Submit an order with an ID that already exists in the book. | Request fails with `AlreadyExists` error.                                                                              | `orderbook_test.go:TestDuplicateOrderID` (core), Need `grpc_orderbook_service_test.go` case                                                                                         |   ❌   |
| EH-006      | **Operation on Non-Existent Book**: `create-order`, `get-state`, etc. on unknown book name. | Request fails with `NotFound` error.                                                                                   | `grpc_orderbook_service_test.go:TestGRPCOrderBookService/GetOrderBook_NotFound`. Need cases for other ops (e.g., create-order).                                                     |   ❌   |
| EH-007      | **Operation on Non-Existent Order**: `cancel-order`, `get-order` on unknown order ID. | Request fails with `NotFound` error.                                                                                   | `integration_v2_test.go:TestIntegrationV2_CancelOrder` (GetOrder check), Need specific `grpc_orderbook_service_test.go` tests for GetOrder/CancelOrder Not Found.                  |   ❌   |

### 4.7. Concurrency (Basic)

| Scenario ID | Description                                                                                   | Expected Result                                                                                                                  | Coverage                                                                                                             | Status |
| :---------- | :-------------------------------------------------------------------------------------------- | :------------------------------------------------------------------------------------------------------------------------------- | :------------------------------------------------------------------------------------------------------------------- | :----: |
| CC-001      | **Concurrent Additions**: Multiple clients add non-matching limit orders concurrently.        | All orders are added correctly to the book. Final book state reflects all added orders. No deadlocks or race conditions observed. | Manual/TBD. Race detector (`go test -race`) run in CI.                                                                |   ❌   |
| CC-002      | **Concurrent Matching Orders**: Submit crossing buy and sell orders from different clients concurrently. | Orders match correctly according to price-time priority. Final book state is consistent. No deadlocks or lost updates.         | Manual/TBD. Race detector (`go test -race`) run in CI.                                                                |   ❌   |
| CC-003      | **Concurrent Add/Cancel**: One client adds orders while another cancels orders concurrently. | Cancellations only affect existing orders. Additions work correctly. Final state is consistent.                                | Manual/TBD. Race detector (`go test -race`) run in CI.                                                                |   ❌   |

## 5. Test Coverage Summary

Based on the scenarios above:

*   **Good Coverage:** Core matching logic (price-time, multi-level), basic limit/market order placement and matching, basic GTC TIF, order book creation/deletion/state retrieval (memory backend), explicit order cancellation, basic gRPC operations, core order validation, market maker strategy implementation and price fetching.
*   **Partial/Needs Improvement:**
    *   **Stop Orders:** Core logic tests exist, but activation and cancellation behavior, especially in integration scenarios, needs verification and potentially fixes (⚠️).
    *   **IOC/FOK Orders:** Core logic tests exist, but integration tests indicate potential issues needing investigation (⚠️).
    *   **Market Orders:** Full fill scenario and no-liquidity behavior at the gRPC/integration level needs more explicit testing.
    *   **Redis Backend:** Integration tests exist (`redis_integration_test.go`), but specific backend interaction scenarios beyond basic CRUD could be added.
    *   **Error Handling:** gRPC layer needs more explicit tests for invalid inputs (quantity, type, non-existent entities).
    *   **Kafka Integration:** Message triggers for stop activation and explicit cancellation need clarification and testing.
    *   **Concurrency:** No specific automated tests for concurrent operations beyond running the suite with `-race`.
    *   **Market Maker Integration:** Market maker core functionality and strategy calculations are well tested, but integration with the order book service for order placement and lifecycle management needs dedicated tests.

## 6. Running Tests

### Basic Test Commands
```bash
# Run all tests in the workspace
go test ./...

# Run all tests within the pkg directory
go test ./pkg/...

# Run core package tests
go test ./pkg/core/...

# Run memory backend tests
go test ./pkg/backend/memory/...

# Run redis backend tests (requires Redis running)
go test ./pkg/backend/redis/...

# Run server tests
go test ./pkg/server/...

# Run all integration tests (may require Docker, Redis, Kafka)
go test ./test/integration/...

# Run specific integration test file
go test ./test/integration/... -run TestIntegrationV2_BasicLimitOrder

# Run with race detection
go test -race ./...

# Run with verbose output
go test -v ./...

# Run with coverage analysis
go test ./... -coverprofile=coverage.out && go tool cover -html=coverage.out
```

## 7. Known Issues & Gaps

(Corresponds largely to ⚠️ and ❌ in the scenario tables)

1.  **Stop Order Handling:**
    *   Activation logic and event triggering in integration tests are unclear/potentially incorrect.
    *   Cancellation of pending stop orders needs robust integration testing.
    *   Stop book internal mechanisms might be incomplete.
2.  **IOC/FOK Orders:**
    *   Partial fills for IOC orders show discrepancies in integration tests vs. core tests.
    *   FOK orders might not cancel correctly in all integration scenarios when full fill is impossible.
3.  **Market Orders:**
    *   Handling of no liquidity scenarios at the integration level needs verification.
    *   Full fill scenarios need explicit integration tests.
4.  **Integration Test Accuracy:**
    *   Some tests show order book state or trade reporting discrepancies (potentially due to timing, decimal precision, or underlying logic bugs).
5.  **gRPC Error Handling Gaps:**
    *   Missing explicit tests for invalid quantities, types, duplicate order IDs, and operations on non-existent entities at the service boundary.
6.  **Concurrency:** Lack of dedicated concurrent operation tests.

## 8. Test Execution Strategy

*   **Automated Unit & Integration Tests**: Primary method using Go's testing framework (`go test`). Covers core logic, backend interactions, server logic, and key end-to-end flows via integration tests.
*   **Manual Testing**: Use the `orderbook-client` CLI for exploratory testing and verifying scenarios not easily automated (e.g., specific concurrent interactions).
*   **Backend Variation**: Integration tests should ideally run against both `memory` and `redis` backends where state management is critical. (`redis_integration_test.go` helps cover Redis).
*   **CI Pipeline**: Automated execution of unit tests, integration tests, race detection, and coverage reporting on every PR and merge to main.

## 9. Tools

*   `orderbook-server`: The gRPC server binary.
*   `orderbook-client`: The gRPC client CLI binary.
*   Go testing framework (`go test`).
*   Docker (for integration test dependencies like Redis/Kafka).
*   Kafka Consumer Tool (e.g., `kcat`, custom Go consumer) for manual inspection of messages.
*   Redis client (`redis-cli`) to inspect state for Redis backend tests (optional).

## 10. Troubleshooting

Common test failures:

1.  Kafka/Redis connection errors (expected in local testing if services aren't running; should pass in CI).
2.  Decimal precision mismatches (investigate source - calculation errors or representation issues).
3.  Order state inconsistencies (potential bugs in state management or matching logic).
4.  Race conditions (run with `-race` flag to detect).
5.  Timeout errors in integration tests (check service responsiveness, test setup complexity).

## 11. Continuous Integration

Tests are run on each PR and main branch push via GitHub Actions:
*   Unit tests (`go test ./pkg/...`)
*   Integration tests (`go test ./test/integration/...` - requires service setup)
*   Race detection (`go test -race ./...`)
*   Coverage reporting (`go tool cover`)

Note: Integration tests require dependencies (Redis, potentially Kafka) to be available, typically managed via Docker Compose in the CI environment. 