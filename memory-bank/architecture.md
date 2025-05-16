# RudderStack Server Architecture

## High-Level Architecture

RudderStack Server follows a modular architecture with several key components that work together to process and route
customer data.
The system is designed to be highly available, scalable, and extensible.

## Data Flow

1. **Data Collection**: The Gateway component receives data from various sources (SDKs, webhooks, etc.).
2. **Data Storage**: The received data is stored in JobsDB, a PostgreSQL-based job queue.
3. **Data Processing**: The Processor component retrieves jobs from JobsDB, processes them
   (applies transformations, filtering, etc.), and prepares them for routing.
4. **Data Routing**: The Router component routes the processed data to various destinations.
5. **Data Delivery**: Data is delivered to configured destinations (warehouses, third-party tools, etc.).

## Key Components

### Gateway
The Gateway module handles incoming requests from client devices. It batches web requests and writes to the database in
bulk to improve I/O performance.
Only after the request payload is persisted, an acknowledgment is sent to the client.

### JobsDB
JobsDB is a PostgreSQL-based job queue that stores events at various stages of processing.
It ensures durability and reliability of the data pipeline.

### Processor
The Processor component retrieves jobs from JobsDB, processes them through several stages (preprocessing,
transformation, filtering), and prepares them for routing.
It interacts with the transformer service to apply user-defined transformations.

### Router
The Router component routes processed events to their destinations. It handles retries, rate limiting, and other aspects
of reliable delivery.

### Warehouse
The Warehouse component handles syncing data to data warehouses. It has specialized logic for efficiently loading data
into various warehouse destinations.

## Scalability and Reliability

- **Horizontal Scalability**: RudderStack can be scaled horizontally by adding more instances.
- **Fault Tolerance**: The system is designed to be fault-tolerant, with robust error handling and retry mechanisms.
- **High Availability**: RudderStack is designed for high availability, with at least 99.99% uptime.
- **Data Durability**: Events are persisted in JobsDB before acknowledgment, ensuring data durability.

## Configuration Management

RudderStack uses a backend configuration service to manage sources, destinations, and other configuration aspects.
This allows for dynamic updates to the configuration without requiring a restart of the server.
