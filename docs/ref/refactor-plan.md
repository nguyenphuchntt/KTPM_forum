# Kế hoạch Refactor: SSR → REST API + Frontend riêng biệt

Module forum hiện tại đang là web application Go **server-side rendering (SSR)**: Go template render HTML, controller trộn lẫn business logic lẫn view, xác thực bằng session cookie. Hướng refactor là tách thành **backend API JSON** (Go) và **frontend SPA** riêng biệt.

---

## Phần 1: Ưu nhược điểm của kiến trúc hiện tại (SSR)

### Ưu điểm

- **Đơn giản, dễ khởi tạo:** một codebase duy nhất, không cần build riêng frontend, triển khai 1 container là chạy.
- **SEO tốt:** toàn bộ nội dung nằm trong HTML trả về sẵn, crawler đọc được ngay mà không cần JavaScript.
- **Load nhanh lần đầu:** không cần chờ bundle JS render, browser hiển thị ngay ngày khi nhận response.
- **Security phần nào giảm tải:** logic nằm server, client ít tiếp xúc với data model.
- **Không có CORS:** frontend và backend cùng origin nên không vướng chính sách CORS.

### Nhược điểm

- **Khó mở rộng / đa nền tảng:** không thể dùng lại API cho mobile app, desktop, hoặc tích hợp bên thứ ba.
- **Mỗi request render nguyên trang:** tốn băng thông (truyền lại HTML lặp lại: navbar, footer), server tải nặng hơn về CPU/render.
- **UX kém trên interaction dày đặc:** like/comment/upload cần full-page reload hoặc AJAX phức tạp.
- **Trộn lẫn trách nhiệm trong controller:** controller vừa gọi DB, vừa lấy session, vừa render template, khó test unit.
- **Template HTML dính vào backend:** thay đổi giao diện phải deploy lại toàn bộ server.
- **Khó test frontend:** không có sự phân tách rõ ràng giữa view và logic.
- **Cache kém linh hoạt:** page cache cả HTML, khó cache dữ liệu ở mức field.

---

## Phần 2: Ưu nhược điểm của kiến trúc đích (REST API + SPA)

### Ưu điểm

- **Backend tái sử dụng:** cùng API phục vụ web, mobile, hay client khác.
- **Tách biệt trách nhiệm:** controller/template backend chỉ trả JSON; frontend quản lý UI, state, routing.
- **Test dễ hơn:** API test độc lập (unit + contract), frontend test với mocked API.
- **Phát triển song song:** backend team và frontend team làm độc lập, chỉ ràng buộc qua API contract (spec OpenAPI).
- **Performance phía client tốt hơn:** SPA chỉ fetch dữ liệu cần thiết dưới dạng JSON, không reload toàn trang; dễ làm optimistic UI (like/comment phản hồi tức thì).
- **Dễ scale:** backend stateless (nếu dùng JWT), frontend có thể serve CDN/static riêng.
- **Cải thiện codebase hiện tại:** giữ nguyên cache, Materialized View, rate limit, retry, metrics — chỉ đổi output từ HTML sang JSON.

### Nhược điểm

- **SEO khó hơn:** SPA render bằng JS nên crawler cần pre-render / SSR riêng (Next.js/Nuxt) hoặc có workaround.
- **Phức tạp hơn:** thêm 1 codebase, 2 bước build/deploy, cấu hình CORS.
- **Lần đầu load có thể chậm hơn:** browser cần tải JS bundle rồi mới vẽ, tuỳ vào bundle size.
- **Rò rỉ nhiều dữ liệu hơn:** API thường trả toàn bộ field của model; cần DTO/vuln bảo mật khắt khe hơn (không trả password hash, kiểm soát field theo role).
- **Cần quản lý auth ở client:** lưu token an toàn, thêm vào mọi request, xử lý expried/refresh.
- **Routing bảo mật dời lên frontend:** phải đảm bảo API vẫn check quyền ở backend (frontend chỉ ẩn UI).
- **Chi phí bảo trì 2 project, 2 pipeline CI/CD.**

---

## Phần 3: Kế hoạch Refactor

### 3.1. Nguyên tắc chung

- **Refactor dần, không làm vỡ hệ thống.** Backend vẫn giữ các class/business layer (models, database wrapper, cache, rate limit, retry, metrics, storage) — chỉ thay "đầu ra" từ render template sang JSON.
- **Backend:** thêm layer REST controller trả JSON, tạo endpoint `/api/v1/*`, giữ session/JWT auth.
- **Frontend:** tạo project **Next.js** riêng, dùng SSR/ISR cho nội dung public và client-side API cho tương tác.
- **Giữ contract API rõ ràng** (định nghĩa schema response) để 2 bên làm việc song song.
- **Không lưu password hash / field nhạy cảm vào response JSON.**

### 3.2. Chi tiết Backend

1. **Thêm package `server/api/`** chứa các handler JSON, tách khỏi `controllers` (đang render HTML).
   - Giữ logic nghiệp vụ ở `models/*` (tái sử dụng nguyên trạng).
   - Handler JSON chỉ: nhận request → validate → gọi model → trả JSON.
2. **Response chuẩn hoá:**
   ```json
   { "success": true, "data": {...}, "error": null }
   { "success": false, "data": null, "error": { "code": 404, "message": "..." } }
   ```
3. **Auth:**
   - Lựa chọn đề xuất: **JWT** (access token ngắn hạn + refresh token) để backend stateless, dễ scale.
   - Giữ bảng `sessions` nếu chọn session token stateful (chỉ đổi cách truyền/từ cookie sang header `Authorization`).
4. **CORS middleware:** cho phép origin của frontend (vd `http://localhost:5173`), xử lý preflight `OPTIONS`.
5. **Route mapping đề xuất (`/api/v1`):**

   | Method | Path | Chức năng | Cũ (SSR) |
   |---|---|---|---|
   | GET | `/posts` | Danh sách post + phân trang | `/` |
   | GET | `/posts/{id}` | Chi tiết post + comments | `/post/{id}` |
   | POST | `/posts` | Tạo post | `/post/createpost` |
   | DELETE | `/posts/{id}` | Xoá post | `/post/delete/{id}` |
   | GET | `/categories` | Danh sách category | (navbar) |
   | GET | `/categories/{id}/posts` | Post theo category | `/category/{id}` |
   | GET | `/users/me/posts` | Bài đã tạo | `/mycreatedposts` |
   | GET | `/users/me/liked` | Bài đã like | `/mylikedposts` |
   | POST | `/posts/{id}/comments` | Tạo comment | `/post/addcommentREQ` |
   | POST | `/posts/{id}/reactions` | Reaction post | `/post/postreaction` |
   | POST | `/comments/{id}/reactions` | Reaction comment | `/post/commentreaction` |
   | POST | `/auth/register` | Đăng ký | `/signup` |
   | POST | `/auth/login` | Đăng nhập | `/signin` |
   | POST | `/auth/logout` | Đăng xuất | `/logout` |
   | GET | `/upload/request-url` | Lấy SAS token upload | `/api/upload/request-url` |
6. **Giữ nguyên:** cache, Materialized View, rate limiting, retry, metrics, quarantine watcher, Azure storage, webhook.
7. **Xoá hoặc bỏ dần:** `utils/templates.go`, thư mục `web/templates/`, `renderHTML` trong controller (thay bằng JSON handler).

### 3.3. Chi tiết Frontend (project mới)

1. **Framework đề xuất:** Next.js, tạo ở thư mục `/frontend` riêng. Next.js gọi Go API để SSR/ISR nội dung public.
2. **Cấu trúc đề xuất:** components + pages + services (API client) + stores (quản lý state, ví dụ Zustand/Redux) + routes (React Router).
3. **Auth flow:** login → nhận token → lưu (context + localStorage) → axios/fetch interceptor tự gắn header `Authorization`.
4. **Route mapping trang:** `/` (danh sách post), `/post/:id`, `/category/:id`, `/my-posts`, `/my-liked`, `/login`, `/register`.
5. **Upload ảnh:** frontend validate (size, extension, mime, magic bytes — đã có `web/assets/js/validation/*` sẵn), request SAS token từ `/api/v1/upload/request-url`, upload thẳng lên Azure.

### 3.4. Triển khai theo giai đoạn (Phase)

| Phase | Nội dung |
|---|---|
| **1** | Dựng skeleton backend API JSON: response wrapper, CORS, auth JWT/session, route `/api/v1` chuyển dần từng endpoint sang JSON. |
| **2** | Dựng skeleton frontend Next.js: router, API client, SSR/ISR cho trang public, component cơ bản (login, danh sách post). |
| **3** | Chuyển đủ toàn bộ endpoint + tương tác (create/comment/reaction/upload). |
| **4** | Khi cả 2 chạy ổn, **xoá bỏ** phần SSR cũ (template, controller render HTML, endpoint HTML). |
| **5** | Hoàn thiện: test API (contract/integration), test frontend, CORS hardening, bảo mật token, đóng gói CI/CD, tài liệu API (OpenAPI). |

### 3.5. Rủi ro & cách giảm thiểu

- **Vỡ chức năng trong lúc chuyển:** chuyển dần từng endpoint, chạy song song SSR và API cho đến khi API đủ cho frontend.
- **Lộ dữ liệu qua API:** dùng struct DTO rõ ràng, không trả thẳng entity DB; test field response.
- **SEO giảm:** nếu cần, chuyển sang Next.js (SSR/SSG) hoặc thêm pre-render; với forum nội bộ ít đòi hỏi SEO thì chấp nhận.
- **Auth phức tạp hơn:** chuẩn hoá refresh token + interceptor, bảo mật lưu trữ token, check quyền vẫn đặt ở backend.
- **Tăng chi phí deploy:** tách Dockerfile riêng cho frontend (build static → serve qua nginx) và backend.

---

## Kết luận

SSR hiện tại đơn giản và tốt cho SEO nhưng khó mở rộng, trộn view với logic, và không tái dùng được API. Việc refactor sang **REST API JSON + SPA** phù hợp khi cần: phát triển multi-platform, tách team, thao tác UI mượt, test dễ. Có thể thực hiện **dần theo phase** để giảm rủi ro, giữ lại toàn bộ phần đã tối ưu (cache, materialized view, rate limit, retry, metrics, storage secure upload) và chỉ đổi lớp hiển thị.

**Đề xuất chốt:** Next.js cho frontend (SSR/ISR trang public, client-side API cho tương tác), JWT cho auth, CORS giới hạn theo origin frontend, backend giữ nguyên business layer, chuyển controller sang trả JSON theo `/api/v1`. Đây là hướng phù hợp nhất cho forum vì vừa tách frontend/backend, vừa giữ SEO.