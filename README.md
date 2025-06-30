# OpenList

🗂 一个支持多存储的文件列表程序，使用 Gin 和 SolidJS，基于 AList 项目 fork 开发

> [!WARNING]
>
> **⚠️ 高风险实验性分支 - 严禁生产使用**
>
> 此存储库为 [OpenListTeam/OpenList](https://github.com/OpenListTeam/OpenList) 的**非官方实验性 Fork**，包含**高风险未验证功能**：
>
> - 🚫 **可能违反第三方服务 TOS** 的功能实现
> - ⚠️ **争议性技术方案** 和不稳定的实验代码
> - 🐛 **严重 BUG 和安全漏洞** 风险
> - 💥 **可能导致数据丢失、账号封禁** 等严重后果
>
> ---
>
> **⚠️ 使用条件与免责声明**
>
> - ✅ **仅限技术研究** 和功能验证，禁止商业或生产环境使用
> - 🔒 **使用者需具备** 充分的技术能力和风险承受能力
> - 📋 **一切风险和后果** 完全由使用者自行承担
> - 🚫 **不信任本人或缺乏技术经验者** 请立即停止使用
>
> **强烈建议：** 使用 [官方 OpenList 稳定版本](https://github.com/OpenListTeam/OpenList)。
>
> **特别声明：** 此分支的所有代码、构建产物及产生的任何后果与 OpenListTeam 完全无关。此分支**并非特立独行**，我会将合适的功能 PR 到官方仓库，尽量减少差异。

## 文档

- https://docs.oplist.org
- https://docs.openlist.team

## 使用方法

此存储库仅在 `ghcr.io` 上提供 CI 版本的 Docker 镜像。Docker Hub 上没有镜像。

存在两个镜像，一个镜像是用于替换原版 Alist，无需更改 Docker 挂载的目录：

```bash
docker pull ghcr.io/xrgzs/alist:main
```

一个镜像是 OpenList，如果从 Alist 迁移，需要更改 `/opt/alist` 为 `/opt/openlist`：

```bash
docker pull ghcr.io/xrgzs/openlist:beta
```

为了加快构建速度，仅构建 ARM64 和 AMD64 的镜像。

如果您需要在其他平台上运行，请自行构建。

## Demo

最好没有。

## 讨论

如果是本分支的特性，请私下讨论。

如果原版也有问题，请使用原版测试并反馈至上游，不得包含本分支。

## 许可

`OpenList` 是按 AGPL-3.0 许可证许可的开源软件。

## 免责声明

- 本程序为免费开源项目，旨在分享网盘文件，方便下载以及学习 golang，使用时请遵守相关法律法规，请勿滥用；
- 本程序通过调用官方 sdk/接口实现，无破坏官方接口行为；
- 本程序仅做 302 重定向/流量转发，不拦截、存储、篡改任何用户数据；
- 在使用本程序之前，你应了解并承担相应的风险，包括但不限于账号被 ban，下载限速等，与本程序无关。
