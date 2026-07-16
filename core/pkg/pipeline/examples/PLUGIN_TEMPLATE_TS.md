# TypeScript/Node.js 插件开发模板

本模板演示如何为 Centag 流水线开发一个标准的 TypeScript/Node.js 节点插件。

## 目录结构

```
my-plugin/
├── package.json
├── tsconfig.json        # TypeScript 项目
└── src/
    ├── index.ts          # 插件主文件
    └── register.ts      # 注册逻辑
```

## package.json

```json
{
  "name": "centag-plugin-my-plugin",
  "version": "1.0.0",
  "description": "My custom node plugin",
  "main": "dist/index.js",
  "scripts": {
    "build": "tsc",
    "start": "node dist/index.js",
    "validate": "curl -X POST <http://localhost:3000/validate> -H 'Content-Type: application/json' -d @validate_req.json"
  },
  "dependencies": {
    "express": "^4.18.0"
  },
  "devDependencies": {
    "@types/express": "^4.17.0",
    "typescript": "^5.0.0"
  }
}
```

## tsconfig.json

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "module": "commonjs",
    "outDir": "./dist",
    "rootDir": "./src",
    "strict": true,
    "esModuleInterop": true
  }
}
```

## src/index.ts

```typescript
import express, { Request, Response } from 'express';

const app = express();
app.use(express.json());

// 插件描述符
const descriptor = {
  name: 'My Plugin',
  implementation: 'example.my-plugin',
  kind: 'custom.my-category',
  version: '1.0.0',
  description: '插件功能描述',
  config_schema: {
    type: 'object',
    properties: {
      my_param: {
        type: 'string',
        description: '参数说明',
        default: 'default_value',
      },
    },
  },
  input_schema: {
    type: 'object',
    properties: {
      content: {
        type: 'string',
        description: '输入内容',
      },
    },
    required: ['content'],
  },
  output_schema: {
    type: 'object',
    properties: {
      content: {
        type: 'string',
        description: '输出内容',
      },
    },
  },
  permissions: ['llm.call'],
  supports_stream: false,
  api_version: 'centag.pipeline.node/v1alpha1',
};

// 健康检查
app.get('/.well-known/centag-node-plugin.json', (req: Request, res: Response) => {
  res.json(descriptor);
});

// 配置校验
app.post('/validate', (req: Request, res: Response) => {
  const config = req.body.config;
  // 校验配置逻辑
  res.json({ valid: true });
});

// 执行
app.post('/execute', (req: Request, res: Response) => {
  const request = req.body;
  const input = request.input?.content || '';

  // 业务逻辑
  const output = `Processed: ${input}`;

  res.json({
    output: {
      content: output,
      metadata: {
        processed: true,
      },
    },
  });
});

// 可选：流式执行
app.post('/stream', (req: Request, res: Response) => {
  const request = req.body;
  res.setHeader('Content-Type', 'text/event-stream');
  res.setHeader('Cache-Control', 'no-cache');
  res.setHeader('Connection', 'keep-alive');

  // SSE 流式输出
  res.write(`data: ${JSON.stringify({ content: 'chunk1' })}\n\n`);
  res.write(`data: ${JSON.stringify({ content: 'chunk2' })}\n\n`);
  res.write('data: [DONE]\n\n');
  res.end();
});

const PORT = process.env.PORT || 3000;
app.listen(PORT, () => {
  console.log(`Plugin server listening on port ${PORT}`);
});
```

## 使用方式

1. 安装依赖：`npm install`
2. 编译 TypeScript：`npm run build`
3. 启动插件服务：`npm start`
4. 在 Centag 中配置为远程插件：将 `implementation` 设置为 `http://localhost:3000`

## 权限说明

在 `descriptor` 中声明插件需要的权限，例如：

- `llm.call` — 调用 LLM 服务
- `storage.read` / `storage.write` — 读写存储
- `memory.read` / `memory.write` — 读写记忆
- `network.outbound` — 发起外部 HTTP 请求
- `secrets.read` — 读取密钥

## 配置 Schema

使用 [JSON Schema](https://json-schema.org/) 定义 `config_schema`，Centag WebUI 会根据该 Schema 自动生成配置表单。

## 输入/输出 Schema

定义 `input_schema` 和 `output_schema`，便于流水线编排和类型检查。
