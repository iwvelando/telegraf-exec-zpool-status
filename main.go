package main

import (
	"fmt"
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

	cmd := exec.Command("zpool", "list", "-H", "-p")
	ts := time.Now()
	stdout, err := cmd.Output()
	if err != nil {
		logger.Error("failed to execute zpool list -H -p",
			zap.String("op", "main"),
			zap.Error(err),
		)
		return
	}

	columns := strings.Fields(string(stdout))
	if len(columns)%11 != 0 {
		logger.Error("unexpected number of columns in output",
			zap.String("op", "main"),
			zap.Error(err),
		)
		return
	}

	for i := 0; i < len(columns)/11; i++ {
		poolName := columns[11*i]
		poolSize, err := strconv.Atoi(columns[11*i+1])
		if err != nil {
			logger.Error("failed to convert pool size to int",
				zap.String("op", "main"),
				zap.Error(err),
			)
			return
		}
		poolAlloc, err := strconv.Atoi(columns[11*i+2])
		if err != nil {
			logger.Error("failed to convert pool allocated size to int",
				zap.String("op", "main"),
				zap.Error(err),
			)
			return
		}
		poolFree, err := strconv.Atoi(columns[11*i+3])
		if err != nil {
			logger.Error("failed to convert pool free size to int",
				zap.String("op", "main"),
				zap.Error(err),
			)
			return
		}
		var poolCheckpoint int
		poolCheckpointRaw := columns[11*i+4]
		if poolCheckpointRaw == "-" {
			poolCheckpoint = 0
		} else {
			poolCheckpoint, err = strconv.Atoi(poolCheckpointRaw)
			if err != nil {
				logger.Error("failed to convert pool checkpoint size to int",
					zap.String("op", "main"),
					zap.Error(err),
				)
				return
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
					zap.String("op", "main"),
					zap.Error(err),
				)
				return
			}
		}
		poolFragmentation, err := strconv.Atoi(columns[11*i+6])
		if err != nil {
			logger.Error("failed to convert pool fragmentation to int",
				zap.String("op", "main"),
				zap.Error(err),
			)
			return
		}
		poolCapacity, err := strconv.Atoi(columns[11*i+7])
		if err != nil {
			logger.Error("failed to convert pool capacity to int",
				zap.String("op", "main"),
				zap.Error(err),
			)
			return
		}
		poolDedup, err := strconv.ParseFloat(columns[11*i+8], 32)
		if err != nil {
			logger.Error("failed to convert pool dedup ratio to float",
				zap.String("op", "main"),
				zap.Error(err),
			)
			return
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
				"name":             poolName,
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

}
