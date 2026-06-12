# キー入力処理の再設計 (KeyDown/KeyUp発火の廃止) — 完了

全ステップ実装済み。本ファイルは設計判断とFyne依存挙動の検証記録として残す。
現行仕様の正は `docs/architecture/ui-input.md`。

## 背景 / 動機

- 現状はKeyDown/KeyUpでShift/Ctrl/Altや他のキーを見ており、UpとDownを別々に
  処理する必要がある。押したまま画面遷移したり他アプリへフォーカスが移ると、
  遷移先でキーが処理されたりUpを取りこぼしたりするため、多数の抑止機構
  (`pressedKeys`/`pending`/`suppressTyped`/`suppressRune`/`activeTypedKey`、
  `DeferUntilKeysReleased`、`ForceReleaseAllKeys`)を入れる原因になっていた。
- 本設計はバインドの発火をTypedKey/TypedShortcut(=活性化イベント)に一本化し、
  KeyDown/KeyUpをKeyManager内部の配管(modifier追跡とゲートのarm)に格下げする。
  これにより抑止機構は小さな状態2つに置き換えられる。
- 旧todo「KeyDown/KeyUp系キーバインドのrepeat適性を棚卸ししたい」(GPT-5.5見解)の
  「完全移行は複雑さの置き場所が変わるだけ」という結論は採らない。
  pending/commit方式は不要で、「遷移を次tickへ送る + 新規KeyDownでarm」で足りる。

## Fyne v2.7.3 ドライバ配送仕様 (検証済みの事実)

`fyne.io/fyne/v2@v2.7.3/internal/driver/glfw/window.go` で確認。
Fyneバージョン更新時はここを再検証すること。

- `processKeyPressed` (window.go:658) の配送順:
  - press: KeyDown → (Tab処理/shortcut判定) → TypedKey または TypedShortcut → TypedRune
  - repeat: KeyDownなし。shortcut判定 → TypedKey/TypedShortcut → TypedRune
  - release: KeyUpのみ
  - つまりキーリピートはTyped系/Shortcut系にだけ流れ、KeyDownを生まない。
- `triggersShortcut` (window.go:765) の畳み込み規則:
  - Ctrl/Alt/Superを含む組み合わせ → `desktop.CustomShortcut`。
    イベント自体に正確なmodifierビットが載る(追跡状態より信頼できる)。
  - ただし Ctrl単独+C/X/V/A/Z/Y/Insert と Shift単独+Insert/Delete は標準Shortcut
    (Copy/Cut/Paste/SelectAll/Undo/Redo)に畳まれ、元のキー名が消える。
    C-CとC-Insertは事象として区別不能。C-XとS-Deleteはどちらも Cut になる。
  - Shift単独+キーはshortcutにならない(window.go:817で明示的に除外)。
    `fyne.KeyEvent`にmodifier情報が無いため、TypedKeyだけではUpとS-Upを
    区別できない。→ Shift状態の追跡だけは廃止できない(本設計唯一の追跡)。
  - 1打鍵で発火するのはTypedKeyかTypedShortcutのどちらか片方のみ(排他)。
- 配送の排他性: フォーカスがあれば focused object のみ、canvasコールバックは
  無フォーカス時のみ(既存ui-input.mdに記載済み)。
- `fyne.Do` は実行中アプリのメインスレッドから呼ぶとキューに積まれ
  (loop.go:40-56 `runOnMainWithWait`)、現在のPollEventsバッチ
  (同一打鍵のchar→TypedRuneまで)が終わってから実行される。
  → 「遷移を次tickへ送る」の実装基盤。将来の仕様変更に備えラッパ関数で隔離する。
- ウィンドウのフォーカス喪失は `processFocused(false)` → canvas →
  `FocusManager.FocusLost()` (internal/app/focus_manager.go:74) →
  focused widgetの`FocusLost()`に届く。→ KeySinkでalt-tabを検知できる。

## 設計

### 1. 活性化イベントへの一本化

- バインドの発火はTypedKey/TypedShortcutのみ。KeyHandlerの入口を統合する:

```go
type KeyHandler interface {
    OnKeyActivated(ev *fyne.KeyEvent, mods ModifierState) bool // typed/shortcut両経路の合流点
    OnTypedRune(r rune, mods ModifierState) bool
    GetName() string
}
```

- Ctrl/Alt系はshortcut経路(modifierはイベントから取得)、Shift単独・無修飾は
  typed経路(Shiftのみ追跡状態を使用)で解決し、KeyManagerが合流させる。
  handlerとconfigはどの経路かを知らない。
- config/Starlarkの `event: down|up|typed` は廃止。バインドは key + command のみ。
  既存設定のevent指定はパース時に警告して無視(後方互換)。
- 全バインドが一律リピートするようになる。増殖して困る操作(=入力オーナー遷移系)は
  後述のarm-gateが構造的に保護するため、repeat適性の個別棚卸しは不要になる。
- incremental searchのBackspace等、現状downでリピートしない操作が自動的に直る。

### 2. 畳み込みShortcutの逆引き

- KeyManagerは「直近の非修飾KeyDownのキー名」(lastKeyDown、1フィールド)を覚える。
- ShortcutCopy/Cut/Paste/SelectAll/Undo/Redo を受けたら元キーを復元する:
  - Copy: lastKeyDown==Insert → C-Insert / それ以外 → C-C
  - Cut: lastKeyDown==Delete → S-Delete / それ以外 → C-X
  - Paste: lastKeyDown==Insert → S-Insert / それ以外 → C-V
  - SelectAll → C-A、Undo → C-Z、Redo → C-Y
- pressでは KeyDown→TypedShortcut が同一コールバック内で連続するので確実。
  repeatではlastKeyDownが変化しないため同様に正しい。
- 非ASCIIレイアウトでdriverがkeyASCIIを代入するケース(KeyUnknown)でも
  lastKeyDownがInsert/Deleteにならないので正しくC-C等へ落ちる。

### 3. modifier追跡の縮小

- KeyDown/KeyUpはKeyManager内部専用とし、用途を3つに限定:
  1. 修飾キー6種のModifierState更新(実質Shiftのためだけに必要。
     Ctrl/Altはshortcutイベント側に正確な値が載る)
  2. arm-gateのarm(非修飾キーの新規pressのみ)
  3. lastKeyDownの記録
- handlerにはOnKeyDown/OnKeyUpを公開しない。
- KeySinkのFocusGained/FocusLost(window unfocusでも届く)でModifierStateと
  gateをリセット → 「修飾キー押下中のフォーカスロストで状態残留」問題の解消。
  KeySinkはkm参照を持っているので全KeySinkで自動的に行う(個別配線不要)。

### 4. 遷移ゲートの置き換え

抑止5点セット+DeferUntilKeysReleased+ForceReleaseAllKeysを次の2ルールに置換:

- **(a) 入力オーナー遷移は次tickで実行**
  - dialog開閉・window生成/フォーカス移動など入力オーナーが変わるコマンドは
    fyne.Doラッパ経由でキューし、次のループ反復で実行する。
  - 同一打鍵の残りイベント(TypedRune等)は旧オーナーに配送され無害に捨てられる。
    例: Jでjumpダイアログを開いても 'j' のruneは旧main画面のOnTypedRune(無視)に落ちる。
  - 現仕様の「全キー解放まで開かない」より応答が速くなる。
  - Rename(R)が`up`イベントだった理由(runeリーク回避)も消えるためtypedに戻す。
- **(b) arm-gate** (bool 1個+遷移中フラグ1個、実装はtri-state):
  - 状態: Armed / TransitionPending / WaitingFreshPress
  - disarm条件: handler push/remove、KeySinkのフォーカス変化、遷移キュー時
    (キュー時はTransitionPending、遷移実行完了でWaitingFreshPressへ)
  - arm条件: 非修飾キーの新規KeyDownのみ。同一打鍵のTypedKey/TypedRuneは
    KeyDownの後に来るため取りこぼしゼロ。
  - disarm中のTypedKey/TypedShortcut/TypedRuneは**キューせず破棄**(仕様として明文化)。
  - リピートはKeyDownを生まないため、押しっぱなしのキーが遷移をまたいで
    新オーナーで発火することは構造的に起こらない:
    - Enter押しっぱなしでdialogを閉じてもmain画面にEnterリピートが落ちない
    - ←押しっぱなしのwindowフォーカス移動ping-pongは移動先KeyManagerが
      FocusGainedでdisarmされるため止まる
    - alt-tabで他アプリへ逃げてもキー集合の追跡自体が無いので破綻しない
- `shouldDeferCommand`のハードコードswitchはコマンド定義側の`Transition bool`属性へ
  移行(遷移フラグ自体はルール(a)のために引き続き必要)。
- conflict dialogのname entryのKeyDown/KeyUp転送は「TypedShortcutだけ転送」に簡素化。

## トレードオフ (受け入れ済み)

- `up`/`down`発火のバインドは表現不能になる。現在実質使用はR(up)のみで、
  それもリーク回避目的だったため実害なし。
- Entryフォーカス型dialog(rename等)で押しっぱなしにするとリピートがentryに入る
  (一般GUIと同じ挙動。現仕様は解放まで抑止していた)。
- 遷移キュー〜handler設置間の約1tick(≦16ms)の入力は破棄される。
  ただし現仕様の「全キー解放まで破棄」より窓は狭い。
- C-CとC-Insert等の区別はlastKeyDown逆引きに依存(イベント単体では区別不能)。
- macOSではCtrl畳み込みがCmd基準(`isMacOSRuntime`)になる。現対象はWindows/Linuxの
  ため注記のみ。将来macOS対応時はSuper modifierの扱いを再設計する。

## 依存するFyne挙動 (バージョン更新時の再検証ポイント)

1. focused objectとcanvasコールバックの排他配送
2. リピートがKeyDownを生まずTypedKey/TypedShortcutへ流れること
3. `triggersShortcut`の畳み込み表(標準Shortcut化される組み合わせ)
4. `fyne.Do`(メインスレッド発)が現在のPollEventsバッチ後に実行されること
5. window unfocusでfocused widgetの`FocusLost()`が呼ばれること

## 移行ステップ (各段で出荷可能)

- [x] Step1: 発火をTypedKey/TypedShortcutへ一本化
  - KeySinkで全Shortcut転送、KeyManagerにlastKeyDown+畳み込み逆引き
    (`HandleShortcut`)を追加
  - main画面bindingのevent廃止(specから自動解決)、config/Starlarkのeventは
    deprecated警告(受理して無視)
  - dialog handlerのOnTypedKeyベアキー分岐にmodifierガード追加
    (quitのY/N vs C-Y、各dialogのDelete vs S-Delete等の衝突回避)
  - 無フォーカスfallback用にcanvasへ`ActivationShortcuts()`を登録
    (折り畳みShortcutはCustomShortcutにならないため明示登録が必要)
  - 既存の遷移ゲートはこの段階では維持(typed発火の遷移は既存機構で動作する)
- [x] Step2: KeyHandlerからOnKeyDown/OnKeyUpを削除
  - KeyHandlerはOnKeyActivated+OnTypedRune+GetNameの3メソッドに縮小。
    HandleKeyDown/HandleKeyUpはKeyManager内部配管(modifier追跡・lastKeyDown・
    pendingフラッシュ)専用になり、handlerへはdispatchしない
  - 全dialog handlerのOnKeyDownロジックをOnKeyActivatedへ移植
    (viewerの矢印/incremental searchのBackspace等がリピート対応になった)
  - conflictNameEntryのKeyDown/KeyUp転送をAlt系TypedShortcut転送に置換
  - fileViewerTextGridのTypedShortcutも全Shortcut転送に統一
    (Ctrl+CのコピーはShortcutCopy経由でhandlerに届く)
- [x] Step3: 抑止機構をarm-gate+次tick遷移に置換
  - `armed` + `queuedTransitions`の2状態に縮小(tri-state相当)。
    `BeginOwnerTransition`がfyne.Doラッパ(`queueOnMain`、app無しでは同期実行)で
    次tickに遷移を実行。disarm中のイベントはキューせず破棄
  - 5点セット(pressedKeys/pending/suppressTyped/suppressRune/activeTypedKey)、
    `DeferUntilKeysReleased`、`ForceReleaseAllKeys`を削除。
    `ResetTransientState`を追加(modifier+gate+lastKeyDownのリセット)
  - `shouldDeferCommand`のswitchを廃止し、コマンド定義側の`commandSpec.transition`
    属性へ移行。旧switchから漏れていた`archive.extract`の遷移指定もこの際修正
  - KeySinkのFocusGained/FocusLostで`ResetTransientState`(alt-tab/dialog開閉での
    状態残留を解消)
  - 「activationをKeyManagerへ転送する経路は必ずKeyDownも転送する」を不変条件化
    (conflictNameEntryのKeyDown/KeyUp転送はgate armのため維持)
- [x] Step4: 残課題整理
  - currentHandlerAndVersionを削除。stackVersionはDumpState専用の診断カウンタとして残置
  - スレッディング方針を決定: dispatchはUIスレッド、push/remove・遷移は
    バックグラウンドgoroutineからも来るためmutexは維持(構造体コメントに明文化)
- [x] Step5: ドキュメント更新
  - ui-input.md(活性化モデル・gate仕様・依存ドライバ挙動5点の検証記録)
  - configuration.md / starlark-configuration.md(event廃止、deprecated注記)
  - CLAUDE.md(KeyManager説明の更新)、todo.mdの関連項目をDONEへ移動

## 関連する既存todoとの関係

- 「KeyManagerまわりの小さめの設計改善」: shouldDeferCommandの属性化、
  5状態の整理、TOCTOU/mutex方針、modifier残留 → 本設計で全て決着する。
- 「KeyDown/KeyUp系キーバインドのrepeat適性を棚卸ししたい」: 全バインドが
  リピートする+遷移はgateで保護、により棚卸し自体が不要になる。
- 「ダイアログ系KeyHandlerの共通化」: Step2のOnKeyActivated移植と同時に
  共通ベースhandler化すると一石二鳥(必須ではない)。
