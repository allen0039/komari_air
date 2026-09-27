# Three-carrier return-route probes

This branch includes an experimental return-route probe for self-hosted Komari panels. It is intended for route inspection and comparison, not as a guarantee of carrier quality, bandwidth, or the forward route direction.

## Scope

- Linux IPv4 Agents with the `trace:v1` capability.
- Agent-side NextTrace-compatible tracing with TCP targets.
- One configurable public target for each of Telecom, Unicom, and Mobile.
- Server-side sample storage, conservative AS-path classification, per-node summaries, and probe logs.
- A configurable daily schedule, manual per-node probes, sequential probe-all runs, and configurable log retention.
- Separate result clearing and log clearing. Clearing results archives the samples used for the current summary; logs can be retained independently.

The default targets are public IPv4 hostnames. Literal target IPs are checked against private, loopback, link-local, multicast, and unspecified ranges; manual probes also require a hostname to resolve to a public IPv4 address.

## Admin RPCs

- `admin:listReturnRouteTargets`
- `admin:getReturnRouteSettings`
- `admin:setReturnRouteEnabled` with `{ "enabled": true }`
- `admin:setReturnRouteSchedule` with `{ "time": "04:20" }`
- `admin:setReturnRouteRetention` with `{ "days": 2 }`
- `admin:saveReturnRouteTargets` with `{ "targets": [...] }`
- `admin:runReturnRoutes` with `{ "uuid": "..." }`
- `admin:runReturnRoute` with `{ "uuid": "...", "target_id": "..." }`
- `admin:runAllReturnRoutes`
- `admin:getReturnRoutes` with `{ "uuid": "..." }`
- `admin:listReturnRouteLogs` with `{ "limit": 50, "offset": 0 }`
- `admin:clearReturnRouteResults` with `{ "confirm": true }`
- `admin:clearReturnRouteLogs` with `{ "confirm": true }`

## Interpretation and limitations

The classifier only reports a named route when the observed path contains stable evidence for a known backbone. Otherwise it returns `Unknown` with low confidence. A summary becomes stale when it is older than 48 hours or when the recent evidence is inconsistent. Agents need the privileges required for IPv4 tracing; failed or incomplete traces are recorded as failed/unknown samples.
