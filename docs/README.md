# TradeMind 文档中心

项目已进入生产维护阶段。工作区只保留当前开发、部署、运维和人工验收所需文档；历史阶段报告、一次性门禁报告和运行证据不再保存在当前工作树中，必要时从 Git 历史查询。

## 产品方向

- [产品愿景与北极星](PRODUCT_VISION.md)
- [Now / Next / Later 路线图](roadmap.md)
- [当前维护状态](PROGRESS.md)

## 使用与部署

- [本地开发](development.md)
- [Docker 部署](docker-deployment.md)
- [环境变量](env.md)
- [API 契约](api.md)
- [Provider 扩展](provider.md)
- [系统架构](architecture.md)

## 生产运维

- [生产边界](P10_PRODUCTION_BOUNDARY.md)
- [预生产架构](P10_PREPRODUCTION_ARCHITECTURE.md)
- [人工验收清单](P10_MANUAL_ACCEPTANCE_CHECKLIST.md)
- [风险登记](P10_RISK_REGISTER.md)
- [可观测性架构](P5_OBSERVABILITY_ARCHITECTURE.md)
- [备份架构](P6_BACKUP_ARCHITECTURE.md)
- [恢复架构](P6_RESTORE_ARCHITECTURE.md)
- [发布架构](P6_RELEASE_ARCHITECTURE.md)
- [数据库回滚边界](P6_DATABASE_ROLLBACK_BOUNDARY.md)
- [灾难恢复计划](P6_DISASTER_RECOVERY_PLAN.md)

## 工程协作

- [AI 工作流](ai-workflow.md)
- [AI 编码规则](ai-coding-rules.md)
- [模块关联索引](module-map.md)
- [任务检查清单](task-checklist.md)
- [分支与 PR](branching.md)

## 验收约定

- 自动化测试只通过 `.github/workflows/` 持续执行；不得删除工作流依赖的核心测试。
- 功能、页面和业务流程的最终签收由人工按验收清单完成。
- 本地不要求创建测试数据库；CI 使用隔离的 PostgreSQL/Redis service container。
- 不为单次阶段、批次或验收创建持久化 gate、fixture 报告、截图报告或 `artifacts/` 证据。
