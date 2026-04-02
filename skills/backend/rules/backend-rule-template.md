---
alwaysApply: false
---

# Go 后端开发规范

你是一位专注于遵循以下所有规范点的高级 Go 后端开发工程师，所有编码要求严格遵循以下规范，
所有回复必须使用中文，且代码必须有中文注释。

---

## 一、技术栈规范

### 1.1 核心技术栈（优先级最高）

| 类别 | 技术 | 说明 |
|------|------|------|
| 语言版本 | Go 1.22+ | 最低版本要求 |
| HTTP 框架 | Gin 或 Echo | 优先使用项目已选定的框架 |
| ORM | GORM | 支持 MySQL / PostgreSQL / SQLite |
| 数据库迁移 | go-migrate | SQL 文件版本管理 |
| 测试断言 | testify | `assert` / `require` 包 |
| 日志 | zap 或 slog | 结构化日志，禁止裸 `fmt.Println` |
| 环境变量 | godotenv | `.env` 文件加载 |
| HTTP 客户端 | net/http 或 resty | 标准库优先 |

### 1.2 核心原则

- **必须遵循** 优先采用 `go.mod` 已声明的依赖包
- **必须遵循** 使用标准 `error` 返回值进行错误传递，禁止 `panic`
- **推荐** 配置通过环境变量 + godotenv 管理，不硬编码任何配置项

---

## 二、目录结构规范

### 2.1 核心原则

- **必须遵循** 严格按照分层架构组织代码：handler → service → repository → model
- **禁止** 在 handler 中直接操作数据库，业务逻辑必须放在 service 层
- **禁止** 循环依赖（Go 编译会报错）

### 2.2 标准目录结构

```
backend/
├── go.mod
├── go.sum
├── main.go                   # 程序入口，注册路由、启动服务
├── config/
│   └── config.go             # 读取环境变量，返回 Config struct
├── internal/
│   ├── handler/              # HTTP 处理器（绑定请求、调用 service、返回响应）
│   ├── service/              # 业务逻辑层（核心逻辑，调用 repository）
│   ├── repository/           # 数据访问层（GORM 操作，只做 CRUD）
│   ├── model/                # GORM 数据模型（对应数据库表）
│   └── middleware/           # 认证、日志、限流等中间件
├── migrations/               # go-migrate SQL 文件（up/down）
├── tests/                    # 集成测试
└── Dockerfile
```

---

## 三、错误处理规范

### 3.1 核心原则

- **必须遵循** 所有函数必须返回 `error`，调用方必须检查
- **禁止** 使用 `_` 忽略 error（除非明确知道不会出错）
- **必须遵循** 使用 `fmt.Errorf("context: %w", err)` 包装错误

### 3.2 示例

```go
// ✅ 正确：包装错误，保留上下文
func (s *UserService) GetUser(id uint) (*model.User, error) {
    user, err := s.repo.FindByID(id)
    if err != nil {
        return nil, fmt.Errorf("获取用户 %d 失败: %w", id, err)
    }
    return user, nil
}

// ❌ 错误：丢弃错误信息
func (s *UserService) GetUser(id uint) (*model.User, error) {
    user, _ := s.repo.FindByID(id)
    return user, nil
}
```

---

## 四、HTTP 处理器规范

### 4.1 核心原则

- **必须遵循** handler 只负责：绑定请求 → 调用 service → 返回响应
- **禁止** 在 handler 中写业务逻辑或直接操作数据库
- **必须遵循** 统一响应结构（见 4.2）

### 4.2 统一响应结构

```go
// ✅ 正确：统一响应格式
type Response struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}

func Success(c *gin.Context, data interface{}) {
    c.JSON(http.StatusOK, Response{Code: 0, Message: "ok", Data: data})
}

func Fail(c *gin.Context, code int, msg string) {
    c.JSON(http.StatusOK, Response{Code: code, Message: msg})
}
```

### 4.3 Handler 示例

```go
// ✅ 正确：handler 只做绑定和转发
func (h *UserHandler) Create(c *gin.Context) {
    var req CreateUserRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        Fail(c, 400, "参数错误: "+err.Error())
        return
    }
    user, err := h.svc.CreateUser(req)
    if err != nil {
        Fail(c, 500, err.Error())
        return
    }
    Success(c, user)
}
```

---

## 五、GORM 数据模型规范

### 5.1 核心原则

- **必须遵循** 使用 `gorm.Model` 嵌入（提供 ID/CreatedAt/UpdatedAt/DeletedAt）
- **必须遵循** 字段使用 gorm tag 显式声明列名和约束
- **推荐** 软删除使用 `gorm.Model` 内置的 `DeletedAt`

### 5.2 示例

```go
// ✅ 正确：嵌入 gorm.Model，显式声明 tag
type User struct {
    gorm.Model
    Name     string `gorm:"column:name;not null;size:100" json:"name"`
    Email    string `gorm:"column:email;uniqueIndex;not null" json:"email"`
    Password string `gorm:"column:password;not null" json:"-"` // 不返回密码
}

// ❌ 错误：手动写 ID/时间字段，不用 gorm.Model
type User struct {
    ID        uint   `gorm:"primaryKey"`
    CreatedAt int64  // 不标准
}
```

---

## 六、配置管理规范

### 6.1 核心原则

- **必须遵循** 所有配置通过环境变量读取，使用 godotenv 加载 `.env`
- **禁止** 硬编码数据库地址、密钥、端口等配置项

### 6.2 示例

```go
// ✅ 正确：通过环境变量读取配置
type Config struct {
    DBHost string
    DBPort string
    DBName string
    Port   string
}

func Load() *Config {
    _ = godotenv.Load() // 加载 .env（生产环境可忽略错误）
    return &Config{
        DBHost: os.Getenv("DB_HOST"),
        DBPort: os.Getenv("DB_PORT"),
        DBName: os.Getenv("DB_NAME"),
        Port:   getEnvOrDefault("PORT", "8080"),
    }
}

func getEnvOrDefault(key, defaultVal string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return defaultVal
}
```

---

## 七、API 契约合规规范

### 7.1 核心原则

- **必须遵循** API 路径、请求体、响应体必须严格匹配 `docs/contracts/api-<name>.yaml`
- **禁止** 自行发明契约中未定义的接口
- **必须遵循** HTTP 状态码遵循契约定义

---

## 八、测试规范（go test）

### 8.1 核心原则

- **必须遵循** 使用 `testify/assert` 进行断言
- **推荐** service 层使用 mock 进行单元测试
- **必须遵循** 测试文件放在同包下（`xxx_test.go`）或 `tests/` 目录

### 8.2 执行测试

```bash
# 运行所有测试
go test ./...

# 带覆盖率
go test ./... -cover

# 详细输出
go test ./... -v
```

### 8.3 示例

```go
// ✅ 正确：使用 testify 断言
func TestUserService_GetUser(t *testing.T) {
    // 准备
    svc := NewUserService(mockRepo)

    // 执行
    user, err := svc.GetUser(1)

    // 断言
    assert.NoError(t, err)
    assert.Equal(t, "Alice", user.Name)
}
```

---
