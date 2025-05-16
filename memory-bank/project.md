# RudderStack Server

## Overview
RudderStack Server is the core backend component of RudderStack, an open-source Customer Data Platform (CDP) that
provides data pipelines to collect data from every application, website, and SaaS platform, then activate it in your
warehouse and business tools.

## Purpose
RudderStack helps businesses collect, process, and route customer data to various destinations, including data
warehouses and third-party tools. It serves as a central hub for customer data, enabling businesses to build
comprehensive customer profiles and activate that data for various use cases.

## Key Features
- **Warehouse-first**: RudderStack treats your data warehouse as a first-class citizen among destinations, with advanced
    features and configurable, near real-time sync.
- **Developer-focused**: RudderStack is built API-first, integrating seamlessly with the tools that developers already use.
- **High Availability**: RudderStack comes with at least 99.99% uptime, with sophisticated error handling and retry systems.
- **Privacy and Security**: RudderStack allows you to collect and store customer data without sending everything to a
    third-party vendor, giving you fine-grained control over what data to forward to which analytical tool.
- **Unlimited Events**: With RudderStack Open Source, you can collect as much data as possible without worrying about event budgets.
- **Segment API-compatible**: RudderStack is fully compatible with the Segment API, making migration from Segment straightforward.

## Technology Stack
- **Backend**: Written in Go
- **Frontend**: Written in React.js
- **Database**: PostgreSQL
- **Deployment**: Docker, Kubernetes

## Repository Structure
The repository is organized into several key directories:
- `gateway`: Handles incoming requests from client devices
- `processor`: Processes events, applies transformations, and prepares them for routing
- `router`: Routes events to their destinations
- `warehouse`: Handles syncing data to data warehouses
- `jobsdb`: Manages the job queue for processing events
- `backend-config`: Handles configuration management
- `app`: Core application logic
- `services`: Various supporting services
