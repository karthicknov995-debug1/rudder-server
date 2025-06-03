# Hard dependencies

# TODO partitioning strategies

## Isolation Modes

### Processor Isolation Modes
The Processor supports three isolation modes:
1. **None** - No isolation, all jobs are processed together
2. **Workspace** - Jobs are isolated by workspace, ensuring that jobs from different workspaces are processed separately
3. **Source** - Jobs are isolated by source, ensuring that jobs from different sources are processed separately

These isolation modes affect how jobs are queried from the database and processed, providing different levels of
isolation and parallelism.

### Router Isolation Modes
The Router supports three isolation modes:
1. **None** - No isolation, all jobs are processed together
2. **Workspace** - Jobs are isolated by workspace, ensuring that jobs from different workspaces are processed separately
3. **Destination** - Jobs are isolated by destination, ensuring that jobs for different destinations are processed separately

### BatchRouter Isolation Modes
The BatchRouter supports three isolation modes:
1. **None** - No isolation, all jobs are processed together
2. **Workspace** - Jobs are isolated by workspace, ensuring that jobs from different workspaces are processed separately
3. **Destination** - Jobs are isolated by destination, ensuring that jobs for different destinations are processed separately

The isolation modes provide different trade-offs between throughput, resource utilization, and isolation guarantees.
They help ensure that issues in one partition (workspace, source, or destination) don't affect others.

## Gateway

* GW job-status, rudder-sources depends on this
  * at the moment in the ingestion-svc we proxy all the job-status requests to the GW
  * GW is using a shared DB to store the job-status
* GW features (injected via Application)
  * SuppressUser
    * Can be fetched from one of Repo (Badger), FullRepo (Badger) or MemoryRepo
  * Reporting - NOT USED
  * ConfigEnv - NOT USED
  * TrackedUsers - NOT USED
* BackendConfig
  * Can have a cache
  * Used to fetch control plane configurations
  * Used for SuppressUser feature
  * It has a Diagnostics dependency
    * The data for Diagnostics ultimately ends up in Snowflake, we call an HTTP endpoint to send the data to the
      RudderStack dataplane with a writeKey that has a connection setup to send to SnowFlake via RudderServer
    * It would be disabled for dataplanes run by RudderStack but enabled for OpenSource users
* JobsDB
  * Used to store jobs that the processor can pick up later
  * Jobs are stored in a transaction to ensure data consistency
  * Provides at-least-once delivery guarantee for events
* errDB
  * Used to save webhook failures
  * In the IngestionSvc we publish webhook failures into a procError topic instead, so we could do the same here
  * Errors are stored in a transaction to ensure data consistency
  * Provides reliable error tracking and retry capabilities
* rateLimiter
  * We use the usual throttler, could be GCRA, could be something else (e.g. distributed)
  * We do `if sourcesJobRunID == "" && sourcesTaskRunID == ""` when limiting so we don't rate limit rudder-sources
    * There is no need to rate limit rudder-sources (aka Reverse ETL) because they send big batches and wait for the
      jobs to be done before sending the next batch (i.e. by checking via `/job-status`)
* transformerFeaturesSvc
  * Used by the webhook only to get the transformer version
  * The rest of the interface/contract is not used by the GW
* source debugger
  * It depends on a backendConfig as well, we upload data to ControlPlane, AKA Live Events
  * There might be a chance that an event is persisted in JobsDB but not Recorded since the uploader used for Live Events
    uses a non-buffered go channel to propagate the events. The moment the event is popped from the channel, the GW
    is unblocked and can go ahead with the pipeline, but there is no guaranteed that the event was actually "uploaded".
  * Only successful events are recorded thus shown in LiveEvents.
  * This is a fire-and-forget communication pattern with no delivery guarantees
  * The asynchronous nature of this communication means that debugging information may be lost without affecting the
    main event processing pipeline

## Processor

* Reporting
  * Comes from the app features, see `NewReportingMediator`
  * It depends on the BackendConfig
  * We use the [Transactional Outbox pattern](https://microservices.io/patterns/data/transactional-outbox.html) to
    report the metrics
    * It uses a Postgres database to store the metrics in the outbox table
    * It uses an HTTP client to send the metrics stored in the outbox table to an endpoint like https://reporting.rudderstack.com/
    * It reports both successful and failed events, including status codes and error information
    * This ensures comprehensive monitoring and diagnostics of the event processing pipeline
* destDebugger
  * Used to debug destination events
  * Uploads events to the control plane for debugging purposes
  * Similar to the source debugger in the Gateway, there's no guarantee that events will be uploaded successfully
  * Only successful events are recorded and shown in the debugger UI
  * Uses a fire-and-forget communication pattern with no delivery guarantees
  * The debugging information is sent outside the main transaction flow, so failures don't affect event processing
* transDebugger
  * Used to debug transformation events
  * Uploads events to the control plane for debugging purposes
  * Similar to the source debugger, there's no guarantee that events will be uploaded successfully
  * Uses a fire-and-forget communication pattern with no delivery guarantees
  * The debugging information is sent outside the main transaction flow, so failures don't affect event processing
* backendConfig
  * Used to fetch control plane configurations
  * Provides information about sources, destinations, and their configurations
  * The Processor uses this to determine how to process events for different destinations
  * It has a cache to reduce the number of API calls to the control plane
* gatewayDB
  * Used to read jobs that were stored by the Gateway
  * The Processor picks up jobs from this database and processes them
  * Uses a transaction to mark jobs as executing to ensure they're not picked up by other Processor instances
* routerDB
  * Used to store processed jobs that need to be sent to destinations via the Router
  * The Processor writes to this database after processing events
  * Uses a transaction to ensure atomicity when writing jobs
* batchRouterDB
  * Used to store processed jobs that need to be sent to batch destinations via the BatchRouter
  * The Processor writes to this database after processing events
  * Uses a transaction to ensure atomicity when writing jobs
* readErrorDB
  * Used to store jobs that failed during reading from the Gateway DB
  * Allows for retry of failed jobs
* writeErrorDB
  * Used to store jobs that failed during writing to the Router or BatchRouter DBs
  * Allows for retry of failed jobs
* eventSchemaDB
  * Used to store event schemas for validation and tracking
  * Part of the schema management system
  * Schema operations are performed within a transaction to ensure data consistency
  * Uses the same transaction as other database operations to maintain atomicity
  * There is no partitioning strategy, we simply have 1 goroutine that processes all events.
    * On Pulsar we do have a partitioning key and that is the writeKey.
* archivalDB
  * Used to archive processed events for long-term storage
  * Only used if archival is enabled in the configuration
  * Operations are performed within a transaction to ensure data consistency
  * Uses the same transaction as other database operations to maintain atomicity
  * Archiving is done via sourceID
* pendingEventsRegistry
  * Used to track pending events in the system
  * Helps with monitoring and diagnostics
* transientSources
  * Used to handle transient sources that don't persist data
  * These sources might have different processing requirements
* fileuploader
  * Used to upload files to storage services
  * Used for storing large payloads or debug information
* rsourcesService
  * Used to handle RudderStack sources (Reverse ETL)
  * Provides special handling for jobs from RudderStack sources
* enrichers
  * Used to enrich events with additional data
  * Applied in a pipeline during event processing
  * Each enricher can modify the event or add new properties
* transformerFeaturesService
  * Used to fetch transformer features and configurations
  * Determines which transformations are available and how they should be applied
* adaptiveLimit
  * Used to dynamically adjust limits based on system load
  * Helps prevent overloading the system during high traffic
* storePlocker
  * Used to lock partitions during store operations
  * Ensures that only one process is writing to a partition at a time
  * Prevents race conditions and data corruption
* trackedUsersReporter
  * Used to report tracked users to the control plane
  * Helps with user analytics and tracking
* sourceObservers
  * Used to observe events from sources
  * Allows for monitoring and metrics collection

## Router

* jobsDB
  * Used to read jobs that were stored by the Processor
  * The Router picks up jobs from this database and sends them to destinations
  * Uses a transaction to mark jobs as executing to ensure they're not picked up by other Router instances
  * Provides at-least-once delivery guarantee for events
  * Transaction isolation ensures data consistency and prevents race conditions
  * Jobs are marked as processed within the same transaction after successful delivery
* errorDB
  * Used to store jobs that failed during delivery to destinations
  * Allows for retry of failed jobs with exponential backoff
  * Helps ensure eventual delivery of events even in case of temporary destination failures
  * Failed jobs are stored in a transaction to ensure data consistency
  * The transaction ensures that jobs are either successfully delivered or properly stored for retry
* throttlerFactory
  * Used to create throttlers for rate limiting requests to destinations
  * Prevents overwhelming destinations with too many requests
  * Implements various throttling strategies (e.g., GCRA, token bucket)
* backendConfig
  * Used to fetch control plane configurations
  * Provides information about destinations and their configurations
  * The Router uses this to determine how to send events to different destinations
  * It has a cache to reduce the number of API calls to the control plane
* Reporting
  * Used to report metrics about event delivery
  * Helps with monitoring and diagnostics
  * Reports both successful and failed deliveries
  * Uses the Transactional Outbox pattern to ensure metrics are reliably recorded
  * Metrics are stored in a database transaction along with the job status updates
  * This ensures that metrics are only recorded when the corresponding job status changes are committed
* transientSources
  * Used to handle transient sources that don't persist data
  * These sources might have different delivery requirements
* rsourcesService
  * Used to handle RudderStack sources (Reverse ETL)
  * Provides special handling for jobs from RudderStack sources
* transformerFeaturesService
  * Used to fetch transformer features and configurations
  * Determines which transformations are available and how they should be applied
* debugger
  * Used to debug destination events
  * Uploads events to the control plane for debugging purposes
  * Similar to the source debugger in the Gateway, there's no guarantee that events will be uploaded successfully
  * Uses a fire-and-forget communication pattern with no delivery guarantees
  * The debugging information is sent outside the main transaction flow, so failures don't affect event delivery
* pendingEventsRegistry
  * Used to track pending events in the system
  * Helps with monitoring and diagnostics
* adaptiveLimit
  * Used to dynamically adjust limits based on system load
  * Helps prevent overloading the system during high traffic
* destinationResponseHandler
  * Used to handle responses from destinations
  * Processes success and error responses
  * Updates job status based on destination responses
* netHandle
  * Used to make HTTP requests to destinations
  * Handles retries and timeouts
  * Implements connection pooling for better performance
* customDestinationManager
  * Used to manage custom destinations
  * Handles special requirements for custom destinations
* transformer
  * Used to transform events before sending to destinations
  * Applies destination-specific transformations
* oauth
  * Used to handle OAuth authentication for destinations
  * Manages OAuth tokens and refreshes them when needed
  * Ensures secure communication with destinations that require OAuth

## BatchRouter

* jobsDB
  * Used to read jobs that were stored by the Processor
  * The BatchRouter picks up jobs from this database and batches them before sending to destinations
  * Uses a transaction to mark jobs as executing to ensure they're not picked up by other BatchRouter instances
  * Provides at-least-once delivery guarantee for events
  * Transaction isolation ensures data consistency and prevents race conditions
  * Jobs are marked as processed within the same transaction after successful batching and delivery
* errorDB
  * Used to store jobs that failed during delivery to destinations
  * Allows for retry of failed jobs with exponential backoff
  * Helps ensure eventual delivery of events even in case of temporary destination failures
  * Failed jobs are stored in a transaction to ensure data consistency
  * The transaction ensures that jobs are either successfully delivered or properly stored for retry
* reporting
  * Used to report metrics about batch delivery
  * Helps with monitoring and diagnostics
  * Reports both successful and failed deliveries
  * Uses the Transactional Outbox pattern to ensure metrics are reliably recorded
  * Metrics are stored in a database transaction along with the job status updates
  * This ensures that metrics are only recorded when the corresponding job status changes are committed
* backendConfig
  * Used to fetch control plane configurations
  * Provides information about destinations and their configurations
  * The BatchRouter uses this to determine how to batch and send events to different destinations
  * It has a cache to reduce the number of API calls to the control plane
* fileManagerFactory
  * Used to create file managers for different storage providers
  * The BatchRouter uses file managers to store batched events before sending them to destinations
  * Supports various storage providers (e.g., S3, GCS, Azure Blob)
  * Implements retries and error handling for file operations
* transientSources
  * Used to handle transient sources that don't persist data
  * These sources might have different batching requirements
* rsourcesService
  * Used to handle RudderStack sources (Reverse ETL)
  * Provides special handling for jobs from RudderStack sources
* warehouseClient
  * Used to communicate with the warehouse service
  * Sends batched events to data warehouses
  * Handles warehouse-specific requirements and limitations
  * Uses a reliable communication pattern with retries and error handling
* debugger
  * Used to debug destination events
  * Uploads events to the control plane for debugging purposes
  * Similar to the source debugger in the Gateway, there's no guarantee that events will be uploaded successfully
  * Uses a fire-and-forget communication pattern with no delivery guarantees
  * The debugging information is sent outside the main transaction flow, so failures don't affect batch processing
* Diagnostics
  * Used for system diagnostics and monitoring
  * Helps identify and troubleshoot issues
  * Uses a fire-and-forget communication pattern with no delivery guarantees
  * Diagnostic information is sent asynchronously to avoid impacting the main processing pipeline
* pendingEventsRegistry
  * Used to track pending events in the system
  * Helps with monitoring and diagnostics
* adaptiveLimit
  * Used to dynamically adjust limits based on system load
  * Helps prevent overloading the system during high traffic
* isolationStrategy
  * Used to isolate batches by different criteria
  * Ensures that events are properly grouped and processed
* netHandle
  * Used to make HTTP requests to destinations
  * Handles retries and timeouts
  * Implements connection pooling for better performance
* asyncDestinationStruct
  * Used to manage asynchronous destinations
  * Handles the complexities of asynchronous communication with destinations
  * Implements polling for job status and result retrieval
  * Ensures that events are properly tracked and accounted for even with asynchronous processing
