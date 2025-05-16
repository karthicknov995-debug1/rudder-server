# Hard dependencies

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
  * Used to just store jobs that the processor can pick up later
* errDB
  * Used to save webhook failures
  * In the IngestionSvc we publish webhook failures into a procError topic instead, so we could do the same here
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

## Processor

* Reporting
  * Comes from the app features, see `NewReportingMediator`
  * It depends on the BackendConfig
  * We use the [Transactional Outbox pattern](https://microservices.io/patterns/data/transactional-outbox.html) to
    report the metrics
    * It uses a Postgres database to store the metrics in the outbox table
    * It uses an HTTP client to send the metrics stored in the outbox table to an endpoint like https://reporting.rudderstack.com/
  * TODO - is it used to report successful events only? or do we report failed events as well?
* destDebugger - TODO
* transDebugger - TODO
* backendConfig - TODO
* proc.gatewayDB = gatewayDB
* routerDB - TODO
* batchRouterDB - TODO
* readErrorDB - TODO
* writeErrorDB - TODO
* eventSchemaDB - TODO
* archivalDB - TODO
* pendingEventsRegistry - TODO
* transientSources - TODO
* fileuploader - TODO
* rsourcesService - TODO
* enrichers - TODO
* transformerFeaturesService - TODO
* adaptiveLimit - TODO
* storePlocker - TODO
* trackedUsersReporter - TODO
* sourceObservers - TODO

## Router

TODO

## BatchRouter

TODO
