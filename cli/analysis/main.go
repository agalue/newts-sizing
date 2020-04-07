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
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"code.cloudfoundry.org/bytefmt"
	"github.com/karrick/godirwalk"
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

type metrics struct {
	debug               bool
	numOfGroups         int
	numOfNumericMetrics int
	numOfStringMetrics  int
	nodeMap             map[string]int
	resourceMap         map[string]int
	interfaceMap        map[string]int
	totalSizeInBytes    int64
	mu                  sync.Mutex
}

func (m *metrics) incGroups() {
	m.mu.Lock()
	m.numOfGroups++
	m.mu.Unlock()
}

func (m *metrics) addNumeric(numeric int) {
	m.mu.Lock()
	m.numOfNumericMetrics += numeric
	m.mu.Unlock()
}

func (m *metrics) addString(strings int) {
	m.mu.Lock()
	m.numOfStringMetrics += strings
	m.mu.Unlock()
}

func (m *metrics) addSize(size int64) {
	m.mu.Lock()
	m.totalSizeInBytes += size
	m.mu.Unlock()
}

func (m *metrics) addNode(n string) {
	m.mu.Lock()
	m.nodeMap[n]++
	m.mu.Unlock()
}

func (m *metrics) addIntf(i string) {
	m.mu.Lock()
	m.interfaceMap[i]++
	m.mu.Unlock()
}

func (m *metrics) addResource(r string) {
	m.mu.Lock()
	m.resourceMap[r]++
	m.mu.Unlock()
}

func (m *metrics) printSortedMap(data map[string]int) {
	var keys []string
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		fmt.Printf(" %8d: %s (%d)\n", (i + 1), k, data[k])
	}
}

func (m *metrics) printResults() {
	if m.debug {
		fmt.Println()
		fmt.Println("Nodes:")
		m.printSortedMap(m.nodeMap)
		fmt.Println("IP Interfaces:")
		m.printSortedMap(m.interfaceMap)
		fmt.Println("Resources:")
		m.printSortedMap(m.resourceMap)
		fmt.Println()
	}
	fmt.Printf("Number of Nodes = %d\n", len(m.nodeMap))
	fmt.Printf("Number of IP Interfaces = %d\n", len(m.interfaceMap))
	fmt.Printf("Number of OpenNMS Resources = %d\n", len(m.resourceMap))
	fmt.Printf("Number of Groups (Newts Resources) = %d\n", m.numOfGroups)
	// The total should be consistent with the following command:
	// find /opt/opennms/share/rrd -name strings.properties -exec cat {} \; | grep -v "^[#]" | wc -l
	fmt.Printf("Number of String Metrics = %d\n", m.numOfStringMetrics)
	// The total should be consistent with the following command when storeByGroup is enabled
	// find /opennms-data/rrd -name ds.properties -exec cat {} \; | grep -v "^[#]" | wc -l
	fmt.Printf("Number of Numeric Metrics = %d\n", m.numOfNumericMetrics)
	fmt.Printf("Total Size in Bytes = %s\n", bytefmt.ByteSize(uint64(m.totalSizeInBytes)))
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
	data := metrics{
		debug:        debug,
		nodeMap:      make(map[string]int),
		resourceMap:  make(map[string]int),
		interfaceMap: make(map[string]int),
	}

	// Prints the global variables
	fmt.Printf("RRD Directory = %s\n", rrdDir)
	fmt.Printf("Assuming storeByGroup enabled ? %t\n", !singleMetric)
	fmt.Printf("Checking files newer than %s\n", startDate.Format("2006-01-02 15:04:05 MST"))
	fmt.Println("...")

	// Analyze the data directory
	err := godirwalk.Walk(rrdDir, &godirwalk.Options{
		Callback: func(path string, info *godirwalk.Dirent) error {
			// Skip directories as they are not relevant for this analysis
			if info.IsDir() {
				return nil
			}
			// Get file statistics
			stat, err := os.Stat(path)
			if err != nil {
				return err
			}
			// Ignore files older than the start date (based on the provided time range as Duration)
			if !stat.ModTime().After(startDate) {
				return nil
			}
			// Process only RRD/JRB files
			if isRRD(path) {
				if !singleMetric {
					data.incGroups()
				}
				data.addSize(stat.Size())
				data.addNumeric(countNumericMetrics(path, singleMetric))
				data.addResource(filepath.Base(path))
				// Count unique nodes
				if nodeData := nodeRegExp.FindStringSubmatch(path); len(nodeData) == 2 {
					data.addNode(nodeData[1])
				}
				// Count unique IP interfaces (response time resources from Pollerd)
				if intfData := intfRegExp.FindStringSubmatch(path); len(intfData) == 2 {
					data.addIntf(intfData[1])
				}
			}
			// Process string properties
			if info.Name() == stringsProperties {
				data.addString(countStringMetrics(path))
			}
			return nil
		},
		ErrorCallback: func(osPathname string, err error) godirwalk.ErrorAction {
			return godirwalk.SkipNode
		},
		Unsorted:            true, // set true for faster yet non-deterministic enumeration
		FollowSymbolicLinks: true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}

	// Print the results
	data.printResults()
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
