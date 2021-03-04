// Package sizing contains a CLI to calculate the required number of nodes for a Cassandra cluster
//
// This tool uses the information from the analysis sub-command or the output of the Evaluation Layer.
//
// @author Alejandro Galue <agalue@opennms.com>
package sizing

import (
	"fmt"
	"math"

	"github.com/urfave/cli"
)

// Command returns the CLI handler to calculate the number of nodes
var Command = cli.Command{
	Name:      "size",
	ShortName: "s",
	Usage:     "Calculates the number instances required for a Cassandra/ScyllaDB cluster",
	Action:    calculate,
	Flags: []cli.Flag{
		cli.Float64Flag{
			Name:  "ttl, t",
			Usage: "TTL, or total metric retention in days",
			Value: 365,
		},
		cli.Float64Flag{
			Name:  "interval, i",
			Usage: "Average data collection interval in minutes",
			Value: 5,
		},
		cli.Float64Flag{
			Name:  "sample-size, s",
			Usage: "Average sample size in bytes; the size of a row from the newts.samples table",
			Value: 18,
		},
		cli.Float64Flag{
			Name:  "replication-factor, r",
			Usage: "The desired replication factor for the Cassandra cluster",
			Value: 2,
		},
		cli.Float64Flag{
			Name:  "disk-overhead, o",
			Usage: "The percentage of disk space overhead per Cassandra instance for compactions",
			Value: 15,
		},
		cli.Float64Flag{
			Name:  "total-metrics, m",
			Usage: "The expected total number of metrics to persist into the Cassandra cluster (as an alternative to inyection-rate)",
		},
		cli.Float64Flag{
			Name:  "injection-rate, R",
			Usage: "The avarage number of samples per second injected to the cluster (as an alternative to total-metrics)",
		},
		cli.Float64Flag{
			Name:     "disk-space, d",
			Usage:    "The total disk space per Cassandra instance in Gigabytes",
			Required: true,
		},
	},
}

func calculate(c *cli.Context) error {
	ttl := c.Float64("ttl")
	collectionStep := c.Float64("interval")
	averageSampleSize := c.Float64("sample-size")
	replicationFactor := c.Float64("replication-factor")
	percentageOverhead := c.Float64("disk-overhead")
	metricsCapacity := c.Float64("total-metrics")
	totalDiskSpacePerNode := c.Float64("disk-space")
	injectionRate := c.Float64("injection-rate")

	if injectionRate > 0 && metricsCapacity > 0 {
		return fmt.Errorf("Please especify either the total metrics or the injection rate but not both")
	}

	fmt.Printf("1 GB = %d Bytes\n", int(math.Pow(2, 30)))

	if injectionRate > 0 {
		metricsCapacity = injectionRate * collectionStep * 60
		fmt.Printf("Theoretical metrics capacity is %d\n", int(metricsCapacity))
	}

	totalSamplesPerMetric := (ttl * 86400) / (collectionStep * 60)
	availBytesPerNode := totalDiskSpacePerNode * math.Pow(2, 30) * (1 - percentageOverhead/100)
	rawNumberOfNodes := (totalSamplesPerMetric * metricsCapacity * averageSampleSize * replicationFactor) / availBytesPerNode
	numberOfNodes := roundUp(rawNumberOfNodes)
	calculatedCapacity := (availBytesPerNode * float64(numberOfNodes)) / (totalSamplesPerMetric * averageSampleSize * replicationFactor)

	fmt.Printf("The total samples per metric would be %d assuming %d bytes per sample with a replication factor of %d\n", int(totalSamplesPerMetric), int(averageSampleSize), int(replicationFactor))
	fmt.Printf("The available disk space bytes per Cassandra/ScyllaDB instance would be %d bytes\n", int(availBytesPerNode))
	fmt.Printf("The expected sample injection rate would be around %d samples/sec persisting data every %dmin\n", int(metricsCapacity/(collectionStep*60)), int(collectionStep))
	if numberOfNodes < int(replicationFactor) {
		fmt.Printf("The recommended number of Cassandra instances would be %d, but due to the chosen replication factor, it should be at least %d\n", numberOfNodes, int(replicationFactor))
	} else {
		fmt.Printf("The recommended number of Cassandra instances would be %d\n", numberOfNodes)
	}
	fmt.Printf("The calculated metrics capacity would be %d\n", int(calculatedCapacity))
	return nil
}

func roundUp(value float64) int {
	v := math.Ceil(value)
	if value-v > 0 {
		v++
	}
	return int(v)
}
