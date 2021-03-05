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
	collectionInterval := c.Float64("interval")
	averageSampleSize := c.Float64("sample-size")
	replicationFactor := c.Float64("replication-factor")
	percentageOverhead := c.Float64("disk-overhead")
	totalMetrics := c.Float64("total-metrics")
	totalDiskSpacePerNode := c.Float64("disk-space")
	injectionRate := c.Float64("injection-rate")

	if injectionRate > 0 && totalMetrics > 0 {
		return fmt.Errorf("Please especify either the total number of metrics or the injection rate but not both")
	}

	fmt.Printf("All size calculations assume 1 GB = %d Bytes\n", int(math.Pow(2, 30)))

	if injectionRate > 0 {
		totalMetrics = injectionRate * collectionInterval * 60
		fmt.Printf("The calculated total number of metrics to persist every %dmin would be %d for an injection rate of %d samples/sec.\n", int(collectionInterval), int(totalMetrics), int(injectionRate))
	} else {
		injectionRate = totalMetrics / (collectionInterval * 60)
		fmt.Printf("The expected sample injection rate would be around %d samples/sec persisting data every %dmin for a total number of metrics of %d.\n", int(injectionRate), int(collectionInterval), int(totalMetrics))
	}

	percentageAvailable := (1 - percentageOverhead/100)
	totalDiskPerNodeInBytes := math.Pow(2, 30) * totalDiskSpacePerNode
	availDiskPerNode := totalDiskSpacePerNode * percentageAvailable
	totalSamplesPerMetric := (ttl * 86400) / (collectionInterval * 60)
	sampleCapacityInBytes := totalMetrics * totalSamplesPerMetric
	clusterUsableDiskSpace := sampleCapacityInBytes * averageSampleSize
	numberOfNodes := (clusterUsableDiskSpace * replicationFactor) / (totalDiskPerNodeInBytes * percentageAvailable)
	dailyGrowPerNode := (totalMetrics * (replicationFactor / numberOfNodes) * (86400 / (collectionInterval * 60))) * averageSampleSize / math.Pow(2, 30)

	fmt.Printf("The total samples per metric would be %d, assuming %d bytes per sample with a replication factor of %d.\n", int(totalSamplesPerMetric), int(averageSampleSize), int(replicationFactor))
	fmt.Printf("The available disk space in bytes per Cassandra/ScyllaDB instance would be %d GB.\n", int(availDiskPerNode))
	fmt.Printf("The cluster sample capacity (or size per metric for %d days of TTL) in bytes would be %d (or %d GB).\n", int(ttl), int(sampleCapacityInBytes), int(sampleCapacityInBytes/math.Pow(2, 30)))
	if numberOfNodes < replicationFactor {
		fmt.Printf("The calculated number of Cassandra/ScyllaDB instances would be %.2f instances, but due to the chosen replication factor, it should be at least %d.\n", numberOfNodes, int(replicationFactor))
	} else {
		fmt.Printf("The calculated number of Cassandra/ScyllaDB instances would be %.2f instances.\n", numberOfNodes)
	}
	fmt.Printf("The daily growth in disk space per node would be %.2f GB\n", dailyGrowPerNode)
	return nil
}
