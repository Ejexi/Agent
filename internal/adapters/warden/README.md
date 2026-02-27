# adapters/warden/

Network sandbox proxy adapter. Implements `ports.WardenPort`.

## Files

| File             | Description                                               |
| ---------------- | --------------------------------------------------------- |
| `warden.go`      | Transparent HTTP/HTTPS proxy with Cedar policy evaluation |
| `warden_test.go` | Unit tests for the Warden adapter                         |

## Purpose

All outbound network traffic passes through this transparent proxy, which evaluates Cedar policies to allow/deny requests.

### Features

- Cedar-based policy evaluation
- mTLS (mutual TLS) for agent ↔ server communication
- Default-deny mode (configurable)
- Runs entirely on-premises — no data leaves without policy approval

## Configuration

```toml
[profiles.default.warden]
enabled = true
proxy_addr = "127.0.0.1:9090"
policy_files = ["policies/network.cedar"]
default_deny = true
```
