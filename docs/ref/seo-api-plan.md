# Kế hoạch phù hợp cho forum: API + SEO

## Mục tiêu

- Tách backend Go thành **REST API JSON**.
- Có frontend riêng để dễ phát triển và tái sử dụng cho mobile.
- Giữ SEO tốt cho bài viết và danh mục public.
- Không chuyển toàn bộ website sang SPA thuần.

## Kiến trúc đề xuất

```text
Crawler / Browser
        |
        v
Next.js frontend (SSR/ISR + hydration)
        |
        | REST/JSON
        v
Go API backend
        |
        v
MySQL + cache + Azure Storage
```

- **Go backend:** chỉ xử lý API, xác thực, nghiệp vụ, database, cache, rate limit,
  retry, metrics và upload Azure.
- **Next.js frontend:** gọi Go API; render HTML trên server cho nội dung public,
  sau đó hydrate để bật các tương tác bằng JavaScript.
- **Client-side API:** dùng cho login, reaction, comment, upload và các trang cá nhân.

## Phân loại trang

| Nhóm | Cách render | Ví dụ |
|---|---|---|
| Public, cần SEO | SSR hoặc ISR | `/`, `/post/:id`, `/category/:id`, tìm kiếm |
| Cá nhân, không cần SEO | Client-side fetch | `/my-posts`, `/my-liked`, notification |
| Tương tác | API JSON + cập nhật UI | reaction, comment, tạo/xoá post |

Bài viết mới hoặc dữ liệu thay đổi thường xuyên dùng **SSR**. Category và bài viết
ít thay đổi có thể dùng **ISR** với thời gian revalidate phù hợp. Metadata, canonical
URL, Open Graph, structured data (`Article`/`DiscussionForumPosting`) phải được tạo
trên frontend server.

## Kế hoạch triển khai

### Phase 1 — Chuẩn hoá backend

1. Giữ `models`, database wrapper, cache, materialized view, retry, rate limit,
   metrics và Azure storage.
2. Tạo `server/api/` với JSON handlers, DTO và response/error format thống nhất.
3. Thêm route versioning: `/api/v1/...`.
4. Thêm CORS chỉ cho origin frontend và xử lý `OPTIONS`.
5. Chọn auth: ưu tiên session token trong HttpOnly cookie; chỉ dùng JWT khi thật sự
   cần stateless/mobile. Không trả password hoặc dữ liệu nhạy cảm.
6. Viết OpenAPI/contract cho posts, categories, auth, comments, reactions và upload.

### Phase 2 — Dựng frontend

1. Tạo `/frontend` bằng Next.js.
2. Tạo API client, auth state, layout và route public.
3. Dùng server components/data fetching cho trang cần SEO.
4. Dùng client components cho form, reaction, comment và upload.
5. Thêm loading/error/empty state, sitemap, robots.txt và metadata động.

### Phase 3 — Chuyển từng tính năng

1. Chuyển `GET /posts`, `/posts/:id`, `/categories/:id/posts` trước để kiểm tra SEO.
2. Chuyển login/register/logout và auth middleware.
3. Chuyển tạo/xoá post, comment và reaction.
4. Chuyển SAS upload và giữ nguyên quarantine watcher.
5. Chạy SSR cũ và API/frontend mới song song trong giai đoạn chuyển tiếp.

### Phase 4 — Kiểm thử và loại bỏ SSR cũ

- Kiểm tra response API, quyền truy cập, rate limit và CORS.
- Kiểm tra HTML trả về khi tắt JavaScript; crawler phải thấy title và nội dung bài viết.
- Kiểm tra structured data, canonical, sitemap và preview khi chia sẻ mạng xã hội.
- Đo Core Web Vitals, thời gian phản hồi API và cache hit rate.
- Chỉ xoá templates/controller SSR sau khi toàn bộ route frontend ổn định.

## Đánh giá

Đây là hướng **hợp lý nhất** cho forum: API vẫn tái sử dụng được, frontend/backend tách
độc lập, trong khi các trang nội dung quan trọng vẫn có HTML sẵn cho crawler. Next.js
chỉ là lớp web rendering; Go vẫn là backend nghiệp vụ và API trung tâm. Tránh dùng
React/Vite SPA thuần nếu SEO bài viết là yêu cầu bắt buộc.
