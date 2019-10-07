# newts-sizing

A tool to help to size Cassandra/ScyllaDB cluster for OpenNMS Newts

## Compile

This tool has been implemented in Go version 1.13 using the `go modules` feature. It is important to remember that the target binary should be executed on the server where OpenNMS is running at least for the `analysis` sub-command, meaning that the Linux binary is recommended.

For Windows/macOS users, the following generates a Linux binary:

```bash
GOOS=linux GOARCH=amd64 go build
```

## Analysis

The `analysis` sub-command offers an alternative to the OpenNMS Evaluation Layer, useful on those circumstances when running this feature is impossible and the server contains the RRD/JRB files that are being updated with metrics.

To simplify the analysis and avoid including non-useful metrics, a time range is offered. The idea is to only process the RRD/JRB files that are newer than a given duration provided in hours (defaults to 48 hours).

> **IMPORTANT**: When `Queued` is enabled, the files are not updated after each collection attempt. This is done when the system has enough cycles, meaning that the RRDs might not be up to date. 48 Hours is a good default, but it is recommended to increase it, but without exaggeration to avoid including metrics that have not been updated in months, or metrics included initially that are now excluded.

> **WARNING**: If `storeByGroup` is not enabled on the target OpenNMS system, this tool won't be able to provide the number of resources from Newts perspective, meaning that it will be impossible to calculate the resource cache and the ring buffer.

## Sizing

The `size` sub-command offers a suggestion for the number of instances on a given Cassandra/ScyllaDB cluster based on the total number of metrics to be persisted and the total disk space per instance.

A variety of attributes are offered to finely tune the results.

The total amount of metrics can be obtained using the OpenNMS Evaluation Layer, or the `analysis` sub-command on an installation with JRB/RRD files.
