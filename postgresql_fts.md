# Ke Hoach Trien Khai Full-Text Search (FTS) voi PostgreSQL

Tài liệu hướng dẫn chi tiết từng bước tích hợp tính năng tìm kiếm toàn văn (Full-Text Search) cho bài viết (`posts`) trong dự án KTPM_forum sử dụng giải pháp tích hợp sẵn của **PostgreSQL (FTS + GIN Index)**.

---

## 1. Tong Quan Kien Truc

```
+------------------+         GET /api/v1/posts/search?q=...         +---------------------+
|                  | ---------------------------------------------> |   Go API Server     |
|                  |                                                | (Controller/Usecase)|
|  Client / Web    |                                                +---------------------+
|                  |                                                           |
|                  | <---------------------------------------------------------+
+------------------+              JSON (Posts + Rank + Snippets)               |
                                                                               v SQL Query (tsvector @@ tsquery)
                                                                    +---------------------+
                                                                    | PostgreSQL Database |
                                                                    |  (GIN Index + FTS)  |
                                                                    +---------------------+
```

---

## 2. Ke Hoach Chi Tiet Cac Buoc Trien Khai

### Buoc 1: Database Migration (PostgreSQL Schema)
Tạo file migration mới `server/repository/mysql/migration/20260926100000_add_fts_to_posts.sql`:

1. **Kích hoạt Extension `unaccent` (Hỗ trợ tìm kiếm tiếng Việt không dấu):**
   ```sql
   CREATE EXTENSION IF NOT EXISTS unaccent;
   ```

2. **Thêm cột `search_vector` tự động (Generated Column):**
   * Ưu tiên tiêu đề bài viết (Weight `A`), nội dung bài viết (Weight `B`).
   ```sql
   ALTER TABLE posts
   ADD COLUMN search_vector tsvector
   GENERATED ALWAYS AS (
       setweight(to_tsvector('simple', unaccent(coalesce(title, ''))), 'A') ||
       setweight(to_tsvector('simple', unaccent(coalesce(content, ''))), 'B')
   ) STORED;
   ```

3. **Tạo GIN Index để tăng tốc độ truy vấn FTS:**
   ```sql
   CREATE INDEX idx_posts_search_vector ON posts USING GIN (search_vector);
   ```

---

### Buoc 2: Implement Repository Layer (`server/repository/mysql/post/post.go`)
Bổ sung method `SearchPosts` trong Repository để gọi SQL FTS của PostgreSQL:

```sql
SELECT 
    p.id, p.user_id, p.title, p.content, p.like_count, p.dislike_count, p.comment_count, p.created_at,
    ts_rank(p.search_vector, query) AS rank,
    ts_headline('simple', unaccent(p.content), query, 'StartSel=<mark>, StopSel=</mark>, MaxWords=35, MinWords=15') AS snippet,
    COUNT(*) OVER() as total_count
FROM posts p, websearch_to_tsquery('simple', unaccent($1)) query
WHERE p.search_vector @@ query
ORDER BY rank DESC, p.created_at DESC
LIMIT $2 OFFSET $3;
```
* **`websearch_to_tsquery`**: Cho phép người dùng gõ từ khóa tự nhiên, hỗ trợ các toán tử như `"exact match"`, `OR`, `-exclude` mà không bị văng lỗi cú pháp SQL.
* **`ts_rank`**: Sắp xếp kết quả theo độ tương quan phù hợp của từ khóa.
* **`ts_headline`**: Tự động trích xuất đoạn văn ngắn chứa từ khóa và bọc thẻ `<mark>` để hiển thị highlight bên Frontend.

---

### Buoc 3: Implement Usecase Layer (`server/usecase/post.go`)
* Khai báo interface `SearchPosts(ctx context.Context, query string, page, pageSize int) (*dto.SearchPostResponse, error)`.
* Sanitize input `query` (loại bỏ khoảng trắng thừa, validate độ dài từ khóa tối thiểu 2 ký tự).
* Tính toán `limit` và `offset` cho phân trang (Pagination).

---

### Buoc 4: Implement Controller & DTO (`server/controller/post_api_controller.go`)
* **DTO Response (`server/dto/response/post.go`)**:
  ```go
  type SearchPostItem struct {
      PostResponse
      Rank    float32 `json:"rank"`
      Snippet string  `json:"snippet"`
  }

  type SearchPostResponse struct {
      Posts      []SearchPostItem `json:"posts"`
      TotalCount int              `json:"total_count"`
      Page       int              `json:"page"`
      PageSize   int              `json:"page_size"`
  }
  ```
* **Controller Handler**:
  - Endpoint: `GET /api/v1/posts/search?q={keyword}&page=1&page_size=10`
  - Đọc query parameter `q`, `page`, `page_size`.
  - Trả về JSON chứa danh sách bài viết khớp từ khóa, điểm tương quan (rank), đoạn trích dẫn highlight (snippet) và thông tin phân trang.

---

### Buoc 5: Dang ky Route & Middleware (`server/routes/routes.go`)
* Đăng ký route mới:
  ```go
  mux.HandleFunc("/api/v1/posts/search", postC.SearchPostsJSON)
  ```
*Áp dụng Rate Limiter cho endpoint search để tránh bị spam DDoS query nặng vào DB.*

---

### Buoc 6: Integration Testing & Benchmarking
1. Chạy migration thêm cột và chỉ mục GIN.
2. Kiểm tra câu lệnh `EXPLAIN ANALYZE` để đảm bảo Query Engine của PostgreSQL sử dụng chỉ mục `idx_posts_search_vector` (Bitmap Index Scan / GIN Scan).
3. Test thử các trường hợp:
   * Tìm kiếm có dấu & không dấu (ví dụ: `lập trình` vs `lap trinh`).
   * Tìm kiếm chính xác cụm từ trong ngoặc kép: `"golang framework"`.
   * Tìm kiếm loại trừ: `golang -java`.
