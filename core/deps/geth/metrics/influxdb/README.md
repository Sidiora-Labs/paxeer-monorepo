go-metrics-influxdb
===================

This is a reporter for the [go-metrics](https://github.com/rcrowley/go-metrics) library which will post the metrics to [InfluxDB](https://influxdb.com/).

Note
----

This is only compatible with InfluxDB 0.9+.

Usage
-----

```go
import (
    "time"

    "github.com/ethereum/go-ethereum/metrics"
    "github.com/ethereum/go-ethereum/metrics/influxdb"
)

go influxdb.InfluxDB(
    metrics.DefaultRegistry, // metrics registry
    time.Second * 10,        // interval
    "http://localhost:8086", // the InfluxDB url
    database,                // your InfluxDB database
    username,                // your InfluxDB user
    password,                // your InfluxDB password
    "app.",                  // metric namespace
)
```

License
-------

go-metrics-influxdb is licensed under the MIT license. See the LICENSE file for details.
