# OpenCensus Go Graphite Exporter
[![Build Status][travis-image]][travis-url]

[travis-image]: https://travis-ci.org/census-ecosystem/opencensus-go-exporter-graphite.svg?branch=master
[travis-url]: https://travis-ci.org/census-ecosystem/opencensus-go-exporter-graphite
[![Gitter chat][gitter-image]][gitter-url]

The _OpenCensus Graphite Stats Exporter for Go_ is a package that
exports data to [Graphite](https://graphiteapp.org/), a real-time graphing system that stores and renders graphs of received numeric time-series data on demand.

## Quickstart

### Import

```
import "contrib.go.opencensus.io/exporter/graphite"
```

The API of this project is still evolving.
The use of vendoring or a dependency management tool is recommended.

### Register the exporter

```go
func main() {
    // Namespace is an optional part of the Options struct.
    // Stats will be reported every second by default.
    exporter, err := graphite.NewExporter(graphite.Options{Namespace: "opencensus"})
    ...
}
```

It is possible to set different reporting intervals by using `SetReportingPeriod()`, for example:

```go
func main() {
    graphite.NewExporter(graphite.Options{Namespace: "opencensus"})
    view.RegisterExporter(exporter)
    ....
    view.SetReportingPeriod(5 * time.Second)
}
```

### Options for the Graphite exporter

There are some options that can be defined when registering and creating the exporter. Those options are shown below:

| Field | Description | Default Value |
| ------ | ------ | ------ |
| Host | Type `string`. The Host contains the host address for the graphite server | "127.0.0.1" |
| Port | Type `int`. The Port in which the carbon/graphite endpoint is available | 2003
| Namespace | Type `string`. The Namespace is a string value to build the metric path. It will be the first value on the path | None |

## Implementation Details

To feed data into Graphite in Plaintext, the following format must be used: `<metric path> <metric value> <metric timestamp>`.

  - `metric_path` is the metric name composed of the namespace, view name and tags.
  - `value` is the value of the metric at a given time.
  - `timestamp` is the number of seconds since unix epoch time and the time in which the data is received on Graphite.

## Frequently Asked Questions

### How the stats data is handled?

The stats data is aggregated into Views (which are essentially a collection of metrics, each with a different set of labels). To know more about the definition of views, check the [OpenCensus docs](https://github.com/census-instrumentation/opencensus-specs/blob/master/stats/Export.md)

### How the path is built?

One of the main concepts of Graphite is the `metric path`. This path is used to aggregate and organize the measurements and generate the graphs.

In this exporter, the path is built as follows:

`Options.Namespace`.`View.Name`.`Tags`

  - `Options.Namespace`: Defined in the `Options` object.
  - `View.Name`: The name given to the view.
  - `Tags`: The view tag key and values in the format `key=value`


For example, in a configuration where:

  - `Options.Namespace`: 'opencensus'
  - `View.Name`: 'video_size'
  - `Tags`: { "name": "video1", "author": "john"}

The generated path will look like:

`opencensus.video_size;name=video1;author=john`

#### Graph visualization on Graphite
![Graph visualization on Graphite](https://i.imgur.com/A4AExV8.png)


[gitter-image]: https://badges.gitter.im/census-instrumentation/lobby.svg
[gitter-url]: https://gitter.im/census-instrumentation/lobby?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge
