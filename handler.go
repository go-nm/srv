package srv

import (
	"fmt"
	"log"
	"net/http"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"code.cloudfoundry.org/bytefmt"
	"github.com/julienschmidt/httprouter"

	"github.com/go-nm/jres"
)

// HealthMetricResult is the returning struct for calling the HealthMetricHandler
type HealthMetricResult struct {
	OK     bool                   `json:"-"`              // if false the service will return an error on the health endpoint
	Status string                 `json:"status"`         // an optional status string to use instead of the default "ok" and "not ok"
	Info   map[string]interface{} `json:"info,omitempty"` // additional information about the health (such as response time, uptime, etc.)
}

// HealthMetricHandler is the handler func that needs to be implemented
// for adding custom health checks.
type HealthMetricHandler func() HealthMetricResult

// HealthMetric is the struct for a health metric
type HealthMetric struct {
	Name     string
	GetValue HealthMetricHandler
}

// HealthResponse is the response model for the HealthHandler endpoint
type HealthResponse struct {
	Status string `json:"status"`

	Metrics map[string]HealthMetricResult `json:"metrics"`
}

// HealthHandler returns basic system health information
func HealthHandler(metrics *[]HealthMetric) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		res := HealthResponse{}
		isOk := true

		if metrics != nil {
			res.Metrics = map[string]HealthMetricResult{}

			wg := sync.WaitGroup{}
			wg.Add(len(*metrics))

			for _, metric := range *metrics {
				go func(metric HealthMetric) {
					defer wg.Done()
					data := metric.GetValue()

					if data.OK && data.Status == "" {
						data.Status = "ok"
					} else if !data.OK && data.Status == "" {
						data.Status = "not ok"
					}

					if !data.OK {
						isOk = false
					}

					res.Metrics[metric.Name] = data
				}(metric)
			}

			wg.Wait()
		}

		if !isOk {
			res.Status = "not ok"
			jres.Send(w, http.StatusInternalServerError, res)
		} else {
			res.Status = "ok"
			jres.OK(w, res)
		}
	}
}

// InfoMetricHandler is the handler func that needs to be implemented
// for adding custom info metrics.
type InfoMetricHandler func() interface{}

// InfoMetric is the struct for a info metric
type InfoMetric struct {
	Name     string
	GetValue InfoMetricHandler
}

// InfoResponseGC is the response model for the GC in the InfoResponse struct
type InfoResponseGC struct {
	Enabled  bool   `json:"enabled"`
	Runs     uint32 `json:"runs"`
	NextRun  string `json:"nextRun"`
	CPUUsage string `json:"cpuUsage"`
	Time     string `json:"time"`
}

// InfoResponse is the response model for the InfoHandler endpoint
type InfoResponse struct {
	Goroutines int            `json:"goroutines"`
	Uptime     string         `json:"uptime"`
	NumCPU     int            `json:"numCpu"`
	MemUsed    string         `json:"memUsed"`
	GC         InfoResponseGC `json:"gc"`

	Metrics map[string]interface{} `json:"metrics"`
}

// InfoHandler returns basic system runtime information
func InfoHandler(metrics *[]InfoMetric) httprouter.Handle {
	cpus := runtime.NumCPU()
	startTime := time.Now()

	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		memStats := &runtime.MemStats{}
		runtime.ReadMemStats(memStats)
		nsDuration, _ := time.ParseDuration(fmt.Sprintf("%dns", memStats.PauseTotalNs))
		resp := InfoResponse{
			Uptime:     time.Since(startTime).String(),
			Goroutines: runtime.NumGoroutine(),
			NumCPU:     cpus,
			MemUsed:    bytefmt.ByteSize(memStats.HeapAlloc),
			GC: InfoResponseGC{
				Enabled:  memStats.EnableGC,
				Runs:     memStats.NumGC,
				NextRun:  bytefmt.ByteSize(memStats.NextGC),
				CPUUsage: fmt.Sprintf("%.2f%%", memStats.GCCPUFraction*100),
				Time:     nsDuration.String(),
			},
			Metrics: make(map[string]interface{}),
		}

		for _, m := range *metrics {
			resp.Metrics[m.Name] = m.GetValue()
		}

		jres.OK(w, resp)
	}
}

// RouteInfo is the response object for a single route info object
type RouteInfo struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

// RouteHandler returns the handler for listing out the avaliable
// routes for the system including their HTTP verbs
func RouteHandler(routes *[]RouteInfo) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		jres.OK(w, routes)
	}
}

// NotFoundHandler returns the handler for a not found resource.
// This is used as the default handler for the httprouter not found interface
func NotFoundHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jres.NotFound(w, "")
	}
}

// MethodNotAllowedHandler returns the handler for a request where the HTTP verb
// is not allowed. This is used to override the default httprouter handler
func MethodNotAllowedHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jres.MethodNotAllwed(w, nil)
	}
}

// PanicHandler returns the handler for an application panic during a request
// This overrides the default httprouter handler to not expose stacktraces from the
// application during an exception
func PanicHandler() func(http.ResponseWriter, *http.Request, interface{}) {
	return func(w http.ResponseWriter, r *http.Request, ctx interface{}) {
		log.Printf("[PANIC] caught error: %s - stacktrace: %s", ctx, string(debug.Stack()))

		jres.ServerError(w)
	}
}
