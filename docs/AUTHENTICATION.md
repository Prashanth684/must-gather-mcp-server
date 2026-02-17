# Authentication and Authorization

This guide covers the OAuth/OIDC authentication implementation for must-gather-mcp-server.

## Overview

The MCP server supports enterprise-grade OAuth/OIDC authentication for secure access to must-gather data:

- JWT token validation (offline + online)
- OIDC provider integration
- Support for all JWT signature algorithms
- OAuth discovery endpoints
- STS token exchange support
- Configuration file support with TOML

## Quick Start

### Unprotected Server (Development)

```bash
./must-gather-mcp-server \
  --must-gather-path /data/must-gather \
  --http \
  --port 8080
```

### Protected Server with OAuth

```bash
./must-gather-mcp-server \
  --must-gather-path /data/must-gather \
  --http \
  --port 8080 \
  --require-oauth \
  --oauth-audience must-gather-mcp \
  --authorization-url https://keycloak.example.com/realms/openshift
```

### Using Configuration File (Recommended)

Create `config.toml`:
```toml
port = "8080"
log_level = 2
must_gather_path = "/data/must-gather"

require_oauth = true
oauth_audience = "must-gather-mcp"
authorization_url = "https://keycloak.example.com/realms/openshift"
oauth_scopes = ["openid", "profile", "email"]

[telemetry]
enabled = true
endpoint = "http://jaeger:4318"
service_name = "must-gather-mcp-server"
```

Run:
```bash
./must-gather-mcp-server --config config.toml
```

## Authentication Flow

```
1. HTTP Request → RequestMiddleware
   ↓
2. Extract trace context, start OpenTelemetry span
   ↓
3. AuthorizationMiddleware
   ↓
4. Check if path is unprotected (/healthz, /.well-known/*)
   ↓
5. Check if OAuth is required (require_oauth config)
   ↓
6. Extract Bearer token from Authorization header
   ↓
7. Parse JWT and extract claims
   ↓
8. Validate offline (expiration, audience)
   ↓
9. Validate with OIDC provider (if configured)
   ↓
10. If valid → proceed to MCP handler
    If invalid → return 401 Unauthorized
```

## Configuration

### Configuration File Hierarchy

Configuration values are loaded in order (later overrides earlier):

1. **Defaults** (from code)
2. **Main config file** (via `--config` flag)
3. **Drop-in files** (from `conf.d/`, alphabetically sorted)
4. **Environment variables** (via config file)
5. **CLI flags** (highest priority)

### Configuration Options

#### Server Settings
```toml
port = "8080"                    # HTTP server port
log_level = 2                    # Log verbosity (0-9)
must_gather_path = "/data"       # Path to must-gather directory
stateless = false                # Enable stateless mode for load balancing
```

#### OAuth/OIDC Settings
```toml
require_oauth = true             # Enable OAuth authentication
oauth_audience = "must-gather-mcp"  # Expected audience in JWT
authorization_url = "https://..."   # OIDC provider URL
oauth_scopes = ["openid", "profile", "email"]  # OAuth scopes
```

#### STS (Token Exchange) Settings
```toml
sts_client_id = "mcp-backend"    # STS client ID
sts_client_secret = "${SECRET}"  # STS secret (use env var)
sts_audience = "kubernetes-api"  # Target audience
sts_scopes = ["cluster:admin"]   # STS scopes
```

#### Telemetry Settings
```toml
[telemetry]
enabled = true
endpoint = "http://jaeger:4318"
service_name = "must-gather-mcp-server"
```

### Drop-in Configuration

Create `conf.d/` directory for additional configs:

```bash
config.toml              # Base configuration
conf.d/
  10-oauth.toml          # OAuth settings
  20-telemetry.toml      # Telemetry settings
  99-local.toml          # Local overrides
```

## Authentication Modes

### Mode 1: Unprotected (require_oauth = false)
- No authentication required
- All requests pass through
- Suitable for: Development, trusted networks

### Mode 2: Raw JWT Validation
- `require_oauth = true`
- No `authorization_url` configured
- Offline validation only (expiration + audience)
- Suitable for: Pre-validated tokens, testing

### Mode 3: Full OIDC Validation
- `require_oauth = true`
- `authorization_url` configured
- Both offline and online validation
- Token signature verified against OIDC provider
- Suitable for: Production deployments

## API Endpoints

### Protected Endpoints (require authentication when OAuth enabled)
- `/mcp` - MCP protocol endpoint
- `/sse` - Server-Sent Events endpoint
- `/message` - Message posting endpoint
- `/stats` - Server statistics (JSON)

### Unprotected Endpoints (always accessible)
- `/healthz` - Health check
- `/.well-known/oauth-authorization-server` - OAuth discovery

## Migration from --http-addr to --port

The `--http-addr` flag has been **deprecated** in favor of `--port`.

### Why the Change?

1. **Consistency**: Port-only format matches config file format
2. **Simplicity**: Host binding is automatic (all interfaces)
3. **Best Practice**: Standard Go HTTP server pattern
4. **Configuration**: Better integration with TOML config

### Migration Examples

**Before:**
```bash
./must-gather-mcp-server --must-gather-path /data --http --http-addr localhost:8080
```

**After:**
```bash
./must-gather-mcp-server --must-gather-path /data --http --port 8080
```

### Backward Compatibility

The deprecated `--http-addr` still works with automatic port extraction:

```bash
# This works but shows deprecation warning:
./must-gather-mcp-server --http --http-addr localhost:8080

# Output includes:
# WARNING: --http-addr is deprecated and only supports binding to all interfaces.
# Use --port instead. Extracted port: 8080
```

### Localhost-Only Binding

The `--port` flag binds to all interfaces. For localhost-only access, use:

**Option 1: Reverse Proxy (nginx)**
```nginx
server {
    listen 127.0.0.1:8080;
    location / {
        proxy_pass http://127.0.0.1:9000;
    }
}
```

**Option 2: SSH Tunnel**
```bash
ssh -L 8081:localhost:8080 user@server
```

**Option 3: Firewall Rules**
```bash
sudo iptables -A INPUT -p tcp --dport 8080 ! -s 127.0.0.1 -j DROP
```

## Security Features

### JWT Validation
- ✅ Signature verification
- ✅ Expiration checking
- ✅ Audience validation
- ✅ Issuer validation
- ✅ Scope extraction
- ✅ Support for all standard algorithms (HS*, RS*, ES*, PS*, EdDSA)

### OIDC Integration
- ✅ Dynamic provider discovery
- ✅ JWKS fetching
- ✅ Token verification
- ✅ Client credential flow support

### Transport Security
- ✅ Bearer token authentication
- ✅ WWW-Authenticate header on 401
- ✅ Secure header handling (X-Forwarded-For, X-Real-IP)
- ✅ TLS support (via reverse proxy or direct)

### Observability
- ✅ OpenTelemetry distributed tracing
- ✅ HTTP request logging
- ✅ Authentication failure logging
- ✅ Statistics endpoint

## Production Deployment

### Production Configuration Example

```toml
# production-config.toml
port = "8080"
log_level = 1
must_gather_path = "/data/must-gather"
stateless = true  # For load balancing

require_oauth = true
oauth_audience = "must-gather-mcp"
authorization_url = "https://keycloak.example.com/realms/openshift"
oauth_scopes = ["openid", "profile", "email"]

# STS for backend auth
sts_client_id = "mcp-backend"
sts_client_secret = "${STS_SECRET}"  # From environment
sts_audience = "kubernetes-api"

[telemetry]
enabled = true
endpoint = "https://jaeger.example.com:4318"
service_name = "must-gather-mcp-server"
```

Run with:
```bash
export STS_SECRET="your-secret-here"
./must-gather-mcp-server --config production-config.toml
```

## Troubleshooting

### "Authentication failed - missing or invalid bearer token"
- Ensure you're sending `Authorization: Bearer <token>` header
- Check that token is valid and not expired

### "OIDC token validation error"
- Verify `authorization_url` is correct and reachable
- Check that OIDC provider is running
- Verify `oauth_audience` matches token audience

### "JWT token validation error"
- Check token expiration
- Verify audience matches configuration
- Ensure token is properly formatted

### Configuration not loading
- Check config file path is correct
- Verify TOML syntax is valid
- Check file permissions

## Implementation Details

### Files Created
- `pkg/config/` - Configuration management (4 files, ~750 lines)
  - `config.go` - Main configuration logic
  - `defaults.go` - Default values
  - `telemetry.go` - OpenTelemetry integration
  - `config_test.go` - Comprehensive tests

- `pkg/http/` - HTTP server and authentication (7 files, ~1350 lines)
  - `authorization.go` - JWT validation and auth middleware
  - `authorization_test.go` - Auth tests with 100% coverage
  - `middleware.go` - Request logging and tracing
  - `middleware_test.go` - Middleware tests
  - `http.go` - HTTP server setup
  - `wellknown.go` - OAuth discovery
  - `wellknown_test.go` - Well-known endpoint tests

### Dependencies Added
```go
github.com/BurntSushi/toml v1.6.0
github.com/coreos/go-oidc/v3 v3.17.0
github.com/go-jose/go-jose/v4 v4.0.4
go.opentelemetry.io/otel v1.40.0
go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.40.0
go.opentelemetry.io/otel/sdk v1.40.0
k8s.io/klog/v2 v2.130.1
```

### Test Coverage
- ✅ `pkg/config` - 100% pass (25 tests)
- ✅ `pkg/http` - 100% pass (55 tests)
- ✅ All packages build successfully
