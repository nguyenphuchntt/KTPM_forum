# Giới thiệu dự án gốc
Ứng dụng forum xây dựng bằng ngôn ngữ Golang, cho phép người dùng tương tác với nhau thông qua các bài đăng, bình luận và reaction.

## Tác giả
- Abdelhamid Bouziani
- Hamza Maach
- Omar Ait Benhammou
- Mehdi Moulabbi
- Youssef Basta

## Tính năng chính
### 1. Authentication & Session Management
- *Cơ chế:* Cookie-based Authentication.
- *Bảo mật:* Password hashing.
- *Session:* Kiểm tra tính hợp lệ của session trên mỗi request (ValidSession()).
### 2. Tương tác người dùng
- *Bài viết:* Đăng bài viết theo các danh mục.
- *Tương tác:* Bình luận và thả cảm xúc.
- *Spam protection:* Giới hạn tốc độ comment bằng throttling.
### 3. Filtering & Pagination
  - Theo danh mục: /category/{id}
  - Bài viết đã tạo: /mycreatedposts
  - Bài viết đã thích: /mylikedposts
- *Phân trang:* Tối ưu hóa query với tham số PageID.
---
## Cải tiến
### 1. Tối ưu hóa Database sử dụng MySQL
- *Thay SQLite thành MySQL:* SQLite khi viết vào bảng thì khóa cả DB trong khi MySQL chỉ khóa hàng
- *N+1 Queries:* Chuyển đổi từ vòng lặp insert sang insert ở một câu lệnh duy nhất.
- *Materialized View:* Tạo bảng view đã tính toán sẵn các chỉ số (số like, comment, dislike) thay vì dùng count(*) trên 4 bảng mỗi khi load post.
- *Indexing:* Đánh index cho các cột thường xuyên sử dụng (thời gian, userid, postid).
### 2. Caching và Rate limiting
- *In-memory Caching:* Thêm lớp caching layer cho các trang hiển thị bài viết.
- *Rate Limiting Middleware:*
  - *Global:* Sử dụng thuật toán Token Bucket.
  - *Per User:* Sử dụng Fixed Window.
  - *Per IP:* Giới hạn request theo địa chỉ IP.
### 3. Reliability & Monitoring
- *Retry Pattern:* Tự động thử lại khi mất kết nối Database, Query, Write, hoặc lỗi mạng (đăng lại post, comment).
- *Monitoring Stack:*
  - *Prometheus & Grafana:* Theo dõi Uptime, System Health, HTTP Status, Response Time Percentiles (p95, p99), DB Latency, …
  - *Logging:* Sử dụng zerolog.
### 4. Secure File Upload Architecture
1.  *Pipes and Filters (Client-side Validator):* File đi qua 4 bộ lọc nối tiếp:
    * SizeFilter: Kiểm tra kích thước.
    * ExtensionFilter: Kiểm tra đuôi file.
    * MimeTypeFilter: Kiểm tra Content-Type.
    * MagicBytesFilter: Kiểm tra chữ ký file (Header signature) để chống giả mạo đuôi file.
2.  *Gatekeeper:* Client gọi đến /api/upload/request-url và hệ thống xác thực quyền upload và xác thực metadata của file được upload lên.
3.  *Valet Key:* Server trả về SAS Token. Client sử dụng Token để có thể upload trực tiếp lên *Azure Blob Storage*.
4.  *Quarantine:* Quarantine Watcher sẽ theo dõi file upload lên Quarantine Container và lấy và lấy về 512B đầu tiên của file để kiểm tra chữ ký file trước khi đưa qua Container chính.
---
## Kết quả kiểm thử hiệu năng
Hệ thống đã được kiểm thử tải với 100 Virtual Users (VUs) liên tục, thự hiện ba kịch bản kiểm thử sau:
- Kịch bản 1: 100VUs read-only liên tục trong 1 phút (50% truy cập vào /category/page, 50% truy cập vào /post_id)
- Kịch bản 2: 100VUs write-only liên tục trong 1 phút (tạo post mới)
- Kịch bản 3: 100VUs kết hợp write và read (50% tạo post mới, 25% truy cập vào /category/page, 25% truy cập vào /post_id)

Kết quả so sánh giữa phiên bản gốc và phiên bản tối ưu:

| Kịch bản | Chỉ số | Trước cải tiến | Sau cải tiến |
| :--- | :--- | :--- | :--- |
| **Tổng thể** | Tổng req | 10,628 | 12,655 |
| | Req rate | 49.34 | 59.15 |
| | Failed req rate | 22.59% | **0.00%** |
| **Read Only** | Response Time P95 | 642.91ms | **8.84ms** |
| | Response Time Avg | 307.47ms | 7.26ms |
| | Response Time Max | 1,464ms | 233.64ms |
| | Success rate | 99.63% | 100.00% |
| **Write Only** | Response Time P95 | - | 647.18ms |
| | Response Time Avg | - | 161.89ms |
| | Response Time Max | - | 929.90ms |
| | Success rate | 0% (Thất bại toàn bộ) | **100.00%** |
| **Mix (50/50)** | Response Time P95 | 5,023ms | **82.74ms** |
| | Response Time Avg | 1,434ms | 40.95ms |
| | Response Time Max | 10,658ms | 658.21ms |
| | Create Posts success rate | 0% | 100.00% |

---
## Cài đặt
### 1. Clone Repository
```bash
git clone https://github.com/nguyenphuchntt/KTPM_forum.git
cd KTPM_forum
```
### 2. Cấu hình .ENV
Trước tiên, cần phải tạo 2 container với public access level là Private (no anonymous access) và container còn lại là Blob (anonymous read access for blobs only) trong một storage account. Sau đó vào phần Access keys của storage account đó để lấy Connection String rồi thêm các biến môi trường sau vào file .env tại thư mục gốc
```bash
AZURE_STORAGE_CONNECTION_STRING=”<connection_string>”
AZURE_QUARANTINE_CONTAINER=”<private_container_name>”
AZURE_PRODUCTION_CONTAINER=”<blob_container_name>”
```
### 3. Build
```bash
docker compose up –build -d
```
Ứng dụng chạy trên localhost:8080.
Theo dõi các thông số của hệ thống tại localhost:3000

# Báo cáo đầy đủ [pdf](./docs/report.pdf)
