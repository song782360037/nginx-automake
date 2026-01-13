# nginx-automake

一个使用 Go + Gin 实现的 **Nginx 模块定制编译平台**，支持解析 `nginx -V` 输出、选择第三方模块、排队编译并下载产物。前端页面全部中文，开箱即用。

## 功能亮点

- 自动解析 `nginx -V` 输出（版本、编译参数、内置模块、编译器信息）。
- 预置常用第三方模块列表，可启用/禁用与搜索。
- 支持自定义第三方模块（提供 Git 仓库地址）。
- 服务端执行编译，生成与原环境兼容的 Nginx 二进制。
- 编译任务队列、并发控制与实时进度/日志。
- Docker 部署，环境隔离。

## 快速开始

### 本地运行

```bash
go mod tidy
PORT=8080 MAX_WORKERS=2 MODULES_DIR=./modules WORKDIR=/tmp/nginx-build go run .
```

访问 `http://localhost:8080` 即可使用。

### Docker 运行

```bash
docker build -t nginx-automake:latest .
docker run --rm -p 8080:8080 \
  -e MAX_WORKERS=2 \
  -e MODULES_DIR=/app/modules \
  -e WORKDIR=/tmp/nginx-build \
  nginx-automake:latest
```

## 模块目录说明

- 默认模块列表在 `config/modules.json` 中。
- 预置模块默认从 `MODULES_DIR` 目录读取，例如：

```
modules/
  ngx_brotli/
  lua-nginx-module/
  headers-more-nginx-module/
```

如果预置模块目录不存在且配置中提供了仓库地址，系统会自动 `git clone` 到编译目录。

## 典型 nginx -V 输出示例

```text
nginx version: nginx/1.24.0
built by gcc 11.2.0 (Debian 11.2.0-19)
configure arguments: --with-http_ssl_module --with-threads
```

## 配置项

| 环境变量 | 说明 | 默认值 |
| --- | --- | --- |
| PORT | 服务端口 | 8080 |
| MAX_WORKERS | 并发编译任务数 | 2 |
| MODULES_DIR | 预置模块目录 | ./modules |
| WORKDIR | 编译工作目录 | /tmp/nginx-build |
| BUILD_TIMEOUT | 编译超时时间 | 90m |

## 开发提示

- 本项目会自动生成编译脚本，便于线下复刻。
- 参考 `docker多段构建增加第三方模块.md` 可了解多段构建思路。
