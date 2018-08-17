# AttendanceBot 
[![Build Status](https://travis-ci.org/kishikawakatsumi/attendancebot.svg?branch=master)](https://travis-ci.org/kishikawakatsumi/attendancebot)

## Getting Started
まず、Freeeにログインして自分のFreeeにおける従業員IDを調べます。
従業員情報タブなどに移動して、URLの上で赤く囲った部分です。私の場合は`333233`です。

<kbd><img width="850" alt="Employee ID" src="https://user-images.githubusercontent.com/40610/44285081-e9739a00-a29e-11e8-9936-edb04435c0e3.png"></kbd>

次にDirectMessageから`attendancebot`を探して、

<kbd><img width="450" alt="Direct Message" src="https://user-images.githubusercontent.com/40610/44285077-e8db0380-a29e-11e8-892c-a07dd13a6cd6.png"></kbd>

`attendancebot`とDMを開始します。
`attendancebot`はDMのメッセージにのみ反応するように作られています。

ユーザーを識別する必要があることと、
似たような時間にBotへの命令が複数から行われるはずなので、チャンネル内がその発言で埋め尽くされてしまわないように、という意図があります。

DMで、
```
add [従業員ID]
```
（私の場合は`add 333233`）

<kbd><img width="850" alt="Register" src="https://user-images.githubusercontent.com/40610/44285080-e8db0380-a29e-11e8-9dbb-e79d2c776691.png"></kbd>

と、話しかけるとBotにユーザーが登録されます（SlackのユーザーIDとFreeeの従業員IDをマッピングします）。
これでBotからFreeeに記録できるようになります。

そして自動的にリマインダーが送られるようになります。それ以降、一日２回、朝と夕方にDMでリマインダーが届きます。

<kbd><img width="400" alt="Reminder" src="https://user-images.githubusercontent.com/40610/44285445-50458300-a2a0-11e8-8912-eeccd6fdc231.png"></kbd>

当初に想定しているワークフローでは、BotがDMで朝と夕方に出勤退勤のボタンを自動的に表示するので、それに対応するだけで毎日の記録が完了します。

会社に着いたらDMが来ているので記録、帰るころにDMが来るので適当なときに記録、というワークフローです。

基本的に出勤退勤の記録について考えたくない、受動的にやりたいという人をある程度救えるだろうという発想で作られています。

**注意: FreeeのサイトのHomeに表示される「出勤する」ボタンは反映が遅いので、記録されたかどうかの確認は「勤怠」タブのカレンダーを見てください。**

このワークフローはかえって面倒である、という方にはコマンド形式の命令もサポートしています。
```
in now
in 0930
out now
out 1850
```
のように話しかけると、その時刻で記録されます。
欠勤は`off`または`leave`です。


登録の解除は`remove`です。それ以降リマインダーは送られません。Slackから記録することもできなくなります。

再登録は同じ手順でいつでもできます。


`help`と話しかけると、ヘルプメッセージを表示します。

```
Usage:
    Integration:
        auth
        add [emp_id]

    Deintegration
        remove

    Check In:
        in
        in now
        in 0930

    Check Out:
        out
        out now
        out 1810

    Off:
        leave
        off

    Reminder:
        reminder set 0900 1700
        reminder off

    Report:
        report
        report -json
        report -json -incomplete

    Bulk Update:
        update [
                 {"date":"2018-08-17","in":"09:30","out":"19:20"},
                 {"date":"2018-08-20","in":"1015","out":"2040"},
                 {"date":"2018-08-21","off":true}
               ]
```

## リマインダーのカスタマイズ
`reminder set 0900 1700`のように入力すると、リマインダーの時間を変更できます。

リマインダーを送らないようにするには、`reminder off`と入力します。

## １か月ぶんの記録をみる
`report`と入力すると、１か月ぶんの記録が表示されます。

```
Date        In     Out    Off
----------  -----  -----  ---                
...                 
2018/08/15  09:09  18:09     
2018/08/16  09:29  18:00     
2018/08/17  11:34  19:33     
2018/08/18                 * 
2018/08/20                   
...    
```

Offのカラムに`*`マークがある日は欠勤として記録されています。`*`マークがなく、時間が表示されていない日は未入力です。

`report -json`と`-json`オプションを追加すると、結果をJSON形式で表示します。

```
[
  ...
  {"date":"2018-08-15","in":"2018-08-15T09:09:40.000+09:00","off":false,"out":"2018-08-15T18:09:40.000+09:00"},
  {"date":"2018-08-16","in":"2018-08-16T09:29:51.000+09:00","off":false,"out":"2018-08-16T18:00:49.000+09:00"},
  {"date":"2018-08-17","in":"2018-08-17T09:30:00.000+09:00","off":false,"out":"2018-08-17T19:20:00.000+09:00"},
  {"date":"2018-08-18","in":null,"off":true,"out":null},
  {"date":"2018-08-20","in":"2018-08-20T10:15:00.000+09:00","off":false,"out":"2018-08-20T20:40:00.000+09:00"},
  ...
]
```

さらに`report -json -incomplete`と`-incomplete`オプションを追加すると、未入力のデータだけを表示します。

**注意: FreeeのAPIリクエストは１時間に5000回のレートリミットが設定されています。１日のレコードを取得するために１回のリクエストが必要です。**

**つまり、１か月ぶんの記録を表示するために28〜31回のリクエストが送られます。reportコマンドを何度も連続して使用しないように気をつけてください。**

## Bulk Update
`update`コマンドで任意の日付のデータを更新できます。コマンドに続けてJSON形式でデータを渡します。
（例）
```
update
[
  {"date":"2018-08-17","in":"09:30","out":"19:20"},
  {"date":"2018-08-20","in":"1015","out":"2040"},
  {"date":"2018-08-21","off":true}
]
```

**注意: FreeeのAPIリクエストは１時間に5000回のレートリミットが設定されています。１日のレコードを更新するために１回のリクエストが必要です。**

**あまり多くの日付を一度に更新しないように気をつけてください。**
