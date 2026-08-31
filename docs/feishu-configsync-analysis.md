# Centag Feishu ConfigSync 系统分析

## 已完成的工作

### 1. Feishu 配置加载改进
- **文件**: `centag-pro/tools/pricescraper/providers/feishu.go`
- **改进**: 
  - 将原始 `os.Getenv()` 替换为 `configsync.LoadFeishuProviderConfig`
  - 支持旧环境变量名 (`CENTAG_CONFIGSYNC_WRITER_*`) 的向后兼容
  - 添加 `CENTAG_CONFIGSYNC_DIR` 支持

### 2. start.sh 修复
- **文件**: `centag-pro/start.sh`
- **改进**: 
  - 在 pricescraper 子 shell 前添加 `export CENTAG_CONFIGSYNC_DIR`
  - 兼容 bash 3.2

### 3. Feishu 权限配置
- 为 `caijun@atoml.com` 授予两个 Bitable base 的 full_access 权限
- 主 base: `CuE0bC6kRazW1lswb6Ccc3HEnyo`
- 遥测 base: `WRhdbSW9qa2LiLsSXN2chkZanwh`

### 4. Pricescraper 全面更新
更新了所有 5 个 provider 的价格数据：
- **DeepSeek**: V4 系列 (v4-flash/pro/vision-exp)
- **Zhipu**: GLM-5.3/5.2/5.1/5/4.7/4.6/4.5 (17 models)
- **Moonshot**: Kimi K3/K2.7 Code/K2.6
- **Anthropic**: Fable 5/Opus 5/Sonnet 5/Haiku 4.5
- **OpenAI**: GPT-5.6 Sol/Terra/Luna

### 5. 架构改进
- 添加 `LogOutput` (stderr) 日志支持
- 所有 scraper 记录 fallback 使用情况

## 系统架构分析

### 数据流概览

```
飞书 Bitable → ConfigSync Provider → Scheduler → Snapshot → OnUpdate → Billing System
     ↓                                    ↓
  centag_config 表                    持久化快照
  centag_model_price 表               (configsync-snapshot.json)
```

### 核心组件

#### 1. ConfigSync Provider
- **FeishuProvider** (Team Edition): 直接调用飞书 API
- **SnapshotProvider** (Personal Edition): 从公开 HTTP 端点获取快照

#### 2. ConfigScheduler
- 定时轮询 (默认 30 分钟)
- 初始抖动 (30-90 秒)
- 错误退避 (指数退避，最大 8 倍)
- 快照持久化

#### 3. Appliers
- **ApplyPrices**: 将远程价格同步到本地 billing RuleStore
- **ApplyVersions**: 处理版本更新信息
- **GenericApplier**: 存储功能开关

### 配置优先级

```
1. 显式配置 (opts.Explicit)
2. 环境变量 (opts.EnvPrefix + fieldName)
3. dotenv 文件 (opts.ConfigDir/.env)
4. 缺失配置错误
```

### 错误处理策略

1. **启动时失败**: 读取最后良好快照 (fail-open)
2. **同步时失败**: 保持最后良好快照，记录错误
3. **验证失败**: 拒绝整批数据，保持缓存
4. **空批次**: 计为成功，不覆盖缓存

## 关键代码路径

### 启动流程
```
Run() 
  → bootstrap.Seed() (首次启动初始化)
  → loadInitialBackends() (从 JSON 加载后端配置)
  → startConfigsync(srv) (启动配置同步)
```

### Team Edition 初始化
```go
// centag-pro/plugins/team/plugin.go
if proconfigsync.IsConfigured() {
    provider := proconfigsync.MustNewFeishuProvider()
    scheduler := configsync.NewScheduler(configsync.SchedulerConfig{
        Provider: provider,
        StateDir: os.Getenv("CENTAG_CONFIGSYNC_STATE_DIR"),
        Interval: 30 * time.Minute,
    })
    scheduler.Start(context.Background())
}
```

### Personal Edition 初始化
```go
// core/pkg/entrypoint/entrypoint_full.go
snapshotURL := os.Getenv("CENTAG_CONFIGSYNC_SNAPSHOT_URL")
if snapshotURL != "" {
    provider := configsync.NewSnapshotProvider([]string{snapshotURL})
    scheduler := configsync.NewScheduler(configsync.SchedulerConfig{
        Provider: provider,
        StateDir: stateDir,
        OnUpdate: func(snap *configsync.Snapshot) {
            configsync.ApplyPrices(...)
            srv.InvalidatePricingCache()
        },
    })
    scheduler.Start(context.Background())
}
```

### 价格应用流程
```go
// core/pkg/configsync/appliers.go
func ApplyPrices(...) {
    for _, p := range prices {
        if !p.Enabled { continue }
        backendIDs := mapBackend(NormalizeBaseURL(p.BaseURL))
        for _, backendID := range backendIDs {
            for _, m := range p.Models {
                upsertPriceRule(ctx, store, backendID, m, p.Currency, skipManual)
            }
        }
    }
}
```

## 安全与权限

### 凭证保护
- 飞书凭证存储在 gitignored 文件中 (`config/secrets/configsync/feishu.env`)
- 文件权限设置为 0600

### API 权限
- 飞书应用需要 base 级别的读写权限
- 支持 `full_access` 权限级别

### 数据验证
- 所有远程数据都经过验证 (`ValidateRows`, `ValidatePriceRow`)
- Schema 版本检查

## 性能优化

### 调度策略
- **初始抖动**: 避免启动时所有实例同时请求
- **指数退避**: 失败时减少请求频率
- **智能间隔**: 根据失败情况动态调整

### 数据处理
- **批量获取**: 一次获取所有配置
- **增量同步**: 只同步启用的记录
- **本地缓存**: 快照持久化到磁盘

### 资源管理
- **后台同步**: 不阻塞主服务启动
- **超时控制**: 所有操作都有超时限制
- **内存优化**: 流式处理大响应

## 监控与可观测性

### 状态端点
```go
// GET /configsync/status
{
    "last_sync_time": "2026-08-31T10:00:00Z",
    "last_sync_ok": true,
    "sync_count": 10,
    "error_count": 0,
    "last_error": ""
}
```

### 手动同步
```go
// POST /configsync/sync-now
// 立即触发一次同步
```

### 日志记录
- 同步成功/失败日志
- 价格应用统计
- 错误详情记录

## 故障排查指南

### 常见问题

1. **凭证错误**
   - 检查飞书应用权限
   - 验证 APP_ID/APP_SECRET

2. **网络问题**
   - 检查防火墙设置
   - 验证代理配置

3. **数据格式错误**
   - 检查飞书表格字段类型
   - 验证 JSON 格式

4. **权限不足**
   - 确保应用有 base 读写权限
   - 检查用户协作权限

5. **快照过期**
   - 检查快照 URL 可访问性
   - 验证网络连接

### 调试命令

```bash
# 检查配置
CENTAG_CONFIGSYNC=off ./centag  # 禁用同步

# 查看日志
grep "configsync" logs/centag.log

# 手动同步
curl -X POST http://localhost:8080/api/v1/configsync/sync-now

# 检查快照文件
cat $CENTAG_DATA_DIR/configsync-snapshot.json
```

## 未来改进方向

### 1. 增量同步
- 当前: 每次获取所有记录
- 改进: 基于 updated_at 的增量同步

### 2. 多源聚合
- 当前: 单一配置源
- 改进: 支持多个配置源的优先级聚合

### 3. 配置版本控制
- 当前: 无版本历史
- 改进: 保留配置变更历史

### 4. 实时推送
- 当前: 定时轮询
- 改进: 飞书 Webhook 实时推送

### 5. 配置模板
- 当前: 静态配置
- 支持: 基于环境的配置模板

## 总结

Centag 的 Feishu ConfigSync 系统是一个成熟、可靠的配置同步解决方案，具有以下特点：

1. **高可用性**: 失败时保持最后良好状态
2. **安全性**: 凭证保护和数据验证
3. **性能**: 智能调度和批量处理
4. **可观测性**: 完整的监控和日志
5. **可扩展性**: 支持多种配置源和提供者

该系统确保了所有 Centag 实例都能及时获得最新的模型价格和配置更新，同时保持系统的稳定性和可靠性。
