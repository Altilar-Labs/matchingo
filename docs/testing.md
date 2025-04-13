# Matchingo Testing Guide

## Test Categories

### 1. Core Package Tests
- Order Management
  - ✅ Basic order creation (market, limit, stop-limit)
  - ✅ Order validation (quantity, price, TIF)
  - ✅ Order JSON serialization
  - ✅ Order setters and getters
  - ❌ Stop order activation (failing)
  - ❌ IOC/FOK order handling (failing)

- Order Book
  - ✅ Basic order book creation
  - ✅ Market order execution
  - ✅ Limit order matching
  - ✅ Price-time priority
  - ✅ Multi-level matching
  - ❌ Stop order triggering (failing)
  - ❌ Market order no liquidity handling (failing)
  - ❌ Cancel pending stop orders (failing)

### 2. Backend Tests
Memory Backend:
- ✅ Basic initialization
- ✅ Order CRUD operations
- ✅ Order book side management
- ✅ Price level ordering
- ❌ Stop book operations (failing)

Redis Backend:
- ✅ Basic initialization
- ✅ Order CRUD operations
- ✅ Order book side management
- ✅ Multiple orders at same price
- ✅ Component retrieval

### 3. Server Tests
gRPC Service:
- ✅ Order book creation/deletion
- ✅ Basic order operations
- ✅ Order book state retrieval
- ❌ Duplicate order handling (incorrect error code)
- ❌ Order book state accuracy (failing)

Integration Tests:
- ✅ Basic limit order placement
- ✅ Order cancellation
- ❌ Market order matching (failing)
- ❌ IOC/FOK order handling (failing)
- ❌ Stop order execution (failing)

## Running Tests

### Basic Test Commands
```bash
# Run all tests
go test ./pkg/...

# Run specific package tests
go test ./pkg/core/...
go test ./pkg/backend/memory/...
go test ./pkg/backend/redis/...
go test ./pkg/server/...

# Run with race detection
go test -race ./pkg/...

# Run with verbose output
go test -v ./pkg/...
```

### Known Issues
1. Stop Order Handling:
   - Stop orders are not properly removed after triggering
   - Stop book interface implementation is incomplete

2. IOC/FOK Orders:
   - Partial fills for IOC orders not working correctly
   - FOK orders not being properly canceled when full fill is impossible

3. Market Orders:
   - Issues with handling no liquidity scenarios
   - Multiple trade entries being created incorrectly

4. Integration Tests:
   - Order book state not accurately reflecting trades
   - Decimal precision issues in trade quantity reporting

## Test Coverage
Current test coverage shows good coverage of basic functionality but gaps in:
- Stop order handling
- Time-in-force order types (IOC/FOK)
- Market order edge cases
- Integration scenarios

## Troubleshooting
Common test failures:
1. Kafka connection errors (expected in local testing)
2. Decimal precision mismatches
3. Order state inconsistencies
4. Race conditions in concurrent operations

## Continuous Integration
Tests are run on each PR and main branch push:
- Unit tests
- Integration tests
- Race detection
- Coverage reporting

Note: Some tests require Redis to be running locally. Kafka errors can be ignored in local testing as they don't affect core functionality. 