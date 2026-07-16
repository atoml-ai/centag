# 测试脚本目录

本目录包含项目各个功能模块的测试脚本，按功能分类组织。

## 目录结构

```
test/
├── README.md                          # 本文件
├── cache/                             # 缓存测试脚本
│   ├── test-cache-strategies.sh      # 缓存策略完整测试
│   ├── simple-cache-test.sh          # 简单缓存测试
│   ├── test_stream_semantic_cache.sh # 流式语义缓存测试
│   └── README_TEST_STRATEGIES.md     # 测试说明
├── proxy/                             # 代理测试脚本
│   └── test-proxy.sh
├── processor/                          # 处理器测试脚本
│   ├── test-question-processor.sh     # 问题处理器测试
│   ├── test-processor-quick.sh       # 快速处理器测试
│   └── test-question-split-e2e.sh    # 问题拆分端到端测试
├── model/                             # 模型测试脚本
│   ├── test-multi-model.sh           # 多模型测试
│   ├── test-ollama.sh                # Ollama 测试
│   └── test-model-extension.sh       # 模型扩展测试
├── storage/                           # 存储测试脚本
│   ├── test-storage.sh                # 存储测试
│   └── verify-chromadb.sh            # ChromaDB 验证
└── daemon/                            # 守护进程测试脚本
    └── test-daemon-logs.sh
```

## 快速开始

```bash
# 运行缓存策略测试
bash scripts/test/cache/test-cache-strategies.sh

# 运行单个模块测试
bash test/processor/test-processor-quick.sh
```

### 按功能模块测试

#### 缓存测试
```bash
# 流式语义缓存完整测试
./cache/test_stream_semantic_cache.sh

# 简单缓存测试
./cache/simple-cache-test.sh
```

#### 代理测试
```bash
# 代理功能测试
./proxy/test-proxy.sh
```

#### 处理器测试
```bash
# 问题处理器测试
./processor/test-question-processor.sh

# 处理器快速测试
./processor/test-processor-quick.sh

# 问题分割E2E测试
./processor/test-question-split-e2e.sh
```

#### 模型测试
```bash
# 多模型测试
./model/test-multi-model.sh

# Ollama后端测试
./model/test-ollama.sh

# 模型扩展测试
./model/test-model-extension.sh
```

#### 存储测试
```bash
# 存储功能测试
./storage/test-storage.sh

# ChromaDB验证
./storage/verify-chromadb.sh
```

#### 守护进程测试
```bash
# 日志功能测试
./daemon/test-daemon-logs.sh
```

## 测试脚本说明

### cache/test_stream_semantic_cache.sh
**功能**: 完整的流式语义缓存测试脚本

**测试覆盖**:
- ✅ 生成缓存测试
- ✅ 相似问题命中测试(相似度≥0.85)
- ✅ 不相关问题未命中测试(相似度<0.85)
- ✅ 响应内容验证测试
- ✅ 精确匹配优先级测试

**使用方法**:
```bash
./cache/test_stream_semantic_cache.sh
```

**输出**:
- 彩色测试结果
- 相似度分数
- 缓存命中率
- 详细日志

### test-e2e.sh
**功能**: 端到端测试脚本

**使用方法**:
```bash
./test-e2e.sh
```

## 测试环境要求

### 必需
- 服务已启动: `./start.sh`
- curl命令可用
- bash shell

### 可选
- jq (JSON处理工具)
- bc (计算工具)

## 测试结果解读

### 缓存测试
- **HIT**: 缓存命中
- **MISS**: 缓存未命中
- **HIT-EXACT**: 精确匹配命中
- **HIT-SEMANTIC**: 语义匹配命中

### 相似度
- **0.85以上**: 高度相似,应该HIT
- **0.7-0.85**: 中度相似,根据阈值决定
- **0.7以下**: 低度相似,应该MISS

## 日志查看

### 实时查看测试日志
```bash
# 查看服务日志
tail -f bin/logs/centag.log

# 查看语义缓存相关日志
tail -f bin/logs/centag.log | grep -E 'semantic|similarity'

# 查看缓存命中日志
tail -f bin/logs/centag.log | grep -E 'HIT|MISS'
```

## 故障排查

### 问题1: 测试脚本执行失败
**可能原因**:
- 服务未启动
- 端口被占用
- 权限不足

**解决方法**:
```bash
# 重启服务
./start.sh restart

# 检查端口
lsof -i :20060

# 添加执行权限
chmod +x test/**/*.sh
```

### 问题2: 所有测试都失败
**可能原因**:
- 配置文件错误
- 依赖服务未启动(如ElasticSearch、Redis)

**解决方法**:
```bash
# 检查配置
cat bin/configs/config.yaml

# 检查依赖服务
curl http://localhost:9200/_cluster/health  # ElasticSearch
redis-cli ping                              # Redis
```

### 问题3: 语义缓存测试失败
**可能原因**:
- Embedding服务未启动
- 向量存储没有数据
- 相似度阈值设置不当

**解决方法**:
```bash
# 降低相似度阈值
curl -X POST http://localhost:20060/api/v1/cache/semantic/threshold \
  -d '{"value": 0.7}'

# 清空缓存重新测试
curl -X POST http://localhost:20060/api/v1/cache/clear
```

## 最佳实践

1. **测试前检查**: 运行测试前确保服务状态正常
2. **逐个测试**: 先运行单独的功能测试,再运行E2E测试
3. **查看日志**: 测试失败时查看详细日志
4. **记录结果**: 记录测试结果用于对比和回归测试
5. **定期测试**: 代码修改后重新运行测试验证

## 持续集成

### CI/CD集成
可以将测试脚本集成到CI/CD流水线:

```yaml
test:
  stage: test
  script:
    - ./start.sh
    - sleep 10
    - ./test-e2e.sh
    - ./cache/test_stream_semantic_cache.sh
```

### 自动化测试报告
```bash
# 运行所有测试并生成报告
./run-all-tests.sh > test-report-$(date +%Y%m%d).log
```

## 相关文档

- [测试指南](../docs/guide/Testing-Guide.md)
- [缓存使用指南](../docs/cache/Cache-Guide.md)
- [故障排除](../docs/guide/Chat-Stream-Fix.md)

## 贡献指南

如需添加新测试:
1. 根据功能选择合适的子目录
2. 使用bash脚本编写测试
3. 添加详细的注释和使用说明
4. 在本README中添加说明
5. 测试通过后提交代码
