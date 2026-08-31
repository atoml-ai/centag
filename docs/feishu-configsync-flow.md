# Centag Feishu ConfigSync 数据流分析

## 概述

Centag 通过 configsync 系统从飞书多维表格（Bitable）同步配置到本地，包括后端模型价格、功能开关和版本信息。系统支持两种模式：

1. **Team Edition**：通过飞书 API 直接读取 Bitable
2. **Personal Edition**：通过公开快照 URL 读取配置

## 数据流架构

```
┌─────────────────────────────────────────────────────────────────┐
│                     飞书 Bitable (Feishu)                       │
│  ┌─────────────────────┐    ┌─────────────────────┐            │
│  │ centag_config 表    │    │ centag_model_price 表│            │
│  │ (配置/功能/版本)    │    │ (模型价格)          │            │
│  └──────────┬──────────┘    └──────────┬──────────┘            │
└─────────────┼──────────────────────────┼───────────────────────┘
              │                          │
              ▼                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                    ConfigSync Provider                          │
│  ┌─────────────────────┐    ┌─────────────────────┐            │
│  │ FeishuProvider      │    │ SnapshotProvider    │            │
│  │ (Team Edition)      │    │ (Personal Edition)  │            │
│  │ - API认证           │    │ - 公开HTTP          │            │
│  │ - 实时读取          │    │ - 静态快照          │            │
│  └──────────┬──────────┘    └──────────┬──────────┘            │
└─────────────┼──────────────────────────┼───────────────────────┘
              │                          │
              ▼                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                    ConfigScheduler                              │
│  - 定时轮询 (默认30分钟)                                       │
│  - 初始抖动 (30-90秒)                                          │
│  - 错误退避 (指数退避, 最大8倍)                                 │
│  - 快照持久化 (stateDir/configsync-snapshot.json)              │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Snapshot (内存 + 磁盘)                       │
│  {                                                             │
│    "schema": 1,                                                │
│    "generated_at": "2026-08-31T10:00:00Z",                     │
│    "config": [...],                                            │
│    "prices": [...]                                             │
│  }                                                             │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                    OnUpdate Callback                            │
│  1. ApplyPrices() - 价格规则写入数据库                          │
│  2. InvalidatePricingCache() - 清除计费缓存                    │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Billing System                               │
│  - PricingRule 表 (backend_id, model, input/output_price)      │
│  - 实时计费计算                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## 详细流程

### 1. 启动阶段 (Entry Point)

**文件**: `core/pkg/entrypoint/entrypoint_full.go`

```
Run() 
  → Step 6: bootstrap.Seed() (首次启动初始化)
  → Step 6.5: loadInitialBackends() (从JSON加载后端配置)
  → Step 7b: startConfigsync(srv) (启动配置同步)
```

### 2. ConfigSync 初始化

#### Team Edition (centag-pro/plugins/team/plugin.go)

```go
func (Plugin) Init(host extension.Host) error {
    // 1. 检查飞书凭证是否配置
    if proconfigsync.IsConfigured() {
        // 2. 创建飞书提供者
        provider := proconfigsync.MustNewFeishuProvider()
        
        // 3. 创建调度器
        scheduler := configsync.NewScheduler(configsync.SchedulerConfig{
            Provider: provider,
            StateDir: os.Getenv("CENTAG_CONFIGSYNC_STATE_DIR"),
            Interval: 30 * time.Minute,
        })
        
        // 4. 启动调度器
        scheduler.Start(context.Background())
        
        // 5. 注册管理端点
        h := proconfigsync.NewAdminHandler(provider, scheduler)
        h.RegisterRoutes(rg)
    }
}
```

#### Personal Edition (core/pkg/entrypoint/entrypoint_full.go)

```go
func startConfigsync(srv *server.Server) {
    // 1. 检查快照URL
    snapshotURL := os.Getenv("CENTAG_CONFIGSYNC_SNAPSHOT_URL")
    if snapshotURL != "" {
        // 2. 创建快照提供者
        provider := configsync.NewSnapshotProvider([]string{snapshotURL})
        
        // 3. 创建调度器
        scheduler := configsync.NewScheduler(configsync.SchedulerConfig{
            Provider: provider,
            StateDir: stateDir,
            OnUpdate: func(snap *configsync.Snapshot) {
                // 4. 应用价格到数据库
                result, err := configsync.ApplyPrices(
                    context.Background(), snap.Prices, mapper, priceStore, true,
                )
                // 5. 清除计费缓存
                srv.InvalidatePricingCache()
            },
        })
        
        // 6. 启动调度器
        scheduler.Start(context.Background())
    }
}
```

### 3. 数据获取

#### FeishuProvider (centag-pro/internal/configsync/feishu_provider.go)

```go
func (p *FeishuProvider) FetchConfig(ctx context.Context, q configsync.Query) ([]configsync.Row, error) {
    // 1. 从飞书API搜索记录
    records, err := p.client.SearchRecords(ctx, p.appToken, p.configTableID, 
        feishu.NewFilter("enabled", "is", "true"))
    
    // 2. 转换为内部格式
    var rows []configsync.Row
    for _, rec := range records {
        row := feishuRecordToConfigRow(rec)
        if row != nil {
            rows = append(rows, *row)
        }
    }
    return rows, nil
}

func (p *FeishuProvider) FetchModelPrices(ctx context.Context) ([]configsync.ProviderPrice, error) {
    // 1. 从飞书API搜索价格记录
    records, err := p.client.SearchRecords(ctx, p.appToken, p.priceTableID, 
        feishu.NewFilter("enabled", "is", "true"))
    
    // 2. 转换为内部格式
    var prices []configsync.ProviderPrice
    for _, rec := range records {
        pp := feishuRecordToProviderPrice(rec)
        if pp != nil {
            prices = append(prices, *pp)
        }
    }
    return prices, nil
}
```

#### SnapshotProvider (core/pkg/configsync/snapshot_provider.go)

```go
func (p *SnapshotProvider) FetchAll(ctx context.Context, q Query) ([]Row, []ProviderPrice, error) {
    // 1. 从HTTP获取快照
    snap, err := p.fetchSnapshot(ctx)
    if err != nil {
        return nil, nil, err
    }
    return snap.Config, snap.Prices, nil
}

func (p *SnapshotProvider) fetchOne(ctx context.Context, url string) (*Snapshot, error) {
    // 1. 发送HTTP请求
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    resp, err := p.httpc.Do(req)
    
    // 2. 读取响应体
    body, err := io.ReadAll(io.LimitReader(resp.Body, maxSnapshotBody))
    
    // 3. 解析JSON
    var snap Snapshot
    if err := json.Unmarshal(body, &snap); err != nil {
        return nil, fmt.Errorf("snapshot decode %s: %w", url, err)
    }
    
    // 4. 验证schema版本
    if snap.Schema != snapshotSchema {
        return nil, fmt.Errorf("snapshot schema %d (expected %d)", snap.Schema, snapshotSchema)
    }
    return &snap, nil
}
```

### 4. 调度器逻辑

**文件**: `core/pkg/configsync/scheduler.go`

```go
func (s *ConfigScheduler) run(ctx context.Context) {
    // 1. 初始抖动 (30-90秒)
    timer := time.NewTimer(initialJitter())
    select {
    case <-timer.C:
    }
    
    // 2. 首次同步
    _ = s.SyncNow(ctx)
    
    // 3. 循环同步
    for {
        timer.Reset(s.nextDelay())
        select {
        case <-timer.C:
            _ = s.SyncNow(ctx)
        }
    }
}

func (s *ConfigScheduler) doSync(ctx context.Context) error {
    // 1. 获取数据
    if fa, ok := s.provider.(fetchAller); ok {
        rows, prices, err = fa.FetchAll(ctx, Query{})
    } else {
        rows, err = s.provider.FetchConfig(ctx, Query{})
        prices, err = s.provider.FetchModelPrices(ctx)
    }
    
    // 2. 验证数据
    if err := ValidateRows(rows); err != nil {
        return err
    }
    for i := range prices {
        if err := ValidatePriceRow(&prices[i]); err != nil {
            return err
        }
    }
    
    // 3. 空批次处理
    if len(rows) == 0 && len(prices) == 0 {
        // 保持缓存，计为成功
        return nil
    }
    
    // 4. 创建快照
    snap := &Snapshot{Schema: snapshotSchema, GeneratedAt: time.Now(), Config: rows, Prices: prices}
    
    // 5. 持久化快照
    if s.stateDir != "" {
        WriteSnapshot(s.stateDir, snap)
    }
    
    // 6. 更新内存快照
    s.snap = snap
    
    // 7. 触发回调
    if s.onUpdate != nil {
        s.onUpdate(snap)
    }
    return nil
}
```

### 5. 价格应用

**文件**: `core/pkg/configsync/appliers.go`

```go
func ApplyPrices(ctx context.Context, prices []ProviderPrice, mapBackend BackendMapper, store PriceStore, skipManual bool) (*PriceApplierSyncResult, error) {
    result := &PriceApplierSyncResult{}
    for _, p := range prices {
        if !p.Enabled {
            continue // 跳过禁用的提供者
        }
        
        // 1. 映射base_url到本地后端ID
        backendIDs := mapBackend(NormalizeBaseURL(p.BaseURL))
        if len(backendIDs) == 0 {
            continue // 无匹配后端
        }
        
        // 2. 为每个后端和模型创建/更新价格规则
        for _, backendID := range backendIDs {
            for _, m := range p.Models {
                skip, err := upsertPriceRule(ctx, store, backendID, m, p.Currency, skipManual)
                if skip {
                    result.Skipped++
                } else {
                    result.Applied++
                }
            }
        }
    }
    return result, nil
}

func upsertPriceRule(ctx context.Context, store PriceStore, backendID string, m ModelPrice, currency string, skipManual bool) (skipped bool, err error) {
    // 1. 查询现有规则
    existing, err := store.GetRuleByModelAndType(ctx, backendID, m.Model, billing.PriceTypeCost)
    
    // 2. 跳过手动规则
    if existing != nil && existing.Source == "manual" && skipManual {
        return true, nil
    }
    
    // 3. 创建或更新规则
    rule := &billing.PricingRule{
        Name:            fmt.Sprintf("%s/%s", backendID, m.Model),
        BackendID:       backendID,
        Model:           m.Model,
        PriceType:       billing.PriceTypeCost,
        InputPricePerM:  m.InputPricePerM,
        OutputPricePerM: m.OutputPricePerM,
        Currency:        currency,
        Priority:        10,
        Enabled:         true,
        Source:          "config",
    }
    
    if existing != nil {
        return false, store.UpdateRule(ctx, existing.ID, rule)
    }
    return false, store.CreateRule(ctx, rule)
}
```

## 配置优先级

### 通道配置加载 (core/pkg/configsync/channel.go)

```
1. 显式配置 (opts.Explicit)
2. 环境变量 (opts.EnvPrefix + fieldName)
3. dotenv文件 (opts.ConfigDir/.env)
4. 缺失配置错误
```

### 飞书提供者配置 (core/pkg/configsync/channel_feishu.go)

```go
type FeishuProviderConfig struct {
    AppID        string
    AppSecret    string
    AppToken     string
    TableID      string
    PriceTableID string
}
```

环境变量:
- `CENTAG_CONFIGSYNC_FEISHU_APP_ID`
- `CENTAG_CONFIGSYNC_FEISHU_APP_SECRET`
- `CENTAG_CONFIGSYNC_FEISHU_APP_TOKEN`
- `CENTAG_CONFIGSYNC_FEISHU_TABLE_ID`
- `CENTAG_CONFIGSYNC_FEISHU_PRICE_TABLE_ID`

### 配置目录 (centag-pro/tools/pricescraper/providers/feishu.go)

```go
func defaultConfigDir() string {
    // 优先级:
    // 1. CENTAG_CONFIGSYNC_DIR 环境变量
    // 2. config/secrets/configsync (相对于可执行文件)
    return filepath.Join("config", "secrets", "configsync")
}
```

## 数据结构

### Row (配置行)

```go
type Row struct {
    Edition    string          `json:"edition"`
    Key        string          `json:"key"`
    Channel    string          `json:"channel"`
    MinVersion string          `json:"min_version"`
    MaxVersion string          `json:"max_version"`
    Priority   int             `json:"priority"`
    Value      json.RawMessage `json:"value"`
    Enabled    bool            `json:"enabled"`
    UpdatedAt  time.Time       `json:"updated_at"`
    Remark     string          `json:"remark"`
}
```

### ProviderPrice (提供者价格)

```go
type ProviderPrice struct {
    BaseURL      string       `json:"base_url"`
    ProviderName string       `json:"provider_name"`
    Currency     string       `json:"currency"`
    Models       []ModelPrice `json:"models"`
    Enabled      bool         `json:"enabled"`
}

type ModelPrice struct {
    Model           string  `json:"model"`
    InputPricePerM  float64 `json:"input_price_per_m"`
    OutputPricePerM float64 `json:"output_price_per_m"`
}
```

### Snapshot (快照)

```go
type Snapshot struct {
    Schema      int             `json:"schema"`
    GeneratedAt time.Time       `json:"generated_at"`
    Config      []Row           `json:"config"`
    Prices      []ProviderPrice `json:"prices,omitempty"`
}
```

## 错误处理

### 失败策略

1. **启动时失败**: 读取最后良好快照 (fail-open)
2. **同步时失败**: 保持最后良好快照，记录错误
3. **验证失败**: 拒绝整批数据，保持缓存
4. **空批次**: 计为成功，不覆盖缓存

### 重试机制

- **初始抖动**: 30-90秒随机延迟
- **指数退避**: 失败次数 * 基础间隔 (最大8倍)
- **速率限制**: 429响应时使用服务器请求的重试时间

## 监控与管理

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

## 安全考虑

1. **凭证保护**: 飞书凭证存储在gitignored文件中
2. **权限控制**: API应用需要base级别权限
3. **数据验证**: 所有远程数据都经过验证
4. **速率限制**: 尊重服务器速率限制
5. **审计日志**: 同步操作记录日志

## 性能优化

1. **增量同步**: 只同步启用的记录
2. **批量处理**: 一次获取所有配置
3. **本地缓存**: 快照持久化到磁盘
4. **后台同步**: 不阻塞主服务启动
5. **智能调度**: 根据失败情况调整间隔

## 故障排查

### 常见问题

1. **凭证错误**: 检查飞书应用权限
2. **网络问题**: 检查防火墙和代理设置
3. **数据格式错误**: 检查飞书表格字段类型
4. **权限不足**: 确保应用有base读写权限
5. **快照过期**: 检查快照URL是否可访问

### 调试命令

```bash
# 检查配置
CENTAG_CONFIGSYNC=off ./centag  # 禁用同步

# 查看日志
grep "configsync" logs/centag.log

# 手动同步
curl -X POST http://localhost:8080/api/v1/configsync/sync-now
```

## 总结

Centag 的 Feishu ConfigSync 系统实现了:

1. **实时配置同步**: 从飞书Bitable获取最新配置
2. **多模式支持**: Team Edition (API) 和 Personal Edition (快照)
3. **高可用性**: 失败时保持最后良好状态
4. **性能优化**: 智能调度和批量处理
5. **安全可靠**: 凭证保护和数据验证

这个系统确保了所有Centag实例都能及时获得最新的模型价格和配置更新，同时保持系统的稳定性和可靠性。
