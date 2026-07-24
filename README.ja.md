# Centag

[English](README.md) | [简体中文](README.zh-CN.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | [Русский](README.ru.md) | [Español](README.es.md)

コーディング Agent を**本機ワンクリックでプロキシ接続**し、バックエンドと API Key を**一元管理**。シナリオごとに**プロキシ動作を設定**（切替・フェイルオーバー・パイプライン）——ツールごとに何度も設定し直す必要はありません。

個人開発者向け：Centag を入れる → wrap または設定ファイルで Agent を接続 → Web でバックエンドとポリシーを管理。

## インストール

いずれかの方法で導入。インストール後は `centag` を実行し、**http://localhost:20060** を開きます。

### 方法 1：ワンライナー（推奨、Node.js 不要）

```bash
curl -fsSL https://raw.githubusercontent.com/atoml-ai/centag/main/scripts/install.sh | bash
```

既定で `~/.centag/` に入り、PATH への追加を試みます。その後 `centag` / `centag wrap` が使えます。

### 方法 2：npm（すでに Node.js がある場合）

```bash
# グローバルインストール（オンライン版。Release からバイナリを取得）
npm install -g @atomlai/centag

# グローバルを汚さず試す
npx --yes @atomlai/centag

# オフライン / 閉域網向け
npm install -g @atomlai/centag-offline
```

`npm install -g` で権限エラーになる場合は `npx` か上記スクリプトを使ってください。詳細：[apps/centag-npm/README.md](apps/centag-npm/README.md)。

### 方法 3：Docker（ソースから）

```bash
git clone https://github.com/atoml-ai/centag.git
cd centag
cp config/secrets/.env.example config/secrets/.env   # 必要に応じて編集
./start.sh docker up                                 # 既定: personal コンテナ
```

管理画面は同じく http://localhost:20060。停止：`./start.sh docker down`。

---

## 導入後：Agent の接続

目標：Agent はこれまで通り使い、トラフィックだけ Centag 経由（バックエンド共有・フェイルオーバー・計量）。

1. **Web を開く** → バックエンドを少なくとも 1 つ追加して有効化（API Key / ローカル互換エンドポイント）。
2. **Agent 連携**（Web メニュー）— ウィザードで主要ツール向け設定を生成/書き込み。または
3. **プロセスプロキシ（推奨・Agent 設定をほぼ変えない）**：

```bash
# 本機で Centag 起動済みなら wrap で Agent を起動
centag wrap run -- opencode
# opencode を自分の Agent 起動コマンドに置き換え

# 自己診断
centag wrap doctor
```

補足：`centag wrap` はゲートウェイを**起動しません**。稼働中の Centag に Agent プロセスの通信を流すだけです。詳細：[本機プロキシ出口](docs/guide/system-proxy-egress.md)。

---

## なぜ Centag か

| 欲しいこと | Centag のやり方 |
|------------|-----------------|
| **バックエンドを素早く切替** | 複数バックエンドを一元管理；Web で有効化/切替。Agent 側を何度も直さない |
| **自動フェイルオーバー + API プール** | 複数 Key をローテーション；制限や障害時に自動で付け替え |
| **シナリオ別パイプライン** | 透過転送・直結・スケジューリング・レビューなど設定可能；シナリオ変更＝ポリシー変更 |
| **利用量・課金の可視化** | Token / 費用を追跡し、個人利用を把握 |

一言で：**バックエンドとポリシーは一つの入口、Agent はコードを書くだけ。**

## 機能一覧

1. **バックエンド / モデルと API Key プール**  
   Web でバックエンドとモデルを設定。同一バックエンドで**複数 API Key をプールしてローテーション**（制限・障害時に自動切替）。

2. **パイプラインのビジュアル編集**  
   キャンバスでプロキシ動作をカスタム（転送・スケジュール・レビューなど）。シナリオごとにポリシー切替、Agent コード変更不要。

3. **`centag wrap` による非侵襲接続**  
   wrap で Agent を起動し Centag にトラフィックを取り込み、**Agent 本体の設定を変えなくてよい**。

4. **Agent 設定ファイルを直接編集して接続**  
   Agent の API Base / Key を Centag に向け、通常の LLM ゲートウェイとして利用（Web「Agent 連携」ウィザードが書き込みを支援）。

どちらか好きな方を選んでください：wrap は設定変更が少ない；設定ファイル方式は標準の OpenAI 互換エンドポイント向け。

## スクリーンショット

| ダッシュボード | Agent 連携 |
|----------------|------------|
| ![ダッシュボード](docs/assets/readme/dashboard.png) | ![Agent 連携](docs/assets/readme/agent-setup.png) |

## ドキュメント

- [ドキュメント一覧](docs/README.md)
- [環境変数](docs/guide/environment-variables.md)
- [本機プロキシ / wrap](docs/guide/system-proxy-egress.md)
- [API リファレンス](docs/api/API_REFERENCE.md)

## フィードバックとサポート

質問・提案は [GitHub Issues](https://github.com/atoml-ai/centag/issues)、または **centag@atoml.com** まで。

## ライセンス

MIT License（オープンソース版：`minimal` / `personal`）
