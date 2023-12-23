package main

import (
	"context"
	"encoding/json"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.18.0"
	"jaeger-otel-http/log"
	"sync"
	"time"
)

const (
	traceName = "mxshop-otel"
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
				semconv.ServiceNameKey.String("mxshop-user-http"),
				// 设置Process键值对 可以让其他人员分析 全局的，设置到trace上的
				attribute.String("environment", "dev"),
				attribute.Int("ID", 1),
			),
		),
	)
	otel.SetTracerProvider(tp)
	// 设置传播提取器
	return nil
}
func funcA(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	tr := otel.Tracer(traceName)
	spCtx, span := tr.Start(ctx, "func-a")
	span.SetAttributes(attribute.String("name", "funA"))
	type _LogStruct struct {
		CurrentTime time.Time `json:"currentTime"`
		PassWho     string    `json:"passWho"`
		Name        string    `json:"name"`
	}
	logTest := _LogStruct{
		CurrentTime: time.Time{},
		PassWho:     "jzin",
		Name:        "func-a",
	}
	log.InfofC(spCtx, "is logs")
	b, _ := json.Marshal(logTest)
	log.InfofC(spCtx, string(b))
	span.SetAttributes(attribute.Key("测试key").String(string(b)))
	time.Sleep(time.Second)
	span.End()
}
func funcB(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	tr := otel.Tracer(traceName)
	spCtx, span := tr.Start(ctx, "func-b")
	span.SetAttributes(attribute.String("name", "funB"))
	type _LogStruct struct {
		CurrentTime time.Time `json:"currentTime"`
		PassWho     string    `json:"passWho"`
		Name        string    `json:"name"`
	}
	logTest := _LogStruct{
		CurrentTime: time.Time{},
		PassWho:     "bjzin",
		Name:        "func-b",
	}
	log.InfofC(spCtx, "is logs b")
	b, _ := json.Marshal(logTest)
	log.InfofC(spCtx, string(b))
	span.SetAttributes(attribute.Key("测试key").String(string(b)))
	time.Sleep(time.Second)
	span.End()
}
func main() {
	_ = tracerProvider()
	ctx, cancel := context.WithCancel(context.Background())
	defer func(ctx context.Context) {
		ctx, cancel = context.WithTimeout(ctx, time.Second*5)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			panic(err)
		}
	}(ctx)
	tr := otel.Tracer(traceName)
	spanCtx, span := tr.Start(ctx, "func-main")
	wg := &sync.WaitGroup{}
	wg.Add(2)
	go funcA(spanCtx, wg)
	go funcB(spanCtx, wg)
	// 设置logs
	span.AddEvent("this is an event")
	time.Sleep(time.Second)
	wg.Wait()
	span.End()
}
