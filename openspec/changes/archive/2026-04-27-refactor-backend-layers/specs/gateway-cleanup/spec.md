## ADDED Requirements

### Requirement: No duplicate CORS between Traefik and Gateway
The Gateway service MUST NOT configure its own CORS middleware. Traefik SHALL be the sole CORS handler via Docker labels.

#### Scenario: CORS preflight request
- **WHEN** a browser sends an OPTIONS preflight request
- **THEN** Traefik responds with CORS headers; Gateway does not add its own

#### Scenario: Gateway CORS middleware removed
- **WHEN** the Gateway source code is inspected
- **THEN** there SHALL be no `cors.Handler` or `cors.Options` configuration

### Requirement: No WebSocket proxy in Gateway
The Gateway MUST NOT proxy WebSocket connections. Traefik SHALL route `/api/ws` directly to the Messaging service WebSocket port.

#### Scenario: WebSocket connection
- **WHEN** a client connects to `/api/ws`
- **THEN** Traefik routes directly to Messaging `:8081`, not through Gateway

#### Scenario: Gateway WS handler removed
- **WHEN** the Gateway source code is inspected
- **THEN** there SHALL be no `handleWebSocket` method or `websocket.Upgrader` usage

### Requirement: Input validation on all endpoints
The Gateway MUST validate JSON request body parsing and return `400 Bad Request` with a descriptive error if decoding fails.

#### Scenario: Malformed JSON body
- **WHEN** a client sends invalid JSON to a POST endpoint
- **THEN** Gateway SHALL return HTTP 400 with `{"error":"invalid request body"}` instead of forwarding empty data to gRPC

#### Scenario: Missing required fields
- **WHEN** a required field (e.g., `name` for channel creation) is empty
- **THEN** Gateway SHALL return HTTP 400 with a field-specific error message
