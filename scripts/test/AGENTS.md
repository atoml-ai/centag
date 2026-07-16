# test/ — 测试目录

> 面向 Agent：本目录包含集成测试和场景测试脚本。

## 目录职责

存放需要外部依赖或特定环境的测试，单元测试在各包内的 `*_test.go`。

## 子目录

| 目录 | 用途 |
|------|------|
| `cache/` | 缓存测试 |
| `daemon/` | 守护进程测试 |
| `model/` | 模型测试 |
| `processor/` | 处理器测试 |
| `proxy/` | 代理测试 |
| `storage/` | 存储测试 |

## 测试类型

| 类型 | 位置 | 运行方式 |
|------|------|----------|
| 单元测试 | `internal/*_test.go` | `make test` |
| 集成测试 | `test/` | 按目录运行 |
| 场景测试 | `scripts/test/e2e/` | 手动运行 |

## 约束

- ❌ **禁止**测试依赖生产环境
- ❌ **禁止**测试修改生产数据
- ✅ **必须**：测试可独立运行
- ✅ **必须**：测试后清理临时数据

## 运行测试

```bash
# 运行所有单元测试
make test

# 运行特定包测试
go test ./internal/cache/...

# 运行集成测试
cd test/cache && go test -v
```

## 相关文档

- 测试指南：`docs/development/testing.md`
- Makefile：`../Makefile`

---

*最后更新：2026-04-27*
