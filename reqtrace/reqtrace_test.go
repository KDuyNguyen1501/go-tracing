package reqtrace_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KDuyNguyen1501/go-tracing/reqtrace"
	"github.com/imroc/req/v3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// setup dung mot TracerProvider ghi span vao bo nho, giong nhu Init that
// nhung khong can backend nao.
func setup(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(sr),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return sr
}

// Service nhan phai doc duoc traceparent va noi tiep DUNG trace cua caller.
func TestWrap_InjectTraceparent(t *testing.T) {
	sr := setup(t)

	var serverSideTrace trace.TraceID
	var serverSideParent trace.SpanID
	var rawHeader string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawHeader = r.Header.Get("traceparent")
		// Mo phong dung viec otelgin lam o service nhan.
		sc := trace.SpanContextFromContext(
			otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header)),
		)
		serverSideTrace = sc.TraceID()
		serverSideParent = sc.SpanID()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cli := reqtrace.Wrap(req.C().SetBaseURL(srv.URL))

	ctx, root := otel.Tracer("caller").Start(context.Background(), "root")
	if _, err := cli.R().SetContext(ctx).Get("/price"); err != nil {
		t.Fatalf("request loi: %v", err)
	}
	root.End()

	if rawHeader == "" {
		t.Fatal("khong co header traceparent nao duoc gui di")
	}

	rootTrace := root.SpanContext().TraceID()
	if serverSideTrace != rootTrace {
		t.Errorf("traceID khong khop: server=%s caller=%s", serverSideTrace, rootTrace)
	}

	// Span cha ma service nhan thay phai la span CLIENT vua tao, khong phai root.
	var clientSpanID trace.SpanID
	for _, s := range sr.Ended() {
		if s.SpanKind() == trace.SpanKindClient {
			clientSpanID = s.SpanContext().SpanID()
			if s.Name() != "GET /price" {
				t.Errorf("ten span = %q, muon %q", s.Name(), "GET /price")
			}
			if s.Parent().SpanID() != root.SpanContext().SpanID() {
				t.Error("span client khong treo vao root span")
			}
		}
	}
	if !clientSpanID.IsValid() {
		t.Fatal("khong tao duoc span nao cho request di ra")
	}
	if serverSideParent != clientSpanID {
		t.Errorf("parent phia server=%s, muon span client=%s", serverSideParent, clientSpanID)
	}
}

// Khong SetContext thi van chay duoc, chi la span thanh root roi.
func TestWrap_KhongCoContext(t *testing.T) {
	sr := setup(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cli := reqtrace.Wrap(req.C().SetBaseURL(srv.URL))
	if _, err := cli.R().Get("/ping"); err != nil {
		t.Fatalf("request loi: %v", err)
	}

	if len(sr.Ended()) != 1 {
		t.Fatalf("muon 1 span, co %d", len(sr.Ended()))
	}
	if sr.Ended()[0].Parent().IsValid() {
		t.Error("khong truyen context ma span van co cha")
	}
}

// Loi HTTP phai duoc danh dau tren span de loc duoc tren dashboard.
func TestWrap_StatusLoi(t *testing.T) {
	sr := setup(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "khong tim thay", http.StatusNotFound)
	}))
	defer srv.Close()

	cli := reqtrace.Wrap(req.C().SetBaseURL(srv.URL))
	if _, err := cli.R().Get("/khong-ton-tai"); err != nil {
		t.Fatalf("request loi: %v", err)
	}

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("muon 1 span, co %d", len(spans))
	}
	if spans[0].Status().Code.String() != "Error" {
		t.Errorf("status span = %v, muon Error", spans[0].Status().Code)
	}
}

// Body chi duoc ghi khi bat option, vi no hay chua token va thong tin khach.
func TestWrap_KhongGhiBodyTheoMacDinh(t *testing.T) {
	sr := setup(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cli := reqtrace.Wrap(req.C().SetBaseURL(srv.URL))
	if _, err := cli.R().SetBodyString(`{"token":"bi-mat"}`).Post("/login"); err != nil {
		t.Fatalf("request loi: %v", err)
	}

	for _, attr := range sr.Ended()[0].Attributes() {
		if string(attr.Key) == "http.request.body" {
			t.Fatal("body bi ghi vao span du chua bat option")
		}
	}
}
