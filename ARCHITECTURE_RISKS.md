# NMF Architecture Risk Notes (by Codex CLI)

このドキュメントは、現在のコードベースを踏まえて見つかった懸念点（主に設計・並行性・入力経路・設定整合）を短く要約したメモです。優先度の高いものから順に列挙します。

## High Risk
- キーイベントの二重配送
  - 症状: `main.go` の `desktop.Canvas` ハンドラと `ui.KeySink` の双方が `KeyManager` にイベントを転送している。フォーカス状況によっては同一イベントが二重に処理される可能性。
  - 影響: ショートカットやカーソル移動の多重反応、不安定な入力体験。
  - 根拠: `main.go` の `dc.SetOnKeyDown/Up` と `KeySink.KeyDown/KeyUp/Typed*` が同じ `KeyManager` を呼ぶ実装。

- UI スレッド安全性（ウォッチャ → UI 更新）
  - 症状: `watcher.DirectoryWatcher` のゴルーチンから `fm.UpdateFiles()`・`fileBinding.Set()`・`fm.files/selectedFiles` へ直接アクセス。
  - 影響: データレースおよび UI スレッド外更新によるクラッシュ・不整合の可能性。
  - 根拠: `watcher.applyDataChanges()` 内で直接 `fm.UpdateFiles(files)` を呼び、`main` 側のスライス/マップを同時更新。

- ウォッチャのチャネルクローズ時パニックの可能性
  - 症状: `Stop()` で `changeChan` を `close` 後、変更適用側ゴルーチンの `select { case changes := <-dw.changeChan: ... }` が `nil` を受信し、`changes.Added` 参照で `panic` し得る。
  - 影響: まれにウィンドウクローズ/再起動時にクラッシュ。
  - 根拠: 受信側で `ok` チェックがなく、`*PendingChanges` のゼロ値は `nil`。

- Windows のツリーダイアログのルート未対応
  - 症状: ツリー起点が常に `/`。Windows でドライブ列挙がされない。
  - 影響: Windows 環境でナビゲーション不能/不自然な表示。
  - 根拠: `ui/tree_dialog.go` で `currentRoot: "/"` 固定、プラットフォーム分岐なし。

- 設定未適用/未実装
  - 症状: `ui.showHiddenFiles` がファイル一覧の表示制御に未反映。`cursorStyle.type` の `icon`/`font` は未実装（フォールバックなし）。
  - 影響: 設定と実表示の乖離、ユーザー期待とのズレ。

- 直値（ハードコード）の散在
  - 症状: 選択背景色・監視間隔などがコード直書き。
  - 影響: 一貫性低下、調整が局所最適化に。
  - 根拠: `main.go` の選択色、`watcher` の `time.NewTicker(2 * time.Second)` など。

- 削除→再追加での重複表示の恐れ
  - 症状: 削除ファイルは一覧に残し（打消し表示）、再追加は末尾に **追加**。同一パスが二重に存在し得る。
  - 影響: 重複行・予期しないソート/選択挙動。
  - 根拠: `applyDataChanges` で Deleted はステータス更新のみ、Added は重複チェックなしで append。

## Medium Risk / Quality
- `main.go` の肥大化
  - 症状: UI 構築、描画、入出力、ソート/フィルタ適用、監視起動などが集中。
  - 影響: 可読性・変更容易性の低下、変更時の副作用増。

- ソートの安定性
  - 症状: `sort.Slice` は不安定ソート。等価キー間で並びが揺れる可能性。
  - 影響: リストのちらつき/選択位置の小さなズレ。

- Theme 実装の更新遅れ
  - 症状: 非推奨 `ioutil.ReadFile` を使用。
  - 影響: 低。とはいえ将来のワーニング・整合性。

## Claude検証結果（2025-08-15）

上記GPT5分析を詳細コード検証した結果、**6/7項目のHigh Riskに同意**する。

### ✅ 同意するHigh Risk（危険度順）
1. **UI スレッド安全性**: `watcher.go:242`でゴルーチンから`UpdateFiles()`直接呼び出し → 確実なデータレース・クラッシュリスク
2. **キーイベント二重配送**: `main.go:456` + `KeySink.KeyDown`が同じ`KeyManager`を呼ぶ → カーソル飛び・多重発動
3. **ウォッチャのパニック**: `watcher.go:85`でクローズチェックなし → Stop()後の稀なクラッシュ
4. **Windows ツリー未対応**: `tree_dialog.go:43`でルート`"/"`固定 → Windows環境で機能破綻
5. **設定未適用**: `ShowHiddenFiles`設定は存在するがフィルタリング処理未実装 → ユーザー期待とのズレ
6. **削除→再追加重複**: `watcher.go:236-237`で重複チェックなし → UI破綻・予期しない動作

### ⚠️ Medium Riskに格下げ
- **直値の散在**: クラッシュリスクなし、保守性の問題

## Notes / 参考（対処の方向性のメモ）
- UI 更新のメインスレッド化: `CallOnMain` で `UpdateFiles`/`binding.Set` を包む。`FileManager` 状態は `sync.Mutex` 等で保護。
- ツリーの Windows 対応: 複数ドライブをルート列挙、子ノードは名称昇順でソート。
- 設定整合: `ShowHiddenFiles` のフィルタリング適用、未実装カーソルタイプはフォールバック（例: `underline`）。
- 直値の定数化: 選択背景色・監視間隔などを `internal/constants` に寄せる。
- 削除/再追加時: 同一パスの既存行があれば上書き（ステータス復帰）し、重複追加を避ける。

## 対応状況 (2025-08-15)
2. **キーイベント二重配送**: 済み
3. **ウォッチャのパニック**: 済み
4. **Windows ツリー未対応**: 済み (2025-08-15) - プラットフォーム固有のドライブ列挙機能を実装
