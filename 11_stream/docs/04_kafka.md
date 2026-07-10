# 04_kafka: 分散追記ログのパーティション・オフセット・コンシューマグループ

`01_concepts.md` は stream を「消費されても消えない追記ログ」と定義し、その代表例として Kafka を挙げた。本ドキュメントでは Kafka 固有の設計 — topic を分割する partition と、コンシューマが自ら管理する offset、group という単位でパーティションを分担しあるいは独立して読む consumer group、そして offset を巻き戻すことで実現する replay — を `apps/kafka/` の実コードと `make demo-kafka` / `make demo-kafka-replay` の実行結果に沿って掘り下げる。at-least-once や冪等性といった用語は `01_concepts.md` の定義をそのまま使うので、ここでは再定義しない。

---

## 1. イントロ — Kafka は queue ではなく「分散追記ログ」

SQS も ActiveMQ も、メッセージは消費（ACK / delete）された瞬間にブローカーから消える「配って消す」queue だった。Kafka はそれとは根本的に異なるデータモデルに立っている。topic の実体は末尾にしかレコードを追加できない追記専用のログであり、consumer がレコードを読み取っても、そのレコード自体はログから削除されない。レコードは topic ごとに設定された retention（保持期間・保持サイズ）に達するまでログに残り続け、その間は同じレコードを何度でも、何系統の consumer からでも読み返せる。「読んだら消える」queue と「読んでも残る」stream の違いは、単なる実装の細部ではなく、この章の残りすべてが依拠する前提である。

本ハンズオンの CLI は `produce` / `consume` の 2 サブコマンドで構成され（[apps/kafka/main.go:39](../apps/kafka/main.go#L39)-[47](../apps/kafka/main.go#L47)）、Kafka 実装には Go でワイヤプロトコルを直接話す `twmb/franz-go` を使っている。ブローカーに接続できない場合は「`make up` を先に実行してください」という具体的なヒント付きのエラーを返す（[apps/kafka/main.go:26](../apps/kafka/main.go#L26)-[31](../apps/kafka/main.go#L31)）。ここまでは ActiveMQ（`03_activemq.md` セクション 1）と共通する作法だが、この先の produce / consume の中身は queue とは別物になる。

---

## 2. partition / offset — ログ内の位置であり、コンシューマの所有物

Kafka の topic は 1 本の巨大なログではなく、**partition** という単位に分割された複数の追記ログの集合である。本ハンズオンの `orders` topic は `kafka-init` サービスが起動時に `--partitions 3` で作成しており（[docker-compose.yml:42](../docker-compose.yml#L42)-[52](../docker-compose.yml#L52)）、3 本の独立したログとして存在する。各 partition の内部では、レコードは 0 から始まる連番の **offset** を振られて追記されていく。offset は「そのレコードが partition 内のどの位置にあるか」を示すだけの単純な整数であり、ブローカーが管理する状態ではなく、後述する consumer group が「自分がどこまで読んだか」を記録するために使う値である。

`make demo-kafka` で 5 件を produce すると、各レコードがどの partition のどの offset に書き込まれたかがそのまま出力される（実測）。

```
sent: key=order-0001 partition=2 offset=0 {"id":"order-0001", ...}
sent: key=order-0002 partition=2 offset=1 {"id":"order-0002", ...}
sent: key=order-0003 partition=1 offset=0 {"id":"order-0003", ...}
sent: key=order-0004 partition=2 offset=2 {"id":"order-0004", ...}
sent: key=order-0005 partition=1 offset=1 {"id":"order-0005", ...}
```

上の実測ログを partition ごとに並べ替えると、3 本の独立したログの姿がはっきり見える。

```
partition 0: (空 — 今回の 5 件はいずれもここには書き込まれなかった)
partition 1: [offset=0: order-0003] [offset=1: order-0005]
partition 2: [offset=0: order-0001] [offset=1: order-0002] [offset=2: order-0004]
```

どの partition に書き込まれるかは、レコードの key によって決まる。

```go
// key に注文 ID を使う → 同じ key は必ず同じ partition = key 単位の順序保証
rec := &kgo.Record{Topic: "orders", Key: []byte(o.ID), Value: body}
```
（[apps/kafka/producer.go:31](../apps/kafka/producer.go#L31)-[32](../apps/kafka/producer.go#L32)）

key が指定されている場合、franz-go の既定パーティショナーは Java クライアントの既定パーティショナーと互換の murmur2 ハッシュ関数で key をハッシュ化し、そのハッシュ値を partition 数で割った余りで送信先の partition を決定する。同じ key は常に同じハッシュ値になるため必ず同じ partition に書き込まれ、これが「key 単位の順序保証」の実装原理である。逆に言えば、Kafka が保証するのは topic 全体の順序ではなく、**同一 key に限った partition 内の順序**でしかない。上の実測ログでも、5 つの異なる key が partition 1 と partition 2 の 2 つに分かれており、partition 0 にはレコードが来ていない。murmur2 ハッシュの分布はラウンドロビンのように key 数だけ全 partition に均等配分するものではないため、key の種類が少ない今回のような実行では、3 partition あってもすべてにレコードが行き渡るとは限らない。

---

## 3. consumer group — queue 的競合と pub/sub 的独立を 1 つのモデルで包含する

Kafka の設計上の勝利点は、consumer group という 1 つの仕組みで「queue のように分担して読む」動作と「pub/sub のように複数系が独立して全件読む」動作の両方を表現できることにある。

- **group 内**: 同じ group 名を名乗る複数の consumer プロセスがいれば、Kafka はその group に対して 3 つの partition を分担して割り当てる。1 つの partition を同時に読めるのは同一 group 内で 1 プロセスだけであり、これは queue の競合コンシューマに相当する挙動である。
- **group 間**: 異なる group 名を名乗れば、その group は他の group とは無関係に、同じ topic の全レコードを独立した offset で読める。これは pub/sub の fan-out に相当する挙動である。

`make demo-kafka` の consume 部分は `-group demo`（[apps/kafka/consumer.go:16](../apps/kafka/consumer.go#L16)）という単一プロセスで 3 partition すべてを担当するため、5 件全部を 1 プロセスが受信する（実測）。

```
received: group=demo partition=1 offset=0 key=order-0003 {"id":"order-0003", ...}
received: group=demo partition=1 offset=1 key=order-0005 {"id":"order-0005", ...}
received: group=demo partition=2 offset=0 key=order-0001 {"id":"order-0001", ...}
received: group=demo partition=2 offset=1 key=order-0002 {"id":"order-0002", ...}
received: group=demo partition=2 offset=2 key=order-0004 {"id":"order-0004", ...}
```

ここで重要なのは、**committed offset は「group がどこまで読んだか」を記録しているだけの値であり、それ以上でもそれ以下でもない**という点である。本ハンズオンの consumer は受信ループを抜けた後、明示的に offset を確定させてから終了する。

```go
// group の committed offset を確定させてから終了（次回は続きから読む）
if err := cl.CommitUncommittedOffsets(context.Background()); err != nil {
    return err
}
```
（[apps/kafka/consumer.go:55](../apps/kafka/consumer.go#L55)-[58](../apps/kafka/consumer.go#L58)）

`group=demo` を再度起動すればこの committed offset の続きから読み進めるが、それは「demo という group がそこまで読んだ」という記録が残っているからにすぎない。同じログを別の group 名で読めば、その group にとっての committed offset は存在せず、話は変わってくる — それが次のセクションの replay である。

---

## 4. replay — 新しい group 名で過去レコードを再消費する

`make demo-kafka-replay` は、レコードを再 produce することなく、過去に書き込まれた 5 件をもう一度最初から読み直す。仕掛けは 2 つの組み合わせでできている。

```makefile
# 新しい group 名 + --from-beginning = 再 produce せずに過去レコードを再消費（replay）
# group 名にタイムスタンプと PID を入れて毎回「初見の group」にする（秒内衝突回避）
demo-kafka-replay:
	go run ./apps/kafka consume -group replay-$$(date +%s)-$$$$ --from-beginning -max 5
```
（[Makefile:47](../Makefile#L47)-[50](../Makefile#L50)）

1 つ目は `--from-beginning` フラグで、これは内部で `kgo.ConsumeResetOffset(kgo.NewOffset().AtStart())` を有効にする（[apps/kafka/consumer.go:26](../apps/kafka/consumer.go#L26)-[28](../apps/kafka/consumer.go#L28)）。ただしここには見落としやすい注意点がある。**`ConsumeResetOffset(AtStart())` が実際に効くのは、その group にまだ committed offset が 1 件も存在しない場合に限られる**。もし `group=demo` のように一度でも `CommitUncommittedOffsets` を呼んだことのある既存 group 名に対して `--from-beginning` を指定しても、Kafka は committed offset を優先して続きから読み始めるため、先頭に巻き戻る効果は発生しない。

2 つ目の仕掛けはこれに対応している。`demo-kafka-replay` は毎回 `replay-$(date +%s)-$$`（タイムスタンプと実行プロセスの PID を組み合わせた文字列）という、過去に一度も使われたことのない group 名を生成する。だからこそ committed offset が存在しない「初見の group」として `--from-beginning` が意味を持ち、繰り返し実行しても毎回同じ 5 件を先頭から読み直せる。実行結果は次の通り（実測、group 名の末尾数字は実行のたびに変わる）。

```
received: group=replay-1783650609-49494 partition=1 offset=0 key=order-0003 {"id":"order-0003", ...}
received: group=replay-1783650609-49494 partition=1 offset=1 key=order-0005 {"id":"order-0005", ...}
received: group=replay-1783650609-49494 partition=2 offset=0 key=order-0001 {"id":"order-0001", ...}
received: group=replay-1783650609-49494 partition=2 offset=1 key=order-0002 {"id":"order-0002", ...}
received: group=replay-1783650609-49494 partition=2 offset=2 key=order-0004 {"id":"order-0004", ...}
```

producer 側は一切動かしておらず、`make demo-kafka` が過去に書き込んだ同じ 5 件のレコードが、`group=demo` の消費とは無関係にもう一度読み出されている。これが queue では原理的にできないことである — SQS や ActiveMQ の queue はメッセージを ACK すれば消えてしまうため、後から「もう一度最初から読み直す」手段が存在しない。

この replay という能力は、実務では次のような場面で使われる。

- **障害後の再処理**: 下流の集計バッチがバグでクラッシュし、一定期間分のレコードを正しく処理できていなかった場合、新しい group でその期間の先頭から読み直せば、失われたはずの処理をやり直せる。
- **新システムへのバックフィル**: 新しく作った分析基盤に、稼働開始前の過去データも取り込みたい場合、新しい group を作って topic の先頭から読ませるだけで、過去分と以降のリアルタイム分を同じ経路で流し込める。
- **バグ修正後の再集計**: 集計ロジックにバグがあったと判明した場合、修正後のコードで新しい group から先頭を読み直せば、過去のレコードすべてを新ロジックで計算し直せる。producer 側は何も変更する必要がない。

---

## 5. なぜ高スループットか — 速いのは魔法ではなく、ランダムアクセスを設計で排除したから

Kafka が高スループットを謳える理由は、特別なハードウェアや魔法のアルゴリズムではなく、**ランダムアクセスを徹底して設計から排除した**ことに尽きる。

- **追記のみのシーケンシャル I/O**: partition のログは末尾に追記するだけで、途中のレコードを書き換えたり削除したりしない。ディスク（特に回転式 HDD）はランダムな位置への読み書きが最も遅く、連続した領域への追記が最も速い。Kafka の書き込みパターンはこの「最も速い経路」だけを使うように設計されている。
- **OS ページキャッシュへの依存**: Kafka はレコードを JVM のヒープ上にキャッシュしようとせず、書き込みも読み出しも OS のページキャッシュを積極的に利用する。直近に書かれた、あるいは直近に読まれたレコードはページキャッシュ上に乗るため、多くの読み取りはディスクまで降りずにメモリ速度で処理される。
- **sendfile によるゼロコピー転送**: consumer へのレスポンスは、多くの場合ページキャッシュ上のデータをそのままネットワークソケットへ渡す `sendfile` システムコールで送られ、カーネル空間からユーザー空間へデータをコピーする手間を省く（この最適化はプレーンテキスト転送の場合に成立する。TLS を有効にすると暗号化のためにデータを一度ユーザー空間へ持ち上げる必要が生じ、ゼロコピー転送の恩恵は失われて通常のコピーを伴う経路に戻る）。
- **バッチングと圧縮**: producer 側でレコードをまとめて 1 回のリクエストに詰め、必要なら圧縮してから送ることで、ネットワークのラウンドトリップ回数と転送バイト数の両方を減らす。

この 4 つはいずれも「ランダムアクセスや不要なコピーを避ける」という同じ方針の異なる現れであり、Kafka の速さは特殊な仕組みの結果ではなく、素直な工学的判断の積み重ねの結果である。

---

## 6. 運用の現実 — broker / controller、rebalance、retention の容量管理

ここまでの話は Kafka の設計の強みだが、その強みは無償では手に入らない。SQS のようなフルマネージド queue と比べると、Kafka は運用側が引き受ける責務がはるかに大きい。

本ハンズオンの compose は KRaft モード（ZooKeeper を使わない Kafka 自身によるメタデータ管理）で、1 つのプロセスに broker と controller の両ロールを同居させている（`KAFKA_PROCESS_ROLES: broker,controller`、[docker-compose.yml:20](../docker-compose.yml#L20)-[40](../docker-compose.yml#L40)）。これは学習用に単純化した最小構成であり、本番相当のクラスタでは複数の broker ノードと controller の合意形成（quorum）を運用し、partition のリーダー配置や ISR（in-sync replica）の健全性、controller クォーラムの整合性まで監視対象に入る。

**rebalance** も queue にはない Kafka 固有の運用課題である。consumer group のメンバー構成が変わる（consumer の追加・離脱・クラッシュ）たびに、Kafka はその group に対する partition の割り当てをやり直す。本ハンズオンの demo は group あたり 1 プロセスしか起動しないため rebalance は発生しないが、本番で 1 つの group に何台もの consumer をぶら下げる構成では、メンバー構成の変化のたびに一時的な割り当てのやり直しが発生し、その間は該当 partition の消費が滞る。この影響を小さくする割り当て戦略（cooperative-sticky など）を選ぶかどうかも運用上の検討事項になる。

**retention の容量管理**もセクション 1 で述べた「消費しても消えない」ことの裏返しである。queue はメッセージを ACK すれば即座に解放されるが、Kafka の partition は retention（時間または容量ベース）に達するまでレコードを溜め込み続けるため、topic の書き込み量と retention 期間を掛け合わせたディスク容量をあらかじめ見積もり、実際の使用量を監視し続ける必要がある。consumer の消費が遅れて追いつかなくなった場合も、レコードは消えずに残るぶん実害は避けられるが、その分ディスクの逼迫は早まる。

自分でクラスタを構築・監視・拡張する必要があるこの運用負担こそが、Kafka をセルフホストで使うか、マネージドサービス（Amazon MSK や Confluent Cloud など）に寄せるかを検討する動機になる。この比較は `07_selection_guide.md` で改めて扱う。

---

## 7. make demo-kafka / demo-kafka-replay の実行手順と観察ポイント

スタックが起動していない場合は `make up` を先に実行する（`docker compose up -d` とヘルスチェック待ち）。基本の produce / consume は次で確認する。

```bash
make demo-kafka
```

これは 5 件を `produce -n 5` した後、`-group demo` で `consume -max 5` する最短経路である（[Makefile:43](../Makefile#L43)-[45](../Makefile#L45)）。実行結果はセクション 2・3 に掲載した実測ログの通りで、観察ポイントは 2 つある。1 つ目は produce 時の `partition=X offset=N` — 同じ key の注文 ID が常に同じ partition に書かれること（本実行では key ごとの偏りで partition 1 と partition 2 のみが使われた）。2 つ目は consume 側の出力順が produce した順（`order-0001`〜`0005`）とは一致せず、`partition=1` の 2 件、`partition=2` の 3 件という partition 単位のまとまりで出てくること — Kafka が保証するのは topic 全体の順序ではなく partition 内の順序だけである、という事実がそのままログに現れている。

replay は次で確認する。

```bash
make demo-kafka-replay
```

このターゲットは produce を一切行わず、タイムスタンプと PID から作った初見の group 名で `--from-beginning` 付き consume だけを実行する（[Makefile:47](../Makefile#L47)-[50](../Makefile#L50)）。観察ポイントは、`make demo-kafka` で書き込んだのと同じ 5 件・同じ partition・同じ offset の組み合わせが、`group=demo` の消費実績とは無関係にもう一度そのまま出てくることである。何度実行しても group 名が毎回変わるため、このコマンドは繰り返し実行できる。もし同じ group 名を使い回して `--from-beginning` を付けた場合は、すでに committed offset が存在するために巻き戻りは起こらず、続きの読み取り（この場合は 0 件、または timeout）になる点は本文中で述べた通りである。
