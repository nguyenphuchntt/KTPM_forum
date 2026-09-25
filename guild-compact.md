# Luồng chi tiết API (từ client gửi request đến server trả response)

Tài liệu này mô tả chi tiết quá trình xử lý 3 loại endpoint chính: **Auth JSON**, **Post Reaction**, và **SSR Trang chủ**.

---

## **1. POST `/api/v1/auth/signin` (Đăng nhập bằng JSON)**

### **Bước 1: Middleware xử lý sơ cấp**
```
client ← GET /api/v1/auth/signin (JSON body: {email, password})
        ↓
MetricsMiddleware:
  - Start timer, đếm in-flight request
  - Gán requestId cho log

RateLimitMiddleware (endpoint):
  - Kiểm tra global rate limit
  - Kiểm tra per-IP rate limit (ip:<ip>)
  - Nếu vượt: return 429 JSON {"error": {"message": "rate_limit_exceeded"}}
```

### **Bước 2: Handler chính**
```
mux → AuthController.SigninJSON(r)
    - Parse path value (không có)
    - Check method == POST? → không → 405 Method Not Allowed
```

### **Bước 3: Validation**
```
validators.BindSigninRequest(r)
  → Parse JSON body → malformed JSON → 400 {"error": {...}}

validators.ValidateSigninRequest(req)
  → Email rỗng hoặc password rỗng → 400 {"error": {"details": "invalid email"}}
```

### **Bước 4: Gọi Usecase**
```
c.Auth.Signin(usecase.SigninInput{Email: ..., Password: ...})

Trong AuthUsecase.Signin():
  1. userRepository.GetCredentialsByEmail(db, email)
     - SQL: SELECT id, password FROM accounts WHERE email = ? LIMIT 1
     - Nếu không tìm thấy → return ErrInvalidCredentials (401)

  2. bcrypt.CompareHashAndPassword(storedHash, inputPassword)
     - Sai → return ErrInvalidCredentials (401)
     - Đúng → tiếp tục

  3. config.GenerateSessionID() → chuỗi ngẫu nhiên 32 ký tự

  4. userRepository.StoreSession(db, userID, sessionID, expiresAt)
     - SQL: REPLACE INTO sessions (user_id, session_id, expires_at) VALUES (?, ?, ?)
     - expiresAt = now() + 24h
```

### **Bước 5: Cache session**
```
cache.GlobalSessionCache.Set(sessionID, userID, username, expiresAt)
  - TTL = 5 giây (session cache dùng cho validation nhanh)
```

### **Bước 6: Response**
```
← Controller:
  http.SetCookie("session_id", {
    Value: sessionID,
    HttpOnly: true,
    SameSite: "Lax",
    Path: "/",
    Expires: expiresAt
  })
  writeJSON(200, SigninResponse{
    UserID: accountID,
    Email: email,
    Username: username,
    SessionID: sessionID,
    ExpiresAt: expiresAt
  })
```

---

## **2. POST `/api/v1/posts/{id}/reactions` (Like/Dislike bài viết)**

### **Bước 1: Middleware**
```
client ← POST /api/v1/posts/123/reactions (body: {reaction: "like"})
        ↓
MetricsMiddleware (như trên)

RateLimitMiddleware:
  - Global + per-user check

RateLimitMiddleware.LimitCreatePost:
  - Key: "post:<userID>"
  - Cửa sổ: PostsPerHour (mặc định 10)
  - Nếu vượt: 429 {"error": "too_many_posts"}
```

### **Bước 2: Handler**
```
mux → PostController.ReactToPostJSON
    - r.PathValue("id") → "123" → strconv.ParseInt() → nếu <= 0 → 400
```

### **Bước 3: Validation Body**
```
validators.BindReactToPostRequest(r)
  → Parse JSON → malformed → 400

validators.ValidateReactToPostRequest(req)
  → reaction != "like" && reaction != "dislike" → 400 {"details": "reaction must be like or dislike"}
```

### **Bước 4: Xác thực người dùng**
```
AuthUsecase.RequireSession(r) → trả về SessionInfo{UserID, Username}
  - Cache GlobalSessionCache.get(sessionID) → hit → trả user_id
  - Miss → DB: SELECT user_id FROM sessions WHERE session_id = ? AND expires_at > NOW()
```

### **Bước 5: Gọi Usecase qua Transaction**
```
PostUsecase.ReactToPost(ReactToPostInput{
  AccountID: userID,
  PostID: 123,
  Reaction: "like"
})

Trong PostUsecase.ReactToPost():
  1. Kiểm tra PostID có tồn tại không (Authorization)
  2. Gọi postRepository.ReactToPost()
     - BEGIN TRANSACTION
     - SELECT reaction FROM likes WHERE user_id = ? AND target_type = 'post' AND target_id = ?
       - Nếu record tồn tại + reaction mới = "like" → UPDATE tăng like, giảm dislike
       - Nếu record tồn tại + reaction mới = "dislike" → DELETE record, giảm like
       - Nếu không tồn tại → INSERT và tăng like/dislike tương ứng
     - UPDATE posts SET 
         like_count = GREATEST(0, like_count + (reaction=="like"? 1 : -1)),
         dislike_count = GREATEST(0, dislike_count + (reaction=="dislike"? 1 : -1))
       WHERE id = 123
     - SELECT like_count, dislike_count FROM posts WHERE id = 123 → đọc lại sau tx
     - COMMIT TRANSACTION
  3. invalidatePostCache(123) → xóa key "post_123" trong AppCache
```

### **Bước 6: Response**
```
← Controller writeJSON(200, ReactToPostResponse{
  Likes: 42,
  Dislikes: 3
})
```

---

## **3. GET `/` (Trang chủ SSR)**

### **Bước 1: Middleware**
```
client ← GET /
        ↓
MetricsMiddleware (bắt buộc cho SSR)
RateLimitMiddleware (global + per-user/IP)
```

### **Bước 2: Handler SSR (bỏ qua usecase)**
```
mux → controllers.IndexPosts(w, r, db)
    - Kiểm tra method == GET? → không → 405
```

### **Bước 3: Session validation (cache hit thường)**
```
userRepo.ValidSession(r, db)
  1. Lấy cookie "session_id"
  2. cache.GlobalSessionCache.Get(sessionID)
     - Hit: trả về {UserID, Username, IsValid: true}
     - Miss: SQL SELECT ... FROM sessions WHERE session_id = ? LIMIT 1 → cập nhật cache

Kết quả: userID = 5, username = "alice", valid = true
```

### **Bước 4: Lấy danh sách bài viết**
```
PostRepository:
  1. postRepo.FetchPostIDsByTimestamp(db, offset=0, limit=10)
     - SQL: SELECT id FROM posts ORDER BY created_at DESC LIMIT 10 OFFSET 0
     - Trả về []PostID{101, 98, 87, 65, ...}

  2. getCachedPosts(postIDs)
     - Duyệt từng ID, check cache.AppCache.Get("post_101")
     - Nếu cache có → dùng luôn
     - Nếu cache không có → gọi FetchPostsByIDs()

  3. FetchPostsByIDs(db, missingPostIDs)
     - SQL: SELECT ... FROM posts WHERE id IN (…) JOIN accounts ON posts.user_id = accounts.id
     - Với mỗi post: tính thời gian hiển thị "x phút trước", định dạng ngày tháng
     - Gọi cacheAppCache.Set("post_101", post, TTL=5phút)

  4. Gộp kết quả thành []model.Post đầy đủ
```

### **Bước 5: Render Template HTML**
```
utils.RenderTemplate(db, w, r, "home", 200, posts, valid, username)

Trong RenderTemplate():
  1. ParseTemplates("home"):
     - Đọc header.html, navbar.html, home.html, footer.html
     - Kết hợp thành template "home"

  2. FetchCategories(db) → cache.GlobalCategoryCache
     - SQL: SELECT id, label FROM categories
     - Nếu lỗi → categories = nil (soft fail, không trả lỗi)

  3. Tạo GlobalData:
     {
       IsAuthenticated: true,
       Data: []model.Post,
       UserName: "alice",
       Categories: [{1, "Go"}, {2, "SQL"}, ...]
     }

  4. w.WriteHeader(200)

  5. t.ExecuteTemplate(w, "home.html", globalData)
     - Template nhận .UserName, .Likes, .Dislikes, .Comments từ Post.Get-methods
     - Render thành HTML thuần
```

### **Bước 6: Trả về HTML**
```
← Server trả về:
HTTP/200 OK
Content-Type: text/html

<!DOCTYPE html>
<html>
  <head>...title...</head>
  <body>
    <nav>... categories ... <a href="/post/101">Tiêu đề bài 101</a>...</nav>
    <main>
      <article class="post">
        <h2><a href="/post/101">Tiêu đề bài viết</a></h2>
        <p>Nguyên tác: alice</p>
        <span class="likes">👍 42</span>
        <span class="dislikes">👎 3</span>
        <span class="comments">💬 5</span>
      </article>
      ...
    </main>
  </body>
</html>
```

---

## **So sánh thời gian xử lý (cold cache vs warm cache)**

### **GET `/` với cache nóng:**
- RateLimitMiddleware → Cache hit (không DB)
- ValidSession → Cache hit
- FetchPostIDsByTimestamp → 1 query
- getCachedPosts → 0 query (tất cả post đều ở cache)
- FetchCategories → Cache hit
- **Tổng: ~10-20ms**

### **GET `/` với cache lạnh:**
- RateLimitMiddleware → Cache miss → 1 query SELECT session...
- FetchPostIDsByTimestamp → 1 query
- FetchPostsByIDs → 1 query batch lấy tất cả post
- FetchCategories → 1 query
- **Tổng: ~100-200ms**

### **POST `/api/v1/posts/{id}/reactions` (có session nóng):**
- Middleware xác thực → Cache hit
- Validation → In-memory (không DB)
- Postgres update + SELECT → 1 transaction
- Cache invalidation → xóa 1 key
- **Tổng: ~30-50ms**

---

## **Lưu ý quan trọng**

| Aspect | SSR | JSON API |
|--------|-----|----------|
| Xác thực | `userRepo.ValidSession` gọi trực Tiếp | `AuthUsecase.RequireSession` |
| Validation | Inline trong controller | `validators.Bind*/Validate*` |
| Response lỗi | `RenderError()` → HTML `error.html` | `writeAppError()` → JSON envelope |
| Cache | Bạn đọc (`getCachedPosts`, `getCachedPostByID`) | Bạn ghi (`invalidatePostCache`) |
| Transaction | SSR không dùng | API dùng `db.Begin()...Commit()` |

**Envelope JSON thành công:** `{"data": {...}, "pagination": {...}}`
**Envelope JSON lỗi:** `{"error": {"code": "unauthorized", "message": "...", "details": "..."}}`