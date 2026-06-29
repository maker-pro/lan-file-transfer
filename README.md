# lan-file-transfer

`lan-file-transfer` 是一个局域网文件互传 Web 工具。电脑启动服务后，手机、平板和电脑只要在同一个局域网内，就可以通过浏览器上传、下载、删除文件。

后端使用 Go，便于编译成 Windows、macOS、Linux 单文件程序；前端使用原生 HTML、CSS、JavaScript，不依赖前端框架。

## 功能特性

- 默认随机选择一个可用端口，也可以手动指定端口。
- 自动检测本机局域网 IP，并在终端打印访问地址。
- 页面显示二维码，手机扫码即可打开。
- 支持 Android、iOS、Windows、macOS、Linux 浏览器访问。
- 支持上传图片、视频、压缩包、Office、PDF 和任意普通文件。
- 支持大文件上传和下载，服务端使用流式处理。
- 支持浏览器文件夹上传，使用 `webkitdirectory` 并保留相对路径。
- 上传文件保存到 `uploads` 目录。
- 文件名重复时自动重命名，避免覆盖。
- 页面显示 `uploads` 目录下的文件列表。
- 支持点击下载文件。
- 下载文件夹时自动流式打包为 ZIP。
- 支持删除文件或文件夹，前端会二次确认。
- 所有文件访问都限制在 `uploads` 目录内，防止路径穿越。

## 目录结构

```text
lan-file-transfer/
├── .gitignore
├── go.mod
├── go.sum
├── main.go
├── README.md
├── start.bat
├── start.sh
└── web/
    ├── app.js
    ├── index.html
    └── styles.css
```

运行时会自动创建：

```text
uploads/
```

`uploads/`、`bin/`、`*.exe` 等本地文件已经写入 `.gitignore`，不会上传到 GitHub。

## 快速启动

### Windows

```bat
start.bat
```

指定端口：

```bat
start.bat 8080
```

也可以使用环境变量：

```bat
set LAN_FILE_PORT=8080
set LAN_FILE_UPLOAD_DIR=my-uploads
start.bat
```

### macOS / Linux

```bash
chmod +x start.sh
./start.sh
```

指定端口：

```bash
./start.sh 8080
```

也可以使用环境变量：

```bash
PORT=8080 UPLOAD_DIR=my-uploads ./start.sh
```

### 手动运行

```bash
go mod tidy
go run .
```

指定端口：

```bash
go run . -port 8080
```

指定上传目录：

```bash
go run . -dir my-uploads
```

默认不传 `-port` 时，程序会随机选择一个可用端口，并在终端打印实际访问地址。

## 使用方式

1. 确保手机和电脑连接到同一个 Wi-Fi 或局域网。
2. 在电脑上运行 `start.bat`、`./start.sh` 或 `go run .`。
3. 终端会打印访问地址，例如：

```text
http://<电脑局域网IP>:<端口>
```

4. 手机浏览器打开该地址，或扫描页面二维码。
5. 通过网页上传、下载或删除文件。

如果电脑有多个网卡，终端可能会打印多个地址。请选择和手机处于同一局域网的地址，通常是私有局域网地址。

## 手机文件夹上传说明

文件夹上传依赖浏览器的 `webkitdirectory` 能力。

桌面端 Chrome、Edge 等浏览器通常支持选择文件夹；手机端 Android、iOS 或微信内置浏览器可能只支持选择文件，不能选择整个文件夹。这是系统文件选择器和浏览器限制，不是后端限制。

手机端需要上传文件夹时，建议先在手机上把文件夹压缩为 `zip`、`7z` 或 `rar`，再上传压缩包。

## 后端接口

### `GET /`

返回 Web 页面。

### `POST /upload`

上传文件，使用 `multipart/form-data`。

字段：

- `path`：文件相对路径，文件夹上传时包含相对目录。
- `file`：文件内容。

服务端使用 `MultipartReader` 流式读取上传内容，不会把大文件一次性读入内存。

### `GET /files`

获取 `uploads` 目录下的文件列表。

返回示例：

```json
[
  {
    "path": "docs/report.pdf",
    "name": "report.pdf",
    "isDir": false,
    "size": 1048576,
    "modTime": "2026-06-29T10:00:00+08:00"
  }
]
```

### `GET /download?path=xxx`

下载文件或文件夹。

- 如果 `path` 是文件，直接流式下载。
- 如果 `path` 是文件夹，服务端会流式创建 ZIP 并下载。

### `DELETE /delete?path=xxx`

删除文件或文件夹。路径必须在 `uploads` 目录内。

### `GET /qr?text=xxx`

生成二维码 PNG 图片。页面二维码使用该接口生成。

## 构建

当前系统构建：

```bash
go build -buildvcs=false -o lan-file-transfer .
```

Windows：

```bash
GOOS=windows GOARCH=amd64 go build -buildvcs=false -o lan-file-transfer.exe .
```

macOS Intel：

```bash
GOOS=darwin GOARCH=amd64 go build -buildvcs=false -o lan-file-transfer-macos-amd64 .
```

macOS Apple Silicon：

```bash
GOOS=darwin GOARCH=arm64 go build -buildvcs=false -o lan-file-transfer-macos-arm64 .
```

Linux x86_64：

```bash
GOOS=linux GOARCH=amd64 go build -buildvcs=false -o lan-file-transfer-linux-amd64 .
```

Linux ARM64：

```bash
GOOS=linux GOARCH=arm64 go build -buildvcs=false -o lan-file-transfer-linux-arm64 .
```

Windows PowerShell 跨平台编译示例：

```powershell
$env:GOOS="windows"; $env:GOARCH="amd64"; go build -buildvcs=false -o lan-file-transfer.exe .
$env:GOOS="darwin";  $env:GOARCH="arm64"; go build -buildvcs=false -o lan-file-transfer-macos-arm64 .
$env:GOOS="linux";   $env:GOARCH="amd64"; go build -buildvcs=false -o lan-file-transfer-linux-amd64 .
Remove-Item Env:GOOS
Remove-Item Env:GOARCH
```

## 安全说明

- 本工具适合可信局域网内临时传文件。
- 不建议直接暴露到公网。
- 所有路径都会转换为安全相对路径。
- 禁止空路径、绝对路径、带 `:` 的路径和 `../` 路径穿越。
- 服务端会在拼接路径后再次确认目标仍位于 `uploads` 目录内。
- 删除接口不允许删除上传根目录。

## 上传 GitHub 前检查

建议执行：

```bash
git status --short
```

确认不要提交以下内容：

- `uploads/`
- `test-uploads/`
- `bin/`
- `*.exe`
- 私人文件、压缩包、照片、文档

这些路径已经写入 `.gitignore`，正常使用 Git 时不会被提交。
