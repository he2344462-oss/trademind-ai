# 模块关联索引

本文件用于帮助开发者和 AI Agent 判断“改一个点时还要检查哪些关联内容”。遇到不确定的改动，先查本表，再读取对应代码和文档。

## 通用原则

- 改代码时同步检查配置、示例、Docker、CI、文档和前端展示。
- 改公共契约时同步检查调用方，不只改定义处。
- 涉及敏感字段时同步检查加密、脱敏、日志和 `SECURITY.md`。
- 涉及较大模块或阶段性能力时同步更新 `docs/PROGRESS.md`。

## 关联检查表

| 改动类型 | 必须检查 / 同步 |
| --- | --- |
| 环境变量 | `.env.example`、`docker-compose.yml`、`docker-compose.full.yml`、`docs/env.md`、`docs/development.md`、`docs/docker-deployment.md` |
| 启动命令 / pnpm 脚本 | `package.json`、`README.md`、`README.en.md`、`docs/development.md`、`.github/workflows/*.yml` |
| Docker 部署 | `docker-compose.full.yml`、服务 Dockerfile、`.env.example`、`docs/docker-deployment.md`、`.github/workflows/docker.yml` |
| 后端 API | `backend/internal/api`、对应 handler/service/dto、`docs/api.md`、`admin/src/services`、`admin/src/types`、相关页面 |
| 统一返回 / 错误码 | `backend/internal/pkg/response`、所有调用方、`docs/api.md`、前端错误处理 |
| 管理端页面 | `admin/config/routes.ts`、`admin/src/pages`、`admin/src/services`、`admin/src/types`、README 能力描述、相关 docs |
| 数据库模型 / 自动迁移 | `backend/internal/modules/**/model`、`backend/internal/database`、`docs/architecture.md`、`docs/PROGRESS.md` |
| 异步任务 / 队列 | 任务 model/service/worker、Redis 配置、健康检查、`.env.example`、`docs/env.md`、任务中心页面 |
| AI Provider | `backend/internal/providers`、AI settings、Prompt 模板、调用记录、`docs/provider.md`、`docs/provider-template.md` |
| Storage Provider | Provider 接口、文件上传 API、settings.storage、本地/对象存储文档、`docs/provider.md` |
| Image Provider | 图片任务、队列、settings.image、任务页面、`docs/provider.md` |
| Platform Provider | 店铺授权、Token 加密、平台配置、订单/库存/客服调用方、`docs/provider.md`、`SECURITY.md` |
| 多平台 / 批量刊登 | `backend/internal/modules/productpublish`、`docs/MULTI_PLATFORM_PUBLISHING_DESIGN.md`、`docs/PUBLISH_BATCH_MIGRATION.md`、`docs/api.md`（batch-targets / batches）、`admin/src/pages/Product/PublishBatch*`、`admin/src/pages/Product/PublishTasks`、`admin/src/constants/publishLabels.ts`、`admin/src/constants/publishLimits.ts`、相关 CI 测试 |
| 商品经营四层模型 | `backend/internal/modules/productflow`、`backend/internal/modules/product`、`backend/internal/modules/collect`、`backend/internal/database/migrate.go`、`admin/src/pages/Commerce`、`admin/src/services/productFlow.ts`、`docs/api.md`；Catalog 复用 `products`，Listing Draft 不得直接依赖真实发布 Provider |
| Collector Provider | `collector/`、`collector/src/security` 出站安全策略、采集任务 API、队列、raw 原始数据、`docs/provider.md`、**1688 改解析时必读 [`docs/collector-1688-pitfalls.md`](collector-1688-pitfalls.md)** |
| 安全 / 密钥 / Token | 加密、脱敏、日志、环境模板、`SECURITY.md`、相关 settings 文档 |
| CI / 分支 / PR 流程 | `.github/workflows`、`docs/branching.md`、`CONTRIBUTING.md`、`.github/PULL_REQUEST_TEMPLATE.md` |
| 开源治理 | `README.md`、`README.en.md`、`docs/README.md`、`CHANGELOG.md`、`.github/*` |
| AI 工作流 / Agent 规则 | `AGENTS.md`、`docs/ai-workflow.md`、`docs/ai-coding-rules.md`、`docs/README.md`、`docs/cursor-rules-usage.md`、`.cursorrules`、`.cursor/rules/README.md`、必要时新增或更新 `.cursor/rules/*.mdc`、`CONTRIBUTING.md`、PR 模板 |

## 前后端联动

后端 API 或 DTO 变化时，必须检查：

- `admin/src/services/**` 的请求路径、方法、参数、响应字段。
- `admin/src/types/**` 或页面内类型定义。
- ProTable / ProForm 字段名、枚举、状态文案和空状态。
- `docs/api.md` 的接口契约。

## 配置联动

新增配置时优先判断配置归属：

- 部署级固定配置：进入唯一模板 `.env.example`，Docker 需要时同步 Compose。
- 可变业务配置：优先进入 settings 表和后台设置页。
- 敏感业务配置：存库时必须 AES-GCM 加密，前端展示必须脱敏。

## 文档联动

- README 只保留首页重点和入口。
- 细节放入 `docs/`，并在 `docs/README.md` 增加入口。
- 新增 AI 规则或关联说明时，同步 `AGENTS.md`、`docs/ai-workflow.md` 和 `.cursor/rules/README.md`。
- 重复出现的坑、质量门槛或工具协作经验，应写回对应 pitfalls、模块文档、`docs/PROGRESS.md` 或 AI 规则，避免只停留在单次对话。

## Sprint 2 成本利润与选品评分

相关实现包括 `backend/internal/modules/productflow/pricingengine`、`selectionengine`、`candidate_analyses`、Candidate 分析 API、Catalog 成本快照、Listing Pricing Profile、Admin Candidates 和 CostCenter。确定性规则负责金额与分数，AI 只解释且必须可降级。
# 预生产基础设施

Changes to `.env.example`, `deploy/preproduction/**`, or `deploy/scripts/*preproduction*` must be checked together with `docs/P10_PREPRODUCTION_ARCHITECTURE.md`, `docs/env.md`, `docs/docker-deployment.md`, workflow configuration, sensitive-diff checks, and the manual acceptance checklist. Production resources and credentials are outside the default writable scope.

## P10 Credential / Read-only / Control Modules

Changes under `backend/internal/modules/credentialp10`, `inventoryreadp10`, or `productioncontrolp10` must be checked with backend routing, migration, `adminperm`, metrics/redaction, config validation, Admin `/ops/p10-readiness`, `admin/src/services/p10Readiness.ts`, API/provider/security docs, environment templates, CI regression, and `P10_MANUAL_ACCEPTANCE_CHECKLIST.md`. `inventoryreadp10` may depend on exported inventory Provider/calibration/audit contracts. No production code may expose `sku.syncStock`, a Worker, scheduler, queue consumer, or automatic business retry without separate approval.
# Sprint 3 模块补充

批量选品继续归属 `backend/internal/modules/productflow`：`batch_analysis.go` 负责 Redis 批次与幂等领取，`rankingengine` 负责可解释排序，`market_signal.go` 保存带来源和新鲜度的信号，`pricing_profile.go` 负责数据库费用模型。Admin 对应 `Commerce/Candidates`、`Commerce/Recommendations` 与 `Settings/PricingProfiles`。

# Sprint 4 模块补充

批任务控制继续位于 `productflow/batch_control.go`，Worker 不复制领域状态机。`market_signal.go` 负责合法来源 Provider 边界、导入、新鲜度和聚合；`performance.go` 负责用户授权销售表现导入与预测结果评估。新表由 `backend/internal/database/migrate.go` 正式迁移。Admin 对应 `Commerce/AnalysisBatches`、选品工作台、主运营首页与平台费用页面。

# Sprint 5 有限领域拆分

ProductFlow 外部 API 保持兼容，内部新增三个小边界：`credentialstore` 负责环境变量/KMS 可替换的凭据引用；`importcsv` 负责 Market 与 Performance 共用的安全 CSV 解析；`calibration` 负责不依赖数据库的 Recommendation、Score Bucket、维度、规则效果和 Readiness 计算。主包中的 `provider_config.go`、`selection_config.go`、`calibration_service.go` 继续承担应用服务与持久化编排，未重写既有 ProductFlow。

Admin 对应 `Commerce/DataImports`、`Commerce/Calibration`、`Settings/MarketProviders` 和 `Settings/SelectionConfigs`。所有真实覆盖统计默认排除 fixture；Selection Config 激活必须写 Operation Log，新旧 Candidate Analysis 通过版本字段隔离。
## Sprint 6 内容与半自动铺货

- `backend/internal/modules/productflow/contentengine/`：平台 Profile、模板生成、事实边界、标题/关键词与 Content Risk Guard。
- `backend/internal/modules/productflow/listing_content.go`：内容版本、人工审核、资产清单、发布资料包和 Manual Publish 记录。
- `backend/internal/modules/productflow/listing_assets.go`：运营者本地图片上传、真实文件类型校验、hash 去重、受控缓存、只读货源图边界与安全预览。
- `backend/internal/modules/productflow/listing_batch.go`：内容生成/发布包 Redis 后台批次、并发限制、失败隔离、重试与幂等结果关联。
- `backend/internal/modules/productflow/content_quality.go`：人工内容质量结论与事实准确性记录；仅用于验收记录，不自动训练或调权。
- `admin/src/pages/Commerce/ListingDrafts` 与 `ListingReview`：铺货工作台和三栏审核工作区。
