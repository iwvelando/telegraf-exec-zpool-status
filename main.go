package main

import (
	"code.cloudfoundry.org/bytefmt"
	"flag"
	"fmt"
	"github.com/TobiEiss/go-textfsm/pkg/ast"
	"github.com/TobiEiss/go-textfsm/pkg/process"
	"github.com/TobiEiss/go-textfsm/pkg/reader"
	_ "github.com/influxdata/influxdb1-client"
	influxdb "github.com/influxdata/influxdb1-client"
	"go.uber.org/zap"
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

	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Println("{\"op\": \"main\", \"level\": \"fatal\", \"msg\": \"failed to initiate logger\"}")
		os.Exit(1)
	}
	defer logger.Sync()

	template := flag.String(
		"template",
		"./zpool_status_template.txt",
		"path to the TextFSM template for parsing zpool status -s -p",
	)
	flag.Parse()

	cmd := exec.Command("zpool", "list", "-H", "-p")
	stdout, err := cmd.Output()
	if err != nil {
		logger.Error("failed to execute zpool list -H -p",
			zap.String("op", "main"),
			zap.Error(err),
		)
		os.Exit(1)
	}
	err = ParseZpoolList(logger, stdout)
	if err != nil {
		os.Exit(2)
	}

	cmd = exec.Command("zpool", "status", "-s", "-p")
	stdout, err = cmd.Output()
	if err != nil {
		logger.Error("failed to execute zpool status -s -p",
			zap.String("op", "main"),
			zap.Error(err),
		)
		os.Exit(3)
	}
	err = ParseZpoolStatus(logger, stdout, *template)
	if err != nil {
		os.Exit(4)
	}

}

func ParseZpoolList(logger *zap.Logger, output []byte) error {
	ts := time.Now()
	columns := strings.Fields(string(output))
	if len(columns)%11 != 0 {
		logger.Error("unexpected number of columns in output",
			zap.String("op", "ParseZpoolList"),
		)
		return nil
	}

	for i := 0; i < len(columns)/11; i++ {
		poolName := columns[11*i]
		poolSize, err := strconv.Atoi(columns[11*i+1])
		if err != nil {
			logger.Error("failed to convert pool size to int",
				zap.String("op", "ParseZpoolList"),
				zap.Error(err),
			)
			return err
		}

		poolAlloc, err := strconv.Atoi(columns[11*i+2])
		if err != nil {
			logger.Error("failed to convert pool allocated size to int",
				zap.String("op", "ParseZpoolList"),
				zap.Error(err),
			)
			return err
		}

		poolFree, err := strconv.Atoi(columns[11*i+3])
		if err != nil {
			logger.Error("failed to convert pool free size to int",
				zap.String("op", "ParseZpoolList"),
				zap.Error(err),
			)
			return err
		}

		var poolCheckpoint int
		poolCheckpointRaw := columns[11*i+4]
		if poolCheckpointRaw == "-" {
			poolCheckpoint = 0
		} else {
			poolCheckpoint, err = strconv.Atoi(poolCheckpointRaw)
			if err != nil {
				logger.Error("failed to convert pool checkpoint size to int",
					zap.String("op", "ParseZpoolList"),
					zap.Error(err),
				)
				return err
			}
		}

		var poolExpandSize int
		poolExpandSizeRaw := columns[11*i+5]
		if poolExpandSizeRaw == "-" {
			poolExpandSize = 0
		} else {
			poolExpandSize, err = strconv.Atoi(poolExpandSizeRaw)
			if err != nil {
				logger.Error("failed to convert pool expand size to int",
					zap.String("op", "ParseZpoolList"),
					zap.Error(err),
				)
				return err
			}
		}

		poolFragmentation, err := strconv.Atoi(columns[11*i+6])
		if err != nil {
			logger.Error("failed to convert pool fragmentation to int",
				zap.String("op", "ParseZpoolList"),
				zap.Error(err),
			)
			return err
		}

		poolCapacity, err := strconv.Atoi(columns[11*i+7])
		if err != nil {
			logger.Error("failed to convert pool capacity to int",
				zap.String("op", "ParseZpoolList"),
				zap.Error(err),
			)
			return err
		}

		poolDedup, err := strconv.ParseFloat(columns[11*i+8], 32)
		if err != nil {
			logger.Error("failed to convert pool dedup ratio to float",
				zap.String("op", "ParseZpoolList"),
				zap.Error(err),
			)
			return err
		}

		var poolHealth int
		var ok bool
		poolHealthRaw := columns[11*i+9]
		if poolHealth, ok = statusInt[poolHealthRaw]; !ok {
			poolHealth = 99
		}

		poolAltRoot := columns[11*i+10]

		data := influxdb.Point{
			Measurement: "zpool",
			Tags: map[string]string{
				"pool":             poolName,
				"alternative_root": poolAltRoot,
			},
			Fields: map[string]interface{}{
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
			Time:      ts,
			Precision: "ns",
		}
		fmt.Println(data.MarshalString())
	}
	return nil
}

func ParseZpoolStatus(logger *zap.Logger, output []byte, templatePath string) error {
	ts := time.Now()
	tmplCh := make(chan string)
	go reader.ReadLineByLine(templatePath, tmplCh)

	srcCh := make(chan string)
	go reader.ReadLineByLineFileAsString(string(output), srcCh)

	ast, err := ast.CreateAST(tmplCh)
	if err != nil {
		logger.Error("failed to create AST",
			zap.String("op", "ParseZpoolStatus"),
			zap.Error(err),
		)
		return err
	}

	record := make(chan []interface{})
	process, err := process.NewProcess(ast, record)
	if err != nil {
		logger.Error("failed to create record processor",
			zap.String("op", "ParseZpoolStatus"),
			zap.Error(err),
		)
		return err
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
				logger.Error(fmt.Sprintf("failed to parse %s", row[1].(string)),
					zap.String("op", "ParseZpoolStatus"),
					zap.Error(err),
				)
				continue
			}

			errorsFoundRaw := row[2].(string)
			errorsFound, err := strconv.Atoi(errorsFoundRaw)
			if err != nil {
				logger.Error("failed to convert errors found to int",
					zap.String("op", "ParseZpoolStatus"),
					zap.Error(err),
				)
				continue
			}

			data := influxdb.Point{
				Measurement: "zpool_scrub",
				Tags: map[string]string{
					"pool": poolName,
				},
				Fields: map[string]interface{}{
					"bytes_repaired": bytesRepaired,
					"errors_found":   errorsFound,
				},
				Time:      ts,
				Precision: "ns",
			}
			fmt.Println(data.MarshalString())
		} else if row[3].(string) != "" {
			poolName := row[0].(string)
			errors := row[3].(string)

			var errorsFound int
			if errors == "No known data errors" {
				errorsFound = 0
			} else {
				errorsFound = 1
			}

			data := influxdb.Point{
				Measurement: "zpool_errors",
				Tags: map[string]string{
					"pool": poolName,
				},
				Fields: map[string]interface{}{
					"errors":       errors,
					"errors_found": errorsFound,
				},
				Time:      ts,
				Precision: "ns",
			}
			fmt.Println(data.MarshalString())
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
				logger.Error("failed to convert read errors to int",
					zap.String("op", "ParseZpoolStatus"),
					zap.Error(err),
				)
				continue
			}

			writeErrorsRaw := row[7].(string)
			writeErrors, err := strconv.Atoi(writeErrorsRaw)
			if err != nil {
				logger.Error("failed to convert write errors to int",
					zap.String("op", "ParseZpoolStatus"),
					zap.Error(err),
				)
				continue
			}

			checksumErrorsRaw := row[8].(string)
			checksumErrors, err := strconv.Atoi(checksumErrorsRaw)
			if err != nil {
				logger.Error("failed to convert checksum errors to int",
					zap.String("op", "ParseZpoolStatus"),
					zap.Error(err),
				)
				continue
			}

			slowIORaw := row[9].(string)
			slowIO, err := strconv.Atoi(slowIORaw)
			if err != nil {
				logger.Error("failed to convert slow IOs to int",
					zap.String("op", "ParseZpoolStatus"),
					zap.Error(err),
				)
				continue
			}

			notes := row[10].(string)

			data := influxdb.Point{
				Measurement: "zpool_device",
				Tags: map[string]string{
					"pool":   poolName,
					"device": deviceName,
				},
				Fields: map[string]interface{}{
					"health":          deviceState,
					"read_errors":     readErrors,
					"write_errors":    writeErrors,
					"checksum_errors": checksumErrors,
					"slow_ios":        slowIO,
					"notes":           notes,
				},
				Time:      ts,
				Precision: "ns",
			}
			fmt.Println(data.MarshalString())
		}
	}
	return nil
}
