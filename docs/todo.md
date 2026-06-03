# 実用になるために必要なToDo

## KeyDown/KeyUp系キーバインドのrepeat適性を棚卸ししたい
- Fyne/GLFWではキーリピートがKeyDownではなくTypedKey/TypedShortcut側へ流れる。
- repeatしてほしい操作はtyped/TypedShortcutへ寄せ、長押しで増殖して困る操作はdown/upに残す。
- Fyneのイベント順は概ね KeyDown -> TypedKey/TypedShortcut -> TypedRune -> KeyUp。
  KeyDown/KeyUpを全廃してTyped系へ完全移行するには、現行の`KeyName + modifier` bindingを
  TypedRune/TypedKey/TypedShortcutのどれで解決するか分類する必要がある。
- 文字キーをTypedKeyで即実行すると、同じ打鍵由来のTypedRuneが後から新しいdialogへ漏れる可能性がある。
  完全移行するなら、文字系bindingは「TypedKeyで候補を記録し、続くTypedRuneで確定する」ような
  pending/commit方式が必要になりそう。
- Ctrl/Alt系はTypedShortcutへ寄せられるが、Fyne組み込みのShortcutCopy/SelectAll/Cut/Paste等に
  畳まれる組み合わせがあり、`Ctrl+C`と`Ctrl+Insert`のように区別できないケースがある。
- 以上から、大規模なKeyDown/KeyUp全廃は複雑さの置き場所が変わるだけの可能性が高い。
  当面は全移行ではなく、repeatが必要な操作だけtyped/TypedShortcutへ寄せる方針が費用対効果よさそう。

## 簡易viewerのText表示をEntry以外に置き換えたい
- 64KiB程度の表示でもFyneのmulti-line Entryは初期レイアウトが重い。
- Text/Hex表示と移動だけはTextGrid PoCへ置き換えた。
- 長い行は横スクロールを標準にし、折り返し表示へ切り替えられるようにした。
- Text/Hex表示のマウスドラッグ選択とコピーはTextGrid上で対応した。
- Text/Hex表示のliteral検索はTextGrid上で対応した。
- 未対応: Text/Hex表示のキーボード選択、Markdown高速化。

## ファイルコピーの高速化を検討したい
- 現状のCopy/Moveは進捗表示のために、汎用的なread -> writeループでコピーしている。
- Linuxでは`os.File.ReadFrom`経由で`copy_file_range`/`splice`を使える可能性があり、
  ローカル通常ファイル同士ならユーザ空間へデータを持ってこない高速化が期待できる。
- Windowsでは現時点のGo標準`os.File.ReadFrom`にfile-to-file向けの独自高速化はなさそうなので、
  Windows高速化は別途調査が必要。
- ただしSMB、HDD、archive、progress表示との相性があるため、過度な並列化は避けたい。
- 進捗表示は必須。高速化案はチャンク単位などで現在ファイルの進捗を維持できることを条件にする。

# DONE 以下は一応終わったもの
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
