package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
)

type summaryResponse struct {
	ClusterName             string                 `json:"clusterName"`
	Provider                string                 `json:"provider"`
	Region                  string                 `json:"region"`
	TotalHourlyCost         float64                `json:"totalHourlyCost"`
	TotalCpuCores           float64                `json:"totalCpuCores"`
	TotalCpuRequestedCores  float64                `json:"totalCpuRequestedCores"`
	TotalMemoryGiB          float64                `json:"totalMemoryGiB"`
	TotalMemoryRequestedGiB float64                `json:"totalMemoryRequestedGiB"`
	TopNamespaces           []map[string]any       `json:"topNamespaces"`
	CostByLabel             map[string][]labelCost `json:"costByLabel"`
	CostByInstanceType      []instanceTypeCost     `json:"costByInstanceType"`
}

type labelCost struct {
	Value      string  `json:"value"`
	HourlyCost float64 `json:"hourlyCost"`
}

type instanceTypeCost struct {
	InstanceType string  `json:"instanceType"`
	NodeCount    int     `json:"nodeCount"`
	HourlyCost   float64 `json:"hourlyCost"`
}

func main() {
	listen := flag.String("listen", ":8080", "listen address")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"status": "healthy", "version": "mock-1"})
	})

	mux.HandleFunc("/api/cost/summary", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, summaryResponse{
			ClusterName:             "dev-cluster",
			Provider:                "aws",
			Region:                  "us-east-1",
			TotalHourlyCost:         12.34,
			TotalCpuCores:           64,
			TotalCpuRequestedCores:  52,
			TotalMemoryGiB:          256,
			TotalMemoryRequestedGiB: 210,
			TopNamespaces: []map[string]any{
				{"namespace": "payments", "hourlyCost": 4.2},
				{"namespace": "api", "hourlyCost": 3.1},
			},
			CostByLabel: map[string][]labelCost{
				"team": {
					{Value: "backend", HourlyCost: 6.8},
					{Value: "ml", HourlyCost: 2.1},
				},
				"env": {
					{Value: "prod", HourlyCost: 10.1},
					{Value: "staging", HourlyCost: 2.2},
				},
			},
			CostByInstanceType: []instanceTypeCost{
				{InstanceType: "m5.large", NodeCount: 4, HourlyCost: 0.384},
				{InstanceType: "m5.xlarge", NodeCount: 2, HourlyCost: 0.768},
			},
		})
	})

	mux.HandleFunc("/api/cost/namespaces", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]any{
			{
				"namespace":          "payments",
				"team":                "backend",
				"env":                 "prod",
				"hourlyCost":          4.2,
				"cpuRequestedCores":   3.5,
				"cpuUsedCores":        2.8,
				"memoryRequestedGiB":  8,
				"memoryUsedGiB":       6.4,
				"podCount":            23,
			},
			{
				"namespace":          "api",
				"team":                "platform",
				"env":                 "prod",
				"hourlyCost":          3.5,
				"cpuRequestedCores":   2.7,
				"cpuUsedCores":        2.2,
				"memoryRequestedGiB":  6,
				"memoryUsedGiB":       4.8,
				"podCount":            18,
			},
		})
	})

	mux.HandleFunc("/api/cost/nodes", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]any{
			{
				"name":                 "ip-10-0-1-23",
				"instanceType":         "m5.large",
				"availabilityZone":     "us-east-1a",
				"rawNodePriceHourly":   0.096,
				"allocatedCostHourly":  0.087,
				"cpuAllocatableCores":  4,
				"cpuRequestedCores":    3.6,
				"cpuUsedCores":         2.9,
				"memoryAllocatableGiB": 16,
				"memoryRequestedGiB":   14.2,
				"memoryUsedGiB":        10.8,
			},
		})
	})

	mux.HandleFunc("/api/cost/workloads", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]any{
			{
				"namespace":          "payments",
				"workloadKind":       "Deployment",
				"workloadName":       "payments-api",
				"team":               "backend",
				"env":                "prod",
				"replicas":           6,
				"hourlyCost":         3.42,
				"cpuRequestedCores":  2.4,
				"cpuUsedCores":       1.8,
				"memoryRequestedGiB": 6,
				"memoryUsedGiB":      4.2,
				"nodes":              []string{"ip-10-0-1-12", "ip-10-0-2-04"},
			},
		})
	})

	mux.HandleFunc("/api/cost/pods", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]any{
			{
				"namespace":          "payments",
				"podName":            "payments-api-123",
				"nodeName":           "ip-10-0-1-23",
				"hourlyCost":         0.32,
				"cpuRequestedCores":  0.4,
				"cpuUsedCores":       0.32,
				"memoryRequestedGiB": 0.9,
				"memoryUsedGiB":      0.7,
			},
		})
	})

	log.Printf("mock agent listening on %s", *listen)
	log.Fatal(http.ListenAndServe(*listen, mux))
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}
