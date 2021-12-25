package main

import (
	"code.cloudfoundry.org/bytefmt"
	"flag"
	"fmt"
	"github.com/TobiEiss/go-textfsm/pkg/ast"
	"github.com/TobiEiss/go-textfsm/pkg/process"
	"github.com/TobiEiss/go-textfsm/pkg/reader"
	influx "github.com/influxdata/influxdb-client-go/v2"
	influxWrite "github.com/influxdata/influxdb-client-go/v2/api/write"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var statusInt = map[string]int{
	"ONLINE":    0,
	"OFFLINE":   1,
	"DEGRADED":  2,
	"FAULTED":   3,
	"REMOVED":   4,
	"UNAVAIL":   5,
	"SUSPENDED": 6,
}

func main() {

	template := flag.String(
		"template",
		"./zpool_status_template.txt",
		"path to the TextFSM template for parsing zpool status -s -p",
	)
	flag.Parse()

	cmd := exec.Command("zpool", "list", "-H", "-p")
	stdout, err := cmd.Output()
	if err != nil {
		log.WithFields(log.Fields{
			"op":    "main",
			"error": err,
		}).Error("failed to execute zpool list -H -p")
		os.Exit(1)
	}
	err = ParseZpoolList(stdout)
	if err != nil {
		log.WithFields(log.Fields{
			"op":    "main.ParseZpoolList",
			"error": err,
		}).Error("failed to parse zpool list -H -p")
		os.Exit(2)
	}

	cmd = exec.Command("zpool", "status", "-s", "-p")
	stdout, err = cmd.Output()
	if err != nil {
		log.WithFields(log.Fields{
			"op":    "main",
			"error": err,
		}).Error("failed to execute zpool status -s -p")
		os.Exit(3)
	}
	err = ParseZpoolStatus(stdout, *template)
	if err != nil {
		log.WithFields(log.Fields{
			"op":    "main.ParseZpoolStatus",
			"error": err,
		}).Error("failed to parse zpool status -s -p")
		os.Exit(4)
	}

}

func ParseZpoolList(output []byte) error {
	ts := time.Now()
	columns := strings.Fields(string(output))
	if len(columns)%11 != 0 {
		return fmt.Errorf("unexpected number of columns in output")
	}

	for i := 0; i < len(columns)/11; i++ {
		poolName := columns[11*i]
		poolSize, err := strconv.Atoi(columns[11*i+1])
		if err != nil {
			return fmt.Errorf("failed to convert pool size to int; %s", err)
		}

		poolAlloc, err := strconv.Atoi(columns[11*i+2])
		if err != nil {
			return fmt.Errorf("failed to convert pool allocated size to int; %s", err)
		}

		poolFree, err := strconv.Atoi(columns[11*i+3])
		if err != nil {
			return fmt.Errorf("failed to convert pool free size to int; %s", err)
		}

		var poolCheckpoint int
		poolCheckpointRaw := columns[11*i+4]
		if poolCheckpointRaw == "-" {
			poolCheckpoint = 0
		} else {
			poolCheckpoint, err = strconv.Atoi(poolCheckpointRaw)
			if err != nil {
				return fmt.Errorf("failed to convert pool checkpoint size to int; %s", err)
			}
		}

		var poolExpandSize int
		poolExpandSizeRaw := columns[11*i+5]
		if poolExpandSizeRaw == "-" {
			poolExpandSize = 0
		} else {
			poolExpandSize, err = strconv.Atoi(poolExpandSizeRaw)
			if err != nil {
				return fmt.Errorf("failed to convert pool expand size to int; %s", err)
			}
		}

		poolFragmentation, err := strconv.Atoi(columns[11*i+6])
		if err != nil {
			return fmt.Errorf("failed to convert pool fragmentation to int; %s", err)
		}

		poolCapacity, err := strconv.Atoi(columns[11*i+7])
		if err != nil {
			return fmt.Errorf("failed to convert pool capacity to int; %s", err)
		}

		poolDedup, err := strconv.ParseFloat(columns[11*i+8], 32)
		if err != nil {
			return fmt.Errorf("failed to convert pool dedup ratio to float; %s", err)
		}

		var poolHealth int
		var ok bool
		poolHealthRaw := columns[11*i+9]
		if poolHealth, ok = statusInt[poolHealthRaw]; !ok {
			poolHealth = 99
		}

		poolAltRoot := columns[11*i+10]

		data := influx.NewPoint(
			"zpool",
			map[string]string{
				"pool":             poolName,
				"alternative_root": poolAltRoot,
			},
			map[string]interface{}{
				"size":          poolSize,
				"allocated":     poolAlloc,
				"free":          poolFree,
				"checkpoint":    poolCheckpoint,
				"expand":        poolExpandSize,
				"fragmentation": poolFragmentation,
				"capacity":      poolCapacity,
				"dedup":         poolDedup,
				"health":        poolHealth,
			},
			ts,
		)
		fmt.Println(influxWrite.PointToLineProtocol(data, time.Nanosecond))
	}
	return nil
}

func ParseZpoolStatus(output []byte, templatePath string) error {
	ts := time.Now()
	tmplCh := make(chan string)
	go reader.ReadLineByLine(templatePath, tmplCh)

	srcCh := make(chan string)
	go reader.ReadLineByLineFileAsString(string(output), srcCh)

	ast, err := ast.CreateAST(tmplCh)
	if err != nil {
		return fmt.Errorf("failed to create AST; %s", err)
	}

	record := make(chan []interface{})
	process, err := process.NewProcess(ast, record)
	if err != nil {
		return fmt.Errorf("failed to create record processor; %s", err)
	}
	go process.Do(srcCh)

	for {
		row, ok := <-record
		if !ok {
			break
		}
		if row[1].(string) != "" {
			poolName := row[0].(string)

			bytesRepaired, err := bytefmt.ToBytes(row[1].(string))
			if err != nil {
				return fmt.Errorf("failed to parse %s; %s", row[1].(string), err)
			}

			errorsFoundRaw := row[2].(string)
			errorsFound, err := strconv.Atoi(errorsFoundRaw)
			if err != nil {
				return fmt.Errorf("failed to convert errors found to int; %s", err)
			}

			data := influx.NewPoint(
				"zpool_scrub",
				map[string]string{
					"pool": poolName,
				},
				map[string]interface{}{
					"bytes_repaired": bytesRepaired,
					"errors_found":   errorsFound,
				},
				ts,
			)
			fmt.Println(influxWrite.PointToLineProtocol(data, time.Nanosecond))
		} else if row[3].(string) != "" {
			poolName := row[0].(string)
			errors := row[3].(string)

			var errorsFound int
			if errors == "No known data errors" {
				errorsFound = 0
			} else {
				errorsFound = 1
			}

			data := influx.NewPoint(
				"zpool_errors",
				map[string]string{
					"pool": poolName,
				},
				map[string]interface{}{
					"errors":       errors,
					"errors_found": errorsFound,
				},
				ts,
			)
			fmt.Println(influxWrite.PointToLineProtocol(data, time.Nanosecond))
		} else {
			poolName := row[0].(string)
			deviceName := row[4].(string)

			deviceStateRaw := row[5].(string)
			var deviceState int
			var ok bool
			if deviceState, ok = statusInt[deviceStateRaw]; !ok {
				deviceState = 99
			}

			readErrorsRaw := row[6].(string)
			readErrors, err := strconv.Atoi(readErrorsRaw)
			if err != nil {
				return fmt.Errorf("failed to convert read errors to int; %s", err)
			}

			writeErrorsRaw := row[7].(string)
			writeErrors, err := strconv.Atoi(writeErrorsRaw)
			if err != nil {
				return fmt.Errorf("failed to convert write errors to int; %s", err)
			}

			checksumErrorsRaw := row[8].(string)
			checksumErrors, err := strconv.Atoi(checksumErrorsRaw)
			if err != nil {
				return fmt.Errorf("failed to convert checksum errors to int; %s", err)
			}

			slowIORaw := row[9].(string)
			slowIO, err := strconv.Atoi(slowIORaw)
			if err != nil {
				return fmt.Errorf("failed to convert slow IOs to int; %s", err)
			}

			notes := row[10].(string)

			data := influx.NewPoint(
				"zpool_device",
				map[string]string{
					"pool":   poolName,
					"device": deviceName,
				},
				map[string]interface{}{
					"health":          deviceState,
					"read_errors":     readErrors,
					"write_errors":    writeErrors,
					"checksum_errors": checksumErrors,
					"slow_ios":        slowIO,
					"notes":           notes,
				},
				ts,
			)
			fmt.Println(influxWrite.PointToLineProtocol(data, time.Nanosecond))
		}
	}
	return nil
}
