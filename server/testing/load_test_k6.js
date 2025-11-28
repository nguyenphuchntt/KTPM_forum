import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Trend, Rate, Counter, Gauge } from 'k6/metrics';
import exec from 'k6/execution';

// --- 1. ĐỊNH NGHĨA METRICS ---
// Các metrics mặc định
let errorRate = new Rate('errors');
let dbLockCounter = new Counter('db_lock_errors');

// Các metrics tùy chỉnh (Custom Metrics) để hứng dữ liệu từ server
let dbOpenConns = new Gauge('db_open_conns');
let dbIdleConns = new Gauge('db_idle_conns');
let dbWaitDuration = new Gauge('db_wait_duration');
let memoryUsage = new Gauge('memory_usage_mb');
let goroutines = new Gauge('goroutines_count');

// --- 2. CẤU HÌNH TEST ---
export const options = {
  scenarios: {
    // Scenario A: Tạo tải người dùng (Viral Event)
    viral_event: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '10s', target: 50 },  // Ramp-up
        { duration: '30s', target: 100 }, // Stress: 100 VUs
        { duration: '10s', target: 0 },   // Ramp-down
      ],
    },
    // Scenario B: Giám sát hệ thống (System Monitor)
    // Chạy song song 1 VU riêng chỉ để gọi API metrics
    monitor: {
      executor: 'constant-vus',
      vus: 1,
      duration: '55s', // Chạy lâu hơn scenario A một chút
    },
  },
  
  thresholds: {
    errors: ['rate<0.05'], // Cho phép lỗi < 5%
    http_req_duration: ['p(95)<3000'], // 95% request < 3s
  },
  
  // QUAN TRỌNG: Cấu hình để hiển thị p(99) trong báo cáo
  summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(90)', 'p(95)', 'p(99)'],
};

// --- 3. HÀM CHÍNH ---
export default function () {
  // Phân luồng dựa trên tên Scenario
  if (exec.scenario.name === 'monitor') {
    collectSystemMetrics();
  } else {
    runUserScenario();
  }
}

// --- 4. LOGIC TẠO TẢI (USER) ---
function runUserScenario() {
  let uniqueID = `user_${__VU}_${__ITER}_${Date.now()}`;
  let userCreds = {
    email: `${uniqueID}@test.com`,
    username: uniqueID,
    password: 'password123',
  };

  group('Onboarding', function () {
    // 1. Đăng ký
    let resReg = http.post('http://localhost:8080/signup', {
      email: userCreds.email,
      username: userCreds.username,
      password: userCreds.password,
      'password-confirmation': userCreds.password,
    });
    
    // Kiểm tra đăng ký
    check(resReg, { 'Registered': (r) => r.status === 200 });

    // 2. Đăng nhập
    let resLogin = http.post('http://localhost:8080/signin', {
      username: userCreds.username,
      password: userCreds.password,
    });
    
    let isLoggedIn = check(resLogin, { 'Logged In': (r) => r.status === 200 });
    if (!isLoggedIn) {
      errorRate.add(1);
      return; // Dừng nếu không login được
    }
  });

  group('Viral Action', function () {
    // 1. Xem bài viết (Read)
    let resView = http.get('http://localhost:8080/');
    check(resView, { 'View Home': (r) => r.status === 200 });

    // 2. Comment (Write) - Tỷ lệ 30%
    if (Math.random() < 0.3) {
      let resComment = http.post('http://localhost:8080/post/addcommentREQ', {
        postid: '1',
        comment: `Viral comment from ${uniqueID}`,
      });

      let success = check(resComment, { 
        'Commented': (r) => r.status === 200 
      });

      if (!success) {
        errorRate.add(1);
        if (resComment.status === 500) {
            dbLockCounter.add(1);
            // Log ít lại để tránh spam terminal
            if (__ITER % 10 === 0) {
               console.log(`🔥 DB Lock! User: ${userCreds.username}`);
            }
        }
      }
    }
  });

  sleep(1);
}

// --- 5. LOGIC THU THẬP METRICS (MONITOR) ---
function collectSystemMetrics() {
  // Gọi API metrics mà bạn đã cài vào server Go
  let res = http.get('http://localhost:8080/debug/metrics');
  
  if (res.status === 200) {
    try {
      let body = JSON.parse(res.body);
      
      // Đẩy số liệu vào Gauge để hiển thị cuối cùng
      dbOpenConns.add(body.db_connections_active);
      dbIdleConns.add(body.db_connections_idle);
      memoryUsage.add(body.memory_usage_mb);
      goroutines.add(body.goroutines);
      
      // Convert ns to ms for easier reading
      dbWaitDuration.add(body.db_wait_duration / 1000000); 
    } catch (e) {
      console.error("Failed to parse metrics JSON");
    }
  }
  sleep(1); // Lấy mẫu mỗi 1 giây
}