# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added
- gRPC server implementation for order book operations
- gRPC client implementation with command-line interface
- Comprehensive documentation for gRPC server and client usage
- Support for multiple order books
- Improved error handling and logging

### Changed
- Reorganized project structure to follow Go's best practices
- Removed example applications in favor of gRPC client
- Updated documentation to reflect current state
- Improved build process with Makefile

### Fixed
- Order ID handling in client implementation
- Protocol buffer import issues
- Server startup and shutdown procedures

## [1.0.0] - 2023-06-10

### Added
- Initial release of the refactored and enhanced matchingo package
- Comprehensive test suite with high code coverage
- In-memory backend implementation for single-instance deployments
- Redis backend implementation for distributed deployments
- Support for various order types (Market, Limit, Stop, OCO)
- Support for different time-in-force options (GTC, IOC, FOK)
- Benchmarking suite for performance testing
- Example applications demonstrating usage 