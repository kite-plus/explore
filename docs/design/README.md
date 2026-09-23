# Explore 设计文档

Explore 的产品与技术设计。这些文档是**开发期的约束来源**，不是事后补写的说明书：与文档冲突的实现，先改文档，再改代码。

## 文档索引

| 文档 | 内容 | 读者 |
|---|---|---|
| [architecture.md](architecture.md) | 总体设计：两个核心判断、产品定位、收录、接入的博客系统、抓取与展示规则、里程碑、风险 | 所有贡献者，**先读这篇** |
| [data-model.md](data-model.md) | PostgreSQL 表结构、同步事务、查询、数据保留、schema 守护、迁移 | 后端 |
| [worker.md](worker.md) | 抓取流水线：调度、礼貌抓取、解析、规范化、检查（`explore check`） | 后端 |
| [api.md](api.md) | HTTP 接口：约定、公开接口、提交、管理、`/feed.xml` 与 OPML、错误码 | 后端、前端 |
| [frontend.md](frontend.md) | 前端（Astro）：选型、页面与路由、中英双语、组件规则、数据获取、提交流程、SEO、CSP | 前端 |
| [project-layout.md](project-layout.md) | 工程结构：目录、包的职责与依赖规则、命令、配置、测试、本地开发、部署 | 所有写代码的人 |

## 阅读顺序

1. `architecture.md` 的 **§0（两个核心判断）**。不认同这两条，后面的设计都没有意义。
2. `architecture.md` 的 §5、§6：支持哪些博客系统，抓取和展示遵守哪些规则。
3. 按要做的部分读 `data-model.md`、`worker.md`、`api.md` 或 `frontend.md`。
4. 动手前读 `project-layout.md`，以及 `architecture.md` 的 §12（现在必须定的，和现在不做的）。

## 文档约定

沿用 [Kite 设计文档的约定](https://github.com/kite-plus/kite/blob/main/docs/design/README.md)：

- **语言**：正文中文，专有名词保留英文。代码、注释、commit message 一律用英文。
- **`[EV]`**：结论有实测或既有项目佐证。
- **「现在留接口，不现在实现」**：将来做一件事时，如果只需新增代码、不需修改已有接口，就现在不做；反之必须现在做。
- **状态标注**：`[设计中]` 表示实现前还可调整；`[待定]` 表示明确推迟决策，通常在等 E0 实测的数据。
- **单一出处**：同一条规则只写在一个地方，其他文档链接过去。抓取与展示规则的数值只写在 `architecture.md` §6；代码里对应的常量只定义在 `internal/policy`。E0 回填阈值时，这两处一起改。

## 当前状态

| 文档 | 状态 | 最近更新 |
|---|---|---|
| architecture.md | 设计中（E0 开工前的基线） | 2026-09-23 |
| data-model.md | 设计中 | 2026-09-23 |
| worker.md | 设计中，阈值待 E0 回填 | 2026-09-23 |
| api.md | 设计中 | 2026-09-23 |
| frontend.md | 设计中，E2 开工 | 2026-09-23 |
| project-layout.md | 设计中 | 2026-09-23 |

里程碑定义见 [architecture.md §9](architecture.md#9-里程碑)。
