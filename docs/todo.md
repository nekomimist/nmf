# 実用になるために必要なToDo

# 優先度高いの
## File ManagerのタイトルをNekomimist Filerとし、バージョンを1.0.0にする
- 以降はユーザーから見える変更はdocs/CHANGELOG.mdに記載する。

# 優先度低めのもの
## main packageの構造整理(メモ)
- FileManagerが70ファイル超に分割されているが、全部`fm *FileManager`のメソッドで
  分割が「ファイル単位」であって「責務単位」になっていない。
- (`jobsWindow`シングルトンの解消は完了。残りは責務単位の再編のみ)

## ファイルコピーの高速化を検討したい
- 現状のCopy/Moveは進捗表示のために、汎用的なread -> writeループでコピーしている。
- Linuxでは`os.File.ReadFrom`経由で`copy_file_range`/`splice`を使える可能性があり、
  ローカル通常ファイル同士ならユーザ空間へデータを持ってこない高速化が期待できる。
- Windowsでは現時点のGo標準`os.File.ReadFrom`にfile-to-file向けの独自高速化はなさそうなので、
  Windows高速化は別途調査が必要。
- ただしSMB、HDD、archive、progress表示との相性があるため、過度な並列化は避けたい。
- 進捗表示は必須。高速化案はチャンク単位などで現在ファイルの進捗を維持できることを条件にする。

# 優先度かなり低いの
## SMB/UNCまわりの残課題
- Network/auth error typingを追加し、auth required/failed、host unreachable、
  share not found、timeout、credential conflictなどをUIで読みやすいエラーにする。
- Transient network failureには保守的なretry/backoffを検討する。auth failureでは
  stale credentialを消して再promptする。
- Watcherのlist source注入やprovider `Capabilities()` ベースのpolling判定を検討し、
  `smb://` 文字列heuristicsへの依存を減らす。
- Non-Linux direct SMB provider / copy-move behavior の方針を決める。
  Windows native UNC backed に寄せるか、unsupported-by-design として明文化するか、
  direct provider parity を実装するかを選び、OS別のテストを追加する。
- SMB integration test は `NMF_SMB_TEST_DIR=...` の手動実行のみなので、
  dockerized Samba などの repeatable fixture を使うCI/gated jobを検討する。
- Windows long-path (`\\?\UNC\...`) のresolver、display normalization、
  file opening、Windows connection retryへの影響をauditする。
- Credential cacheを複数window間でどう扱うかを明文化する。
- 将来必要ならshare enumeration/network discovery UIや、SMB copy/moveの
  conflict handling/partial artifact cleanupを検討する。
- 詳細な設計は `docs/architecture/vfs-smb.md` を参照する。

# DONE 以下は終わったもの
## Starlark設定I/Fの整理
- `nmf.clipboard`を`nmf.set_clipboard`へ改名し、旧名は初回呼び出し時のみ警告する
  非推奨エイリアスとして残した。
- `nmf.run`/`nmf.exec`のキーワード引数を`cmd`に統一した。旧`command=`は
  警告付きで動作を維持し、`cmd`と`command`を同時に渡すとエラーにした。
- window/theme/key/external_command/commandなど設定系builtinをinit.star読み込み時
  専用にし、カスタムコマンド実行中に呼ぶとエラーにした。`nmf.sort`のみ
  `temporary = True`でカスタムコマンドからの利用を維持する。
- `nmf.mkdir`/`nmf.save_clipboard`/`nmf.exec`の`edit = True`時の即時戻り値を、
  `nmf.message`と同じくダイアログ予約成功を示す`True`へ統一した。
- `Load`/`LoadWithDisplay`/`LoadWithDisplayAndDebugHook`の3つのGoローダーAPIを、
  `Options`構造体を取る単一の`Load`へ統合した。

## ダイアログ系KeyHandlerの共通化
- keymanagerに宣言的binding表ベースの共通ベースhandler(`dialog_handler.go`)を追加し、
  16個のdialog handlerのうち13個(sort/history/filter/tree/compare/conflict/copymove/
  deleteconfirm/directoryjump/jobs/maintenance/quit/incremental_search)を移行した。
- key specはconfigキーバインドと同じ構文(`parseKeySpec`)を再利用し、修飾キーは厳密一致。
  旧switch文の緩い一致(修飾キー無視のelse分岐等)は厳密一致へ正規化し、
  Enter/KP_Enterは全ダイアログ統一で受けるようにした(7ダイアログでKP_Enterが新たに有効)。
- busy/fileviewer/lineeditは既に宣言的または特殊用途のため対象外のまま維持。
- 未テストだった8 handlerにテーブル駆動テストを追加し、共通ベース自体のテストも追加した。

## keymanager.FileManagerInterfaceの縮小
- `Show○○Dialog`系など22のUI起動メソッドを`DialogActions`クロージャ構造体
  (bootstrapで`SetActions`により注入)へ移し、インターフェースを51→29メソッドに縮小した。
- コマンドID・キーバインド表・transition属性はkeymanager側の定義サイトに維持した。
- Starlark(configscript)が使うShow系4メソッドは、既存のRunExternalCommand/SetClipboardと
  同じ形でCommandContextのクロージャフィールドへ移した。

## jobsWindowシングルトンの解消
- main package最後のパッケージグローバルだった`jobsWindow`を、bootstrap時に1つ生成して
  各FileManager(Ctrl-N/再オープン含む)へ注入する`JobsWindowController`へ置き換えた
  (`WatchHub`と同じ共有パターン)。
- 挙動は従来どおり: 全ウィンドウで単一Jobs Window共有、ユーザーが閉じた後の再表示、
  終了時のクローズ。`fyne.io/fyne/v2/test`ベースのユニットテストを追加した。

## Sort DialogのKeySink化 (Tabで入力不能になる潜在バグの修正)
- Sort DialogだけコンテンツがKeySinkで包まれておらず、TabでFyneネイティブの
  フォーカス移動がradioItem(TypedKeyがno-op)に着地して以降の全キー入力が
  飲み込まれる潜在バグをGUI検証中に発見し、修正した。
- 他のダイアログと同じKeySinkパターン(`WithTabCapture`+表示時フォーカス+
  `unfocusIfDialogOwned`)へ揃えた。
- スタブだったTab/S-Tabのフィールド移動を仮想「現在フィールド」ハイライトとして実装し、
  Spaceで現在フィールドのトグル/サイクルができるようにした。radio/checkのマウス操作は
  従来どおり有効で、クリック後はsinkへ再フォーカスする。

## キーハンドリングレビューのフォローアップ修正
- d7bfa767以降のキーハンドリング変更をレビューし、4件を修正した。
- File Viewerの文字キーバインドがTypedKeyとTypedRuneの両経路で照合され
  1押下2回発火していたのを、キースペック由来の経路分割で修正した。
- Conflict DialogのRename入力欄が修飾キー状態を無視していたのを修正し、
  Shift+矢印のテキスト選択とShift付きlineEditバインドが効くようにした。
- コマンドメニューの外側クリックがDismissを経由せずキー状態リセットと
  フォーカス復帰をスキップしていたのを、専用オーバーレイ化で修正した。
- Rename入力欄の埋め込みLineEditEntryがwidget implを先取りしてテーマ
  オーバーライドが効かず、カーソル/選択色がFyne標準のままだったのを修正した。
- 設計上の契約はdocs/architecture/ui-input.mdへ反映した(driver fact 6、
  経路分割、popup dismissal、埋め込みwidgetのimplスロット)。

## 簡易viewerのテキスト全選択
- File ViewerのText/Markdown/Hex各ペインで、C-aに全選択をバインドした。
- C-a後のC-cで、現在ペインの表示テキスト全体をクリップボードへコピーできる。
- キーボードによる他の範囲選択は追加せず、既存どおりマウス選択に任せる。
- 旧「簡易viewerの残課題」は、MarkdownのTextGrid化済み方針を含めて完了扱いにした。

## いくつかのダイヤログをFile Managerのサイズに合わせて横に広げる
- Navigation History、Copy/Move、Compare Directories、Tree Dialog、
  Directory Jumpは、表示時点のFile Manager幅の90%を目安に横へ広がるようにした。
- 現在の固定幅は最小幅として維持し、高さ方向は従来どおり固定値のままにした。
- Renameだけ一行編集ダイアログの可変幅対象にし、File Manager幅の70%、
  最大960pxまで広がるようにした。他のLine Edit系は現状維持。
- File viewerは既に親サイズ比率と`viewer.maxWidth`/`viewer.maxHeight`で
  サイズ調整しているため、既存挙動を維持した。
- OK/Cancelなどの標準ボタン列は、Fyne標準の自然幅・中央配置のままにした。

## ファイル一覧のマウス操作改善
- ファイル名領域からもWindows Shell D&Dを開始できるようにした。
  アイコンからの既存D&Dは維持している。
- ファイル名領域の左クリックでマークをトグルできるようにした。
- Shift+左クリックで、クリック前のカーソル位置からクリック先までを範囲マークするようにした。
  `..` と削除済み表示は、既存のキーボード操作と同じくマーク対象外。

## LineEditとFileViewerのキーバインド設定
- `ui.keyBindings`と`nmf.key()`/`nmf.unkey()`/`nmf.clear_keys()`に
  `target`を追加し、`main`、`lineEdit`、`fileViewer`を指定できるようにした。
- 対象をLineEdit/FileViewerの既存組み込みコマンドに絞り、`user.*`や
  Starlark callable bindingはmain-screen専用のまま維持した。
- Conflict DialogのRename入力欄も`lineEdit` targetのキーバインドを使うようにした。

## FileViewの検索/行ジャンプ欄と選択色
- FileViewの検索欄と行ジャンプ欄のカーソルと選択範囲に、
  既存の`lineEditCursor`/`lineEditSelection`色を適用した。
- 閲覧部のマウス選択範囲にも`lineEditSelection`を適用した。
- 設定名は互換性優先で維持し、docsの適用範囲説明を更新した。

## FileViewの検索/行ジャンプ欄のEsc戻り
- FileViewの検索欄と行ジャンプ欄にフォーカスがある時、Escで入力を送信せず、
  viewerを閉じずに閲覧部へ戻るようにした。

## Explorerメニューの重複項目抑制
- TABによるExplorerメニュー表示で、Shell拡張由来の同じ項目が複数出る場合に
  2つ目以降を表示前に間引くようにした。

## パス監視の負荷低減
- `github.com/fswatcher/fswatcher` を導入し、ローカルのwatch可能なパスは
  OSイベント監視を主経路にした。
- アプリ全体で共有する `WatchHub` を追加し、同じパスを複数File Managerで
  開いても監視sourceとsnapshot読み取りを1つにまとめるようにした。
- watcher作成・path登録・runtime error時は、そのpath sourceだけ既存相当の
  polling fallbackへ切り替える。

## 簡易viewerの高速化
- 遅さの正体はMarkdownタブだった。`widget.NewRichTextFromMarkdown`に全文を渡して
  ダイアログ表示時に即時構築しており、FyneのRichTextは非仮想化のため
  `dialog.Show()`/`Resize()`のレイアウト計算で全文のテキスト計測が走っていた
  (228KBのJSONで計約1分30秒)。Text/Hex用の自作TextGridは仮想化済みで無関係。
- MarkdownタブをHexタブと同じ遅延構築(タブ選択時に生成)へ変更し、
  Markdown専用の64KB表示上限(打ち切り注記つき)を追加した。
- Text/Hex/Markdownの表示上限は`fileinfo.PreviewReadLimit`(1MiB)へ一本化した。
- 同じ223KBのJSONで open-ready が約23msになった。残課題は優先度低の
  「簡易viewerの残課題」へ移動。

## Markdown viewerのTextGrid化
- MarkdownタブからFyne RichTextを撤去し、`goldmark` ASTを簡易テキストへ変換して
  既存のTextGridで表示するようにした。
- GFM pipe tableは等幅テキスト表へ整形し、長いセルは80桁目安で複数行化する。
  Mermaid等のfenced code blockはコードブロックとしてそのまま表示する。
- 先頭のYAML front matterはGitHub風にkey/valueの等幅テーブルとして表示し、
  長い値は同じく複数行化する。

## キー入力処理の再設計 (KeyDown/KeyUp発火の廃止)
- 現行仕様(活性化モデル・arm-gate・依存するFyne挙動の再検証ポイント・
  トレードオフ)は `docs/architecture/ui-input.md` に集約した。
  設計検討の経緯はgit履歴(PR #5、旧`docs/todo-keyboard.md`)にある。
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
