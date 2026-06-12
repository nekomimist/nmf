# 実用になるために必要なToDo

## 優先度高いの

### File Managerのタイトル
- Nekomimist File Managerにしたい

## 簡易viewerの高速化
- Text/Hex表示はTextからTextGridに置きかえたけどまだ遅い。
- 別windowに表示したほうが早いかもしれない。
- 抜本的対策が必要かも。
- TextGridで未対応項目がある。Text/Hex表示のキーボードによる範囲選択、MarkdownはTextのままで遅い、など。

## 優先度低めのもの

## ダイアログ系KeyHandlerの共通化
- keymanagerパッケージにdialog毎のhandlerが16ファイルあり、
  Esc=cancel / Enter=accept / ↑↓=移動 のパターンがほぼ重複している。
- メイン画面だけ宣言的binding+command registry(`keybinding.go`)へ移行済みなので、
  dialog側にも横展開して共通ベースhandler化したい。

## keymanager.FileManagerInterfaceの縮小
- 50超メソッドの神インターフェースになっていて、keymanagerがFileManagerの全機能に依存している。
- `NewMainScreenKeyHandlerWithCommands`のクロージャ登録の仕組みが既にあるので、
  `Show○○Dialog`系はクロージャ注入へ寄せてインターフェースを削りたい。

## main packageの構造整理(メモ)
- FileManagerが70ファイル超に分割されているが、全部`fm *FileManager`のメソッドで
  分割が「ファイル単位」であって「責務単位」になっていない。
- `jobs_window_controller.go`のパッケージグローバル`jobsWindow`だけシングルトンで、
  他のインスタンス指向な作りと不揃い。テスタビリティの穴にもなっている。

## ファイルコピーの高速化を検討したい
- 現状のCopy/Moveは進捗表示のために、汎用的なread -> writeループでコピーしている。
- Linuxでは`os.File.ReadFrom`経由で`copy_file_range`/`splice`を使える可能性があり、
  ローカル通常ファイル同士ならユーザ空間へデータを持ってこない高速化が期待できる。
- Windowsでは現時点のGo標準`os.File.ReadFrom`にfile-to-file向けの独自高速化はなさそうなので、
  Windows高速化は別途調査が必要。
- ただしSMB、HDD、archive、progress表示との相性があるため、過度な並列化は避けたい。
- 進捗表示は必須。高速化案はチャンク単位などで現在ファイルの進捗を維持できることを条件にする。

# DONE 以下は一応終わったもの
## キー入力処理の再設計 (KeyDown/KeyUp発火の廃止)
- 設計・検証記録・移行ステップは [todo-keyboard.md](todo-keyboard.md) を参照。
- バインドの発火をTypedKey/TypedShortcut(活性化イベント)へ一本化し、
  KeyDown/KeyUpはKeyManager内部配管(modifier追跡・畳み込みShortcut逆引き・
  arm-gateのarm)へ格下げした。configの`event`(down/up/typed)は廃止(警告+無視)。
- 抑止5点セットと`DeferUntilKeysReleased`/`ForceReleaseAllKeys`を、
  arm-gate(armed+queuedTransitions)と次tick遷移(`BeginOwnerTransition`)に置換。
- 旧todo「KeyManagerまわりの小さめの設計改善」(shouldDeferCommand属性化、
  stackVersion整理、5状態整理、mutex方針、modifier残留)と
  「KeyDown/KeyUp系キーバインドのrepeat適性を棚卸ししたい」はこれで決着。

## KeyDown/KeyUpの二重配送経路を整理したい
- Fyne v2.7.3のGLFW driver(`internal/driver/glfw/window.go`)を確認した結果、
  キーイベントの配送は排他だった: フォーカスがあれば focused object のみ、
  canvasレベルのcallbackは「何もフォーカスされていないとき」だけ呼ばれる(else if)。
  懸念していた「KeySink + canvas の二重配送」はこのバージョンでは起きない。
- したがって`ui_setup.go`の `Focused() == fm.fileListView` ガードは到達不能なデッドコードだった。
  これを一般形の防御ガード(`Focused() != nil` ならskip)に置き換え、4つのcallback全部
  (KeyDown/KeyUp/TypedKey/TypedRune)に適用した。将来Fyneが両方に配る仕様へ変わっても
  単一配送が保たれる。
- canvas callbackの役割は「フォーカス喪失時のフォールバック」であることを
  `docs/architecture/ui-input.md` に明文化した。
- Entry系widget(conflict dialogのname entry等)が必要なイベントだけKeyManagerへ
  転送するのは正しいパターン(排他配送なので重複しない)であることも確認した。

## KeyManagerのPopHandlerに所有権チェックを入れたい
- `PushHandler`が`HandlerToken`を返し、`PopHandler()`を廃止して`RemoveHandler(token)`に置き換えた。
- tokenは自分のスタックエントリだけを除去する。最上段以外の除去はWARNINGログ付きでその場から除去し、
  他のhandlerを誤って外すことはなくなった。未知のtoken(二重除去含む)はWARNINGログのみのno-op。
- 最上段除去時のみmodifier状態をリセットする(非最上段除去では入力オーナーが変わらないため維持)。
- 全dialog・busy handler・incremental search・sort dialogの呼び出し側をtoken方式に移行した。

## 起動時のウィンドウ位置を設定できるようにしたい
- `config.json` の `window.x` / `window.y` と Starlark の
  `nmf.window(x = ..., y = ...)` を追加した。
- Windowsでは起動後の最初のウィンドウを指定座標へ移動する。
- 指定座標がモニタ削除・RDP・配置変更で画面外になる場合は、
  指定座標に最も近いモニタの作業領域へウィンドウ全体が収まるように丸める。
- Windows以外では位置指定は無視する。

## 起動時のディレクトリをコマンドライン以外で設定できるようにしたい
- `config.json` の `startup.directory` と Starlark の
  `nmf.startup(directory = "...")` を追加した。
- `-path` または位置引数でコマンドライン指定された場合は、設定よりそちらを優先する。
- コマンドライン指定も設定指定もなければ、従来どおりカレントディレクトリから起動する。

## フォント指定を分ける
- 既存の `fontName` / `fontPath` はUI用フォントとして残した。
- `monospaceFontName` / `monospaceFontPath` と Starlark の
  `monospace_font_name` / `monospace_font_path` を追加した。
- `TextStyle.Monospace` の表示はmonospace用フォントを使い、未指定ならUI用フォントを継承する。
- ファイルリスト、パス表示、ステータス、履歴/コピー/比較/ディレクトリジャンプのパス候補表示をmonospace対象にした。

## デバッグログの強化
- `config.json` か `init.star` でログディレクトリを設定できるようにした。
  デフォルトは `config.json` と `init.star` と同じ場所の `logs` フォルダ。
- `config.json` か `init.star` でデバッグログを有効化できるようにした。
- デバッグログが有効な場合は、起動毎に新しいログファイルを作る。
  指定の個数を超えたら古いファイルから消える。
- デバッグログ有効時は toolbar に KeyManager 状況ダンプ用アイコンを出す。

## OK/Cancel的な二択ボタン
- 自作ボタン列をFyne標準のConfirm系に寄せ、CancelIcon/ConfirmIconとPrimary表示を揃えた。
- 1行編集、削除確認、コンパクトメッセージ、Maintenance、Jobsのボタン表示を整理した。
- Jobs WindowはEnterでCloseするようにした。

## 内部で使っている色定数を設定可能にしたい
- UI内で直接使っている色に名前をつけ、初期値は現在の内蔵値にした。
- `config.json` の `theme.colors` と Starlark の `nmf.color()` からカスタマイズできるようにした。
- 共通指定と dark/light 別指定に対応した。
- Fyne Theme の `primary` と `focus` は標準Themeへ委譲する形に戻した。

## 全ファイルマークできるコマンドを追加
- `selection.markAll` を追加。
- デフォルトキーバインドはC-A。
- `..` と削除済み表示は対象外。

## 絞り込み系のキーバインドを改善
- History/Filter/DirectoryJump/CopyMove/IncrementalSearchでCtrl-HをBackspace相当にした。
- focusless dialog用のKeySinkでCustomShortcutをKeyManagerへ流し、Ctrl-Hのリピートにも対応。

## 実行前のコマンドライン編集
- nmf.execおよびnmf.external_commandにeditオプションを追加。
- editがtrueのときは、一行編集ダイヤログで実行前のコマンドラインを確認・編集できる。
- editがtrueのときは、編集して任意のコマンドを入れられるように空コマンドも許可する。
- 内部的にはexec.Commandのcommand/argsを一度1行に合成し、編集後にシェル風の引用符・バックスラッシュ規則で再分解して実行する。

## 1行編集のReadline系制御キーのリピートを効かせたい
- Ctrl-Hなどの制御キーは単発では効くが、キーリピートが効かない問題に対応。
- Fyne/driver側ではrepeatがKeyDownではなくTypedShortcut側へ流れるため、1行編集EntryでCtrl系CustomShortcutを処理する。

## 1行編集ダイヤログを実装する
- GNU readlineっぽいキーバインドの汎用1行編集ダイヤログを実装。
- Renameコマンドの入力を汎用1行編集ダイヤログに載せ替え。
- pathEntryの直接編集をやめ、編集操作で汎用1行編集ダイヤログを出すように変更。

## コンフィグ記述言語の導入
- Starlarkの `init.star` を `config.json` の後に読む。
- `config.json` の主なカスタマイズ項目を Starlark API から設定できる。
- `user.*` コマンドとして Starlark 関数を登録し、キーに割り当てられる。
- Starlark由来のオーバーレイは通常保存時に `config.json` へ逆流しない。
- デフォルトキーを無効化するための `noop` コマンドと `nmf.unkey()`。
- `nmf.sort(..., temporary = True)` で設定を保存せず一時的にソート状態を変えられる。
- Starlarkから固定登録の名前付きメニューを作って `nmf.show_menu()` で表示できる。

## 新規File Managerの横配置
- WindowsではCtrl-Nで出した新しいFile Managerを元のFile Managerの横に出す。

## コマンド実行メニュー
- 現状ENTERでopen相当の動作ができるが、Xで拡張子毎の実行メニューを出したい。
  - 複数のコマンドを登録できて、ファイル名はコマンドの引数として渡される
    - どんな引数で渡すかの形式指定もいるかも？(単なる引数じゃなくてオプション指定がいるものもあるかもしれないからね))
  - メニューを出す仕組みはwidget.PopUpMenu？
  
## キーバインドの任意割り当て
- (SHIFTキー|ALTキー|CTRLキー)+キー名の、任意の内部機能を呼びたい。
  - "A", "S-A", "A-A", "C-A"みたいな感じの書き方ができるといいな
  - 内部機能のそれぞれのコマンド化が必要。(引き数もいるかも？)
  - Fyneのキーの扱いがやっかいかもしれない。
    - OnTypedKeyで扱えるものはそうしたほうがいい(リピートも効く)が、"文字"として取れないと扱えない。
	- OnTypedKeyで扱えないものはOnKeyDownでハンドリングしてるものが多いが、OnKeyUpであるべきかも。
  - 現状のconfig.jsonの枠組みだと書き辛いかもしれないが、
    この機能を作ってしまわないとコンフィグ記述言語を導入する意味が小さいかもしれない。
