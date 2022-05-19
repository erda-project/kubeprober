package fluentbit_checker

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/sirupsen/logrus"
)

type FluentBitMetric struct {
	Input  map[string]FluentBitInputMetric  `json:"input"`
	Filter map[string]FluentBitFilterMetric `json:"filter"`
	Output map[string]FluentBitOutputMetric `json:"output"`
}

type FluentBitInputMetric struct {
	Records      int `json:"records"`
	Bytes        int `json:"bytes"`
	FilesOpened  int `json:"files_opened"`
	FilesClosed  int `json:"files_closed"`
	FilesRotated int `json:"files_rotated"`
}

type FluentBitFilterMetric struct {
	DropRecords int `json:"drop_records"`
	AddRecords  int `json:"add_records"`
}

type FluentBitOutputMetric struct {
	ProcRecords    int `json:"proc_records"`
	ProcBytes      int `json:"proc_bytes"`
	Errors         int `json:"errors"`
	Retries        int `json:"retries"`
	RetriesFailed  int `json:"retries_failed"`
	DroppedRecords int `json:"dropped_records"`
	RetriedRecords int `json:"retried_records"`
}

func getMetricByIp(ip string) (*FluentBitMetric, error) {
	resp, err := http.Get(fmt.Sprintf("http://%v:2020/api/v1/metrics", ip))
	if err != nil {
		logrus.Errorf("failed to get metric, error: %v", err)
		return nil, err
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		logrus.Errorf("failed to read from response, error: %v", err)
		return nil, err
	}
	r := &FluentBitMetric{}
	if err = json.Unmarshal(data, &r); err != nil {
		logrus.Errorf("failed to unmarshal response, error: %v", err)
		return nil, err
	}
	return r, nil
}

func (metric FluentBitMetric) totalProc() (int, int) {
	inputProc := 0
	outputProc := 0
	for _, outputMetric := range metric.Output {
		outputProc += outputMetric.ProcRecords
	}
	for _, inputMetric := range metric.Input {
		inputProc += inputMetric.Records
	}
	return inputProc, outputProc
}
