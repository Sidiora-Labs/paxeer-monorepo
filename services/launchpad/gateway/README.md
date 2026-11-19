# Gateway private identity listener

The private identity listener requires a client certificate verified by the gateway's private authority. An admitted client can read `GET /health/ready`; it returns `200` with `{"status":"ok"}` while the gateway is ready and `503` with `{"status":"degraded"}` before readiness or after shutdown begins.

Identity service routes remain under `/internal/v2/identity/` on the same listener.
