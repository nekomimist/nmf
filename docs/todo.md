# 実用になるために必要なToDo

## 絞り込み系のキーバインドを改善したい
- Readlineほどのバインドは不要だが、History等の絞り込み系の欄もCtrl-HがBSになるくらいはほしい

## KeyDown/KeyUp系キーバインドのrepeat適性を棚卸ししたい
- Fyne/GLFWではキーリピートがKeyDownではなくTypedKey/TypedShortcut側へ流れる。
- repeatしてほしい操作はtyped/TypedShortcutへ寄せ、長押しで増殖して困る操作はdown/upに残す。
- History/Filter/DirectoryJump/CopyMoveなどの検索系でCtrl-HをBackspace相当にする余地がある。

## OK/Cancel的な二択ボタン
- あまり統一感がないかもしれない。CancelIconとConfirmIcon をつけて、Confirmのほうのアイコン色を
  ThemeのPrimaryColorにした上でボタンサイズを揃えたほうがFyneのアプリっぽいかもしれない。
  (優先度低)

# DONE 以下は一応終わったもの
## 全ファイルマークできるコマンドを追加
- `selection.markAll` を追加。
- デフォルトキーバインドはC-A。
- `..` と削除済み表示は対象外。

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
