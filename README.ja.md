# Centag

<p align="center">
  <strong>あなたの LLM プロキシハブ — パイプラインが戦略</strong><br/>
  汎用の大規模モデル代理ゲートウェイ。バックエンドの LLM プロバイダを一元管理し、API Key プールとカスタム代理戦略に対応。カスタマイズ可能なパイプラインとオープンなプラグインアーキテクチャでクライアント Agent の振る舞いを定義します。<br/>
  <em>中継局にもなれるが、中継局にとどまらない。</em>
</p>

<p align="center">
  <a href="https://github.com/atoml-ai/centag/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue" alt="License" /></a>
  <img src="https://img.shields.io/badge/go-1.25+-00ADD8?logo=go" alt="Go Version" />
  <a href="https://github.com/atoml-ai/centag/releases"><img src="https://img.shields.io/github/v/release/atoml-ai/centag" alt="Release" /></a>
  <a href="https://github.com/atoml-ai/centag/releases"><img src="https://img.shields.io/github/downloads/atoml-ai/centag/total" alt="Downloads" /></a>
</p>

<p align="center">
  <a href="README.md">English</a> | <a href="README.zh-CN.md">简体中文</a> | 日本語 | <a href="README.ko.md">한국어</a> | <a href="README.ru.md">Русский</a> | <a href="README.es.md">Español</a>
</p>

---

## 解決する課題

一般的な LLM「中継局」はリクエストをそのまま転送するだけです。Key が落ちたら手で差し替え、モデルが違えば再設定、Agent を増やすたびにまた設定——戦略は各ツールに散らばり、ゲートウェイ自体には戦略がありません。

**Centag は中継局ではなく、オーケストレーション可能なプロキシハブです。** バックエンドプール、フェイルオーバーとデグレード、シナリオルーティング、計量・課金を同一パイプラインに集約し、Agent 側はほぼ無感です。

| 能力 | 得られること |
|---|---|
| **バックエンド LLM プール管理** | OpenAI、Anthropic、智譜、Ollama、および任意の互換エンドポイントを一元管理；複数 Key・複数バックエンドを Web で一箇所設定 |
| **自動フェイルオーバー · マッチング · デグレード** | レート制限時に Key を自動ローテーション；障害時にバックエンド切替；モデル能力と負荷で最適出口をマッチ |
| **モデルルーティング** | 質問タイプに応じてバックエンドモデルをリアルタイム切替；同一セッション・同一タスク内でも動的に換模、クライアントの再設定不要 |
| **Agent シナリオ切替** | コーディング、Q&A などシナリオごとにパイプライン——シナリオ変更＝戦略変更、Agent は無感 |
| **Agent の迅速接続** | よく使う Agent は設定のワンクリック書き込み；`centag wrap` プロセスプロキシでゼロ変更接続；未対応は Web UI の設定ガイド。対応リストは継続拡大 |
| **System Prompt 戦略** | クライアント system prompt の透過・追加・置換に対応——Agent の人格を保ちつつ、シナリオごとに規範を重ねる／統一上書きも可能、パイプライン単位で柔軟に設定 |
| **計量と課金** | リクエスト・バックエンド・モデル単位で Token と費用を追跡 |
| **高性能・無損失アクセス** | 透過転送と SSE パススルー——プロトコル互換・低オーバーヘッド、上流セマンティクスを極力書き換えない |

---

## コアの強み

### ビジュアルパイプライン編成

中継局は転送だけ。**Centag はリクエストのライフサイクル全体を設計できる**——キャンバス上で DAG をドラッグ＆ドロップし、パイプラインが戦略そのものです。

**16 種の内蔵ノード**を自由に組み合わせ：

| ノード | Kind | 役割 |
|------|------|------|
| Generator | `llm.generate` | 任意の LLM バックエンドで生成 |
| Router | `route.decide` | 意図・キーワード・LLM 分類で分岐 |
| Scheduler | `scheduling.decide` | バックエンド横断のスマートスケジュールとマッチング |
| Transparent Forward | `proxy.transparent_forward` | 生 HTTP プロキシ（SSE 透過） |
| Aggregator | `aggregate.merge` | 並列生成のマージ / 投票 / 最良選択 |
| Reviewer | `quality.review` | 上流回答の採点・監査 |
| Memory | `memory.query` | クラウド記憶 / ローカルベクトルから文脈を想起 |
| Audit | `audit.safety` | コンテンツ審査と安全フィルタ |
| Token Usage | `metrics.token_usage` | Token 消費とコスト追跡 |
| Cache | `cache.access` | キャッシュ読み書き（厳密 / 意味 / ハイブリッド） |
| Processor | `content.transform` | コンテンツ変換と後処理 |
| Tool Call | `inject.tool_call` | Function Calling ツールを注入 |
| Prompt Ops | `prompt.ops` | ユーザー Prompt の前処理 |
| Output Post-ops | `prompt.postprocess` | 出力の後処理 |
| Loop Controller | — | 反復ワークフロー向けループ制御 |
| Plugin Node | *(リモート / 業務)* | HTTP または Go SDK のカスタムノード |

**パイプライン = 戦略。** シナリオ変更 → パイプライン変更 → Agent のコードは一行も変えない。

| シナリオ | パイプライン例 |
|------|-----------|
| コーディング助手 | ルーティング → コード特化モデル → コードレビュー |
| スマートスケジュール | 意図認識 → モデル能力マッチ → フェイルオーバー |
| 企業コンプライアンス | 安全審査 → 生成 → PII マスキング → 監査 |
| サポート / RAG | 記憶または検索想起 → 生成 → 品質レビュー |

### 統一バックエンドと Key プール

| 能力 | 説明 |
|------|------|
| **マルチバックエンド管理** | 主要ベンダーと OpenAI 互換エンドポイントを Web で一元管理 |
| **API Key プーリング** | バックエンドごとに複数 Key；制限や障害時に自動ローテーション |
| **自動フェイルオーバーとデグレード** | Key 失敗 → 次の Key；バックエンド障害 → 次のバックエンド |
| **スマートマッチング** | 重み・優先度・モデル能力で最適出口を選択 |
| **コスト追跡** | リクエスト・バックエンド・モデル単位で Token と費用を集計 |

### Agent の迅速接続 — 3 つの方法

業務コードを変えずに Agent を Centag へ接続。適応状況に応じて選択：

| 方法 | 向き | 説明 |
|------|------|------|
| **ワンクリック設定書き込み** | 既に対応済みの主要 Agent | Web UI で Base URL / API Key などを一括書き込み、すぐ利用 |
| **centag wrap プロセスプロキシ** | 設定を一切変えたくない場合 | プロセス級の透過プロキシ。Agent の設定・コードを変えずにトラフィックを Centag へ |
| **UI 設定ガイド** | まだワンクリック未対応の Agent | ゲートウェイへ手動で向ける手順をページ内で案内 |

主要 Agent の対応は継続拡大；未対応でもガイドまたは wrap で先に接続できます。

```bash
# Centag を起動
centag

# wrap の例——Agent 設定は変更しない
centag wrap run -- opencode

# 自己診断
centag wrap doctor
```

### オープンなプラグイン生態

パイプラインノードは拡張可能：Go SDK のローカルプラグイン、または任意言語のリモート HTTP プラグイン。

```go
type NodePlugin interface {
    Descriptor() NodePluginDescriptor
    ValidateConfig(config NodeConfig) error
    Execute(ctx context.Context, req *NodeExecutionRequest) (*NodeExecutionResponse, error)
}
```

リモートプラグインの規約：

```
GET  /.well-known/centag-node-plugin.json   →  自動検出
POST /validate                               →  設定検証
POST /execute                                →  ノード実行
```

---

## クイックスタート

```bash
# 1. インストール（いずれか）
curl -fsSL https://raw.githubusercontent.com/atoml-ai/centag/main/scripts/install.sh | bash
# または
npm install -g @atomlai/centag

# 2. 起動
centag

# 3. Web UI → http://localhost:20060 → 最初のバックエンドを追加

# 4. Agent 接続（ワンクリック設定、または wrap でゼロ変更）
centag wrap run -- opencode
```

完了。トラフィックは Centag 経由：共有バックエンドプール、自動フェイルオーバー、モデルルーティング、コスト可視化。

### その他のインストール方法

<details>
<summary>npm（グローバルパスを変更しない）</summary>

```bash
npx --yes @atomlai/centag
```
</details>

<details>
<summary>オフライン / 閉域網</summary>

```bash
npm install -g @atomlai/centag-offline
```
</details>

<details>
<summary>Docker（ソースから）</summary>

```bash
git clone https://github.com/atoml-ai/centag.git
cd centag
cp config/secrets/.env.example config/secrets/.env   # 必要に応じて編集
./start.sh docker up                                 # 既定: personal
```

管理画面：http://localhost:20060 · 停止：`./start.sh docker down`
</details>

---

## スクリーンショット

<p align="center">
  <strong>ダッシュボード</strong><br/>
  <img src="docs/assets/readme/screenshot-dashboard.png" alt="ダッシュボード" width="900" />
</p>

<p align="center">
  <strong>パイプラインビジュアルエディタ</strong><br/>
  <img src="docs/assets/readme/screenshot-pipeline-visual-editor.png" alt="パイプラインビジュアルエディタ" width="900" />
</p>

<p align="center">
  <strong>Agent 設定</strong><br/>
  <img src="docs/assets/readme/screenshot-agent-config.png" alt="Agent 設定" width="900" />
</p>

<p align="center">
  <strong>Token 利用量と課金</strong><br/>
  <img src="docs/assets/readme/screenshot-token-usage.png" alt="Token 利用量と課金" width="900" />
</p>

---

## プロキシモード — すぐ使える

シナリオ別パイプラインテンプレートを内蔵（`#` ショートカットで切替）：

| モード | ショートカット | 説明 |
|------|--------|------|
| スマートスケジュール | (既定) | モデル互換性とバックエンド負荷に基づく知能ルーティング |
| 透過プロキシ | `#t` | そのまま転送——高性能・無損失、system prompt を注入しない |
| 直結バックエンド | `#d` | 固定出口 + 管理された system prompt |
| フェイルオーバー | `#f` | バックエンド横断の自動デグレード |
| ルーティング | `#r` | 意図認識の多分岐ルーティング（シナリオ / モデル自動切替） |
| 監査 | `#a` | 生成 → 品質監査 → フィードバック |
| 最適化 | `#o` | 生成 → コンテンツ最適化 |
| 集約 | `#ag` | 並列マルチモデル生成 → 結果マージ |
| セキュリティファイアウォール | `#sec` | 安全審査 → 生成 → PII マスキング |
| RAG ゲートウェイ | `#rag` | キャッシュ優先の検索拡張生成 |
| 地理ルーティング | `#geo` | ルールベースのリージョンルーティング |
| Pi Agent | `#pi` | コードタスク → サンドボックス；Q&A → LLM |
| CI/CD Webhook | — | 外部システムからパイプラインを起動 |

真の魅力は**カスタムパイプライン**——キャンバス上で自分の DAG を設計することです。

---

## ドキュメント

| トピック | リンク |
|------|------|
| ドキュメント一覧 | [docs/README.md](docs/README.md) |
| パイプラインプラグイン標準 | [docs/guide/pipeline-plugin-standard.md](docs/guide/pipeline-plugin-standard.md) |
| Processor プラグインガイド | [docs/guide/processor-plugins.md](docs/guide/processor-plugins.md) |
| パイプライン変数リファレンス | [docs/guide/pipeline-variables.md](docs/guide/pipeline-variables.md) |
| プロキシモード | [docs/guide/proxy-modes.md](docs/guide/proxy-modes.md) |
| バックエンド設定 | [docs/guide/backend-configuration.md](docs/guide/backend-configuration.md) |
| 本機プロキシ / wrap | [docs/guide/system-proxy-egress.md](docs/guide/system-proxy-egress.md) |
| 環境変数 | [docs/guide/environment-variables.md](docs/guide/environment-variables.md) |
| API リファレンス | [docs/api/API_REFERENCE.md](docs/api/API_REFERENCE.md) |
| アーキテクチャ | [docs/architecture/](docs/architecture/) |
| セキュリティ | [docs/security/](docs/security/) |

---

## フィードバックとサポート

質問・提案は [GitHub Issues](https://github.com/atoml-ai/centag/issues)、または **centag@atoml.com** まで。

---

## コントリビュート

開発者の参加を歓迎します。バグ修正、機能追加、ドキュメント、Agent 対応の拡充など、[Pull Request](https://github.com/atoml-ai/centag/pulls) または [Issues](https://github.com/atoml-ai/centag/issues) で一緒に Centag を育てましょう。

---

## ライセンス

MIT License（オープンソース版：`minimal` / `personal`）
