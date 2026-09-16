# Kiến trúc Backend và kế hoạch tách layer

## 1. Mục tiêu

Tài liệu này đề xuất cách tổ chức lại backend Go của forum theo hướng **modular
monolith**. Mục tiêu chính là tách rõ trách nhiệm giữa HTTP, nghiệp vụ và database
để code dễ mở rộng, dễ test và có thể thêm API client sau này.

Tài liệu phân biệt ba trạng thái:

- **Hiện tại:** những gì source code đang thực sự làm.
- **Kiến trúc đích:** cấu trúc nên hướng tới khi refactor.
- **Roadmap:** cải tiến chưa triển khai, không nên ghi là tính năng đã hoàn thành.

Đây không phải kế hoạch đổi toàn bộ raw SQL sang ORM. Dự án đang có các query dùng
`post_materialized_view`, aggregate reaction/comment và transaction nhiều bước.
Giữ `database/sql` với parameterized SQL trong repository là lựa chọn phù hợp.
`sqlc` có thể được cân nhắc sau cho query CRUD lặp lại, nhưng không phải điều kiện
để đạt kiến trúc tốt.

## 2. Quyết định kiến trúc

### 2.1. Modular monolith

Backend tiếp tục là một service Go và một database, nhưng bên trong được chia thành
các module có dependency direction rõ ràng:

```text
HTTP Router + Middleware
          |
          v
Transport Handler
          |
          v
Application Service / Use Case
          |
          +--> Repository interface --> MySQL implementation --> database/sql
          +--> Cache interface -------> in-memory cache
          +--> Storage interface ------> Azure Blob adapter
          +--> Worker/event boundary --> quarantine processing
```

Không cần tách microservices ở giai đoạn hiện tại vì:

- Domain forum còn vừa phải và chưa có nhu cầu scale độc lập theo module.
- Post, category và materialized view có các transaction liên quan.
- Một process giúp local development, debugging, deployment và CI đơn giản hơn.
- Interface giữa service và repository vẫn cho phép tách thành service riêng trong
  tương lai nếu traffic hoặc ownership team thực sự yêu cầu.

### 2.2. Nguyên tắc dependency

```text
handler -> service -> interface (repository/cache/storage)
repository/mysql -> database/sql + query wrapper
storage/azure -> Azure SDK
```

Các quy tắc bắt buộc cho code mới:

- Handler biết HTTP, request DTO, response DTO và status code; không biết SQL.
- Service xử lý use case, validation nghiệp vụ, authorization và transaction boundary.
- Repository xử lý SQL, scan row và mapping giữa database row với domain type.
- Cache/storage/metrics là dependency được inject qua interface khi service cần.
- Domain/service không import `net/http` hoặc Azure SDK.
- Composition root tạo concrete dependency; tránh global mutable dependency.

## 3. Hiện trạng

### 3.1. Bootstrap và route

`cmd/main.go` hiện là composition root theo nghĩa rộng: load dotenv, kết nối MySQL,
khởi tạo session/category cache, Azure storage, quarantine watcher, rate limiter,
metrics và HTTP server. Tuy nhiên nhiều chi tiết lifecycle vẫn nằm trực tiếp trong
`main`, chưa có shutdown có kiểm soát.

`server/routes/routes.go` dùng `http.ServeMux`, đăng ký route và truyền `*sql.DB`
trực tiếp vào controller. Một số endpoint được bọc bằng endpoint rate limiter.

### 3.2. Controller và model hiện tại

Các file trong `server/controllers/` vừa làm transport vừa làm orchestration:

- `server/controllers/post_controller.go` parse form, đọc session, escape input,
  xử lý upload, gọi model, invalid cache và render HTML/JSON.
- `server/controllers/comment_controller.go` tương tự; reaction comment còn query
  database trực tiếp trong controller.
- `server/controllers/login_controller.go` và `register_controller.go` xử lý cả
  request parsing, session và user operation.

Các file trong `server/model/` đang chứa đồng thời:

- Struct dữ liệu được dùng như domain/model.
- SQL query và `rows.Scan`.
- Transaction và cập nhật `post_materialized_view`.
- Retry, timeout và mapping lỗi database.

Đây là lý do một thay đổi nghiệp vụ hiện có thể lan qua controller, model, cache và
route cùng lúc. Vấn đề chính là ranh giới layer, không phải việc SQL được viết tay.

### 3.3. Hạ tầng đã có

Các thành phần nên tái sử dụng khi tách layer:

- `server/database/query_wrapper.go`: wrapper query/exec và database metrics.
- `server/cache/`: LRU post cache, session cache và category cache.
- `server/utils/retry/`: backoff và phân loại lỗi retry.
- `server/cloud/`: abstraction và implementation Azure Blob Storage.
- `server/validation/` và `server/validators/`: validation ở boundary.
- `server/metrics/` và `server/logger/`: observability hiện có.
- `server/workers/quarantine_watcher.go`: xử lý upload quarantine.
- `server/database/sql/schema.sql`: schema InnoDB, index và materialized view.

Docker Compose hiện chạy app, MySQL, Prometheus và Grafana. Automated Go tests chưa
có coverage đáng kể; các test trong phần sau là roadmap.

## 4. Cấu trúc thư mục đích

Không tạo toàn bộ cây thư mục trong một commit. Có thể chuyển từng module từ cấu
trúc hiện tại sang cấu trúc sau:

```text
cmd/
  forum/main.go                 # composition root

server/
  http/
    handler/                    # HTTP/SSR/API handlers
    response/                   # JSON envelope và error mapping
    middleware/                 # auth, request-id, CORS, metrics, recovery
    router/                     # route registration
  post/
    domain.go                   # Post, PostPage, input và domain errors
    service.go                  # use cases của post
    repository.go               # PostRepository interface
    handler.go                  # PostHandler cho HTTP/API
  comment/
    domain.go
    service.go
    repository.go
    handler.go
  user/
    domain.go
    service.go
    repository.go
    handler.go
  category/
    service.go
    repository.go
  upload/
    service.go
    storage.go                  # storage interface
  repository/
    mysql/
      post_repository.go        # SQL và mapping row
      comment_repository.go
      user_repository.go
  platform/
    cache/
    storage/                    # Azure adapter
    database/                   # DB open/pool/query metrics
    observability/
  worker/
    quarantine.go

  # Các package hiện tại được migrate dần hoặc giữ làm adapter
  controllers/                  # compatibility layer trong giai đoạn chuyển tiếp
  models/                       # legacy persistence, giảm dần theo module
  routes/
  config/
  database/
  cache/
  cloud/
  workers/
```

Có hai cách tổ chức thường gặp: chia toàn bộ theo layer (`services/`,
`repositories/`) hoặc chia theo feature (`post/`, `comment/`, `user/`). Với forum
này nên dùng **feature module bên ngoài, layer bên trong**. Code liên quan đến post
nằm gần nhau, nhưng dependency vẫn phải đi theo handler → service → interface.

## 5. Thiết kế mẫu cho module `post`

### 5.1. Mapping từ code hiện tại

Luồng hiện tại của danh sách post là:

```text
routes.Routes
  -> controllers.IndexPosts
  -> models.FetchPostIDsByTimestamp / FetchPostsByIDs
  -> cache.AppCache + database.QueryWithMetrics
  -> utils.RenderTemplate
```

Luồng mục tiêu là:

```text
routes
  -> post.Handler.List
  -> post.Service.List
  -> PostRepository.List
  -> repository/mysql SQL
  -> post.Handler viết HTML hoặc JSON
```

Tương tự, tạo post hiện tại:

```text
controllers.CreatePost
  -> parse multipart + session + validation + upload
  -> models.StorePost + StoreAllPostCategories
  -> cache invalidation
```

Nên chuyển thành:

```text
post.Handler.Create
  -> decode CreatePostRequest
  -> post.Service.Create
       -> authorize user
       -> validate input
       -> storage interface nếu có upload
       -> repository transaction
       -> invalidate cache sau commit
  -> response.Created
```

### 5.2. Interface và service

Interface chỉ mô tả nhu cầu của use case, không làm lộ SQL:

```go
type PostRepository interface {
    List(ctx context.Context, q ListQuery) (PostPage, error)
    Find(ctx context.Context, id int64) (PostDetail, error)
    Create(ctx context.Context, tx Tx, input CreatePost) (Post, error)
    Delete(ctx context.Context, tx Tx, id, ownerID int64) error
    React(ctx context.Context, tx Tx, postID, userID int64, reaction string) (ReactionCounts, error)
}
```

`repository/mysql` triển khai interface trên bằng SQL hiện tại, sử dụng
`database.QueryWithMetrics` và `ExecWithMetrics`. Service không cần biết câu query
nào được dùng hoặc dữ liệu nằm trong table thường hay materialized view.

Service nên có các use case rõ ràng:

- `List(ctx, query)` và `Get(ctx, id)` cho read path.
- `Create(ctx, actor, input)` cho post + category relation + image reference.
- `Delete(ctx, actor, id)` với ownership check trong service.
- `React(ctx, actor, postID, reaction)` với transaction và invalidation.

Cache key, TTL và invalidation nên được gom trong cache adapter hoặc policy của
service, không rải string literal trong handler. Invalidate sau khi transaction
commit thành công.

### 5.3. Domain type, row và DTO

Không dùng một struct cho cả ba mục đích:

```text
PostRow       # shape của SELECT trong repository/mysql
post.Post     # domain data service cần
PostResponse  # public API/SSR response
```

DTO giúp kiểm soát field trả ra và tránh vô tình đưa password hash, session token,
internal storage key hoặc database detail vào response. Mapping DTO đặt ở handler
hoặc một package transport riêng.

## 6. HTTP và API boundary

SSR hiện tại nên tiếp tục chạy trong lúc refactor. Handler mới có thể phục vụ API
JSON tại `/api/v1` mà không phá route cũ:

```text
GET    /api/v1/posts
GET    /api/v1/posts/{id}
POST   /api/v1/posts
DELETE /api/v1/posts/{id}
POST   /api/v1/posts/{id}/reactions
POST   /api/v1/posts/{id}/comments
GET    /api/v1/categories/{id}/posts
```

Dùng response envelope thống nhất:

```json
{"data": {"id": 42, "title": "Example"}, "meta": null, "error": null}
```

Lỗi có code ổn định, message không lộ SQL/Azure detail và request ID nếu có.
`400`, `401`, `403`, `404`, `409` và `429` cần được map nhất quán.

Handler phải giới hạn body, validate JSON/form, giới hạn `limit` và whitelist sort.
Pagination mới có thể dùng cursor theo `created_at, id`; vẫn hỗ trợ page/offset
trong compatibility route nếu template hiện tại đang cần.

## 7. Authentication và authorization

Session cookie hiện tại phù hợp với browser và SSR. Khi tách service:

- Middleware/handler đọc session và đưa `actor` hoặc `userID` vào request context.
- Service vẫn kiểm tra actor cho mọi use case protected; không tin frontend.
- Chỉ owner được delete post; user authenticated mới tạo post/comment/reaction.
- Session cache chỉ là optimization, database/session store vẫn là source of truth.
- Cookie production cần `HttpOnly`, `Secure`, `SameSite` và expiration phù hợp.
- API write dùng cookie cần CSRF protection hoặc kiểm tra Origin/Referer đúng cách.

JWT chỉ nên bổ sung khi có mobile client/stateless requirement rõ ràng. Không cần
đổi auth chỉ vì tách layer.

## 8. Database, transaction, cache và retry

Repository giữ SQL-first persistence:

- Query phức tạp, aggregate và `post_materialized_view` tiếp tục là SQL rõ ràng.
- Mọi input dùng parameterized query.
- Service sở hữu transaction boundary cho operation nhiều bảng.
- Tạo post, comment, reaction và delete phải xác định rõ consistency với view.
- Retry chỉ áp dụng transient error; không retry validation, authorization hoặc write
  non-idempotent sau khi chưa xác định trạng thái commit.
- Query nhận `context.Context` từ request thay vì tự tạo context nền trong model.
- Dùng `EXPLAIN` và integration test để bảo vệ index/query plan quan trọng.

Cache-aside cho dữ liệu public:

```text
read:  cache hit -> return; miss -> repository -> set TTL -> return
write: repository commit -> invalidate affected keys
```

Không cache password, authorization decision lâu dài hoặc response phụ thuộc user
mà không có user-specific key. Khi scale nhiều replica, cân nhắc Redis hoặc chấp
nhận local cache eventual consistency.

Schema hiện tại ở `server/database/sql/schema.sql` dùng tốt cho dev/bootstrap. Sau
này production nên chuyển sang versioned migrations chạy trong deploy step, không
drop/recreate schema khi app khởi động.

## 9. Upload và worker boundary

Handler không gọi Azure SDK trực tiếp. `upload.Service` kiểm tra actor và metadata,
sau đó gọi `Storage` interface. Azure implementation nằm trong `server/cloud/`
hoặc `server/platform/storage/`.

Luồng cần giữ:

```text
request SAS -> upload quarantine -> watcher/webhook validate -> promote -> persist reference
```

SAS phải ngắn hạn và có quyền tối thiểu. Validate size, extension, MIME và magic
bytes; không tin duy nhất Content-Type của client. Webhook cần được xác thực. Worker
cần có deduplication, cleanup blob quá hạn và context để dừng khi shutdown.

## 10. Composition root và vận hành

Mục tiêu của `main` là:

```text
load config -> open DB -> build repositories/services/handlers
            -> build router -> run HTTP/worker -> graceful shutdown
```

Việc này chưa được triển khai đầy đủ trong source hiện tại. Roadmap vận hành gồm:

- `signal.NotifyContext` và shutdown deadline.
- HTTP read/write/idle timeout.
- `/health/live` và `/health/ready` với timeout riêng.
- Context propagation cho DB và external operation.
- Structured log có request ID, route, status và duration.
- Metrics HTTP, DB, cache, rate limit, upload và worker.
- Giữ Prometheus/Grafana hiện có; tracing là bước sau.

Security baseline gồm secret qua environment/secret manager, CORS allowlist, body
limit, security headers, dependency/image scan, `go vet`, không log token/credential
và chỉ tin proxy headers từ proxy đã cấu hình.

## 11. Testing theo layer

Hiện repository chưa có automated test đáng kể. Nên bổ sung theo thứ tự:

- Service unit test: create/delete/react, ownership, validation và error mapping;
  dùng fake `PostRepository`, fake cache và fake storage.
- Repository integration test: MySQL container, transaction rollback, foreign key,
  materialized view và pagination.
- Handler test với `httptest`: malformed request, status code, DTO và response
  envelope; không cần database thật nếu service được mock.
- Authorization matrix: anonymous, owner, non-owner và admin policy nếu có.
- Upload/worker test: invalid magic bytes, SAS constraints, duplicate event,
  quarantine cleanup và promotion failure.
- CI chạy `go test ./...`, `go test -race ./...`, `go vet ./...` và integration test
  khi MySQL service sẵn sàng.

## 12. Lộ trình refactor

Mỗi phase nên là một nhóm commit nhỏ, có thể build và rollback độc lập. Trong giai
đoạn chuyển tiếp, `models` có thể giữ các compatibility wrapper gọi implementation
mới; chỉ xóa wrapper sau khi đã kiểm tra toàn bộ caller bằng `rg` và có test thay thế.

### Phase 0: Baseline behavior

Trước khi di chuyển code, ghi nhận behavior hiện tại bằng `go test ./...`,
`go vet ./...`, `go build ./...` và smoke test các route chính. Cần ghi lại page
default/page size, cache key và TTL, HTML escaping, ưu tiên `image_url` so với local
upload, redirect/status code và các JSON key hiện có. Đây là baseline để refactor
không vô tình đổi contract SSR.

### Phase 1: Chuẩn hóa boundary

1. Giữ route và behavior hiện tại.
2. Định nghĩa domain input/output và error types cho module post.
3. Tạo `PostRepository` interface dựa trên các hàm đang có trong `models/post.go`:
   `ListPage`, `ListByCategoryPage`, `FindByID`, `FindByIDs`, các read path theo user,
   create/category, reaction và delete.
4. Tạo implementation adapter gọi lại SQL cũ, chưa rewrite query. Giữ nguyên
   `database.QueryWithMetrics`, `ExecWithMetrics`, operation labels và thứ tự tham số.
5. Thêm service unit test đầu tiên với fake repository và fake cache.

### Phase 2: Tách MySQL repository

1. Di chuyển SQL, `rows.Scan`, retry và mapping lỗi method-by-method từ
   `models/post.go` sang `repository/mysql`.
2. Giữ `post_materialized_view`, aggregate và transaction semantics hiện tại; không
   đổi schema chỉ để phục vụ việc đổi package.
3. Cho các hàm trong `models` delegate sang repository mới để các controller cũ vẫn
   biên dịch trong thời gian chuyển tiếp.
4. Kiểm tra query args, metric labels, commit/rollback và kết quả với SQL mock hoặc
   MySQL integration test.

### Phase 3: Tách read path của post

1. Chuyển `IndexPosts`, `IndexPostsByCategory`, `ShowPost`, `MyCreatedPosts` và
   `MyLikedPosts` sang service.
2. Đưa cache read-through, thứ tự post theo danh sách ID và invalidation policy ra
   khỏi controller.
3. Controller chỉ parse page/id, gọi service và render template.
4. Test cache hit/miss, page rỗng ở trang đầu và trang sau, category không tồn tại,
   detail cache và query count; trước mắt phải đặc tả các quirk hiện tại trước khi
   chủ động sửa chúng.

### Phase 4: Tách write path của post

1. Chuyển `CreatePost`, `DeletePost` và `ReactToPost` sang use case.
2. Đưa ownership check, category validation và transaction boundary vào service.
3. Đưa local/Azure upload sau interface `Storage` và giữ precedence/validation hiện
   tại.
4. Invalidate cache sau commit; thêm test rollback và authorization matrix.
5. Nếu `StorePost` và `StoreAllPostCategories` hiện dùng hai transaction, giữ behavior
   đó ở lần extraction đầu tiên; chỉ gộp thành một transaction sau khi có test atomicity
   và một thay đổi riêng được review.

### Phase 5: Thin handler và wiring

Tạo handler có service dependency, map typed error sang status hiện tại và bảo toàn
body, header, redirect, template context cùng JSON response. Sau đó đổi `routes`
từ việc nhận `*sql.DB` trực tiếp sang nhận handler/route dependencies. Có thể giữ
một composition function tương thích với chữ ký `Routes` cũ cho đến khi toàn bộ route
đã được chuyển; `main` là nơi duy nhất tạo repository, service, cache/storage adapter.

### Phase 6: Mở rộng pattern

Áp dụng cùng pattern cho comment, category và auth. Đặc biệt chuyển query post ID
trong `comment_controller.go` vào `CommentRepository`; handler không được gọi
`db.Query` trực tiếp. Tách `models.ValidSession` thành session repository/auth
service sau khi post đã ổn định.

### Phase 7: API và dọn legacy

Thêm `/api/v1` với DTO/response chuẩn, chạy song song SSR. Chỉ xóa hoặc đổi tên
`models`/controller cũ sau khi route mới có test và được xác nhận bằng runtime.
Frontend SSR/ISR là bước tiếp theo nếu cần SEO, không phải prerequisite của việc
tách layer backend.

## 13. Cách trình bày trong CV

Có thể ghi những gì source hiện tại kiểm chứng được:

> Built a Go forum backend with MySQL/InnoDB transactions, parameterized SQL,
> materialized read model, LRU caching, retry handling, endpoint rate limiting,
> Azure Blob quarantine validation, Prometheus metrics and Grafana monitoring.

Sau khi refactor và có test tương ứng, có thể bổ sung:

> Separated HTTP handlers, application services and repository implementations behind
> interfaces, enabling isolated use-case tests while preserving optimized SQL queries.

Chỉ ghi sau khi thực sự triển khai và kiểm chứng: versioned migrations, graceful
shutdown, OpenAPI contract, tracing, API `/api/v1` đầy đủ hoặc JWT.

## 14. Definition of done

Refactor đạt mục tiêu khi:

- Handler không import hoặc gọi `database/sql` cho use case thông thường.
- Service có thể test mà không cần MySQL hoặc Azure thật.
- Repository là nơi duy nhất chứa SQL và mapping row.
- Authorization và transaction boundary được test ở service/integration layer.
- SSR hiện tại không đổi behavior ngoài ý muốn.
- Cache invalidation có policy tập trung và chạy sau commit.
- Module post hoàn thành pattern trước khi nhân rộng sang toàn bộ domain.
- Tài liệu, CV và README phân biệt rõ implemented capability với roadmap.

Trade-off chấp nhận được là code có thêm interface và constructor trong ngắn hạn.
Đổi lại, feature mới không phải biết toàn bộ chi tiết SQL, cache và HTTP rendering,
đồng thời có thể thay frontend hoặc persistence implementation mà không viết lại
business logic.
