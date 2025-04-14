# Market Maker Service Design

## 1. Overview

The Market Maker service is a standalone component designed to provide liquidity to the Matchingo order book service for a specific trading pair (e.g., BTC/USDT). It achieves this by automatically fetching real-time price data from an external source and placing corresponding buy and sell limit orders on the Matchingo gRPC service, maintaining a defined spread.

## 2. Architecture

The service operates independently from the main Matchingo order book engine.

*   **Standalone Binary:** Runs as a separate Go binary, built from `cmd/marketmaker/main.go`.
*   **gRPC Client:** Contains a gRPC client to communicate with the Matchingo service (`orderbook.proto`). It will primarily use `CreateOrder`, `CancelOrder`, and potentially `GetOrderBookState` or `ListOrders` (if added) to manage its orders.
*   **Price Data Feed:** Connects to an external public API (initially Binance) to fetch the latest market price for the target asset pair.
*   **Core Logic:** The market making strategy, price fetching, and gRPC interaction logic reside within a new package: `pkg/marketmaker`.

```mermaid
graph TD
    A[Market Maker Service] -->|gRPC Calls (Create/Cancel Order)| B(Matchingo gRPC Service);
    A -->|HTTP GET Request| C(External Price API e.g., Binance);
    C -->|Price Data| A;
    B -->|Order Confirmations/Errors| A;

    subgraph Market Maker Process
        D[Fetch External Price] --> E[Calculate Bid/Ask];
        E --> F[Check/Cancel Existing Orders];
        F --> G[Place New Bid/Ask Orders];
        G --> D;
    end
```

## 3. Data Source

*   **Requirement:** A free, reliable, public API providing real-time or near real-time price data for major cryptocurrency pairs like BTC/USDT.
*   **Initial Choice:** Binance Public API. It's widely used, offers REST endpoints for ticker prices, and doesn't require authentication for public data.
*   **Endpoint:** `GET https://api.binance.com/api/v3/ticker/price?symbol=BTCUSDT`
*   **Fallback:** Consider adding logic to switch to alternative sources (e.g., Coinbase, Kraken) if the primary source fails.

## 4. Market Making Strategy (Layered Symmetric Quoting)

Instead of a single bid/ask pair, this strategy maintains a *ladder* of orders on both sides of the mid-price to provide depth.

*   **Goal:** Maintain `NUM_LEVELS` buy orders and `NUM_LEVELS` sell orders, symmetrically placed around the mid-price.
*   **Parameters (Configurable):**
    *   `NUM_LEVELS`: Number of price levels on each side (e.g., 5 or 10).
    *   `BASE_SPREAD_PERCENT`: Spread for the *innermost* level (e.g., 0.1%).
    *   `PRICE_STEP_PERCENT`: Price increment between levels as a % of mid-price (e.g., 0.05%).
    *   `ORDER_SIZE`: Quantity for *each* order (can be constant or varied).
    *   `UPDATE_INTERVAL`: Frequency of updates (e.g., 10 seconds).
*   **Logic:**

    1.  **Mid-Price Determination:** Fetch current external price `P`.
    2.  **Calculate Initial Offsets:**
        *   `BaseHalfSpread = P * (BASE_SPREAD_PERCENT / 2 / 100)`
        *   `PriceStep = P * (PRICE_STEP_PERCENT / 100)`
    3.  **Determine Target Price Levels:**
        *   Innermost Bid (Level 1): `P_bid_1 = P - BaseHalfSpread`
        *   Innermost Ask (Level 1): `P_ask_1 = P + BaseHalfSpread`
        *   For subsequent levels `i` from 2 to `NUM_LEVELS`:
            *   `P_bid_i = P_bid_{i-1} - PriceStep`
            *   `P_ask_i = P_ask_{i-1} + PriceStep`
    4.  **Order Placement & Management:**
        *   Track active order IDs (e.g., `active_buy_order_ids[NUM_LEVELS]`, `active_sell_order_ids[NUM_LEVELS]`).
        *   **Update Cycle (every `UPDATE_INTERVAL`):**
            *   Fetch latest external price `P_new`.
            *   Recalculate all `NUM_LEVELS` target bid prices (`P_bid_i_new`) and ask prices (`P_ask_i_new`) based on `P_new`.
            *   **Cancellation (Simple Approach):** Cancel *all* currently active orders tracked in `active_buy_order_ids` and `active_sell_order_ids`. Clear the tracking lists.
            *   **Placement:**
                *   For `i` from 1 to `NUM_LEVELS`:
                    *   Send `CreateOrder` for buy @ `P_bid_i_new`, `ORDER_SIZE`. Store ID in `active_buy_order_ids`. Use ID like `{MARKET_MAKER_ID}-buy-{i}-{timestamp}`.
                    *   Send `CreateOrder` for sell @ `P_ask_i_new`, `ORDER_SIZE`. Store ID in `active_sell_order_ids`. Use ID like `{MARKET_MAKER_ID}-sell-{i}-{timestamp}`.
        *   **Handling Fills:** This simple version relies on periodic cancellation/replacement. It doesn't explicitly react to fills or manage inventory risk.
    5.  Handle potential errors during API calls and order management.

*   **Concrete Example (Layered Strategy):**

    *   **Parameters:**
        *   `MARKET_MAKER_ID`: `mm-01`
        *   `NUM_LEVELS`: 3
        *   `BASE_SPREAD_PERCENT`: 0.1%
        *   `PRICE_STEP_PERCENT`: 0.05%
        *   `ORDER_SIZE`: 0.01 BTC
        *   `UPDATE_INTERVAL`: 10 seconds
    *   **Initial State:** No active orders.

    *   **Cycle 1 (t = 0s):**
        1.  Fetch Price: `P = 50000.00`.
        2.  Calculate Offsets:
            *   `BaseHalfSpread = 50000 * (0.1 / 2 / 100) = 25.00`.
            *   `PriceStep = 50000 * (0.05 / 100) = 25.00`.
        3.  Calculate Target Levels:
            *   Bid 1: `49975.00`, Ask 1: `50025.00`
            *   Bid 2: `49950.00`, Ask 2: `50050.00`
            *   Bid 3: `49925.00`, Ask 3: `50075.00`
        4.  Cancel Orders: None active.
        5.  Place Orders (6 total):
            *   Place Buy @ 49975.00, Qty 0.01 (ID: mm-01-buy-1-ts1) -> Store ID
            *   Place Ask @ 50025.00, Qty 0.01 (ID: mm-01-sell-1-ts1) -> Store ID
            *   Place Buy @ 49950.00, Qty 0.01 (ID: mm-01-buy-2-ts1) -> Store ID
            *   Place Ask @ 50050.00, Qty 0.01 (ID: mm-01-sell-2-ts1) -> Store ID
            *   Place Buy @ 49925.00, Qty 0.01 (ID: mm-01-buy-3-ts1) -> Store ID
            *   Place Ask @ 50075.00, Qty 0.01 (ID: mm-01-sell-3-ts1) -> Store ID

    *   **Cycle 2 (t = 10s):**
        1.  Fetch Price: `P_new = 50100.00`.
        2.  Recalculate Offsets:
            *   `BaseHalfSpread = 50100 * (0.1 / 2 / 100) = 25.05`.
            *   `PriceStep = 50100 * (0.05 / 100) = 25.05`.
        3.  Recalculate Target Levels:
            *   Bid 1: `50074.95`, Ask 1: `50125.05`
            *   Bid 2: `50049.90`, Ask 2: `50150.10`
            *   Bid 3: `50024.85`, Ask 3: `50175.15`
        4.  Cancel Orders: Send `CancelOrder` for all 6 stored IDs from Cycle 1. Clear stored IDs.
        5.  Place Orders (6 new):
            *   Place Buy @ 50074.95, Qty 0.01 (ID: mm-01-buy-1-ts2) -> Store ID
            *   Place Ask @ 50125.05, Qty 0.01 (ID: mm-01-sell-1-ts2) -> Store ID
            *   ... and so on for levels 2 and 3 with new prices and IDs (e.g., mm-01-buy-2-ts2, mm-01-sell-2-ts2) ...

## 5. Configuration

The service will be configured using environment variables or command-line flags:

*   `MATCHINGO_GRPC_ADDR`: Address:port of the Matchingo gRPC server (e.g., `localhost:50051`).
*   `MARKET_SYMBOL`: The symbol identifier used within the Matchingo order book (e.g., `BTC-USDT`).
*   `EXTERNAL_SYMBOL`: The symbol identifier used by the external price API (e.g., `BTCUSDT` for Binance).
*   `PRICE_SOURCE_URL`: Base URL of the external price API (e.g., `https://api.binance.com`).
*   `SPREAD_PERCENT`: Market making spread percentage (e.g., `0.1`).
*   `ORDER_SIZE`: Quantity for market making orders (e.g., `0.01`).
*   `UPDATE_INTERVAL_SECONDS`: Interval for price fetching and order updates (e.g., `10`).
*   `MARKET_MAKER_ID`: A unique identifier for this market maker instance (used in order IDs, e.g., `mm-01`).

## 6. Task Breakdown

Implementation will proceed through the following high-level tasks:

1.  **Project Setup:**
    *   Create `cmd/marketmaker/main.go` structure.
    *   Create `pkg/marketmaker/` directory.
    *   Define core interfaces (e.g., `PriceFetcher`, `OrderPlacer`, `MarketMakerStrategy`).
2.  **Configuration:**
    *   Implement configuration loading (e.g., using `viper`) for all parameters: `MATCHINGO_GRPC_ADDR`, `MARKET_SYMBOL`, `EXTERNAL_SYMBOL`, `PRICE_SOURCE_URL`, `NUM_LEVELS`, `BASE_SPREAD_PERCENT`, `PRICE_STEP_PERCENT`, `ORDER_SIZE`, `UPDATE_INTERVAL_SECONDS`, `MARKET_MAKER_ID`.
    *   Add validation for critical configuration values on startup.
3.  **gRPC Client (`pkg/marketmaker/grpc_client.go`):**
    *   Implement robust connection handling to the Matchingo gRPC server (persistent connection, keepalives, reconnection logic).
    *   Implement wrappers for `CreateOrder` and `CancelOrder` with clear error handling and potentially configurable timeouts/retries.
    *   Implement an interface `OrderPlacer` for easier mocking in tests.
4.  **Price Fetching (`pkg/marketmaker/price_fetcher.go`):**
    *   Implement a function/method to fetch the price from the chosen external API (e.g., Binance), handling HTTP requests and JSON parsing.
    *   Implement robust error handling (network errors, API rate limits, invalid responses).
    *   Use an efficient HTTP client that reuses connections.
    *   Implement an interface `PriceFetcher` for easier mocking.
5.  **Core Market Making Logic (`pkg/marketmaker/strategy.go`, `pkg/marketmaker/market_maker.go`):**
    *   Implement the `LayeredSymmetricQuoting` strategy logic:
        *   Calculation of multiple target bid/ask price levels based on configuration.
        *   State management for tracking multiple active buy and sell order IDs (e.g., using slices or maps: `[]string` or `map[int]string`).
        *   Logic for the "cancel all, place all" update cycle.
    *   Implement the main `MarketMaker` struct/service:
        *   Orchestrates the main loop: triggers price fetching, strategy calculation, and order updates periodically (`UPDATE_INTERVAL`).
        *   Manages dependencies (gRPC client, price fetcher, strategy).
        *   Handles concurrency (e.g., separate goroutines for the main loop and signal handling).
6.  **Application Lifecycle (`cmd/marketmaker/main.go`):**
    *   Initialize components (config, logger, gRPC client, price fetcher, market maker).
    *   Implement graceful shutdown using signal handling (e.g., `os.Signal`, `context.Context`).
    *   Ensure the shutdown process attempts to cancel all outstanding market maker orders via the gRPC client.
    *   Add structured logging (e.g., `slog`) throughout the application for key events and errors.
7.  **Build & Deployment:**
    *   Update `Makefile` with a `build-marketmaker` target.
    *   (Optional) Create a `Dockerfile` for containerizing the market maker service.
8.  **Testing (`pkg/marketmaker/*_test.go`):**
    *   Write unit tests for core logic:
        *   Price level calculations.
        *   Strategy decision logic (using mocked `PriceFetcher` and `OrderPlacer`).
        *   Configuration parsing/validation.
    *   (Optional) Implement integration tests (more complex, may require mock servers or careful environment setup).
9.  **Documentation:**
    *   Update `README.md` with build/run instructions and configuration details for the market maker.
    *   Keep this design document (`docs/marketmaker_design.md`) updated.

## 7. Engineering Considerations

To ensure a robust and maintainable service, consider the following during implementation:

*   **Concurrency:** Utilize Go's concurrency primitives (goroutines, channels, mutexes) appropriately, especially for the main update loop, signal handling, and potentially parallelizing gRPC calls if performance becomes an issue. Ensure thread safety where state is shared.
*   **Error Handling:** Implement comprehensive error handling for all external interactions (gRPC, Price API). Use error wrapping (`fmt.Errorf` with `%w`) for better diagnostics. Implement retries with exponential backoff for transient network errors.
*   **State Management:** Carefully manage the state of active order IDs. The "cancel all, replace all" strategy simplifies this, but ensure the state tracking (clearing lists/maps after cancellation, adding new IDs after placement) is accurate. For simplicity, the service will not attempt state recovery on restart; it will start fresh.
*   **Resource Management:** Ensure resources like network connections (gRPC client, HTTP client) are properly managed and closed during shutdown. Use `defer` statements effectively.
*   **Modularity & Interfaces:** Design components with clear responsibilities and use interfaces (`PriceFetcher`, `OrderPlacer`) to decouple implementations and facilitate unit testing.
*   **Configuration:** Use a flexible configuration library (`viper`) and provide clear documentation for all parameters. Validate inputs early.
*   **Logging:** Employ structured logging (`slog`) with meaningful context (e.g., order IDs, prices, errors) to aid debugging and monitoring. Avoid logging sensitive information.
*   **Idempotency:** Ensure that retrying actions (like cancelling an already cancelled order) is safe. The gRPC server should ideally handle this, but the client should be aware.
*   **Efficiency:** While the "cancel all, replace all" approach is chosen for simplicity, be mindful of the potential volume of gRPC calls. If this becomes a bottleneck, future optimization could involve more granular order updates (only cancelling/replacing orders whose prices have deviated significantly). 