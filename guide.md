# Luồng đi của Request trong KTPM_forum

Tài liệu này mô tả **cách một HTTP request đi qua codebase thực tế** — không phải thiết kế mong muốn, mà là đúng những file / hàm / object đang chạy ở thời điểm hiện tại (sau khi hoàn thành migration schema).

Đọc kèm: `docs/ref/arch.md` (layering đích), `docs/api-ssr.md` (inventory route), `migration-impl.md` (lịch sử migration), `CLAUDE.md` (quy ước).

---

## 1. Bức tranh tổng quát — 5 tầng

```
                        ┌───────────────────────────────────────────────┐
   HTTP request  ──────▶│  cmd/main.go  →  middleware  →  mux (routes)  │
                        └───────────────────────────────────────────────┘
                                              │
                    ┌─────────────────────────┴─────────────────────────┐
                    │                                                   │
              NHÁNH SSR (HTML)                                   NHÁNH JSON API (/api/v1)
              controller/handler ──▶ repository               controller ──▶ usecase ──▶ repository
              ──▶ utils.RenderTemplate ──▶ text/template              ──▶ writeJSON (envelope)
```

| Tầng | Thư mục | Trách nhiệm | Có được làm gì |
|---|---|---|---|
| **entry** | `cmd/main.go` | khởi tạo db, cache, storage, middleware chain, server | wiring |
| **middleware** | `server/middleware/` | metrics, rate limit, upload gatekeeper | cắt request trước handler |
| **routes** | `server/routes/routes.go` | đăng ký toàn bộ pattern → handler | chỉ map URL → handler |
| **controller** | `server/controller/` | bind / validate / gọi usecase (API) hoặc gọi repo trực tiếp (SSR) / ghi response | HTTP concern, không SQL nghiệp vụ |
| **usecase** | `server/usecase/` | nghiệp vụ, authorization, ranh giới transaction | không đụng `http.ResponseWriter` |
| **repository** | `server/repository/mysql/` | toàn bộ SQL, `rows.Scan`, map row → domain | dùng `database.*WithMetrics` |
| **domain** | `server/model/` | struct thuần + typed ID | không import `net/http`, không SQL |

Quy tắc phụ thuộc (một chiều): `controller → usecase → repository → database/sql`. Không bao giờ ngược lại.

---

## 2. Giai đoạn khởi động — object nào được tạo

`cmd/main.go` chạy tuần tự, tạo ra các object sống suốt vòng đời server:

```
main()
 ├─ logger.Init()                          → logger.Log (zerolog global)
 ├─ godotenv.Load()                        → .env
 ├─ config.Connect()                       → *sql.DB            (biến db)
 ├─ nếu Docker: config.CreateDemoData(db)  → chạy migration + seed
 │  nếu local:  utils.HandleFlags(...)     → --migrate / --seed / --drop rồi exit
 ├─ cache.InitSessionCache(5s)             → cache.GlobalSessionCache
 ├─ cache.NewCategoryCache(5m)             → cache.GlobalCategoryCache (+ StartAutoRefresh goroutine)
 ├─ cloud.NewAzureStorage(...)             → uploadGatekeeper, webhookController  (chỉ khi có env)
 ├─ workers.StartQuarantineWatcher(...)    → goroutine nền (chỉ khi env bật)
 ├─ metrics.ProcessStartTimeSeconds.Set()  → Prometheus
 ├─ go collectDBStats(db)                   → goroutine 10s/lần
 ├─ go collectRuntimeStats()                → goroutine 10s/lần
 ├─ middleware.NewRateLimitMiddleware(db)   → rateLimitMiddleware
 └─ middleware.MetricsMiddleware(           → handler cuối cùng
       rateLimitMiddleware.Limit(
         routes.Routes(db, uploadGatekeeper, webhookController)))
```

Hai goroutine nền chạy song song, không liên quan request:
- `collectDBStats` → đẩy `db.Stats()` vào metrics connection pool.
- `collectRuntimeStats` → GC / heap / goroutine / uptime.
- Metrics server riêng ở `:9090` (`/metric` và `/metrics`) — tách khỏi app server `:8080`.

Bên trong `routes.Routes` (file `server/routes/routes.go`), chuỗi dependency của tầng mới được dựng **một lần** khi build mux:

```go
authUc    := usecase.NewAuthUsecase(db)
postUc    := usecase.NewPostUsecase(db, authUc)
commentUc := usecase.NewCommentUsecase(db, authUc)
authC     := controllers.NewAuthController(authUc)
postC     := controllers.NewPostController(postUc)
commentC  := controllers.NewCommentController(commentUc)
RegisterAuthRoutes(mux, authC)
RegisterCommentRoutes(mux, commentC)
RegisterPostRoutes(mux, postC)
```

Lưu ý: `postUc` và `commentUc` **giữ con trỏ tới `authUc`** (dependency injection thủ công), để tự validate session mà không cần service locator.

---

## 3. Chuỗi middleware — thứ tự thực thi

Request đi vào qua `handler` do `main` dựng, bọc từ ngoài vào trong:

```
MetricsMiddleware                 (server/middleware/metrics_middleware.go)
  └─ RateLimitMiddleware.Limit    (server/middleware/ratelimit_middleware.go)
       └─ routes.Routes mux       (server/routes/routes.go)
            └─ handler theo pattern
```

### 3.1 `MetricsMiddleware`

`middleware.MetricsMiddleware(next http.Handler)`:
1. `start := time.Now()`, tăng `metrics.HttpInFlightRequests`.
2. Bọc `w` trong `responseWriter` — struct nhúng `http.ResponseWriter` + field `statusCode`, override `WriteHeader(code)` để **ghi nhận status** mà không đổi hành vi.
3. `next.ServeHTTP(wrapped, r)`.
4. Sau khi handler xong: `metrics.NormalizeEndpoint(r.URL.Path)` để gộp các path có id động (`/post/123` → `/post/{id}`), rồi:
   - `HttpRequestsTotal.WithLabelValues(method, endpoint, status).Inc()`
   - `HttpRequestDuration.WithLabelValues(method, endpoint).Observe(duration)`

Điểm cần nhớ: status mặc định khởi tạo `http.StatusOK`; nếu handler không gọi `WriteHeader` thì coi như 200.

### 3.2 `RateLimitMiddleware.Limit`

`middleware.(*RateLimitMiddleware).Limit(next http.Handler) http.Handler`:
1. **Bỏ qua `/assets/`** — static không bị giới hạn.
2. **Global** — `globalLimiter.Allow("global")` (token bucket, `MemoryRateLimiter`). Vượt → `sendRateLimitError`, tăng `RateLimitDropsTotal{path,"global"}`.
3. **Xác định danh tính**:
   - `getUserID(r)` gọi `userRepo.ValidSession(r, m.db)` → nếu hợp lệ dùng limiter per-user (`user:<uuid>`).
   - ngược lại dùng `getClientIP(r)` → per-IP (`ip:<ip>`).
   - `getClientIP` ưu tiên `X-Forwarded-For` → `X-Real-IP` → `RemoteAddr`.
4. Vượt → `sendRateLimitError` (429). Hàm này **tự chọn content**: nếu `X-Requested-With: XMLHttpRequest` hoặc `Accept: application/json` thì trả JSON `{"error":"rate_limit_exceeded",...}`, ngược lại trả **HTML 429** — quan trọng vì page route phải trả HTML, không được trả JSON.
5. Đặt header `X-RateLimit-Limit: 100`, rồi `next.ServeHTTP`.

> Điểm đáng lưu ý về hiệu năng: rate-limit middleware gọi `ValidSession` cho *mọi* request không phải `/assets/`. Nó đi qua `cache.GlobalSessionCache` (TTL 5s) nên hit cache không đụng DB; chỉ miss mới `SELECT ... FROM sessions JOIN accounts`.

### 3.3 Rate limit theo endpoint

`EndpointRateLimiter` được tạo trong `Routes` (`middleware.NewEndpointRateLimiter(db, rateLimitConfig)`) và bọc **từng handler riêng lẻ**:

| Wrapper | Route | Key | Cửa sổ |
|---|---|---|---|
| `LimitLogin` | `/signin` | `login:<ip>` | `LoginAttemptsPerWindow` |
| `LimitRegister` | `/signup` | `register:<ip>` | `RegisterAttemptsPerWindow` |
| `LimitCreatePost` | `/post/createpost` | `post:<userID>` | `PostsPerHour` |
| `LimitCreateComment` | (dùng cho tạo comment) | `comment:<userID>` | `CommentsPerHour` |
| `LimitUpload` | `/api/upload/request-url` | `upload:user/ip` | `UploadRequestsPerMinute` |

Mọi ngưỡng lấy từ `config.DefaultRateLimitConfig()` (`server/config/ratelimit_config.go`), override bằng env.

---

## 4. Routing — `server/routes/routes.go`

Dùng `http.ServeMux` chuẩn (Go 1.22+ pattern syntax: `{id}`, `{post_id}`). Một `mux` duy nhất, `HandleFunc(pattern, handler)`.

### 4.1 Nhóm route JSON API v1 (tầng mới)

| Pattern | Handler | Ghi chú |
|---|---|---|
| `/api/v1/auth/signin` | `AuthController.SigninJSON` | `RegisterAuthRoutes` |
| `/api/v1/auth/signup` | `AuthController.SignupJSON` | |
| `/api/v1/auth/logout` | `AuthController.LogoutJSON` | |
| `/api/v1/auth/me` | `AuthController.MeJSON` | |
| `/api/v1/posts` | `PostController.PostsCollectionJSON` | GET list / POST create |
| `/api/v1/posts/{id}` | `PostController.PostItemJSON` | GET detail / DELETE |
| `/api/v1/posts/{id}/reactions` | `PostController.ReactToPostJSON` | |
| `/api/v1/me/posts` | `PostController.MyPostsJSON` | |
| `/api/v1/me/liked-posts` | `PostController.MyLikedPostsJSON` | |
| `/api/v1/posts/{post_id}/comments` | `CommentController.CommentCollectionJSON` | GET list / POST create |
| `/api/v1/comments/{comment_id}/reactions` | `CommentController.ReactToCommentJSON` | |

Mỗi `Register*Routes` nhận `(mux, controllerValue)` — controller là struct **value** (không phải pointer), chứa con trỏ usecase.

### 4.2 Nhóm route SSR (HTML) và interaction legacy

| Pattern | Handler | Kiểu trả về |
|---|---|---|
| `/` | `controllers.IndexPosts` | HTML (`home.html`) |
| `/category/{id}` | `controllers.IndexPostsByCategory` | HTML |
| `/post/{id}` | `controllers.ShowPost` | HTML (`post.html`) |
| `/post/create` | `controllers.GetPostCreationForm` | HTML form |
| `/mycreatedposts` | `controllers.MyCreatedPosts` | HTML |
| `/mylikedposts` | `controllers.MyLikedPosts` | HTML |
| `/login`, `/register` | `GetLoginPage`, `GetRegisterPage` | HTML |
| `/post/createpost` | `controllers.CreatePost` | **không HTML, không JSON — set status thô** |
| `/post/postreaction` | `controllers.ReactToPost` | JSON |
| `/post/delete/{id}` | `controllers.DeletePost` | JSON |
| `/signin`, `/signup`, `/logout` | `Signin`, `Signup`, `Logout` | form + redirect |
| `/assets/` | `controllers.ServeStaticFiles` | static |
| `/api/upload/request-url` | `uploadGatekeeper.GenerateUploadURL` | JSON SAS token |
| `/api/webhook/blob-created` | `webhookController.HandleBlobCreated` | webhook |

> **Quan trọng — hai phong cách controller đang song song.** Nhóm SSR (`post_controller.go`, `auth_controller.go` legacy `Signin/Signup`) **gọi repository trực tiếp**, bỏ qua usecase. Nhóm JSON API đi đúng chuỗi `controller → usecase → repository`. Đây là trạng thái chuyển tiếp có chủ đích: page SSR giữ nguyên để không vỡ SEO, còn interaction mới đi qua tầng sạch.

---

## 5. Nhánh JSON API — luồng đầy đủ

Đây là luồng "chuẩn" của kiến trúc mới. Ví dụ **POST `/api/v1/posts/{id}/reactions`** (like/dislike):

```
1. MetricsMiddleware          → đo thời gian, đếm in-flight
2. RateLimitMiddleware.Limit  → global + per-user
3. mux → /api/v1/posts/{id}/reactions
4. PostController.ReactToPostJSON          (controller/post_api_controller.go)
     ├─ strconv.ParseInt(r.PathValue("id"))         ← id từ URL, có kiểm tra > 0
     ├─ validators.BindReactToPostRequest(r)        ← đọc JSON body
     ├─ validators.ValidateReactToPostRequest(req)  ← kiểm tra reaction ∈ {like,dislike}
     └─ c.Post.ReactToPost(usecase.ReactToPostInput{...})
5. PostUsecase.ReactToPost                  (usecase/post.go)
     ├─ uc.auth.ValidateSession(r)          ← session → SessionInfo
     ├─ kiểm tra method == POST, PostID > 0, reaction hợp lệ
     ├─ postRepository.ReactToPost(db, AccountID, PostID, reaction)
     ├─ invalidatePostListCaches(nil)       ← xoá cache list
     └─ invalidatePostCache(postID)         ← xoá "post_<id>"
6. post.ReactToPost                         (repository/mysql/post/post.go)
     ├─ tx := db.Begin()  (defer tx.Rollback())
     ├─ SELECT reaction FROM likes WHERE user_id=? AND target_type='post' AND target_id=?
     ├─ nhánh: INSERT / DELETE / UPDATE bảng likes  → tính likeDelta/dislikeDelta
     ├─ UPDATE posts SET like_count = GREATEST(0, like_count + ?),
     │                    dislike_count = GREATEST(0, dislike_count + ?)
     ├─ SELECT like_count, dislike_count FROM posts WHERE id=?   ← đọc lại trong tx
     └─ tx.Commit()
7. quay lại controller:
     writeJSON(w, 200, response.ReactToPostResponse{Likes, Dislikes})
```

Điểm cốt lõi của bước 6: **bảng `likes` và cột denormalized `posts.like_count/dislike_count` được cập nhật trong CÙNG một transaction, bằng delta** (`GREATEST(0, ...)` chống âm). Đây là thay thế cho `post_materialized_view` cũ — count nằm ngay trên row `posts`, không còn COUNT(*) qua bảng reaction.

### 5.1 Envelope JSON

Controller dùng hai helper dùng chung (`controller/auth_controller.go`, áp dụng mọi API controller):

```go
writeJSON(w, status, payload)          // set Content-Type application/json; WriteHeader; Encode
writeAppError(w, err)                  // usecase.ToHTTP(err) → response.NewError(...)
```

`usecase.ToHTTP` (`usecase/errors.go`) là điểm map lỗi duy nhất:

| Loại error | Kết quả |
|---|---|
| `*AppError` | dùng `Status`, `Code`, `Message`, `Details` của chính nó |
| bất kỳ error khác | **500 `internal_error`** — SQL/chi tiết nội bộ không rò ra ngoài |

Envelope lỗi: `{"error": {"code", "message", "details"}}`.
Envelope thành công: `data` / `pagination` (xem `dto/response/`).

### 5.2 Luồng auth JSON — ví dụ POST `/api/v1/auth/signin`

```
mux → AuthController.SigninJSON
  ├─ method == POST? (nếu không → 405)
  ├─ validators.BindSigninRequest(r)            → malformed → 400
  ├─ validators.ValidateSigninRequest(req)      → invalid   → 400 + details
  ├─ c.Auth.Signin(req)  → usecase/auth.go
  │    ├─ userRepository.GetCredentialsByUsername(db, username)
  │    ├─ bcrypt.CompareHashAndPassword(...)    → sai → ErrInvalidCredentials
  │    ├─ config.GenerateSessionID()
  │    ├─ userRepository.StoreSession(db, userID, sessionID, expiresAt)   (REPLACE INTO sessions)
  │    └─ cache.GlobalSessionCache.Set(sessionID, userID, username, expiresAt)
  ├─ http.SetCookie(w, usecase.SessionCookie(result.SessionID, result.ExpiresAt))
  └─ writeJSON(w, 200, result)
```

Cookie `session_id`: `HttpOnly`, `SameSite=Lax`, `Path=/`. Đăng xuất dùng `ClearSessionCookie()` (MaxAge -1).

---

## 6. Nhánh SSR — luồng đầy đủ

Đây là nhánh **bỏ qua usecase** (trừ khi sau này refactor). Ví dụ **GET `/`**:

```
1. MetricsMiddleware → RateLimitMiddleware → mux
2. controllers.IndexPosts(w, r, db)                (controller/post_controller.go)
     ├─ userID, username, valid := userRepo.ValidSession(r, db)   ← gọi repo trực tiếp
     ├─ log := logger.WithRequest(r, userID); log.Info()...
     ├─ kiểm tra path == "/" và method == GET
     ├─ đọc PageID, tính offset = (page-1)*10
     ├─ postRepo.FetchPostIDsByTimestamp(db, offset, 10)   → []int id theo created_at DESC
     ├─ getCachedPosts(postIDs)                            → lấy từ cache.AppCache ("post_<id>")
     ├─ với id thiếu: postRepo.FetchPostsByIDs(db, missing) → nạp DB rồi cachePost()
     └─ utils.RenderTemplate(db, w, r, "home", 200, posts, valid, username)
3. utils.RenderTemplate                             (utils/templates.go)
     ├─ ParseTemplates("home")  ← nối header + footer + navbar + home.html
     ├─ categoryRepo.FetchCategories(db)   ← soft-fail: lỗi thì categories = nil
     ├─ dựng GlobalData{IsAuthenticated, Data, UserName, Categories}
     ├─ w.WriteHeader(statusCode)
     └─ t.ExecuteTemplate(w, "home.html", globalData)
4. web/templates/home.html render HTML  (vòng {{range .Data}} qua []model.Post)
```

### 6.1 Template nhận field gì

`GlobalData` (`utils/templates.go`):

```go
type GlobalData struct {
    IsAuthenticated bool
    Data            any            // []model.Post hoặc PostDetail, tuỳ route
    UserName        string
    Categories      []model.Category
}
```

`home.html` render từng `.Data.*` và cần các field sau trên `model.Post`. Vì `text/template` gọi **method không tham số**, `model.Post` (`server/model/post.go`) cung cấp getter khớp tên template:

| Template đọc | Method trên `model.Post` | Nguồn |
|---|---|---|
| `.UserName` | `func (p Post) UserName() string` | `Username` (JOIN `accounts`) |
| `.Likes` | `func (p Post) Likes() int` | `LikeCount` |
| `.Dislikes` | `func (p Post) Dislikes() int` | `DislikeCount` |
| `.Comments` | `func (p Post) Comments() int` | `CommentCount` |
| `.ImagePath` | `func (p Post) ImagePath() string` | `MediaID` → `/api/v1/medias/{id}` |
| `.ID`, `.Title`, `.Content`, `.CreatedAt` | field trực tiếp | cột `posts` |
| `{{range .Categories}}#{{.}}{{end}}` | field `Categories []PostCategory` | `post_category` JOIN `categories` |

> Đây là lý do tồn tại các getter "lạ" `Likes()/Dislikes()/Comments()`: template HTML viết theo tên số nhiều, còn model dùng tên cột `*Count`. Getter là lớp keo giữa hai bên.

### 6.2 Cache trong nhánh SSR

- `cache.AppCache` (golang-lru) giữ key `post_<id>` → `model.Post`, TTL `config.CacheTTL`.
- List page dùng cache theo id: lấy id list mới mỗi lần (`FetchPostIDsByTimestamp`) nhưng thân post lấy từ cache; chỉ id thiếu mới hit DB (`FetchPostsByIDs`).
- `ShowPost` cache **`PostDetail`** (post + comments) dưới cùng key `post_<id>` — trùng key với `cachePost` ở home. Cần chú ý: hai kiểu giá trị khác nhau cùng namespace key, `getCachedPostByID` sẽ type-assert fail và tự xoá key nếu sai kiểu.
- Invalidation **sau commit**: `invalidatePostCache` / `invalidatePostListCaches` (usecase) và `cache.AppCache.Delete(...)` (SSR controller) đều chạy sau khi write xong.

### 6.3 Lỗi ở SSR → HTML, không JSON

`utils.RenderError(db, w, r, status, isauth, username)` render `error.html`. Quy ước bất di bất dịch:
- **Page route** lỗi → HTML (`error.html`).
- **Interaction route** (`/post/postreaction`, `/post/delete/{id}`, `/api/v1/*`) lỗi → JSON.

Một page route trả JSON là regression.

---

## 7. Ví dụ viết đầy đủ — POST `/post/createpost` (tạo bài SSR)

```
MetricsMiddleware → RateLimitMiddleware(global + per-user)
  → EndpointRateLimiter.LimitCreatePost (post:<userID>, PostsPerHour)
    → controllers.CreatePost(w, r, db)              (controller/post_controller.go)
        ├─ userRepo.ValidSession              → không hợp lệ → 401 thô
        ├─ method == POST?                    → không → 405
        ├─ r.ParseMultipartForm(10<<20)
        ├─ đọc title, content, categories[]; html.EscapeString
        ├─ categoryRepo.CheckCategories(db, ids)   → sai → 400
        ├─ xử lý ảnh: image_url, hoặc utils.NewLocalStorage(BasePath+"web/assets/uploads").Save()
        ├─ postRepo.StorePost(db, model.AccountID(userID), title, content, nil)
        │     └─ INSERT INTO posts (user_id, title, content, media_id) ...
        ├─ postRepo.StoreAllPostCategories(db, pid, catidsInt)
        │     └─ tx: INSERT INTO post_category(...) cho từng id
        ├─ xoá cache "index_posts_page_0" + "category_posts_<id>_page_0"
        └─ w.WriteHeader(200)   ← KHÔNG trả body, KHÔNG envelope
```

> `CreatePost` SSR là route **lệch chuẩn nhất**: chỉ set status code, không HTML, không JSON envelope. Client JS (`web/assets/js/`) chỉ kiểm tra status. Đây là ứng viên nên chuyển sang `/api/v1/posts` (đã có sẵn, xem mục 4.1).

---

## 8. Luồng upload ảnh (Azure)

```
client (web/assets/js/validation/)  ── validate size/extension/MIME/magic bytes
   │
   ├─ GET /api/upload/request-url
   │     → EndpointRateLimiter.LimitUpload
   │     → uploadGatekeeper.GenerateUploadURL                     (middleware/upload_gatekeeper.go)
   │     → trả SAS token cho container quarantine
   │
   ├─ client upload thẳng blob lên quarantine
   │
   └─ promote blob (chọn 1 trong 2, tuỳ env):
        (a) Event Grid → POST /api/webhook/blob-created
              → webhookController.HandleBlobCreated
        (b) ENABLE_QUARANTINE_WATCHER=true
              → workers.StartQuarantineWatcher goroutine nền
        → đọc lại bytes đầu, kiểm tra chữ ký → move sang container production
```

Khi `AZURE_STORAGE_CONNECTION_STRING` không set: `uploadGatekeeper`/`webhookController` là `nil`, route không được đăng ký (đã guard `if ... != nil` trong `Routes`), app chạy bình thường nhưng tắt upload. Fallback local: `utils.NewLocalStorage` lưu vào `web/assets/uploads`.

---

## 9. Observability — request để lại dấu vết gì

| Cơ chế | Ở đâu | Ghi chú |
|---|---|---|
| `HttpRequestsTotal{method,endpoint,status}` | `MetricsMiddleware` | endpoint đã normalize (id → placeholder) |
| `HttpRequestDuration{method,endpoint}` | `MetricsMiddleware` | histogram |
| `HttpInFlightRequests` | `MetricsMiddleware` | gauge |
| `RateLimitDropsTotal{path,limiterType}` | rate limit middleware | |
| DB query metrics | `database.QueryWithMetrics` / `ExecWithMetrics(Tx)` | label theo `queryType` |
| Log có cấu trúc | `logger.WithRequest(r, userID)` → `log.Info().Msg(...)` | zerolog |
| Endpoint scrape | `:9090/metric` và `/metrics` | Prometheus |

> **Bẫy zerolog đáng nhớ:** `logger.WithRequest(r, id).Info()` **không compile** — `Info()` là pointer method, không gọi được trên giá trị return không addressable. Phải tách: `log := logger.WithRequest(r, id); log.Info().Msg(...)`. Mọi controller mới phải theo pattern này.

---

## 10. Bản đồ file → vai trò

```
cmd/main.go                                 entry: wiring toàn bộ
server/routes/routes.go                     mux: map URL → handler, dựng usecase/controller
server/middleware/metrics_middleware.go     đo metrics mọi request
server/middleware/ratelimit_middleware.go   global / per-user / per-IP / per-endpoint
server/middleware/ratelimit/ratelimit.go    cài đặt limiter (token bucket, window)
server/middleware/upload_gatekeeper.go      cấp SAS token upload
server/controller/post_controller.go        SSR post (gọi repo trực tiếp)
server/controller/auth_controller.go        AuthController JSON + writeJSON/writeAppError
server/controller/auth_routes.go            RegisterAuthRoutes
server/controller/post_api_controller.go    PostController JSON (list/detail/create/delete/react)
server/controller/post_routes.go            RegisterPostRoutes
server/controller/comment_controller.go     CommentController JSON + RegisterCommentRoutes
server/controller/assets_controller.go      static files
server/controller/webhook_controller.go     Event Grid webhook
server/usecase/auth.go                      signin/signup/session/logout + cookie helpers
server/usecase/post.go                      nghiệp vụ post, cache invalidation, transaction intent
server/usecase/comment.go                   nghiệp vụ comment
server/usecase/errors.go                    AppError + ToHTTP (map lỗi → HTTP)
server/repository/mysql/post/post.go        SQL post (đọc posts, ghi likes + delta count trong tx)
server/repository/mysql/comment/comment.go  SQL comment (+ tăng posts.comment_count trong tx)
server/repository/mysql/user/user.go        SQL account/session (package `auth`)
server/repository/mysql/category/category.go SQL category (+ cache)
server/model/post.go                        domain Post + getter cho template
server/utils/templates.go                   RenderTemplate / GlobalData / RenderError
server/dto/{request,response}/              struct vận chuyển
server/validators/                          Bind* / Validate*
server/cache/                               AppCache, session cache, category cache
server/database/                            wrapper query/exec + metrics
server/logger/                              zerolog + WithRequest
web/templates/                              header/footer/navbar + home/post/error/...
```

---

## 11. Hai luồng song song — tóm tắt để nhớ

| | SSR (page) | JSON API v1 |
|---|---|---|
| Đi qua usecase? | **Không** (gọi repo trực tiếp) | **Có** |
| Response lỗi | `utils.RenderError` → HTML | `writeAppError` → JSON envelope |
| Validate | inline trong controller | `validators.Bind*/Validate*` |
| Session | `userRepo.ValidSession` | `AuthUsecase.ValidateSession` / `RequireSession` |
| Cache | `AppCache` key `post_<id>` | usecase invalidate cùng key |
| Ví dụ | `GET /`, `GET /post/{id}` | `POST /api/v1/posts/{id}/reactions` |

Khi thêm endpoint mới: theo `CLAUDE.md`, **đăng ký cạnh route legacy chứ không thay thế**, giữ đúng envelope `data`/`{"error":{"code","message","details"}}`, và giữ nguyên quy ước page HTML vs interaction JSON.

---

## 12. Một request đi qua bao nhiêu lần chạm DB?

Ví dụ điển hình `GET /` (đã có session trong cookie, cache nóng):

1. `RateLimitMiddleware` → `ValidSession`: **cache hit** (không DB).
2. `IndexPosts` → `ValidSession` lần nữa: **cache hit**.
3. `FetchPostIDsByTimestamp`: **1 query** (lấy list id).
4. Thân post: **cache hit** cho mọi id → 0 query.
5. `RenderTemplate` → `FetchCategories`: **cache hit** (`GlobalCategoryCache`).

→ 1 query/request khi mọi thứ ấm. Cache lạnh sẽ thêm `FetchPostsByIDs` (1 query) + `select_session` (1 query) + category load (1 query). Đây chính là lý do tồn tại của 3 tầng cache (session 5s / category 5m / post LRU) và của cột count denormalized — để list page không bao giờ phải COUNT qua `likes`/`comments`.
