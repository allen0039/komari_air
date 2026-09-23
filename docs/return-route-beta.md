# Three-carrier return-route beta

This branch contains the first test version of Komari's three-carrier return-route probe.

## Scope

- Linux IPv4 agents with the `trace:v1` capability.
- Native Go traceroute collection from the agent.
- Telecom, Unicom, and Mobile target registry with two default targets per carrier.
- Server-side sample storage, conservative classification, and multi-target aggregation.
- Daily 04:20 scheduling and seven-day sample cleanup.
- Node-card summaries and admin RPCs for manual runs.

The classifier intentionally returns `Unknown` when the path does not contain stable evidence. It is not a guarantee of carrier quality, bandwidth, or the forward route direction.

## Admin RPCs

- `admin:listReturnRouteTargets`
- `admin:runReturnRoutes` with `{ "uuid": "..." }`
- `admin:runReturnRoute` with `{ "uuid": "...", "target_id": "..." }`
- `admin:getReturnRoutes` with `{ "uuid": "..." }`

## Beta limitations

- ASN database enrichment and editable target/rule settings are not included yet.
- The first release is intended for internal validation before public badge display.
- Agents need the privileges required for IPv4 ICMP tracing; otherwise the result is recorded as a failed/unknown sample.
