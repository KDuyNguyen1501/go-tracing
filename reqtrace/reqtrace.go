// Package reqtrace gan OpenTelemetry vao HTTP client cua github.com/imroc/req/v3.
//
// No lam hai viec ma mot client chua duoc boc se thieu:
//
//  1. Tao span con cho moi request di ra, gan vao trace hien tai.
//  2. Inject header traceparent (W3C) de service nhan noi tiep DUNG trace do.
//
// Thieu viec thu hai la loi hay gap nhat: span van hien tren dashboard, nhung
// service ben kia tao mot trace ROOT moi, nen waterfall dut ngay tai bien gioi
// giua hai service.
//
//	cli := req.C().SetBaseURL(host)
//	reqtrace.Wrap(cli)
//
// Nho truyen context xuong request, neu khong span se khong co cha:
//
//	cli.R().SetContext(ctx).Get(path)
package reqtrace

import (
	"fmt"
	"net/http"

	"github.com/imroc/req/v3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

const defaultTracerName = "github.com/KDuyNguyen1501/go-tracing/reqtrace"

type config struct {
	tracerName     string
	spanName       func(*req.Request) string
	recordReqBody  bool
	recordRespBody bool
}

// Option tuy chinh hanh vi cua Wrap.
type Option func(*config)

// WithTracerName doi ten tracer, mac dinh la duong dan cua package nay.
func WithTracerName(name string) Option {
	return func(c *config) { c.tracerName = name }
}

// WithSpanNameFormatter tu quyet dinh ten span.
//
// Mac dinh la "<METHOD> <path>". Neu path co chua ID (/order/12345) thi so
// luong ten span se phinh ra rat nhanh; luc do nen dung ham nay de tra ve
// dang route co dinh, vi du "POST /order/{id}".
func WithSpanNameFormatter(f func(*req.Request) string) Option {
	return func(c *config) { c.spanName = f }
}

// WithRequestBody ghi body request vao span.
//
// Mac dinh tat: body thuong chua token, mat khau, thong tin khach hang, va
// lam span phinh to. Chi bat o moi truong dev.
func WithRequestBody() Option {
	return func(c *config) { c.recordReqBody = true }
}

// WithResponseBody ghi body response vao span khi status khong thanh cong.
// Mac dinh tat, cung ly do voi WithRequestBody.
func WithResponseBody() Option {
	return func(c *config) { c.recordRespBody = true }
}

// Wrap gan middleware tracing vao client va tra lai chinh client do.
//
// Goi mot lan luc khoi tao client, khong goi lai cho tung request.
func Wrap(c *req.Client, opts ...Option) *req.Client {
	cfg := config{
		tracerName: defaultTracerName,
		spanName:   defaultSpanName,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	c.WrapRoundTripFunc(func(rt req.RoundTripper) req.RoundTripFunc {
		return func(r *req.Request) (*req.Response, error) {
			tracer := otel.GetTracerProvider().Tracer(cfg.tracerName)

			ctx, span := tracer.Start(r.Context(), cfg.spanName(r),
				trace.WithSpanKind(trace.SpanKindClient))
			defer span.End()

			span.SetAttributes(
				semconv.HTTPMethod(r.Method),
				semconv.HTTPURL(r.URL.String()),
				semconv.NetPeerName(r.URL.Hostname()),
			)
			if cfg.recordReqBody && len(r.Body) > 0 {
				span.SetAttributes(attribute.String("http.request.body", string(r.Body)))
			}

			// Day moi la phan quan trong: bao cho service nhan biet no dang
			// nam trong trace nao. Phai inject ctx CUA SPAN CON vua tao,
			// khong phai ctx cha, neu khong cay span se treo sai nhanh.
			if r.Headers == nil {
				r.Headers = make(http.Header)
			}
			otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(r.Headers))

			resp, err := rt.RoundTrip(r)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				return resp, err
			}
			if resp == nil || resp.Response == nil {
				return resp, nil
			}

			span.SetAttributes(semconv.HTTPStatusCode(resp.StatusCode))
			if resp.StatusCode >= http.StatusBadRequest {
				span.SetStatus(codes.Error, resp.Status)
				if cfg.recordRespBody {
					span.SetAttributes(attribute.String("http.response.body", resp.String()))
				}
			}
			return resp, nil
		}
	})

	return c
}

func defaultSpanName(r *req.Request) string {
	path := r.URL.Path
	if path == "" {
		path = "/"
	}
	return fmt.Sprintf("%s %s", r.Method, path)
}
