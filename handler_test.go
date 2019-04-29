package srv_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/go-nm/srv"
)

func TestHealthHandler(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	type metricRes map[string]srv.HealthMetricResult
	metricName := "testing"
	infoMetricInfo := map[string]interface{}{"test": "testdata"}
	customMetricStatus := "awesome"
	badMetricStatus := "not good at all"
	baseMetric := srv.HealthMetric{Name: metricName, GetValue: func() srv.HealthMetricResult {
		return srv.HealthMetricResult{OK: true}
	}}
	customStatusMetric := srv.HealthMetric{Name: metricName, GetValue: func() srv.HealthMetricResult {
		return srv.HealthMetricResult{OK: true, Status: customMetricStatus}
	}}
	infoMetric := srv.HealthMetric{Name: metricName, GetValue: func() srv.HealthMetricResult {
		return srv.HealthMetricResult{OK: true, Info: infoMetricInfo}
	}}
	badMetric := srv.HealthMetric{Name: metricName, GetValue: func() srv.HealthMetricResult {
		return srv.HealthMetricResult{OK: false}
	}}
	badMetricCustomStatus := srv.HealthMetric{Name: metricName, GetValue: func() srv.HealthMetricResult {
		return srv.HealthMetricResult{OK: false, Status: badMetricStatus}
	}}
	type args struct {
		metrics *[]srv.HealthMetric
	}
	tests := []struct {
		name     string
		args     args
		wantData srv.HealthResponse
	}{
		{
			name:     "SuccessBasic",
			wantData: srv.HealthResponse{Status: "ok"},
		},
		{
			name:     "SuccessMetrics",
			wantData: srv.HealthResponse{Status: "ok", Metrics: metricRes{metricName: srv.HealthMetricResult{Status: "ok"}}},
			args:     args{metrics: &[]srv.HealthMetric{baseMetric}},
		},
		{
			name:     "SuccessCustomStatus",
			wantData: srv.HealthResponse{Status: "ok", Metrics: metricRes{metricName: srv.HealthMetricResult{Status: customMetricStatus}}},
			args:     args{metrics: &[]srv.HealthMetric{customStatusMetric}},
		},
		{
			name:     "SuccessMetricInfo",
			wantData: srv.HealthResponse{Status: "ok", Metrics: metricRes{metricName: srv.HealthMetricResult{Status: "ok", Info: infoMetricInfo}}},
			args:     args{metrics: &[]srv.HealthMetric{infoMetric}},
		},
		{
			name:     "FailureBadMetric",
			wantData: srv.HealthResponse{Status: "not ok"},
			args:     args{metrics: &[]srv.HealthMetric{badMetric}},
		},
		{
			name:     "FailureBadMetricCustomStatus",
			wantData: srv.HealthResponse{Status: "not ok", Metrics: metricRes{metricName: srv.HealthMetricResult{Status: badMetricStatus}}},
			args:     args{metrics: &[]srv.HealthMetric{badMetricCustomStatus}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := srv.HealthHandler(tt.args.metrics)
			req := httptest.NewRequest("GET", "http://localhost/_system/health", nil)
			w := httptest.NewRecorder()

			// Act
			handler(w, req, nil)
			resp := w.Result()
			var data srv.HealthResponse
			err := json.NewDecoder(resp.Body).Decode(&data)

			// Assert
			assert.NoError(err)
			assert.Equal(tt.wantData.Status, data.Status)

			for metricKey, metricValue := range tt.wantData.Metrics {
				assert.NotNil(data.Metrics[metricKey])
				assert.Equal(metricValue.Status, data.Metrics[metricKey].Status)
				assert.Equal(metricValue.Info, data.Metrics[metricKey].Info)
			}
		})
	}
}

func TestInfoHandler(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	handler := srv.InfoHandler(nil)
	req := httptest.NewRequest("GET", "http://localhost/_system/info", nil)
	w := httptest.NewRecorder()

	// Act
	handler(w, req, nil)
	resp := w.Result()
	var data srv.InfoResponse
	err := json.NewDecoder(resp.Body).Decode(&data)

	// Assert
	assert.NoError(err)
	assert.NotNil(data)
}

func TestRouteHandler(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	routes := &[]srv.RouteInfo{{Method: "GET", Path: "/testpath"}}
	handler := srv.RouteHandler(routes)
	req := httptest.NewRequest("GET", "http://localhost/_system/routes", nil)
	w := httptest.NewRecorder()

	// Act
	handler(w, req, nil)
	resp := w.Result()
	var data []srv.RouteInfo
	err := json.NewDecoder(resp.Body).Decode(&data)

	// Assert
	assert.NoError(err)
	assert.Equal(*routes, data)
}

func TestNotFoundHandler(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	handler := srv.NotFoundHandler()
	req := httptest.NewRequest("GET", "http://localhost/route-not-found", nil)
	w := httptest.NewRecorder()

	// Act
	handler(w, req)
	resp := w.Result()
	var data interface{}
	err := json.NewDecoder(resp.Body).Decode(&data)

	// Assert
	assert.NoError(err)
	assert.Equal(http.StatusNotFound, resp.StatusCode)
}

func TestMethodNotAllowedHandler(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	handler := srv.MethodNotAllowedHandler()
	req := httptest.NewRequest("GET", "http://localhost/route-not-found", nil)
	w := httptest.NewRecorder()

	// Act
	handler(w, req)
	resp := w.Result()
	var data interface{}
	err := json.NewDecoder(resp.Body).Decode(&data)

	// Assert
	assert.NoError(err)
	assert.Equal(http.StatusMethodNotAllowed, resp.StatusCode)
}

func TestPanicHandler(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	handler := srv.PanicHandler()
	req := httptest.NewRequest("GET", "http://localhost/route-not-found", nil)
	w := httptest.NewRecorder()

	// Act
	handler(w, req, nil)
	resp := w.Result()
	var data interface{}
	err := json.NewDecoder(resp.Body).Decode(&data)

	// Assert
	assert.NoError(err)
	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
}
