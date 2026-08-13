# TradeMind 当前维护状态

更新时间：2026-08-13

2026-08-13 Sprint 5：Market Provider 配置补齐平台、来源、凭据引用、健康状态、最后成功与脱敏错误；新增环境变量 Credential Store 抽象且不存储明文 Secret。Market Signal / Performance CSV 增加上传预览、字段映射、逐行验证和安全确认导入；校准报告默认排除 fixture，输出 Recommendation、Score Bucket、维度与规则效果、Suggestion Only 和 Learning Readiness。Selection Config 正式版本化并要求 Review/人工 Activate，新分析保存版本而旧分析不变。ProductFlow 仅拆出 credentialstore、importcsv、calibration 三个小边界，没有改动既有外部 API 行为或真实发布能力。

2026-08-13 Sprint 4：批量候选分析增加暂停、恢复、取消、可重试失败恢复与任务明细；市场信号增加来源等级、Provider 状态、CSV/结构化导入、去重、新鲜度和多来源聚合；新增精确金额销售表现快照及 Selection Outcome Evaluation；Pricing Profile 增加不可变版本修订；选品工作台、主运营首页、分析任务页和费用设置页接入运营闭环。当前没有配置官方/授权的闲鱼或淘宝市场数据 Provider，系统明确显示未配置，不进行非授权抓取，也不实现真实平台发布。

2026-08-13 Sprint 2 新增精确金额 Cost/Profit Engine、可配置且默认未配置的 Platform Pricing Profile、SKU 级利润、确定性 Selection Score Engine、动态权重/置信度/Blocker、不可变 `candidate_analyses` 历史快照，以及 Admin 候选分析与成本利润中心。AI 仅作为可选解释层，失败时使用规则模板；真实需求和竞争数据继续显示 unknown。本阶段仍不连接或发布到闲鱼、淘宝。

2026-08-13 进入 Sprint 1 商品业务模型重构：新增 `source_products`、`candidates`、`listing_drafts`，并以兼容字段将旧 `products` 定义为 Catalog Product 实现基础。Collector 成功结果先幂等写入货源池，再保留旧商品草稿写入与 `result_product_id`；新 API 和 Admin 页面支持人工完成货源 → 候选 → 批准 → 商品库 → 闲鱼/淘宝草稿闭环。本阶段没有启用 AI 自动决策或任何真实平台发布。

2026-08-12 完成运行基线与 Collector 安全加固：统一 Node.js 24 LTS、pnpm 9.15.4、Go 1.25.x 的文档、CI 与 Docker 约定；Compose PostgreSQL 与后端复用同一组 `DB_*` 凭据；本地基础设施和 Collector 发布端口仅绑定回环地址；Backend→Collector `/v1/*` 使用至少 32 字符的内部 Token 鉴权，并限制 Collector JSON 请求体大小。

## 生命周期

TradeMind 已由项目所有者确认进入生产维护阶段。当前工作重点是稳定性、安全修复、必要功能维护和简洁的生产文档，不再在工作树中累积阶段性开发报告、一次性验收门禁或本地运行证据。

“生产维护阶段”描述项目生命周期，不代表仓库可以自动启用真实平台能力。真实凭据、真实平台网络、库存写入、后台 Worker、自动业务重试和灰度开关仍受现有 fail-closed 配置与外部生产审批控制，本次整理没有改变任何运行时开关。

## 验收模式

- GitHub Actions 是唯一持续保留的自动化测试执行入口。
- 前端、后端、契约、架构、数据库、Redis 和 Admin E2E 核心测试继续作为工作流依赖保留。
- 功能和业务签收由维护者人工完成，参考 `P10_MANUAL_ACCEPTANCE_CHECKLIST.md`。
- 本地 `trademind_test` 已由项目所有者删除，不要求重建；数据库集成测试由 CI 创建隔离 service container 执行。
- 本地生成的 Playwright 报告、测试结果、截图、临时日志和运行证据不纳入版本控制，完成诊断后可直接清理。

## 仓库整理

2026-08-09 完成生产维护清理：删除历史阶段 gate/fixture、负载测试、一次性验收脚本、阶段报告和本地测试产物；精简根脚本入口；将 PostgreSQL 库存集成覆盖直接接入通用 CI 命令。随后清除了残留 P6/P7 验证命令、未引用后端占位包、一次性 Admin 迁移脚本和 Admin/Collector 未使用符号；复用单一品牌图片、统一 Collector Playwright 版本，并将跟踪文档路径检查接入现有项目测试工作流。Demo/copy audit 运行输出仍保持为本地生成产物，不提交 Git；历史记录可从 Git 历史恢复。

## Admin 体验

2026-08-09 增加全局浅色/深色主题：默认保持浅色，登录后可通过顶部纯图标入口切换，悬浮提示目标主题并在本地持久化选择；桌面端完整品牌仅在侧栏展示，移动端品牌、主题和账户操作合并为单行固定顶栏；共享布局、登录页、运营总览及常用状态表面统一使用 Ant Design 主题 Token，主题切换采用同帧提交并暂停颜色插值，避免顶栏、侧栏和内容出现明暗混色；侧栏抽屉使用不透明主题表面遮挡滚动内容，深色切回浅色会完整恢复浮层与容器样式，登录/注册在移动端采用可滚动的安全居中布局；五档响应式与主题往返回归继续由现有 Admin CI 套件覆盖。

## 当前边界

- 不在本次整理中提交、推送、打 Tag 或发布 Release。
- 不创建或连接本地测试数据库。
- 不修改 API、权限、状态机、业务语义或生产能力开关。
- 后续变更仍须保持核心 CI 绿色，并完成人工验收说明。
## Sprint 6（工作区，待人工签收）

- 新增平台 Content Engine、不可变内容版本、人工审核与 Listing Ready 检查。
- 新增 Product Asset、public/internal 发布资料包、人工发布记录和 Performance 关联。

## Sprint 6.1（工作区，待人工签收）

- 本地图片经 MIME + 文件签名 + 解码三重校验写入 Listing 专属受控缓存；发布包只接受该目录内的真实图片。
- 内容生成和发布包支持 Redis 后台批次、分离并发、失败隔离、取消与失败项重试。
- 内容版本记录 AI provider/model/token/cost（Provider 可提供时）/latency，AI 失败继续降级模板。
- 新增人工质量记录与 Sell Test Readiness；不包含真实平台发布、Cookie 或自动采购能力。
- Admin 新增铺货工作台与三栏内容审核页；不包含真实闲鱼/淘宝自动发布。
