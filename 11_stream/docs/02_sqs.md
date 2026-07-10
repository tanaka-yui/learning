# 02_sqs: マネージド queue の消費ループと DLQ

`01_concepts.md` では queue を「ブローカーが配って消す」モデルとして位置づけ、SQS がその代表例だと述べた。本ドキュメントでは SQS 固有の設計 — standard / FIFO の違い、可視性タイムアウトによるロック機構、DLQ による毒メッセージの隔離、long polling によるコスト最適化 — を `apps/sqs/` の実コードと `make demo-sqs` / `make demo-sqs-dlq` の実行結果に沿って掘り下げる。at-least-once や冪等性といった用語は `01_concepts.md` の定義をそのまま使うので、ここでは再定義しない。

---

## 1. イントロ — サーバーを 1 台も持たない queue

SQS を使うのに、こちらが管理するサーバーは 1 台もない。ブローカーのプロセスを起動する必要も、ディスク容量やレプリケーションを気にする必要もなく、必要なのは API エンドポイントを叩くことだけである。しかもその API は驚くほど小さい。本ハンズオンのコードが実際に呼んでいる SQS API は次の 3 つだけである。

- `SendMessage` — キューにメッセージを 1 通置く（[apps/sqs/producer.go:30](../apps/sqs/producer.go#L30)）
- `ReceiveMessage` — キューからメッセージを受け取る（[apps/sqs/consumer.go:31](../apps/sqs/consumer.go#L31)）
- `DeleteMessage` — 受け取ったメッセージを消費済みとして削除する（[apps/sqs/consumer.go:48](../apps/sqs/consumer.go#L48)）

この 3 API だけで「積む・取る・消す」という queue の本質的な操作がすべて表現できる、というのが SQS の設計思想である。裏側でブローカーが何台構成でどう冗長化されているかは AWS 側の関心事であり、利用者は「メッセージがどこかに確実に届く」という契約だけを信頼して API を呼べばよい。この小ささこそが、後述する visibility timeout や DLQ という一見高度な機能が、実は上記 3 API のパラメータと組み合わせの工夫だけで実現されていることの裏付けになっている。

---

## 2. standard vs FIFO

SQS には standard キューと FIFO キューの 2 種類があり、順序保証・スループット・重複排除のトレードオフが逆になる。

**standard キュー**（本ハンズオンの `orders` はこちら）は順序保証を持たない。メッセージがほぼ同時に複数のバックエンドパーティションに分散配置されるため、送った順に受信される保証はなく、まれに同じメッセージが 2 回配信されることもある（= at-least-once）。その代わりスループットに実質的な上限がなく、必要なだけ producer / consumer を並べてスケールできる。

**FIFO キュー**（キュー名が `.fifo` で終わる）は `MessageGroupId` を指定することで、同一グループ内のメッセージ順序を厳密に保証する。ただしスループットには上限があり、バッチを使わない場合は 1 キューあたり 300 TPS、`SendMessageBatch` などのバッチ API を使えば 3,000 TPS まで引き上げられる。この上限をさらに超えたい場合は High Throughput FIFO mode を有効にし、`MessageGroupId` を多数に分散させることでリージョン単位の上限近くまでスケールできるが、その代わりに「同一グループ内の順序」という保証の粒度は変わらない。

重複排除（deduplication）も FIFO キュー固有の機能である。`MessageDeduplicationId`（または content-based dedup でメッセージ本文のハッシュ）を使うと、5 分間の重複排除ウィンドウ内で同一 ID のメッセージは 1 通に丸められる。standard キューにはこの仕組みがなく、再送によって生じた重複は producer 側では防げない。ゆえに standard キューを使う本ハンズオンの `orders` は、`01_concepts.md` で述べた「コンシューマ側の冪等化」がそのまま設計の前提になる。

順序も重複排除も要らず、スループットだけが欲しいジョブ配布（本ハンズオンの用途）には standard キューが素直に嵌まる。一方「注文の状態遷移を順番通りに処理したい」といった要求が出たら、スループットの天井と引き換えに FIFO キューを検討することになる。

---

## 3. visibility timeout の仕組み — 受信はロックであり削除ではない

SQS を理解するうえで最も重要な発想の転換は、「`ReceiveMessage` はメッセージを取り出す操作ではなく、一時的にロックする操作である」という点である。メッセージはキューから消えるのではなく、一定時間「他のコンシューマから見えない」状態（invisible）になるだけであり、その間に明示的に `DeleteMessage` を呼ばない限り、時間切れとともに再び見える状態に戻る。時系列で書くと次のようになる。

```
t=0    consumer が ReceiveMessage を呼ぶ
         └ メッセージが visible → invisible に遷移（可視性タイムアウトのカウント開始）
t=0〜5  他のコンシューマからはこのメッセージが見えない（orders キューは VisibilityTimeout=5 秒）
t=5    DeleteMessage が呼ばれていなければ invisible → visible に自動で戻る
         └ 次の ReceiveMessage で再び配信される（redelivery）
```

このロック機構はコード上で `--no-delete` フラグとして切り替えられるようになっている。

```go
noDelete := fs.Bool("no-delete", false, "receive without deleting (visibility timeout demo)")
```
（[apps/sqs/consumer.go:16](../apps/sqs/consumer.go#L16)）

受信ループの中で、このフラグが立っていない時だけ `DeleteMessage` を呼ぶ。

```go
if !*noDelete {
    if _, err := client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
        QueueUrl:      urlOut.QueueUrl,
        ReceiptHandle: m.ReceiptHandle,
    }); err != nil {
        return connectHint(err)
    }
}
```
（[apps/sqs/consumer.go:47](../apps/sqs/consumer.go#L47)）

`--no-delete` を付けて実行すると、メッセージは受信されても削除されないため、`VisibilityTimeout`（`orders` キューは 5 秒、[localstack-init/init-aws.sh:11](../localstack-init/init-aws.sh#L11)）が経過した後に同じメッセージが再びキューに現れる。ここで押さえておきたいのは、**「受信 (Receive)」と「削除 (Delete)」が別の API 呼び出しとして分離されていること自体が at-least-once の実装そのものである** という点だ。もしこの 2 つが 1 つの不可分な操作（受け取ったら即座に消える）だったら、コンシューマがメッセージを受け取った直後、処理を終える前にクラッシュした場合にそのメッセージは永遠に失われてしまう。Receive と Delete を分けておくことで、「処理が終わって初めて Delete を呼ぶ」という運用が可能になり、途中でクラッシュしても未削除のメッセージは可視性タイムアウト経過後に再配信される。再配信は重複を生むが、消失は起きない — これが `01_concepts.md` で定義した at-least-once を SQS が実現している具体的な仕組みである。

---

## 4. DLQ と maxReceiveCount — 毒メッセージの隔離

再配信は消失を防ぐが、そのメッセージ自体に問題があって毎回処理が失敗する「毒メッセージ（poison message）」の場合、再配信が無限に繰り返されてしまう。これを防ぐのが Dead Letter Queue（DLQ）と `maxReceiveCount` である。

本ハンズオンの初期化スクリプトは、DLQ を先に作成し、その ARN を本体キュー `orders` の `RedrivePolicy` に埋め込んでいる。

```bash
awslocal sqs create-queue --queue-name orders --attributes "{
  \"VisibilityTimeout\": \"5\",
  \"RedrivePolicy\": \"{\\\"deadLetterTargetArn\\\":\\\"${DLQ_ARN}\\\",\\\"maxReceiveCount\\\":\\\"2\\\"}\"
}"
```
（[localstack-init/init-aws.sh:10](../localstack-init/init-aws.sh#L10)-[12](../localstack-init/init-aws.sh#L12)）

`maxReceiveCount=2` は「このメッセージの受信回数（`ApproximateReceiveCount`）が 2 を超えたら、本体キューへは配信せず DLQ (`orders-dlq`) へ移送する」という意味である。`make demo-sqs-dlq` はこれを実際に発生させるシナリオで、実行すると次の出力が得られる（実測、[Makefile:16](../Makefile#L16)-[23](../Makefile#L23)）。

```
go run ./apps/sqs produce -n 1
sent: {"id":"order-0001","item":"book","amount":1001,"created_at":"2026-07-10T11:04:21+09:00"}
go run ./apps/sqs consume -max 1 --no-delete
received (receiveCount=1): {"id":"order-0001", ...}
sleep 6
go run ./apps/sqs consume -max 1 --no-delete
received (receiveCount=2): {"id":"order-0001", ...}
sleep 6
go run ./apps/sqs consume -max 1
error: expected 1 messages, got 0
go run ./apps/sqs consume -max 1 --queue orders-dlq
received (receiveCount=3): {"id":"order-0001", ...}
```

観察すべきポイントは 3 回目の `consume` である。1 回目・2 回目の受信はいずれも `--no-delete` で削除しないため、可視性タイムアウト（5 秒）経過後に同じメッセージが再配信され `receiveCount` が 1 → 2 と増える。ここまでは受信回数がまだ `maxReceiveCount=2` に達していないため、メッセージは本体キューに留まる。ところが **3 回目の受信試行そのものが `maxReceiveCount=2` を超える**ため、SQS はこのタイミングでメッセージを `orders` から `orders-dlq` へ移送し、3 回目の `consume` には何も返さない。そのため `orders` に対する `consume -max 1` は「1 通期待して 0 通しか取れなかった」として `expected 1 messages, got 0` エラーで終了する。この行は Makefile 上で `-`（先頭ハイフン）付きで実行され（[Makefile:22](../Makefile#L22)）、意図的にこのエラーを無視するようになっている。

最後に `orders-dlq` を直接 `consume` すると、移送されたメッセージが得られる。ここで実測される値が **`receiveCount=3`** であることに注意したい。DLQ へ移動しても `ApproximateReceiveCount` は 0 にリセットされず、本体キューでの受信回数を引き継いだ累積値のまま現れる。これは「DLQ に届いた時点で過去何回配信を試みたか」という情報がそのまま保たれるということであり、障害調査や再処理の判断材料になる。

---

## 5. long polling — 空振り API 課金を減らす

`ReceiveMessage` にはもう 1 つ重要なパラメータがある。

```go
out, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
    QueueUrl:                    urlOut.QueueUrl,
    MaxNumberOfMessages:         10,
    WaitTimeSeconds:             2, // long polling
    MessageSystemAttributeNames: []types.MessageSystemAttributeName{types.MessageSystemAttributeNameApproximateReceiveCount},
})
```
（[apps/sqs/consumer.go:31](../apps/sqs/consumer.go#L31)-[36](../apps/sqs/consumer.go#L36)）

`WaitTimeSeconds` を 0（デフォルト）にすると short polling になり、その瞬間にメッセージがなければ即座に空の応答が返る。コンシューマが「メッセージが来るまでポーリングし続けたい」場合、short polling では高頻度にリクエストを送り続けることになり、そのほとんどが空振りに終わる。SQS はメッセージの有無に関わらず `ReceiveMessage` の呼び出し自体に課金するリクエスト課金モデルであるため、空振りのポーリングもそのままコストになる。

`WaitTimeSeconds` を正の値（本ハンズオンでは 2 秒）にすると long polling になり、SQS はその秒数の間、少なくとも 1 通のメッセージが到着するかタイムアウトするまで応答を保留する。これにより「メッセージがないのに空応答を返すためだけのリクエスト」の回数が減り、同じ待受時間をより少ないリクエスト回数でカバーできる。これは単なる実装上の行儀の良さではなく、SQS がリクエスト単位で課金される以上、直接コストに跳ね返ってくる設計判断である。この「サービスごとに課金の単位が違う」という視点は、07 章の選定ガイドで他のミドルウェアと比較する際の伏線になる。

---

## 6. make demo-sqs / demo-sqs-dlq の実行手順と観察ポイント

スタックが起動していない場合は `make up` を先に実行する（`docker compose up -d` とヘルスチェック待ち、[Makefile:3](../Makefile#L3)-[5](../Makefile#L5)）。その後、基本の送受信は次で確認する。

```bash
make demo-sqs
```

これは 5 通を `produce` し、そのまま `consume -max 5` で受け取るだけの最短経路である（[Makefile:10](../Makefile#L10)-[12](../Makefile#L12)）。実行結果は次の通りで、すべて `receiveCount=1` のまま削除される（デフォルトは `--no-delete` を付けないため、受信後に即 `DeleteMessage` される）。

```
sent: {"id":"order-0001", ...}
...(5 通)
received (receiveCount=1): {"id":"order-0001", ...}
...(5 通、いずれも receiveCount=1)
```

DLQ と可視性タイムアウトの挙動を観察するには次を実行する。

```bash
make demo-sqs-dlq
```

観察すべきポイントは 3 つである。1 つ目は `receiveCount` が `--no-delete` の繰り返しで 1 → 2 と増えていくこと（セクション 3 の可視性タイムアウトの復習）。2 つ目は 3 回目の受信試行が空振りし、`expected 1 messages, got 0` として（意図的に無視される形で）失敗すること — これは「メッセージが消えた」のではなく「DLQ へ移送された」ことのサインである（セクション 4）。3 つ目は `orders-dlq` から読み出した際の `receiveCount=3` が、本体キューでの受信回数を引き継いだ累積値であること。この 3 点をログの数字として自分の目で追えることが、visibility timeout と DLQ が単なる概念ではなく実際に手を動かして確認できる仕組みであることの理解につながる。
