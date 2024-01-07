package main

import (
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.18.0"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
	"gorm.io/plugin/opentelemetry/tracing"
)

const (
	traceName = "mxshop-otel-gorm"
)

type BaseModel struct {
	ID        int32          `gorm:"primary_key;comment:ID"`
	CreatedAt time.Time      `gorm:"column:add_time;comment:创建时间"`
	UpdatedAt time.Time      `gorm:"column:update_time;comment:更新时间"`
	DeletedAt gorm.DeletedAt `gorm:"comment:删除时间"`
	IsDeleted bool           `gorm:"comment:是否删除"`
}
type User struct {
	BaseModel
	Mobile   string     `gorm:"index:idx_mobile;unique;type:varchar(11);not null;comment:手机号"`
	Password string     `gorm:"type:varchar(100);not null;comment:密码"`
	NickName string     `gorm:"type:varchar(20);comment:账号名称"`
	Birthday *time.Time `gorm:"type:datetime;comment:出生日期"`
	Gender   string     `gorm:"column:gender;default:male;type:varchar(6);comment:femail表示女,male表示男"`
	Role     int        `gorm:"column:role;default:1;type:int;comment:1表示普通用户,2表示管理员"`
}

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
				semconv.ServiceNameKey.String("mxshop-user-gorm"),
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

func Server2(c *gin.Context) {
	// dsn := "root:123456@tcp(localhost:3306)/mxshop_user_srv2?charset=utf8mb4&parseTime=True&loc=Local"
	dsn := "postgresql://dbuser_dba:DBUser.DBA@192.168.2.158:5432/meta"
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			LogLevel: logger.Info,
			Colorful: true,
		},
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
		Logger: newLogger,
	})
	if err != nil {
		panic(err)
	}
	// 之前初始化好了*trace.TracerProvider这里就不用再初始化了
	if err := db.Use(tracing.NewPlugin()); err != nil {
		panic(err)
	}

	// 负责span的抽取和生成
	// 如果使用中间件otelgin.Middleware 它会把自己trace span，把context放到c.Request.Context()中
	// ctx := c.Request.Context()
	// p := otel.GetTextMapPropagator()
	// tr := tp.Tracer(traceName)
	// sCtx := p.Extract(ctx, propagation.HeaderCarrier(c.Request.Header))
	// spanCtx, span := tr.Start(sCtx, "server")

	if err := db.WithContext(c.Request.Context()).Model(User{BaseModel: BaseModel{ID: 12}}).First(&User{}).Error; err != nil {
		panic(err)
	}

	time.Sleep(500 * time.Millisecond)
	// span.End()
	c.JSON(200, gin.H{})

}

func main() {
	_ = tracerProvider()
	r := gin.Default()
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{})
	})
	r.GET("/server", Server2)
	err := r.Run(":8090")
	if err != nil {
		return
	}
}
