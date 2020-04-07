// Package analysis contains a CLI to emulate the Evaluation Layer for Newts based on the RRD directory.
//
// This tool only works better when storeByGroup enabled; otherwise, limited results will be provided.
// Queued could confuse the statistics if the data has not been persisted yet.
//
// @author Alejandro Galue <agalue@opennms.com>
package analysis

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/bytefmt"
	"github.com/urfave/cli"
)

// Define constants
const dsProperties = "ds.properties"
const stringsProperties = "strings.properties"

// To identify RRD or JRB files
var rrdRegExp = regexp.MustCompile(`\.(rrd|jrb)$`)

// To identify node directories (Collectd), regardless if storeByForeignSource is enabled
var nodeRegExp = regexp.MustCompile(`snmp\/(\d+|fs\/[^\/]+\/[^\/]+)\/`)

// To identify response time directories (Pollerd)
var intfRegExp = regexp.MustCompile(`response\/([\d.]+)\/`)

var debug = false

// Command analyses the RRD/JRB directory to produce metrics estimates
var Command = cli.Command{
	Name:      "analysis",
	ShortName: "a",
	Usage:     "Analyses the RRD/JRB directory to produce estimates about total metrics similar to the Evaluation Layer",
	Action:    analyze,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "rrd-dir, r",
			Usage: "The RRD/JRB directory",
			Value: "/opt/opennms/share/rrd",
		},
		cli.DurationFlag{
			Name:  "newer-than, n",
			Usage: "To only process files newer than a given number of hours",
			Value: 48 * time.Hour,
		},
		cli.BoolFlag{
			Name:  "single-metric, s",
			Usage: "To instruct the analyzer that storeByGroup is disabled",
		},
		cli.BoolFlag{
			Name:  "debug, d",
			Usage: "To show debug information while processing the data directory",
		},
	},
}

func analyze(c *cli.Context) error {
	log.SetOutput(os.Stdout)
	rrdDir := c.String("rrd-dir")
	newerThan := c.Duration("newer-than")
	singleMetric := c.Bool("single-metric")
	debug = c.Bool("debug")

	// Initialized the expected modified date on files based on the provided range in hours
	startDate := time.Now().Add(-newerThan)

	// Initialize local variables that will hold the statistics
	numOfGroups := 0
	numOfNumericMetrics := 0
	numOfStringMetrics := 0
	nodeMap := make(map[string]int)
	resourceMap := make(map[string]int)
	interfaceMap := make(map[string]int)
	var totalSizeInBytes int64 = 0

	// Prints the global variables
	fmt.Printf("RRD Directory = %s\n", rrdDir)
	fmt.Printf("Assuming storeByGroup enabled ? %t\n", !singleMetric)
	fmt.Printf("Checking files newer than %s\n", startDate.Format("2006-01-02 15:04:05 MST"))
	fmt.Println("...")

	// Analyze the data directory
	err := filepath.Walk(rrdDir, func(path string, info os.FileInfo, err error) error {
		// Skip errors while processing the files
		if err != nil {
			return err
		}
		// Skip directories as they are not relevant for this analysis
		if info.IsDir() {
			return nil
		}
		// Ignore files older than the start date (based on the provided time range as Duration)
		if !info.ModTime().After(startDate) {
			return nil
		}
		// Process only RRD/JRB files
		if isRRD(path) {
			if !singleMetric {
				numOfGroups++ // Resources from Newts perspective
			}
			totalSizeInBytes += info.Size()
			numOfNumericMetrics += countNumericMetrics(path, singleMetric)
			resourceMap[filepath.Base(path)]++
			// Count unique nodes
			if nodeData := nodeRegExp.FindStringSubmatch(path); len(nodeData) == 2 {
				nodeMap[nodeData[1]]++
			}
			// Count unique IP interfaces (response time resources from Pollerd)
			if intfData := intfRegExp.FindStringSubmatch(path); len(intfData) == 2 {
				interfaceMap[intfData[1]]++
			}
		}
		// Process string properties
		if info.Name() == stringsProperties {
			numOfStringMetrics += countStringMetrics(path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Print the results
	fmt.Printf("Number of Nodes = %d\n", len(nodeMap))
	fmt.Printf("Number of IP Interfaces = %d\n", len(interfaceMap))
	fmt.Printf("Number of Resources = %d\n", len(resourceMap))
	fmt.Printf("Number of Groups = %d\n", numOfGroups)
	fmt.Printf("Number of String Metrics = %d\n", numOfStringMetrics)
	fmt.Printf("Number of Numeric Metrics = %d\n", numOfNumericMetrics)
	fmt.Printf("Total Size in Bytes = %s\n", bytefmt.ByteSize(uint64(totalSizeInBytes)))

	return nil
}

// Returns true if the path is associated with an RRD or JRB file
func isRRD(path string) bool {
	return rrdRegExp.MatchString(path)
}

// Returns the total number of numeric metrics on a given RRD/JRB file assuming storeByGroup
func countNumericMetrics(path string, singleMetric bool) int {
	if singleMetric {
		info(fmt.Sprintf("Assuming single metric per RRD/JRB files for %s", path))
		return 1 // Assumning single metric
	}
	resource := rrdRegExp.ReplaceAllString(filepath.Base(path), "")
	dsFile := fmt.Sprintf("%s/%s", filepath.Dir(path), dsProperties)
	properties := getProperties(dsFile)
	count := 0
	for _, value := range properties {
		if value == resource {
			count++
		}
	}
	info(fmt.Sprintf("There are %d numeric metrics for %s on %s", count, resource, dsFile))
	return count
}

// Returns the total number of strings metrics for a given OnmsResource
func countStringMetrics(path string) int {
	stringsFile := fmt.Sprintf("%s/%s", filepath.Dir(path), stringsProperties)
	properties := getProperties(stringsFile)
	count := len(properties)
	info(fmt.Sprintf("There are %d string attributes on %s", count, stringsFile))
	return count
}

// Parse and return a Java properties file as a map
func getProperties(path string) map[string]string {
	properties := make(map[string]string)
	file, err := os.Open(path)
	if err != nil {
		return properties
	}
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		if data := strings.Split(scanner.Text(), "="); len(data) == 2 {
			properties[data[0]] = data[1]
		}
	}
	file.Close()
	return properties
}

// Verify if a given file exists on disk
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// Returns true if a given text is a number
func isNum(text string) bool {
	if _, err := strconv.Atoi(text); err == nil {
		return true
	}
	return false
}

// Displays logging information only when debug is enabled
func info(text string) {
	if debug {
		log.Println(text)
	}
}
