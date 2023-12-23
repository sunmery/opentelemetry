package main

import (
	"context"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.18.0"
)

const (
	traceName = "mxshop-otel-redis"
)

var tp *trace.TracerProvider

func tracerProvider() error {
	// url := "http://127.0.0.1:14268/api/traces"
	url := "http://192.168.2.152:32561/api/traces"
	jexp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(url)))
	if err != nil {
		panic(err)
	}

	// 上报器 批量处理链路追踪器
	tp = trace.NewTracerProvider(
		trace.WithBatcher(jexp),
		// 如果未使用此选项，跟踪程序提供程序将使用该资源 默认资源。
		trace.WithResource(
			resource.NewWithAttributes(
				// 固定写法
				semconv.SchemaURL,
				// 设置service
				semconv.ServiceNameKey.String("mxshop-user-redis"),
				// 设置Process键值对 可以让其他人员分析 全局的，设置到trace上的
				attribute.String("environment", "dev"),
				attribute.Int("ID", 1),
			),
		),
	)
	otel.SetTracerProvider(tp)
	// 设置传播提取器
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return nil
}

func main() {
	_ = tracerProvider()
	cli := redis.NewClient(&redis.Options{
		// Addr: "127.0.0.1:端口",
		Addr:     "192.168.2.158:6379",
		Username: "default",
		Password: "263393",
	})
	// Enable tracing instrumentation.
	if err := redisotel.InstrumentTracing(cli); err != nil {
		panic(err)
	}
	tr := otel.Tracer(traceName)
	spanCtx, span := tr.Start(context.Background(), "redis")
	cli.Set(spanCtx, "name", "jzin", 0)
	span.End()
	err := tp.Shutdown(context.Background())
	if err != nil {
		return
	}
}
