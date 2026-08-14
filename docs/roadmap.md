# TradeMind 产品路线图

本路线图把 [`PRODUCT_VISION.md`](PRODUCT_VISION.md) 转换为近期实施顺序。它不是无限功能清单，也不承诺固定发布日期；实际完成状态以 [`PROGRESS.md`](PROGRESS.md) 为准。

## 当前基线

TradeMind 已具备 Source Product → Candidate → Catalog → Platform Listing 的商品主链路，并已实现精确 Pricing、规则 Selection、批量分析与 Ranking、市场信号/Performance 导入基础、Calibration、平台差异化内容、发布资料包和人工发布记录。

真实 1688 UAT 已验证专用登录会话、详情采集、二维 SKU 矩阵、队列 Worker、幂等刷新和多 SKU 风险定价。采购运费自动化 V1 已能区分已验证、估算、包邮和未知；当前真实样本只能确认“运费 ¥2 起”，因此保持 unknown，没有伪造固定运费。

这套基线仍主要处于 **Level 1（AI 辅助运营）**，部分批处理达到 Level 2。尚未形成“自动发现 → TOP N → 真实销售反馈”的稳定日常闭环。

## Now

Now 只容纳直接阻塞首批真实卖货闭环的工作。优先修复真实 UAT 问题，不并行扩展无关大型能力。

### 1. Freight V1 收口

状态：核心模型与真实样本验证已完成，继续做使用收口。

- 让未知运费进入清晰的人工补充队列，而不是逐商品隐藏处理
- 保留运营者已确认值，刷新供应数据时生成可追溯 snapshot
- 验证默认收货地区、采购数量和数量摊销在批量分析中的一致性
- 确保所有利润、推荐和 Ready Check 明确区分“含可靠运费”与“不含可靠运费”

完成条件：批量商品无需逐条填写已能确认的运费；unknown 商品能集中处理；没有 unknown 被当作零元确定成本。

### 2. 修复 Selection Score 高分膨胀

状态：V4 语义拆分已完成，待真实批次持续 UAT。

- Base Quality、Market Opportunity、Evidence Coverage、Confidence 与 Final Recommendation 已独立输出
- V4 `overallScore` 仅作为 Base Quality 兼容字段，不再单独表达“值得卖”
- Demand 与 Competition 均 unknown 时禁止 Recommend/Strong Recommend；单一可靠信号最高 Recommend
- Risk 与 Platform Fit 已改为证据状态语义，未知不再视为充分验证
- 用现有真实商品和覆盖不同证据状态的回归样本校验 Ranking 差异

完成条件：高基础利润但无市场证据的商品不会被包装成高确定性机会；评分、推荐和 Confidence 能独立解释。

### 3. 1688 Source Discovery V1

- 只使用合法、可验证且符合 Collector Security Gate 的来源
- 支持关键词、类目、价格区间、起订量、供应条件、排除词和风险条件
- 发现结果去重后进入 Source Product，再复用现有详情采集、Freight、Candidate 和分析链路
- 保存发现条件、来源、时间和原始证据；无法合规获得的数据保持 unknown
- 设计定时/批量任务的并发、重试、暂停、恢复和审计

完成条件：运营者无需先提供商品详情 URL，即可从一组经营条件产生一批可追溯 Candidate，且不依赖私有 API、Cookie 注入或反风控方案。

### 4. 自动发现到 TOP N 的最小闭环

- Source Discovery → 去重 → 详情采集 → Freight → Candidate → Pricing → Selection → Ranking
- 失败隔离、幂等和任务进度复用现有 Redis Worker/Batch 能力
- TOP N 展示真实证据、缺失项、假设和排名原因
- 只把人工批准的商品推进 Catalog、Listing 和内容生成

完成条件：一批发现任务能够后台完成并输出可人工审核的 TOP N，不要求运营者逐条触发分析。

### 5. 首批真实商品 UAT 与人工卖货测试

- 用真实商品持续验证 SKU、运费、成本刷新、内容、图片和发布包
- 选择少量低风险商品进行闲鱼人工发布测试，不做自动发布
- 回填平台 Listing ID、实际成本、曝光、咨询、订单、退款和实际利润
- 明确预计值与实际值，不让 fixture 进入真实指标

完成条件：至少形成一批从合法发现/采集到真实 Performance 回填的可追溯闭环样本。

## Next

Next 在 Now 形成稳定真实闭环后进入，顺序由 UAT 数据决定。

### 1. 真实经营反馈与成本对账

- 改善 Performance CSV/人工导入的日常操作体验
- 增加订单级或周期级实际采购成本、实际运费、平台费用和退款对账
- 建立预测利润与实际利润差异提示
- 提升 Listing、Content Version、Pricing Snapshot 与 Outcome 的关联完整度

### 2. 人工校准 Selection

- 使用真实样本生成 Recommendation、Score Bucket、Dimension 和 Rule Effectiveness 报告
- 样本不足时保持 `insufficient_sample`
- 调权只生成 Suggestion，经 Review、回测和人工 Activate 进入新 Selection Config
- 旧 Analysis 始终保留原配置版本

### 3. 供应侧变化监控

- 监控采购价、SKU、库存、图片和运费证据的新鲜度与变化
- 变化触发重新分析建议，不静默覆盖运营者 Pricing 参数或已发布价格
- 为缺货、成本上升和利润跌破阈值提供运营提醒

### 4. Level 2 日常自动化

- 配置每日发现数量、每日测试商品数量和风险边界
- 定时发现、采集、分析、排序并生成待审核队列
- 任务具备健康检查、异常恢复、审计和成本控制
- 人工继续负责批准、发布、重大调价和授权动作

## Later

Later 仅在真实使用证明价值且合规条件满足后进入。

- 取得合法官方授权后增加 Authorized Publish Provider
- 通过 Provider 扩展其他合法货源、市场信号和销售平台
- 在足够真实样本基础上开展受控 A/B、规则校准和 Level 3 自动化
- 将本地开发收敛为一键启动，将生产运行补齐监控、备份、恢复和升级演练
- 按真实用户和团队需求逐步引入 Tenant、角色、配额和计费，而非提前建设复杂 SaaS
- 仅在核心闭环明确需要时增加轻量供应商、采购或订单协作能力，不扩张成重型 ERP

## 明确暂缓

- 未授权平台抓取、私有接口、Cookie 注入、验证码或风控绕过
- Playwright 模拟平台发布、自动采购、自动付款和无人审批重大调价
- 缺少来源的市场数据、虚假“爆品”指标和自动修改 Selection 权重
- 完整 WMS/OMS、多仓、复杂财务、复杂 Tenant 和套餐计费
- 与“发现 → 决策 → 铺货 → 真实反馈”无直接关系的大型功能或重构

## 路线准入检查

新需求进入 Now 前必须回答：

1. 是否直接缩短或改善核心经营闭环？
2. 是否能复用现有 Source、Pricing、Selection、Batch、Listing、Performance 或 Provider 能力？
3. 需要哪些真实数据和 UAT 才能验收？
4. 是否制造无法验证的 AI 能力或确定性？
5. 是否减少日常人工步骤；若增加，是否是必要审批？
6. 是否引入新的安全、隐私、账号或平台合规风险？

无法给出清晰答案的需求默认留在 Later 或不做。
