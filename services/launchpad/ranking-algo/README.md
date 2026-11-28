# Ranking service

A complete, fresh ranking-visibility response with `pools: {}` is valid when
there are no canonical markets. With coherent source watermarks and projections,
the heavy refresh and the independent `new` refresh publish all eight category
heads with zero entries. Each head retains the empty universe digest and the
visibility revision. An empty universe does not produce a refresh failure.

The `ranking` readiness check passes once every category head is current and
matches the visibility revision, subject to the usual secret, recovery epoch and
publication health checks. Ranking page routes return HTTP 200 with an empty
`items` array, `totalEligible: "0"` and no next cursor. Snapshot inspection also
returns HTTP 200. When the first visible, unpaused canonical market arrives, the
next `new` refresh lists it before its first trade; other categories keep their
usual eligibility rules.

A non-200 visibility response, incomplete response, missing or null `pools`,
missing observation time, future observation, observation older than 60 seconds,
or revision whose length is not 64 characters remains unavailable. A frozen
refresh also requires the visibility observation to be at or before its cutoff
and no more than 60 seconds earlier. On the service path, visibility is read first
and the cutoff is then taken from the database clock. Empty markets do not bypass
source completeness, accounting coherence or publication checks.
