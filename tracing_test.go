package tracing

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

func TestParseEndpoint(t *testing.T) {
	cases := []struct {
		in         string
		wantHost   string
		wantSecure bool
	}{
		{"localhost:4318", "localhost:4318", false},
		{"http://localhost:4318", "localhost:4318", false},
		{"https://otel.citigo.net:4318", "otel.citigo.net:4318", true},
		{"http://jaeger-collector:4318/v1/traces", "jaeger-collector:4318", false},
		{"", "", false},
	}
	for _, c := range cases {
		host, secure := parseEndpoint(c.in)
		if host != c.wantHost || secure != c.wantSecure {
			t.Errorf("parseEndpoint(%q) = (%q, %v), muon (%q, %v)",
				c.in, host, secure, c.wantHost, c.wantSecure)
		}
	}
}

// Khi tat, Init khong duoc dung provider that de tranh gui span di dau ca.
func TestInit_Disabled(t *testing.T) {
	otel.SetTracerProvider(trace.NewNoopTracerProvider())

	shutdown, err := Init(context.Background(), Config{Disabled: true})
	if err != nil {
		t.Fatalf("Init loi: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("shutdown loi: %v", err)
	}

	_, span := otel.Tracer("t").Start(context.Background(), "test")
	defer span.End()
	if span.SpanContext().IsValid() {
		t.Error("tracing da tat ma span van duoc ghi")
	}
}

// OTEL_SDK_DISABLED co tac dung tuong duong Config.Disabled.
func TestInit_DisabledTuEnv(t *testing.T) {
	t.Setenv("OTEL_SDK_DISABLED", "true")
	otel.SetTracerProvider(trace.NewNoopTracerProvider())

	if _, err := Init(context.Background(), Config{ServiceName: "x"}); err != nil {
		t.Fatalf("Init loi: %v", err)
	}

	_, span := otel.Tracer("t").Start(context.Background(), "test")
	defer span.End()
	if span.SpanContext().IsValid() {
		t.Error("OTEL_SDK_DISABLED khong duoc ton trong")
	}
}

// Resource phai giu duoc service.name - day la cho bug resource.Merge hay xay ra.
func TestBuildResource_GiuServiceName(t *testing.T) {
	res, err := buildResource(context.Background(), Config{
		ServiceName: "check-price",
		Version:     "v1.2.3",
		Environment: "local",
	})
	if err != nil {
		t.Fatalf("buildResource loi: %v", err)
	}

	got := map[string]string{}
	for _, attr := range res.Attributes() {
		got[string(attr.Key)] = attr.Value.AsString()
	}
	for key, want := range map[string]string{
		"service.name":           "check-price",
		"service.version":        "v1.2.3",
		"deployment.environment": "local",
	} {
		if got[key] != want {
			t.Errorf("%s = %q, muon %q", key, got[key], want)
		}
	}
}

// Init that phai dat duoc global provider ghi span.
func TestInit_DatGlobalProvider(t *testing.T) {
	otel.SetTracerProvider(trace.NewNoopTracerProvider())

	shutdown, err := Init(context.Background(), Config{
		ServiceName: "check-price",
		Endpoint:    "localhost:4318", // khong can co ai lang nghe: exporter gui nen
	})
	if err != nil {
		t.Fatalf("Init loi: %v", err)
	}
	defer shutdown(context.Background())

	_, span := otel.Tracer("t").Start(context.Background(), "test")
	defer span.End()
	if !span.SpanContext().TraceID().IsValid() {
		t.Error("span khong co traceID hop le")
	}
}
