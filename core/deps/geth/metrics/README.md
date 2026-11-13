go-metrics
==========

![travis build status](https://travis-ci.org/rcrowley/go-metrics.svg?branch=master)

Go port of Coda Hale's Metrics library: <https://github.com/dropwizard/metrics>.

This repository's package is `github.com/ethereum/go-ethereum/metrics` in
[`core/deps/geth`](../). The links to go-metrics below describe the upstream project.

Usage
-----

Create and update metrics:

```go
c := metrics.NewCounter()
metrics.Register("foo", c)
c.Inc(47)

g := metrics.NewGauge()
metrics.Register("bar", g)
g.Update(47)

r := metrics.NewRegistry()
cacheGauge := metrics.NewRegisteredGauge("cache-evictions", r)
cacheGauge.Update(47)

s := metrics.NewExpDecaySample(1028, 0.015) // or metrics.NewUniformSample(1028)
h := metrics.NewHistogram(s)
metrics.Register("baz", h)
h.Update(47)

m := metrics.NewMeter()
metrics.Register("quux", m)
m.Mark(47)

t := metrics.NewTimer()
metrics.Register("bang", t)
t.Time(func() {})
t.Update(47)
```

Use `GetOrRegisterTimer` to retrieve or register a timer in one operation:

```go
t := metrics.GetOrRegisterTimer("account.create.latency", nil)
t.Time(func() {})
t.Update(47)
```

**NOTE:** Be sure to unregister short-lived meters and timers otherwise they will
leak memory:

```go
// Will call Stop() on the Meter to allow for garbage collection
metrics.Unregister("quux")
// Or similarly for a Timer that embeds a Meter
metrics.Unregister("bang")
```

Periodically log every metric in human-readable form to standard error:

```go
go metrics.Log(metrics.DefaultRegistry, 5 * time.Second, log.New(os.Stderr, "metrics: ", log.Lmicroseconds))
```

Periodically log every metric in slightly-more-parseable form to syslog:

```go
w, _ := syslog.Dial("unixgram", "/dev/log", syslog.LOG_INFO, "metrics")
go metrics.Syslog(metrics.DefaultRegistry, 60e9, w)
```

Periodically emit every metric into InfluxDB using the local reporter:

```go
import "github.com/ethereum/go-ethereum/metrics/influxdb"

go influxdb.InfluxDB(metrics.DefaultRegistry, 10*time.Second,
  influxURL, influxDatabase, influxUsername, influxPassword, "metrics.")
```

See the [reporter documentation](influxdb/README.md) for its arguments.

Maintain all metrics along with expvars at `/debug/metrics`:

This uses the same mechanism as [the official expvar](https://golang.org/pkg/expvar/)
but exposed under `/debug/metrics`, which shows a json representation of all your usual expvars
as well as all your go-metrics.


```go
import "github.com/ethereum/go-ethereum/metrics/exp"

exp.Exp(metrics.DefaultRegistry)
```

Installation
------------

```sh
go get github.com/ethereum/go-ethereum/metrics
```

Publishing Metrics
------------------

The upstream project lists clients for the following destinations:

* Graphite - https://github.com/cyberdelia/go-metrics-graphite
* InfluxDB - https://github.com/vrischmann/go-metrics-influxdb
* Ganglia - https://github.com/appscode/metlia
* Prometheus - https://github.com/deathowl/go-metrics-prometheus
* DataDog - https://github.com/syntaqx/go-metrics-datadog
* SignalFX - https://github.com/pascallouisperez/go-metrics-signalfx
