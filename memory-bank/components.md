# RudderStack Server Components

This document provides detailed information about the key components of RudderStack Server.

## Gateway

The Gateway component is responsible for receiving and processing incoming requests from client devices.

### Responsibilities
- Receive and validate incoming requests
- Parse and process request payloads
- Batch web requests for improved I/O performance
- Store events in JobsDB
- Send acknowledgments to clients
- Handle rate limiting and user suppression

### Key Files
- `gateway/gateway.go`: Core gateway package
- `gateway/handle.go`: Main request handling logic
- `gateway/handle_http.go`: HTTP request handling
- `gateway/handle_webhook.go`: Webhook handling
- `gateway/regular_handler.go`: Regular request handler
- `gateway/import_handler.go`: Import request handler

### Internal Structure
The Gateway uses a worker-based architecture to process incoming requests.
It batches requests based on user ID to maintain event ordering, and then stores them in JobsDB.
It also supports various types of requests, including HTTP, webhooks, and imports.

## Processor

The Processor component is responsible for processing events stored in JobsDB, applying transformations, and preparing them for routing.

### Responsibilities
- Retrieve jobs from JobsDB
- Apply user transformations
- Apply destination transformations
- Filter events based on rules
- Prepare events for routing
- Store processed events in RouterDB

### Key Files
- `processor/processor.go`: Main processor implementation
- `processor/transformer/`: Transformer client and utilities
- `processor/eventfilter/`: Event filtering logic
- `processor/stash/`: Stashing logic for deferred processing

### Internal Structure
The Processor processes events through several stages:
1. Preprocessing stage: Retrieves and prepares jobs for processing
2. Pre-transformation stage: Prepares events for transformation
3. User transformation stage: Applies user-defined transformations
4. Destination transformation stage: Applies destination-specific transformations
5. Store stage: Stores processed events in RouterDB

## Router

The Router component is responsible for routing processed events to their destinations.

### Responsibilities
- Retrieve processed events from RouterDB
- Route events to their destinations
- Handle retries and failures
- Manage rate limiting and throttling
- Track delivery status

### Key Files
- `router/handle.go`: Main router implementation
- `router/worker.go`: Worker implementation for routing
- `router/factory.go`: Factory for creating router instances
- `router/batchrouter/`: Batch router implementation for warehouse destinations

### Internal Structure
The Router uses a worker-based architecture to route events to destinations.
It supports both real-time routing to API destinations and batch routing to warehouse destinations.
It also handles retries, rate limiting, and other aspects of reliable delivery.

## JobsDB

JobsDB is a PostgreSQL-based job queue that stores events at various stages of processing.

### Responsibilities
- Store events durably
- Support efficient querying and retrieval
- Handle job status management
- Support job migration and cleanup

### Key Files
- `jobsdb/jobsdb.go`: Main JobsDB implementation
- `jobsdb/query.go`: Query-related functionality
- `jobsdb/store.go`: Storage-related functionality
- `jobsdb/migrate.go`: Migration-related functionality

### Internal Structure
JobsDB uses PostgreSQL tables to store jobs, with separate tables for different job statuses
(e.g., unprocessed, executing, processed). It also supports partitioning for improved performance with large datasets.

## Warehouse

The Warehouse component is responsible for syncing data to data warehouses.

### Responsibilities
- Retrieve events for warehouse destinations
- Transform events into warehouse-compatible formats
- Load data into warehouses efficiently
- Handle schema management
- Track sync status

### Key Files
- `warehouse/warehouse.go`: Main warehouse implementation
- `warehouse/manager.go`: Warehouse manager
- `warehouse/slave.go`: Warehouse slave for distributed processing
- `warehouse/[destination]/`: Destination-specific implementations (e.g., Redshift, Snowflake, BigQuery)

### Internal Structure
The Warehouse component uses a manager-slave architecture for distributed processing.
It supports various warehouse destinations, each with its own implementation for schema management, data transformation, and loading.

## Backend Config

The Backend Config component is responsible for managing configuration for sources, destinations, and other aspects of the system.

### Responsibilities
- Retrieve and cache configuration
- Notify components of configuration changes
- Manage source and destination configurations
- Handle workspace settings

### Key Files
- `backend-config/backend-config.go`: Main backend config implementation
- `backend-config/identity.go`: Identity management
- `backend-config/workspace.go`: Workspace configuration

### Internal Structure
The Backend Config component retrieves configuration from a backend service and caches it locally.
It also provides subscription mechanisms for other components to be notified of configuration changes.
