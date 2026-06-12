# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## プロジェクト概要

llmcは、複数のLLMプロバイダー(OpenAI, Gemini, Anthropic)と対話するためのGoで書かれたコマンドラインツールです。チャット、プロンプトテンプレート、会話セッション、対話モードをサポートします。

## 開発コマンド

```bash
make build          # ./bin/ にビルド (.product_name のバイナリ名を使用)
make test           # go test ./...
make fmt            # go fmt ./...
make vet            # go vet ./...
make tidy           # go mod tidy

# 単一テスト実行
go test ./internal/llmc/ -run TestParseModelString -v

# リリース (デフォルトdry-run、dryrun=false で実行)
make release type=patch|minor|major dryrun=false
make re-release tag=<tag> dryrun=false   # 既存タグの再リリース
```

リリースはgitタグのpush起因でGitHub Actions + goreleaserがバイナリをビルドする。

## アーキテクチャ

### Providerインターフェースによる抽象化

中核は `internal/llmc/llmc.go` の `Provider` インターフェース。全プロバイダー実装(`internal/openai`, `internal/gemini`, `internal/anthropic`)がこれを実装する。各パッケージは `ProviderName` 定数と `NewProvider(cfg)` を公開する。

新プロバイダー追加時に触る箇所:
1. `internal/<provider>/` に `Provider` 実装を作成(`ProviderName`, `NewProvider` を公開)
2. `cmd/provider.go` の `newProvider` の switch に case を追加
3. `internal/llmc/config/config.go` の `Config` 構造体にトークン/ベースURLフィールドを追加し、`GetToken`/`GetBaseURL`(`config.go`)と `LoadConfig` の環境変数展開、`NewDefaultConfig` のデフォルト値を更新

### モデル指定

モデルは `provider:model` 形式(例 `openai:gpt-4o`, `anthropic:claude-3-5-sonnet-20241022`)。`llmc.ParseModelString` / `FormatModelString` で相互変換。

### 設定 (internal/llmc/config)

`Config` 構造体は viper で TOML からアンマーシャルされる。トークン/ベースURLは `$VAR` および `${VAR}` 形式の環境変数参照を `LoadConfig` 内で展開する(未設定時は空文字)。相対パス(prompt_dirs等)は `ResolvePath` で設定ファイルのディレクトリ基準に絶対パス化される。

設定の優先順位(高→低):
1. コマンドラインフラグ
2. 環境変数 (`LLMC_` プレフィックス)
3. プロンプトテンプレート (`model`, `web_search` のみ)
4. ユーザー設定 `$HOME/.config/llmc/config.toml`
5. システム設定 `/etc/llmc/config.toml`
6. デフォルト値

### セッション (internal/llmc/session)

セッションはUUIDをファイル名とするJSONファイルで永続化される。保存先は設定ファイルと同じディレクトリの `sessions/`(設定ファイル未使用時は `$HOME/.config/llmc/sessions`)。`storage.go` が CRUD と、4文字以上のプレフィックス検索(`FindSessionByPrefix`、複数一致は `AmbiguousIDError`)、`latest` エイリアス(`GetLatestSession`)を提供。

`SessionMessageThreshold`(デフォルト50、0で無効)と `SessionRetentionDays`(デフォルト30日)で自動削除を制御する。

### コマンド層 (cmd/)

Cobraベース。`root.go` が共通フラグと設定読み込み、`chat.go`/`sessions.go`(対話モード含む)/`models.go`/`prompts.go`/`config.go`/`init.go` が各サブコマンド。`provider.go` はプロバイダー生成のユーティリティ(コマンドではない)。

## 主要な依存関係

- `github.com/spf13/cobra` - CLIフレームワーク
- `github.com/spf13/viper` - 設定管理
- `github.com/BurntSushi/toml` - TOMLパーサー
- `github.com/chzyer/readline` - 対話モードの行編集/履歴
- `github.com/google/uuid` - セッションID生成
