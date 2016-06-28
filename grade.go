package grade

import (
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/influxdata/grade/internal/parse"
	"github.com/influxdata/influxdb/client"
)

// Config represents the settings to process benchmarks.
type Config struct {
	// Database is the name of the database into which to store the processed benchmark results.
	Database string

	// GoVersion is the tag value to use to indicate which version of Go was used for the benchmarks that have run.
	GoVersion string

	// Timestamp is the time to use when recording all of the benchmark results,
	// and is typically the timestamp of the commit used for the benchmark.
	Timestamp time.Time

	// Revision is the tag value to use to indicate which revision of the repository was used for the benchmarks that have run.
	// Feel free to use a SHA, tag name, or whatever will be useful to you when querying.
	Revision string

	// HardwareID is a user-specified string to represent the hardware on which the benchmarks have run.
	HardwareID string
}

func (cfg Config) validate() error {
	var msg []string

	if cfg.Database == "" {
		msg = append(msg, "Database cannot be empty")
	}

	if cfg.GoVersion == "" {
		msg = append(msg, "Go version cannot be empty")
	}

	if cfg.Timestamp.Unix() <= 0 {
		msg = append(msg, "Timestamp must be greater than zero")
	}

	if cfg.Revision == "" {
		msg = append(msg, "Revision cannot be empty")
	}

	if cfg.HardwareID == "" {
		msg = append(msg, "Hardware ID cannot be empty")
	}

	if len(msg) > 0 {
		return errors.New(strings.Join(msg, "\n"))
	}

	return nil
}

// Run processes the benchmark output in the given io.Reader and
// converts that data to InfluxDB points to be sent through the provided Client.
func Run(r io.Reader, cl *client.Client, cfg Config) error {
	if err := cfg.validate(); err != nil {
		return err
	}

	benchset, err := parse.ParseMultipleBenchmarks(r)
	if err != nil {
		return err
	}

	bp := client.BatchPoints{
		Database: cfg.Database,
		Tags: map[string]string{
			"goversion": cfg.GoVersion,
			"hwid":      cfg.HardwareID,
		},
		Time:      cfg.Timestamp,
		Precision: "s",
	}

	for pkg, bs := range benchset {
		for _, b := range bs {
			p := client.Point{
				Measurement: "benchmarks",
				Tags: map[string]string{
					"pkg":  pkg,
					"ncpu": strconv.Itoa(b.NumCPU),
					"name": b.Name,
				},
				Fields: makeFields(b, cfg.Revision),
			}

			bp.Points = append(bp.Points, p)
		}
	}

	_, err = cl.Write(bp)
	return err
}

func makeFields(b *parse.Benchmark, revision string) map[string]interface{} {
	f := make(map[string]interface{}, 6)

	f["revision"] = revision
	f["n"] = b.N

	if (b.Measured & parse.NsPerOp) != 0 {
		f["ns_per_op"] = b.NsPerOp
	}
	if (b.Measured & parse.MBPerS) != 0 {
		f["mb_per_s"] = b.MBPerS
	}
	if (b.Measured & parse.AllocedBytesPerOp) != 0 {
		f["alloced_bytes_per_op"] = int64(b.AllocedBytesPerOp)
	}
	if (b.Measured & parse.AllocsPerOp) != 0 {
		f["allocs_per_op"] = int64(b.AllocsPerOp)
	}

	return f
}