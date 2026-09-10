# go-tracing

Bo cong cu OpenTelemetry tracing dung chung cho cac service Go.

Giai quyet hai thu ma moi repo hay tu viet lai va hay viet thieu:

- **Khoi tao TracerProvider** doc cau hinh tu bien moi truong chuan OTel,
  khong nhet endpoint vao file config cua tung repo.
- **Boc HTTP client `req/v3`** de request di ra vua co span, vua mang theo
  header `traceparent`. Thieu header nay thi service nhan se mo mot trace
  ROOT moi, waterfall dut ngay tai bien gioi giua hai service.

## Cai dat

```bash
go get github.com/KDuyNguyen1501/go-tracing
```

## Dung

### Khoi tao

```go
shutdown, err := tracing.Init(ctx, tracing.Config{ServiceName: "check-price"})
if err != nil {
    return err
}
defer shutdown(context.Background())
```

Voi `uber-go/fx`:

```go
func InitTracer(lc fx.Lifecycle) error {
    shutdown, err := tracing.Init(context.Background(), tracing.Config{
        ServiceName: configs.Get().Server.Name,
    })
    if err != nil {
        return err
    }
    lc.Append(fx.Hook{OnStop: shutdown})
    return nil
}
```

### Boc HTTP client

```go
cli := req.C().SetBaseURL(host).SetTimeout(timeout)
reqtrace.Wrap(cli)
```

Roi luon truyen context xuong request:

```go
resp, err := cli.R().SetContext(ctx).Post(path)
```

Khong co `SetContext` thi span van duoc tao nhung thanh root - khong noi vao
trace cua request dang xu ly.

## Bien moi truong

| Bien | Y nghia |
| --- | --- |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Noi nhan span, vi du `http://localhost:4318` |
| `OTEL_SERVICE_NAME` | Ten service, thay cho `Config.ServiceName` |
| `OTEL_TRACES_SAMPLER` | `parentbased_traceidratio` de lay mau mot phan |
| `OTEL_TRACES_SAMPLER_ARG` | `0.1` = 10% |
| `OTEL_RESOURCE_ATTRIBUTES` | Attribute bo sung, `key=value,key2=value2` |
| `OTEL_SDK_DISABLED` | `true` de tat han |

Sampler co y khong duoc set trong code, de env dieu khien duoc ma khong phai
build lai. Khong co env thi mac dinh cua SDK la lay 100%.

## Chay thu o local

```bash
docker run -d --name jaeger -p 16686:16686 -p 4318:4318 \
  -e COLLECTOR_OTLP_ENABLED=true jaegertracing/all-in-one:1.62.0

OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 go run .
```

Mo http://localhost:16686 de xem waterfall.

## Nhung cho de sai

**Body request/response khong duoc ghi vao span theo mac dinh.** Body hay chua
token va thong tin khach hang, lai lam span phinh to. Chi bat o dev:

```go
reqtrace.Wrap(cli, reqtrace.WithRequestBody(), reqtrace.WithResponseBody())
```

**Ten span mac dinh la `<METHOD> <path>`.** Neu path chua ID (`/order/12345`)
thi so luong ten span se phinh rat nhanh va dashboard kho gom nhom. Luc do:

```go
reqtrace.Wrap(cli, reqtrace.WithSpanNameFormatter(func(r *req.Request) string {
    return "POST /order/{id}"
}))
```

**Luon goi `shutdown` truoc khi thoat.** Span nam trong buffer va chi duoc gui
di moi 5 giay hoac khi du 512 span. Thoat dot ngot la mat nhung span cuoi.

**`resource.Merge(resource.Default(), ...)` voi semconv khac version SDK se tra
ve loi "conflicting Schema URL"**, resource mat sach attribute va service hien
thanh `unknown_service:<binary>` tren dashboard. Package nay dung
`resource.New` de tranh han loi do - day la ly do khong nen tu viet lai phan
khoi tao o tung repo.

## Test

```bash
go test ./...
```
