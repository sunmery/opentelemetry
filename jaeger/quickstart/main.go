package main

import (
	"context"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.18.0"
	"time"
)

func main() {
	// url := "http://127.0.0.1:14268/api/traces"
	url := "http://192.168.2.152:32561/api/traces"
	// 专门生成Exporter
	jexp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(url)))
	if err != nil {
		panic(err)
	}
	// 后续使用telemetry，和jaeger无关 ↓初始化
	tp := trace.NewTracerProvider(
		// 上报器 批量上报处理链路
		trace.WithBatcher(jexp),
		// 如果未使用此选项，跟踪程序提供程序将使用该资源 默认资源。
		trace.WithResource(
			resource.NewWithAttributes(
				// 固定写法
				semconv.SchemaURL,
				// 设置service
				semconv.ServiceNameKey.String("mxshop-user"),
				// 设置Process键值对 可以让其他人员分析 全局的，设置到trace上的
				attribute.String("environment", "dev"),
				attribute.Int("ID", 1),
			),
		),
	)
	otel.SetTracerProvider(tp)

	// 自己创建context
	ctx, cancel := context.WithCancel(context.Background())
	// 优雅退出
	defer func(ctx context.Context) {
		ctx, cancel = context.WithTimeout(ctx, time.Second*5)
		defer cancel()
		if err = tp.Shutdown(ctx); err != nil {
			panic(err)
		}
	}(ctx)

	// 生成tracer
	tr := otel.Tracer("mxshop-otel")
	// 生成span
	_, span := tr.Start(ctx, "func-main")
	// 业务逻辑
	time.Sleep(time.Second)
	var attrs []attribute.KeyValue
	attrs = append(attrs, attribute.String("key1", "value1"))
	attrs = append(attrs, attribute.Bool("key2", false))
	attrs = append(attrs, attribute.Int("key2", 123))
	attrs = append(attrs, attribute.StringSlice("key2", []string{"value1", "value2"}))

	// 设置span里的Tags键值对
	span.SetAttributes(attrs...)
	// 设置logs
	span.AddEvent("this is an event")
	// 业务逻辑
	time.Sleep(time.Second)
	// 结束此span
	span.End()
}
