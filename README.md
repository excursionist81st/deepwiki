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
├── deepwiki.env.example    # 配置文件示例
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

**方式1：使用外部配置文件（推荐，更安全）**

将 `deepwiki.env.example` 复制并重命名为 `deepwiki.env`，放置到项目目录外：

```bash
# Windows 推荐路径
D:\env\deepwiki.env           # 配置文件
D:\projects\deepseek_wiki\    # 项目目录
```

**deepwiki.env.example 内容：**

```env
# DeepWiki 配置文件示例
# 将此文件移动到 D:\env\deepwiki.env

# 数据库配置
DATABASE_PASSWORD=your_password_here
DATABASE_NAME=deepwiki

# DeepSeek API
DEEPSEEK_API_KEY=sk-your_deepseek_key_here
DEEPSEEK_BASE_URL=https://api.deepseek.com

# 阿里云百炼 API
ALIBABA_API_KEY=sk-your_alibaba_key_here
ALIBABA_BASE_URL=https://dashscope.aliyuncs.com/api/v1/services/embeddings/text-embedding/text-embedding

# QA 配置
QA_TOP_K=10
QA_MAX_CONTEXT_TOKENS=4000
QA_BATCH_SIZE=25
QA_MAX_CONCURRENCY=5
QA_MAX_TEXT_LEN=7000
QA_VECTOR_WEIGHT=0.7
QA_KEYWORD_WEIGHT=0.3

# 记忆文件
MEMORY_FILE_PATH=data/memory.md
```

**启动方式：**
```powershell
# 默认读取 D:\env\deepwiki.env
.\start.ps1

# 或指定自定义路径
.\start.ps1 -EnvPath D:\env\my-config.env
```

**方式2：使用 config.yaml（项目内）**

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

**方式1：使用外部配置文件（推荐）**
```bash
.\start.ps1
```

**方式2：使用 config.yaml**
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

**优势：**
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

## 功能简介

- ✅ **仓库摄取**：给一个 GitHub 公开仓库地址，系统能拉取仓库并完成摄取（建立可检索的索引）
- ✅ **智能过滤**：摄取时自动跳过 .git/、vendor/、node_modules/、二进制、超大文件等，并支持用户自定义 include/exclude 规则
- ✅ **代码追溯**：每段代码块都能追溯到来源文件、行号范围、语言
- ✅ **智能问答**：能基于代码内容做问答，并在答案里指明引用了哪些文件（能定位到行号）
- ✅ **异步摄取**：摄取异步进行，提交仓库立即返回任务 ID，之后能凭 ID 查到进度
- ✅ **索引持久化**：索引持久化到本地，重启服务后不用重新摄取
- ✅ **REST 接口**：提供完整的 REST 接口可供验收
  - POST /api/ingest 传入 {repo_url}，返回任务 ID
  - GET /api/ingest/:id/status 查询摄取进度
  - POST /api/ask 传入 {repo_name, question}，返回答案及引用来源(流式)
- ✅ **端到端验收**：已测试 gin-gonic/gin 等真实公开仓库，摄取后能正确回答针对代码的问题
- ✅ **流式输出**：问答支持流式输出（POST /api/ask，走 SSE）
- ✅ **并发控制**：索引阶段支持并发、限速和失败重试
- ✅ **检索增强**：
  - 支持 Embedding（阿里云百炼 text-embedding-v2）
  - 支持按文件路径过滤
  - 支持 Top-K 后 Rerank 重排序
  - 支持关键词 + 向量混合检索（70% 向量 + 30% 关键词，可配置）
- ✅ **多仓库管理**：支持多仓库管理，向量库互相隔离(没有API鉴权)
- ✅ **WebSocket 推送**：摄取进度通过 WebSocket 实时推送
- ✅ **前端界面**：提供简单前端（支持主题切换、流式响应、任务列表、实时进度显示）

## 许可证

MIT
