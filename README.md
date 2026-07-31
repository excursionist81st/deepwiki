# DeepWiki - 代码仓库知识库

一个基于 DeepSeek API 的代码仓库问答系统，实现"代码仓库 → 可问答知识库"的转换。

## 功能特性

- ✅ 拉取 GitHub 公开仓库并建立索引
- ✅ 自动跳过 .git/、vendor/、node_modules/ 等目录
- ✅ 支持用户自定义 include/exclude 规则
- ✅ 代码块追溯到来源文件、行号范围、语言
- ✅ 混合检索（向量检索 + 关键词检索）
- ✅ Rerank 重排序优化检索结果
- ✅ WebSocket 实时推送摄取进度
- ✅ 前端任务列表实时显示多个任务
- ✅ 流式响应，实时显示回答
- ✅ 异步摄取，支持进度查询
- ✅ 索引持久化，重启后无需重新摄取
- ✅ 函数/方法级分块（支持 206 种语言）
- ✅ 懒加载向量化（首次提问时计算并缓存）
- ✅ 向量化增强（包含函数名、签名、类名）
- ✅ AI 自主维护对话记忆（存储在 data/memory.md）
- ✅ 批量向量化与并发控制（避免API限流）
- ✅ 仓库重复检查（避免重复摄取）
- ✅ 权重可配置化（向量权重、关键词权重）

## 项目中遇到的问题与处理方式

**Q1: DeepSeek不提供embedding接口，如何实现RAG？**

A1: 使用第三方embedding服务，如阿里云百炼embedding API。

**Q2: 在摄取仓库时就完成代码块向量化，可能造成 token 浪费（只想克隆到本地，不会提问或先简单的看了看代码后才会提问）**

A2: 功能分离，在提问时完成代码向量化，只有在最后提问时才会使用 api 消耗 token。

**Q3: 多次提问，出现将代码块重复向量化的问题，token 浪费**

A3: 标记仓库是否被提问，提问过则已完成向量化，读取首次提问时存储的向量化结果。

**Q4: 标记仓库是否被提问不妥，可能因部分代码块向量化失败而全部重复向量化**

A4: 改为标记代码块是否向量化，增强颗粒度。

**Q5: 增强代码块颗粒度，但代码类型不同，读 context 太难**

A5: 解析 AST（抽象类型树）实现代码分块（gotreesitter，实现了大部分语言（本项目中，包含了 go, py, js, ts, java, cpp, cxx, cc, c, rs, rb, php）的 AST 解析）。

**Q6: 解析 AST 可能耗时较长（长注释，长字符串，多导入等造成的多无效节点）**

A6: 添加 ShouldSkip 检验跳过一些节点，避免无效检查子节点耗时。

**Q7: 使用AST实现函数/类级别代码块后，代码块数量大增，向量化耗时长**

A7: 向量化时批量向量化(25个一批)，减少API调用次数。

**Q8: 出现多批代码块时，单批向量化速度低**

A8: 使用go异步与并发限制和WaitGroup，提升向量化速度同时避免触发API限流。

**Q9: 如何增强向量检索能力？**

A9: 向量化代码块前强调函数签名，父级结构体等。

**Q10: 如何实现多轮对话？**

A10: 使用文件存储对话记忆（data/memory.md），AI 自主决定是否更新记忆。在回答末尾使用 `<memory_update>...</memory_update>` 标记更新内容，后端自动提取并追加到记忆文件中。

## 项目框架

```
deepseek_wiki/
├── main.go                 # 入口：初始化配置、数据库、路由，启动服务
├── go.mod                  # 依赖管理
├── go.sum                  # 依赖校验
├── README.md               # 项目文档
├── start.ps1               # Windows 启动脚本
├── deepwiki.env            # 配置文件示例
│
├── config/                 # 配置层
│   ├── config.go           # Viper 配置读取
│   ├── config.yaml         # 配置文件（数据库、DeepSeek API、QA 参数）
│   └── config.yaml.example # 配置范例
│
├── model/                  # 数据模型层
│   └── model.go            # Repository、IngestTask、CodeChunk、请求/响应结构体
│
├── dao/                    # 数据访问层
│   ├── database.go         # MySQL 连接初始化
│   ├── repository.go       # 仓库 CRUD
│   ├── task.go             # 任务 CRUD
│   └── codechunk.go        # 代码块 CRUD（含向量存储/读取）
│
├── pkg/                    # 工具包
│   ├── gitclone.go         # Git 克隆 + 文件过滤
│   ├── chunker.go          # AST 分块器（支持 206 种语言）
│   └── top.go              # 检索算法（向量、关键词、混合、Rerank）
│
├── deepseek/               # DeepSeek API 封装
│   └── client.go           # Chat、ChatStream、Embedding 方法
│
├── service/                # 业务逻辑层
│   ├── ingest.go           # 仓库摄取（异步任务、代码分块）
│   ├── qa.go               # 问答服务（向量检索、流式回答）
│   └── websocket.go        # WebSocket 服务（实时进度推送）
│
├── handlers/               # HTTP 处理层
│   └── handlers.go         # IngestHandler、StatusHandler、AskStreamHandler
│
├── routers/                # 路由层
│   └── route.go            # 注册 API 路由
│
├── frontend/               # 前端
│   └── index.html          # 单页面应用（主题切换、流式响应、任务列表）
│
└── data/                   # 数据存储
    ├── repos/              # 克隆的仓库本地目录
    └── memory.md           # AI 自主维护的对话记忆文件
```

## 快速开始

### 1. 配置

创建 `config/config.yaml`:

```yaml
database:
  host: localhost
  port: 3306
  user: root
  password: your_password
  name: deepwiki

deepseek:
  api_key: "your_deepseek_api_key"
  base_url: "https://api.deepseek.com"

alibaba:
  api_key: "your_alibaba_api_key"  # 阿里云百炼 API Key
  base_url: "https://dashscope.aliyuncs.com/api/v1/services/embeddings/text-embedding/text-embedding"

qa:
  top_k: 10                    # 检索代码块数量
  max_context_tokens: 4000     # 最大上下文 token 数
  batch_size: 25               # 批量向量化大小
  max_concurrency: 5           # 最大并发数
  max_text_len: 7000           # 最大文本长度
  vector_weight: 0.7           # 向量检索权重
  keyword_weight: 0.3          # 关键词检索权重

memory:
  file_path: "data/memory.md"  # 对话记忆文件路径
```

### 2. 创建数据库

```sql
CREATE DATABASE deepwiki CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

### 3. 获取 API Key

- DeepSeek API Key: https://platform.deepseek.com/
- 阿里云百炼 API Key: https://dashscope.console.aliyun.com/

### 4. 启动服务

**方式1：使用外部配置文件（推荐，更安全）**
```bash
# 1. 将 deepwiki.env 移动到 D:\env\deepwiki.env
# 2. 使用启动脚本加载
.\start.ps1

# 或指定自定义路径
.\start.ps1 -EnvPath D:\env\my-config.env
```

**方式2：使用 .env 文件（项目内）**
```bash
# 创建 .env 文件（参考 deepwiki.env）
.\deepwiki.exe
```

**方式3：直接运行**
```bash
go run main.go
```

服务将在 `http://localhost:8000` 启动

### 5. 访问前端

打开浏览器访问: `http://localhost:8000`

## 配置文件安全建议

### 推荐方案：外部配置文件

将配置文件放在项目目录外，即使打开项目文件夹也看不到敏感信息：

```bash
# Windows
D:\env\deepwiki.env           # 配置文件
D:\projects\deepseek_wiki\    # 项目目录
```

**启动方式：**
```powershell
.\start.ps1
```

**优势：**
- ✅ 配置文件完全隔离，项目内看不到
- ✅ 多个项目可共享同一配置文件
- ✅ 配置文件可单独备份和管理
- ✅ 团队协作时每人使用自己的配置文件
- ✅ 配置文件完全隔离，项目内看不到
- ✅ 多个项目可共享同一配置文件
- ✅ 配置文件可单独备份和管理
- ✅ 团队协作时每人使用自己的配置文件

## API 接口

### POST /api/ingest

摄取仓库（异步）

```bash
curl -X POST http://localhost:8000/api/ingest \
  -H "Content-Type: application/json" \
  -d '{
    "repo_url": "https://github.com/gin-gonic/gin.git",
    "repo_name": "gin_repo",
    "include_exts": [".go", ".md"]
  }'
```

**响应：**
```json
{
  "task_id": "task_1234567890",
  "message": "摄取任务已提交"
}
```

**错误：仓库已存在**
```json
{
  "error": "仓库 'gin_repo' 已存在，请使用不同的名称或删除现有仓库"
}
```

### WebSocket /api/ingest/progress

实时推送摄取进度

**连接：**
```javascript
const ws = new WebSocket('ws://localhost:8000/api/ingest/progress?task_id=task_xxx');
```

**消息格式：**
```json
{
  "task_id": "task_1234567890",
  "status": "processing",
  "progress": 50,
  "error": ""
}
```

**状态说明：**
- `processing`：处理中
- `completed`：完成（进度100%，自动关闭连接）
- `failed`：失败

**进度节点：**
- 0%：任务开始
- 10%：克隆完成
- 50%：分块完成
- 80%：保存完成
- 100%：任务完成

### GET /api/ingest/:id/status

查询摄取进度

```bash
curl http://localhost:8000/api/ingest/task_1234567890/status
```

### POST /api/ask

代码问答（流式响应）

```bash
curl -X POST http://localhost:8000/api/ask \
  -H "Content-Type: application/json" \
  -d '{
    "repo_name": "gin_repo",
    "question": "如何创建路由？"
  }'
```

**检索流程：**
1. 向量检索（语义相似度）
2. 关键词检索（精确匹配）
3. 混合检索（70% 向量 + 30% 关键词）
4. Rerank 重排序
5. 返回 Top-K 代码块


## 核心模块详解

### main.go

程序入口，执行以下初始化：

1. `config.Init()` - 读取配置文件
2. `dao.InitDB()` - 初始化数据库连接
3. `gin.Default()` - 初始化 Gin 框架
   - `r.Static()` - 实现 URL 映射，前端才能找到静态文件
   - `r.GET("/")` - 简化路径输入，返回 `index.html`
   - `routers.SetupRoutes(r)` - 注册 API 路由

### config/ 配置层

**config.go**
- `Init() error` - 初始化 Viper，读取 `./config/config.yaml`
  - 优先读取环境变量
  - 错误处理：区分配置文件不存在和语法错误
- `GetString(key string) string` / `GetInt(key string) int` - 包装 Viper 方法，避免空值错误

**config.yaml** - 实际配置文件（已加入 .gitignore）

**config.yaml.example** - 配置范例文件（供参考）

### model/ 数据模型层

定义以下结构体：
- `Repository` - 仓库信息（ID、名称、URL、本地路径、状态、HasEmbedding）
- `IngestTask` - 摄取任务（ID、仓库ID、状态、进度、错误信息）
- `CodeChunk` - 代码块（仓库ID、文件路径、行号范围、语言、SymbolName、SymbolType、ParentSymbol、Signature、内容、向量、向量化状态）
- `IngestRequest` / `IngestResponse` / `AskRequest` - 请求响应结构体

### dao/ 数据访问层

- `database.go` - MySQL 连接初始化，自动迁移表结构
- `repository.go` - 仓库 CRUD 操作
- `task.go` - 任务 CRUD 操作
- `codechunk.go` - 代码块 CRUD 操作（含向量存储/读取）

### pkg/chunker.go AST 分块器

**NewChunker()** - 获取一个 Chunker

**ChunkFile()** - 分块入口：
- 读取文件内容
- 获取文件扩展名和语言类型
- 调用 `getLanguage()` 获取 gotreesitter.Language
- 支持的语言调用 `chunkWithTreeSitter()` 返回函数/方法级分块
- 不支持的语言调用 `chunkGenericFile()` 返回文件级分块

**getLanguage()** - 根据扩展名返回语言对象（支持 Go、Python、JavaScript、TypeScript、Java、C/C++、Rust、Ruby、PHP）

**chunkWithTreeSitter()** - AST 解析分块：
- 创建指定语言的解析器
- 解析源代码生成 AST
- 获取根节点
- 调用 `getNodeTypes()` 获取节点类型映射
- 调用 `extractChunks()` 提取有效节点

**getNodeTypes()** - 节点类型映射：
- Go：`function_declaration` → function，`method_declaration` → method
- Python：`function_definition` → function，`class_definition` → class

**extractChunks()** - 提取代码块：
- 遍历 AST 节点
- 匹配节点类型，加入代码块
- 调用 `shouldSkipChildren()` 判断是否跳过子节点

**shouldSkipChildren()** - 判断可跳过的节点类型（comment、string、import_declaration 等）

**walkNode()** - 递归遍历 AST 节点

**nodeToChunk()** - 将节点转换为 CodeChunk：
- 获取字节范围
- 提取代码内容
- 获取行列信息
- 调用 `extractSymbolName()` 获取函数/类名

**extractSymbolName()** - 提取符号名称：
- 遍历子节点
- 调用 `isNameNode()` 判断是否为名称节点
- 返回第一个名称节点的内容

**isNameNode()** - 判断是否为名称节点（identifier、type_identifier、name 等）

**chunkGenericFile()** - 文件级分块

**detectLanguage()** - 根据扩展名判断语言类型

### pkg/gitclone.go 工具包

**GitCloner 结构体** - Git 克隆器

**FileFilter 结构体** - 文件过滤规则

**NewFileFilter()** - 初始化默认过滤规则：
- 排除目录：`.git`、`vendor`、`node_modules`、`__pycache__`、`.venv`、`dist`、`build`、`target` 等
- 排除扩展名：`.pyc`、`.so`、`.dll`、`.exe`、`.png`、`.jpg`、`.pdf`、`.zip` 等
- 最大文件大小：1MB（节约 token，避免 API 限制）

**Clone()** - 克隆仓库：
- 使用 `git.PlainClone()` 浅克隆（Depth=1，仅最新提交）
- 调用 `cleanFiles()` 执行文件过滤

**cleanFiles()** - 遍历文件并删除不符合规则的文件

**ShouldSkip()** - 判断文件是否应跳过（4 步匹配）：
1. 检查是否在排除目录下
2. 检查扩展名是否在排除列表
3. 如果设置了包含列表，检查是否在包含列表中
4. 检查是否超过文件大小限制

### service/ingest.go 摄取服务

**IngestRepo()** - 提交摄取任务：
- 生成任务 ID（`task_` 前缀 + 纳秒时间戳 + 3 位随机数）
- 创建仓库记录和任务记录
- 异步调用 `executeIngest()` 执行摄取

**executeIngest()** - 执行摄取流程：
1. 更新进度 0% → 状态 processing
2. 克隆仓库 → 进度 10%
3. 分块处理 → 进度 50%
4. 保存代码块 → 进度 80%
5. 完成 → 进度 100%

**chunkRepository()** - 遍历仓库文件，生成代码块：
- 使用 `filepath.Walk()` 遍历
- 调用 `chunker.ChunkFile()` 获取代码块

**saveCodeChunks()** - 将代码块批量存入数据库

**GetTaskStatus()** - 查询任务状态

**generateTaskID()** - 生成任务 ID

### service/qa.go 问答服务

**AskStream()** - 流式问答：
1. 获取仓库信息和代码块
2. 从配置读取 top_k
3. 调用 `retrieveRelevantChunks()` 检索相关代码块
4. 调用 `buildPrompt()` 构建提示词
5. 调用 `client.ChatStream()` 获取流式回答

**retrieveRelevantChunks()** - 向量检索：
1. 检查代码块是否全部完成向量化
2. 未完成则调用 `embedAllChunks()` 完成向量化
3. 更新仓库向量化状态
4. 调用 `embeddingWithRetry()` 将问题向量化
5. 遍历代码块，调用 `dao.GetEmbeddingFromChunk()` 获取向量
6. 调用 `cosineSimilarity()` 计算相似度
7. 使用 `sort.Slice()` 按相似度排序
8. 返回 Top-K 个最相关代码块

**embedAllChunks()** - 批量向量化：
- 从配置读取 batch_size、max_concurrency、max_text_len
- 遍历代码块，检查向量化状态
- 按批次分组（batch_size 个一批）
- 使用 goroutine 并发处理（max_concurrency 个并发）
- 调用 `buildEnhancedContent()` 构建增强内容
- 调用 `client.EmbeddingBatch()` 批量向量化
- 失败时降级为单个向量化
- 更新向量化结果或错误状态

**buildEnhancedContent()** - 构建增强向量化内容：
- 包含函数名、签名、父级类名、代码内容
- 提升检索准确度

**embeddingWithRetry()** - 带重试的向量化：
- 最多重试 3 次
- 失败时等待递增时间后重试

**buildPromptWithMemory()** - 构建包含记忆的提示词：
- 读取记忆文件内容
- 将记忆内容添加到提示词中
- 支持多轮对话上下文

**getMemoryFilePath()** - 获取记忆文件路径：
- 从配置文件读取 `memory.file_path`
- 默认为 `data/memory.md`

**readMemory()** / **updateMemory()** - 记忆文件读写：
- 读取或更新记忆文件

**extractMemoryUpdate()** - 提取记忆更新：
- 从 AI 回答中提取 `<memory_update>...</memory_update>` 标记
- 返回实际回答和记忆更新内容

**cosineSimilarity()** - 计算余弦相似度：
- 用于衡量问题向量与代码块向量的语义相似性

### deepseek/client.go API 封装

**NewClient()** - 创建 DeepSeek 客户端：
- 使用 OpenAI SDK v3
- 支持自定义 API Key 和 Base URL

**NewClientWithAlibaba()** - 创建带阿里云 Embedding 的客户端：
- 支持 DeepSeek Chat + 阿里云 Embedding
- 需要配置 alibaba.api_key

**ChatStream()** - 流式对话：
- 调用 `Chat.Completions.NewStreaming()` 获取流式响应
- 通过回调函数 `onChunk` 实时返回内容块

**Embedding()** - 文本向量化：
- 使用阿里云百炼 `text-embedding-v2` 模型
- 调用 https://dashscope.aliyuncs.com/api/v1/services/embeddings/text-embedding/text-embedding
- 设置 30 秒超时限制
- 返回 1536 维向量

**EmbeddingBatch()** - 批量文本向量化：
- 支持一次请求处理多个文本（最多25个）
- 减少API调用次数，提升效率
- 失败时返回 nil，调用方应降级为单个向量化

### handlers/ HTTP 处理层

**IngestHandler** - 处理仓库摄取请求：
- 绑定请求参数
- 初始化文件过滤规则
- 调用 `service.IngestRepo()` 提交任务
- 返回任务 ID

**StatusHandler** - 处理进度查询请求：
- 获取任务 ID
- 调用 `service.GetTaskStatus()` 查询状态
- 返回任务状态 JSON

**AskStreamHandler** - 处理问答请求：
- 绑定请求参数
- 支持临时 API Key 和 Base URL
- 设置 SSE 响应头
- 调用 `qaService.AskStream()` 流式返回答案

## 技术栈

| 层级 | 技术 | 版本 |
|------|------|------|
| Web 框架 | Gin | v1.12.0 |
| ORM | GORM | v1.31.2 |
| 数据库 | MySQL | - |
| 配置管理 | Viper | v1.21.0 |
| Git 操作 | go-git/v5 | v5.19.1 |
| AST 解析 | gotreesitter | v0.46.0 |
| LLM API | OpenAI SDK v3 | v3.44.0 |
| Embedding API | 阿里云百炼 | text-embedding-v2 |
| 前端 | 原生 HTML/JS + Fetch Stream | - |

## 数据流

```
用户提交仓库
    ↓
Git 克隆 → 文件过滤 → AST 分块（函数/方法级）
    ↓
存入 MySQL（代码块 + 元数据）
    ↓
用户提问
    ↓
读取对话记忆（记忆文件）
    ↓
问题向量化（阿里云）→ 检索 Top-K 代码块（余弦相似度）
    ↓
构建 Prompt（包含记忆） → DeepSeek 流式回答
    ↓
提取记忆更新 → 更新记忆文件
    ↓
前端实时显示（支持换行）
```

## Embedding API 说明

由于 DeepSeek Embedding API 返回 404 错误，项目改用阿里云百炼 Embedding API。

**对比：**

| 项目 | DeepSeek | 阿里云百炼 |
|------|----------|-----------|
| 端点 | /embeddings | /text-embedding/text-embedding |
| 模型 | deepseek-embedding | text-embedding-v2 |
| 状态 | 404 Not Found | ✓ 可用 |
| 维度 | - | 1536 |
| 价格 | - | ¥0.0007/千token |
| 批量支持 | - | ✓ 最多25个/批 |

**使用方法：**

1. 获取阿里云 API Key：https://dashscope.console.aliyun.com/
2. 配置 `config.yaml` 中的 `alibaba.api_key`
3. 启动服务即可使用

**工作流程：**

- 摄取仓库：保存代码块（不向量化）
- 首次提问：懒加载向量化（阿里云 Embedding）
  - 批量向量化（batch_size=25）
  - 并发控制（max_concurrency=5）
  - 避免API限流
- 后续提问：直接使用已存储的向量

**限流说明：**

阿里云百炼API有以下限流：
- **RPM限流**：每分钟请求次数限制
- **TPM限流**：每分钟Token消耗限制
- 限流按主账号维度计算，所有子账号、业务空间合并统计
- 超限后通常1分钟内自动恢复

**应对策略：**
- 代码层面已做并发控制（max_concurrency=5）
- 批量接口减少API调用次数
- 可在百炼控制台申请临时提额（有效期30天）
- 高峰期可考虑使用Batch API（不受实时限流约束）

## 许可证

MIT
