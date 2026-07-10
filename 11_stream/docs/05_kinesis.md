# 05_kinesis: shard 制約と Kafka 対応 — マネージドされた stream の切り方

`01_concepts.md` は stream を「消費されても消えない追記ログ」と定義し、`04_kafka.md` では Kafka のログを partition・offset・consumer group という 3 つの概念に分解した。本ドキュメントでは、同じ stream モデルに立つマネージドサービス Kinesis Data Streams が、この 3 概念をどう再構成しているか — partition ではなく shard、consumer group ではなく KCL + DynamoDB checkpoint という「ブローカー機能をクライアント側へ肩代わりさせる」設計 — を `apps/kinesis/` の実コードと `make demo-kinesis` の実行結果に沿って掘り下げる。partition / offset / consumer group という用語自体の定義は `04_kafka.md` のものをそのまま使うので、ここでは再定義しない。

---

## 1. イントロ — 同じ stream でも、切り方が違う

Kinesis Data Streams はしばしば「マネージドな Kafka」と紹介されるが、この言い方は半分しか正しくない。「消費しても消えない追記ログ」という stream のデータモデルは確かに共通している。しかし AWS がマネージドサービスとして外に見せる抽象の単位は、Kafka とは異なる切り方をしている。両者の対応関係を整理すると次のようになる。

| Kafka | Kinesis | 備考 |
|---|---|---|
| partition | shard | スケール単位（スループットの器） |
| offset | sequence number | ログ内の位置を示す値 |
| key | partition key | ハッシュ値で書き込み先を決定 |
| consumer group | KCL + DynamoDB checkpoint | consumer group はブローカー機能、KCL はクライアント側ライブラリ |
| retention（実質無制限に設定可） | 24h（既定）〜365d（追加課金で延長） | |

一見きれいに 1 対 1 で対応しているように見えるが、実際に手を動かして違いが際立つのは 2 か所である。1 つは shard の制約がそのままスループットの天井になること（セクション 2）、もう 1 つは consumer group が Kinesis 本体には存在せず、クライアント側の責務にすり替わっていること（セクション 3）である。

本ハンズオンの CLI も Kafka 版と同じく `produce` / `consume` の 2 サブコマンドで構成され（[apps/kinesis/main.go:58](../apps/kinesis/main.go#L58)-[62](../apps/kinesis/main.go#L62)）、実装には aws-sdk-go-v2 の `kinesis` クライアントを使っている。LocalStack に接続できない場合は「`make up` を先に実行してください」という具体的なヒント付きのエラーを返す点も他の章と同様である（[apps/kinesis/main.go:45](../apps/kinesis/main.go#L45)-[50](../apps/kinesis/main.go#L50)）。ストリーム `orders` は起動時の init スクリプトで shard 数 1 として作成される（[localstack-init/init-aws.sh:15](../localstack-init/init-aws.sh#L15)）。

---

## 2. shard の制約が全てを決める — per shard の固定枠

Kinesis の shard は Kafka の partition と違い、単なる「分割単位」ではなく **スループットの契約そのもの** である。1 つの shard には次の固定枠が付いている。

- **書き込み**: 1 shard あたり 1 MB/s、または 1,000 records/s のいずれか早く到達した方が上限
- **読み込み**: 1 shard あたり 2 MB/s。これは **その shard を読む全コンシューマの合計** に対する枠であり、1 コンシューマ専用ではない
- **GetRecords 呼び出し回数**: 1 shard あたり 5 回/秒まで

produce 側は `PutRecord` の `PartitionKey` に注文 ID を渡しており、Kafka の key と同じ役割を果たす。

```go
// PartitionKey が shard を決める（Kafka の key ≈ PartitionKey）
out, err := client.PutRecord(ctx, &kinesis.PutRecordInput{
	StreamName:   aws.String("orders"),
	PartitionKey: aws.String(o.ID),
	Data:         body,
})
```
（[apps/kinesis/producer.go:26](../apps/kinesis/producer.go#L26)-[31](../apps/kinesis/producer.go#L31)）

Kinesis は `PartitionKey` をハッシュ化し、そのハッシュ値が属する範囲を担当する shard へレコードを振り分ける。今回のストリームは shard 数 1 なので全レコードが同じ `shardId-000000000000` に集まるが、shard 数を増やせば Kafka の murmur2 ハッシュと同様、key（PartitionKey）単位で書き込み先が分散する。

consume 側の `GetRecords` ループには、この「5 回/秒」という上限を意識した sleep が入っている。

```go
iter = out.NextShardIterator
if len(out.Records) == 0 {
	// 実 AWS では GetRecords は shard あたり 5 回/秒まで。ポーリング間隔を空ける
	time.Sleep(500 * time.Millisecond)
}
```
（[apps/kinesis/consumer.go:56](../apps/kinesis/consumer.go#L56)-[60](../apps/kinesis/consumer.go#L60)）

LocalStack はこの上限を実際には強制していないが、このコードは「本番の AWS Kinesis に向けて動かしたときにスロットリングされない」ことを前提にした防御的な実装になっている。空振り（レコード 0 件）のたびに 500ms 待てば 1 秒あたりの呼び出し回数は最大でも 2 回程度に収まり、5 回/秒の枠に十分な余裕を残す。

ここで重要なのは、読み込みの 2 MB/s は **shard 単位** であって **コンシューマ単位** ではないという点である。同じ shard を 2 つのアプリケーションが読もうとすると、2 MB/s を分け合うことになり、片方のスループットがもう片方を圧迫する。この制約を避けたい場合に使うのが **Enhanced Fan-Out（EFO）** である。EFO に登録したコンシューマは、それぞれ専用に 2 MB/s の帯域を持ち、GetRecords によるポーリング（pull）ではなく HTTP/2 経由でレコードが push される。ただし EFO はコンシューマ数に応じた追加コストがかかる有償オプションであり、本ハンズオンの `consumer.go` は素朴な GetRecords ポーリングのみを実装している。

---

## 3. consumer group がないことの意味 — checkpoint はクライアントの責務

`04_kafka.md` セクション 3 で見た consumer group は、Kafka ブローカー自身が「どの group がどこまで読んだか」を committed offset として管理する機能だった。Kinesis にはこれに相当するブローカー側の機能が存在しない。

本ハンズオンの consumer は shard を列挙し、`iterator` フラグで指定した位置（`TRIM_HORIZON` または `LATEST`）から shard iterator を取得するだけである。

```go
// Kafka と違い consumer group はない。shard を列挙し iterator を自分で管理する
ls, err := client.ListShards(ctx, &kinesis.ListShardsInput{StreamName: aws.String("orders")})
if err != nil {
	return connectHint(err)
}
itOut, err := client.GetShardIterator(ctx, &kinesis.GetShardIteratorInput{
	StreamName:        aws.String("orders"),
	ShardId:           ls.Shards[0].ShardId,
	ShardIteratorType: types.ShardIteratorType(*iterType),
})
```
（[apps/kinesis/consumer.go:27](../apps/kinesis/consumer.go#L27)-[36](../apps/kinesis/consumer.go#L36)）

この実装には「前回どこまで読んだか」を記録する状態が一切ない。DynamoDB もローカルファイルも使っていない。だから毎回の実行は必ず `TRIM_HORIZON`（ストリームの先頭）か `LATEST`（実行時点の末尾）のどちらかから始まり、前回の続きから読むという概念自体が存在しない。この挙動はセクション 5 の実行結果でそのまま確認できる。

実務でこの「どこまで読んだか」を扱うのが **KCL（Kinesis Client Library）** である。KCL はコンシューマのアプリケーション名に対応する DynamoDB テーブルを自動作成し、shard ごとに「どのワーカーが担当しているか（リース）」と「どの sequence number まで処理済みか（checkpoint）」を書き込む。複数ワーカーでの shard の分担や、shard の分割・統合（resharding）に伴うリースの引き継ぎも KCL が面倒を見る。つまり Kafka では consumer group という **ブローカー機能** だった「読み進み位置の管理」が、Kinesis では KCL + DynamoDB という **クライアント側ライブラリと利用者が所有するテーブル** に置き換わっている。ブローカー（Kinesis 本体）はレコードを保持して配るだけで、「誰がどこまで読んだか」には一切関与しない。

---

## 4. オンデマンド vs プロビジョンド — shard 管理を AWS に預けるか、自分で握るか

shard はスループットの器であると同時に、コストと運用負担の単位でもある。Kinesis Data Streams にはこの shard をどちらが管理するかで 2 つのモードがある。

- **プロビジョンドモード**: shard 数を利用者が明示的に指定する。本ハンズオンの `awslocal kinesis create-stream --stream-name orders --shard-count 1`（[localstack-init/init-aws.sh:15](../localstack-init/init-aws.sh#L15)）はこのモードで、実際に `describe-stream-summary` を叩くと `"StreamMode": "PROVISIONED"` が返る（実測）。課金は shard 時間単位と、書き込みデータ量に応じた PUT payload unit の組み合わせになる。トラフィックが増えれば shard を split して増やし、減れば merge して減らす resharding を利用者自身が判断・実行しなければならない。
- **オンデマンドモード**: shard 数を AWS が自動的に調整する。利用者は shard を意識せず、書き込み・読み込みのデータ量（GB）に応じて課金される。

どちらを選ぶべきかは、Kafka のセルフホスト運用と Amazon MSK のようなマネージドサービスを比較する `07_selection_guide.md` の議論に直結する伏線になっている。トラフィックの見通しが立たない立ち上げ期は、shard 数を読み違えてスループット不足やコスト過剰を招くリスクがあるオンデマンドの方が安全に倒せる。逆に定常トラフィックの水準が見えてきた段階では、shard 数を実測値に合わせて自分で握るプロビジョンドの方が、同じスループットをより安く確保できる。「最初はオンデマンド、定常化したらプロビジョンドへ」という移行が典型的な運用パターンになる。

---

## 5. make demo-kinesis の実行手順と観察ポイント

スタックが起動していない場合は `make up` を先に実行する。基本の produce / consume は次で確認する（[Makefile:54](../Makefile#L54)-[56](../Makefile#L56)）。

```bash
make demo-kinesis
```

これは `produce -n 5` で 5 件書き込んだ後、`-iterator TRIM_HORIZON -max 5` で consume する。実行結果は次の通り（実測、1 回目）。

```
sent: shard=shardId-000000000000 seq=49676285433625005722605852880929566957920310041325338626 {"id":"order-0001", ...}
sent: shard=shardId-000000000000 seq=49676285433625005722605852880930775883739924670500044802 {"id":"order-0002", ...}
sent: shard=shardId-000000000000 seq=49676285433625005722605852880931984809559539299674750978 {"id":"order-0003", ...}
sent: shard=shardId-000000000000 seq=49676285433625005722605852880933193735379153928849457154 {"id":"order-0004", ...}
sent: shard=shardId-000000000000 seq=49676285433625005722605852880934402661198768558024163330 {"id":"order-0005", ...}
received: seq=49676285433625005722605852880929566957920310041325338626 {"id":"order-0001", ...}
received: seq=49676285433625005722605852880930775883739924670500044802 {"id":"order-0002", ...}
received: seq=49676285433625005722605852880931984809559539299674750978 {"id":"order-0003", ...}
received: seq=49676285433625005722605852880933193735379153928849457154 {"id":"order-0004", ...}
received: seq=49676285433625005722605852880934402661198768558024163330 {"id":"order-0005", ...}
```

観察ポイントの 1 つ目は `shard=` が全レコードで `shardId-000000000000` に固定されていることである。ストリームの shard 数が 1 なので、Kafka のように書き込み先が複数 partition に分かれることはない。2 つ目は `seq=` が Kafka の offset のような単純な連番ではなく、巨大な数値文字列であることである。sequence number は shard 内の順序を保証する値ではあるが、offset のように「先頭から何番目か」を直接表す添字ではない。

もう一段深い観察は、**同じコマンドをもう一度実行するとどうなるか** である。セクション 3 で述べた通り、この consumer は checkpoint を持たないため、2 回目の `make demo-kinesis` も produce 後に `TRIM_HORIZON` から consume する。つまり新しく書いた 5 件だけでなく、1 回目に書いた 5 件も含めてストリームの先頭から読み直す。実際に連続実行すると次のようになる（実測、2 回目）。

```
sent: shard=shardId-000000000000 seq=49676285433625005722605852880935611587018383462076776450 {"id":"order-0001", ...}
sent: shard=shardId-000000000000 seq=49676285433625005722605852880936820512837998091251482626 {"id":"order-0002", ...}
sent: shard=shardId-000000000000 seq=49676285433625005722605852880938029438657612720426188802 {"id":"order-0003", ...}
sent: shard=shardId-000000000000 seq=49676285433625005722605852880939238364477227349600894978 {"id":"order-0004", ...}
sent: shard=shardId-000000000000 seq=49676285433625005722605852880940447290296841978775601154 {"id":"order-0005", ...}
received: seq=49676285433625005722605852880929566957920310041325338626 {"id":"order-0001", ...}
received: seq=49676285433625005722605852880930775883739924670500044802 {"id":"order-0002", ...}
received: seq=49676285433625005722605852880931984809559539299674750978 {"id":"order-0003", ...}
received: seq=49676285433625005722605852880933193735379153928849457154 {"id":"order-0004", ...}
received: seq=49676285433625005722605852880934402661198768558024163330 {"id":"order-0005", ...}
received: seq=49676285433625005722605852880935611587018383462076776450 {"id":"order-0001", ...}
received: seq=49676285433625005722605852880936820512837998091251482626 {"id":"order-0002", ...}
received: seq=49676285433625005722605852880938029438657612720426188802 {"id":"order-0003", ...}
received: seq=49676285433625005722605852880939238364477227349600894978 {"id":"order-0004", ...}
received: seq=49676285433625005722605852880940447290296841978775601154 {"id":"order-0005", ...}
```

`received` は 10 行あり、前半 5 行（`seq=...929...`〜`...934...`）は 1 回目の produce で書いた分、後半 5 行（`seq=...935...`〜`...940...`）は今回の produce で書いた分である。`-max 5` という指定にもかかわらず 10 件受信して終了している。これは `GetRecords` を 1 回呼ぶだけで `Limit: 100`（[apps/kinesis/consumer.go:47](../apps/kinesis/consumer.go#L47)）の枠内に収まる件数がまとめて返ってくるためで、受信件数が `-max` を超えたかどうかのチェックは 1 レコードずつではなくバッチ単位でしか行われない。この「意図せず超過する」挙動自体も、checkpoint を持たない実装の素朴さを表している。

この結果は 2 つのことを裏付けている。1 つは、TRIM_HORIZON は「ストリームの先頭（保持期間内で最も古いレコード）」を指すのであって「前回読んだ続き」ではないこと。もう 1 つは、Kinesis が SQS のような queue ではなく Kafka と同じ stream であり、消費してもレコードが消えないことである。もし「新しく書いた分だけ」を読みたいなら `-iterator LATEST`（[apps/kinesis/consumer.go:17](../apps/kinesis/consumer.go#L17)）を使えばよいが、それでも前回の実行がどこまで読んだかを覚えているわけではなく、あくまで「今この瞬間の末尾」から読み始めるだけである点に変わりはない。
