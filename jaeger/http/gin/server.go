package main

import (
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.18.0"
	"time"
)

const (
	traceName = "mxshop-otel-gin-server"
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
				semconv.ServiceNameKey.String("mxshop-user-gin-server"),
				// 设置Process键值对 可以让其他人员分析 全局的，设置到trace上的
				attribute.String("environment", "dev"),
				attribute.Int("ID", 1),
			),
		),
	)
	otel.SetTracerProvider(tp)
	// 全局设置传播提取器
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return nil
}
func Server(c *gin.Context) {
	// 负责span的抽取和生成
	ctx := c.Request.Context()
	p := otel.GetTextMapPropagator()
	// 生成新的context
	sCtx := p.Extract(ctx, propagation.HeaderCarrier(c.Request.Header))
	// 拿到tracer
	tr := tp.Tracer(traceName)
	_, span := tr.Start(sCtx, "server")
	time.Sleep(time.Millisecond * 500)
	span.End()
	c.JSON(200, gin.H{})
}
func main() {
	_ = tracerProvider()
	r := gin.Default()
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{})
	})
	r.GET("/server", Server)
	if err := r.Run(":8090"); err != nil {
		panic(err)
	}
}
