package sys

import (
	"fmt"
	"net/http"
)

const (
	ST_WATCHDOG_NOTIFY = iota
	ST_ARRAY_SIZE
)

var (
	statsKeys = []string{
		"watchdog_notify",
	}
	statsValues = make([]uint64, ST_ARRAY_SIZE)
)

func Mm2metrics(w http.ResponseWriter, in map[string]map[string]uint64) {
	var buf string

	for mname, module := range in {
		for metric, value := range module {
			buf += fmt.Sprintf("# HELP %s %s\n", metric, metric)
			buf += fmt.Sprintf("# TYPE %s counter\n", metric)
			buf += fmt.Sprintf("%s{module=\"%s\"} %d\n", metric, mname, value)
		}
	}
	w.Write([]byte(buf))
}
