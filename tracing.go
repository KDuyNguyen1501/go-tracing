// Package tracing khoi tao OpenTelemetry tracing cho cac service Go.
//
// Muc tieu: moi service chi can goi Init mot lan, con lai cau hinh bang
// bien moi truong chuan cua OpenTelemetry, khong nhet vao file config rieng.
//
//	shutdown, err := tracing.Init(ctx, tracing.Config{})
//	if err != nil {
//		return err
//	}
//	defer shutdown(context.Background())
package tracing

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// Config la phan cau hinh dat trong code. Moi field deu co the bo trong:
// khi do gia tri duoc lay tu bien moi truong tuong ung.
type Config struct {
	// ServiceName ghi de OTEL_SERVICE_NAME. Bat buoc phai co mot trong hai,
	// neu khong service se hien la "unknown_service" tren dashboard.
	ServiceName string

	// Endpoint ghi de OTEL_EXPORTER_OTLP_ENDPOINT.
	// Chap nhan ca "localhost:4318" lan "http://localhost:4318".
	Endpoint string

	// Environment va Version la attribute mo ta, khong bat buoc.
	Environment string
	Version     string

	// Insecure dung HTTP thay vi HTTPS khi Endpoint khong co scheme.
	// Mac dinh true vi endpoint noi bo hau het chay HTTP.
	Insecure *bool

	// Disabled tat han tracing. Tuong duong OTEL_SDK_DISABLED=true.
	Disabled bool
}

// ShutdownFunc flush not cac span con trong buffer roi dong provider.
// Luon goi no truoc khi thoat, neu khong nhung span cuoi cung se mat.
type ShutdownFunc func(context.Context) error

// Init dung TracerProvider, dat no lam global va bat propagation W3C.
//
// Sampler KHONG duoc set trong code, de OTEL_TRACES_SAMPLER va
// OTEL_TRACES_SAMPLER_ARG dieu khien. Khong co env thi mac dinh cua SDK la
// ParentBased(AlwaysSample) - lay 100%, hop cho local va staging.
func Init(ctx context.Context, cfg Config) (ShutdownFunc, error) {
	if cfg.Disabled || envBool("OTEL_SDK_DISABLED") {
		return noopShutdown, nil
	}

	res, err := buildResource(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("tracing: build resource: %w", err)
	}

	exp, err := otlptracehttp.New(ctx, exporterOptions(cfg)...)
	if err != nil {
		return nil, fmt.Errorf("tracing: create otlp exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exp),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

// buildResource gom cac attribute mo ta service.
//
// Chu y: khong dung resource.Merge(resource.Default(), ...) voi semconv khac
// version SDK - Merge se tra ve loi "conflicting Schema URL" va resource mat
// sach attribute, dan toi service hien thanh "unknown_service:<binary>".
// resource.New gom moi detector trong cung mot schema nen khong dinh loi do.
func buildResource(ctx context.Context, cfg Config) (*resource.Resource, error) {
	attrs := make([]attribute.KeyValue, 0, 3)
	if cfg.ServiceName != "" {
		attrs = append(attrs, semconv.ServiceName(cfg.ServiceName))
	}
	if cfg.Version != "" {
		attrs = append(attrs, semconv.ServiceVersion(cfg.Version))
	}
	if env := firstNonEmpty(cfg.Environment, os.Getenv("APP_ENV")); env != "" {
		attrs = append(attrs, semconv.DeploymentEnvironment(env))
	}

	return resource.New(ctx,
		resource.WithFromEnv(), // OTEL_SERVICE_NAME, OTEL_RESOURCE_ATTRIBUTES
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithAttributes(attrs...),
	)
}

func exporterOptions(cfg Config) []otlptracehttp.Option {
	if cfg.Endpoint == "" {
		// Khong set gi ca: exporter tu doc OTEL_EXPORTER_OTLP_ENDPOINT,
		// ke ca phan scheme va duong dan.
		return nil
	}

	host, secure := parseEndpoint(cfg.Endpoint)
	opts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(host)}

	insecure := !secure
	if cfg.Insecure != nil {
		insecure = *cfg.Insecure
	}
	if insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	return opts
}

// parseEndpoint tach "http://host:4318" thanh ("host:4318", false).
// Chuoi khong co scheme duoc tra ve nguyen ven, secure = false.
func parseEndpoint(endpoint string) (host string, secure bool) {
	if !strings.Contains(endpoint, "://") {
		return endpoint, false
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return endpoint, false
	}
	return u.Host, u.Scheme == "https"
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func envBool(key string) bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(key)), "true")
}

func noopShutdown(context.Context) error { return nil }
