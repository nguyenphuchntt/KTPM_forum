# Ke Hoach Trien Khai Valet Key Pattern voi MinIO (S3-Compatible)

Tài liệu hướng dẫn chi tiết chuyển đổi hệ thống Upload ảnh từ Azure Storage sang mẫu thiết kế **Valet Key (Presigned URL)** sử dụng **MinIO**.

---

## 1. Tong Quan Kien Truc (Valet Key Pattern)

```
+--------+            1. Request Presigned URL            +-------------------+
|        | ---------------------------------------------> |   App Server      |
|        |                                                |  (Gatekeeper)     |
|        | <--------------------------------------------- |                   |
| Client |            2. Return Presigned PUT URL         +-------------------+
| (Web)  |
|        |            3. Direct PUT Upload (File Data)    +-------------------+
|        | ---------------------------------------------> |   MinIO Object    |
|        |                                                |      Storage      |
|        | <--------------------------------------------- |                   |
+--------+            4. HTTP 200 OK                      +-------------------+
```

---

## 2. Danh Sach Cac Viec Can Lam (Task Breakdown)

### Buoc 1: Cai dat MinIO SDK & Variables
* **Thu vien Go:** Thêm package Go official của MinIO:
  ```bash
  go get github.com/minio/minio-go/v7
  ```
* **Env Variables (`.env` & Docker):**
  ```env
  MINIO_ENDPOINT=localhost:9000
  MINIO_ACCESS_KEY=minioadmin
  MINIO_SECRET_KEY=minioadmin
  MINIO_USE_SSL=false
  MINIO_BUCKET_NAME=forum-uploads
  ```

---

### Buoc 2: Implement MinIO Storage Client (`server/cloud/minio_storage.go`)
Thay thế `azure_storage.go` bằng client MinIO với các chức năng:
* **Khởi tạo Client:** `minio.New(endpoint, &minio.Options{...})`
* **Kiểm tra/Tạo Bucket:** Tự động tạo Bucket `forum-uploads` nếu chưa tồn tại.
* **Cấu hình Public Read Policy:** Đảm bảo ảnh sau khi upload có thể truy cập công khai qua HTTP GET.
* **Sinh Presigned PUT URL:** Hàm `GeneratePresignedUploadURL(objectKey string, expiry time.Duration) (string, error)`
  ```go
  presignedURL, err := minioClient.PresignedPutObject(
      context.Background(),
      bucketName,
      objectKey,
      expiry,
  )
  ```

---

### Buoc 3: Cap nhat Upload Gatekeeper (`server/middleware/upload_gatekeeper.go`)
* Chuyển dependency trong `UploadGatekeeper` từ `AzureStorage` sang `MinIOStorage`.
* Trong method `GenerateUploadURL`:
  1. **Authen Check:** Giữ nguyên kiểm tra Session người dùng.
  2. **Metadata Validation:** Giữ nguyên kiểm tra file extension, mime-type (image/*), max size (< 5MB).
  3. **Object Key Generation:** Sinh key theo format: `images/{userID}/{timestamp}_{filename}`.
  4. **Presign URL:** Gọi `minioStorage.GeneratePresignedUploadURL(objectKey, 15 * time.Minute)`.
  5. **Response:** Trả về JSON chứa `upload_url` (Presigned PUT URL), `public_url` (Link truy cập ảnh), và `object_key`.

---

### Buoc 4: Cap nhat `cmd/main.go` va `server/routes/routes.go`
* **Trong `main.go`:**
  - Khởi tạo MinIO client khi khởi động ứng dụng.
  - Khởi tạo `UploadGatekeeper` với `MinIOStorage`.
* **Trong `routes.go`:**
  - Endpoint `POST /api/upload/request-url` giữ nguyên contract API hiện tại để không gây đứt gãy giao diện Frontend.

---

### Buoc 5: Cau hinh CORS cho MinIO
Trình duyệt sẽ gửi `PUT` request trực tiếp tới cổng MinIO (`:9000`), do đó MinIO bắt buộc phải bật CORS:
* Cấu hình CORS trên MinIO Server cho phép:
  - Allowed Origins: `http://localhost:8080` (hoặc domain client)
  - Allowed Methods: `PUT`, `GET`, `OPTIONS`
  - Allowed Headers: `Content-Type`, `Authorization`

---

### Buoc 6: Luong Phia Client (Frontend Integration)
Client thực hiện upload theo 2 bước:
1. **Bước 1 (Xin Valet Key):** `POST /api/upload/request-url`
   * Body: `{ "filename": "avatar.png", "size": 102400, "content_type": "image/png" }`
   * Nhận về `upload_url` (Presigned PUT URL).
2. **Bước 2 (Upload trực tiếp lên MinIO):**
   * HTTP Request: `PUT <upload_url>`
   * Header: `Content-Type: image/png`
   * Body: File Binary Blob.
