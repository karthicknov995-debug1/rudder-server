# RudderStack Server Code Organization

This document describes how the code is organized in the RudderStack Server repository.

## Directory Structure

The repository is organized into several key directories, each responsible for a specific aspect of the system:

- `app/`: Core application logic and initialization
- `gateway/`: Handles incoming requests from client devices
- `processor/`: Processes events, applies transformations, and prepares them for routing
- `router/`: Routes events to their destinations
- `warehouse/`: Handles syncing data to data warehouses
- `jobsdb/`: Manages the job queue for processing events
- `backend-config/`: Handles configuration management
- `services/`: Various supporting services
- `utils/`: Utility functions and helpers
- `cmd/`: Command-line entry points
- `config/`: Configuration-related code
- `build/`: Build-related files
- `scripts/`: Various scripts for development, deployment, etc.
- `testhelper/`: Helper functions for testing
- `integration_test/`: Integration tests

## Main Components

### App

The `app` directory contains the core application logic and initialization code. It defines the `App` interface and provides implementations for different application types (embedded, gateway, processor, router).

Key files:
- `app/app.go`: Defines the `App` interface
- `app/embedded.go`: Implementation for the embedded application type
- `app/apphandlers/`: Handlers for different application types

### Gateway

The `gateway` directory contains the code for handling incoming requests from client devices. It batches web requests and writes to the database in bulk to improve I/O performance.

Key files:
- `gateway/gateway.go`: Core gateway package
- `gateway/handle.go`: Main request handling logic
- `gateway/handle_http.go`: HTTP request handling
- `gateway/handle_webhook.go`: Webhook handling
- `gateway/regular_handler.go`: Regular request handler
- `gateway/import_handler.go`: Import request handler

### Processor

The `processor` directory contains the code for processing events, applying transformations, and preparing them for routing.

Key files:
- `processor/processor.go`: Main processor implementation
- `processor/transformer/`: Transformer client and utilities
- `processor/eventfilter/`: Event filtering logic
- `processor/stash/`: Stashing logic for deferred processing

### Router

The `router` directory contains the code for routing events to their destinations.

Key files:
- `router/handle.go`: Main router implementation
- `router/worker.go`: Worker implementation for routing
- `router/factory.go`: Factory for creating router instances
- `router/batchrouter/`: Batch router implementation for warehouse destinations

### Warehouse

The `warehouse` directory contains the code for syncing data to data warehouses.

Key files:
- `warehouse/warehouse.go`: Main warehouse implementation
- `warehouse/manager.go`: Warehouse manager
- `warehouse/slave.go`: Warehouse slave for distributed processing
- `warehouse/[destination]/`: Destination-specific implementations (e.g., Redshift, Snowflake, BigQuery)

### JobsDB

The `jobsdb` directory contains the code for managing the job queue for processing events.

Key files:
- `jobsdb/jobsdb.go`: Main JobsDB implementation
- `jobsdb/query.go`: Query-related functionality
- `jobsdb/store.go`: Storage-related functionality
- `jobsdb/migrate.go`: Migration-related functionality

### Backend Config

The `backend-config` directory contains the code for handling configuration management.

Key files:
- `backend-config/backend-config.go`: Main backend config implementation
- `backend-config/identity.go`: Identity management
- `backend-config/workspace.go`: Workspace configuration

## Coding Conventions

### Package Structure

RudderStack Server follows Go's package structure conventions:
- Each directory is a package
- Package names match directory names
- Packages are organized by functionality
- Interfaces are defined in the package where they are used

### Error Handling

RudderStack Server follows Go's error handling conventions:
- Functions return errors as their last return value
- Errors are checked immediately after function calls
- Custom error types are defined for specific error cases
- Error messages are descriptive and actionable

### Concurrency

RudderStack Server makes extensive use of Go's concurrency primitives:
- Goroutines for concurrent execution
- Channels for communication between goroutines
- Mutexes for protecting shared state
- Context for cancellation and timeouts

### Testing

RudderStack Server has a comprehensive test suite:
- Unit tests for individual functions and methods
- Integration tests for testing component interactions
- End-to-end tests for testing the entire system
- Test helpers for common testing functionality

## Code Flow

The main code flow in RudderStack Server is as follows:

1. The `main.go` file initializes the application and starts the appropriate components based on the configuration.
2. The `runner` package orchestrates the startup and shutdown of the various components.
3. The Gateway component receives incoming requests and stores them in JobsDB.
4. The Processor component retrieves jobs from JobsDB, processes them, and stores them in RouterDB.
5. The Router component retrieves jobs from RouterDB and routes them to their destinations.
6. The Warehouse component handles syncing data to data warehouses.

Each component has its own lifecycle and can be started and stopped independently, allowing for flexible deployment configurations.
