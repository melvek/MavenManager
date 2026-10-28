# MavenManager

MavenManager 是一个基于 Go、Wails v3 和 Vue 3 的桌面应用，用于浏览和维护本机 Maven 本地仓库。它可以查看依赖及版本、识别常见缓存残留、修复缺失依赖，并按 Maven 仓库结构导出依赖包。

## 功能

### 仓库浏览

- 管理多个本地 Maven 仓库，并通过 UUID 标识仓库。
- 浏览 Group、Artifact 和版本，查看版本文件、大小及修改时间。
- 查看仓库中的 Group、包、版本和文件统计。
- 启动应用后不会自动扫描仓库；选择仓库后，可按需开始扫描。

### 垃圾扫描与清理

扫描时可选择检查以下项目：

- Maven 下载失败标记（`.lastUpdated`）
- 来源记录（`_remote.repositories`）
- 中断下载残留（`*.in-progress`）
- 零字节 JAR / POM
- 空目录
- POM 声明需要 JAR，但对应 JAR 缺失的诊断项

扫描结果支持选择和批量处理。文件可移入系统回收站，也可选择直接删除；删除缺失 JAR 诊断项时会按整个版本目录处理。`.lastUpdated` 项可尝试重新下载。删除和修复后不会自动重新扫描。

HTML 错误页、非 Maven `project` XML 和结构损坏的 POM 不会作为有效 POM 参与缺失 JAR 检查。

### 依赖修复

- 可对缺失 JAR 或 `.lastUpdated` 对应的版本重新下载。
- 优先使用 Maven 自身的 `settings.xml` 仓库和镜像配置。
- 也可在应用设置中配置额外的 HTTP(S) 远程仓库；应用不会修改 Maven 的 `settings.xml`。
- Maven 路径可以填写 Maven 根目录或 Maven 命令路径。填写根目录时，应用会查找 `bin/mvn`（Windows 为 `bin/mvn.cmd`）及该目录下的 `conf/settings.xml`；也兼容 `config/settings.xml`。

### 批量导出

在依赖详情的版本表格中，将所需版本加入“导出列表”，即可从多个 Artifact 汇总选择。导出时可选择生成 Windows `.bat` 或 Linux/macOS `.sh` 安装脚本，并将所选版本打包为 ZIP。

ZIP 中的依赖文件按标准 Maven 仓库结构排列，例如：

```text
com/example/library/1.2.3/library-1.2.3.jar
com/example/library/1.2.3/library-1.2.3.pom
install-to-maven.bat
README.txt
```

可将 ZIP 解压到另一个 Maven 本地仓库的根目录，直接复制这些文件；也可解压到任意目录后运行对应脚本，通过 Maven `install:install-file` 命令批量导入本地仓库。

## 开发环境

- Go（版本要求见 [`go.mod`](./go.mod)）
- Node.js 和 npm
- Wails v3 CLI（`wails3`）
- Task（运行项目提供的 Taskfile 构建任务）

## 运行与构建

在项目根目录执行：

```powershell
# 启动 Wails 开发模式
wails3 dev -config ./build/config.yml

# 构建当前平台应用
wails3 task build

# 运行构建结果（Windows）
wails3 task run

# 运行 Go 测试
go test ./...
```

前端单独开发或构建：

```powershell
cd frontend
npm install
npm run dev
npm run build
```

Windows 下默认构建产物为 `bin/mvnmanager.exe`。首次构建前应按 Wails v3 的平台要求安装相应系统构建依赖。

## 配置与数据

应用配置文件与运行程序放在同一目录：

- `config.json`：Maven 命令/安装路径、仓库列表、当前仓库及额外远程仓库。
- `repository-info.json`：仓库统计、上次扫描时间和上次清理时间。

删除仓库时会同步清理对应的仓库信息。开发模式下，配置文件保存在项目运行目录。

## 技术栈

- 后端：Go、Wails v3
- 前端：Vue 3、TypeScript、Vite、Element Plus
- Maven 依赖操作：调用 Maven 命令及 Maven Dependency / Install 插件
