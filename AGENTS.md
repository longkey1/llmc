# AGENTS.md

This file provides guidance to AI coding agents (Claude Code, etc.) when working with code in this repository.

## プロジェクト概要

llmcは、複数のLLMプロバイダー(OpenAI, Gemini, Anthropic, Ollama)と対話するためのGoで書かれたコマンドラインツールです。チャット、プロンプトテンプレート、会話セッション、対話モードをサポートします。

## 開発コマンド

```bash
make build          # ./bin/ にビルド (.product_name のバイナリ名を使用)
make test           # go test ./...
make fmt            # go fmt ./...
make vet            # go vet ./...
make lint           # go tool golangci-lint run (バージョンは go.mod の tool ディレクティブで管理)
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

中核は `internal/llmc/llmc.go` の `Provider` インターフェース。全プロバイダー実装(`internal/openai`, `internal/gemini`, `internal/anthropic`, `internal/ollama`)がこれを実装する。各パッケージは `ProviderName` 定数と `NewProvider(cfg)` を公開する。

Ollamaのみトークンが省略可能(ローカルサーバーは認証不要)。`GetToken("ollama")` は空文字でもエラーを返さず、プロバイダー側はトークンが設定されている場合のみ `Authorization` ヘッダーを送る。

新プロバイダー追加時に触る箇所:
1. `internal/<provider>/` に `Provider` 実装を作成(`ProviderName`, `NewProvider` を公開)
2. `cmd/provider.go` の `newProvider` の switch に case を追加
3. `internal/llmc/config/config.go` の `Config` 構造体にトークン/ベースURLフィールドを追加し、`GetToken`/`GetBaseURL`(`config.go`)と `LoadConfig` の環境変数展開、`NewDefaultConfig` のデフォルト値を更新

### ツール実行 (function calling)

`--tools` / `enable_tools` で有効化するモデル主導のツール実行機能。中央オーケストレーター方式で、ループは `internal/llmc/toolloop.go` の `RunToolLoop` に1箇所だけ存在する(上限 `MaxToolIterations`)。

- 中立型(`ToolDef`/`ToolCall`/`ToolResult`/`TurnResult`)は `internal/llmc/tool.go`
- プロバイダーはオプショナルインターフェース `ToolChatter`(`ChatWithTools`)を実装し、ワイヤ形式変換のみを担う。cmd層(`cmd/turn.go` の `runTurn`)が型アサーションで検出する。既存の `ChatWithHistory` はツール無効時のパスとして無変更
- 組み込みツール4種は `internal/llmc/tools/`(fetch_url, read_file, exec_command, write_file)。`Executor` が実行と確認フローを担い、exec_command/write_file は `RequiresConfirmation` でy/N確認必須(`--yes` でスキップ、非TTYは自動拒否)
- exec_command のポリシーは `internal/llmc/tools/policy.go`。`exec_allowed_commands`(`map[string][]string`: コマンド名 → サブコマンド。空配列または `["*"]` で全サブコマンド)は**確認のスキップ**を決める。`exec_denied_commands` は許可より優先で即拒否(`--yes` も上書き)。未登録は `exec_unlisted`(`confirm` 既定 / `deny`)次第。判定は `scanShellCommands` がクォートを追跡しつつ `;`/`|`/`&`/改行 で分割し、コマンド名は完全一致、サブコマンドは単語境界プレフィックスで照合。自動承認には行内の全コマンドが許可対象である必要がある
- プレフィックス照合で効果を検証できない構文は自動承認しない: コマンド置換(`$(`, backtick, `<(`)と**リダイレクト**(`>`, `>>`, `<`)。`ls > ~/.ssh/authorized_keys` は `ls` の許可だけで書き込みになってしまうため
- `exec_unlisted = "deny"` にすると許可リストが実行可否のゲートになる(未登録・置換・リダイレクトは確認せず拒否)。ただしサンドボックスではない(`find -exec`、`git -c core.pager=`、`sh`/`env` の許可は実質何でも通る)。この構成で何も実行できない場合は `Definitions(opts)` が exec_command をモデルに提示しない
- exec_command の環境変数は `exec_env_mode`(`filtered` 既定 / `minimal` / `all`)と `exec_env_passthrough` で制御(`buildEnv` in `exec_command.go`)。既定で `LLMC_*` と名前に `TOKEN`/`SECRET`/`API_KEY` 等を含む変数を除去する — これがないとモデルが `env` を実行してプロバイダートークンを自分に送り返せてしまう
- ファイルパスのポリシーは `internal/llmc/tools/paths.go`。`write_allowed_paths`/`write_denied_paths`/`write_unlisted` はコマンド側と同じ意味論、`read_denied_paths` は read_file 用(確認フローがないので拒否リストのみ)。ルールは**区切り文字を含めばパス前方一致**(`~` 展開あり)、**含まなければパス構成要素へのグロブ**(`.git`, `*.pem`)
- 照合の前に `resolvePath` で必ず正規化する(絶対パス化 + `..` 解決 + 親のsymlink解決)。これがないと `./a/../../../etc/hosts` やsymlinkディレクトリでルールを迂回できる。含有判定は `filepath.Rel` を使う(`strings.HasPrefix` だと `/tmp/a` のルールが `/tmp/ab` に誤マッチ)。確認プロンプトにも正規化後のパスを表示する
- write_file は対象自体がsymlinkなら `refuseSymlink` で拒否する(承認したパスの外を書き換えてしまうため)。read_file はsymlinkを追うが、拒否判定は解決後の実体に対して行う
- 既定の拒否リストは `config.DefaultReadDeniedPaths` / `DefaultWriteDeniedPaths`。`~/.config/llmc` を両方に入れているのは config.toml にプロバイダートークンが入るため
- パスのルールは `exec_command` 経由の書き込みをカバーしない(承認したシェルコマンドや、許可した `tee`/`sed` は何でも書ける)。コマンド側のルールと補完関係
- 履歴は `llmc.Message` の拡張フィールド(`ToolCalls`, `ToolCallID`, `ToolName`, `ToolIsError`、いずれも omitempty)と新role `"tool"` で表現し、セッションJSONにそのまま永続化される(旧形式と後方互換)。ツール無効時は `SanitizeHistory` でtool系メッセージを除去してから送信
- プロバイダー固有の注意: Anthropicは連続する tool_result を1つの user メッセージにマージ(user/assistant交互制約)。Geminiは call ID が無いため `call-%d` を合成し、応答は `ToolName` でマッチング。OpenAIはResponses APIのフラット形式(`function_call`/`function_call_output` アイテム)。OllamaはChat Completions標準形

### モデル指定

モデルは `provider:model` 形式(例 `openai:gpt-4o`, `anthropic:claude-3-5-sonnet-20241022`)。`llmc.ParseModelString` / `FormatModelString` で相互変換。

`@name` 形式はモデルエイリアス参照。config の `[model_aliases]` マップ(`name = "provider:model-pattern"`)を引き、値にワイルドカード `*` が含まれる場合は `ListModels` の結果から最新の一致モデルに動的解決される(`internal/llmc/resolve.go` の `ResolveModelPattern`。数値バージョンベクトル比較、日付トークンはスナップショット扱い)。`*` なしの値はAPI呼び出しなしでそのまま使用。解決オーケストレーターは `cmd/provider.go` の `resolveModelAlias` で、チャットの単発実行時と新規セッション作成直前(セッションには解決済みの具体名が固定保存される)に呼ばれる。未定義の `@name` はエラー。`llmc models resolve <@alias|provider:pattern>` で解決結果を確認できる。

### 設定 (internal/llmc/config)

`Config` 構造体は viper で TOML からアンマーシャルされる。トークン/ベースURLは `$VAR` および `${VAR}` 形式の環境変数参照を `LoadConfig` 内で展開する(未設定時は空文字)。相対パス(prompt_dirs等)は `ResolvePath` で設定ファイルのディレクトリ基準に絶対パス化される。

設定の優先順位(高→低):
1. コマンドラインフラグ
2. 環境変数 (`LLMC_` プレフィックス)
3. プロンプトテンプレート (`model`, `web_search`, `tools` のみ)
4. ユーザー設定 `$HOME/.config/llmc/config.toml`
5. システム設定 `/etc/llmc/config.toml`
6. デフォルト値

### セッション (internal/llmc/session)

セッションはUUIDをファイル名とするJSONファイルで永続化される。保存先は設定ファイルと同じディレクトリの `sessions/`(設定ファイル未使用時は `$HOME/.config/llmc/sessions`)。`storage.go` が CRUD と、4文字以上のプレフィックス検索(`FindSessionByPrefix`、複数一致は `AmbiguousIDError`)、`latest` エイリアス(`GetLatestSession`)を提供。

`SessionMessageThreshold`(デフォルト50、0で無効)と `SessionRetentionDays`(デフォルト30日)で自動削除を制御する。

### コマンド層 (cmd/)

Cobraベース。`root.go` が共通フラグと設定読み込み、`chat.go`/`sessions.go`(対話モード含む)/`models.go`(`resolve` サブコマンド含む)/`prompts.go`/`config.go`/`init.go` が各サブコマンド。`provider.go` はプロバイダー生成とモデルエイリアス解決のユーティリティ(コマンドではない)。

## 主要な依存関係

- `github.com/spf13/cobra` - CLIフレームワーク
- `github.com/spf13/viper` - 設定管理
- `github.com/BurntSushi/toml` - TOMLパーサー
- `github.com/knz/bubbline` - 対話モードのマルチライン行編集/履歴/外部エディタ起動 (bubbletea ベース)
- `github.com/google/uuid` - セッションID生成
