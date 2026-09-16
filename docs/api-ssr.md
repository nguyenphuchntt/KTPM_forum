# Phân định SSR và API JSON

> **Phạm vi:** tài liệu này mô tả các route đang có trong code hiện tại và boundary được đề xuất cho bước tách layer tiếp theo.
>
> **Lưu ý:** phần ghi **CURRENT** là hành vi đang được đăng ký trong `server/routes/routes.go`. Phần **PROPOSED** chỉ là thiết kế mục tiêu; các route `/api/v1` chưa được triển khai.

## 1. Quy ước

### SSR

SSR (Server-Side Rendering) là request mà server truy vấn dữ liệu, dựng template HTML và trả về một trang hoàn chỉnh. Các route SSR phù hợp cho:

- Điều hướng trực tiếp bằng trình duyệt.
- Hiển thị nội dung post và danh sách post.
- SEO và chia sẻ URL.
- Giữ giao diện HTML hiện tại mà không bắt frontend phải tự dựng toàn bộ màn hình.

Trong code hiện tại, SSR chủ yếu đi qua `utils.RenderTemplate` trong `server/utils/templates.go`. Template được dùng gồm `home.html`, `post.html`, `post-form.html`, `login.html`, `register.html` và `error.html`.

### API JSON

API JSON trả dữ liệu hoặc kết quả mutation dưới dạng JSON, thường được gọi bằng JavaScript/XHR/fetch. API phù hợp cho:

- Reaction không cần reload trang.
- Tạo comment rồi cập nhật danh sách ngay trên trang.
- Xóa post.
- Client khác ngoài browser SSR.

Một số endpoint hiện tại là **API-like** nhưng chưa có contract REST/JSON nhất quán. Ví dụ `/post/createpost` là mutation nhưng response thành công hiện tại là body rỗng với `Content-Type: text/html`. Tài liệu không gọi những endpoint đó là API-v1 hoàn chỉnh.

### Infrastructure endpoint

Upload SAS và webhook Event Grid là endpoint JSON phục vụ hạ tầng tích hợp. Chúng không phải trang SSR và không nên gộp vào resource API của post nếu không cần thiết.

## 2. Sơ đồ phân loại hiện tại

```text
Browser navigation
  ├── GET page routes --------------------> SSR HTML/template
  ├── POST/DELETE interaction routes ------> JSON hoặc legacy form flow
  └── /assets/* --------------------------> static files

External/client integration
  ├── /api/upload/request-url -------------> JSON infrastructure API
  └── /api/webhook/blob-created -----------> Event Grid JSON webhook
```

## 3. Các route SSR hiện tại (CURRENT)

### 3.1 Danh sách và chi tiết post

| Method | Path | Handler hiện tại | Auth | Template | Response |
|---|---|---|---|---|---|
| `GET` | `/` | `controllers.IndexPosts` | Không bắt buộc | `home.html` | HTML danh sách post |
| `GET` | `/category/{id}` | `controllers.IndexPostsByCategory` | Không bắt buộc | `home.html` | HTML danh sách post theo category |
| `GET` | `/post/{id}` | `controllers.ShowPost` | Không bắt buộc | `post.html` | HTML chi tiết post và comment |
| `GET` | `/mycreatedposts` | `controllers.MyCreatedPosts` | Session bắt buộc | `home.html` | HTML post do user tạo |
| `GET` | `/mylikedposts` | `controllers.MyLikedPosts` | Session bắt buộc | `home.html` | HTML post user đã like |

#### `GET /`

- Query tùy chọn: `PageID`.
- Mỗi trang đọc tối đa 10 post.
- Dữ liệu truyền cho `home.html` gồm các trường post như `ID`, `Title`, `Content`, `UserName`, `CreatedAt`, `ImagePath`, `Categories`, `Likes`, `Dislikes` và `Comments`.
- Trang đầu không có post vẫn trả `200` và danh sách rỗng.
- Trang sau không có dữ liệu có thể trả trang lỗi `404`.
- Lỗi parse `PageID` trả trang lỗi `400`.

#### `GET /category/{id}`

- Path parameter: `id` là category ID dạng số nguyên.
- Query tùy chọn: `PageID`.
- Category không hợp lệ hoặc không tồn tại trả trang lỗi `400`/`404` theo nhánh xử lý hiện tại.
- Response thành công dùng lại `home.html`, không trả JSON.

#### `GET /post/{id}`

- Path parameter: `id` là post ID dạng số nguyên.
- Response thành công dùng `post.html`, gồm post và danh sách comments.
- ID không hợp lệ trả `400` dưới dạng trang lỗi.
- Post không tồn tại thường trả `404` dưới dạng trang lỗi.

#### `GET /mycreatedposts` và `GET /mylikedposts`

- Session không hợp lệ sẽ redirect `302` về `/login`.
- Query tùy chọn: `PageID`.
- Response thành công đều dùng `home.html`.
- Đây là các trang SSR cá nhân hóa, không phải API JSON.

### 3.2 Các trang form

| Method | Path | Handler | Auth | Template/response |
|---|---|---|---|---|
| `GET` | `/post/create` | `controllers.GetPostCreationForm` | Session bắt buộc | `post-form.html` |
| `GET` | `/login` | `controllers.GetLoginPage` | Không; user đã login có thể redirect | `login.html` |
| `GET` | `/register` | `controllers.GetRegisterPage` | Không; user đã login có thể redirect | `register.html` |

Các route trên chỉ hiển thị form. Việc submit form được phân loại riêng ở phần legacy form flow và JSON-like mutation bên dưới.

### 3.3 Error page

Các handler SSR dùng `utils.RenderError` để render `error.html` khi lỗi parse input, không tìm thấy resource, lỗi database hoặc lỗi render. Vì vậy cùng một lỗi của page route thường là HTML error page, không phải JSON error object.

## 4. API JSON và API-like hiện tại (CURRENT)

Các endpoint dưới đây đang phục vụ thao tác JavaScript trên giao diện. Chúng đang dùng path legacy `/post/*`; chưa có namespace `/api/v1`.

### 4.1 Tạo comment

**`POST /post/addcommentREQ`**

Handler: `controllers.CreateComment` trong `server/controller/comment_controller.go`.

**Request hiện tại**

- Auth: session cookie `session_id` bắt buộc.
- Content-Type hiện tại: `application/x-www-form-urlencoded`.
- Form fields:
  - `comment`: nội dung comment.
  - `postid`: post ID dạng số nguyên.
- Nội dung được trim và HTML-escape ở server.
- Endpoint được bọc bởi rate limiter tạo comment.

**Response thành công hiện tại**

`200`, `Content-Type: application/json`:

```json
{
  "ID": 42,
  "username": "alice",
  "created_at": "09/16/2026 10:30 AM",
  "content": "Nội dung đã escape",
  "likes": 0,
  "dislikes": 0,
  "commentscount": 8
}
```

**Lỗi hiện tại**

- `401`: không có session hợp lệ.
- `405`: method không phải `POST`.
- `400`: form, post ID hoặc content không hợp lệ.
- `500`: lỗi lưu comment, đếm comment hoặc đọc timestamp.

### 4.2 Reaction cho post

**`POST /post/postreaction`**

Handler: `controllers.ReactToPost` trong `server/controller/post_controller.go`.

**Request hiện tại**

- Auth: session cookie bắt buộc.
- Content-Type: `application/x-www-form-urlencoded`.
- Form fields:
  - `reaction`: giá trị reaction hiện được xử lý là `like` hoặc `dislike`.
  - `post_id`: post ID dạng số nguyên.

**Response thành công hiện tại**

`200`, JSON:

```json
{
  "likesCount": 12,
  "dislikesCount": 2
}
```

**Lỗi hiện tại**

- `401`: chưa đăng nhập.
- `400`: form hoặc post ID không hợp lệ.
- `405`: method không phải `POST`.
- `500`: lỗi xử lý reaction/database.

Sau mutation, cache post liên quan được invalidate.

### 4.3 Reaction cho comment

**`POST /post/commentreaction`**

Handler: `controllers.ReactToComment` trong `server/controller/comment_controller.go`.

**Request hiện tại**

- Auth: session cookie bắt buộc.
- Content-Type: `application/x-www-form-urlencoded`.
- Form fields:
  - `reaction`: `like` hoặc `dislike`.
  - `comment_id`: comment ID dạng số nguyên.

**Response thành công hiện tại**

`200`, JSON:

```json
{
  "commentlikesCount": 5,
  "commentdislikesCount": 1
}
```

**Lỗi hiện tại**

- `401`: chưa đăng nhập.
- `400`: form hoặc comment ID không hợp lệ.
- `405`: method không phải `POST`.
- `500`: không tìm được post của comment hoặc lỗi reaction/database.

### 4.4 Xóa post

**`DELETE /post/delete/{id}`**

Handler: `controllers.DeletePost` trong `server/controller/post_controller.go`.

**Request hiện tại**

- Auth: session cookie bắt buộc.
- Path parameter: `id` là post ID.
- User phải có quyền sở hữu post theo logic hiện tại.
- Không yêu cầu request body.

**Response thành công hiện tại**

`200`, JSON:

```json
{
  "message": "Post deleted successfully"
}
```

**Response lỗi hiện tại**

```json
{
  "error": "Unauthorized"
}
```

hoặc:

```json
{
  "error": "Invalid post ID"
}
```

Status có thể là `401`, `400`, `404` hoặc status lỗi được model trả về. Cache post bị invalidate sau khi xóa.

### 4.5 Tạo post: mutation nhưng chưa phải JSON API chuẩn

**`POST /post/createpost`**

Handler: `controllers.CreatePost`.

**Request hiện tại**

- Auth: session cookie bắt buộc.
- Content-Type: multipart form.
- Fields:
  - `title`.
  - `content`.
  - `categories` (có thể gửi nhiều giá trị hoặc chuỗi phân tách bằng dấu phẩy).
  - `image_url` tùy chọn.
  - `image` tùy chọn nếu dùng local upload fallback.
- Giới hạn parse multipart hiện tại là 10 MB.
- Category được kiểm tra trước khi lưu.

**Response hiện tại**

- Thành công: `200`, `Content-Type: text/html`, body rỗng.
- Lỗi: thường chỉ trả status (`400`, `401`, `405`, `500`) và body không có JSON contract thống nhất.
- Đây là **legacy form mutation**, không xếp vào nhóm JSON API chuẩn dù được gọi bằng JavaScript.

Khi chuyển sang API-v1, endpoint này nên trả JSON chứa post vừa tạo hoặc ID của post vừa tạo.

## 5. Auth legacy và redirect flow (CURRENT)

### `POST /signin`

Handler: `controllers.Signin`.

- Request: form URL-encoded với `username`, `password`.
- Session hiện tại được lưu bằng cookie `session_id` sau khi đăng nhập thành công.
- Thành công: redirect `302` về `/`.
- User đã có session: redirect `302`.
- Input không hợp lệ: `400`.
- Không tìm thấy user: `404`.
- Mật khẩu sai: `401`.
- Lỗi database/session: `500`.
- Đây là form/redirect flow, **chưa phải JSON API**.

### `POST /signup`

Handler: `controllers.Signup`.

- Request: form URL-encoded với `email`, `username`, `password`, `password-confirmation`.
- Thành công hiện tại: `200`, `text/html`, body rỗng.
- User đã có session: redirect `302`.
- Username đã tồn tại: hiện có thể trả `304`.
- Input không hợp lệ: `400`.
- Lỗi lưu user: `500`.
- Đây là form/redirect flow, **chưa phải JSON API**.

### `GET /logout`

- Không trả JSON.
- Xóa session theo logic hiện tại rồi redirect về `/`.
- Đây là browser navigation/redirect flow.

## 6. Infrastructure JSON endpoints (CURRENT)

### 6.1 Xin URL upload

**`POST /api/upload/request-url`**

Handler hiện tại là `UploadGatekeeper.GenerateUploadURL` trong `server/middleware/upload_gatekeeper.go`. Route chỉ được đăng ký khi upload gatekeeper khác `nil`.

**Request JSON**

```json
{
  "filename": "photo.png",
  "size": 245760,
  "content_type": "image/png"
}
```

**Điều kiện**

- Session hợp lệ.
- Kích thước tối đa hiện tại: 5 MB ở bước metadata.
- Extension cho phép: `.jpg`, `.jpeg`, `.png`, `.gif`, `.webp`.
- MIME type phải bắt đầu bằng `image/`.
- Endpoint được áp dụng rate limit upload.

**Response thành công**

```json
{
  "upload_url": "https://...SAS...",
  "public_url": "https://.../post-images/123_456.png",
  "object_key": "quarantine/123_456.png",
  "expires_in": 300
}
```

`upload_url` trỏ tới quarantine container; file chỉ được đưa sang production sau quy trình validate/webhook hoặc watcher. Đây là API hạ tầng, không phải SSR.

**Lỗi**

- `401`: session không hợp lệ.
- `400`: JSON sai hoặc metadata không hợp lệ.
- `429`: vượt rate limit.
- `500`: không tạo được SAS token.

### 6.2 Webhook blob-created

**`POST /api/webhook/blob-created`** và **`OPTIONS /api/webhook/blob-created`**

Handler: `WebhookController.HandleBlobCreated` trong `server/controller/webhook_controller.go`.

- Request POST là một mảng Event Grid event JSON.
- `SubscriptionValidationEvent` trả JSON dạng:

```json
{
  "validationResponse": "..."
}
```

- Event `BlobCreated` được xử lý qua quy trình đọc blob quarantine, kiểm tra magic bytes/integrity, copy sang production nếu hợp lệ rồi xóa quarantine.
- Lỗi đọc/parse request trả `400`.
- Lỗi xử lý từng event được log; request hiện tại vẫn có thể trả `200`.
- OPTIONS trả `200` và header cho phép origin theo implementation hiện tại.

Webhook là endpoint tích hợp bên ngoài, không đưa vào SSR và không dùng contract resource post của client.

## 7. Static assets (không phải SSR/API)

**`GET /assets/*`**

- Được phục vụ bởi `controllers.ServeStaticFiles`.
- Trả CSS, JavaScript, hình ảnh và các asset trong `web/assets`.
- File không tồn tại hoặc path là directory trả error page `404` theo code hiện tại.
- Đây là static file delivery, không phải API JSON.

## 8. Boundary mục tiêu đề xuất (PROPOSED, chưa triển khai)

Giữ các page route hiện tại cho browser/SEO, đồng thời tạo namespace rõ ràng cho JSON API. Không đổi ngay trong một lần; route legacy có thể tồn tại trong giai đoạn chuyển tiếp.

### 8.1 Giữ SSR

Các route sau tiếp tục là SSR:

```text
GET  /
GET  /category/{id}
GET  /post/{id}
GET  /mycreatedposts
GET  /mylikedposts
GET  /post/create
GET  /login
GET  /register
```

SSR handler chỉ nên điều phối request, lấy session context và gọi service; repository/domain không biết `http.ResponseWriter` hoặc template.

### 8.2 API-v1 cho auth

Đề xuất:

```text
POST /api/v1/auth/signin
POST /api/v1/auth/signup
POST /api/v1/auth/logout
GET  /api/v1/auth/me
```

- Request là JSON thay vì form legacy.
- Response là JSON nhất quán.
- Có thể tiếp tục dùng session cookie hiện tại; JWT không phải điều kiện của thiết kế này.
- `/signin`, `/signup`, `/logout` cũ được giữ làm compatibility flow cho SSR trong thời gian migration.

### 8.3 API-v1 cho post

Đề xuất:

```text
GET    /api/v1/posts
GET    /api/v1/posts/{id}
POST   /api/v1/posts
DELETE /api/v1/posts/{id}
POST   /api/v1/posts/{id}/reactions
GET    /api/v1/me/posts
GET    /api/v1/me/liked-posts
```

- `GET` trả dữ liệu post JSON, không render template.
- `POST` nhận JSON hoặc multipart tùy quyết định upload; nếu upload tách riêng thì post chỉ nhận `image_url`/object key đã được cấp.
- `DELETE` giữ ownership check trong application service.
- Reaction trả counts hoặc resource reaction theo contract đã thống nhất.
- Các query pagination nên dùng `page`/`limit` hoặc cursor rõ ràng; không bắt client phụ thuộc tên legacy `PageID` nếu không cần.

### 8.4 API-v1 cho comment

Đề xuất:

```text
GET  /api/v1/posts/{post_id}/comments
POST /api/v1/posts/{post_id}/comments
POST /api/v1/comments/{comment_id}/reactions
```

- `POST` nhận JSON như `{ "content": "..." }`.
- Response chứa comment resource đầy đủ và count cần thiết.
- Authorization/session được xử lý ở middleware/service, không để handler tự query SQL.
- Route legacy `/post/addcommentREQ` và `/post/commentreaction` giữ trong giai đoạn chuyển tiếp.

### 8.5 API category

Đề xuất nếu client cần dữ liệu category riêng:

```text
GET /api/v1/categories
GET /api/v1/categories/{id}
```

Hiện category được nạp kèm `GlobalData` khi render SSR và chưa có category API riêng trong `routes.go`. Không cần tạo API chỉ để thay đổi behavior hiện tại.

### 8.6 Giữ infrastructure path

Giữ riêng:

```text
POST    /api/upload/request-url
POST    /api/webhook/blob-created
OPTIONS /api/webhook/blob-created
```

Có thể đổi thành `/api/v1/uploads/request-url` trong tương lai nếu cần versioning, nhưng đây là quyết định infrastructure riêng và không phải điều kiện để chuyển post/comment sang API JSON.

## 9. Contract JSON đề xuất cho API-v1

Đây là quy ước mục tiêu, chưa áp dụng cho các route legacy.

### Thành công

- Luôn đặt `Content-Type: application/json`.
- Resource đơn trả object JSON.
- Collection trả cấu trúc có pagination:

```json
{
  "data": [],
  "pagination": {
    "page": 1,
    "limit": 10,
    "has_next": true
  }
}
```

### Lỗi

Đề xuất dùng một cấu trúc ổn định:

```json
{
  "error": {
    "code": "validation_error",
    "message": "Request is invalid",
    "details": {
      "field": "content"
    }
  }
}
```

Các status chính:

| Status | Ý nghĩa |
|---:|---|
| `400` | Request/field/path không hợp lệ |
| `401` | Chưa xác thực |
| `403` | Đã xác thực nhưng không có quyền |
| `404` | Không tìm thấy resource |
| `409` | Xung đột dữ liệu |
| `429` | Rate limit |
| `500` | Lỗi server |

Không nên áp dụng envelope mới cho legacy route nếu làm hỏng JavaScript hiện tại; hãy chuyển từng client call cùng handler tương ứng.

## 10. Lộ trình chuyển đổi không phá SSR

1. **Giữ route SSR ổn định.** Không biến `GET /` hoặc `GET /post/{id}` thành JSON.
2. **Tách service dùng chung.** SSR handler và API handler cùng gọi service; chỉ khác presenter (template vs JSON).
3. **Tạo API-v1 song song.** Đăng ký route mới, chưa xóa route legacy.
4. **Chuyển JavaScript từng nhóm:** reaction, comment, delete, create post, auth.
5. **Chuẩn hóa status/error/JSON contract** ở route mới; giữ response cũ cho route cũ.
6. **Theo dõi compatibility.** Khi không còn client dùng legacy, đánh dấu deprecated và mới cân nhắc xóa.

## 11. Bảng quyết định cuối

| Nhóm | Route hiện tại | Định dạng hiện tại | Quyết định |
|---|---|---|---|
| Page home/list/detail | `/`, `/category/{id}`, `/post/{id}` | SSR HTML | Giữ SSR |
| Page cá nhân | `/mycreatedposts`, `/mylikedposts` | SSR HTML | Giữ SSR; có API-v1 song song nếu cần |
| Form page | `/post/create`, `/login`, `/register` | SSR HTML | Giữ SSR |
| Auth submit | `/signin`, `/signup`, `/logout` | Form + redirect/HTML | Giữ legacy; thêm API-v1 JSON |
| Comment mutation | `/post/addcommentREQ` | JSON nhưng legacy path/form | Chuyển dần sang API-v1 |
| Post reaction | `/post/postreaction` | JSON nhưng legacy path/form | Chuyển dần sang API-v1 |
| Comment reaction | `/post/commentreaction` | JSON nhưng legacy path/form | Chuyển dần sang API-v1 |
| Delete post | `/post/delete/{id}` | JSON nhưng legacy path | Chuyển dần sang API-v1 |
| Create post | `/post/createpost` | Multipart + HTML rỗng | Chuẩn hóa thành JSON/multipart API-v1 |
| Upload | `/api/upload/request-url` | JSON infrastructure | Giữ riêng infrastructure |
| Blob webhook | `/api/webhook/blob-created` | Event Grid JSON | Giữ riêng webhook |
| Static | `/assets/*` | File | Giữ static delivery |

## 12. Tóm tắt ngắn

- **SSR:** các `GET` hiển thị trang và form.
- **JSON-based hiện tại:** comment, post reaction, comment reaction và delete post; tuy nhiên chúng vẫn là legacy API-like vì path, form field và error contract chưa thống nhất.
- **Legacy form flow:** signin, signup, logout và create post.
- **JSON infrastructure:** upload request URL và blob webhook.
- **Mục tiêu:** giữ SSR cho navigation/SEO, thêm `/api/v1` cho data/mutation, dùng chung application service và không để API handler truy cập SQL trực tiếp.
