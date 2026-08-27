# AGENTS.md — Viva小铺 API / fullstack 定制项目

> 本仓库负责 Viva小铺后端 API、fullstack 前后台资源嵌入、生产构建和 Deploy Production 工作流。

---

## 仓库信息

```text
路径：/Users/lutao/vibeCodingProjects/viva/viva-faka-custom
GitHub：https://github.com/LTXWorld/viva-faka-custom
上游：https://github.com/dujiao-next/dujiao-next
生产分支：viva-custom
```

`main` 用于跟随上游，`viva-custom` 是 Viva 生产定制分支。不要为了消除 GitHub 的 ahead/behind 提示随意把生产定制合并回 `main`。

---

## 作用范围

本仓库负责：

```text
后端 API 和服务端业务逻辑
充值供应商、充值任务和卡密批次绑定逻辑
fullstack Go 二进制构建
嵌入 viva-faka-user/dist 和 viva-faka-admin/dist
GitHub Actions Deploy Production
生产备份脚本及 systemd 单元的版本化源码
```

不在本仓库直接修改：

```text
前台页面/UI：去 viva-faka-user:viva-custom
后台页面/UI：去 viva-faka-admin:viva-custom
new-api / CPA：属于独立运维链路
```

---

## 当前生产基线

2026-08-27 核验结果：

```text
站点：https://faka.bfsmlt.com
服务器：LTGer
服务：dujiao-next
运行目录：/srv/faka
线上二进制：/srv/faka/app/dujiao-server
监听：宿主机 8080
反代：Docker imagecreate-caddy-1 → host.docker.internal:8080
部署时间：2026-06-18 19:53（Asia/Shanghai）
线上嵌入版本：viva-626df0f9bce1a25619a395403117964af5a28ddb
API/fullstack commit：626df0f9bce1a25619a395403117964af5a28ddb
user commit：5f2d22214740bae9aa4563600e5684aaddc8816e
admin commit：1774ce305d2a950f44b1319d54fea6a85ca29be0
```

服务器上的 `/srv/faka/src` 是旧官方源码 checkout，不是生产构建来源，不得在该目录直接开发或部署。

---

## 已完成的 Viva 定制

```text
游客购买邮箱/密码提示源码化
关闭/调整部分官方导航和 Viva 品牌文案
密码找回流程恢复
ChatGPT Plus 兑换和直充
Gemini 兑换和直充
统一兑换入口
充值供应商抽象
充值任务数据库记录和后台 API
卡密批次绑定供应商元数据
订单交付页兑换按钮
自动 Deploy Production
```

旧文档中的“仍运行官方 release”“游客字段提示仍依赖 site_config.scripts”“初期只手动部署”等描述均已失效。

---

## 生产数据与敏感路径

```text
生产配置：/srv/faka/app/config.yml
SQLite：/srv/faka/data/db/dujiao.db
日志：/srv/faka/data/logs/
实际上传目录：/srv/faka/app/uploads/
兼容上传目录：/srv/faka/data/uploads/
发布二进制：/srv/faka/releases/
旧二进制备份：/srv/faka/backup/dujiao-server-before-gha-*
数据备份：/srv/faka/backup/data/
```

部署不得覆盖生产配置、数据库、日志和 uploads。正常部署只替换 `/srv/faka/app/dujiao-server`。

---

## 生产备份

服务器已安装：

```text
脚本：/usr/local/bin/faka-backup.sh
定时器：faka-backup.timer
执行时间：每天 03:30 Asia/Shanghai，随机延迟最多 5 分钟
保留时间：默认 14 天
latest：/srv/faka/backup/data/latest
```

版本化源码：

```text
scripts/production/faka-backup.sh
scripts/production/faka-backup.service
scripts/production/faka-backup.timer
```

备份内容：

```text
使用 Python sqlite3 backup API 生成的一致性 dujiao.db
生产 config.yml
/srv/faka/app/uploads 和 /srv/faka/data/uploads
/srv/faka/app/logs 和 /srv/faka/data/logs
manifest.txt
SHA256SUMS
```

脚本会对备份数据库执行 `PRAGMA quick_check`，并验证所有备份文件校验和。Deploy Production 中备份失败必须直接终止部署，禁止使用 `|| true` 忽略备份错误。

常用检查：

```bash
ssh LTGer "/usr/local/bin/faka-backup.sh"
ssh LTGer "systemctl status faka-backup.timer --no-pager"
ssh LTGer "readlink -f /srv/faka/backup/data/latest"
```

---

## 构建与检查

```bash
cd /Users/lutao/vibeCodingProjects/viva/viva-faka-custom
git checkout viva-custom
go test ./...
```

推荐在 pi 中运行：

```text
/viva-check api
/viva-check push
/viva-predeploy
```

---

## 部署规则

生产入口：

```text
GitHub → LTXWorld/viva-faka-custom → Actions → Deploy Production
```

触发方式：

```text
push 到 viva-faka-custom:viva-custom：自动部署生产
workflow_dispatch：手动补救或指定 user_ref/admin_ref
```

自动构建会拉取：

```text
LTXWorld/viva-faka-user:viva-custom
LTXWorld/viva-faka-admin:viva-custom
```

因此前台/后台修改必须先推送各自 `viva-custom`。推送本仓库 `viva-custom` 前必须完成测试、敏感信息检查，并确认确实准备上线。

部署后至少检查：

```bash
ssh LTGer "systemctl is-active dujiao-next redis-server"
curl -I https://faka.bfsmlt.com/
```

并人工检查首页、商品详情、游客下单、支付回跳、订单详情、卡密显示、兑换入口和后台登录。

---

## 禁止提交

```text
KPay EPay Key
充值供应商 API Key
后台密码
服务器 SSH 私钥
Cloudflare Token
/srv/faka/app/config.yml
本地 config.yml
数据库和用户上传数据
```

---

*最后更新：2026-08-27*
