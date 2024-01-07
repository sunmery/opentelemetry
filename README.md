## 安装
### Ops
#### [OpenTelemetry Operator Chart](https://opentelemetry.io/docs/kubernetes/helm/operator/)

- --namespace 命名空间
```bash
kubectl create ns opentelemetry-operator

helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
helm repo update
helm install opentelemetry-operator open-telemetry/opentelemetry-operator \
--set admissionWebhooks.certManager.enabled=false \
--set admissionWebhooks.certManager.autoGenerateCert.enabled=true \
--namespace opentelemetry-operator
```
#### OpenTelemetry Collector

需要修改的的点:
- jaeger-collector的地址: 例如jaeger-collector.istio-system.svc.cluster.local:14250
- prometheus导出到的端点和namespace
- SVC类型, 本例使用LoadBalancer, 也可以使用NodePort

```yaml
# Copyright The OpenTelemetry Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

apiVersion: v1
kind: ConfigMap
metadata:
  name: otel-collector-conf
  namespace: observability
  labels:
    app: opentelemetry
    component: otel-collector-conf
data:
  otel-collector-config: |
    receivers:
      # Make sure to add the otlp receiver.
      # This will open up the receiver on port 4317
      otlp:
        protocols:
          grpc:
            endpoint: "0.0.0.0:4317" # 导出到的端点
    processors:
    extensions:
      health_check: {}
    exporters:
      jaeger:
        #endpoint: "jaeger-collector.observability.svc.cluster.local:14250"
        endpoint: "jaeger-collector.istio-system.svc.cluster.local:14250" # Jaeger Collector的地址
        insecure: true
      prometheus:
        endpoint: 0.0.0.0:8889  # 导出到0.0.0.0:8889该端口, 然后让Prometheus去抓取
        namespace: "monitoring" # Prometheus的namespace
      logging:

    service:
      extensions: [health_check]
      pipelines:
        traces:
          receivers: [otlp]
          processors: []
          exporters: [jaeger]

        metrics:
          receivers: [otlp]
          processors: []
          exporters: [prometheus, logging]
---
apiVersion: v1
kind: Service
metadata:
  name: otel-collector
  namespace: observability
  labels:
    app: opentelemetry
    component: otel-collector
spec:
  ports:
    - name: otlp # Default endpoint for otlp receiver.
      port: 4317
      protocol: TCP
      targetPort: 4317
      nodePort: 30080
    - name: metrics # Default endpoint for metrics.
      port: 8889
      protocol: TCP
      targetPort: 8889
  selector:
    component: otel-collector
  type: LoadBalancer # SVC类型
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: otel-collector
  namespace: observability
  labels:
    app: opentelemetry
    component: otel-collector
spec:
  selector:
    matchLabels:
      app: opentelemetry
      component: otel-collector
  minReadySeconds: 5
  progressDeadlineSeconds: 120
  replicas: 1 #TODO - adjust this to your own requirements
  template:
    metadata:
      annotations:
        prometheus.io/path: "/metrics"
        prometheus.io/port: "8889"
        prometheus.io/scrape: "true"
      labels:
        app: opentelemetry
        component: otel-collector
    spec:
      containers:
        - command:
            - "/otelcol"
            - "--config=/conf/otel-collector-config.yaml"
            # Memory Ballast size should be max 1/3 to 1/2 of memory.
            - "--mem-ballast-size-mib=683"
          env:
            - name: GOGC
              value: "80"
          image: otel/opentelemetry-collector:0.6.0
          name: otel-collector
          resources:
            limits:
              cpu: "1"
              memory: 2Gi
            requests:
              cpu: 200m
              memory: 400Mi
          ports:
            - containerPort: 4317 # Default endpoint for otlp receiver.
            - containerPort: 8889 # Default endpoint for querying metrics.
          volumeMounts:
            - name: otel-collector-config-vol
              mountPath: /conf
          # - name: otel-collector-secrets
          #   mountPath: /secrets
          livenessProbe:
            httpGet:
              path: /
              port: 13133 # Health Check extension default port.
          readinessProbe:
            httpGet:
              path: /
              port: 13133 # Health Check extension default port.
      volumes:
        - configMap:
            name: otel-collector-conf
            items:
              - key: otel-collector-config
                path: otel-collector-config.yaml
          name: otel-collector-config-vol
#        - secret:
#            name: otel-collector-secrets
#            items:
#              - key: cert.pem
#                path: cert.pem
#              - key: key.pem
#                path: key.pem

```
## Dev
1. 创建mod并运行该代码:
```shell
go mod init otel-test
go mod tidy
```
2. 运行代码
将`OTEL_URL` 替换为你的otel-collector的真实地址
```go
// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Example using OTLP exporters + collector + third-party backends. For
// information about using the exporter, see:
// https://pkg.go.dev/go.opentelemetry.io/otel/exporters/otlp?tab=doc#example-package-Insecure
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

// otel collector的地址
const OTEL_URL = "192.168.2.152:30080"

// Initializes an OTLP exporter, and configures the corresponding trace and
// metric providers.
func initProvider() (func(context.Context) error, error) {
	ctx := context.Background()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			// the service name used to display traces in backends
			semconv.ServiceName("test-service"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// If the OpenTelemetry Collector is running on a local cluster (minikube or
	// microk8s), it should be accessible through the NodePort service at the
	// `localhost:30080` endpoint. Otherwise, replace `localhost` with the
	// endpoint of your cluster. If you run the app inside k8s, then you can
	// probably connect directly to the service through dns.
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, OTEL_URL,
		// Note the use of insecure transport here. TLS is recommended in production.
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection to collector: %w", err)
	}

	// Set up a trace exporter
	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Register the trace exporter with a TracerProvider, using a batch
	// span processor to aggregate spans before export.
	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)
	otel.SetTracerProvider(tracerProvider)

	// set global propagator to tracecontext (the default is no-op).
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Shutdown will flush any remaining spans and shut down the exporter.
	return tracerProvider.Shutdown, nil
}

func main() {
	log.Printf("Waiting for connection...")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	shutdown, err := initProvider()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			log.Fatal("failed to shutdown TracerProvider: %w", err)
		}
	}()

	tracer := otel.Tracer("test-tracer")

	// Attributes represent additional key-value descriptors that can be bound
	// to a metric observer or recorder.
	commonAttrs := []attribute.KeyValue{
		attribute.String("attrA", "chocolate"),
		attribute.String("attrB", "raspberry"),
		attribute.String("attrC", "vanilla"),
	}

	// work begins
	ctx, span := tracer.Start(
		ctx,
		"CollectorExporter-Example",
		trace.WithAttributes(commonAttrs...))
	defer span.End()
	for i := 0; i < 10; i++ {
		_, iSpan := tracer.Start(ctx, fmt.Sprintf("Sample-%d", i))
		log.Printf("Doing really hard work (%d / 10)\n", i+1)

		<-time.After(time.Second)
		iSpan.End()
	}

	log.Printf("Done!")
}
```

## 资料
1. https://github.com/open-telemetry/opentelemetry-go/tree/main/example/otel-collector
